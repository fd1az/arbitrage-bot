// Package components provides reusable TUI components.
package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/shopspring/decimal"
)

// ExecutionStepRow represents an execution step for display.
type ExecutionStepRow struct {
	Number      int
	Description string
}

// RiskFactorRow represents a risk factor for display.
type RiskFactorRow struct {
	Name     string
	Severity string
}

// OpportunityRow represents an opportunity in the list.
type OpportunityRow struct {
	Timestamp       string
	BlockNumber     uint64
	Pair            string
	TradeSize       string
	Direction       string
	SpreadBps       decimal.Decimal
	Profit          decimal.Decimal
	PoolFeeTier     string
	RequiredCapital decimal.Decimal
	CEXPrice        decimal.Decimal
	ExecutionSteps  []ExecutionStepRow
	RiskFactors     []RiskFactorRow
	Status          string
	Profitable      bool
}

// OpportunitiesComponent renders the opportunities list.
type OpportunitiesComponent struct {
	rows       []OpportunityRow
	maxRows    int
	offset     int // For scrolling
	visibleMax int // How many to show at once
	maxHeight  int // Max lines to render
}

// NewOpportunitiesComponent creates a new opportunities component.
func NewOpportunitiesComponent(maxRows int) *OpportunitiesComponent {
	return &OpportunitiesComponent{
		rows:       make([]OpportunityRow, 0),
		maxRows:    maxRows,
		offset:     0,
		visibleMax: 3, // Show max 3 opportunities at once
		maxHeight:  25, // Max lines to render
	}
}

// Add adds a new opportunity to the list.
func (o *OpportunitiesComponent) Add(row OpportunityRow) {
	o.rows = append([]OpportunityRow{row}, o.rows...)
	if len(o.rows) > o.maxRows {
		o.rows = o.rows[:o.maxRows]
	}
	// Reset scroll to top on new opportunity
	o.offset = 0
}

// Clear clears all opportunities.
func (o *OpportunitiesComponent) Clear() {
	o.rows = make([]OpportunityRow, 0)
	o.offset = 0
}

// ScrollUp scrolls the list up.
func (o *OpportunitiesComponent) ScrollUp() {
	if o.offset > 0 {
		o.offset--
	}
}

// ScrollDown scrolls the list down.
func (o *OpportunitiesComponent) ScrollDown() {
	maxOffset := len(o.rows) - o.visibleMax
	if maxOffset < 0 {
		maxOffset = 0
	}
	if o.offset < maxOffset {
		o.offset++
	}
}

// Count returns the total number of opportunities.
func (o *OpportunitiesComponent) Count() int {
	return len(o.rows)
}

// View renders the opportunities component.
func (o *OpportunitiesComponent) View() string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	profitStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
	scrollHint := lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA"))
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))

	var result string
	result = headerStyle.Render("OPPORTUNITIES")

	// Show count and scroll position
	if len(o.rows) > 0 {
		countStr := fmt.Sprintf(" (%d total, ↑↓ scroll)", len(o.rows))
		result += mutedStyle.Render(countStr)
	}
	result += "\n\n"

	if len(o.rows) == 0 {
		result += mutedStyle.Render("  No opportunities detected yet.\n")
		result += mutedStyle.Render("  Monitoring spreads...\n")
		return result
	}

	// Scroll indicator (top)
	if o.offset > 0 {
		result += scrollHint.Render(fmt.Sprintf("  ▲ %d above\n", o.offset))
	}

	// Render visible rows (compact format: 4 lines per opportunity)
	end := o.offset + o.visibleMax
	if end > len(o.rows) {
		end = len(o.rows)
	}

	for i := o.offset; i < end; i++ {
		row := o.rows[i]
		icon := "●"
		style := profitStyle
		if !row.Profitable {
			icon = "○"
			style = mutedStyle
		}

		// Line 1: icon [time] Pair | Direction | Size
		result += fmt.Sprintf("  %s [%s] %s | %s | %s\n",
			style.Render(icon),
			row.Timestamp,
			row.Pair,
			row.Direction,
			row.TradeSize,
		)

		// Line 2: Spread | Net | Pool | Capital
		result += fmt.Sprintf("    Spread: %.1f bps | Net: %s | Pool: %s\n",
			row.SpreadBps.InexactFloat64(),
			style.Render(fmt.Sprintf("$%.0f", row.Profit.InexactFloat64())),
			row.PoolFeeTier,
		)

		// Line 3: Risks (compact)
		if len(row.RiskFactors) > 0 {
			result += dimStyle.Render("    Risks: ")
			for j, risk := range row.RiskFactors {
				sev := risk.Severity
				if sev == "medium" {
					sev = "med"
				}
				name := risk.Name
				if len(name) > 8 {
					name = name[:8]
				}
				if j > 0 {
					result += " "
				}
				result += dimStyle.Render(fmt.Sprintf("%s(%s)", name, sev))
			}
			result += "\n"
		}

		// Separator between opportunities
		if i < end-1 {
			result += dimStyle.Render("    ─────────────────────────────────\n")
		}
	}

	// Scroll indicator (bottom)
	if end < len(o.rows) {
		result += scrollHint.Render(fmt.Sprintf("\n  ▼ %d more below\n", len(o.rows)-end))
	}

	return result
}
