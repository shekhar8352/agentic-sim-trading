package orderbook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/agentic-sim-trading/market-simulator/internal/fees"
	"github.com/agentic-sim-trading/market-simulator/internal/market"
	"github.com/agentic-sim-trading/market-simulator/internal/orders"
	"github.com/agentic-sim-trading/market-simulator/internal/portfolio"
	"github.com/agentic-sim-trading/market-simulator/internal/universe"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const minStockPrice = 10.0
const concentrationPct = 0.20

// Engine executes Phase 1 market orders per docs/rules.md §7–9.
type Engine struct {
	Pool *pgxpool.Pool
	Data *market.Data
	PM   *portfolio.Manager
	RDB  *redis.Client
}

// NewEngine constructs a matcher backed by Postgres OHLCV and portfolios.
func NewEngine(pool *pgxpool.Pool, data *market.Data, pm *portfolio.Manager, rdb *redis.Client) *Engine {
	return &Engine{Pool: pool, Data: data, PM: pm, RDB: rdb}
}

func (e *Engine) publish(ctx context.Context, payload map[string]any) {
	if e.RDB == nil {
		return
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = e.RDB.Publish(ctx, "sim.events", string(b)).Err()
}

func (e *Engine) prevClose(ctx context.Context, sym string, tradeDate time.Time, cache map[string]float64) (float64, error) {
	if x, ok := cache[sym]; ok {
		return x, nil
	}
	prevBar, err := e.Data.PrevTradingDayBar(ctx, sym, tradeDate)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			cache[sym] = 0
			return 0, nil
		}
		return 0, err
	}
	cache[sym] = prevBar.Close
	return prevBar.Close, nil
}

func (e *Engine) portfolioPV(ctx context.Context, tradeDate time.Time, cash float64, holdings map[string]portfolio.Holding, cache map[string]float64) (float64, error) {
	v := cash
	for sym, h := range holdings {
		pc, err := e.prevClose(ctx, sym, tradeDate, cache)
		if err != nil {
			return 0, err
		}
		v += float64(h.Quantity) * pc
	}
	return v, nil
}

