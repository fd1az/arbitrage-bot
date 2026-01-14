// Package app contains application services and port definitions for the arbitrage context.
package app

import (
	"github.com/fd1az/arbitrage-bot/business/arbitrage/domain"
	pricingDomain "github.com/fd1az/arbitrage-bot/business/pricing/domain"
	"github.com/shopspring/decimal"
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
func (c *ProfitCalculator) Calculate(
	spread pricingDomain.Spread,
	tradeSize decimal.Decimal,
	gasCost *domain.GasCost,
) *domain.ProfitResult {
	// Gross profit = spread * trade size
	grossProfit := spread.Absolute.Mul(tradeSize)

	// Net profit = gross - gas cost
	netProfit := grossProfit.Sub(gasCost.USD)

	// Calculate net profit percentage
	tradeValue := spread.CEXPrice.Mul(tradeSize)
	netProfitPct := decimal.Zero
	if !tradeValue.IsZero() {
		netProfitPct = netProfit.Div(tradeValue).Mul(decimal.NewFromInt(100))
	}

	// Determine if profitable
	isProfitable := spread.BasisPoints.Abs().GreaterThanOrEqual(c.minProfitBps) &&
		netProfit.GreaterThanOrEqual(c.minProfitUSD)

	return &domain.ProfitResult{
		GrossProfit:  grossProfit,
		GasCost:      gasCost.USD,
		NetProfit:    netProfit,
		NetProfitPct: netProfitPct,
		IsProfitable: isProfitable,
	}
}
