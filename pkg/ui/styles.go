// Package ui provides the Bubble Tea TUI for the arbitrage bot.
package ui

import "github.com/charmbracelet/lipgloss"

// Colors
var (
	ColorPrimary   = lipgloss.Color("#7C3AED") // Purple
	ColorSecondary = lipgloss.Color("#10B981") // Green
	ColorDanger    = lipgloss.Color("#EF4444") // Red
	ColorWarning   = lipgloss.Color("#F59E0B") // Amber
	ColorMuted     = lipgloss.Color("#6B7280") // Gray
	ColorBorder    = lipgloss.Color("#374151") // Dark gray
)

// Styles
var (
	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	// Header style
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Padding(0, 1)

	// Title style
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(ColorPrimary).
			Padding(0, 2)

	// Status styles
	StatusConnected = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	StatusDisconnected = lipgloss.NewStyle().
				Foreground(ColorDanger).
				Bold(true)

	StatusReconnecting = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Bold(true)

	// Value styles
	PositiveValue = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	NegativeValue = lipgloss.NewStyle().
			Foreground(ColorDanger)

	MutedValue = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Table styles
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder())

	TableCellStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// Help style
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Padding(0, 1)
)
