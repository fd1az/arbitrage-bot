// Package app contains application services and port definitions for the arbitrage context.
package app

import (
	"context"
	"log/slog"

	blockchainApp "github.com/fd1az/arbitrage-bot/business/blockchain/app"
	blockchainDomain "github.com/fd1az/arbitrage-bot/business/blockchain/domain"
	pricingApp "github.com/fd1az/arbitrage-bot/business/pricing/app"
	pricingDomain "github.com/fd1az/arbitrage-bot/business/pricing/domain"
	"github.com/shopspring/decimal"
)

// DetectorConfig holds configuration for the arbitrage detector.
type DetectorConfig struct {
	Pairs      []pricingDomain.Pair
	TradeSizes []decimal.Decimal
}

// Detector orchestrates arbitrage detection.
type Detector struct {
	blockchain *blockchainApp.BlockchainService
	pricing    *pricingApp.PricingService
	calculator *ProfitCalculator
	reporter   Reporter
	config     DetectorConfig
	logger     *slog.Logger
}

// NewDetector creates a new arbitrage Detector.
func NewDetector(
	blockchain *blockchainApp.BlockchainService,
	pricing *pricingApp.PricingService,
	calculator *ProfitCalculator,
	reporter Reporter,
	config DetectorConfig,
	logger *slog.Logger,
) *Detector {
	return &Detector{
		blockchain: blockchain,
		pricing:    pricing,
		calculator: calculator,
		reporter:   reporter,
		config:     config,
		logger:     logger,
	}
}

// Start begins the arbitrage detection loop.
func (d *Detector) Start(ctx context.Context) error {
	d.logger.Info("starting arbitrage detector")

	// Subscribe to new blocks
	blocks, err := d.blockchain.SubscribeBlocks(ctx)
	if err != nil {
		return err
	}

	// Start reporter
	if err := d.reporter.Start(ctx); err != nil {
		return err
	}

	// Main detection loop
	go d.run(ctx, blocks)

	return nil
}

func (d *Detector) run(ctx context.Context, blocks <-chan *blockchainDomain.Block) {
	for {
		select {
		case <-ctx.Done():
			d.logger.Info("detector stopping", "reason", ctx.Err())
			return
		case block := <-blocks:
			if block != nil {
				d.onNewBlock(ctx, block)
			}
		}
	}
}

func (d *Detector) onNewBlock(ctx context.Context, block *blockchainDomain.Block) {
	// TODO: Implement block processing
	// 1. Fetch prices from CEX and DEX
	// 2. Calculate spreads for each trade size
	// 3. Calculate profit for each spread
	// 4. Report opportunities that meet thresholds
	d.logger.Debug("processing block", "number", block.Number, "hash", block.Hash.Hex())
}

// Stop gracefully shuts down the detector.
func (d *Detector) Stop() error {
	d.logger.Info("stopping arbitrage detector")
	return d.reporter.Stop()
}
