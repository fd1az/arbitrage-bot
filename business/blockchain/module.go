// Package blockchain implements the blockchain bounded context for Ethereum integration.
package blockchain

import (
	"context"

	"github.com/fd1az/arbitrage-bot/business/blockchain/app"
	blockchainDI "github.com/fd1az/arbitrage-bot/business/blockchain/di"
	"github.com/fd1az/arbitrage-bot/business/blockchain/infra/ethereum"
	"github.com/fd1az/arbitrage-bot/internal/config"
	"github.com/fd1az/arbitrage-bot/internal/di"
	"github.com/fd1az/arbitrage-bot/internal/logger"
	"github.com/fd1az/arbitrage-bot/internal/monolith"
)

// Module implements the blockchain bounded context.
type Module struct{}

// RegisterServices registers all blockchain services with the DI container.
func (m *Module) RegisterServices(c di.Container) error {
	// Register BlockSubscriber (private - internal dependency)
	di.RegisterToken(c, blockchainDI.BlockSubscriber, func(sr di.ServiceRegistry) app.BlockSubscriber {
		cfg := sr.Get("config").(*config.Config)
		log := sr.Get("logger").(logger.LoggerInterface)

		subCfg := ethereum.DefaultSubscriberConfig(cfg.Ethereum.WebSocketURL, cfg.Ethereum.HTTPURL)
		sub, err := ethereum.NewSubscriber(subCfg, log)
		if err != nil {
			panic("failed to create subscriber: " + err.Error())
		}
		return sub
	})

	// Register GasOracle (private - internal dependency)
	di.RegisterToken(c, blockchainDI.GasOracle, func(sr di.ServiceRegistry) app.GasOracle {
		cfg := sr.Get("config").(*config.Config)
		log := sr.Get("logger").(logger.LoggerInterface)

		oracleCfg := ethereum.DefaultGasOracleConfig(cfg.Ethereum.HTTPURL)
		oracle, err := ethereum.NewGasOracle(oracleCfg, log)
		if err != nil {
			panic("failed to create gas oracle: " + err.Error())
		}
		return oracle
	})

	// Register BlockchainService (public - exposed to other modules)
	di.RegisterToken(c, blockchainDI.BlockchainService, func(sr di.ServiceRegistry) *app.BlockchainService {
		sub := blockchainDI.GetBlockSubscriber(sr)
		oracle := blockchainDI.GetGasOracle(sr)
		return app.NewBlockchainService(sub, oracle)
	})

	return nil
}

// Startup initializes the blockchain module.
func (m *Module) Startup(ctx context.Context, mono monolith.Monolith) error {
	log := mono.Logger()

	// Connect services
	sub := blockchainDI.GetBlockSubscriber(mono.Services())
	oracle := blockchainDI.GetGasOracle(mono.Services())

	// Connect subscriber (type assertion to access Connect method)
	if connector, ok := sub.(interface{ Connect(context.Context) error }); ok {
		if err := connector.Connect(ctx); err != nil {
			log.Error(ctx, "failed to connect block subscriber", "error", err)
			// Don't fail - will retry on Subscribe
		}
	}

	// Connect gas oracle
	if connector, ok := oracle.(interface{ Connect(context.Context) error }); ok {
		if err := connector.Connect(ctx); err != nil {
			log.Error(ctx, "failed to connect gas oracle", "error", err)
		}
	}

	log.Info(ctx, "blockchain module started")
	return nil
}
