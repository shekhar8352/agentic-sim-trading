package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate applies idempotent schema additions for existing databases (init.sql only runs on first volume create).
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return nil
	}
	stmts := []string{
		`ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS status VARCHAR(20) NOT NULL DEFAULT 'active'`,
		`ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS consecutive_missed_ticks INT NOT NULL DEFAULT 0`,
		`ALTER TABLE portfolios ADD COLUMN IF NOT EXISTS consecutive_4xx INT NOT NULL DEFAULT 0`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS fee_brokerage NUMERIC(15,4)`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS fee_stt NUMERIC(15,4)`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS fee_gst NUMERIC(15,4)`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS fee_exchange NUMERIC(15,4)`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS fee_sebi NUMERIC(15,4)`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS fee_stamp NUMERIC(15,4)`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			return fmt.Errorf("migrate: %s: %w", s, err)
		}
	}
	return nil
}
