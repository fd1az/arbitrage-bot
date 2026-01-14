// Package components provides reusable TUI components.
package components

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ConnectionStatus represents a connection's status.
type ConnectionStatus struct {
	Name      string
	Connected bool
	Latency   time.Duration
	LastBlock uint64
	LastUpdate time.Time
}

// StatusComponent renders connection status.
type StatusComponent struct {
	connections []ConnectionStatus
}

// NewStatusComponent creates a new status component.
func NewStatusComponent() *StatusComponent {
	return &StatusComponent{
		connections: make([]ConnectionStatus, 0),
	}
}

// Update updates a connection's status.
func (s *StatusComponent) Update(status ConnectionStatus) {
	for i, conn := range s.connections {
		if conn.Name == status.Name {
			s.connections[i] = status
			return
		}
	}
	s.connections = append(s.connections, status)
}

// View renders the status component.
func (s *StatusComponent) View() string {
	if len(s.connections) == 0 {
		return "No connections"
	}

	var result string
	for _, conn := range s.connections {
		status := "● Connected"
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
		if !conn.Connected {
			status = "○ Disconnected"
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
		}

		line := fmt.Sprintf("├─ %s: %s", conn.Name, style.Render(status))
		if conn.Connected && conn.Latency > 0 {
			line += fmt.Sprintf(" (%s)", conn.Latency.Round(time.Millisecond))
		}
		result += line + "\n"
	}

	return result
}
