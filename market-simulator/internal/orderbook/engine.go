package orderbook

import (
	"context"
	"time"

	"github.com/agentic-sim-trading/market-simulator/internal/portfolio"
)

// Engine applies fills at each simulated date (Step 8).
type Engine struct {
	Portfolio *portfolio.Manager
	Queue     *Queue
}

func NewEngine(pm *portfolio.Manager, q *Queue) *Engine {
	return &Engine{Portfolio: pm, Queue: q}
}

func (e *Engine) ProcessTick(ctx context.Context, date time.Time) error {
	_, err := e.Queue.Drain(ctx)
	if err != nil {
		return err
	}
	_ = date
	// Matching against OHLCV and portfolio checks lands here (Step 8).
	return nil
}
