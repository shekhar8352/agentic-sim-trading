package portfolio

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	StatusActive       = "active"
	StatusDisqualified = "disqualified"

	MissedTicksDQThreshold = 10
	Consecutive4xxDQ       = 500
)

// IsDisqualified reports whether the agent's portfolio is frozen for this simulation.
func (m *Manager) IsDisqualified(ctx context.Context, simulationID, agentID uuid.UUID) (bool, error) {
	if m.pool == nil {
		return false, errors.New("no database")
	}
	var status string
	err := m.pool.QueryRow(ctx, `
		SELECT COALESCE(status, 'active') FROM portfolios
		WHERE simulation_id = $1 AND agent_id = $2
	`, simulationID, agentID).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return status == StatusDisqualified, nil
}

func (m *Manager) AgentStatusTx(ctx context.Context, tx pgx.Tx, simulationID, agentID uuid.UUID) (string, error) {
	var status string
	err := tx.QueryRow(ctx, `
		SELECT COALESCE(status, 'active') FROM portfolios
		WHERE simulation_id = $1 AND agent_id = $2
	`, simulationID, agentID).Scan(&status)
	if err != nil {
		return "", err
	}
	return status, nil
}

func (m *Manager) SetDisqualifiedTx(ctx context.Context, tx pgx.Tx, simulationID, agentID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		UPDATE portfolios SET status = $3
		WHERE simulation_id = $1 AND agent_id = $2
	`, simulationID, agentID, StatusDisqualified)
	return err
}

func (m *Manager) SetDisqualified(ctx context.Context, simulationID, agentID uuid.UUID) error {
	if m.pool == nil {
		return errors.New("no database")
	}
	_, err := m.pool.Exec(ctx, `
		UPDATE portfolios SET status = $3
		WHERE simulation_id = $1 AND agent_id = $2
	`, simulationID, agentID, StatusDisqualified)
	return err
}

func (m *Manager) LastSnapshotTx(ctx context.Context, tx pgx.Tx, simulationID, agentID uuid.UUID) (total, cash, invested float64, ok bool, err error) {
	err = tx.QueryRow(ctx, `
		SELECT total_value::float8, cash::float8, invested_value::float8
		FROM portfolio_snapshots
		WHERE simulation_id = $1 AND agent_id = $2
		ORDER BY date DESC
		LIMIT 1
	`, simulationID, agentID).Scan(&total, &cash, &invested)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, 0, false, err
	}
	return total, cash, invested, true, nil
}

func (m *Manager) IncrementMissedTicks(ctx context.Context, simulationID, agentID uuid.UUID) (int, error) {
	if m.pool == nil {
		return 0, errors.New("no database")
	}
	var n int
	err := m.pool.QueryRow(ctx, `
		UPDATE portfolios
		SET consecutive_missed_ticks = consecutive_missed_ticks + 1
		WHERE simulation_id = $1 AND agent_id = $2 AND COALESCE(status, 'active') <> $3
		RETURNING consecutive_missed_ticks
	`, simulationID, agentID, StatusDisqualified).Scan(&n)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return n, err
}

func (m *Manager) ResetMissedTicks(ctx context.Context, simulationID, agentID uuid.UUID) error {
	if m.pool == nil {
		return nil
	}
	_, err := m.pool.Exec(ctx, `
		UPDATE portfolios SET consecutive_missed_ticks = 0
		WHERE simulation_id = $1 AND agent_id = $2
	`, simulationID, agentID)
	return err
}

func (m *Manager) Increment4xx(ctx context.Context, agentID uuid.UUID) (simID uuid.UUID, n int, err error) {
	if m.pool == nil {
		return uuid.Nil, 0, errors.New("no database")
	}
	err = m.pool.QueryRow(ctx, `
		UPDATE portfolios
		SET consecutive_4xx = consecutive_4xx + 1
		WHERE agent_id = $1 AND COALESCE(status, 'active') <> $2
		RETURNING simulation_id, consecutive_4xx
	`, agentID, StatusDisqualified).Scan(&simID, &n)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, 0, nil
	}
	return simID, n, err
}

func (m *Manager) Reset4xx(ctx context.Context, agentID uuid.UUID) error {
	if m.pool == nil {
		return nil
	}
	_, err := m.pool.Exec(ctx, `
		UPDATE portfolios SET consecutive_4xx = 0 WHERE agent_id = $1
	`, agentID)
	return err
}

func (m *Manager) SimulationIDForAgent(ctx context.Context, agentID uuid.UUID) (uuid.UUID, error) {
	if m.pool == nil {
		return uuid.Nil, errors.New("no database")
	}
	var id uuid.UUID
	err := m.pool.QueryRow(ctx, `
		SELECT simulation_id FROM portfolios WHERE agent_id = $1 ORDER BY id DESC LIMIT 1
	`, agentID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, nil
	}
	return id, err
}

func (m *Manager) ListAgentIDs(ctx context.Context, simulationID uuid.UUID) ([]uuid.UUID, error) {
	if m.pool == nil {
		return nil, errors.New("no database")
	}
	rows, err := m.pool.Query(ctx, `SELECT agent_id FROM portfolios WHERE simulation_id = $1`, simulationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (m *Manager) InsertDay0Snapshot(ctx context.Context, tx pgx.Tx, simulationID, agentID uuid.UUID, startDate time.Time) error {
	day0 := startDate.UTC().Truncate(24*time.Hour).AddDate(0, 0, -1)
	return m.UpsertSnapshotTx(ctx, tx, simulationID, agentID, day0, StartingCapitalINR, StartingCapitalINR, 0)
}
