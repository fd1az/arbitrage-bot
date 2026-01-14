// Package domain contains the core domain types for the pricing context.
package domain

import "github.com/shopspring/decimal"

// Spread represents the price difference between two sources.
type Spread struct {
	CEXPrice    decimal.Decimal
	DEXPrice    decimal.Decimal
	Absolute    decimal.Decimal // DEX - CEX
	BasisPoints decimal.Decimal // (DEX - CEX) / CEX * 10000
	Direction   SpreadDirection
}

// SpreadDirection indicates the profitable trade direction.
type SpreadDirection string

const (
	SpreadCEXToDEX SpreadDirection = "CEX_TO_DEX" // Buy on CEX, sell on DEX
	SpreadDEXToCEX SpreadDirection = "DEX_TO_CEX" // Buy on DEX, sell on CEX
	SpreadNone     SpreadDirection = "NONE"       // No profitable spread
)

// CalculateSpread computes the spread between CEX and DEX prices.
func CalculateSpread(cexPrice, dexPrice decimal.Decimal) Spread {
	absolute := dexPrice.Sub(cexPrice)
	bps := decimal.Zero
	if !cexPrice.IsZero() {
		bps = absolute.Div(cexPrice).Mul(decimal.NewFromInt(10000))
	}

	var direction SpreadDirection
	switch {
	case absolute.IsPositive():
		direction = SpreadCEXToDEX
	case absolute.IsNegative():
		direction = SpreadDEXToCEX
	default:
		direction = SpreadNone
	}

	return Spread{
		CEXPrice:    cexPrice,
		DEXPrice:    dexPrice,
		Absolute:    absolute,
		BasisPoints: bps,
		Direction:   direction,
	}
}
