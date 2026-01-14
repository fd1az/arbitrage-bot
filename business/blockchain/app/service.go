// Package app contains application services and port definitions for the blockchain context.
package app

import (
	"context"

	"github.com/fd1az/arbitrage-bot/business/blockchain/domain"
)

// BlockchainService coordinates blockchain interactions.
type BlockchainService struct {
	subscriber BlockSubscriber
	gasOracle  GasOracle
}

// NewBlockchainService creates a new BlockchainService.
func NewBlockchainService(subscriber BlockSubscriber, gasOracle GasOracle) *BlockchainService {
	return &BlockchainService{
		subscriber: subscriber,
		gasOracle:  gasOracle,
	}
}

// SubscribeBlocks starts the block subscription and returns the channel.
func (s *BlockchainService) SubscribeBlocks(ctx context.Context) (<-chan *domain.Block, error) {
	return s.subscriber.Subscribe(ctx)
}

// GetGasPrice retrieves the current gas price.
func (s *BlockchainService) GetGasPrice(ctx context.Context) (*domain.GasPrice, error) {
	return s.gasOracle.GetGasPrice(ctx)
}

// ConnectionState returns the current connection state.
func (s *BlockchainService) ConnectionState() domain.ConnectionState {
	return s.subscriber.State()
}
