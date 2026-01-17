// Package ui provides the Bubble Tea TUI for the arbitrage bot.
package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/fd1az/arbitrage-bot/business/arbitrage/domain"
	"github.com/fd1az/arbitrage-bot/pkg/ui/components"
	"github.com/shopspring/decimal"
)

// ConnectionInfo holds connection state and latency.
type ConnectionInfo struct {
	Connected bool
	Latency   time.Duration
	LastSeen  time.Time
}

// StartupStep represents a step in the startup process.
type StartupStep struct {
	Name   string
	Status string // "pending", "connecting", "connected", "failed"
}

// Phase represents the current UI phase.
type Phase string

const (
	PhaseWelcome   Phase = "welcome"   // Initial welcome screen
	PhaseStartup   Phase = "startup"   // Loading/connecting
	PhaseDashboard Phase = "dashboard" // Main dashboard
)

// WelcomeDuration is how long the welcome screen shows before auto-advancing.
const WelcomeDuration = 2 * time.Second

// ErrorEntry represents an error with timestamp.
type ErrorEntry struct {
	Message   string
	Timestamp time.Time
}

// Model is the main Bubble Tea model for the TUI.
type Model struct {
	// Components
	prices        *components.PricesComponent
	opportunities *components.OpportunitiesComponent

	// Phase state
	phase        Phase
	welcomeStart time.Time

	// State
	ready           bool
	quitting        bool
	paused          bool // Pause detection
	width           int
	height          int
	currentBlock    uint64
	gasPrice        float64
	connectionState map[string]*ConnectionInfo
	lastUpdate      time.Time
	errorMsg        string
	errors          []ErrorEntry // Persistent error panel (last 3)
	logs            []string     // Recent log messages

	// Startup state
	startupComplete bool
	startupSteps    map[string]*StartupStep
	startupTime     time.Time

	// Activity tracking
	scanCount      uint64
	pricesBySize   map[string]components.PriceRow // Trade size -> latest price
	activityFeed   []string                       // Recent activity messages
	lastScanTime   time.Time
	blocksScanned  uint64

	// Cost breakdown (pre-calculated by domain, UI just displays)
	costBreakdown *CostBreakdownMsg
}

// New creates a new TUI model.
func New() Model {
	now := time.Now()
	return Model{
		prices:        components.NewPricesComponent(),
		opportunities: components.NewOpportunitiesComponent(50), // Store more for scrolling
		phase:         PhaseWelcome,
		welcomeStart:  now,
		connectionState: map[string]*ConnectionInfo{
			"Ethereum": {Connected: false},
			"Binance":  {Connected: false},
		},
		logs:         make([]string, 0, 10),
		errors:       make([]ErrorEntry, 0, 3),
		pricesBySize: make(map[string]components.PriceRow),
		activityFeed: make([]string, 0, 8),
		startupSteps: map[string]*StartupStep{
			"config":   {Name: "Loading configuration", Status: "pending"},
			"ethereum": {Name: "Connecting to Ethereum", Status: "pending"},
			"binance":  {Name: "Connecting to Binance", Status: "pending"},
			"uniswap":  {Name: "Initializing Uniswap", Status: "pending"},
		},
		startupTime: now,
	}
}

// Init initializes the TUI model.
func (m Model) Init() tea.Cmd {
	return tickCmd()
}

