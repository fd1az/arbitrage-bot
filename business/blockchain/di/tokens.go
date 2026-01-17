// Package di contains dependency injection tokens for the blockchain context.
package di

import (
	"github.com/fd1az/arbitrage-bot/business/blockchain/app"
	"github.com/fd1az/arbitrage-bot/internal/di"
)

// Public service tokens - exposed to other modules
var (
	BlockchainService = di.NewToken[*app.BlockchainService]("blockchain.BlockchainService")
)

// Private dependency tokens - internal to blockchain module
var (
	BlockSubscriber = di.NewToken[app.BlockSubscriber]("blockchain:blockSubscriber")
	GasOracle       = di.NewToken[app.GasOracle]("blockchain:gasOracle")
)

// Helper functions for type-safe access
func GetBlockchainService(c di.ServiceRegistry) *app.BlockchainService {
	return di.GetToken(c, BlockchainService)
}

func GetBlockSubscriber(c di.ServiceRegistry) app.BlockSubscriber {
	return di.GetToken(c, BlockSubscriber)
}

func GetGasOracle(c di.ServiceRegistry) app.GasOracle {
	return di.GetToken(c, GasOracle)
}
