package clock

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/agentic-sim-trading/market-simulator/internal/market"
	"github.com/agentic-sim-trading/market-simulator/internal/portfolio"
	"github.com/agentic-sim-trading/market-simulator/internal/simulation"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

const maxAPICallsPerTick = 100

// DayMatcher runs persisted order matching for one simulation trading session (Step 8).
type DayMatcher interface {
	ProcessSimulationDay(ctx context.Context, simulationID uuid.UUID, tradeDate time.Time) error
}

type submissionWindow struct {
	Open    bool
	Date    time.Time
	CloseAt time.Time
}

// Registry owns in-memory SimClocks and persists `simulations.as_of_date` / status (Step 7).
type Registry struct {
	pool    *pgxpool.Pool
	redis   *redis.Client
	market  *market.Data
	matcher DayMatcher
	pm      *portfolio.Manager

	mu           sync.Mutex
	clocks       map[uuid.UUID]*SimClock
	orderCounts  map[uuid.UUID]map[uuid.UUID]int
	apiCalls     map[uuid.UUID]map[uuid.UUID]int
	windows      map[uuid.UUID]submissionWindow
	lastMatched  map[uuid.UUID]time.Time
	lastAutoTick map[uuid.UUID]time.Time
}

// NewRegistry constructs an empty registry; clocks are loaded on Start.
func NewRegistry(pool *pgxpool.Pool, rdb *redis.Client, data *market.Data, matcher DayMatcher) *Registry {
	return &Registry{
		pool:         pool,
		redis:        rdb,
		market:       data,
		matcher:      matcher,
		clocks:       make(map[uuid.UUID]*SimClock),
		orderCounts:  make(map[uuid.UUID]map[uuid.UUID]int),
		apiCalls:     make(map[uuid.UUID]map[uuid.UUID]int),
		windows:      make(map[uuid.UUID]submissionWindow),
		lastMatched:  make(map[uuid.UUID]time.Time),
		lastAutoTick: make(map[uuid.UUID]time.Time),
	}
}

// SetPortfolioManager wires DQ / missed-tick tracking (optional).
func (r *Registry) SetPortfolioManager(pm *portfolio.Manager) {
	r.pm = pm
}

// ShouldAdvance reports whether Tick should move to the next day (current day already matched).
func ShouldAdvance(lastMatched, current time.Time) bool {
	if lastMatched.IsZero() {
		return false
	}
	cur := current.UTC().Truncate(24 * time.Hour)
	last := lastMatched.UTC().Truncate(24 * time.Hour)
	return !last.Before(cur)
}

