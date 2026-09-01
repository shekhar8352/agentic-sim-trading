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
		`CREATE TABLE IF NOT EXISTS ohlcv_bars (
			symbol   VARCHAR(20) NOT NULL REFERENCES stocks(symbol),
			ts       TIMESTAMPTZ NOT NULL,
			interval VARCHAR(8)  NOT NULL,
			open NUMERIC(12,4), high NUMERIC(12,4), low NUMERIC(12,4),
			close NUMERIC(12,4), volume BIGINT,
			UNIQUE (symbol, ts, interval)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ohlcv_bars_symbol_ts ON ohlcv_bars(symbol, interval, ts)`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS filled_at_ts TIMESTAMPTZ`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS filled_quantity INT`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS remaining_quantity INT`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS match_on_ts TIMESTAMPTZ`,
		`CREATE INDEX IF NOT EXISTS idx_orders_sim_match_ts_pending ON orders(simulation_id, match_on_ts, status) WHERE status = 'pending'`,
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s); err != nil {
			return fmt.Errorf("migrate: %s: %w", s, err)
		}
	}
	return nil
}