// tickCmd returns a command that sends a tick every 100ms for smooth animations.
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg{}
	})
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Always allow quit
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
		// During welcome phase, any other key skips to startup
		if m.phase == PhaseWelcome {
			m.phase = PhaseStartup
			m.startupTime = time.Now()
			// Trigger callback directly (don't use Send() from within Update)
			if OnStartModules != nil {
				go OnStartModules()
			}
			return m, tickCmd()
		}
		// Normal key handling
		switch msg.String() {
		case "c":
			m.opportunities.Clear()
			return m, nil
		case "p":
			m.paused = !m.paused
			return m, nil
		case "up", "k":
			m.opportunities.ScrollUp()
			return m, nil
		case "down", "j":
			m.opportunities.ScrollDown()
			return m, nil
		case "e":
			// Clear errors
			m.errors = make([]ErrorEntry, 0, 3)
			m.errorMsg = ""
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

	case TickMsg:
		// Check if welcome timeout has elapsed
		if m.phase == PhaseWelcome && time.Since(m.welcomeStart) >= WelcomeDuration {
			m.phase = PhaseStartup
			m.startupTime = time.Now()
			// Trigger callback directly (don't use Send() from within Update)
			if OnStartModules != nil {
				go OnStartModules()
			}
		}
		return m, tickCmd()

	case OpportunityMsg:
		if msg.Opportunity != nil {
			opp := msg.Opportunity

			// Build execution step rows
			execSteps := make([]components.ExecutionStepRow, 0, len(opp.ExecutionSteps))
			for _, step := range opp.ExecutionSteps {
				execSteps = append(execSteps, components.ExecutionStepRow{
					Number:      step.Number,
					Description: step.Description,
				})
			}

			// Build risk factor rows
			riskFactors := make([]components.RiskFactorRow, 0, len(opp.RiskFactors))
			for _, risk := range opp.RiskFactors {
				riskFactors = append(riskFactors, components.RiskFactorRow{
					Name:     risk.Name,
					Severity: risk.Severity,
				})
			}

			// Get pool fee tier
			poolFeeTier := "0.30%"
			if opp.DEXQuote != nil {
				poolFeeTier = opp.DEXQuote.FeeTierPercent()
			}

			row := components.OpportunityRow{
				Timestamp:       opp.Timestamp.Format("15:04:05"),
				BlockNumber:     opp.BlockNumber,
				Pair:            opp.Pair.String(),
				TradeSize:       opp.TradeSize.String() + " ETH",
				Direction:       opp.Direction.ShortString(),
				SpreadBps:       opp.Spread.BasisPoints,
				Profit:          opp.Profit.NetProfitRaw, // Use raw value to preserve sign
				PoolFeeTier:     poolFeeTier,
				RequiredCapital: opp.RequiredCapital,
				CEXPrice:        opp.CEXPrice,
				ExecutionSteps:  execSteps,
				RiskFactors:     riskFactors,
				Profitable:      opp.IsProfitable(),
				Status:          getOpportunityStatus(opp),
			}
			m.opportunities.Add(row)
			m.lastUpdate = time.Now()
		}

	case PriceUpdateMsg:
		if msg.Snapshot != nil {
			s := msg.Snapshot
			m.prices.SetPair(s.Pair.String())

			// Build price row from snapshot
			cexPrice := decimal.Zero
			dexPrice := decimal.Zero
			spreadBps := decimal.Zero

			if s.CEXAsk != nil {
				cexPrice = s.CEXAsk.Rate.Rate()
			}
			if s.DEXQuote != nil {
				dexPrice = s.DEXQuote.Price.Rate()
				if !cexPrice.IsZero() {
					spreadBps = dexPrice.Sub(cexPrice).Div(cexPrice).Mul(decimal.NewFromInt(10000))
				}
			}

			// Get trade size from DEX quote (this is the requested size, not filled)
			tradeSize := decimal.NewFromFloat(1.0)
			if s.DEXQuote != nil {
				tradeSize = s.DEXQuote.AmountIn.ToDecimal()
			} else if s.CEXAsk != nil {
				tradeSize = s.CEXAsk.Size.ToDecimal()
			}

			// Accumulate prices by trade size
			sizeKey := tradeSize.StringFixed(0)
			m.pricesBySize[sizeKey] = components.PriceRow{
				TradeSize: tradeSize,
				CEXPrice:  cexPrice,
				DEXPrice:  dexPrice,
				SpreadBps: spreadBps,
			}

			// Update prices component with all accumulated sizes
			rows := make([]components.PriceRow, 0, len(m.pricesBySize))
			for _, key := range []string{"1", "10", "100"} {
				if row, ok := m.pricesBySize[key]; ok {
					rows = append(rows, row)
				}
			}
			m.prices.Update(rows)

			// Increment scan count
			m.scanCount++
			m.lastScanTime = time.Now()
			m.lastUpdate = time.Now()
		}

	case ScanMsg:
		// Add to activity feed
		activity := fmt.Sprintf("%s %s: CEX $%.2f | DEX $%.2f | %+.1f bps",
			msg.TradeSize, msg.Pair, msg.CEXPrice, msg.DEXPrice, msg.SpreadBps)
		m.activityFeed = addActivity(m.activityFeed, activity)
		m.scanCount++
		m.lastScanTime = time.Now()
		m.lastUpdate = time.Now()

	case ConnectionStatusMsg:
		m.connectionState[msg.Name] = &ConnectionInfo{
			Connected: msg.Connected,
			Latency:   msg.Latency,
			LastSeen:  time.Now(),
		}
		m.lastUpdate = time.Now()

		// Update startup steps based on connection
		stepKey := strings.ToLower(msg.Name)
		if step, ok := m.startupSteps[stepKey]; ok {
			if msg.Connected {
				step.Status = "connected"
			} else {
				step.Status = "connecting"
			}
		}
		// Also mark config and uniswap as done if we get any connection
		if m.startupSteps["config"] != nil {
			m.startupSteps["config"].Status = "done"
		}
		if m.startupSteps["uniswap"] != nil && m.connectionState["Ethereum"] != nil && m.connectionState["Ethereum"].Connected {
			m.startupSteps["uniswap"].Status = "done"
		}

	case BlockMsg:
		m.currentBlock = msg.Number
		m.blocksScanned++
		m.lastUpdate = time.Now()
		// Add to activity feed
		activity := fmt.Sprintf("Block #%d received", msg.Number)
		m.activityFeed = addActivity(m.activityFeed, activity)

	case GasPriceMsg:
		m.gasPrice = msg.GweiPrice
		m.lastUpdate = time.Now()

	case ErrorMsg:
		m.errorMsg = msg.Error.Error()
		m.logs = addLog(m.logs, "error", msg.Error.Error())
		// Add to persistent errors (keep last 3)
		m.errors = append(m.errors, ErrorEntry{
			Message:   msg.Error.Error(),
			Timestamp: time.Now(),
		})
		if len(m.errors) > 3 {
			m.errors = m.errors[len(m.errors)-3:]
		}

	case LogMsg:
		m.logs = addLog(m.logs, msg.Level, msg.Message)

	case StartupMsg:
		if step, ok := m.startupSteps[msg.Step]; ok {
			step.Status = msg.Status
		}
		// Check if all steps are complete
		allConnected := true
		for _, step := range m.startupSteps {
			if step.Status != "connected" && step.Status != "done" {
				allConnected = false
				break
			}
		}
		if allConnected {
			m.startupComplete = true
		}

	case CostBreakdownMsg:
		// Store the domain-calculated cost breakdown for display
		m.costBreakdown = &msg
		// Pass to prices component for rendering (convert message to component type)
		m.prices.SetCostBreakdown(components.CostBreakdown{
			TradeSize:     msg.TradeSize,
			TradeValueUSD: msg.TradeValueUSD,
			GrossProfit:   msg.GrossProfit,
			GasCostUSD:    msg.GasCostUSD,
			ExchangeFees:  msg.ExchangeFees,
			TotalCosts:    msg.TotalCosts,
			NetProfit:     msg.NetProfit,
			IsProfitable:  msg.IsProfitable,
		})
	}

	return m, nil
}

