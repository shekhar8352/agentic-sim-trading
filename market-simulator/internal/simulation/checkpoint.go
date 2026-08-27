package simulation

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultCheckpointIntervalDays = 5
const defaultTickIntervalSeconds = 5.0
const defaultOrderWindowSeconds = 3.0

// DefaultConfig is applied to new simulations so long runs pause for user approval.
func DefaultConfig() json.RawMessage {
	b, _ := json.Marshal(map[string]any{
		"checkpoint_interval_days": defaultCheckpointIntervalDays,
		"days_since_checkpoint":    0,
		"awaiting_proceed":         false,
		"auto_tick_enabled":        true,
		"tick_interval_seconds":    defaultTickIntervalSeconds,
		"order_window_seconds":     defaultOrderWindowSeconds,
		"tick_speed_multiplier":    1.0,
	})
	return b
}

func configMap(cfg json.RawMessage) map[string]any {
	if len(cfg) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(cfg, &m); err != nil {
		return map[string]any{}
	}
	return m
}

// CheckpointIntervalDays returns trading days per segment before pausing for approval (default 5).
func CheckpointIntervalDays(cfg json.RawMessage) int {
	m := configMap(cfg)
	x, ok := m["checkpoint_interval_days"].(float64)
	if !ok || x < 1 {
		return defaultCheckpointIntervalDays
	}
	if x > 60 {
		return 60
	}
	return int(x)
}

// DaysSinceCheckpoint counts trading days advanced in the current segment.
func DaysSinceCheckpoint(cfg json.RawMessage) int {
	m := configMap(cfg)
	x, ok := m["days_since_checkpoint"].(float64)
	if !ok || x < 0 {
		return 0
	}
	return int(x)
}

// AwaitingProceed is true when the sim paused at a checkpoint and needs user approval.
func AwaitingProceed(cfg json.RawMessage) bool {
	m := configMap(cfg)
	v, ok := m["awaiting_proceed"].(bool)
	return ok && v
}

// AutoTickEnabled controls the background day-advance loop (default true).
func AutoTickEnabled(cfg json.RawMessage) bool {
	m := configMap(cfg)
	if v, ok := m["auto_tick_enabled"].(bool); ok {
		return v
	}
	return true
}

// TickIntervalSeconds is wall-clock delay between auto ticks in demo mode.
func TickIntervalSeconds(cfg json.RawMessage) float64 {
	m := configMap(cfg)
	x, ok := m["tick_interval_seconds"].(float64)
	if !ok || x <= 0 {
		return defaultTickIntervalSeconds
	}
	if x > 3600 {
		return 3600
	}
	return x
}

// OrderWindowSeconds is how long agents may submit after sim.tick (0 = batch / immediate match).
func OrderWindowSeconds(cfg json.RawMessage) float64 {
	m := configMap(cfg)
	x, ok := m["order_window_seconds"].(float64)
	if !ok {
		return defaultOrderWindowSeconds
	}
	if x < 0 {
		return 0
	}
	if x > 120 {
		return 120
	}
	return x
}

// EffectiveTickInterval applies tick_speed_multiplier to the configured auto-tick delay.
func EffectiveTickInterval(cfg json.RawMessage) float64 {
	mult := TickSpeedMultiplier(cfg)
	if mult <= 0 {
		mult = 1
	}
	iv := TickIntervalSeconds(cfg) / mult
	if iv < 0.05 {
		return 0.05
	}
	return iv
}

// EffectiveOrderWindow applies tick_speed_multiplier to the submission window.
func EffectiveOrderWindow(cfg json.RawMessage) float64 {
	mult := TickSpeedMultiplier(cfg)
	if mult <= 0 {
		mult = 1
	}
	w := OrderWindowSeconds(cfg) / mult
	if w < 0 {
		return 0
	}
	return w
}

// LastMatchedDate reads config.last_matched_date (YYYY-MM-DD) when present.
func LastMatchedDate(cfg json.RawMessage) string {
	m := configMap(cfg)
	s, _ := m["last_matched_date"].(string)
	return s
}

func mergeConfigFragment(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, fragment map[string]any) error {
	b, err := json.Marshal(fragment)
	if err != nil {
		return err
	}
	return MergeConfig(ctx, pool, id, b)
}

// RecordTickForCheckpoint increments days_since_checkpoint and returns new count + interval.
func RecordTickForCheckpoint(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (newDays int, interval int, err error) {
	if pool == nil {
		return 0, defaultCheckpointIntervalDays, nil
	}
	row, err := Get(ctx, pool, id)
	if err != nil {
		return 0, 0, err
	}
	interval = CheckpointIntervalDays(row.Config)
	newDays = DaysSinceCheckpoint(row.Config) + 1
	err = mergeConfigFragment(ctx, pool, id, map[string]any{
		"days_since_checkpoint": newDays,
	})
	return newDays, interval, err
}

// SetAwaitingProceed updates the checkpoint gate flag.
func SetAwaitingProceed(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, awaiting bool) error {
	return mergeConfigFragment(ctx, pool, id, map[string]any{"awaiting_proceed": awaiting})
}

// ResetCheckpointWindow clears the segment counter after the user chooses to continue.
func ResetCheckpointWindow(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) error {
	return mergeConfigFragment(ctx, pool, id, map[string]any{
		"days_since_checkpoint": 0,
		"awaiting_proceed":      false,
	})
}
