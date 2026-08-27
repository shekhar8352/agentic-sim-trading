package portfolio

import (
	"math"
	"sort"
	"strings"
	"time"
)

// EquitySeries holds daily total portfolio values for risk metrics.
type EquitySeries struct {
	Values []float64
}

// SharpeRatio is annualised from daily returns (√252), optional daily risk-free rate.
func SharpeRatio(series EquitySeries, riskFreeDaily float64) float64 {
	if len(series.Values) < 2 {
		return 0
	}
	rets := make([]float64, 0, len(series.Values)-1)
	for i := 1; i < len(series.Values); i++ {
		prev := series.Values[i-1]
		if prev == 0 {
			continue
		}
		rets = append(rets, (series.Values[i]-prev)/prev-riskFreeDaily)
	}
	if len(rets) == 0 {
		return 0
	}
	mean := mean(rets)
	sd := stdDev(rets, mean)
	if sd == 0 {
		return 0
	}
	return math.Sqrt(252) * mean / sd
}

func mean(xs []float64) float64 {
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func stdDev(xs []float64, mu float64) float64 {
	var acc float64
	for _, x := range xs {
		d := x - mu
		acc += d * d
	}
	return math.Sqrt(acc / float64(len(xs)))
}

// MaxDrawdown returns peak-to-trough decline as a positive fraction.
func MaxDrawdown(series EquitySeries) float64 {
	if len(series.Values) == 0 {
		return 0
	}
	var peak = series.Values[0]
	var maxDD float64
	for _, v := range series.Values {
		if v > peak {
			peak = v
		}
		dd := (peak - v) / peak
		if dd > maxDD {
			maxDD = dd
		}
	}
	return maxDD
}

// FillLot is one filled order used to compute closed-trade stats.
type FillLot struct {
	Symbol   string
	Side     string
	Quantity int
	Price    float64
	TradeVal float64
	FilledAt time.Time
}

// ClosedTradeStats is win rate and holding period from FIFO buy/sell matching.
type ClosedTradeStats struct {
	ClosedTrades    int
	Profitable      int
	WinRatePct      float64
	AvgHoldingDays  float64
	TotalTradeValue float64
}

// AnalyzeClosedTrades FIFO-matches buys to sells per symbol.
func AnalyzeClosedTrades(fills []FillLot) ClosedTradeStats {
	type lot struct {
		qty   int
		price float64
		day   time.Time
	}
	bySym := map[string][]FillLot{}
	var totalTV float64
	for _, f := range fills {
		totalTV += f.TradeVal
		bySym[f.Symbol] = append(bySym[f.Symbol], f)
	}
	var closed, profitable int
	var holdSum float64
	for _, list := range bySym {
		sort.Slice(list, func(i, j int) bool {
			if list[i].FilledAt.Equal(list[j].FilledAt) {
				return i < j
			}
			return list[i].FilledAt.Before(list[j].FilledAt)
		})
		var buys []lot
		for _, f := range list {
			side := strings.ToLower(f.Side)
			if side == "buy" {
				buys = append(buys, lot{qty: f.Quantity, price: f.Price, day: f.FilledAt})
				continue
			}
			remain := f.Quantity
			for remain > 0 && len(buys) > 0 {
				b := &buys[0]
				m := remain
				if b.qty < m {
					m = b.qty
				}
				closed++
				if f.Price > b.price {
					profitable++
				}
				holdSum += f.FilledAt.Sub(b.day).Hours() / 24
				b.qty -= m
				remain -= m
				if b.qty == 0 {
					buys = buys[1:]
				}
			}
		}
	}
	out := ClosedTradeStats{ClosedTrades: closed, Profitable: profitable, TotalTradeValue: totalTV}
	if closed > 0 {
		out.WinRatePct = float64(profitable) / float64(closed) * 100
		out.AvgHoldingDays = holdSum / float64(closed)
	}
	return out
}

// TurnoverRate is total trade value divided by average daily portfolio value.
func TurnoverRate(totalTradeValue float64, equity EquitySeries) float64 {
	if len(equity.Values) == 0 {
		return 0
	}
	avg := mean(equity.Values)
	if avg == 0 {
		return 0
	}
	return totalTradeValue / avg
}

// SectorPercents normalizes sector invested values to percentages of total invested.
func SectorPercents(investedBySector map[string]float64) map[string]float64 {
	var sum float64
	for _, v := range investedBySector {
		sum += v
	}
	out := make(map[string]float64, len(investedBySector))
	if sum <= 0 {
		return out
	}
	for k, v := range investedBySector {
		out[k] = v / sum * 100
	}
	return out
}
