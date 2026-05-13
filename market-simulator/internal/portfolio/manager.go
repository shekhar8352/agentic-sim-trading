package portfolio

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Manager coordinates holdings and cash with Postgres (expanded in Step 8/9).
type Manager struct {
	pool *pgxpool.Pool
}

func NewManager(pool *pgxpool.Pool) *Manager {
	return &Manager{pool: pool}
}

// PortfolioRow is a minimal read model for GET /portfolio/:agentId.
type PortfolioRow struct {
	SimulationID uuid.UUID `json:"simulation_id"`
	AgentID      uuid.UUID `json:"agent_id"`
	Cash         float64   `json:"cash"`
}

func (m *Manager) GetPortfolio(ctx context.Context, simulationID, agentID uuid.UUID) (*PortfolioRow, error) {
	if m.pool == nil {
		return &PortfolioRow{SimulationID: simulationID, AgentID: agentID}, nil
	}
	row := m.pool.QueryRow(ctx, `
		SELECT simulation_id, agent_id, cash::float8
		FROM portfolios
		WHERE simulation_id = $1 AND agent_id = $2
	`, simulationID, agentID)
	var p PortfolioRow
	if err := row.Scan(&p.SimulationID, &p.AgentID, &p.Cash); err != nil {
		return nil, err
	}
	return &p, nil
}
