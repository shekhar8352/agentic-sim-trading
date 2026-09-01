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

func scanQuote(symbol string, date time.Time, open, high, low, close float64, vol *int64) models.Quote {
	q := models.Quote{Symbol: symbol, Date: date, Open: open, High: high, Low: low, Close: close}
	if vol != nil {
		q.Volume = *vol
	}
	return q
}

func scanIntradayQuote(symbol, interval string, ts time.Time, open, high, low, close float64, vol *int64) models.Quote {
	q := scanQuote(symbol, SessionCalendarUTC(ts), open, high, low, close, vol)
	q.Ts = ts.UTC()
	q.Interval = interval
	return q
}

// DistinctBars returns sorted bar timestamps for `interval` whose IST session date is in [start, end].
func (d *Data) DistinctBars(ctx context.Context, interval string, start, end time.Time) ([]time.Time, error) {
	if d.pool == nil {
		return nil, ErrNoDatabase
	}
	interval = NormalizeInterval(interval)
	if interval == Interval1d {
		return d.DistinctTradingDays(ctx, start, end)
	}
	from, until := InclusiveDateRangeUTC(start, end)
	rows, err := d.pool.Query(ctx, `
		SELECT DISTINCT ts
		FROM ohlcv_bars
		WHERE interval = $1 AND ts >= $2 AND ts < $3
		ORDER BY ts ASC
	`, interval, from, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []time.Time
	for rows.Next() {
		var ts time.Time
		if err := rows.Scan(&ts); err != nil {
			return nil, err
		}
		out = append(out, ts.UTC())
	}
	return out, rows.Err()
}

// NextBar returns the earliest bar timestamp strictly after `ts` for the interval.
func (d *Data) NextBar(ctx context.Context, interval string, ts time.Time) (time.Time, error) {
	if d.pool == nil {
		return time.Time{}, ErrNoDatabase
	}
	interval = NormalizeInterval(interval)
	if interval == Interval1d {
		return d.NextTradingDay(ctx, ts)
	}
	row := d.pool.QueryRow(ctx, `
		SELECT MIN(ts) FROM ohlcv_bars WHERE interval = $1 AND ts > $2
	`, interval, ts)
	var next time.Time
	if err := row.Scan(&next); err != nil {
		return time.Time{}, err
	}
	return next.UTC(), nil
}

// IntradayBarOnTs loads the exact bar for symbol/interval/ts.
func (d *Data) IntradayBarOnTs(ctx context.Context, symbol, interval string, ts time.Time) (models.Quote, error) {
	var q models.Quote
	q.Symbol = symbol
	if d.pool == nil {
		return q, ErrNoDatabase
	}
	interval = NormalizeInterval(interval)
	row := d.pool.QueryRow(ctx, `
		SELECT ts, open, high, low, close, volume
		FROM ohlcv_bars
		WHERE symbol = $1 AND interval = $2 AND ts = $3
	`, symbol, interval, ts)
	var barTs time.Time
	var vol *int64
	var open, high, low, close float64
	err := row.Scan(&barTs, &open, &high, &low, &close, &vol)
	if err != nil {
		return q, err
	}
	return scanIntradayQuote(symbol, interval, barTs, open, high, low, close, vol), nil
}

// IntradayBarAtOrBefore returns the latest bar with ts <= asOf.
func (d *Data) IntradayBarAtOrBefore(ctx context.Context, symbol, interval string, asOf time.Time) (models.Quote, error) {
	var q models.Quote
	q.Symbol = symbol
	if d.pool == nil {
		return q, ErrNoDatabase
	}
	interval = NormalizeInterval(interval)
	row := d.pool.QueryRow(ctx, `
		SELECT ts, open, high, low, close, volume
		FROM ohlcv_bars
		WHERE symbol = $1 AND interval = $2 AND ts <= $3
		ORDER BY ts DESC
		LIMIT 1
	`, symbol, interval, asOf)
	var barTs time.Time
	var vol *int64
	var open, high, low, close float64
	err := row.Scan(&barTs, &open, &high, &low, &close, &vol)
	if err != nil {
		return q, err
	}
	return scanIntradayQuote(symbol, interval, barTs, open, high, low, close, vol), nil
}

// IntradayBarsForSymbol returns up to limit bars with ts <= asOf, oldest first.
func (d *Data) IntradayBarsForSymbol(ctx context.Context, symbol, interval string, asOf time.Time, limit int) ([]models.Quote, error) {
	if d.pool == nil {
		return nil, ErrNoDatabase
	}
	interval = NormalizeInterval(interval)
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.pool.Query(ctx, `
		SELECT ts, open, high, low, close, volume
		FROM ohlcv_bars
		WHERE symbol = $1 AND interval = $2 AND ts <= $3
		ORDER BY ts DESC
		LIMIT $4
	`, symbol, interval, asOf, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Quote
	for rows.Next() {
		var barTs time.Time
		var vol *int64
		var open, high, low, close float64
		if err := rows.Scan(&barTs, &open, &high, &low, &close, &vol); err != nil {
			return nil, err
		}
		out = append(out, scanIntradayQuote(symbol, interval, barTs, open, high, low, close, vol))
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

// SessionBarsUpTo returns bars for symbol on the IST session of sessionTs with ts <= asOf.
func (d *Data) SessionBarsUpTo(ctx context.Context, symbol, interval string, sessionTs, asOf time.Time) ([]models.Quote, error) {
	if d.pool == nil {
		return nil, ErrNoDatabase
	}
	interval = NormalizeInterval(interval)
	from, until := ISTDayRangeUTC(SessionCalendarUTC(sessionTs))
	rows, err := d.pool.Query(ctx, `
		SELECT ts, open, high, low, close, volume
		FROM ohlcv_bars
		WHERE symbol = $1 AND interval = $2 AND ts >= $3 AND ts < $4 AND ts <= $5
		ORDER BY ts ASC
	`, symbol, interval, from, until, asOf)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Quote
	for rows.Next() {
		var barTs time.Time
		var vol *int64
		var open, high, low, close float64
		if err := rows.Scan(&barTs, &open, &high, &low, &close, &vol); err != nil {
			return nil, err
		}
		out = append(out, scanIntradayQuote(symbol, interval, barTs, open, high, low, close, vol))
	}
	return out, rows.Err()
}
