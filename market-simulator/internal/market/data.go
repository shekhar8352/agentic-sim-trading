package market

import (
	"context"
	"time"

	"github.com/agentic-sim-trading/market-simulator/pkg/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Data loads OHLCV rows from Postgres.
type Data struct {
	pool *pgxpool.Pool
}

func NewData(pool *pgxpool.Pool) *Data {
	return &Data{pool: pool}
}

// LatestBar returns the most recent candle for a symbol (by date).
func (d *Data) LatestBar(ctx context.Context, symbol string) (models.Quote, error) {
	var q models.Quote
	q.Symbol = symbol
	if d.pool == nil {
		return q, ErrNoDatabase
	}
	row := d.pool.QueryRow(ctx, `
		SELECT date, open, high, low, close, volume
		FROM ohlcv
		WHERE symbol = $1
		ORDER BY date DESC
		LIMIT 1
	`, symbol)
	var vol *int64
	err := row.Scan(&q.Date, &q.Open, &q.High, &q.Low, &q.Close, &vol)
	if vol != nil {
		q.Volume = *vol
	}
	return q, err
}

// BarsForSymbol returns up to limit rows ending at asOf (inclusive), oldest first.
func (d *Data) BarsForSymbol(ctx context.Context, symbol string, asOf time.Time, limit int) ([]models.Quote, error) {
	if d.pool == nil {
		return nil, ErrNoDatabase
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.pool.Query(ctx, `
		SELECT date, open, high, low, close, volume
		FROM ohlcv
		WHERE symbol = $1 AND date <= $2::date
		ORDER BY date DESC
		LIMIT $3
	`, symbol, asOf, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Quote
	for rows.Next() {
		var q models.Quote
		q.Symbol = symbol
		var vol *int64
		if err := rows.Scan(&q.Date, &q.Open, &q.High, &q.Low, &q.Close, &vol); err != nil {
			return nil, err
		}
		if vol != nil {
			q.Volume = *vol
		}
		out = append(out, q)
	}
	// reverse to chronological order
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

// DistinctTradingDays returns sorted calendar dates present in OHLCV between start and end (inclusive).
func (d *Data) DistinctTradingDays(ctx context.Context, start, end time.Time) ([]time.Time, error) {
	if d.pool == nil {
		return nil, ErrNoDatabase
	}
	rows, err := d.pool.Query(ctx, `
		SELECT DISTINCT date
		FROM ohlcv
		WHERE date >= $1::date AND date <= $2::date
		ORDER BY date ASC
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var days []time.Time
	for rows.Next() {
		var t time.Time
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		days = append(days, t.UTC())
	}
	return days, rows.Err()
}

// NextTradingDay returns the earliest global calendar date in ohlcv strictly after `day`.
func (d *Data) NextTradingDay(ctx context.Context, day time.Time) (time.Time, error) {
	if d.pool == nil {
		return time.Time{}, ErrNoDatabase
	}
	row := d.pool.QueryRow(ctx, `
		SELECT MIN(date) FROM ohlcv WHERE date > $1::date
	`, day)
	var next time.Time
	if err := row.Scan(&next); err != nil {
		return time.Time{}, err
	}
	return next.UTC(), nil
}

// BarOnDate loads OHLCV for symbol on an exact trading date.
func (d *Data) BarOnDate(ctx context.Context, symbol string, day time.Time) (models.Quote, error) {
	var q models.Quote
	q.Symbol = symbol
	if d.pool == nil {
		return q, ErrNoDatabase
	}
	row := d.pool.QueryRow(ctx, `
		SELECT date, open, high, low, close, volume
		FROM ohlcv WHERE symbol = $1 AND date = $2::date
	`, symbol, day)
	var vol *int64
	err := row.Scan(&q.Date, &q.Open, &q.High, &q.Low, &q.Close, &vol)
	if vol != nil {
		q.Volume = *vol
	}
	return q, err
}

// PrevTradingDayBar returns the latest OHLCV row strictly before `day` for the symbol.
func (d *Data) PrevTradingDayBar(ctx context.Context, symbol string, day time.Time) (models.Quote, error) {
	var q models.Quote
	q.Symbol = symbol
	if d.pool == nil {
		return q, ErrNoDatabase
	}
	row := d.pool.QueryRow(ctx, `
		SELECT date, open, high, low, close, volume
		FROM ohlcv WHERE symbol = $1 AND date < $2::date
		ORDER BY date DESC LIMIT 1
	`, symbol, day)
	var vol *int64
	err := row.Scan(&q.Date, &q.Open, &q.High, &q.Low, &q.Close, &vol)
	if vol != nil {
		q.Volume = *vol
	}
	return q, err
}
