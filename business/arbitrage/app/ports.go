// Package app contains application services and port definitions for the arbitrage context.
package app

import (
	"context"
	"time"

	"github.com/fd1az/arbitrage-bot/business/arbitrage/domain"
	pricingDomain "github.com/fd1az/arbitrage-bot/business/pricing/domain"
	"github.com/shopspring/decimal"
)

// CostBreakdown contains all cost data for UI display.
// This is a pure data transfer object - UI should not calculate anything.
type CostBreakdown struct {
	TradeSize     string
	TradeValueUSD decimal.Decimal
	GrossProfit   decimal.Decimal
	GasCostUSD    decimal.Decimal
	ExchangeFees  decimal.Decimal
	TotalCosts    decimal.Decimal
	NetProfit     decimal.Decimal
	IsProfitable  bool
}

// Reporter defines the interface for reporting arbitrage opportunities.
type Reporter interface {
	// Start initializes the reporter.
	Start(ctx context.Context) error

	// Report sends an arbitrage opportunity to be displayed/logged.
	Report(opp *domain.Opportunity)

	// UpdatePrices updates the current price display.
	UpdatePrices(prices *pricingDomain.PriceSnapshot)

	// UpdateConnectionStatus updates a connection status display.
	UpdateConnectionStatus(name string, connected bool, latency time.Duration)

	// UpdateBlock updates the current block number.
	UpdateBlock(blockNumber uint64)

	// UpdateGasPrice updates the current gas price in gwei.
	UpdateGasPrice(gweiPrice float64)

	// UpdateCostBreakdown sends calculated cost data to the UI.
	// UI should display this data directly without any calculations.
	UpdateCostBreakdown(breakdown *CostBreakdown)

	// Stop gracefully shuts down the reporter.
	Stop() error
}
