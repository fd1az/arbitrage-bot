// Package infra contains infrastructure adapters for the arbitrage context.
package infra

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/fd1az/arbitrage-bot/business/arbitrage/domain"
	pricingDomain "github.com/fd1az/arbitrage-bot/business/pricing/domain"
)

// ConsoleReporter implements Reporter for CLI output.
type ConsoleReporter struct {
	out io.Writer
}

// NewConsoleReporter creates a new ConsoleReporter.
func NewConsoleReporter() *ConsoleReporter {
	return &ConsoleReporter{
		out: os.Stdout,
	}
}

// Start initializes the console reporter.
func (r *ConsoleReporter) Start(ctx context.Context) error {
	fmt.Fprintln(r.out, "Arbitrage Bot Started")
	fmt.Fprintln(r.out, "======================")
	return nil
}

// Report outputs an arbitrage opportunity to the console.
func (r *ConsoleReporter) Report(opp *domain.Opportunity) {
	fmt.Fprintln(r.out, "")
	fmt.Fprintln(r.out, "================================================================================")
	fmt.Fprintln(r.out, "ARBITRAGE OPPORTUNITY DETECTED")
	fmt.Fprintln(r.out, "================================================================================")
	fmt.Fprintf(r.out, "Block:          #%d\n", opp.BlockNumber)
	fmt.Fprintf(r.out, "Timestamp:      %s\n", opp.Timestamp.Format(time.RFC3339))
	fmt.Fprintf(r.out, "Pair:           %s\n", opp.Pair.String())
	fmt.Fprintf(r.out, "Direction:      %s\n", opp.Direction.String())
	fmt.Fprintln(r.out, "--------------------------------------------------------------------------------")
	fmt.Fprintln(r.out, "PRICES")
	fmt.Fprintf(r.out, "  CEX (Binance):  $%s\n", opp.CEXPrice.StringFixed(2))
	fmt.Fprintf(r.out, "  DEX (Uniswap):  $%s\n", opp.DEXPrice.StringFixed(2))
	fmt.Fprintf(r.out, "  Spread:         %s bps\n", opp.Spread.BasisPoints.StringFixed(2))
	fmt.Fprintln(r.out, "--------------------------------------------------------------------------------")
	fmt.Fprintln(r.out, "TRADE DETAILS")
	fmt.Fprintf(r.out, "  Size:           %s ETH\n", opp.TradeSize.StringFixed(4))
	if opp.GasCost != nil {
		fmt.Fprintf(r.out, "  Gas Cost:       %s ETH ($%s)\n", opp.GasCost.ETH.StringFixed(6), opp.GasCost.USD.StringFixed(2))
	}
	fmt.Fprintln(r.out, "--------------------------------------------------------------------------------")
	fmt.Fprintln(r.out, "PROFIT")
	if opp.Profit != nil {
		fmt.Fprintf(r.out, "  Gross:          $%s\n", opp.Profit.GrossProfit.StringFixed(2))
		fmt.Fprintf(r.out, "  Net:            $%s (%s%%)\n", opp.Profit.NetProfit.StringFixed(2), opp.Profit.NetProfitPct.StringFixed(2))
	}
	fmt.Fprintln(r.out, "================================================================================")
}

// UpdatePrices outputs current prices (no-op for console in detection mode).
func (r *ConsoleReporter) UpdatePrices(prices *pricingDomain.PriceSnapshot) {
	// Console reporter only outputs opportunities, not continuous price updates
}

// UpdateConnectionStatus outputs connection status changes.
func (r *ConsoleReporter) UpdateConnectionStatus(name string, connected bool, latency time.Duration) {
	status := "disconnected"
	if connected {
		status = fmt.Sprintf("connected (%s)", latency)
	}
	fmt.Fprintf(r.out, "[%s] %s: %s\n", time.Now().Format("15:04:05"), name, status)
}

// Stop gracefully shuts down the console reporter.
func (r *ConsoleReporter) Stop() error {
	fmt.Fprintln(r.out, "")
	fmt.Fprintln(r.out, "Arbitrage Bot Stopped")
	return nil
}
