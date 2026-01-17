// Package app contains application services and port definitions for the arbitrage context.
package app

import (
	"context"
	"fmt"
	"time"

	blockchainApp "github.com/fd1az/arbitrage-bot/business/blockchain/app"
	blockchainDomain "github.com/fd1az/arbitrage-bot/business/blockchain/domain"
	"github.com/fd1az/arbitrage-bot/business/arbitrage/domain"
	pricingApp "github.com/fd1az/arbitrage-bot/business/pricing/app"
	pricingDomain "github.com/fd1az/arbitrage-bot/business/pricing/domain"
	"github.com/fd1az/arbitrage-bot/internal/logger"
	"github.com/shopspring/decimal"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName = "github.com/fd1az/arbitrage-bot/business/arbitrage/app"
	meterName  = "github.com/fd1az/arbitrage-bot/business/arbitrage/app"
)

// DetectorConfig holds configuration for the arbitrage detector.
type DetectorConfig struct {
	Pairs      []pricingDomain.Pair
	TradeSizes []decimal.Decimal
}

// detectorMetrics holds OTEL metric instruments for the detector.
type detectorMetrics struct {
	opportunitiesAnalyzed  metric.Int64Counter
	opportunitiesProfitable metric.Int64Counter
	spreadBPS              metric.Float64Histogram
	netProfitUSD           metric.Float64Histogram
	analysisLatency        metric.Float64Histogram
}

// Detector orchestrates arbitrage detection.
type Detector struct {
	blockchain *blockchainApp.BlockchainService
	pricing    *pricingApp.PricingService
	calculator *ProfitCalculator
	reporter   Reporter
	config     DetectorConfig
	logger     logger.LoggerInterface

	// OTEL instrumentation
	tracer  trace.Tracer
	metrics *detectorMetrics

	// ETH price in USD for gas cost conversion (updated on each block)
	ethPriceUSD decimal.Decimal
}

// NewDetector creates a new arbitrage Detector.
func NewDetector(
	blockchain *blockchainApp.BlockchainService,
	pricing *pricingApp.PricingService,
	calculator *ProfitCalculator,
	reporter Reporter,
	config DetectorConfig,
	log logger.LoggerInterface,
) *Detector {
	d := &Detector{
		blockchain:  blockchain,
		pricing:     pricing,
		calculator:  calculator,
		reporter:    reporter,
		config:      config,
		logger:      log,
		tracer:      otel.Tracer(tracerName),
		ethPriceUSD: decimal.NewFromInt(3000), // Default, will be updated
	}

	// Initialize metrics (errors are logged but don't fail startup)
	if err := d.initMetrics(); err != nil {
		log.Error(context.Background(), "failed to initialize detector metrics", "error", err)
	}

	return d
}

