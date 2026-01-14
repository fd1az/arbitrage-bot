// Package app contains application services and port definitions for the pricing context.
package app

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/fd1az/arbitrage-bot/business/pricing/domain"
	"github.com/shopspring/decimal"
)

// CEXProvider defines the interface for centralized exchange price providers.
type CEXProvider interface {
	// GetOrderbook retrieves the current orderbook for a trading pair.
	GetOrderbook(ctx context.Context, pair domain.Pair) (*domain.Orderbook, error)

	// GetEffectivePrice calculates the effective price for a given trade size,
	// accounting for orderbook depth and slippage.
	GetEffectivePrice(ctx context.Context, pair domain.Pair, size decimal.Decimal, side domain.Side) (*domain.Price, error)
}

// DEXProvider defines the interface for decentralized exchange price providers.
type DEXProvider interface {
	// GetQuote retrieves a price quote for swapping tokens on a DEX.
	GetQuote(ctx context.Context, tokenIn, tokenOut common.Address, amountIn *big.Int) (*domain.Quote, error)
}
