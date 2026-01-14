// Package app contains application services and port definitions for the blockchain context.
package app

import (
	"context"

	"github.com/fd1az/arbitrage-bot/business/blockchain/domain"
)

// BlockSubscriber defines the interface for subscribing to new blocks.
type BlockSubscriber interface {
	// Subscribe starts listening for new blocks and returns a channel of blocks.
	Subscribe(ctx context.Context) (<-chan *domain.Block, error)

	// LatestBlock retrieves the most recent block.
	LatestBlock(ctx context.Context) (*domain.Block, error)

	// State returns the current connection state.
	State() domain.ConnectionState
}

// GasOracle defines the interface for gas price information.
type GasOracle interface {
	// GetGasPrice retrieves the current gas price.
	GetGasPrice(ctx context.Context) (*domain.GasPrice, error)

	// EstimateGas estimates the gas needed for a transaction.
	EstimateGas(ctx context.Context, data []byte, to string) (uint64, error)
}
