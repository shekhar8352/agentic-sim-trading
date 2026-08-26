package orderbook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

// Engine executes EOD matching per docs/rules.md §7–9 (market, limit, and stop).
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
		breakerOn[sym] = CircuitTriggered(barD.Open, prevBar.Close)
	}

	var redisPayloads []map[string]any

	for _, o := range pending {
		reason := e.tryFillOrder(ctx, tx, simulationID, tradeDate, o, breakerOn, prevVol, prevCloseCache, &redisPayloads)
		if reason == ReasonDefer {
			next, err := e.Data.NextTradingDay(ctx, tradeDate)
			if err != nil || next.IsZero() {
				if err := orders.MarkRejected(ctx, tx, o.ID, "not_filled_before_end"); err != nil {
					return err
				}
				redisPayloads = append(redisPayloads, map[string]any{
					"event":         "order.rejected",
					"order_id":      o.ID.String(),
					"simulation_id": simulationID.String(),
					"agent_id":      o.AgentID.String(),
					"symbol":        o.Symbol,
					"reason":        "not_filled_before_end",
					"match_on_date": tradeDate.UTC().Format(time.DateOnly),
				})
				continue
			}
			if err := orders.UpdateMatchOnDate(ctx, tx, o.ID, next); err != nil {
				return err
			}
			continue
		}
		if reason != "" {
			if err := orders.MarkRejected(ctx, tx, o.ID, reason); err != nil {
				return err
			}
			redisPayloads = append(redisPayloads, map[string]any{
				"event":         "order.rejected",
				"order_id":      o.ID.String(),
				"simulation_id": simulationID.String(),
				"agent_id":      o.AgentID.String(),
				"symbol":        o.Symbol,
				"reason":        reason,
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
		status, err := e.PM.AgentStatusTx(ctx, tx, simulationID, aid)
		if err != nil {
			return err
		}
		if status == portfolio.StatusDisqualified {
			total, cash, invested, ok, err := e.PM.LastSnapshotTx(ctx, tx, simulationID, aid)
			if err != nil {
				return err
			}
			if ok {
				if err := e.PM.UpsertSnapshotTx(ctx, tx, simulationID, aid, tradeDay, total, cash, invested); err != nil {
					return err
				}
			}
			continue
		}

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
		if total <= 0 {
			if err := e.PM.SetDisqualifiedTx(ctx, tx, simulationID, aid); err != nil {
				return err
			}
			redisPayloads = append(redisPayloads, map[string]any{
				"event":         "agent.disqualified",
				"simulation_id": simulationID.String(),
				"agent_id":      aid.String(),
				"reason":        "portfolio_value_zero",
				"date":          tradeDay.Format(time.DateOnly),
			})
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
	status, err := e.PM.AgentStatusTx(ctx, tx, simulationID, o.AgentID)
	if err == nil && status == portfolio.StatusDisqualified {
		return "agent_disqualified"
	}
	if breakerOn[o.Symbol] {
		return "circuit_breaker_triggered"
	}
	if !universe.IsNifty50(o.Symbol) {
		return "invalid_symbol"
	}

	barD, err := e.Data.BarOnDate(ctx, o.Symbol, tradeDate)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "invalid_symbol"
		}
		return "market_data_error"
	}
	if BelowMinPrice(barD.Open) {
		return "below_minimum_price"
	}

	if LiquidityExceeded(o.Quantity, prevVol[o.Symbol]) {
		return "liquidity_limit_exceeded"
	}

	limitPx := 0.0
	if o.Price != nil {
		limitPx = *o.Price
	}
	fillPx, decision, reason := ResolveFill(o.OrderType, o.Side, limitPx, barD)
	switch decision {
	case DeferFill:
		return ReasonDefer
	case RejectFill:
		return reason
	}

	fillPx = ApplyMarketImpact(o.Side, o.Quantity, prevVol[o.Symbol], fillPx)

	switch strings.ToLower(strings.TrimSpace(o.Side)) {
	case "buy":
		return e.fillBuy(ctx, tx, simulationID, tradeDate, o, fillPx, prevCloseCache, redisPayloads)
	case "sell":
		return e.fillSell(ctx, tx, simulationID, tradeDate, o, fillPx, redisPayloads)
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
	fillPx float64,
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

	tradeVal := float64(o.Quantity) * fillPx
	br := fees.BuyFees(tradeVal)
	totalDebit := br.TotalDebit(tradeVal)
	if cash < totalDebit {
		return "insufficient_cash"
	}

	h := holdings[o.Symbol]
	newStockExposure := float64(h.Quantity+o.Quantity) * prevSym
	if ConcentrationExceeded(newStockExposure, pv) {
		return "concentration_limit_exceeded"
	}

	newCash := cash - totalDebit
	if err := e.PM.SetCashTx(ctx, tx, pid, newCash); err != nil {
		return "portfolio_error"
	}
	if err := e.PM.UpsertHoldingAfterBuy(ctx, tx, pid, o.Symbol, o.Quantity, fillPx); err != nil {
		return "portfolio_error"
	}
	if err := orders.MarkFilled(ctx, tx, o.ID, fillPx, tradeDate.UTC(), br, tradeVal); err != nil {
		return "persist_error"
	}
	*redisPayloads = append(*redisPayloads, map[string]any{
		"event":         "order.filled",
		"order_id":      o.ID.String(),
		"simulation_id": simulationID.String(),
		"agent_id":      o.AgentID.String(),
		"symbol":        o.Symbol,
		"side":          "buy",
		"quantity":      o.Quantity,
		"filled_price":  fillPx,
		"fees_total":    br.Total,
		"fees":          br,
		"trade_value":   tradeVal,
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
	fillPx float64,
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

	tradeVal := float64(o.Quantity) * fillPx
	br := fees.SellFees(tradeVal)
	credit := br.TotalCredit(tradeVal)

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
	if err := orders.MarkFilled(ctx, tx, o.ID, fillPx, tradeDate.UTC(), br, tradeVal); err != nil {
		return "persist_error"
	}
	*redisPayloads = append(*redisPayloads, map[string]any{
		"event":         "order.filled",
		"order_id":      o.ID.String(),
		"simulation_id": simulationID.String(),
		"agent_id":      o.AgentID.String(),
		"symbol":        o.Symbol,
		"side":          "sell",
		"quantity":      o.Quantity,
		"filled_price":  fillPx,
		"fees_total":    br.Total,
		"fees":          br,
		"trade_value":   tradeVal,
		"net_credit":    credit,
		"match_on_date": tradeDate.UTC().Format(time.DateOnly),
	})
	return ""
}
