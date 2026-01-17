// Package app contains application services and port definitions for the pricing context.
package app

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/fd1az/arbitrage-bot/business/pricing/domain"
	"github.com/fd1az/arbitrage-bot/internal/asset"
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
func (s *PricingService) GetPriceSnapshot(ctx context.Context, pair domain.Pair, tradeSize decimal.Decimal) (*domain.PriceSnapshot, error) {
	snapshot := &domain.PriceSnapshot{
		Pair:      pair,
		Timestamp: time.Now(),
	}

	// Get CEX prices (bid and ask for the trade size)
	cexBid, err := s.cex.GetEffectivePrice(ctx, pair, tradeSize, domain.SideSell)
	if err != nil {
		return nil, fmt.Errorf("failed to get CEX bid: %w", err)
	}
	snapshot.CEXBid = cexBid

	cexAsk, err := s.cex.GetEffectivePrice(ctx, pair, tradeSize, domain.SideBuy)
	if err != nil {
		return nil, fmt.Errorf("failed to get CEX ask: %w", err)
	}
	snapshot.CEXAsk = cexAsk

	// Get DEX quote
	// Convert trade size to raw amount (considering base asset decimals)
	amountIn := toRawAmount(pair.Base, tradeSize)

	// For DEX, convert native ETH to WETH (Uniswap uses WETH)
	tokenIn := pair.Base.Address()
	tokenOut := pair.Quote.Address()
	if pair.Base.IsNative() {
		tokenIn = asset.AddrWETHEthereum // Use WETH for native ETH
	}
	if pair.Quote.IsNative() {
		tokenOut = asset.AddrWETHEthereum
	}

	dexQuote, err := s.dex.GetQuote(ctx, tokenIn, tokenOut, amountIn)
	if err != nil {
		return nil, fmt.Errorf("failed to get DEX quote: %w", err)
	}
	snapshot.DEXQuote = dexQuote

	return snapshot, nil
}

// GetCEXOrderbook retrieves the current orderbook from CEX.
func (s *PricingService) GetCEXOrderbook(ctx context.Context, pair domain.Pair) (*domain.Orderbook, error) {
	return s.cex.GetOrderbook(ctx, pair)
}

// toRawAmount converts a decimal amount to raw (wei-like) representation.
func toRawAmount(a *asset.Asset, amount decimal.Decimal) *big.Int {
	// Multiply by 10^decimals
	multiplier := decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(a.Decimals())))
	raw := amount.Mul(multiplier)
	result := raw.BigInt()
	return result
}
