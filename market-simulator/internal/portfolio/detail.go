package portfolio

import (
	"context"
	"errors"
	"time"

	"github.com/agentic-sim-trading/market-simulator/internal/market"
	"github.com/agentic-sim-trading/market-simulator/pkg/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// StartingCapitalINR is docs/rules.md §1 starting cash per agent.
const StartingCapitalINR = 1_000_000.0

// BarLookup loads OHLCV for mark-to-market at the simulated calendar date.
type BarLookup interface {
	BarOnDate(ctx context.Context, symbol string, day time.Time) (models.Quote, error)
}

// IntradayMark is implemented by *market.Data for 60m portfolio marks.
type IntradayMark interface {
	IntradayBarAtOrBefore(ctx context.Context, symbol, interval string, asOf time.Time) (models.Quote, error)
}

// HoldingDetail is one equity line with simulated marks (Step 9 GET portfolio).
type HoldingDetail struct {
	Symbol        string  `json:"symbol"`
	Quantity      int     `json:"quantity"`
	AvgBuyPrice   float64 `json:"avg_buy_price"`
	MarkPrice     float64 `json:"mark_price"`
	PositionValue float64 `json:"position_value"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
}

// PortfolioDetail is the agent-facing portfolio payload (cash, holdings, MTM, P&L).
type PortfolioDetail struct {
	SimulationID    uuid.UUID       `json:"simulation_id"`
	AgentID         uuid.UUID       `json:"agent_id"`
	AsOfDate        string          `json:"as_of_date"`
	AsOfTs          string          `json:"as_of_ts,omitempty"`
	Cash            float64         `json:"cash"`
	Holdings        []HoldingDetail `json:"holdings"`
	InvestedValue   float64         `json:"invested_value"`
	TotalValue      float64         `json:"total_value"`
	TotalPnL        float64         `json:"total_pnl"`
	TotalReturnPct  float64         `json:"total_return_pct"`
	StartingCapital float64         `json:"starting_capital"`
}

// GetPortfolioDetail loads cash and holdings and marks positions at asOf using BarLookup.
func (m *Manager) GetPortfolioDetail(ctx context.Context, simulationID, agentID uuid.UUID, asOf time.Time, bars BarLookup) (*PortfolioDetail, error) {
	if m.pool == nil {
		return nil, errors.New("database not configured")
	}
	if bars == nil {
		return nil, errors.New("market data required for portfolio marks")
	}

	asOfDay := asOf.UTC().Truncate(24 * time.Hour)
	hourlyMark := !asOf.Equal(asOfDay)
	if hourlyMark {
		asOfDay = market.SessionCalendarUTC(asOf)
	}

	var pid int64
	err := m.pool.QueryRow(ctx, `
		SELECT id FROM portfolios WHERE simulation_id = $1 AND agent_id = $2
	`, simulationID, agentID).Scan(&pid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		return nil, err
	}

	var cash float64
	if err := m.pool.QueryRow(ctx, `SELECT cash::float8 FROM portfolios WHERE id = $1`, pid).Scan(&cash); err != nil {
		return nil, err
	}

	rows, err := m.pool.Query(ctx, `
		SELECT symbol, quantity, avg_buy_price::float8 FROM holdings WHERE portfolio_id = $1 ORDER BY symbol
	`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := &PortfolioDetail{
		SimulationID:    simulationID,
		AgentID:         agentID,
		AsOfDate:        asOfDay.Format(time.DateOnly),
		Cash:            cash,
		Holdings:        nil,
		StartingCapital: StartingCapitalINR,
	}
	if hourlyMark {
		out.AsOfTs = asOf.UTC().Format(time.RFC3339)
	}

	invested := 0.0
	for rows.Next() {
		var sym string
		var qty int
		var avg float64
		if err := rows.Scan(&sym, &qty, &avg); err != nil {
			return nil, err
		}
		q, err := bars.BarOnDate(ctx, sym, asOfDay)
		if hourlyMark {
			if im, ok := bars.(IntradayMark); ok {
				q, err = im.IntradayBarAtOrBefore(ctx, sym, market.Interval60m, asOf)
			}
		}
		var mark float64
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				mark = 0
			} else {
				return nil, err
			}
		} else {
			mark = q.Close
		}
		posVal := float64(qty) * mark
		unreal := float64(qty) * (mark - avg)
		invested += posVal
		out.Holdings = append(out.Holdings, HoldingDetail{
			Symbol:        sym,
			Quantity:      qty,
			AvgBuyPrice:   avg,
			MarkPrice:     mark,
			PositionValue: posVal,
			UnrealizedPnL: unreal,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out.InvestedValue = invested
	out.TotalValue = cash + invested
	out.TotalPnL = out.TotalValue - StartingCapitalINR
	if StartingCapitalINR > 0 {
		out.TotalReturnPct = out.TotalPnL / StartingCapitalINR * 100
	}
	return out, nil
}
