// Package components provides reusable TUI components.
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/shopspring/decimal"
)

// PriceRow represents a row in the price table.
type PriceRow struct {
	TradeSize decimal.Decimal
	CEXPrice  decimal.Decimal
	DEXPrice  decimal.Decimal
	SpreadBps decimal.Decimal
}

// CostBreakdown holds domain-calculated cost data for display.
type CostBreakdown struct {
	TradeSize     string
	TradeValueUSD float64
	GrossProfit   float64
	GasCostUSD    float64
	ExchangeFees  float64
	TotalCosts    float64
	NetProfit     float64
	IsProfitable  bool
}

// PricesComponent renders the price comparison table.
type PricesComponent struct {
	rows          []PriceRow
	pair          string
	gasGwei       float64
	costBreakdown *CostBreakdown // Pre-calculated by domain
}

// NewPricesComponent creates a new prices component.
func NewPricesComponent() *PricesComponent {
	return &PricesComponent{
		rows: make([]PriceRow, 0),
		pair: "ETH-USDC",
	}
}

// Update updates the price data.
func (p *PricesComponent) Update(rows []PriceRow) {
	p.rows = rows
}

// SetPair sets the trading pair name.
func (p *PricesComponent) SetPair(pair string) {
	p.pair = pair
}

// SetGas sets the gas price in gwei.
func (p *PricesComponent) SetGas(gwei float64) {
	p.gasGwei = gwei
}

// SetCostBreakdown sets the domain-calculated cost breakdown.
// UI just displays this data, no calculations needed.
func (p *PricesComponent) SetCostBreakdown(breakdown CostBreakdown) {
	p.costBreakdown = &breakdown
}

// View renders the prices component.
func (p *PricesComponent) View() string {
	if len(p.rows) == 0 {
		return "Waiting for price data..."
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	positiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	negativeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))

	var result string
	result = headerStyle.Render(fmt.Sprintf("PRICES (%s)", p.pair))
	result += "\n\n"

	// Simple aligned table without box drawing
	result += fmt.Sprintf("  %-10s  %14s  %14s  %12s\n",
		"Size", "Binance (CEX)", "Uniswap (DEX)", "Spread")
	result += dimStyle.Render("  " + strings.Repeat("─", 56)) + "\n"

	for _, row := range p.rows {
		spreadStyle := positiveStyle
		if row.SpreadBps.IsNegative() {
			spreadStyle = negativeStyle
		}

		// Format spread value
		spreadVal := row.SpreadBps.InexactFloat64()
		spreadStr := fmt.Sprintf("%+.1f bps", spreadVal)

		result += fmt.Sprintf("  %-10s  %14s  %14s  %s\n",
			row.TradeSize.StringFixed(0)+" ETH",
			"$"+row.CEXPrice.StringFixed(2),
			"$"+row.DEXPrice.StringFixed(2),
			spreadStyle.Render(fmt.Sprintf("%12s", spreadStr)),
		)
	}

	// Cost breakdown section - DISPLAY ONLY, no calculations
	// All values come pre-calculated from the domain
	result += "\n"
	result += dimStyle.Render("  " + strings.Repeat("─", 56)) + "\n"

	if p.costBreakdown != nil {
		cb := p.costBreakdown

		// Dynamic title based on profitability (from domain)
		if cb.IsProfitable {
			result += headerStyle.Render("  OPPORTUNITY FOUND!") + "\n\n"
		} else {
			result += headerStyle.Render("  WHY NO OPPORTUNITY?") + "\n\n"
		}

		result += fmt.Sprintf("  Best trade: %s\n", dimStyle.Render(cb.TradeSize))
		result += fmt.Sprintf("  Trade value: %s\n", dimStyle.Render(fmt.Sprintf("$%.0f", cb.TradeValueUSD)))
		result += fmt.Sprintf("  Gross profit: %s\n", warnStyle.Render(fmt.Sprintf("$%.2f", cb.GrossProfit)))
		result += fmt.Sprintf("  Gas cost: %s\n", negativeStyle.Render(fmt.Sprintf("-$%.2f", cb.GasCostUSD)))
		result += fmt.Sprintf("  Fees (0.4%%): %s\n", negativeStyle.Render(fmt.Sprintf("-$%.2f", cb.ExchangeFees)))

		if cb.IsProfitable {
			result += fmt.Sprintf("  Net profit: %s\n", positiveStyle.Render(fmt.Sprintf("+$%.2f", cb.NetProfit)))
		} else {
			result += fmt.Sprintf("  Net profit: %s\n", negativeStyle.Render(fmt.Sprintf("-$%.2f", cb.TotalCosts-cb.GrossProfit)))
			result += "\n"
			result += dimStyle.Render("  Need ~50+ bps spread for profit") + "\n"
		}
	} else {
		result += dimStyle.Render("  Waiting for cost analysis...") + "\n"
	}

	return result
}
