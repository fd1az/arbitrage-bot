// Package pricing implements the pricing bounded context for CEX/DEX price comparison.
package pricing

import (
	"context"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/fd1az/arbitrage-bot/business/pricing/app"
	pricingDI "github.com/fd1az/arbitrage-bot/business/pricing/di"
	"github.com/fd1az/arbitrage-bot/business/pricing/infra/binance"
	"github.com/fd1az/arbitrage-bot/business/pricing/infra/uniswap"
	"github.com/fd1az/arbitrage-bot/internal/config"
	"github.com/fd1az/arbitrage-bot/internal/di"
	"github.com/fd1az/arbitrage-bot/internal/logger"
	"github.com/fd1az/arbitrage-bot/internal/monolith"
)

// Module implements the pricing bounded context.
type Module struct{}

// RegisterServices registers all pricing services with the DI container.
func (m *Module) RegisterServices(c di.Container) error {
	// Register CEXProvider (Binance) - private dependency
	di.RegisterToken(c, pricingDI.CEXProvider, func(sr di.ServiceRegistry) app.CEXProvider {
		cfg := sr.Get("config").(*config.Config)
		log := sr.Get("logger").(logger.LoggerInterface)

		providerCfg := binance.ProviderConfig{
			WebSocketURL:  cfg.Binance.WebSocketURL,
			Symbols:       cfg.Binance.Symbols,
			DepthSpeedMs:  cfg.Binance.DepthSpeedMs,
			SnapshotDepth: 20,
			StaleTimeout:  cfg.Binance.StaleTimeout,
		}

		provider, err := binance.NewProvider(providerCfg, log)
		if err != nil {
			panic("failed to create binance provider: " + err.Error())
		}
		return provider
	})

	// Register DEXProvider (Uniswap) - private dependency
	di.RegisterToken(c, pricingDI.DEXProvider, func(sr di.ServiceRegistry) app.DEXProvider {
		cfg := sr.Get("config").(*config.Config)
		log := sr.Get("logger").(logger.LoggerInterface)
		ethClient := sr.Get("ethClient").(*ethclient.Client)

		provider, err := uniswap.NewProvider(ethClient, cfg.Uniswap, log)
		if err != nil {
			panic("failed to create uniswap provider: " + err.Error())
		}
		return provider
	})

	// Register PricingService (public - exposed to other modules)
	di.RegisterToken(c, pricingDI.PricingService, func(sr di.ServiceRegistry) *app.PricingService {
		cex := pricingDI.GetCEXProvider(sr)
		dex := pricingDI.GetDEXProvider(sr)
		return app.NewPricingService(cex, dex)
	})

	return nil
}

// Startup initializes the pricing module.
func (m *Module) Startup(ctx context.Context, mono monolith.Monolith) error {
	log := mono.Logger()

	// Connect Binance provider (don't fail if connection fails - will retry)
	cex := pricingDI.GetCEXProvider(mono.Services())
	if connector, ok := cex.(interface{ Connect(context.Context) error }); ok {
		// Try to connect with a short timeout - don't block startup
		connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if err := connector.Connect(connectCtx); err != nil {
			log.Warn(ctx, "binance connection failed, will retry in background", "error", err)
			// Start background connection retry
			go func() {
				for {
					select {
					case <-ctx.Done():
						return
					case <-time.After(5 * time.Second):
						if err := connector.Connect(ctx); err != nil {
							log.Warn(ctx, "binance retry failed", "error", err)
						} else {
							log.Info(ctx, "binance connected successfully")
							return
						}
					}
				}
			}()
		}
	}

	log.Info(ctx, "pricing module started")
	return nil
}
