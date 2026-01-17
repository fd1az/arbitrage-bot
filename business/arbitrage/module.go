// Package arbitrage implements the arbitrage bounded context for opportunity detection.
package arbitrage

import (
	"context"
	"strings"

	"github.com/fd1az/arbitrage-bot/business/arbitrage/app"
	arbitrageDI "github.com/fd1az/arbitrage-bot/business/arbitrage/di"
	"github.com/fd1az/arbitrage-bot/business/arbitrage/infra"
	blockchainDI "github.com/fd1az/arbitrage-bot/business/blockchain/di"
	pricingDI "github.com/fd1az/arbitrage-bot/business/pricing/di"
	pricingDomain "github.com/fd1az/arbitrage-bot/business/pricing/domain"
	"github.com/fd1az/arbitrage-bot/internal/asset"
	"github.com/fd1az/arbitrage-bot/internal/config"
	"github.com/fd1az/arbitrage-bot/internal/di"
	"github.com/fd1az/arbitrage-bot/internal/logger"
	"github.com/fd1az/arbitrage-bot/internal/monolith"
)

// Module implements the arbitrage bounded context.
type Module struct{}

// RegisterServices registers all arbitrage services with the DI container.
func (m *Module) RegisterServices(c di.Container) error {
	// Register Reporter - private dependency
	di.RegisterToken(c, arbitrageDI.Reporter, func(sr di.ServiceRegistry) app.Reporter {
		cfg := sr.Get("config").(*config.Config)
		if cfg.Arbitrage.TUIMode {
			return infra.NewTUIReporter()
		}
		return infra.NewConsoleReporter()
	})

	// Register ProfitCalculator - private dependency
	di.RegisterToken(c, arbitrageDI.ProfitCalculator, func(sr di.ServiceRegistry) *app.ProfitCalculator {
		cfg := sr.Get("config").(*config.Config)
		return app.NewProfitCalculator(
			cfg.Arbitrage.MinProfitBpsDecimal(),
			cfg.Arbitrage.MinProfitUSDDecimal(),
		)
	})

	// Register Detector - public service
	di.RegisterToken(c, arbitrageDI.Detector, func(sr di.ServiceRegistry) *app.Detector {
		cfg := sr.Get("config").(*config.Config)
		log := sr.Get("logger").(logger.LoggerInterface)
		registry := sr.Get("assetRegistry").(*asset.Registry)

		blockchain := blockchainDI.GetBlockchainService(sr)
		pricing := pricingDI.GetPricingService(sr)
		calculator := arbitrageDI.GetProfitCalculator(sr)
		reporter := arbitrageDI.GetReporter(sr)

		// Build detector config from app config
		detectorCfg := app.DetectorConfig{
			Pairs:      buildPairs(cfg.Arbitrage.Pairs, registry, log),
			TradeSizes: cfg.Arbitrage.TradeSizesDecimal(),
		}

		return app.NewDetector(blockchain, pricing, calculator, reporter, detectorCfg, log)
	})

	return nil
}

// Startup initializes the arbitrage module.
func (m *Module) Startup(ctx context.Context, mono monolith.Monolith) error {
	mono.Logger().Info(ctx, "arbitrage module started")
	return nil
}

// buildPairs converts config strings to domain pairs using the injected registry.
func buildPairs(pairs []string, registry *asset.Registry, log logger.LoggerInterface) []pricingDomain.Pair {
	result := make([]pricingDomain.Pair, 0, len(pairs))
	ctx := context.Background()

	for _, p := range pairs {
		parts := strings.SplitN(p, "-", 2)
		if len(parts) != 2 {
			log.Warn(ctx, "invalid pair format, skipping", "pair", p)
			continue
		}

		baseSymbol := strings.TrimSpace(parts[0])
		quoteSymbol := strings.TrimSpace(parts[1])

		// Get assets from registry (prefer Ethereum mainnet)
		base, ok := registry.GetBySymbolAndChain(baseSymbol, asset.ChainIDEthereum)
		if !ok {
			assets := registry.GetBySymbol(baseSymbol)
			if len(assets) == 0 {
				log.Warn(ctx, "unknown base asset, skipping pair", "asset", baseSymbol, "pair", p)
				continue
			}
			base = assets[0]
		}

		quote, ok := registry.GetBySymbolAndChain(quoteSymbol, asset.ChainIDEthereum)
		if !ok {
			assets := registry.GetBySymbol(quoteSymbol)
			if len(assets) == 0 {
				log.Warn(ctx, "unknown quote asset, skipping pair", "asset", quoteSymbol, "pair", p)
				continue
			}
			quote = assets[0]
		}

		result = append(result, pricingDomain.NewPair(base, quote))
	}

	return result
}
