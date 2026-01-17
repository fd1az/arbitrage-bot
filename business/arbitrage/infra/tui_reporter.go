// Package infra contains infrastructure adapters for the arbitrage context.
package infra

import (
	"context"
	"time"

	"github.com/fd1az/arbitrage-bot/business/arbitrage/app"
	"github.com/fd1az/arbitrage-bot/business/arbitrage/domain"
	pricingDomain "github.com/fd1az/arbitrage-bot/business/pricing/domain"
	"github.com/fd1az/arbitrage-bot/pkg/ui"
)

// TUIReporter implements Reporter for Bubble Tea TUI.
type TUIReporter struct {
	started bool
}

// NewTUIReporter creates a new TUIReporter.
func NewTUIReporter() *TUIReporter {
	return &TUIReporter{}
}

// Start initializes the TUI reporter.
// Note: The actual TUI program should be started separately in main.go
// This reporter just sends messages to the already-running program.
func (r *TUIReporter) Start(ctx context.Context) error {
	r.started = true
	// Send initial startup status
	ui.Send(ui.StartupMsg{Step: "config", Status: "done"})
	return nil
}

// UpdateStartup sends startup progress to the TUI.
func (r *TUIReporter) UpdateStartup(step, status, message string) {
	if !r.started {
		return
	}
	ui.Send(ui.StartupMsg{
		Step:    step,
		Status:  status,
		Message: message,
	})
}

// Report sends an arbitrage opportunity to the TUI.
func (r *TUIReporter) Report(opp *domain.Opportunity) {
	if !r.started {
		return
	}
	ui.Send(ui.OpportunityMsg{Opportunity: opp})
}

// UpdatePrices sends price updates to the TUI.
func (r *TUIReporter) UpdatePrices(prices *pricingDomain.PriceSnapshot) {
	if !r.started {
		return
	}
	ui.Send(ui.PriceUpdateMsg{Snapshot: prices})
}

// UpdateConnectionStatus sends connection status to the TUI.
func (r *TUIReporter) UpdateConnectionStatus(name string, connected bool, latency time.Duration) {
	if !r.started {
		return
	}
	ui.Send(ui.ConnectionStatusMsg{
		Name:      name,
		Connected: connected,
		Latency:   latency,
	})
}

// UpdateBlock sends block number to the TUI.
func (r *TUIReporter) UpdateBlock(blockNumber uint64) {
	if !r.started {
		return
	}
	ui.Send(ui.BlockMsg{
		Number:    blockNumber,
		Timestamp: time.Now(),
	})
}

// UpdateGasPrice sends gas price to the TUI.
func (r *TUIReporter) UpdateGasPrice(gweiPrice float64) {
	if !r.started {
		return
	}
	ui.Send(ui.GasPriceMsg{
		GweiPrice: gweiPrice,
	})
}

// UpdateCostBreakdown sends cost breakdown data to the TUI.
// UI should display this directly without any calculations.
func (r *TUIReporter) UpdateCostBreakdown(breakdown *app.CostBreakdown) {
	if !r.started {
		return
	}
	ui.Send(ui.CostBreakdownMsg{
		TradeSize:     breakdown.TradeSize,
		TradeValueUSD: breakdown.TradeValueUSD.InexactFloat64(),
		GrossProfit:   breakdown.GrossProfit.InexactFloat64(),
		GasCostUSD:    breakdown.GasCostUSD.InexactFloat64(),
		ExchangeFees:  breakdown.ExchangeFees.InexactFloat64(),
		TotalCosts:    breakdown.TotalCosts.InexactFloat64(),
		NetProfit:     breakdown.NetProfit.InexactFloat64(),
		IsProfitable:  breakdown.IsProfitable,
	})
}

// Stop gracefully shuts down the TUI reporter.
func (r *TUIReporter) Stop() error {
	r.started = false
	return nil
}
