package portfolio

import (
	"testing"
	"time"
)

func TestSharpeAndDrawdown(t *testing.T) {
	t.Parallel()
	es := EquitySeries{Values: []float64{100, 110, 105, 120}}
	if SharpeRatio(es, 0) == 0 {
		t.Fatal("expected non-zero sharpe")
	}
	dd := MaxDrawdown(es)
	if dd <= 0 {
		t.Fatalf("expected drawdown, got %v", dd)
	}
}

func TestAnalyzeClosedTrades(t *testing.T) {
	t.Parallel()
	d0 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	d5 := time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC)
	st := AnalyzeClosedTrades([]FillLot{
		{Symbol: "TCS.NS", Side: "buy", Quantity: 10, Price: 100, TradeVal: 1000, FilledAt: d0},
		{Symbol: "TCS.NS", Side: "sell", Quantity: 10, Price: 110, TradeVal: 1100, FilledAt: d5},
	})
	if st.ClosedTrades != 1 || st.Profitable != 1 || st.WinRatePct != 100 {
		t.Fatalf("stats: %+v", st)
	}
	if st.AvgHoldingDays != 5 {
		t.Fatalf("hold days: %v", st.AvgHoldingDays)
	}
}

func TestTurnoverAndSectors(t *testing.T) {
	t.Parallel()
	to := TurnoverRate(200, EquitySeries{Values: []float64{100, 100}})
	if to != 2 {
		t.Fatalf("turnover: %v", to)
	}
	pct := SectorPercents(map[string]float64{"IT": 80, "Energy": 20})
	if pct["IT"] != 80 || pct["Energy"] != 20 {
		t.Fatalf("sectors: %v", pct)
	}
}
