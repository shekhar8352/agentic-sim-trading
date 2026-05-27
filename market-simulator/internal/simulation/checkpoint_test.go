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
}
