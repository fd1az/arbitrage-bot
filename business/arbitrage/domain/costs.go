// Package domain contains the core domain types for the arbitrage context.
package domain

import (
	"math/big"

	"github.com/shopspring/decimal"
)

// GasCost represents the gas cost for a DEX transaction.
type GasCost struct {
	GasLimit  uint64
	GasPrice  *big.Int   // in wei
	TotalWei  *big.Int   // gasLimit * gasPrice
	ETH       decimal.Decimal
	USD       decimal.Decimal // converted using current ETH price
}

// NewGasCost creates a GasCost from gas parameters.
func NewGasCost(gasLimit uint64, gasPriceWei *big.Int, ethPriceUSD decimal.Decimal) *GasCost {
	totalWei := new(big.Int).Mul(gasPriceWei, big.NewInt(int64(gasLimit)))

	// Convert wei to ETH (1 ETH = 10^18 wei)
	weiPerETH := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	ethFloat := new(big.Float).Quo(
		new(big.Float).SetInt(totalWei),
		new(big.Float).SetInt(weiPerETH),
	)
	ethStr := ethFloat.Text('f', 18)
	eth, _ := decimal.NewFromString(ethStr)

	// Convert ETH to USD
	usd := eth.Mul(ethPriceUSD)

	return &GasCost{
		GasLimit: gasLimit,
		GasPrice: gasPriceWei,
		TotalWei: totalWei,
		ETH:      eth,
		USD:      usd,
	}
}

// ProfitResult contains the calculated profit for an opportunity.
type ProfitResult struct {
	GrossProfit  decimal.Decimal
	GasCost      decimal.Decimal
	NetProfit    decimal.Decimal
	NetProfitPct decimal.Decimal // as percentage (e.g., 0.86 for 0.86%)
	IsProfitable bool
}
