// Package main is the entry point for the CEX-DEX Arbitrage Bot.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/fd1az/arbitrage-bot/internal/config"
	"github.com/fd1az/arbitrage-bot/pkg/ui"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	tuiMode := flag.Bool("tui", false, "Enable TUI mode")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("arbitrage-bot %s (commit: %s, built: %s)\n", version, commit, buildDate)
		os.Exit(0)
	}

	// Setup logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Override TUI mode from flag
	if *tuiMode {
		cfg.UI.TUI = true
	}

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logger.Info("received shutdown signal", "signal", sig)
		cancel()
	}()

	// Run application
	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("application error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg *config.Config, logger *slog.Logger) error {
	logger.Info("starting CEX-DEX Arbitrage Bot",
		"tui", cfg.UI.TUI,
	)

	if cfg.UI.TUI {
		// Run TUI mode
		return ui.Run()
	}

	// CLI mode
	logger.Info("running in CLI mode")

	// TODO: Initialize and wire all components:
	// 1. Initialize blockchain service (Ethereum client)
	// 2. Initialize pricing service (Binance + Uniswap)
	// 3. Initialize arbitrage detector
	// 4. Initialize console reporter
	// 5. Start detector
	// 6. Wait for context cancellation

	<-ctx.Done()
	logger.Info("shutting down")

	return nil
}
