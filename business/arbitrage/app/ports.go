// Package app contains application services and port definitions for the arbitrage context.
package app

import (
	"context"
	"time"

	"github.com/fd1az/arbitrage-bot/business/arbitrage/domain"
	pricingDomain "github.com/fd1az/arbitrage-bot/business/pricing/domain"
)

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

	// Stop gracefully shuts down the reporter.
	Stop() error
}
