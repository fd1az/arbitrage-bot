// Package domain contains the core domain types for the pricing context.
package domain

import (
	"math/big"
	"time"

	"github.com/shopspring/decimal"
)

// Side represents the side of a trade (buy or sell).
type Side string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"
)

// Pair represents a trading pair.
type Pair struct {
	Base  string // e.g., "ETH"
	Quote string // e.g., "USDC"
}

func (p Pair) String() string {
	return p.Base + "-" + p.Quote
}

// Price represents a price point with metadata.
type Price struct {
	Value     decimal.Decimal
	Size      decimal.Decimal
	Side      Side
	Source    string
	Timestamp time.Time
}

// Orderbook represents a snapshot of the orderbook.
type Orderbook struct {
	Pair      Pair
	Bids      []OrderbookLevel
	Asks      []OrderbookLevel
	Timestamp time.Time
}

// OrderbookLevel represents a single price level in the orderbook.
type OrderbookLevel struct {
	Price  decimal.Decimal
	Amount decimal.Decimal
}

// Quote represents a DEX price quote.
type Quote struct {
	TokenIn   string
	TokenOut  string
	AmountIn  *big.Int
	AmountOut *big.Int
	Price     decimal.Decimal
	GasEstimate uint64
	Timestamp time.Time
}

// PriceSnapshot contains prices from multiple sources for comparison.
type PriceSnapshot struct {
	Pair       Pair
	CEXPrices  map[decimal.Decimal]*Price // keyed by trade size
	DEXPrices  map[decimal.Decimal]*Price // keyed by trade size
	GasPrice   *big.Int
	BlockNumber uint64
	Timestamp  time.Time
}
