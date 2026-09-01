package orderbook

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agentic-sim-trading/market-simulator/internal/fees"
	"github.com/agentic-sim-trading/market-simulator/internal/market"
	"github.com/agentic-sim-trading/market-simulator/internal/orders"
	"github.com/agentic-sim-trading/market-simulator/internal/portfolio"
	"github.com/agentic-sim-trading/market-simulator/internal/simulation"
	"github.com/agentic-sim-trading/market-simulator/internal/universe"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ProcessSimulationBar matches pending orders against one 60m bar and snapshots on session close.
func (e *Engine) ProcessSimulationBar(ctx context.Context, simulationID uuid.UUID, barTs time.Time) error {
	if e.Pool == nil || e.Data == nil || e.PM == nil {
		return errors.New("engine not fully configured")
	}

	row, err := simulation.Get(ctx, e.Pool, simulationID)
	if err != nil {
		return err
	}
	particip := simulation.ParticipationRate(row.Config)
	sessionDay := market.SessionCalendarUTC(barTs)

	nextBar, nextErr := e.Data.NextBar(ctx, market.Interval60m, barTs)
	lastOfSession := nextErr != nil || nextBar.IsZero() || !market.SameSession(barTs, nextBar)
	var nextMatch *time.Time
	if !lastOfSession {
		n := nextBar
		nextMatch = &n
	}

	tx, err := e.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	pending, err := orders.ListPendingForTs(ctx, tx, simulationID, barTs)
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
		prevBar, err := e.Data.PrevTradingDayBar(ctx, sym, sessionDay)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil {
			prevCloseCache[sym] = prevBar.Close
			prevVol[sym] = prevBar.Volume
		}
		sessionBars, err := e.Data.SessionBarsUpTo(ctx, sym, market.Interval60m, barTs, barTs)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		halt := false
		for _, b := range sessionBars {
			if CircuitBandBreached(b.High, b.Low, prevCloseCache[sym]) {
				halt = true
				break
			}
		}
		breakerOn[sym] = halt
	}

	var redisPayloads []map[string]any

	for _, o := range pending {
		reason := e.tryFillHourly(ctx, tx, simulationID, barTs, sessionDay, o, breakerOn, prevVol, prevCloseCache, particip, lastOfSession, nextMatch, &redisPayloads)
		if reason == ReasonDefer {
			if lastOfSession || nextMatch == nil {
				if o.FilledQuantity > 0 {
					if err := orders.ApplySliceFill(ctx, tx, o.ID, 0, 0, 0, sessionDay, barTs, nil, fees.Breakdown{}, 0); err != nil {
						return err
					}
					continue
				}
				if err := orders.MarkRejected(ctx, tx, o.ID, "day_order_expired"); err != nil {
					return err
				}
				redisPayloads = append(redisPayloads, map[string]any{
					"event":         "order.rejected",
					"order_id":      o.ID.String(),
					"simulation_id": simulationID.String(),
					"agent_id":      o.AgentID.String(),
					"symbol":        o.Symbol,
					"reason":        "day_order_expired",
					"match_on_date": sessionDay.Format(time.DateOnly),
					"bar_ts":        barTs.UTC().Format(time.RFC3339),
				})
				continue
			}
			if err := orders.UpdateMatchOnTs(ctx, tx, o.ID, *nextMatch, sessionDay); err != nil {
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
				"match_on_date": sessionDay.Format(time.DateOnly),
				"bar_ts":        barTs.UTC().Format(time.RFC3339),
			})
		}
	}

	if lastOfSession {
		if err := e.writeSessionSnapshots(ctx, tx, simulationID, barTs, sessionDay, &redisPayloads); err != nil {
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
		"date":          sessionDay.Format(time.DateOnly),
		"bar_ts":        barTs.UTC().Format(time.RFC3339),
		"interval":      market.Interval60m,
	})
	return nil
}

