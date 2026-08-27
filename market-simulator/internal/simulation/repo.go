package simulation

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("simulation not found")

// Row mirrors `simulations` for clock orchestration.
type Row struct {
	ID        uuid.UUID
	Name      string
	StartDate time.Time
	EndDate   time.Time
	AsOfDate  *time.Time
	Status    string
	Config    json.RawMessage
}

func mergeConfigDefaults(override json.RawMessage) json.RawMessage {
	base := configMap(DefaultConfig())
	over := configMap(override)
	for k, v := range over {
		base[k] = v
	}
	b, _ := json.Marshal(base)
	return b
}

func Create(ctx context.Context, pool *pgxpool.Pool, name string, startDate, endDate time.Time, config json.RawMessage) (uuid.UUID, error) {
	if pool == nil {
		return uuid.Nil, errors.New("database not configured")
	}
	config = mergeConfigDefaults(config)
	var id uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO simulations (name, start_date, end_date, as_of_date, status, config)
		VALUES ($1, $2::date, $3::date, NULL, 'paused', $4::jsonb)
		RETURNING id
	`, name, startDate, endDate, config).Scan(&id)
	return id, err
}

func Get(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (*Row, error) {
	if pool == nil {
		return nil, errors.New("database not configured")
	}
	row := pool.QueryRow(ctx, `
		SELECT id, name, start_date, end_date, as_of_date, status, COALESCE(config, '{}'::jsonb)
		FROM simulations WHERE id = $1
	`, id)
	var r Row
	var asOf *time.Time
	err := row.Scan(&r.ID, &r.Name, &r.StartDate, &r.EndDate, &asOf, &r.Status, &r.Config)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	r.AsOfDate = asOf
	return &r, nil
}

// UpdateClock persists virtual calendar position (roadmap Step 7).
func UpdateClock(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, asOf time.Time, status string) error {
	if pool == nil {
		return errors.New("database not configured")
	}
	tag, err := pool.Exec(ctx, `
		UPDATE simulations SET as_of_date = $2::date, status = $3 WHERE id = $1
	`, id, asOf, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// TickSpeedMultiplier reads config.tick_speed_multiplier (default 1). Used by ops UI and orchestrator (Step 9).
func TickSpeedMultiplier(cfg json.RawMessage) float64 {
	if len(cfg) == 0 {
		return 1
	}
	var m map[string]any
	if err := json.Unmarshal(cfg, &m); err != nil {
		return 1
	}
	x, ok := m["tick_speed_multiplier"].(float64)
	if !ok || x <= 0 {
		return 1
	}
	return x
}

// MergeConfig patches `simulations.config` with JSON merge (Step 9 sim speed).
func MergeConfig(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, fragment json.RawMessage) error {
	if pool == nil {
		return errors.New("database not configured")
	}
	if len(fragment) == 0 {
		fragment = json.RawMessage(`{}`)
	}
	tag, err := pool.Exec(ctx, `
		UPDATE simulations
		SET config = COALESCE(config, '{}'::jsonb) || $2::jsonb
		WHERE id = $1
	`, id, fragment)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListIDsByStatus returns simulation IDs with the given status (for clock resume).
func ListIDsByStatus(ctx context.Context, pool *pgxpool.Pool, status string) ([]uuid.UUID, error) {
	if pool == nil {
		return nil, errors.New("database not configured")
	}
	rows, err := pool.Query(ctx, `SELECT id FROM simulations WHERE status = $1`, status)
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
