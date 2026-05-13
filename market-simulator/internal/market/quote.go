package market

import (
	"context"
	"time"

	"github.com/agentic-sim-trading/market-simulator/pkg/models"
)

// QuoteProvider answers price questions for the simulated date (wired in later phases).
type QuoteProvider struct {
	data *Data
	// AsOf is set when simulation clock advances; zero means "latest in DB".
	AsOf time.Time
}

func NewQuoteProvider(data *Data) *QuoteProvider {
	return &QuoteProvider{data: data}
}

// Current returns the bar used as the reference quote (latest bar ≤ AsOf, or latest overall).
func (p *QuoteProvider) Current(ctx context.Context, symbol string) (models.Quote, error) {
	if p.AsOf.IsZero() {
		return p.data.LatestBar(ctx, symbol)
	}
	bars, err := p.data.BarsForSymbol(ctx, symbol, p.AsOf, 1)
	if err != nil {
		return models.Quote{}, err
	}
	if len(bars) == 0 {
		return models.Quote{Symbol: symbol}, nil
	}
	return bars[len(bars)-1], nil
}
