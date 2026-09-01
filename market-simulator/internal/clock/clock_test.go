package clock

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSimClockTickProgression(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	d0 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	d1 := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC)
	days := []time.Time{d0, d1, d2}

	c := NewSimClockAt(id, days, 0, "running", nil)
	ctx := context.Background()

	if c.CurrentDate != d0 || c.CurrentIndex != 0 {
		t.Fatalf("initial date/index: got %v %d", c.CurrentDate, c.CurrentIndex)
	}
	if err := c.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if c.CurrentDate != d1 || c.CurrentIndex != 1 || c.Status != "running" {
		t.Fatalf("after tick1: got %v %d %s", c.CurrentDate, c.CurrentIndex, c.Status)
	}
	if err := c.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if c.CurrentDate != d2 || c.CurrentIndex != 2 {
		t.Fatalf("after tick2: got %v %d", c.CurrentDate, c.CurrentIndex)
	}
	if err := c.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if c.Status != "completed" {
		t.Fatalf("expected completed, got %s", c.Status)
	}

	c2 := NewSimClockAt(id, days, 0, "paused", nil)
	if err := c2.Tick(ctx); err != ErrClockNotRunning {
		t.Fatalf("paused tick: got %v", err)
	}
}

func TestHourlySessionBarIndex(t *testing.T) {
	t.Parallel()
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	a := time.Date(2025, 3, 12, 3, 45, 0, 0, time.UTC)
	b := time.Date(2025, 3, 12, 4, 45, 0, 0, time.UTC)
	c := time.Date(2025, 3, 13, 3, 45, 0, 0, time.UTC)
	clock := NewSimClockAt(id, []time.Time{a, b, c}, 1, "running", nil)
	clock.Interval = "60m"
	if clock.SessionBarIndex() != 2 || clock.SessionBarCount() != 2 {
		t.Fatalf("session bars: %d/%d", clock.SessionBarIndex(), clock.SessionBarCount())
	}
	if clock.IsLastBarOfSession() != true {
		t.Fatal("index 1 should be last of 12 Mar session")
	}
	clock.CurrentIndex = 0
	clock.CurrentDate = a
	if clock.IsLastBarOfSession() {
		t.Fatal("first bar should not be last")
	}
}
