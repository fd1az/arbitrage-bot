// Package domain contains the core domain types for the arbitrage context.
package domain

// Direction represents the arbitrage trade direction.
type Direction string

const (
	// DirectionCEXToDEX means buy on CEX, sell on DEX.
	DirectionCEXToDEX Direction = "CEX_TO_DEX"

	// DirectionDEXToCEX means buy on DEX, sell on CEX.
	DirectionDEXToCEX Direction = "DEX_TO_CEX"
)

// String returns a human-readable description of the direction.
func (d Direction) String() string {
	switch d {
	case DirectionCEXToDEX:
		return "CEX → DEX (Buy on Binance, Sell on Uniswap)"
	case DirectionDEXToCEX:
		return "DEX → CEX (Buy on Uniswap, Sell on Binance)"
	default:
		return "Unknown"
	}
}
