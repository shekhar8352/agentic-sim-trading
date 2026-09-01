package orderbook

import (
	"testing"

	"github.com/agentic-sim-trading/market-simulator/pkg/models"
)

func TestVWAPProxyAndParticipation(t *testing.T) {
	t.Parallel()
	bar := models.Quote{Open: 100, High: 110, Low: 90, Close: 105, Volume: 1000}
	if got := VWAPProxy(bar); got != 101.66666666666667 && (got < 101.6 || got > 101.7) {
		// (110+90+105)/3 = 305/3 = 101.666...
		if got != (110+90+105)/3 {
			t.Fatalf("vwap: %v", got)
		}
	}
	if ParticipationCap(500, 1000, 0.10) != 100 {
		t.Fatalf("cap 10%% of 1000")
	}
	if ParticipationCap(50, 1000, 0.10) != 50 {
		t.Fatal("cap should not exceed remaining")
	}
	if ParticipationCap(10, 0, 0.10) != 0 {
		t.Fatal("zero volume")
	}
}

func TestResolvePathFillMarketPartial(t *testing.T) {
	t.Parallel()
	bar := models.Quote{Open: 100, High: 110, Low: 90, Close: 105, Volume: 1000}
	r := ResolvePathFill("market", "buy", 0, 500, bar, 0.10)
	if r.Decision != FillNow || r.FillQty != 100 {
		t.Fatalf("market partial: %+v", r)
	}
	if r.FillPrice != VWAPProxy(bar) || !r.Aggressive {
		t.Fatalf("market px: %+v", r)
	}
}

func TestResolvePathFillLimitOpenThrough(t *testing.T) {
	t.Parallel()
	bar := models.Quote{Open: 94, High: 100, Low: 90, Close: 96, Volume: 10_000}
	r := ResolvePathFill("limit", "buy", 95, 10, bar, 0.10)
	if r.Decision != FillNow || r.FillPrice != 94 {
		t.Fatalf("buy open through should fill at open: %+v", r)
	}
	bar2 := models.Quote{Open: 100, High: 102, Low: 94, Close: 97, Volume: 10_000}
	r = ResolvePathFill("limit", "buy", 95, 10, bar2, 0.10)
	if r.Decision != FillNow || r.FillPrice != 95 {
		t.Fatalf("buy touch should fill at limit: %+v", r)
	}
	bar3 := models.Quote{Open: 100, High: 102, Low: 96, Close: 97, Volume: 10_000}
	r = ResolvePathFill("limit", "buy", 95, 10, bar3, 0.10)
	if r.Decision != DeferFill {
		t.Fatalf("limit not touched: %+v", r)
	}
}

func TestResolvePathFillStopNotDayOpen(t *testing.T) {
	t.Parallel()
	bar := models.Quote{Open: 100, High: 101, Low: 90, Close: 95, Volume: 10_000}
	r := ResolvePathFill("stop", "sell", 92, 10, bar, 0.10)
	if r.Decision != FillNow || r.FillPrice != 100 {
		t.Fatalf("stop sell fill at max(stop, open)=open: %+v", r)
	}
	barGap := models.Quote{Open: 88, High: 90, Low: 85, Close: 89, Volume: 10_000}
	r = ResolvePathFill("stop", "sell", 92, 10, barGap, 0.10)
	if r.Decision != FillNow || r.FillPrice != 92 {
		t.Fatalf("gapped through stop fills at stop: %+v", r)
	}
}

func TestCircuitBandVsDailyClose(t *testing.T) {
	t.Parallel()
	if CircuitBandBreached(110, 100, 100) {
		t.Fatal("10% high should not trip")
	}
	if !CircuitBandBreached(111, 100, 100) {
		t.Fatal("11% high should trip")
	}
	if !CircuitBandBreached(100, 89, 100) {
		t.Fatal("11% low should trip")
	}
	if CircuitBandBreached(105, 95, 0) {
		t.Fatal("zero prev close")
	}
}

func TestHalfSpreadFloor(t *testing.T) {
	t.Parallel()
	bar := models.Quote{High: 100.1, Low: 100.0}
	if HalfSpread(bar) != NSETickSize {
		t.Fatalf("tick floor: %v", HalfSpread(bar))
	}
	wide := models.Quote{High: 110, Low: 90}
	if HalfSpread(wide) != 2.0 { // 0.1 * 20
		t.Fatalf("range spread: %v", HalfSpread(wide))
	}
	buy := ApplyAggressiveSpread("buy", 100, 0.05)
	if buy != 100.05 {
		t.Fatalf("buy spread: %v", buy)
	}
}
