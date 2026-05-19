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
	ErrInvalidOrderType = errors.New("order_type must be market in phase 1")
	ErrSymbolUniverse   = errors.New("symbol must be a Nifty 50 ticker (.NS)")
)

// MarketOrder checks Phase-1 market order input (Step 10).
func MarketOrder(symbol, orderType, side string, quantity int) error {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return ErrEmptySymbol
	}
	ot := strings.ToLower(strings.TrimSpace(orderType))
	if ot != "market" {
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