// ProcessSimulationDay matches pending orders for tradeDate and writes EOD snapshots (Step 8).
func (e *Engine) ProcessSimulationDay(ctx context.Context, simulationID uuid.UUID, tradeDate time.Time) error {
	if e.Pool == nil || e.Data == nil || e.PM == nil {
		return errors.New("engine not fully configured")
	}

	tx, err := e.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	pending, err := orders.ListPendingForDate(ctx, tx, simulationID, tradeDate)
	if err != nil {
		return err
	}

	prevCloseCache := make(map[string]float64)
	prevVol := make(map[string]int64)
	breakerOn := make(map[string]bool)

	var pendingSymbols []string
	seen := map[string]struct{}{}
	for _, o := range pending {
		if _, ok := seen[o.Symbol]; ok {
			continue
		}
		seen[o.Symbol] = struct{}{}
		pendingSymbols = append(pendingSymbols, o.Symbol)
	}

	for _, sym := range pendingSymbols {
		barD, err := e.Data.BarOnDate(ctx, sym, tradeDate)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				breakerOn[sym] = true
				continue
			}
			return err
		}
		prevBar, err := e.Data.PrevTradingDayBar(ctx, sym, tradeDate)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				prevCloseCache[sym] = 0
				prevVol[sym] = 0
				breakerOn[sym] = false
				continue
			}
			return err
		}
		prevCloseCache[sym] = prevBar.Close
		prevVol[sym] = prevBar.Volume
		pc := prevBar.Close
		open := barD.Open
		if pc > 0 {
			pct := (open - pc) / pc * 100
			if math.Abs(pct) > 10 {
				breakerOn[sym] = true
				continue
			}
		}
		breakerOn[sym] = false
	}

	var redisPayloads []map[string]any

	for _, o := range pending {
		reason := e.tryFillOrder(ctx, tx, simulationID, tradeDate, o, breakerOn, prevVol, prevCloseCache, &redisPayloads)
		if reason != "" {
			if err := orders.MarkRejected(ctx, tx, o.ID, reason); err != nil {
				return err
			}
			redisPayloads = append(redisPayloads, map[string]any{
				"event":          "order.rejected",
				"order_id":       o.ID.String(),
				"simulation_id":  simulationID.String(),
				"agent_id":       o.AgentID.String(),
				"symbol":         o.Symbol,
				"reason":         reason,
				"match_on_date": tradeDate.UTC().Format(time.DateOnly),
			})
		}
	}

	agents, err := e.PM.ListAgentIDsTx(ctx, tx, simulationID)
	if err != nil {
		return err
	}

	tradeDay := tradeDate.UTC().Truncate(24 * time.Hour)

	for _, aid := range agents {
		pid, err := e.PM.PortfolioIDTx(ctx, tx, simulationID, aid)
		if err != nil {
			return err
		}
		cash, err := e.PM.GetCashTx(ctx, tx, pid)
		if err != nil {
			return err
		}
		holdings, err := e.PM.LoadHoldingsTx(ctx, tx, pid)
		if err != nil {
			return err
		}
		total := cash
		invested := 0.0
		for sym := range holdings {
			bar, err := e.Data.BarOnDate(ctx, sym, tradeDate)
			if err != nil {
				return fmt.Errorf("eod bar %s: %w", sym, err)
			}
			h := holdings[sym]
			posVal := float64(h.Quantity) * bar.Close
			total += posVal
			invested += posVal
		}
		if err := e.PM.UpsertSnapshotTx(ctx, tx, simulationID, aid, tradeDay, total, cash, invested); err != nil {
			return err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	for _, p := range redisPayloads {
		e.publish(ctx, p)
	}
	e.publish(ctx, map[string]any{
		"event":         "sim.tick.processed",
		"simulation_id": simulationID.String(),
		"date":          tradeDate.UTC().Format(time.DateOnly),
	})
	return nil
}

func (e *Engine) tryFillOrder(
	ctx context.Context,
	tx pgx.Tx,
	simulationID uuid.UUID,
	tradeDate time.Time,
	o orders.Row,
	breakerOn map[string]bool,
	prevVol map[string]int64,
	prevCloseCache map[string]float64,
	redisPayloads *[]map[string]any,
) string {
	if breakerOn[o.Symbol] {
		return "circuit_breaker_triggered"
	}
	if !universe.IsNifty50(o.Symbol) {
		return "invalid_symbol"
	}
	if strings.EqualFold(o.OrderType, "market") == false {
		return "order_type_not_supported"
	}

	barD, err := e.Data.BarOnDate(ctx, o.Symbol, tradeDate)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "invalid_symbol"
		}
		return "market_data_error"
	}
	open := barD.Open
	if open < minStockPrice {
		return "below_minimum_price"
	}

	maxQty := int(math.Floor(0.01 * float64(prevVol[o.Symbol])))
	if o.Quantity > maxQty {
		return "liquidity_limit_exceeded"
	}

	switch strings.ToLower(strings.TrimSpace(o.Side)) {
	case "buy":
		return e.fillBuy(ctx, tx, simulationID, tradeDate, o, open, prevCloseCache, redisPayloads)
	case "sell":
		return e.fillSell(ctx, tx, simulationID, tradeDate, o, open, redisPayloads)
	default:
		return "invalid_side"
	}
}

