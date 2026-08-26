package clock

import (
	"testing"
	"time"
)

func TestShouldAdvance(t *testing.T) {
	t.Parallel()
	d0 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	d1 := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
	prev := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	if ShouldAdvance(time.Time{}, d0) {
		t.Fatal("zero lastMatched should process current date")
	}
	if ShouldAdvance(prev, d0) {
		t.Fatal("last before current should not advance")
	}
	if !ShouldAdvance(d0, d0) {
		t.Fatal("already matched current should advance")
	}
	if !ShouldAdvance(d1, d0) {
		t.Fatal("last after current should advance")
	}
}
