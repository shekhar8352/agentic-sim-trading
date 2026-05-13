package portfolio

import "math"

// EquitySeries holds daily total portfolio values for risk metrics.
type EquitySeries struct {
	Values []float64
}

// SharpeRatio is a minimal placeholder (annualization assumes daily returns).
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