func (e *Engine) tryFillHourly(
	ctx context.Context,
	tx pgx.Tx,
	simulationID uuid.UUID,
	barTs, sessionDay time.Time,
	o orders.Row,
	breakerOn map[string]bool,
	prevVol map[string]int64,
	prevCloseCache map[string]float64,
	particip float64,
	lastOfSession bool,
	nextMatch *time.Time,
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

	bar, err := e.Data.IntradayBarOnTs(ctx, o.Symbol, market.Interval60m, barTs)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ReasonDefer
		}
		return "market_data_error"
	}
	if BelowMinPrice(bar.Open) {
		return "below_minimum_price"
	}
	if LiquidityExceeded(o.Quantity, prevVol[o.Symbol]) {
		return "liquidity_limit_exceeded"
	}

	remaining := o.WorkingQty()
	limitPx := 0.0
	if o.Price != nil {
		limitPx = *o.Price
	}
	path := ResolvePathFill(o.OrderType, o.Side, limitPx, remaining, bar, particip)
	switch path.Decision {
	case DeferFill:
		return ReasonDefer
	case RejectFill:
		return path.Reason
	}

	fillPx := path.FillPrice
	if path.Aggressive {
		fillPx = ApplyAggressiveSpread(o.Side, fillPx, HalfSpread(bar))
	}
	fillPx = ApplyMarketImpact(o.Side, path.FillQty, prevVol[o.Symbol], fillPx)
	remainingAfter := remaining - path.FillQty
	var next *time.Time
	if remainingAfter > 0 {
		if lastOfSession || nextMatch == nil {
			remainingAfter = 0
		} else {
			next = nextMatch
		}
	}

	var reason string
	switch strings.ToLower(strings.TrimSpace(o.Side)) {
	case "buy":
		reason = e.fillBuy(ctx, tx, simulationID, sessionDay, o, fillPx, path.FillQty, remainingAfter, barTs, next, prevCloseCache, redisPayloads)
	case "sell":
		br := e.hourlySellFees(ctx, tx, simulationID, sessionDay, o, path.FillQty, fillPx)
		reason = e.fillSell(ctx, tx, simulationID, sessionDay, o, fillPx, path.FillQty, remainingAfter, barTs, next, br, redisPayloads)
	default:
		return "invalid_side"
	}
	return reason
}

func (e *Engine) hourlySellFees(ctx context.Context, tx pgx.Tx, simulationID uuid.UUID, sessionDay time.Time, o orders.Row, fillQty int, fillPx float64) fees.Breakdown {
	tradeVal := float64(fillQty) * fillPx
	pid, err := e.PM.PortfolioIDTx(ctx, tx, simulationID, o.AgentID)
	if err != nil {
		return fees.SellFees(tradeVal)
	}
	held, err := e.PM.HoldingQtyTx(ctx, tx, pid, o.Symbol)
	if err != nil {
		return fees.SellFees(tradeVal)
	}
	bought, err := orders.FilledBuyQtyOnSession(ctx, tx, simulationID, o.AgentID, o.Symbol, sessionDay)
	if err != nil {
		return fees.SellFees(tradeVal)
	}
	overnight := held - bought
	if overnight < 0 {
		overnight = 0
	}
	deliveryQty := fillQty
	if overnight < deliveryQty {
		deliveryQty = overnight
	}
	intradayQty := fillQty - deliveryQty
	return fees.SellFeesMixed(float64(deliveryQty)*fillPx, float64(intradayQty)*fillPx)
}

func (e *Engine) writeSessionSnapshots(ctx context.Context, tx pgx.Tx, simulationID uuid.UUID, barTs, sessionDay time.Time, redisPayloads *[]map[string]any) error {
	agents, err := e.PM.ListAgentIDsTx(ctx, tx, simulationID)
	if err != nil {
		return err
	}
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
				if err := e.PM.UpsertSnapshotTx(ctx, tx, simulationID, aid, sessionDay, total, cash, invested); err != nil {
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
			bar, err := e.Data.IntradayBarAtOrBefore(ctx, sym, market.Interval60m, barTs)
			if err != nil {
				return fmt.Errorf("session bar %s: %w", sym, err)
			}
			h := holdings[sym]
			posVal := float64(h.Quantity) * bar.Close
			total += posVal
			invested += posVal
		}
		if err := e.PM.UpsertSnapshotTx(ctx, tx, simulationID, aid, sessionDay, total, cash, invested); err != nil {
			return err
		}
		if total <= 0 {
			if err := e.PM.SetDisqualifiedTx(ctx, tx, simulationID, aid); err != nil {
				return err
			}
			*redisPayloads = append(*redisPayloads, map[string]any{
				"event":         "agent.disqualified",
				"simulation_id": simulationID.String(),
				"agent_id":      aid.String(),
				"reason":        "portfolio_value_zero",
				"date":          sessionDay.Format(time.DateOnly),
			})
		}
	}
	return nil
}
