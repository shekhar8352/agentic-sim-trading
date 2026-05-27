package dashboard

import (
	"context"
	"time"

	"github.com/agentic-sim-trading/market-simulator/internal/portfolio"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EquityPoint is one EOD portfolio value for charting.
type EquityPoint struct {
	Date        string  `json:"date"`
	TotalValue  float64 `json:"total_value"`
	Cash        float64 `json:"cash"`
	Invested    float64 `json:"invested_value"`
	ReturnPct   float64 `json:"return_pct"`
}

// AgentEquityCurve groups snapshot series for one agent.
type AgentEquityCurve struct {
	AgentID   uuid.UUID     `json:"agent_id"`
	AgentName string        `json:"agent_name"`
	Points    []EquityPoint `json:"points"`
}

// EquityCurves loads portfolio_snapshots for all agents in a simulation.
func EquityCurves(ctx context.Context, pool *pgxpool.Pool, simulationID uuid.UUID) ([]AgentEquityCurve, error) {
	if pool == nil {
		return nil, nil
	}
	rows, err := pool.Query(ctx, `
		SELECT ps.agent_id, COALESCE(a.name, ''), ps.date, ps.total_value::float8, ps.cash::float8, ps.invested_value::float8
		FROM portfolio_snapshots ps
		LEFT JOIN agents a ON a.id = ps.agent_id
		WHERE ps.simulation_id = $1
		ORDER BY ps.agent_id ASC, ps.date ASC
	`, simulationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byAgent := make(map[uuid.UUID]*AgentEquityCurve)
	order := make([]uuid.UUID, 0)

	for rows.Next() {
		var aid uuid.UUID
		var name string
		var d time.Time
		var tv, cash, inv float64
		if err := rows.Scan(&aid, &name, &d, &tv, &cash, &inv); err != nil {
			return nil, err
		}
		curve, ok := byAgent[aid]
		if !ok {
			curve = &AgentEquityCurve{AgentID: aid, AgentName: name, Points: nil}
			byAgent[aid] = curve
			order = append(order, aid)
		}
		retPct := (tv - portfolio.StartingCapitalINR) / portfolio.StartingCapitalINR * 100
		if portfolio.StartingCapitalINR == 0 {
			retPct = 0
		}
		curve.Points = append(curve.Points, EquityPoint{
			Date:       d.UTC().Format(time.DateOnly),
			TotalValue: tv,
			Cash:       cash,
			Invested:   inv,
			ReturnPct:  retPct,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]AgentEquityCurve, 0, len(order))
	for _, id := range order {
		out = append(out, *byAgent[id])
	}
	return out, nil
}

// OrderRow is a recent order for the live feed.
type OrderRow struct {
	ID           string  `json:"id"`
	AgentID      string  `json:"agent_id"`
	AgentName    string  `json:"agent_name"`
	Symbol       string  `json:"symbol"`
	Side         string  `json:"side"`
	Quantity     int     `json:"quantity"`
	Status       string  `json:"status"`
	FilledPrice  *float64 `json:"filled_price,omitempty"`
	MatchOnDate  *string `json:"match_on_date,omitempty"`
	RejectReason *string `json:"rejection_reason,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

// RecentOrders returns the latest orders across all agents in a simulation.
func RecentOrders(ctx context.Context, pool *pgxpool.Pool, simulationID uuid.UUID, limit int) ([]OrderRow, error) {
	if pool == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := pool.Query(ctx, `
		SELECT o.id, o.agent_id, COALESCE(a.name, ''), o.symbol, o.side, o.quantity, o.status,
		       o.filled_price, o.match_on_date, o.rejection_reason, o.created_at
		FROM orders o
		LEFT JOIN agents a ON a.id = o.agent_id
		WHERE o.simulation_id = $1
		ORDER BY o.created_at DESC
		LIMIT $2
	`, simulationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []OrderRow
	for rows.Next() {
		var r OrderRow
		var aid uuid.UUID
		var oid uuid.UUID
		var filled *float64
		var matchOn *time.Time
		var reject *string
		var created time.Time
		if err := rows.Scan(&oid, &aid, &r.AgentName, &r.Symbol, &r.Side, &r.Quantity, &r.Status,
			&filled, &matchOn, &reject, &created); err != nil {
			return nil, err
		}
		r.ID = oid.String()
		r.AgentID = aid.String()
		r.FilledPrice = filled
		if matchOn != nil {
			v := matchOn.Format(time.DateOnly)
			r.MatchOnDate = &v
		}
		r.RejectReason = reject
		r.CreatedAt = created.UTC().Format(time.RFC3339)
		out = append(out, r)
	}
	return out, rows.Err()
}

// AgentSummary is one agent registered in a simulation.
type AgentSummary struct {
	AgentID   uuid.UUID `json:"agent_id"`
	AgentName string    `json:"agent_name"`
	Model     string    `json:"model,omitempty"`
}

// ListAgents returns agents with portfolios in the simulation.
func ListAgents(ctx context.Context, pool *pgxpool.Pool, simulationID uuid.UUID) ([]AgentSummary, error) {
	if pool == nil {
		return nil, nil
	}
	rows, err := pool.Query(ctx, `
		SELECT a.id, COALESCE(a.name, ''), COALESCE(a.model, '')
		FROM agents a
		INNER JOIN portfolios p ON p.agent_id = a.id AND p.simulation_id = $1
		ORDER BY a.name ASC, a.id ASC
	`, simulationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AgentSummary
	for rows.Next() {
		var s AgentSummary
		if err := rows.Scan(&s.AgentID, &s.AgentName, &s.Model); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// AgentMetrics extends leaderboard stats with win-rate proxy.
type AgentMetrics struct {
	AgentID            uuid.UUID `json:"agent_id"`
	AgentName          string    `json:"agent_name"`
	TotalReturnPct     float64   `json:"total_return_pct"`
	SharpeRatio        float64   `json:"sharpe_ratio"`
	MaxDrawdownPct     float64   `json:"max_drawdown_pct"`
	WinRatePct         float64   `json:"win_rate_pct"`
	AvgHoldingDays     float64   `json:"avg_holding_days"`
	TotalFilledOrders  int       `json:"total_filled_orders"`
	TotalRejectedOrders int      `json:"total_rejected_orders"`
}

// BuildAgentMetrics computes performance metrics for one agent.
func BuildAgentMetrics(ctx context.Context, pool *pgxpool.Pool, simulationID, agentID uuid.UUID) (*AgentMetrics, error) {
	if pool == nil {
		return nil, nil
	}

	var name string
	if err := pool.QueryRow(ctx, `SELECT COALESCE(name, '') FROM agents WHERE id = $1`, agentID).Scan(&name); err != nil {
		return nil, err
	}

	snapRows, err := pool.Query(ctx, `
		SELECT date, total_value::float8 FROM portfolio_snapshots
		WHERE simulation_id = $1 AND agent_id = $2 ORDER BY date ASC
	`, simulationID, agentID)
	if err != nil {
		return nil, err
	}
	defer snapRows.Close()

	var values []float64
	for snapRows.Next() {
		var d time.Time
		var tv float64
		if err := snapRows.Scan(&d, &tv); err != nil {
			return nil, err
		}
		values = append(values, tv)
	}
	if err := snapRows.Err(); err != nil {
		return nil, err
	}

	es := portfolio.EquitySeries{Values: values}
	sharpe := portfolio.SharpeRatio(es, 0)
	mdd := portfolio.MaxDrawdown(es) * 100

	retPct := 0.0
	if len(values) > 0 && portfolio.StartingCapitalINR > 0 {
		retPct = (values[len(values)-1] - portfolio.StartingCapitalINR) / portfolio.StartingCapitalINR * 100
	}

	positiveDays, totalDays := 0, 0
	for i := 1; i < len(values); i++ {
		if values[i-1] == 0 {
			continue
		}
		totalDays++
		if values[i] > values[i-1] {
			positiveDays++
		}
	}
	winRate := 0.0
	if totalDays > 0 {
		winRate = float64(positiveDays) / float64(totalDays) * 100
	}

	var filled, rejected int
	if err := pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'filled'),
			COUNT(*) FILTER (WHERE status = 'rejected')
		FROM orders WHERE simulation_id = $1 AND agent_id = $2
	`, simulationID, agentID).Scan(&filled, &rejected); err != nil {
		return nil, err
	}

	avgHold := 0.0
	if filled > 0 {
		avgHold = float64(len(values)) / float64(filled)
		if avgHold < 1 {
			avgHold = 1
		}
	}

	return &AgentMetrics{
		AgentID:             agentID,
		AgentName:           name,
		TotalReturnPct:      retPct,
		SharpeRatio:         sharpe,
		MaxDrawdownPct:      mdd,
		WinRatePct:          winRate,
		AvgHoldingDays:      avgHold,
		TotalFilledOrders:   filled,
		TotalRejectedOrders: rejected,
	}, nil
}