func (e *Engine) fillBuy(
	ctx context.Context,
	tx pgx.Tx,
	simulationID uuid.UUID,
	tradeDate time.Time,
	o orders.Row,
	open float64,
	prevCloseCache map[string]float64,
	redisPayloads *[]map[string]any,
) string {
	pid, err := e.PM.PortfolioIDTx(ctx, tx, simulationID, o.AgentID)
	if err != nil {
		return "portfolio_not_found"
	}
	cash, err := e.PM.GetCashTx(ctx, tx, pid)
	if err != nil {
		return "portfolio_not_found"
	}
	holdings, err := e.PM.LoadHoldingsTx(ctx, tx, pid)
	if err != nil {
		return "portfolio_error"
	}

	pv, err := e.portfolioPV(ctx, tradeDate, cash, holdings, prevCloseCache)
	if err != nil {
		return "portfolio_error"
	}

	prevSym, err := e.prevClose(ctx, o.Symbol, tradeDate, prevCloseCache)
	if err != nil {
		return "portfolio_error"
	}

	tradeVal := float64(o.Quantity) * open
	totalDebit, feeTotal := fees.BuyDebit(tradeVal)
	if cash < totalDebit {
		return "insufficient_cash"
	}

	h := holdings[o.Symbol]
	newStockExposure := float64(h.Quantity+o.Quantity) * prevSym
	if pv > 0 && newStockExposure > concentrationPct*pv+1e-9 {
		return "concentration_limit_exceeded"
	}

	newCash := cash - totalDebit
	if err := e.PM.SetCashTx(ctx, tx, pid, newCash); err != nil {
		return "portfolio_error"
	}
	if err := e.PM.UpsertHoldingAfterBuy(ctx, tx, pid, o.Symbol, o.Quantity, open); err != nil {
		return "portfolio_error"
	}
	if err := orders.MarkFilled(ctx, tx, o.ID, open, tradeDate.UTC(), feeTotal, tradeVal); err != nil {
		return "persist_error"
	}
	*redisPayloads = append(*redisPayloads, map[string]any{
		"event":          "order.filled",
		"order_id":       o.ID.String(),
		"simulation_id":  simulationID.String(),
		"agent_id":       o.AgentID.String(),
		"symbol":         o.Symbol,
		"side":           "buy",
		"quantity":       o.Quantity,
		"filled_price":   open,
		"fees_total":     feeTotal,
		"trade_value":    tradeVal,
		"match_on_date": tradeDate.UTC().Format(time.DateOnly),
	})
	return ""
}

func (e *Engine) fillSell(
	ctx context.Context,
	tx pgx.Tx,
	simulationID uuid.UUID,
	tradeDate time.Time,
	o orders.Row,
	open float64,
	redisPayloads *[]map[string]any,
) string {
	pid, err := e.PM.PortfolioIDTx(ctx, tx, simulationID, o.AgentID)
	if err != nil {
		return "portfolio_not_found"
	}
	qtyHeld, err := e.PM.HoldingQtyTx(ctx, tx, pid, o.Symbol)
	if err != nil {
		return "portfolio_error"
	}
	if qtyHeld < o.Quantity {
		return "insufficient_holdings"
	}

	tradeVal := float64(o.Quantity) * open
	credit, feeTotal := fees.SellCredit(tradeVal)

	cash, err := e.PM.GetCashTx(ctx, tx, pid)
	if err != nil {
		return "portfolio_error"
	}
	if err := e.PM.SetCashTx(ctx, tx, pid, cash+credit); err != nil {
		return "portfolio_error"
	}
	if err := e.PM.ReduceHoldingAfterSell(ctx, tx, pid, o.Symbol, o.Quantity); err != nil {
		return "portfolio_error"
	}
	if err := orders.MarkFilled(ctx, tx, o.ID, open, tradeDate.UTC(), feeTotal, tradeVal); err != nil {
		return "persist_error"
	}
	*redisPayloads = append(*redisPayloads, map[string]any{
		"event":          "order.filled",
		"order_id":       o.ID.String(),
		"simulation_id":  simulationID.String(),
		"agent_id":       o.AgentID.String(),
		"symbol":         o.Symbol,
		"side":           "sell",
		"quantity":       o.Quantity,
		"filled_price":   open,
		"fees_total":     feeTotal,
		"trade_value":    tradeVal,
		"net_credit":     credit,
		"match_on_date": tradeDate.UTC().Format(time.DateOnly),
	})
	return ""
}
