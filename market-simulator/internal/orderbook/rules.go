package orderbook

import (
	"math"
	"strings"

	"github.com/agentic-sim-trading/market-simulator/pkg/models"
)

const minStockPrice = 10.0
const concentrationPct = 0.20

// ReasonDefer is an internal sentinel: leave the order pending for a later session.
const ReasonDefer = "_defer"

// CircuitTriggered reports a ±10% open vs previous close halt (rules §9.1).
func CircuitTriggered(open, prevClose float64) bool {
	if prevClose <= 0 {
		return false
	}
	pct := (open - prevClose) / prevClose * 100
	return math.Abs(pct) > 10
}

// LiquidityExceeded is true when quantity is over 1% of previous-day volume (rules §9.2).
func LiquidityExceeded(quantity int, prevVolume int64) bool {
	maxQty := int(math.Floor(0.01 * float64(prevVolume)))
	return quantity > maxQty
}

// BelowMinPrice rejects names opening under ₹10 (rules §9.3).
func BelowMinPrice(open float64) bool {
	return open < minStockPrice
}

// ConcentrationExceeded is true when the post-trade stock mark would exceed 20% of portfolio value.
func ConcentrationExceeded(newStockExposure, portfolioValue float64) bool {
	if portfolioValue <= 0 {
		return newStockExposure > 0
	}
	return newStockExposure > concentrationPct*portfolioValue+1e-9
}

// FillDecision is the matching outcome for one order against a daily bar.
type FillDecision int

const (
	FillNow FillDecision = iota
	DeferFill
	RejectFill
)

// ResolveFill picks a fill price from the EOD bar (market / limit / stop).
func ResolveFill(orderType, side string, limitPrice float64, bar models.Quote) (fillPrice float64, decision FillDecision, reason string) {
	ot := strings.ToLower(strings.TrimSpace(orderType))
	sd := strings.ToLower(strings.TrimSpace(side))
	switch ot {
	case "market":
		return bar.Open, FillNow, ""
	case "limit":
		if limitPrice <= 0 {
			return 0, RejectFill, "invalid_limit_price"
		}
		switch sd {
		case "buy":
			if bar.Low <= limitPrice {
				return limitPrice, FillNow, ""
			}
			return 0, DeferFill, ReasonDefer
		case "sell":
			if bar.High >= limitPrice {
				return limitPrice, FillNow, ""
			}
			return 0, DeferFill, ReasonDefer
		default:
			return 0, RejectFill, "invalid_side"
		}
	case "stop", "stop_loss":
		if limitPrice <= 0 {
			return 0, RejectFill, "invalid_stop_price"
		}
		switch sd {
		case "buy":
			if bar.High >= limitPrice {
				return bar.Open, FillNow, ""
			}
			return 0, DeferFill, ReasonDefer
		case "sell":
			if bar.Low <= limitPrice {
				return bar.Open, FillNow, ""
			}
			return 0, DeferFill, ReasonDefer
		default:
			return 0, RejectFill, "invalid_side"
		}
	default:
		return 0, RejectFill, "order_type_not_supported"
	}
}

// ApplyMarketImpact nudges fill price against the trader when size is a large share of prior volume.
func ApplyMarketImpact(side string, quantity int, prevVolume int64, price float64) float64 {
	if prevVolume <= 0 || price <= 0 || quantity <= 0 {
		return price
	}
	frac := float64(quantity) / float64(prevVolume)
	if frac <= 0.005 {
		return price
	}
	impact := math.Min(0.002, (frac-0.005)*0.1)
	if strings.EqualFold(side, "buy") {
		return price * (1 + impact)
	}
	return price * (1 - impact)
}
