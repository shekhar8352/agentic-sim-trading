package simulation

import (
	"encoding/json"
	"testing"
)

func TestCheckpointIntervalDays(t *testing.T) {
	t.Parallel()
	if got := CheckpointIntervalDays(nil); got != 5 {
		t.Fatalf("default interval: got %d", got)
	}
	cfg, _ := json.Marshal(map[string]any{"checkpoint_interval_days": 7})
	if got := CheckpointIntervalDays(cfg); got != 7 {
		t.Fatalf("custom interval: got %d", got)
	}
}

func TestAwaitingProceed(t *testing.T) {
	t.Parallel()
	cfg, _ := json.Marshal(map[string]any{"awaiting_proceed": true})
	if !AwaitingProceed(cfg) {
		t.Fatal("expected awaiting")
	}
}

func TestDefaultConfigHasCheckpointFields(t *testing.T) {
	t.Parallel()
	m := configMap(DefaultConfig())
	if CheckpointIntervalDays(DefaultConfig()) != 5 {
		t.Fatal("expected 5 day default")
	}
	if _, ok := m["auto_tick_enabled"]; !ok {
		t.Fatal("missing auto_tick_enabled")
	}
	if BarInterval(DefaultConfig()) != Interval1d {
		t.Fatal("expected 1d default bar_interval")
	}
	if OrderWindowSeconds(DefaultConfig()) != 3 {
		t.Fatal("expected 3s default order window")
	}
	if EffectiveTickInterval(DefaultConfig()) != 5 {
		t.Fatal("expected 5s effective interval at 1x")
	}
}

func TestEffectiveTickIntervalUsesMultiplier(t *testing.T) {
	t.Parallel()
	cfg, _ := json.Marshal(map[string]any{
		"tick_interval_seconds": 5.0,
		"tick_speed_multiplier": 10.0,
		"order_window_seconds":  3.0,
	})
	if got := EffectiveTickInterval(cfg); got != 0.5 {
		t.Fatalf("interval: got %v", got)
	}
	if got := EffectiveOrderWindow(cfg); got != 0.3 {
		t.Fatalf("window: got %v", got)
	}
}

func TestBarIntervalAndMissedTicks(t *testing.T) {
	t.Parallel()
	hourly, _ := json.Marshal(map[string]any{"bar_interval": "60m"})
	if BarInterval(hourly) != Interval60m {
		t.Fatal("60m")
	}
	if MissedTicksLimit(hourly) != 20 {
		t.Fatalf("hourly missed ticks default: %d", MissedTicksLimit(hourly))
	}
	if MissedTicksLimit(nil) != 10 {
		t.Fatal("daily missed ticks default")
	}
	if ParticipationRate(nil) != 0.10 {
		t.Fatal("participation default")
	}
}
