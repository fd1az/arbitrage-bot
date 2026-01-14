// Package domain contains the core domain types for the blockchain context.
package domain

import (
	"math/big"
	"time"
)

// GasPrice represents gas price information.
type GasPrice struct {
	Wei       *big.Int
	Gwei      float64
	Timestamp time.Time
}

// NewGasPrice creates a GasPrice from wei.
func NewGasPrice(wei *big.Int) *GasPrice {
	gwei := new(big.Float).SetInt(wei)
	gwei.Quo(gwei, big.NewFloat(1e9))
	gweiFloat, _ := gwei.Float64()

	return &GasPrice{
		Wei:       wei,
		Gwei:      gweiFloat,
		Timestamp: time.Now(),
	}
}

// GasEstimate represents estimated gas costs for an operation.
type GasEstimate struct {
	GasLimit  uint64
	GasPrice  *GasPrice
	TotalWei  *big.Int
	TotalGwei float64
}

// CalculateGasEstimate computes the total gas cost.
func CalculateGasEstimate(gasLimit uint64, gasPrice *GasPrice) *GasEstimate {
	totalWei := new(big.Int).Mul(gasPrice.Wei, big.NewInt(int64(gasLimit)))
	totalGwei := gasPrice.Gwei * float64(gasLimit)

	return &GasEstimate{
		GasLimit:  gasLimit,
		GasPrice:  gasPrice,
		TotalWei:  totalWei,
		TotalGwei: totalGwei,
	}
}
