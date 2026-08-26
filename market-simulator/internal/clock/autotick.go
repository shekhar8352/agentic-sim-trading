package clock

import (
	"context"
	"log"
	"time"

	"github.com/agentic-sim-trading/market-simulator/internal/simulation"
	"github.com/google/uuid"
)

// RunAutoTicker advances running simulations on a wall-clock interval (demo mode).
func (r *Registry) RunAutoTicker(ctx context.Context) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.closeExpiredWindows(ctx)
			r.runDueAutoTicks(ctx)
		}
	}
}

func (r *Registry) closeExpiredWindows(ctx context.Context) {
	for _, id := range r.windowsDue(time.Now()) {
		if err := r.CloseWindowAndMatch(ctx, id); err != nil {
			log.Printf("close-window simulation_id=%s err=%v", id, err)
		}
	}
}

func (r *Registry) runDueAutoTicks(ctx context.Context) {
	if r.pool == nil {
		return
	}

	r.mu.Lock()
	ids := make([]uuid.UUID, 0, len(r.clocks))
	for id, c := range r.clocks {
		if c.Status == "running" && !r.windows[id].Open {
			ids = append(ids, id)
		}
	}
	r.mu.Unlock()

	now := time.Now()
	for _, id := range ids {
		row, err := simulation.Get(ctx, r.pool, id)
		if err != nil {
			continue
		}
		if !simulation.AutoTickEnabled(row.Config) || simulation.AwaitingProceed(row.Config) {
			continue
		}
		interval := time.Duration(simulation.EffectiveTickInterval(row.Config) * float64(time.Second))

		r.mu.Lock()
		last, ok := r.lastAutoTick[id]
		if !ok {
			r.lastAutoTick[id] = now
			r.mu.Unlock()
			continue
		}
		r.mu.Unlock()

		if now.Sub(last) < interval {
			continue
		}

		if err := r.Tick(ctx, id); err != nil {
			log.Printf("auto-tick simulation_id=%s err=%v", id, err)
			continue
		}

		r.mu.Lock()
		r.lastAutoTick[id] = now
		r.mu.Unlock()
	}
}

// Proceed clears a checkpoint gate and resumes the simulation clock.
func (r *Registry) Proceed(ctx context.Context, simulationID uuid.UUID) error {
	if r.pool == nil {
		return ErrDatabaseRequired
	}
	row, err := simulation.Get(ctx, r.pool, simulationID)
	if err != nil {
		return err
	}
	if !simulation.AwaitingProceed(row.Config) {
		return ErrNotAwaitingProceed
	}
	if err := simulation.ResetCheckpointWindow(ctx, r.pool, simulationID); err != nil {
		return err
	}
	return r.Start(ctx, simulationID)
}

func (r *Registry) maybeCheckpoint(ctx context.Context, simulationID uuid.UUID, c *SimClock, advanced bool) error {
	if !advanced || c.Status != "running" || r.pool == nil {
		return nil
	}
	newDays, interval, err := simulation.RecordTickForCheckpoint(ctx, r.pool, simulationID)
	if err != nil {
		return err
	}
	if newDays < interval || c.CurrentIndex >= len(c.TradingDays)-1 {
		return nil
	}

	c.Status = "paused"
	if err := simulation.SetAwaitingProceed(ctx, r.pool, simulationID, true); err != nil {
		return err
	}
	if err := simulation.UpdateClock(ctx, r.pool, simulationID, c.CurrentDate, "paused"); err != nil {
		return err
	}
	r.mu.Lock()
	w := r.windows[simulationID]
	w.Open = false
	r.windows[simulationID] = w
	r.mu.Unlock()
	return r.publish(ctx, "sim.checkpoint", map[string]any{
		"simulation_id":            simulationID.String(),
		"date":                     c.CurrentDate.Format(time.DateOnly),
		"days_since_checkpoint":    newDays,
		"checkpoint_interval_days": interval,
		"trading_day_index":        c.CurrentIndex,
		"total_trading_days":       len(c.TradingDays),
	})
}
