// Package ui provides the Bubble Tea TUI for the arbitrage bot.
package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// Model is the main Bubble Tea model for the TUI.
type Model struct {
	// Sub-components
	status        StatusModel
	prices        PricesModel
	opportunities OpportunitiesModel
	stats         StatsModel

	// State
	ready    bool
	quitting bool
	width    int
	height   int
}

// New creates a new TUI model.
func New() Model {
	return Model{
		status:        NewStatusModel(),
		prices:        NewPricesModel(),
		opportunities: NewOpportunitiesModel(),
		stats:         NewStatsModel(),
	}
}

// Init initializes the TUI model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
	}

	return m, nil
}

// View renders the TUI.
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	if m.quitting {
		return "Goodbye!\n"
	}

	// TODO: Implement full view with all components
	return "CEX-DEX Arbitrage Bot TUI\n\nPress 'q' to quit."
}

// Run starts the Bubble Tea program.
func Run() error {
	p := tea.NewProgram(New(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
