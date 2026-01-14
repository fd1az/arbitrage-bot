// Package components provides reusable TUI components.
package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

// Stats holds statistics for display.
type Stats struct {
	BlocksProcessed   int64
	Opportunities     int64
	Profitable        int64
	AvgLatencyMs      float64
	CacheHitRate      float64
	Errors            int64
}

// StatsComponent renders statistics.
type StatsComponent struct {
	stats Stats
}

// NewStatsComponent creates a new stats component.
func NewStatsComponent() *StatsComponent {
	return &StatsComponent{}
}

// Update updates the statistics.
func (s *StatsComponent) Update(stats Stats) {
	s.stats = stats
}

// View renders the stats component.
func (s *StatsComponent) View() string {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Bold(true)

	profitableRate := float64(0)
	if s.stats.Opportunities > 0 {
		profitableRate = float64(s.stats.Profitable) / float64(s.stats.Opportunities) * 100
	}

	errorsDisplay := valueStyle.Render(fmt.Sprintf("%d", s.stats.Errors))
	if s.stats.Errors > 0 {
		errorsDisplay = errorStyle.Render(fmt.Sprintf("%d", s.stats.Errors))
	}

	return style.Render("STATS") + "\n" +
		fmt.Sprintf("Blocks processed: %s  │  Opportunities: %s  │  Profitable: %s (%.1f%%)\n",
			valueStyle.Render(fmt.Sprintf("%d", s.stats.BlocksProcessed)),
			valueStyle.Render(fmt.Sprintf("%d", s.stats.Opportunities)),
			valueStyle.Render(fmt.Sprintf("%d", s.stats.Profitable)),
			profitableRate,
		) +
		fmt.Sprintf("Avg latency: %s       │  Cache hit rate: %s    │  Errors: %s",
			valueStyle.Render(fmt.Sprintf("%.0fms", s.stats.AvgLatencyMs)),
			valueStyle.Render(fmt.Sprintf("%.1f%%", s.stats.CacheHitRate)),
			errorsDisplay,
		)
}
