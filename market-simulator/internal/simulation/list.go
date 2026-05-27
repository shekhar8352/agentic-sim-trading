package simulation

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Summary is a lightweight row for GET /api/v1/simulations (dashboard list).
type Summary struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	StartDate string          `json:"start_date"`
	EndDate   string          `json:"end_date"`
	AsOfDate  *string         `json:"as_of_date,omitempty"`
	Status    string          `json:"status"`
	Config    json.RawMessage `json:"config,omitempty"`
	CreatedAt string          `json:"created_at"`
}

// List returns simulations ordered by newest first.
func List(ctx context.Context, pool *pgxpool.Pool, limit int) ([]Summary, error) {
	if pool == nil {
		return nil, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := pool.Query(ctx, `
		SELECT id, name, start_date, end_date, as_of_date, status, COALESCE(config, '{}'::jsonb), created_at
		FROM simulations
		ORDER BY created_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Summary
	for rows.Next() {
		var s Summary
		var start, end time.Time
		var asOf *time.Time
		var created time.Time
		if err := rows.Scan(&s.ID, &s.Name, &start, &end, &asOf, &s.Status, &s.Config, &created); err != nil {
			return nil, err
		}
		s.StartDate = start.Format(time.DateOnly)
		s.EndDate = end.Format(time.DateOnly)
		if asOf != nil {
			v := asOf.Format(time.DateOnly)
			s.AsOfDate = &v
		}
		s.CreatedAt = created.UTC().Format(time.RFC3339)
		out = append(out, s)
	}
	return out, rows.Err()
}
