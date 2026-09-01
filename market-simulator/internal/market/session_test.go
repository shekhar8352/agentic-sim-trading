package market

import (
	"testing"
	"time"
)

func TestSessionDateIST(t *testing.T) {
	t.Parallel()
	// 09:15 IST = 03:45 UTC
	bar := time.Date(2025, 3, 12, 3, 45, 0, 0, time.UTC)
	d := SessionDate(bar)
	if d.Year() != 2025 || d.Month() != 3 || d.Day() != 12 {
		t.Fatalf("session date: %v", d)
	}
	cal := SessionCalendarUTC(bar)
	if cal.Format(time.DateOnly) != "2025-03-12" {
		t.Fatalf("calendar utc: %s", cal.Format(time.DateOnly))
	}
}

func TestSameSessionAcrossHours(t *testing.T) {
	t.Parallel()
	a := time.Date(2025, 3, 12, 3, 45, 0, 0, time.UTC)
	b := time.Date(2025, 3, 12, 10, 0, 0, 0, time.UTC)
	c := time.Date(2025, 3, 13, 3, 45, 0, 0, time.UTC)
	if !SameSession(a, b) {
		t.Fatal("same IST day")
	}
	if SameSession(a, c) {
		t.Fatal("next session")
	}
}

func TestNormalizeInterval(t *testing.T) {
	t.Parallel()
	if NormalizeInterval("60m") != Interval60m {
		t.Fatal("60m")
	}
	if NormalizeInterval("1d") != Interval1d {
		t.Fatal("1d")
	}
	if NormalizeInterval("") != Interval1d {
		t.Fatal("default")
	}
}
