// Package main is the entry point for the CEX-DEX Arbitrage Bot.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/joho/godotenv"

	"github.com/fd1az/arbitrage-bot/business/arbitrage"
	arbitrageApp "github.com/fd1az/arbitrage-bot/business/arbitrage/app"
	arbitrageDI "github.com/fd1az/arbitrage-bot/business/arbitrage/di"
	"github.com/fd1az/arbitrage-bot/business/blockchain"
	"github.com/fd1az/arbitrage-bot/business/pricing"
	"github.com/fd1az/arbitrage-bot/internal/apm"
	"github.com/fd1az/arbitrage-bot/internal/config"
	"github.com/fd1az/arbitrage-bot/internal/health"
	"github.com/fd1az/arbitrage-bot/internal/logger"
	"github.com/fd1az/arbitrage-bot/internal/metrics"
	"github.com/fd1az/arbitrage-bot/internal/monolith"
	"github.com/fd1az/arbitrage-bot/pkg/ui"
)

var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	// Load .env file if present (ignore error if not found)
	_ = godotenv.Load()

	// Parse flags
	configPath := flag.String("config", "", "Path to configuration file")
	cliMode := flag.Bool("cli", false, "Run in CLI mode with logs (no TUI)")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("arbitrage-bot %s (commit: %s, built: %s)\n", version, commit, buildDate)
		os.Exit(0)
	}

	// TUI is the default, CLI is for debugging
	tuiMode := !*cliMode

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		if !tuiMode {
			fmt.Fprintf(os.Stderr, "received shutdown signal: %v\n", sig)
		}
		cancel()
	}()

	// Run application
	if err := run(ctx, *configPath, tuiMode); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, configPath string, tuiMode bool) error {
	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Set TUI mode in config so modules know
	cfg.Arbitrage.TUIMode = tuiMode

	// Setup logger (only log to stderr in CLI mode)
	logLevel := logger.LevelInfo
	switch cfg.App.LogLevel {
	case "debug":
		logLevel = logger.LevelDebug
	case "warn":
		logLevel = logger.LevelWarn
	case "error":
		logLevel = logger.LevelError
	}

	var log *logger.Logger
	if tuiMode {
		// In TUI mode, suppress logs (discard output)
		log = logger.New(io.Discard, logLevel, cfg.App.Name, nil)
	} else {
		log = logger.New(os.Stderr, logLevel, cfg.App.Name, nil)
		log.Info(ctx, "starting CEX-DEX Arbitrage Bot",
			"version", version,
			"environment", cfg.App.Environment,
		)
	}

	// Initialize observability if enabled
	var traceProvider apm.TraceProvider
	if cfg.Telemetry.Enabled {
		// Set service name env var for OTEL
		if cfg.Telemetry.ServiceName != "" {
			os.Setenv("OTEL_SERVICE_NAME", cfg.Telemetry.ServiceName)
		}
		if cfg.Telemetry.OTLPEndpoint != "" {
			os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", cfg.Telemetry.OTLPEndpoint)
		}

		// Initialize tracing with Zipkin (local dev friendly)
		traceProvider = apm.NewTraceProvider(log, apm.WithProvider(apm.ZipkinProvider, log))
		log.Info(ctx, "tracing initialized", "provider", "zipkin", "endpoint", cfg.Telemetry.OTLPEndpoint)

		// Initialize metrics with Prometheus
		metrics.NewMetricProvider(
			metrics.WithServiceName(cfg.Telemetry.ServiceName),
			metrics.WithProviderConfig(metrics.ProviderCfg{
				Provider: metrics.PrometheusProvider,
			}),
		)

		// Start Prometheus metrics server in background
		port := cfg.Telemetry.PrometheusPort
		if port == 0 {
			port = 9090
		}
		go metrics.ServePrometheusMetrics(metrics.WithPort(strconv.Itoa(port)))
		log.Info(ctx, "prometheus metrics server started", "port", port)
	}
	defer func() {
		if traceProvider != nil {
			traceProvider.Stop()
		}
	}()

	// Start health check server on port 8081
	healthServer := health.NewServer(8081, version)
	if err := healthServer.Start(); err != nil {
		log.Warn(ctx, "failed to start health server", "error", err)
	} else {
		log.Info(ctx, "health server started", "port", 8081)
	}
	defer healthServer.Stop(ctx)

	// Create monolith (application container)
	mono, err := monolith.New(cfg, log)
	if err != nil {
		return fmt.Errorf("failed to create monolith: %w", err)
	}
	defer mono.Close()

	// Define modules in dependency order
	modules := []monolith.Module{
		&blockchain.Module{}, // Must be first - provides block subscription
		&pricing.Module{},    // Depends on blockchain for eth client
		&arbitrage.Module{},  // Depends on blockchain and pricing
	}

	// Register all module services
	if err := mono.RegisterModules(modules...); err != nil {
		return fmt.Errorf("failed to register modules: %w", err)
	}

	if tuiMode {
		// TUI mode: Start modules in background so TUI shows immediately
		startFunc := func() error {
			if err := mono.StartModules(ctx, modules...); err != nil {
				return fmt.Errorf("failed to start modules: %w", err)
			}
			detector := arbitrageDI.GetDetector(mono.Services())
			return detector.Start(ctx)
		}
		stopFunc := func() {
			detector := arbitrageDI.GetDetector(mono.Services())
			detector.Stop()
		}
		return runTUI(ctx, startFunc, stopFunc)
	}

	// CLI mode: Start modules synchronously
	if err := mono.StartModules(ctx, modules...); err != nil {
		return fmt.Errorf("failed to start modules: %w", err)
	}

	// Get detector
	detector := arbitrageDI.GetDetector(mono.Services())
	return runCLI(ctx, detector, log)
}

func runCLI(ctx context.Context, detector *arbitrageApp.Detector, log *logger.Logger) error {
	log.Info(ctx, "all modules started, beginning arbitrage detection")

	// Start the detector
	if err := detector.Start(ctx); err != nil {
		return fmt.Errorf("failed to start detector: %w", err)
	}

	// Wait for shutdown
	<-ctx.Done()

	log.Info(ctx, "shutting down")

	// Stop detector gracefully
	if err := detector.Stop(); err != nil {
		log.Error(ctx, "error stopping detector", "error", err)
	}

	return nil
}

func runTUI(ctx context.Context, startFunc func() error, stopFunc func()) error {
	// Channel to receive StartModulesMsg signal
	startSignal := make(chan struct{}, 1)
	ui.OnStartModules = func() {
		select {
		case startSignal <- struct{}{}:
		default:
		}
	}

	// Create and start the TUI program IMMEDIATELY (shows welcome screen)
	p := tea.NewProgram(ui.New(), tea.WithAltScreen())
	ui.Program = p

	// Run bot logic in background (non-blocking)
	errCh := make(chan error, 1)
	go func() {
		// Wait for welcome screen to complete (StartModulesMsg signal)
		select {
		case <-startSignal:
			// Welcome complete, start modules
		case <-ctx.Done():
			errCh <- nil
			return
		}

		// Start modules and detector (connections happen here, TUI shows progress)
		if err := startFunc(); err != nil {
			ui.Send(ui.ErrorMsg{Error: err})
			errCh <- err
			return
		}

		// Wait for context cancellation
		<-ctx.Done()

		// Stop detector
		stopFunc()
		errCh <- nil
	}()

	// Run TUI (blocking) - shows immediately with welcome screen
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// Check for bot errors
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}
