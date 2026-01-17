// Package domain contains the core domain types for the pricing context.
package domain

import (
	"fmt"
	"time"

	"github.com/fd1az/arbitrage-bot/internal/asset"
	"github.com/shopspring/decimal"
)

// Side represents the side of a trade (buy or sell).
type Side string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"
)

// Pair represents a trading pair using typed assets.
type Pair struct {
	Base  *asset.Asset // e.g., ETH
	Quote *asset.Asset // e.g., USDC
}

// NewPair creates a new trading pair.
func NewPair(base, quote *asset.Asset) Pair {
	if base == nil || quote == nil {
		panic("pricing: nil asset in pair")
	}
	return Pair{Base: base, Quote: quote}
}

// String returns the pair symbol (e.g., "ETH-USDC").
func (p Pair) String() string {
	return p.Base.Symbol() + "-" + p.Quote.Symbol()
}

// Invert returns the inverted pair (e.g., ETH-USDC -> USDC-ETH).
func (p Pair) Invert() Pair {
	return Pair{Base: p.Quote, Quote: p.Base}
}

// Price represents a price point with metadata.
// Uses asset.Price for the actual rate.
type Price struct {
	Rate      asset.Price
	Size      asset.Amount // Trade size this price is valid for
	Side      Side
	Source    string // "binance", "uniswap", etc.
	Timestamp time.Time
}

// NewPrice creates a new Price.
func NewPrice(rate asset.Price, size asset.Amount, side Side, source string) Price {
	return Price{
		Rate:      rate,
		Size:      size,
		Side:      side,
		Source:    source,
		Timestamp: time.Now(),
	}
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
	Price  decimal.Decimal // Price in quote currency
	Amount asset.Amount    // Amount available at this price
}

// BestBid returns the best (highest) bid price level.
func (o *Orderbook) BestBid() *OrderbookLevel {
	if len(o.Bids) == 0 {
		return nil
	}
	return &o.Bids[0]
}

// BestAsk returns the best (lowest) ask price level.
func (o *Orderbook) BestAsk() *OrderbookLevel {
	if len(o.Asks) == 0 {
		return nil
	}
	return &o.Asks[0]
}

// MidPrice returns the mid-market price.
func (o *Orderbook) MidPrice() decimal.Decimal {
	bid := o.BestBid()
	ask := o.BestAsk()
	if bid == nil || ask == nil {
		return decimal.Zero
	}
	return bid.Price.Add(ask.Price).Div(decimal.NewFromInt(2))
}

// Quote represents a DEX price quote.
type Quote struct {
	TokenIn     *asset.Asset
	TokenOut    *asset.Asset
	AmountIn    asset.Amount
	AmountOut   asset.Amount
	Price       asset.Price // Effective price (AmountOut/AmountIn adjusted)
	GasEstimate uint64
	FeeTier     int // Fee tier in hundredths of a bip (e.g., 3000 = 0.30%)
	Timestamp   time.Time
}

// FeeTierPercent returns the fee tier as a percentage string (e.g., "0.30%").
func (q Quote) FeeTierPercent() string {
	percent := float64(q.FeeTier) / 10000.0
	return fmt.Sprintf("%.2f%%", percent)
}

// NewQuote creates a new DEX quote.
func NewQuote(tokenIn, tokenOut *asset.Asset, amountIn, amountOut asset.Amount, gasEstimate uint64, feeTier int) Quote {
	// Calculate effective price
	rate := decimal.Zero
	if !amountIn.IsZero() {
		rate = amountOut.ToDecimal().Div(amountIn.ToDecimal())
	}
	price := asset.NewPriceNow(tokenIn, tokenOut, rate)

	return Quote{
		TokenIn:     tokenIn,
		TokenOut:    tokenOut,
		AmountIn:    amountIn,
		AmountOut:   amountOut,
		Price:       price,
		GasEstimate: gasEstimate,
		FeeTier:     feeTier,
		Timestamp:   time.Now(),
	}
}

// PriceSnapshot contains prices from multiple sources for comparison.
type PriceSnapshot struct {
	Pair        Pair
	CEXBid      *Price       // Best bid on CEX
	CEXAsk      *Price       // Best ask on CEX
	DEXQuote    *Quote       // DEX quote for the trade size
	GasPrice    asset.Amount // Gas price in ETH
	BlockNumber uint64
	Timestamp   time.Time
}
