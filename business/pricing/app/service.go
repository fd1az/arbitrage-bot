// Package app contains application services and port definitions for the pricing context.
package app

import (
	"context"

	"github.com/fd1az/arbitrage-bot/business/pricing/domain"
	"github.com/shopspring/decimal"
)

// PricingService coordinates price fetching from CEX and DEX providers.
type PricingService struct {
	cex CEXProvider
	dex DEXProvider
}

// NewPricingService creates a new PricingService with the given providers.
func NewPricingService(cex CEXProvider, dex DEXProvider) *PricingService {
	return &PricingService{
		cex: cex,
		dex: dex,
	}
}

// GetPriceSnapshot retrieves current prices from both CEX and DEX for comparison.
func (s *PricingService) GetPriceSnapshot(ctx context.Context, pair domain.Pair, sizes []decimal.Decimal) (*domain.PriceSnapshot, error) {
	// TODO: Implement price snapshot fetching
	return nil, nil
}
