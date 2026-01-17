package asset

import (
	"fmt"
	"math/big"
	"time"

	"github.com/shopspring/decimal"
)

// PricePrecision is the internal precision for price calculations.
// Using 18 decimals to match ETH precision.
const PricePrecision = 18

var pricePrecisionMultiplier = new(big.Int).Exp(big.NewInt(10), big.NewInt(PricePrecision), nil)

// Price represents an exchange rate between two assets.
// Stored as a fixed-point integer with PricePrecision decimals.
// Example: Price of ETH/USDC = 2000.50 stored as 2000500000000000000000
type Price struct {
	rate      *big.Int  // Fixed-point with PricePrecision decimals
	base      *Asset    // The asset being priced (e.g., ETH)
	quote     *Asset    // The unit of price (e.g., USDC)
	timestamp time.Time // When this price was observed
}

// NewPrice creates a new price from a decimal rate.
// For ETH/USDC at 2000.50, rate=2000.50, base=ETH, quote=USDC
func NewPrice(base, quote *Asset, rate decimal.Decimal, timestamp time.Time) Price {
	if base == nil || quote == nil {
		panic("asset: nil base or quote in price")
	}
	if rate.IsNegative() {
		panic("asset: negative price rate")
	}

	// Convert to fixed-point
	scaled := rate.Shift(PricePrecision)
	rawRate := scaled.BigInt()

	return Price{
		rate:      rawRate,
		base:      base,
		quote:     quote,
		timestamp: timestamp,
	}
}

// NewPriceFromBigInt creates a price from a raw fixed-point value.
func NewPriceFromBigInt(base, quote *Asset, rate *big.Int, timestamp time.Time) Price {
	if base == nil || quote == nil {
		panic("asset: nil base or quote in price")
	}
	if rate == nil {
		panic("asset: nil rate")
	}
	if rate.Sign() < 0 {
		panic("asset: negative price rate")
	}

	return Price{
		rate:      new(big.Int).Set(rate),
		base:      base,
		quote:     quote,
		timestamp: timestamp,
	}
}

// NewPriceNow creates a price with current timestamp.
func NewPriceNow(base, quote *Asset, rate decimal.Decimal) Price {
	return NewPrice(base, quote, rate, time.Now())
}

// Rate returns the price rate as a decimal.
func (p Price) Rate() decimal.Decimal {
	if p.rate == nil {
		return decimal.Zero
	}
	return decimal.NewFromBigInt(p.rate, -PricePrecision)
}

// RateRaw returns the raw fixed-point rate.
func (p Price) RateRaw() *big.Int {
	if p.rate == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(p.rate)
}

// Base returns the base asset.
func (p Price) Base() *Asset {
	return p.base
}

// Quote returns the quote asset.
func (p Price) Quote() *Asset {
	return p.quote
}

// Timestamp returns when this price was observed.
func (p Price) Timestamp() time.Time {
	return p.timestamp
}

// Pair returns the trading pair symbol (e.g., "ETH/USDC").
func (p Price) Pair() string {
	if p.base == nil || p.quote == nil {
		return "???/???"
	}
	return fmt.Sprintf("%s/%s", p.base.Symbol(), p.quote.Symbol())
}

// IsZero returns true if the price is zero.
func (p Price) IsZero() bool {
	return p.rate == nil || p.rate.Sign() == 0
}

// Invert returns the inverse price (e.g., ETH/USDC -> USDC/ETH).
func (p Price) Invert() Price {
	if p.IsZero() {
		return Price{
			rate:      big.NewInt(0),
			base:      p.quote,
			quote:     p.base,
			timestamp: p.timestamp,
		}
	}

	// inverse = 1 / rate = precision^2 / rate
	precisionSquared := new(big.Int).Mul(pricePrecisionMultiplier, pricePrecisionMultiplier)
	invertedRate := new(big.Int).Div(precisionSquared, p.rate)

	return Price{
		rate:      invertedRate,
		base:      p.quote,
		quote:     p.base,
		timestamp: p.timestamp,
	}
}

// Convert converts an amount from base to quote currency using this price.
// Returns the equivalent amount in the quote currency.
func (p Price) Convert(amount Amount) (Amount, error) {
	if amount.Asset() == nil {
		return Amount{}, ErrNilAsset
	}

	// Verify the amount is in the base currency
	if !amount.Asset().ID().Equals(p.base.ID()) {
		return Amount{}, fmt.Errorf("%w: expected %s, got %s",
			ErrAssetMismatch, p.base.Symbol(), amount.Asset().Symbol())
	}

	// result = amount * rate / precision
	// But we need to handle decimal shifts between assets
	//
	// amount.raw is in base asset's smallest unit
	// We want result in quote asset's smallest unit
	//
	// Formula: quoteRaw = baseRaw * rate / 10^18 * 10^(quoteDecimals - baseDecimals)

	baseDecimals := int64(p.base.Decimals())
	quoteDecimals := int64(p.quote.Decimals())
	decimalShift := quoteDecimals - baseDecimals

	// Step 1: multiply by rate
	temp := new(big.Int).Mul(amount.Raw(), p.rate)

	// Step 2: divide by precision (10^18)
	temp.Div(temp, pricePrecisionMultiplier)

	// Step 3: adjust for decimal difference
	if decimalShift > 0 {
		multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(decimalShift), nil)
		temp.Mul(temp, multiplier)
	} else if decimalShift < 0 {
		divisor := new(big.Int).Exp(big.NewInt(10), big.NewInt(-decimalShift), nil)
		temp.Div(temp, divisor)
	}

	return NewAmount(p.quote, temp), nil
}

// String returns a human-readable representation.
func (p Price) String() string {
	return fmt.Sprintf("%s %s", p.Rate().String(), p.Pair())
}

// Age returns how old this price is.
func (p Price) Age() time.Duration {
	return time.Since(p.timestamp)
}

// IsStale returns true if the price is older than the given duration.
func (p Price) IsStale(maxAge time.Duration) bool {
	return p.Age() > maxAge
}
