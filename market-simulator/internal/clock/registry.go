package clock

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/agentic-sim-trading/market-simulator/internal/market"
	"github.com/agentic-sim-trading/market-simulator/internal/simulation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// DayMatcher runs persisted order matching for one simulation trading session (Step 8).
type DayMatcher interface {
	ProcessSimulationDay(ctx context.Context, simulationID uuid.UUID, tradeDate time.Time) error
}

// Registry owns in-memory SimClocks and persists `simulations.as_of_date` / status (Step 7).
type Registry struct {
	pool    *pgxpool.Pool
	redis   *redis.Client
	market  *market.Data
	matcher DayMatcher

	mu          sync.Mutex
	clocks      map[uuid.UUID]*SimClock
	orderCounts map[uuid.UUID]map[uuid.UUID]int // simulation -> agent -> submissions since last match
	lastAutoTick map[uuid.UUID]time.Time
}

// NewRegistry constructs an empty registry; clocks are loaded on Start.
func NewRegistry(pool *pgxpool.Pool, rdb *redis.Client, data *market.Data, matcher DayMatcher) *Registry {
	return &Registry{
		pool:        pool,
		redis:       rdb,
		market:      data,
		matcher:     matcher,
		clocks:       make(map[uuid.UUID]*SimClock),
		orderCounts:  make(map[uuid.UUID]map[uuid.UUID]int),
		lastAutoTick: make(map[uuid.UUID]time.Time),
	}
}

// TryAcceptOrderSubmission enforces up to 10 pending submissions per agent between processed ticks.
func (r *Registry) TryAcceptOrderSubmission(simulationID, agentID uuid.UUID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.orderCounts[simulationID] == nil {
		r.orderCounts[simulationID] = map[uuid.UUID]int{}
	}
	n := r.orderCounts[simulationID][agentID]
	if n >= 10 {
		return false
	}
	r.orderCounts[simulationID][agentID] = n + 1
	return true
}

// RollbackOrderSubmission reverses one submission slot after a failed InsertPending (best-effort).
func (r *Registry) RollbackOrderSubmission(simulationID, agentID uuid.UUID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m := r.orderCounts[simulationID]
	if m == nil {
		return
	}
	n := m[agentID]
	if n <= 1 {
		delete(m, agentID)
		if len(m) == 0 {
			delete(r.orderCounts, simulationID)
		}
		return
	}
	m[agentID] = n - 1
}

func (r *Registry) resetOrderSubmissionWindowLocked(simulationID uuid.UUID) {
	delete(r.orderCounts, simulationID)
}

func (r *Registry) publish(ctx context.Context, event string, payload map[string]any) error {
	if r.redis == nil {
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
	return r.redis.Publish(ctx, "sim.events", string(b)).Err()
}

// Start loads trading days, resumes from DB `as_of_date` when present, and sets status running.
func (r *Registry) Start(ctx context.Context, simulationID uuid.UUID) error {
	if r.pool == nil {
		return ErrDatabaseRequired
	}

	row, err := simulation.Get(ctx, r.pool, simulationID)
	if err != nil {
		return err
	}
	if row.Status == "completed" {
		return ErrClockCompleted
	}

	days, err := r.market.DistinctTradingDays(ctx, row.StartDate, row.EndDate)
	if err != nil {
		return err
	}
	if len(days) == 0 {
		return ErrNoTradingDays
	}

	idx := 0
	if row.AsOfDate != nil {
		idx = indexOfTradingDay(days, *row.AsOfDate)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.clocks[simulationID]; ok {
		if existing.Status == "paused" {
			existing.Status = "running"
			if err := simulation.UpdateClock(ctx, r.pool, simulationID, existing.CurrentDate, "running"); err != nil {
				return err
			}
			return r.publish(ctx, "sim.resumed", map[string]any{
				"simulation_id":      simulationID.String(),
				"date":               existing.CurrentDate.Format(time.DateOnly),
				"trading_day_index":  existing.CurrentIndex,
				"total_trading_days": len(existing.TradingDays),
			})
		}
		if existing.Status == "running" {
			return nil
		}
	}

	c := NewSimClockAt(simulationID, days, idx, "running", r.redis)
	r.clocks[simulationID] = c

	if err := simulation.UpdateClock(ctx, r.pool, simulationID, c.CurrentDate, "running"); err != nil {
		delete(r.clocks, simulationID)
		return err
	}

	return r.publish(ctx, "sim.started", map[string]any{
		"simulation_id":      simulationID.String(),
		"date":               c.CurrentDate.Format(time.DateOnly),
		"trading_day_index":  c.CurrentIndex,
		"total_trading_days": len(c.TradingDays),
	})
}

// Pause marks the clock paused and persists status (clock stays in memory).
func (r *Registry) Pause(ctx context.Context, simulationID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c := r.clocks[simulationID]
	if c == nil {
		return ErrNoActiveClock
	}
	if c.Status == "completed" {
		return ErrClockCompleted
	}
	c.Status = "paused"
	return simulation.UpdateClock(ctx, r.pool, simulationID, c.CurrentDate, "paused")
}

// Tick advances one trading day, persists state, publishes Redis, drops clock when completed.
func (r *Registry) Tick(ctx context.Context, simulationID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	c := r.clocks[simulationID]
	if c == nil {
		return ErrNoActiveClock
	}

	prevIdx := c.CurrentIndex
	prevDate := c.CurrentDate
	prevStatus := c.Status

	if err := c.Tick(ctx); err != nil {
		return err
	}

	advanced := c.CurrentIndex != prevIdx

	if advanced && r.matcher != nil {
		if err := r.matcher.ProcessSimulationDay(ctx, simulationID, c.CurrentDate); err != nil {
			c.Restore(prevIdx, prevDate, prevStatus)
			return err
		}
		r.resetOrderSubmissionWindowLocked(simulationID)
	}

	if err := simulation.UpdateClock(ctx, r.pool, simulationID, c.CurrentDate, c.Status); err != nil {
		return err
	}

	if err := r.maybeCheckpoint(ctx, simulationID, c, advanced); err != nil {
		return err
	}

	if c.Status == "completed" {
		delete(r.clocks, simulationID)
	}
	return nil
}

// ActiveClock returns the live clock if it is loaded (running or paused).
func (r *Registry) ActiveClock(simulationID uuid.UUID) (*SimClock, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clocks[simulationID]
	return c, ok
}

// AsOfDate returns the clock's current simulated calendar date when loaded.
func (r *Registry) AsOfDate(simulationID uuid.UUID) (time.Time, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.clocks[simulationID]
	if !ok || c.Status == "completed" {
		return time.Time{}, false
	}
	return c.CurrentDate, true
}

func indexOfTradingDay(days []time.Time, asOf time.Time) int {
	ad := asOf.UTC().Truncate(24 * time.Hour)
	for i, d := range days {
		di := d.UTC().Truncate(24 * time.Hour)
		if di.Equal(ad) {
			return i
		}
	}
	return 0
}
