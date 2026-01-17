// Package app contains application services and port definitions for the arbitrage context.
package app

import (
	"github.com/fd1az/arbitrage-bot/business/arbitrage/domain"
	pricingDomain "github.com/fd1az/arbitrage-bot/business/pricing/domain"
	"github.com/fd1az/arbitrage-bot/internal/asset"
	"github.com/shopspring/decimal"
)

// Fee rates for exchanges
var (
	// Uniswap V3 fee tier (0.3% = 30 bps)
	UniswapFeeBps = decimal.NewFromFloat(0.003)
	// Binance spot trading fee (~0.1% = 10 bps)
	BinanceFeeBps = decimal.NewFromFloat(0.001)
	// Total round-trip fees
	TotalFeeRate = UniswapFeeBps.Add(BinanceFeeBps)
)

// ProfitCalculator calculates arbitrage profitability.
type ProfitCalculator struct {
	minProfitBps decimal.Decimal
	minProfitUSD decimal.Decimal
}

// NewProfitCalculator creates a new ProfitCalculator with thresholds.
func NewProfitCalculator(minProfitBps, minProfitUSD decimal.Decimal) *ProfitCalculator {
	return &ProfitCalculator{
		minProfitBps: minProfitBps,
		minProfitUSD: minProfitUSD,
	}
}

// Calculate computes the profit for a potential arbitrage opportunity.
// Includes all costs: gas + exchange fees (Uniswap 0.3% + Binance 0.1%)
func (c *ProfitCalculator) Calculate(
	spread pricingDomain.Spread,
	tradeSize decimal.Decimal,
	tradeValueUSD decimal.Decimal,
	gasCost *domain.GasCost,
) *domain.ProfitResult {
	// Gross profit = |price difference| × quantity
	// spread.Absolute is DEX-CEX, can be negative when DEX is cheaper
	grossProfit := spread.Absolute.Abs().Mul(tradeSize)

	// Exchange fees = trade value × fee rate (0.4% total)
	exchangeFees := tradeValueUSD.Mul(TotalFeeRate)

	// Gas cost in USD
	gasCostUSD := gasCost.TotalUSD.ToDecimal()

	// Total costs = gas + exchange fees
	totalCosts := gasCostUSD.Add(exchangeFees)

	// Use the domain helper that handles decimal -> Amount conversion
	result := domain.NewProfitResultWithFees(grossProfit, gasCostUSD, exchangeFees, asset.USD)

	// Check if meets minimum thresholds
	meetsThresholds := spread.BasisPoints.Abs().GreaterThanOrEqual(c.minProfitBps) &&
		result.NetProfit.ToDecimal().GreaterThanOrEqual(c.minProfitUSD)

	// In production (positive thresholds), also require gross > costs
	// In testing (negative thresholds), allow all opportunities through
	if c.minProfitBps.IsNegative() || c.minProfitUSD.IsNegative() {
		// Testing mode: only check thresholds
		result.IsProfitable = meetsThresholds
	} else {
		// Production mode: also require actual profitability
		result.IsProfitable = meetsThresholds && grossProfit.GreaterThan(totalCosts)
	}

	return result
}
