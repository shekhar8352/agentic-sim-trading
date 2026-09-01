package clock

import (
	"context"
	"encoding/json"
	"time"

	"github.com/agentic-sim-trading/market-simulator/internal/market"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// SimClock advances discrete trading days or hourly bars and publishes Redis notifications.
type SimClock struct {
	SimulationID uuid.UUID
	CurrentDate  time.Time
	TradingDays  []time.Time
	CurrentIndex int
	Status       string
	Interval     string
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
		Interval:     market.Interval1d,
		redis:        rdb,
	}
	if len(tradingDays) > 0 && currentIndex >= 0 && currentIndex < len(tradingDays) {
		c.CurrentDate = tradingDays[currentIndex]
	}
	return c
}

func (c *SimClock) Hourly() bool {
	return market.NormalizeInterval(c.Interval) == market.Interval60m
}

func (c *SimClock) SessionCalendar() time.Time {
	if c.Hourly() {
		return market.SessionCalendarUTC(c.CurrentDate)
	}
	return c.CurrentDate.UTC().Truncate(24 * time.Hour)
}

func (c *SimClock) SessionBarIndex() int {
	if !c.Hourly() || len(c.TradingDays) == 0 {
		return 1
	}
	n := 0
	for i, ts := range c.TradingDays {
		if !market.SameSession(ts, c.CurrentDate) {
			continue
		}
		n++
		if i == c.CurrentIndex {
			return n
		}
	}
	return n
}

func (c *SimClock) SessionBarCount() int {
	if !c.Hourly() {
		return 1
	}
	n := 0
	for _, ts := range c.TradingDays {
		if market.SameSession(ts, c.CurrentDate) {
			n++
		}
	}
	return n
}

func (c *SimClock) IsLastBarOfSession() bool {
	if !c.Hourly() {
		return true
	}
	if c.CurrentIndex >= len(c.TradingDays)-1 {
		return true
	}
	return !market.SameSession(c.CurrentDate, c.TradingDays[c.CurrentIndex+1])
}

func (c *SimClock) tickPayload() map[string]any {
	p := map[string]any{
		"simulation_id":      c.SimulationID.String(),
		"date":               c.SessionCalendar().Format(time.DateOnly),
		"interval":           market.NormalizeInterval(c.Interval),
		"trading_day_index":  c.CurrentIndex,
		"total_trading_days": len(c.TradingDays),
	}
	if c.Hourly() {
		p["bar_ts"] = c.CurrentDate.UTC().Format(time.RFC3339)
		p["session_bar"] = c.SessionBarIndex()
		p["session_bars"] = c.SessionBarCount()
	}
	return p
}

// Tick moves to the next trading day/bar and emits sim.tick (or sim.completed).
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
		done := map[string]any{
			"simulation_id": c.SimulationID.String(),
			"date":          c.SessionCalendar().Format(time.DateOnly),
		}
		if c.Hourly() {
			done["bar_ts"] = c.CurrentDate.UTC().Format(time.RFC3339)
		}
		return c.publishEvent(ctx, "sim.completed", done)
	}
	c.CurrentIndex++
	c.CurrentDate = c.TradingDays[c.CurrentIndex]
	return c.publishEvent(ctx, "sim.tick", c.tickPayload())
}

// Restore rolls back in-memory clock state after a failed post-tick side effect (e.g. matching).
func (c *SimClock) Restore(index int, date time.Time, status string) {
	c.CurrentIndex = index
	c.CurrentDate = date
	c.Status = status
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
