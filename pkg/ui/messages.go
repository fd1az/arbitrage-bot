// Package ui provides the Bubble Tea TUI for the arbitrage bot.
package ui

import (
	"time"

	"github.com/fd1az/arbitrage-bot/business/arbitrage/domain"
	pricingDomain "github.com/fd1az/arbitrage-bot/business/pricing/domain"
)

// Message types for TUI updates

// OpportunityMsg is sent when an arbitrage opportunity is detected.
type OpportunityMsg struct {
	Opportunity *domain.Opportunity
}

// PriceUpdateMsg is sent when prices are updated.
type PriceUpdateMsg struct {
	Snapshot *pricingDomain.PriceSnapshot
}

// ConnectionStatusMsg is sent when connection status changes.
type ConnectionStatusMsg struct {
	Name      string
	Connected bool
	Latency   time.Duration
}

// BlockMsg is sent when a new block is received.
type BlockMsg struct {
	Number    uint64
	Timestamp time.Time
}

// GasPriceMsg is sent when gas price is updated.
type GasPriceMsg struct {
	GweiPrice float64
}

// ErrorMsg is sent when an error occurs.
type ErrorMsg struct {
	Error error
}

// TickMsg is sent periodically for UI updates.
type TickMsg struct{}

// WelcomeCompleteMsg signals the welcome screen is done (timeout or keypress).
type WelcomeCompleteMsg struct{}

// StartModulesMsg signals that modules should start loading.
type StartModulesMsg struct{}

// LogMsg is sent to display a log message in the UI.
type LogMsg struct {
	Level   string // "info", "warn", "error"
	Message string
}

// ScanMsg is sent when a price scan/analysis is performed.
type ScanMsg struct {
	Pair        string
	TradeSize   string
	CEXPrice    float64
	DEXPrice    float64
	SpreadBps   float64
	BlockNumber uint64
}

// StartupMsg is sent during application startup to show progress.
type StartupMsg struct {
	Step    string // Current step name
	Status  string // "connecting", "connected", "failed"
	Message string // Optional message
}

// CostBreakdownMsg is sent with cost analysis for display.
// All values are pre-calculated by the domain - UI should not calculate anything.
type CostBreakdownMsg struct {
	TradeSize     string
	TradeValueUSD float64
	GrossProfit   float64
	GasCostUSD    float64
	ExchangeFees  float64
	TotalCosts    float64
	NetProfit     float64
	IsProfitable  bool
}