// WindowOpen is true while agents may submit orders for the current tick date.
func (r *Registry) WindowOpen(simulationID uuid.UUID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.windows[simulationID].Open
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

// AllowAPICall enforces 100 agent API calls per simulation tick.
func (r *Registry) AllowAPICall(simulationID, agentID uuid.UUID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.apiCalls[simulationID] == nil {
		r.apiCalls[simulationID] = map[uuid.UUID]int{}
	}
	n := r.apiCalls[simulationID][agentID]
	if n >= maxAPICallsPerTick {
		return false
	}
	r.apiCalls[simulationID][agentID] = n + 1
	return true
}

func (r *Registry) resetTickCountersLocked(simulationID uuid.UUID) {
	delete(r.orderCounts, simulationID)
	delete(r.apiCalls, simulationID)
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

// ResumePersisted reloads clocks for simulations left `running` in Postgres.
func (r *Registry) ResumePersisted(ctx context.Context) error {
	if r.pool == nil {
		return nil
	}
	ids, err := simulation.ListIDsByStatus(ctx, r.pool, "running")
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := r.Start(ctx, id); err != nil {
			log.Printf("resume simulation_id=%s err=%v", id, err)
		}
	}
	return nil
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

	lastMatched := time.Time{}
	if s := simulation.LastMatchedDate(row.Config); s != "" {
		if t, err := time.Parse(time.DateOnly, s); err == nil {
			lastMatched = t.UTC().Truncate(24 * time.Hour)
		}
	}

	r.mu.Lock()
	if existing, ok := r.clocks[simulationID]; ok {
		if existing.Status == "paused" {
			existing.Status = "running"
			r.mu.Unlock()
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
			r.mu.Unlock()
			return nil
		}
	}

	c := NewSimClockAt(simulationID, days, idx, "running", r.redis)
	r.clocks[simulationID] = c
	r.lastMatched[simulationID] = lastMatched
	r.mu.Unlock()

	if err := simulation.UpdateClock(ctx, r.pool, simulationID, c.CurrentDate, "running"); err != nil {
		r.mu.Lock()
		delete(r.clocks, simulationID)
		r.mu.Unlock()
		return err
	}

	ev := "sim.started"
	if row.AsOfDate != nil && lastMatched.IsZero() == false {
		ev = "sim.resumed"
	}
	if err := r.publish(ctx, ev, map[string]any{
		"simulation_id":      simulationID.String(),
		"date":               c.CurrentDate.Format(time.DateOnly),
		"trading_day_index":  c.CurrentIndex,
		"total_trading_days": len(c.TradingDays),
	}); err != nil {
		return err
	}
	return nil
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
	w := r.windows[simulationID]
	w.Open = false
	r.windows[simulationID] = w
	return simulation.UpdateClock(ctx, r.pool, simulationID, c.CurrentDate, "paused")
}

// Tick advances one trading day (or processes the first unmatched day), opens the submission window, then matches.
func (r *Registry) Tick(ctx context.Context, simulationID uuid.UUID) error {
	if r.WindowOpen(simulationID) {
		if err := r.CloseWindowAndMatch(ctx, simulationID); err != nil {
			return err
		}
	}

	row, err := simulation.Get(ctx, r.pool, simulationID)
	if err != nil {
		return err
	}
	windowSecs := simulation.EffectiveOrderWindow(row.Config)

	r.mu.Lock()
	c := r.clocks[simulationID]
	if c == nil {
		r.mu.Unlock()
		return ErrNoActiveClock
	}
	if c.Status != "running" {
		r.mu.Unlock()
		return ErrClockNotRunning
	}

	prevIdx := c.CurrentIndex
	prevDate := c.CurrentDate
	prevStatus := c.Status
	last := r.lastMatched[simulationID]
	advance := ShouldAdvance(last, c.CurrentDate)

	if advance {
		if err := c.Tick(ctx); err != nil {
			r.mu.Unlock()
			return err
		}
		if c.Status == "completed" {
			r.mu.Unlock()
			if err := simulation.UpdateClock(ctx, r.pool, simulationID, prevDate, "completed"); err != nil {
				return err
			}
			r.mu.Lock()
			delete(r.clocks, simulationID)
			delete(r.windows, simulationID)
			r.mu.Unlock()
			return nil
		}
	} else {
		_ = r.publish(ctx, "sim.tick", map[string]any{
			"simulation_id": c.SimulationID.String(),
			"date":          c.CurrentDate.Format(time.DateOnly),
		})
	}

	advanced := c.CurrentIndex != prevIdx
	tradeDate := c.CurrentDate
	r.resetTickCountersLocked(simulationID)
	r.windows[simulationID] = submissionWindow{
		Open:    true,
		Date:    tradeDate,
		CloseAt: time.Now().Add(time.Duration(windowSecs * float64(time.Second))),
	}
	r.mu.Unlock()

	if err := simulation.UpdateClock(ctx, r.pool, simulationID, tradeDate, c.Status); err != nil {
		r.mu.Lock()
		c.Restore(prevIdx, prevDate, prevStatus)
		r.mu.Unlock()
		return err
	}

	if windowSecs <= 0 {
		if err := r.CloseWindowAndMatch(ctx, simulationID); err != nil {
			r.mu.Lock()
			c.Restore(prevIdx, prevDate, prevStatus)
			r.mu.Unlock()
			return err
		}
	}
	_ = advanced
	return nil
}

// CloseWindowAndMatch ends the submission window and runs EOD matching for the window date.
func (r *Registry) CloseWindowAndMatch(ctx context.Context, simulationID uuid.UUID) error {
	r.mu.Lock()
	w := r.windows[simulationID]
	c := r.clocks[simulationID]
	if !w.Open {
		r.mu.Unlock()
		return nil
	}
	w.Open = false
	r.windows[simulationID] = w
	tradeDate := w.Date
	if tradeDate.IsZero() && c != nil {
		tradeDate = c.CurrentDate
	}
	apiSnap := copyCallMap(r.apiCalls[simulationID])
	r.mu.Unlock()

	r.recordMissedTicks(ctx, simulationID, apiSnap)

	if r.matcher != nil {
		if err := r.matcher.ProcessSimulationDay(ctx, simulationID, tradeDate); err != nil {
			return err
		}
	}

	day := tradeDate.UTC().Truncate(24 * time.Hour)
	r.mu.Lock()
	r.lastMatched[simulationID] = day
	r.resetTickCountersLocked(simulationID)
	r.mu.Unlock()

	_ = simulation.MergeConfig(ctx, r.pool, simulationID, mustJSON(map[string]any{
		"last_matched_date": day.Format(time.DateOnly),
	}))

	r.mu.Lock()
	c = r.clocks[simulationID]
	r.mu.Unlock()
	if c != nil && c.Status == "running" {
		return r.maybeCheckpoint(ctx, simulationID, c, true)
	}
	return nil
}

func copyCallMap(in map[uuid.UUID]int) map[uuid.UUID]int {
	out := make(map[uuid.UUID]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mustJSON(m map[string]any) []byte {
	b, _ := json.Marshal(m)
	return b
}

func (r *Registry) recordMissedTicks(ctx context.Context, simulationID uuid.UUID, apiCalls map[uuid.UUID]int) {
	if r.pm == nil {
		return
	}
	ids, err := r.pm.ListAgentIDs(ctx, simulationID)
	if err != nil {
		return
	}
	for _, aid := range ids {
		dq, err := r.pm.IsDisqualified(ctx, simulationID, aid)
		if err != nil || dq {
			continue
		}
		if apiCalls[aid] > 0 {
			_ = r.pm.ResetMissedTicks(ctx, simulationID, aid)
			continue
		}
		n, err := r.pm.IncrementMissedTicks(ctx, simulationID, aid)
		if err != nil {
			continue
		}
		if n >= portfolio.MissedTicksDQThreshold {
			_ = r.pm.SetDisqualified(ctx, simulationID, aid)
			_ = r.publish(ctx, "agent.disqualified", map[string]any{
				"simulation_id": simulationID.String(),
				"agent_id":      aid.String(),
				"reason":        "missed_ticks",
			})
		}
	}
}

func (r *Registry) windowsDue(now time.Time) []uuid.UUID {
	r.mu.Lock()
	defer r.mu.Unlock()
	var ids []uuid.UUID
	for id, w := range r.windows {
		if w.Open && !w.CloseAt.After(now) {
			ids = append(ids, id)
		}
	}
	return ids
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

// MatchDateForOrder is the session date a newly submitted order will be eligible to fill.
// inWindow is true when the current tick's submission window is open.
func (r *Registry) MatchDateForOrder(simulationID uuid.UUID) (date time.Time, inWindow bool, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := r.clocks[simulationID]
	if c == nil {
		return time.Time{}, false, ErrNoActiveClock
	}
	if c.Status == "completed" {
		return time.Time{}, false, ErrClockCompleted
	}
	w := r.windows[simulationID]
	if w.Open {
		return c.CurrentDate, true, nil
	}
	return c.CurrentDate, false, nil
}

// OrderFillsOnCurrentDate is true when the next match session is still the clock's current date.
func (r *Registry) OrderFillsOnCurrentDate(simulationID uuid.UUID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := r.clocks[simulationID]
	if c == nil {
		return false
	}
	w := r.windows[simulationID]
	if w.Open {
		return true
	}
	return !ShouldAdvance(r.lastMatched[simulationID], c.CurrentDate)
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