func getOpportunityStatus(opp *domain.Opportunity) string {
	if opp.IsProfitable() {
		return "PROFITABLE"
	}
	return "Not profitable"
}

// addLog adds a log message and returns the updated slice (keeps last 5).
func addLog(logs []string, level, message string) []string {
	timestamp := time.Now().Format("15:04:05")
	logLine := fmt.Sprintf("[%s] %s: %s", timestamp, level, message)
	logs = append(logs, logLine)
	if len(logs) > 5 {
		logs = logs[len(logs)-5:]
	}
	return logs
}

// addActivity adds an activity message and returns the updated slice (keeps last 6).
func addActivity(feed []string, message string) []string {
	timestamp := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s", timestamp, message)
	feed = append(feed, line)
	if len(feed) > 6 {
		feed = feed[len(feed)-6:]
	}
	return feed
}

// View renders the TUI.
func (m Model) View() string {
	if m.quitting {
		return "\n  Goodbye!\n\n"
	}

	// Phase-based rendering
	switch m.phase {
	case PhaseWelcome:
		return m.renderWelcomeScreen()
	case PhaseStartup:
		// Show startup until first block or all connected
		if m.currentBlock == 0 && !m.startupComplete {
			return m.renderStartupScreen()
		}
		// Transition to dashboard when ready
		m.phase = PhaseDashboard
		fallthrough
	case PhaseDashboard:
		// Continue to main dashboard
	}

	var b strings.Builder

	// Title
	title := TitleStyle.Render(" 🤖 CEX-DEX Arbitrage Bot ")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Status bar
	b.WriteString(m.renderStatusBar())
	b.WriteString("\n\n")

	// Main content: prices on left, activity + opportunities on right
	leftCol := m.prices.View()

	// Right column: activity feed + opportunities
	var rightContent strings.Builder
	rightContent.WriteString(m.renderActivityFeed())
	rightContent.WriteString("\n\n")
	rightContent.WriteString(m.opportunities.View())
	rightCol := rightContent.String()

	// Side by side if enough width
	if m.width > 100 {
		left := BoxStyle.Width(m.width/2 - 2).Render(leftCol)
		right := BoxStyle.Width(m.width/2 - 2).Render(rightCol)
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, right))
	} else {
		b.WriteString(BoxStyle.Width(m.width - 4).Render(leftCol))
		b.WriteString("\n")
		b.WriteString(BoxStyle.Width(m.width - 4).Render(rightCol))
	}

	b.WriteString("\n\n")

	// Persistent error panel (show last 3 errors)
	if len(m.errors) > 0 {
		errorStyle := lipgloss.NewStyle().Foreground(ColorDanger)
		errorHeader := lipgloss.NewStyle().Bold(true).Foreground(ColorDanger)
		mutedError := lipgloss.NewStyle().Foreground(lipgloss.Color("#9CA3AF"))

		b.WriteString(errorHeader.Render("ERRORS"))
		b.WriteString(mutedError.Render(fmt.Sprintf(" (e: clear)")))
		b.WriteString("\n")
		for _, err := range m.errors {
			ago := time.Since(err.Timestamp).Round(time.Second)
			b.WriteString(errorStyle.Render(fmt.Sprintf("  • %s ", err.Message)))
			b.WriteString(mutedError.Render(fmt.Sprintf("(%s ago)", ago)))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Help
	helpText := "q: quit • c: clear • p: pause • ↑↓: scroll"
	if m.paused {
		pauseStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F59E0B"))
		b.WriteString(pauseStyle.Render("⏸ PAUSED"))
		b.WriteString(" • ")
	}
	b.WriteString(HelpStyle.Render(helpText))

	return b.String()
}

// renderActivityFeed renders the recent activity feed.
func (m Model) renderActivityFeed() string {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	blockStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA"))

	var sb strings.Builder
	sb.WriteString(headerStyle.Render("LIVE ACTIVITY"))
	sb.WriteString("\n\n")

	if len(m.activityFeed) == 0 {
		sb.WriteString(mutedStyle.Render("  Waiting for blocks..."))
	} else {
		for _, activity := range m.activityFeed {
			// Color block numbers differently
			if strings.Contains(activity, "Block #") {
				sb.WriteString(blockStyle.Render("  " + activity))
			} else {
				sb.WriteString(mutedStyle.Render("  " + activity))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// renderWelcomeScreen renders the animated welcome screen.
func (m Model) renderWelcomeScreen() string {
	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED"))

	goldStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#F59E0B"))

	mutedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6B7280"))

	greenStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981"))

	// Animated dots based on time
	elapsed := time.Since(m.welcomeStart)
	dotCount := int(elapsed.Milliseconds()/300) % 4
	dots := strings.Repeat(".", dotCount)

	var sb strings.Builder

	// Center the content vertically
	sb.WriteString("\n\n\n\n")

	// ASCII art logo
	logo := `
    ██████╗███████╗██╗  ██╗    ██████╗ ███████╗██╗  ██╗
   ██╔════╝██╔════╝╚██╗██╔╝    ██╔══██╗██╔════╝╚██╗██╔╝
   ██║     █████╗   ╚███╔╝ ────██║  ██║█████╗   ╚███╔╝
   ██║     ██╔══╝   ██╔██╗     ██║  ██║██╔══╝   ██╔██╗
   ╚██████╗███████╗██╔╝ ██╗    ██████╔╝███████╗██╔╝ ██╗
    ╚═════╝╚══════╝╚═╝  ╚═╝    ╚═════╝ ╚══════╝╚═╝  ╚═╝
`
	sb.WriteString(titleStyle.Render(logo))
	sb.WriteString("\n")

	// Subtitle
	subtitle := "               A R B I T R A G E   B O T"
	sb.WriteString(mutedStyle.Render(subtitle))
	sb.WriteString("\n\n\n")

	// Tagline with gold styling
	tagline := "              💰  Let's make money  💰"
	sb.WriteString(goldStyle.Render(tagline))
	sb.WriteString("\n\n\n")

	// Loading indicator
	loading := fmt.Sprintf("                  Initializing%s", dots)
	sb.WriteString(greenStyle.Render(loading))
	sb.WriteString("\n\n")

	// Skip hint
	hint := "            Press any key to skip, or wait..."
	sb.WriteString(mutedStyle.Render(hint))
	sb.WriteString("\n")

	return sb.String()
}

// renderStartupScreen renders the loading/startup screen.
func (m Model) renderStartupScreen() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#7C3AED")).
		MarginBottom(1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF"))

	mutedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	connectingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	failedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))

	var sb strings.Builder

	sb.WriteString("\n\n")
	sb.WriteString(titleStyle.Render("  🤖 CEX-DEX Arbitrage Bot"))
	sb.WriteString("\n\n")
	sb.WriteString(headerStyle.Render("  Starting up..."))
	sb.WriteString("\n\n")

	// Show startup steps in order
	stepOrder := []string{"config", "ethereum", "binance", "uniswap"}
	for _, key := range stepOrder {
		step, ok := m.startupSteps[key]
		if !ok {
			continue
		}

		var icon, statusText string
		var style lipgloss.Style

		switch step.Status {
		case "connected", "done":
			icon = "✓"
			statusText = "Ready"
			style = successStyle
		case "connecting":
			// Animated spinner based on time
			spinners := []string{"◐", "◓", "◑", "◒"}
			idx := int(time.Since(m.startupTime).Milliseconds()/200) % len(spinners)
			icon = spinners[idx]
			statusText = "Connecting..."
			style = connectingStyle
		case "failed":
			icon = "✗"
			statusText = "Failed"
			style = failedStyle
		default:
			icon = "○"
			statusText = "Pending"
			style = mutedStyle
		}

		sb.WriteString(fmt.Sprintf("  %s %s %s\n",
			style.Render(icon),
			mutedStyle.Render(step.Name),
			style.Render(statusText),
		))
	}

	sb.WriteString("\n")
	elapsed := time.Since(m.startupTime).Round(time.Second)
	sb.WriteString(mutedStyle.Render(fmt.Sprintf("  Elapsed: %s", elapsed)))
	sb.WriteString("\n\n")

	// Connection tips
	sb.WriteString(mutedStyle.Render("  Waiting for first Ethereum block..."))
	sb.WriteString("\n")

	return sb.String()
}

func (m Model) renderStatusBar() string {
	var parts []string

	// Scanning indicator (animated when recently scanned)
	if time.Since(m.lastScanTime) < 500*time.Millisecond {
		spinners := []string{"⟳", "◐", "◓", "◑", "◒"}
		idx := int(time.Now().UnixMilli()/100) % len(spinners)
		scanningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Bold(true)
		parts = append(parts, scanningStyle.Render(spinners[idx]+" Scanning"))
	}

	// Block number
	blockStr := fmt.Sprintf("Block: #%d", m.currentBlock)
	parts = append(parts, blockStr)

	// Gas price
	if m.gasPrice > 0 {
		gasStr := fmt.Sprintf("Gas: %.1f gwei", m.gasPrice)
		parts = append(parts, gasStr)
	}

	// Scan stats
	if m.scanCount > 0 {
		scanStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
		parts = append(parts, scanStyle.Render(fmt.Sprintf("Scans: %d", m.scanCount)))
	}

	// Connection status
	for name, info := range m.connectionState {
		var statusStyle lipgloss.Style
		var icon string
		var status string
		if info != nil && info.Connected {
			statusStyle = StatusConnected
			icon = "●"
			if info.Latency > 0 {
				status = fmt.Sprintf("%s (%dms)", name, info.Latency.Milliseconds())
			} else {
				status = name
			}
		} else {
			statusStyle = StatusDisconnected
			icon = "○"
			status = name + " (disconnected)"
		}
		parts = append(parts, statusStyle.Render(icon+" "+status))
	}

	// Last update with activity indicator
	if !m.lastUpdate.IsZero() {
		ago := time.Since(m.lastUpdate).Round(time.Second)
		indicator := ""
		if ago < 2*time.Second {
			indicator = "▪" // Recent activity indicator
		}
		parts = append(parts, MutedValue.Render(fmt.Sprintf("Updated: %s ago %s", ago, indicator)))
	}

	return strings.Join(parts, "  │  ")
}

// Program holds the Bubble Tea program instance for external access.
var Program *tea.Program

// OnStartModules is called when the welcome screen completes and modules should start.
// This is set by main.go to signal when to begin loading modules.
var OnStartModules func()

// Run starts the Bubble Tea program.
func Run() error {
	Program = tea.NewProgram(New(), tea.WithAltScreen())
	_, err := Program.Run()
	return err
}

// Send sends a message to the running program.
func Send(msg tea.Msg) {
	if Program != nil {
		Program.Send(msg)
	}
	// Call OnStartModules callback when StartModulesMsg is sent
	if _, ok := msg.(StartModulesMsg); ok && OnStartModules != nil {
		OnStartModules()
	}
}
