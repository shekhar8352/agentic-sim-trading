package orderbook

import (
	"testing"

	"github.com/agentic-sim-trading/market-simulator/pkg/models"
)

func TestCircuitTriggered(t *testing.T) {
	t.Parallel()
	if CircuitTriggered(110, 100) {
		t.Fatal("10% should not trip")
	}
	if !CircuitTriggered(111, 100) {
		t.Fatal("11% should trip")
	}
	if CircuitTriggered(50, 0) {
		t.Fatal("zero prev close should not trip")
	}
}

func TestLiquidityAndMinPrice(t *testing.T) {
	t.Parallel()
	if !LiquidityExceeded(11, 1000) {
		t.Fatal("11 > 1% of 1000")
	}
	if LiquidityExceeded(10, 1000) {
		t.Fatal("10 == 1% should pass")
	}
	if !BelowMinPrice(9.99) {
		t.Fatal("expected below min")
	}
}

func TestConcentrationExceeded(t *testing.T) {
	t.Parallel()
	if ConcentrationExceeded(19, 100) {
		t.Fatal("19% should pass")
	}
	if !ConcentrationExceeded(21, 100) {
		t.Fatal("21% should fail")
	}
}

func TestResolveFillMarketLimitStop(t *testing.T) {
	t.Parallel()
	bar := models.Quote{Open: 100, High: 110, Low: 90, Close: 105}

	px, d, _ := ResolveFill("market", "buy", 0, bar)
	if d != FillNow || px != 100 {
		t.Fatalf("market: px=%v d=%v", px, d)
	}

	px, d, _ = ResolveFill("limit", "buy", 95, bar)
	if d != FillNow || px != 95 {
		t.Fatalf("limit buy in range: px=%v d=%v", px, d)
	}
	_, d, _ = ResolveFill("limit", "buy", 80, bar)
	if d != DeferFill {
		t.Fatalf("limit buy below low should defer, d=%v", d)
	}

	px, d, _ = ResolveFill("limit", "sell", 108, bar)
	if d != FillNow || px != 108 {
		t.Fatalf("limit sell: px=%v d=%v", px, d)
	}
	_, d, _ = ResolveFill("limit", "sell", 120, bar)
	if d != DeferFill {
		t.Fatalf("limit sell above high should defer")
	}

	px, d, _ = ResolveFill("stop", "sell", 92, bar)
	if d != FillNow || px != 100 {
		t.Fatalf("stop sell triggered at open: px=%v d=%v", px, d)
	}
	_, d, _ = ResolveFill("stop", "sell", 80, bar)
	if d != DeferFill {
		t.Fatalf("stop sell not triggered")
	}

	_, d, reason := ResolveFill("ioc", "buy", 1, bar)
	if d != RejectFill || reason != "order_type_not_supported" {
		t.Fatalf("unknown type: d=%v reason=%s", d, reason)
	}
}

func TestApplyMarketImpact(t *testing.T) {
	t.Parallel()
	base := ApplyMarketImpact("buy", 1, 10_000, 100)
	if base != 100 {
		t.Fatalf("small order should not move price: %v", base)
	}
	moved := ApplyMarketImpact("buy", 200, 10_000, 100)
	if moved <= 100 {
		t.Fatalf("large buy should worsen price: %v", moved)
	}
	sold := ApplyMarketImpact("sell", 200, 10_000, 100)
	if sold >= 100 {
		t.Fatalf("large sell should worsen price: %v", sold)
	}
}
