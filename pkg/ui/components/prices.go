// Package components provides reusable TUI components.
package components

import (
	"fmt"

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

// PricesComponent renders the price comparison table.
type PricesComponent struct {
	rows []PriceRow
	pair string
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

// View renders the prices component.
func (p *PricesComponent) View() string {
	if len(p.rows) == 0 {
		return "Waiting for price data..."
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	positiveStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	negativeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))

	result := headerStyle.Render(fmt.Sprintf("PRICES (%s)\n", p.pair))
	result += "┌─────────────┬──────────────┬──────────────┬──────────────┐\n"
	result += "│ Trade Size  │ Binance (CEX)│ Uniswap (DEX)│   Spread     │\n"
	result += "├─────────────┼──────────────┼──────────────┼──────────────┤\n"

	for _, row := range p.rows {
		spreadStyle := positiveStyle
		if row.SpreadBps.IsNegative() {
			spreadStyle = negativeStyle
		}

		result += fmt.Sprintf("│ %9s   │   $%s  │   $%s  │   %s │\n",
			row.TradeSize.String()+" ETH",
			row.CEXPrice.StringFixed(2),
			row.DEXPrice.StringFixed(2),
			spreadStyle.Render(fmt.Sprintf("%+.1f bps", row.SpreadBps.InexactFloat64())),
		)
	}

	result += "└─────────────┴──────────────┴──────────────┴──────────────┘"

	return result
}
