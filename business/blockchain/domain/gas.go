// Package domain contains the core domain types for the blockchain context.
package domain

import (
	"math/big"
	"time"

	"github.com/fd1az/arbitrage-bot/internal/asset"
)

// GasPrice represents gas price information using asset.Amount.
type GasPrice struct {
	PricePerUnit asset.Amount // Price per gas unit in ETH (wei)
	Timestamp    time.Time
}

// NewGasPrice creates a GasPrice from wei.
func NewGasPrice(weiPerGas *big.Int) *GasPrice {
	return &GasPrice{
		PricePerUnit: asset.NewAmount(asset.ETH, weiPerGas),
		Timestamp:    time.Now(),
	}
}

// Wei returns the gas price in wei.
func (g *GasPrice) Wei() *big.Int {
	return g.PricePerUnit.Raw()
}

// Gwei returns the gas price in gwei (for display).
func (g *GasPrice) Gwei() float64 {
	// 1 gwei = 1e9 wei, ETH has 18 decimals
	// So gwei = wei / 1e9 = ToDecimal * 1e9
	return g.PricePerUnit.ToFloat64() * 1e9
}

// GasEstimate represents estimated gas costs for an operation.
type GasEstimate struct {
	GasLimit uint64       // Gas units needed
	GasPrice *GasPrice    // Price per gas unit
	TotalCost asset.Amount // Total cost in ETH (gasLimit * gasPrice)
}

// NewGasEstimate creates a GasEstimate from gas parameters.
func NewGasEstimate(gasLimit uint64, gasPrice *GasPrice) *GasEstimate {
	// Total = gasLimit * pricePerUnit
	totalWei := new(big.Int).Mul(
		big.NewInt(int64(gasLimit)),
		gasPrice.Wei(),
	)

	return &GasEstimate{
		GasLimit:  gasLimit,
		GasPrice:  gasPrice,
		TotalCost: asset.NewAmount(asset.ETH, totalWei),
	}
}

// TotalWei returns the total gas cost in wei.
func (e *GasEstimate) TotalWei() *big.Int {
	return e.TotalCost.Raw()
}

// TotalETH returns the total gas cost in ETH (for display).
func (e *GasEstimate) TotalETH() float64 {
	return e.TotalCost.ToFloat64()
}

// TotalGwei returns the total gas cost in gwei (for display).
func (e *GasEstimate) TotalGwei() float64 {
	return e.TotalCost.ToFloat64() * 1e9
}
