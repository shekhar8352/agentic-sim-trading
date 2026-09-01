package orderbook

import (
	"math"
	"strings"

	"github.com/agentic-sim-trading/market-simulator/pkg/models"
)

const (
	NSETickSize              = 0.05
	DefaultParticipationRate = 0.10
	spreadRangeK             = 0.10
)

// VWAPProxy is a typical-price stand-in for bar VWAP: (H+L+C)/3.
func VWAPProxy(bar models.Quote) float64 {
	return (bar.High + bar.Low + bar.Close) / 3
}

// HalfSpread is max(NSE tick, k * bar range).
func HalfSpread(bar models.Quote) float64 {
	rng := bar.High - bar.Low
	if rng < 0 {
		rng = 0
	}
	hs := spreadRangeK * rng
	if hs < NSETickSize {
		return NSETickSize
	}
	return hs
}

// ApplyAggressiveSpread worsens the price for market/stop (liquidity taking) fills.
func ApplyAggressiveSpread(side string, price, halfSpread float64) float64 {
	if halfSpread <= 0 {
		return price
	}
	if strings.EqualFold(side, "buy") {
		return price + halfSpread
	}
	return price - halfSpread
}

// ParticipationCap is min(remaining, floor(rate * barVolume)).
func ParticipationCap(remaining int, barVolume int64, rate float64) int {
	if remaining <= 0 {
		return 0
	}
	if rate <= 0 {
		rate = DefaultParticipationRate
	}
	maxFill := int(math.Floor(rate * float64(barVolume)))
	if maxFill < 0 {
		maxFill = 0
	}
	if maxFill > remaining {
		return remaining
	}
	return maxFill
}

// PathResult is the hourly matching outcome against one bar.
type PathResult struct {
	Decision   FillDecision
	FillQty    int
	FillPrice  float64
	Aggressive bool
	Reason     string
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// ResolvePathFill matches one order slice against an hourly bar (not the EOD open).
func ResolvePathFill(orderType, side string, limitPrice float64, remaining int, bar models.Quote, participRate float64) PathResult {
	ot := strings.ToLower(strings.TrimSpace(orderType))
	sd := strings.ToLower(strings.TrimSpace(side))
	if remaining <= 0 {
		return PathResult{Decision: RejectFill, Reason: "invalid_quantity"}
	}

	switch ot {
	case "market":
		qty := ParticipationCap(remaining, bar.Volume, participRate)
		if qty <= 0 {
			return PathResult{Decision: DeferFill, Reason: ReasonDefer}
		}
		return PathResult{Decision: FillNow, FillQty: qty, FillPrice: VWAPProxy(bar), Aggressive: true}
	case "limit":
		if limitPrice <= 0 {
			return PathResult{Decision: RejectFill, Reason: "invalid_limit_price"}
		}
		var px float64
		switch sd {
		case "buy":
			if bar.Low > limitPrice {
				return PathResult{Decision: DeferFill, Reason: ReasonDefer}
			}
			if bar.Open <= limitPrice {
				px = minFloat(limitPrice, bar.Open)
			} else {
				px = limitPrice
			}
		case "sell":
			if bar.High < limitPrice {
				return PathResult{Decision: DeferFill, Reason: ReasonDefer}
			}
			if bar.Open >= limitPrice {
				px = maxFloat(limitPrice, bar.Open)
			} else {
				px = limitPrice
			}
		default:
			return PathResult{Decision: RejectFill, Reason: "invalid_side"}
		}
		qty := ParticipationCap(remaining, bar.Volume, participRate)
		if qty <= 0 {
			return PathResult{Decision: DeferFill, Reason: ReasonDefer}
		}
		return PathResult{Decision: FillNow, FillQty: qty, FillPrice: px, Aggressive: false}
	case "stop", "stop_loss":
		if limitPrice <= 0 {
			return PathResult{Decision: RejectFill, Reason: "invalid_stop_price"}
		}
		var px float64
		switch sd {
		case "buy":
			if bar.High < limitPrice {
				return PathResult{Decision: DeferFill, Reason: ReasonDefer}
			}
			px = minFloat(limitPrice, bar.Open)
		case "sell":
			if bar.Low > limitPrice {
				return PathResult{Decision: DeferFill, Reason: ReasonDefer}
			}
			px = maxFloat(limitPrice, bar.Open)
		default:
			return PathResult{Decision: RejectFill, Reason: "invalid_side"}
		}
		qty := ParticipationCap(remaining, bar.Volume, participRate)
		if qty <= 0 {
			return PathResult{Decision: DeferFill, Reason: ReasonDefer}
		}
		return PathResult{Decision: FillNow, FillQty: qty, FillPrice: px, Aggressive: true}
	default:
		return PathResult{Decision: RejectFill, Reason: "order_type_not_supported"}
	}
}

// CircuitBandBreached is true when the bar's high/low prints through ±10% vs previous daily close.
func CircuitBandBreached(high, low, prevClose float64) bool {
	if prevClose <= 0 {
		return false
	}
	if (high-prevClose)/prevClose*100 > 10 {
		return true
	}
	if (prevClose-low)/prevClose*100 > 10 {
		return true
	}
	return false
}
