package validation

import (
	"errors"
	"fmt"
	"strings"

	"github.com/agentic-sim-trading/market-simulator/internal/universe"
)

const MaxOrderQuantity = 1_000_000

var (
	ErrEmptySymbol      = errors.New("symbol required")
	ErrInvalidSide      = errors.New("side must be buy or sell")
	ErrInvalidQty       = errors.New("quantity must be a positive integer")
	ErrInvalidOrderType = errors.New("order_type must be market, limit, or stop")
	ErrSymbolUniverse   = errors.New("symbol must be a Nifty 50 ticker (.NS)")
	ErrLimitPrice       = errors.New("limit and stop orders require a positive price")
)

// Order checks agent order input (market, limit, stop).
func Order(symbol, orderType, side string, quantity int, price *float64) error {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return ErrEmptySymbol
	}
	ot := strings.ToLower(strings.TrimSpace(orderType))
	switch ot {
	case "market":
	case "limit", "stop", "stop_loss":
		if price == nil || *price <= 0 {
			return ErrLimitPrice
		}
	default:
		return ErrInvalidOrderType
	}
	sd := strings.ToLower(strings.TrimSpace(side))
	if sd != "buy" && sd != "sell" {
		return ErrInvalidSide
	}
	if quantity <= 0 || quantity > MaxOrderQuantity {
		return ErrInvalidQty
	}
	if !universe.IsNifty50(symbol) {
		return ErrSymbolUniverse
	}
	return nil
}

// MarketOrder checks Phase-1 market order input (kept for callers that only place markets).
func MarketOrder(symbol, orderType, side string, quantity int) error {
	return Order(symbol, orderType, side, quantity, nil)
}

// ListedSymbol ensures the ticker is in the tradeable universe (quotes / OHLCV).
func ListedSymbol(symbol string) error {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return ErrEmptySymbol
	}
	if !universe.IsNifty50(symbol) {
		return fmt.Errorf("%w: %s", ErrSymbolUniverse, symbol)
	}
	return nil
}