// initMetrics initializes OTEL metric instruments.
func (d *Detector) initMetrics() error {
	meter := otel.Meter(meterName)
	var err error

	d.metrics = &detectorMetrics{}

	d.metrics.opportunitiesAnalyzed, err = meter.Int64Counter(
		"arbitrage_opportunities_analyzed_total",
		metric.WithDescription("Total number of arbitrage opportunities analyzed"),
		metric.WithUnit("{opportunity}"),
	)
	if err != nil {
		return err
	}

	d.metrics.opportunitiesProfitable, err = meter.Int64Counter(
		"arbitrage_opportunities_profitable_total",
		metric.WithDescription("Total number of profitable arbitrage opportunities detected"),
		metric.WithUnit("{opportunity}"),
	)
	if err != nil {
		return err
	}

	d.metrics.spreadBPS, err = meter.Float64Histogram(
		"arbitrage_spread_bps",
		metric.WithDescription("Arbitrage spread in basis points"),
		metric.WithUnit("{bps}"),
		metric.WithExplicitBucketBoundaries(0, 5, 10, 25, 50, 100, 200, 500, 1000),
	)
	if err != nil {
		return err
	}

	d.metrics.netProfitUSD, err = meter.Float64Histogram(
		"arbitrage_net_profit_usd",
		metric.WithDescription("Net profit in USD (can be negative)"),
		metric.WithUnit("{USD}"),
		metric.WithExplicitBucketBoundaries(-100, -50, -10, 0, 10, 50, 100, 500, 1000),
	)
	if err != nil {
		return err
	}

	d.metrics.analysisLatency, err = meter.Float64Histogram(
		"arbitrage_analysis_latency_ms",
		metric.WithDescription("Time to analyze an opportunity in milliseconds"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(0.1, 0.5, 1, 2, 5, 10, 25, 50, 100),
	)
	if err != nil {
		return err
	}

	return nil
}

// Start begins the arbitrage detection loop.
func (d *Detector) Start(ctx context.Context) error {
	d.logger.Info(ctx, "starting arbitrage detector",
		"pairs", len(d.config.Pairs),
		"trade_sizes", len(d.config.TradeSizes),
	)

	// Start reporter
	if err := d.reporter.Start(ctx); err != nil {
		return err
	}

	// Report initial connecting status
	d.reporter.UpdateConnectionStatus("Ethereum", false, 0)
	d.reporter.UpdateConnectionStatus("Binance", false, 0)

	// Subscribe to new blocks
	blocks, err := d.blockchain.SubscribeBlocks(ctx)
	if err != nil {
		d.logger.Error(ctx, "failed to subscribe to blocks", "error", err)
		return err
	}

	// Report Ethereum connected
	d.reporter.UpdateConnectionStatus("Ethereum", true, 0)

	// Main detection loop
	go d.run(ctx, blocks)

	return nil
}

func (d *Detector) run(ctx context.Context, blocks <-chan *blockchainDomain.Block) {
	for {
		select {
		case <-ctx.Done():
			d.logger.Info(ctx, "detector stopping", "reason", ctx.Err())
			return
		case block := <-blocks:
			if block != nil {
				d.onNewBlock(ctx, block)
			}
		}
	}
}

func (d *Detector) onNewBlock(ctx context.Context, block *blockchainDomain.Block) {
	d.logger.Debug(ctx, "processing block", "number", block.Number, "hash", block.Hash.Hex())

	// Update block in reporter
	d.reporter.UpdateBlock(block.Number)

	// Get current gas price
	gasPrice, err := d.blockchain.GetGasPrice(ctx)
	if err != nil {
		d.logger.Error(ctx, "failed to get gas price", "error", err)
		return
	}

	// Update gas price in reporter (convert wei to gwei)
	gweiPrice := float64(gasPrice.Wei().Int64()) / 1e9
	d.reporter.UpdateGasPrice(gweiPrice)

	// Process each configured pair
	for _, pair := range d.config.Pairs {
		d.processPair(ctx, block, pair, gasPrice)
	}
}

func (d *Detector) processPair(ctx context.Context, block *blockchainDomain.Block, pair pricingDomain.Pair, gasPrice *blockchainDomain.GasPrice) {
	// Track best opportunity across all trade sizes
	var bestBreakdown *CostBreakdown
	var bestGrossProfit decimal.Decimal

	// Process each trade size
	for _, tradeSize := range d.config.TradeSizes {
		opp, breakdown := d.analyzeOpportunity(ctx, block, pair, tradeSize, gasPrice)
		if opp != nil && opp.IsProfitable() {
			d.reporter.Report(opp)
		}
		// Track best breakdown by gross profit (always take first valid, then compare)
		if breakdown != nil {
			if bestBreakdown == nil || breakdown.GrossProfit.GreaterThan(bestGrossProfit) {
				bestBreakdown = breakdown
				bestGrossProfit = breakdown.GrossProfit
			}
		}
	}

	// Send best cost breakdown to UI (not each one individually)
	if bestBreakdown != nil {
		d.reporter.UpdateCostBreakdown(bestBreakdown)
	}
}

func (d *Detector) analyzeOpportunity(
	ctx context.Context,
	block *blockchainDomain.Block,
	pair pricingDomain.Pair,
	tradeSize decimal.Decimal,
	gasPrice *blockchainDomain.GasPrice,
) (*domain.Opportunity, *CostBreakdown) {
	start := time.Now()

	// Start tracing span for opportunity analysis
	ctx, span := d.tracer.Start(ctx, "analyzeOpportunity",
		trace.WithAttributes(
			attribute.String("pair", pair.String()),
			attribute.String("trade_size", tradeSize.String()),
			attribute.Int64("block_number", int64(block.Number)),
		),
	)
	defer span.End()

	// Metric attributes
	metricAttrs := metric.WithAttributes(
		attribute.String("pair", pair.String()),
		attribute.String("trade_size", tradeSize.String()),
	)

	// Get price snapshot from both CEX and DEX
	snapshot, err := d.pricing.GetPriceSnapshot(ctx, pair, tradeSize)
	if err != nil {
		d.logger.Debug(ctx, "failed to get price snapshot",
			"pair", pair.String(),
			"size", tradeSize.String(),
			"error", err,
		)
		// Report Binance disconnected if we can't get prices
		d.reporter.UpdateConnectionStatus("Binance", false, 0)
		span.SetAttributes(attribute.String("error", err.Error()))
		return nil, nil
	}

	// Report Binance connected since we got prices
	d.reporter.UpdateConnectionStatus("Binance", true, 0)

	// Update price display
	d.reporter.UpdatePrices(snapshot)

	// Extract prices
	if snapshot.CEXAsk == nil || snapshot.DEXQuote == nil {
		span.SetAttributes(attribute.Bool("incomplete_snapshot", true))
		return nil, nil
	}

	cexPrice := snapshot.CEXAsk.Rate.Rate() // CEX ask for buying
	dexPrice := snapshot.DEXQuote.Price.Rate()

	// Update ETH price (using CEX price if pair includes ETH)
	if pair.Base.Symbol() == "ETH" {
		d.ethPriceUSD = cexPrice
	}

	// Calculate spread
	spread := pricingDomain.CalculateSpread(cexPrice, dexPrice)

	// Calculate gas cost (estimate ~200k gas for a swap)
	const swapGasLimit = 200_000
	gasCost := domain.NewGasCost(swapGasLimit, gasPrice.Wei(), d.ethPriceUSD)

	// Calculate trade value in USD (for fee calculation)
	tradeValueUSD := cexPrice.Mul(tradeSize)

	// Calculate profit (includes gas + exchange fees)
	// Always calculate this for cost breakdown display
	profit := d.calculator.Calculate(spread, tradeSize, tradeValueUSD, gasCost)

	// Build cost breakdown first (always show analysis even if not profitable)
	breakdown := &CostBreakdown{
		TradeSize:     tradeSize.String() + " ETH",
		TradeValueUSD: tradeValueUSD,
		GrossProfit:   profit.GrossProfit.ToDecimal(),
		GasCostUSD:    profit.GasCost.ToDecimal(),
		ExchangeFees:  profit.ExchangeFees.ToDecimal(),
		TotalCosts:    profit.TotalCosts.ToDecimal(),
		NetProfit:     profit.NetProfitRaw, // Use raw value to preserve sign
		IsProfitable:  profit.IsProfitable,
	}

	// Record spread and profit metrics
	spreadFloat, _ := spread.BasisPoints.Float64()
	netProfitFloat, _ := profit.NetProfit.ToDecimal().Float64()

	if d.metrics != nil {
		d.metrics.spreadBPS.Record(ctx, spreadFloat, metricAttrs)
		d.metrics.netProfitUSD.Record(ctx, netProfitFloat, metricAttrs)
		d.metrics.opportunitiesAnalyzed.Add(ctx, 1, metricAttrs)
	}

	// Add trace attributes for analysis results
	span.SetAttributes(
		attribute.Float64("cex_price", cexPrice.InexactFloat64()),
		attribute.Float64("dex_price", dexPrice.InexactFloat64()),
		attribute.Float64("spread_bps", spreadFloat),
		attribute.Float64("net_profit_usd", netProfitFloat),
		attribute.Bool("profitable", profit.IsProfitable),
	)

	// Determine direction based on spread (for opportunity reporting)
	var direction domain.Direction
	if spread.Direction == pricingDomain.SpreadCEXToDEX {
		direction = domain.DirectionCEXToDEX
	} else if spread.Direction == pricingDomain.SpreadDEXToCEX {
		direction = domain.DirectionDEXToCEX
	} else {
		// No clear direction, but still return breakdown for display
		span.SetAttributes(attribute.String("direction", "none"))
		return nil, breakdown
	}

	// Calculate required capital (trade size * CEX price)
	requiredCapital := tradeSize.Mul(cexPrice)

	// Build opportunity
	opp := &domain.Opportunity{
		ID:              fmt.Sprintf("%d-%s-%s", block.Number, pair.String(), tradeSize.String()),
		BlockNumber:     block.Number,
		Timestamp:       time.Now(),
		Pair:            pair,
		Direction:       direction,
		TradeSize:       tradeSize,
		CEXPrice:        cexPrice,
		DEXPrice:        dexPrice,
		Spread:          spread,
		GasCost:         gasCost,
		Profit:          profit,
		DEXQuote:        snapshot.DEXQuote,
		RequiredCapital: requiredCapital,
	}

	// Add execution steps and risk factors
	opp.ExecutionSteps = d.buildExecutionSteps(opp)
	opp.RiskFactors = d.buildRiskFactors(spread)

	// Record profitable opportunity metric
	if opp.IsProfitable() && d.metrics != nil {
		d.metrics.opportunitiesProfitable.Add(ctx, 1, metricAttrs)
		span.SetAttributes(attribute.Bool("opportunity_detected", true))
	}

	// Record analysis latency
	latencyMs := float64(time.Since(start).Microseconds()) / 1000.0
	if d.metrics != nil {
		d.metrics.analysisLatency.Record(ctx, latencyMs, metricAttrs)
	}

	d.logger.Debug(ctx, "analyzed opportunity",
		"pair", pair.String(),
		"size", tradeSize.String(),
		"spread_bps", spread.BasisPoints.StringFixed(2),
		"profitable", opp.IsProfitable(),
	)

	return opp, breakdown
}

// Stop gracefully shuts down the detector.
func (d *Detector) Stop() error {
	d.logger.Info(context.Background(), "stopping arbitrage detector")
	return d.reporter.Stop()
}

// buildExecutionSteps creates the execution steps for an opportunity.
func (d *Detector) buildExecutionSteps(opp *domain.Opportunity) []domain.ExecutionStep {
	steps := make([]domain.ExecutionStep, 0, 5)

	// Get fee tier percentage for display
	feeTierPct := "0.30%"
	if opp.DEXQuote != nil {
		feeTierPct = opp.DEXQuote.FeeTierPercent()
	}

	// Calculate expected output
	expectedOutput := opp.TradeSize.Mul(opp.DEXPrice)

	if opp.Direction == domain.DirectionCEXToDEX {
		// Buy on CEX, sell on DEX
		steps = append(steps,
			domain.ExecutionStep{
				Number:      1,
				Description: fmt.Sprintf("Buy %s %s on Binance at $%s", opp.TradeSize.StringFixed(4), opp.Pair.Base.Symbol(), opp.CEXPrice.StringFixed(2)),
			},
			domain.ExecutionStep{
				Number:      2,
				Description: fmt.Sprintf("Transfer %s to trading wallet", opp.Pair.Base.Symbol()),
			},
			domain.ExecutionStep{
				Number:      3,
				Description: fmt.Sprintf("Execute Uniswap V3 swap: %s → %s via %s pool", opp.Pair.Base.Symbol(), opp.Pair.Quote.Symbol(), feeTierPct),
			},
			domain.ExecutionStep{
				Number:      4,
				Description: fmt.Sprintf("Receive ~%s %s from swap", expectedOutput.StringFixed(2), opp.Pair.Quote.Symbol()),
			},
			domain.ExecutionStep{
				Number:      5,
				Description: fmt.Sprintf("Transfer %s back to Binance for next cycle", opp.Pair.Quote.Symbol()),
			},
		)
	} else {
		// Buy on DEX, sell on CEX
		steps = append(steps,
			domain.ExecutionStep{
				Number:      1,
				Description: fmt.Sprintf("Execute Uniswap V3 swap: %s → %s via %s pool", opp.Pair.Quote.Symbol(), opp.Pair.Base.Symbol(), feeTierPct),
			},
			domain.ExecutionStep{
				Number:      2,
				Description: fmt.Sprintf("Receive ~%s %s from swap", opp.TradeSize.StringFixed(4), opp.Pair.Base.Symbol()),
			},
			domain.ExecutionStep{
				Number:      3,
				Description: fmt.Sprintf("Transfer %s to Binance", opp.Pair.Base.Symbol()),
			},
			domain.ExecutionStep{
				Number:      4,
				Description: fmt.Sprintf("Sell %s %s on Binance at $%s", opp.TradeSize.StringFixed(4), opp.Pair.Base.Symbol(), opp.CEXPrice.StringFixed(2)),
			},
			domain.ExecutionStep{
				Number:      5,
				Description: fmt.Sprintf("Receive ~%s %s from sale", expectedOutput.StringFixed(2), opp.Pair.Quote.Symbol()),
			},
		)
	}

	return steps
}

// buildRiskFactors creates the risk factors for an opportunity based on spread.
func (d *Detector) buildRiskFactors(spread pricingDomain.Spread) []domain.RiskFactor {
	risks := make([]domain.RiskFactor, 0, 3)

	// Slippage risk - based on spread magnitude
	slippageSeverity := "low"
	if spread.BasisPoints.GreaterThan(decimal.NewFromInt(200)) {
		slippageSeverity = "medium"
	} else if spread.BasisPoints.GreaterThan(decimal.NewFromInt(500)) {
		slippageSeverity = "high"
	}
	risks = append(risks, domain.RiskFactor{
		Name:        "Slippage Risk",
		Description: "Price movement during execution",
		Severity:    slippageSeverity,
	})

	// MEV risk - always medium for any profitable opportunity
	risks = append(risks, domain.RiskFactor{
		Name:        "MEV Risk",
		Description: "Sandwich attacks from MEV bots",
		Severity:    "medium",
	})

	// Timing risk - based on execution complexity
	risks = append(risks, domain.RiskFactor{
		Name:        "Timing Risk",
		Description: "Block confirmation delay",
		Severity:    "low",
	})

	return risks
}
