package leaderboard

import (
	"context"
	"sort"
	"time"

	"github.com/agentic-sim-trading/market-simulator/internal/portfolio"
	"github.com/agentic-sim-trading/market-simulator/internal/universe"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Entry is one ranked row for GET /leaderboard/:simId (Step 9).
type Entry struct {
	Rank           int                `json:"rank"`
	AgentID        uuid.UUID          `json:"agent_id"`
	AgentName      string             `json:"agent_name"`
	Disqualified   bool               `json:"disqualified"`
	SnapshotDate   string             `json:"snapshot_date,omitempty"`
	TotalValue     float64            `json:"total_value"`
	Cash           float64            `json:"cash"`
	InvestedValue  float64            `json:"invested_value"`
	TotalReturnPct float64            `json:"total_return_pct"`
	SharpeRatio    float64            `json:"sharpe_ratio"`
	MaxDrawdownPct float64            `json:"max_drawdown_pct"`
	WinRatePct     float64            `json:"win_rate_pct"`
	AvgHoldingDays float64            `json:"avg_holding_days"`
	TurnoverRate   float64            `json:"turnover_rate"`
	SectorExposure map[string]float64 `json:"sector_exposure,omitempty"`
}

// Build returns agents ranked by latest EOD total_value with risk metrics from snapshot series.
func Build(ctx context.Context, pool *pgxpool.Pool, simulationID uuid.UUID) ([]Entry, error) {
	if pool == nil {
		return nil, nil
	}

	rows, err := pool.Query(ctx, `
		SELECT agent_id, date, total_value::float8, cash::float8, invested_value::float8
		FROM portfolio_snapshots
		WHERE simulation_id = $1
		ORDER BY agent_id ASC, date ASC
	`, simulationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type seriesRow struct {
		date                       time.Time
		totalValue, cash, invested float64
	}
	byAgent := make(map[uuid.UUID][]seriesRow)

	for rows.Next() {
		var aid uuid.UUID
		var d time.Time
		var tv, c, inv float64
		if err := rows.Scan(&aid, &d, &tv, &c, &inv); err != nil {
			return nil, err
		}
		byAgent[aid] = append(byAgent[aid], seriesRow{date: d.UTC(), totalValue: tv, cash: c, invested: inv})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(byAgent) == 0 {
		return []Entry{}, nil
	}

	agentIDs := make([]uuid.UUID, 0, len(byAgent))
	for id := range byAgent {
		agentIDs = append(agentIDs, id)
	}
	sort.Slice(agentIDs, func(i, j int) bool { return agentIDs[i].String() < agentIDs[j].String() })

	names, err := loadAgentNames(ctx, pool, simulationID)
	if err != nil {
		return nil, err
	}
	dq, err := loadDQ(ctx, pool, simulationID)
	if err != nil {
		return nil, err
	}
	tradeStats, err := loadTradeStats(ctx, pool, simulationID)
	if err != nil {
		return nil, err
	}
	sectors, err := loadSectorExposure(ctx, pool, simulationID)
	if err != nil {
		return nil, err
	}

	type scored struct {
		entry  Entry
		lastTV float64
	}
	var scoredRows []scored

	for _, aid := range agentIDs {
		srs := byAgent[aid]
		if len(srs) == 0 {
			continue
		}
		last := srs[len(srs)-1]
		values := make([]float64, len(srs))
		for i, r := range srs {
			values[i] = r.totalValue
		}
		es := portfolio.EquitySeries{Values: values}
		sharpe := portfolio.SharpeRatio(es, 0)
		mdd := portfolio.MaxDrawdown(es)

		retPct := (last.totalValue - portfolio.StartingCapitalINR) / portfolio.StartingCapitalINR * 100
		if portfolio.StartingCapitalINR == 0 {
			retPct = 0
		}

		name := names[aid]
		if dq[aid] {
			name = "[DQ] " + names[aid]
		}

		ts := tradeStats[aid]
		scoredRows = append(scoredRows, scored{
			lastTV: last.totalValue,
			entry: Entry{
				Rank:           0,
				AgentID:        aid,
				AgentName:      name,
				Disqualified:   dq[aid],
				SnapshotDate:   last.date.UTC().Format(time.DateOnly),
				TotalValue:     last.totalValue,
				Cash:           last.cash,
				InvestedValue:  last.invested,
				TotalReturnPct: retPct,
				SharpeRatio:    sharpe,
				MaxDrawdownPct: mdd * 100,
				WinRatePct:     ts.WinRatePct,
				AvgHoldingDays: ts.AvgHoldingDays,
				TurnoverRate:   portfolio.TurnoverRate(ts.TotalTradeValue, es),
				SectorExposure: sectors[aid],
			},
		})
	}

	sort.Slice(scoredRows, func(i, j int) bool {
		if scoredRows[i].lastTV != scoredRows[j].lastTV {
			return scoredRows[i].lastTV > scoredRows[j].lastTV
		}
		return scoredRows[i].entry.SharpeRatio > scoredRows[j].entry.SharpeRatio
	})

	out := make([]Entry, len(scoredRows))
	for i := range scoredRows {
		scoredRows[i].entry.Rank = i + 1
		out[i] = scoredRows[i].entry
	}
	return out, nil
}

func loadAgentNames(ctx context.Context, pool *pgxpool.Pool, simulationID uuid.UUID) (map[uuid.UUID]string, error) {
	out := make(map[uuid.UUID]string)
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT a.id, COALESCE(a.name, '')
		FROM agents a
		INNER JOIN portfolio_snapshots ps ON ps.agent_id = a.id
		WHERE ps.simulation_id = $1
	`, simulationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[id] = name
	}
	return out, rows.Err()
}

func loadDQ(ctx context.Context, pool *pgxpool.Pool, simulationID uuid.UUID) (map[uuid.UUID]bool, error) {
	out := make(map[uuid.UUID]bool)
	rows, err := pool.Query(ctx, `
		SELECT agent_id, COALESCE(status, 'active')
		FROM portfolios WHERE simulation_id = $1
	`, simulationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var status string
		if err := rows.Scan(&id, &status); err != nil {
			return nil, err
		}
		out[id] = status == "disqualified"
	}
	return out, rows.Err()
}

func loadTradeStats(ctx context.Context, pool *pgxpool.Pool, simulationID uuid.UUID) (map[uuid.UUID]portfolio.ClosedTradeStats, error) {
	rows, err := pool.Query(ctx, `
		SELECT agent_id, symbol, side, quantity, COALESCE(filled_price, 0)::float8,
		       COALESCE(trade_value, 0)::float8, filled_at
		FROM orders
		WHERE simulation_id = $1 AND status = 'filled' AND filled_at IS NOT NULL
		ORDER BY agent_id, filled_at, created_at
	`, simulationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byAgent := map[uuid.UUID][]portfolio.FillLot{}
	for rows.Next() {
		var aid uuid.UUID
		var lot portfolio.FillLot
		var filledAt time.Time
		if err := rows.Scan(&aid, &lot.Symbol, &lot.Side, &lot.Quantity, &lot.Price, &lot.TradeVal, &filledAt); err != nil {
			return nil, err
		}
		lot.FilledAt = filledAt.UTC()
		byAgent[aid] = append(byAgent[aid], lot)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]portfolio.ClosedTradeStats, len(byAgent))
	for id, fills := range byAgent {
		out[id] = portfolio.AnalyzeClosedTrades(fills)
	}
	return out, nil
}

func loadSectorExposure(ctx context.Context, pool *pgxpool.Pool, simulationID uuid.UUID) (map[uuid.UUID]map[string]float64, error) {
	rows, err := pool.Query(ctx, `
		SELECT p.agent_id, h.symbol, h.quantity, h.avg_buy_price::float8, COALESCE(s.sector, '')
		FROM holdings h
		INNER JOIN portfolios p ON p.id = h.portfolio_id
		LEFT JOIN stocks s ON s.symbol = h.symbol
		WHERE p.simulation_id = $1
	`, simulationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	raw := map[uuid.UUID]map[string]float64{}
	for rows.Next() {
		var aid uuid.UUID
		var sym, sector string
		var qty int
		var avg float64
		if err := rows.Scan(&aid, &sym, &qty, &avg, &sector); err != nil {
			return nil, err
		}
		if sector == "" {
			sector = universe.SectorOf(sym)
		}
		if raw[aid] == nil {
			raw[aid] = map[string]float64{}
		}
		raw[aid][sector] += float64(qty) * avg
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]map[string]float64, len(raw))
	for id, m := range raw {
		out[id] = portfolio.SectorPercents(m)
	}
	return out, nil
}
