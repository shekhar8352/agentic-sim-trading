package clock

import "errors"

var (
	ErrNoActiveClock    = errors.New("no active simulation clock in memory; call POST .../start")
	ErrClockNotRunning  = errors.New("simulation clock is not running")
	ErrClockCompleted   = errors.New("simulation is completed")
	ErrNoTradingDays    = errors.New("no OHLCV trading days in requested date range")
	ErrDatabaseRequired = errors.New("postgres pool required")
)
