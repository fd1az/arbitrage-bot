// Package infra contains infrastructure adapters for the arbitrage context.
package infra

import (
	"context"
	"time"

	"github.com/fd1az/arbitrage-bot/business/arbitrage/domain"
	pricingDomain "github.com/fd1az/arbitrage-bot/business/pricing/domain"
)

// TUIReporter implements Reporter for Bubble Tea TUI.
type TUIReporter struct {
	// TODO: Add Bubble Tea program reference
}

// NewTUIReporter creates a new TUIReporter.
func NewTUIReporter() *TUIReporter {
	return &TUIReporter{}
}

// Start initializes the TUI reporter and starts the Bubble Tea program.
func (r *TUIReporter) Start(ctx context.Context) error {
	// TODO: Initialize and start Bubble Tea program
	return nil
}

// Report sends an arbitrage opportunity to the TUI.
func (r *TUIReporter) Report(opp *domain.Opportunity) {
	// TODO: Send opportunity to Bubble Tea model via message
}

// UpdatePrices sends price updates to the TUI.
func (r *TUIReporter) UpdatePrices(prices *pricingDomain.PriceSnapshot) {
	// TODO: Send price update to Bubble Tea model via message
}

// UpdateConnectionStatus sends connection status to the TUI.
func (r *TUIReporter) UpdateConnectionStatus(name string, connected bool, latency time.Duration) {
	// TODO: Send connection status to Bubble Tea model via message
}

// Stop gracefully shuts down the TUI reporter.
func (r *TUIReporter) Stop() error {
	// TODO: Stop Bubble Tea program
	return nil
}
