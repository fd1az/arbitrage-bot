// Package components provides reusable TUI components.
package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/shopspring/decimal"
)

// OpportunityRow represents an opportunity in the list.
type OpportunityRow struct {
	BlockNumber uint64
	TradeSize   string
	Direction   string
	SpreadBps   decimal.Decimal
	Profit      decimal.Decimal
	Status      string
	Profitable  bool
}

// OpportunitiesComponent renders the opportunities list.
type OpportunitiesComponent struct {
	rows    []OpportunityRow
	maxRows int
}

// NewOpportunitiesComponent creates a new opportunities component.
func NewOpportunitiesComponent(maxRows int) *OpportunitiesComponent {
	return &OpportunitiesComponent{
		rows:    make([]OpportunityRow, 0),
		maxRows: maxRows,
	}
}

// Add adds a new opportunity to the list.
func (o *OpportunitiesComponent) Add(row OpportunityRow) {
	o.rows = append([]OpportunityRow{row}, o.rows...)
	if len(o.rows) > o.maxRows {
		o.rows = o.rows[:o.maxRows]
	}
}

// Clear clears all opportunities.
func (o *OpportunitiesComponent) Clear() {
	o.rows = make([]OpportunityRow, 0)
}

// View renders the opportunities component.
func (o *OpportunitiesComponent) View() string {
	if len(o.rows) == 0 {
		return "No opportunities detected yet..."
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	profitableStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	unprofitableStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))

	result := headerStyle.Render(fmt.Sprintf("OPPORTUNITIES (last %d)\n", o.maxRows))
	result += "┌─────────┬────────┬───────────┬─────────┬──────────┬─────────────────────┐\n"
	result += "│  Block  │  Size  │ Direction │ Spread  │  Profit  │      Status         │\n"
	result += "├─────────┼────────┼───────────┼─────────┼──────────┼─────────────────────┤\n"

	for _, row := range o.rows {
		statusStyle := profitableStyle
		statusIcon := "✓"
		if !row.Profitable {
			statusStyle = unprofitableStyle
			statusIcon = "✗"
		}

		result += fmt.Sprintf("│%8d │%7s │%10s │%8s │%9s │ %s %-17s│\n",
			row.BlockNumber,
			row.TradeSize,
			row.Direction,
			fmt.Sprintf("%+.1fbp", row.SpreadBps.InexactFloat64()),
			fmt.Sprintf("$%.2f", row.Profit.InexactFloat64()),
			statusIcon,
			statusStyle.Render(row.Status),
		)
	}

	result += "└─────────┴────────┴───────────┴─────────┴──────────┴─────────────────────┘"

	return result
}
