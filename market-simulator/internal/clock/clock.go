package clock

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// SimClock advances discrete trading days and publishes Redis notifications for the orchestrator.
type SimClock struct {
	SimulationID uuid.UUID
	CurrentDate  time.Time
	TradingDays  []time.Time
	CurrentIndex int
	Status       string
	redis        *redis.Client
}

func NewSimClock(simulationID uuid.UUID, tradingDays []time.Time, rdb *redis.Client) *SimClock {
	return NewSimClockAt(simulationID, tradingDays, 0, "paused", rdb)
}

// NewSimClockAt positions the clock on tradingDays[currentIndex] with an explicit lifecycle status.
func NewSimClockAt(simulationID uuid.UUID, tradingDays []time.Time, currentIndex int, status string, rdb *redis.Client) *SimClock {
	c := &SimClock{
		SimulationID: simulationID,
		TradingDays:  tradingDays,
		CurrentIndex: currentIndex,
		Status:       status,
		redis:        rdb,
	}
	if len(tradingDays) > 0 && currentIndex >= 0 && currentIndex < len(tradingDays) {
		c.CurrentDate = tradingDays[currentIndex]
	}
	return c
}

// Tick moves to the next trading day and emits sim.tick (or sim.completed), matching roadmap Step 7.
func (c *SimClock) Tick(ctx context.Context) error {
	if c.Status != "running" {
		return ErrClockNotRunning
	}
	if len(c.TradingDays) == 0 {
		c.Status = "completed"
		return c.publishEvent(ctx, "sim.completed", map[string]any{
			"simulation_id": c.SimulationID.String(),
		})
	}
	if c.CurrentIndex >= len(c.TradingDays)-1 {
		c.Status = "completed"
		return c.publishEvent(ctx, "sim.completed", map[string]any{
			"simulation_id": c.SimulationID.String(),
			"date":          c.CurrentDate.Format(time.DateOnly),
		})
	}
	c.CurrentIndex++
	c.CurrentDate = c.TradingDays[c.CurrentIndex]
	return c.publishEvent(ctx, "sim.tick", map[string]any{
		"simulation_id": c.SimulationID.String(),
		"date":          c.CurrentDate.Format(time.DateOnly),
	})
}

func (c *SimClock) publishEvent(ctx context.Context, event string, payload map[string]any) error {
	if c.redis == nil {
		return nil
	}
	body := map[string]any{"event": event}
	for k, v := range payload {
		body[k] = v
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	return c.redis.Publish(ctx, "sim.events", string(b)).Err()
}
