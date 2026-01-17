// Package domain contains the core domain types for the arbitrage context.
package domain

import (
	"math/big"

	"github.com/fd1az/arbitrage-bot/internal/asset"
	"github.com/shopspring/decimal"
)

// GasCost represents the gas cost for a DEX transaction.
type GasCost struct {
	GasLimit uint64       // Gas units needed
	GasPrice asset.Amount // Price per gas unit in ETH (wei)
	TotalETH asset.Amount // Total cost in ETH
	TotalUSD asset.Amount // Total cost in USD (converted)
}

// NewGasCost creates a GasCost from gas parameters and ETH price.
func NewGasCost(gasLimit uint64, gasPriceWei *big.Int, ethPriceUSD decimal.Decimal) *GasCost {
	// Gas price as Amount
	gasPrice := asset.NewAmount(asset.ETH, gasPriceWei)

	// Total ETH = gasLimit * gasPrice
	totalWei := new(big.Int).Mul(gasPriceWei, big.NewInt(int64(gasLimit)))
	totalETH := asset.NewAmount(asset.ETH, totalWei)

	// Convert ETH to USD
	// USD = ETH amount * ETH price
	ethDecimal := totalETH.ToDecimal()
	usdDecimal := ethDecimal.Mul(ethPriceUSD)
	totalUSD, _ := asset.ParseDecimal(asset.USD, usdDecimal)

	return &GasCost{
		GasLimit: gasLimit,
		GasPrice: gasPrice,
		TotalETH: totalETH,
		TotalUSD: totalUSD,
	}
}

// TotalWei returns the total gas cost in wei.
func (g *GasCost) TotalWei() *big.Int {
	return g.TotalETH.Raw()
}

// ProfitResult contains the calculated profit for an opportunity.
type ProfitResult struct {
	GrossProfit   asset.Amount    // Profit before any costs
	GasCost       asset.Amount    // Gas cost in quote currency
	ExchangeFees  asset.Amount    // Exchange trading fees (Uniswap + Binance)
	TotalCosts    asset.Amount    // Gas + Exchange fees
	NetProfit     asset.Amount    // Profit after all costs (absolute value)
	NetProfitRaw  decimal.Decimal // Net profit with sign (can be negative)
	NetProfitPct  decimal.Decimal // Net profit as percentage of gross
	IsProfitable  bool
	TradeValueUSD asset.Amount // Total trade value for reference
}

// NewProfitResult calculates profit from gross profit and gas cost.
// All amounts should be in the same quote currency (e.g., USDC).
func NewProfitResult(grossProfit, gasCost asset.Amount) (*ProfitResult, error) {
	// Net = Gross - Gas
	netProfit, err := grossProfit.Sub(gasCost)
	if err != nil {
		// If subtraction fails (different assets or negative), profit is negative
		netProfit = asset.Zero(grossProfit.Asset())
	}

	// Calculate percentage: (net / gross) * 100
	pct := decimal.Zero
	if !grossProfit.IsZero() {
		pct = netProfit.ToDecimal().Div(grossProfit.ToDecimal()).Mul(decimal.NewFromInt(100))
	}

	return &ProfitResult{
		GrossProfit:  grossProfit,
		GasCost:      gasCost,
		NetProfit:    netProfit,
		NetProfitRaw: netProfit.ToDecimal(),
		NetProfitPct: pct,
		IsProfitable: netProfit.IsPositive(),
	}, nil
}

// NewProfitResultFromDecimals creates a ProfitResult from decimal values.
// Useful for backward compatibility and simpler calculations.
func NewProfitResultFromDecimals(grossProfit, gasCost decimal.Decimal, quoteAsset *asset.Asset) *ProfitResult {
	netProfit := grossProfit.Sub(gasCost)

	pct := decimal.Zero
	if !grossProfit.IsZero() {
		pct = netProfit.Div(grossProfit).Mul(decimal.NewFromInt(100))
	}

	gross, _ := asset.ParseDecimal(quoteAsset, grossProfit.Abs())
	gas, _ := asset.ParseDecimal(quoteAsset, gasCost.Abs())
	net, _ := asset.ParseDecimal(quoteAsset, netProfit.Abs())

	// Handle negative net profit
	isProfitable := netProfit.IsPositive()
	if netProfit.IsNegative() {
		net = asset.Zero(quoteAsset)
	}

	return &ProfitResult{
		GrossProfit:  gross,
		GasCost:      gas,
		ExchangeFees: asset.Zero(quoteAsset),
		TotalCosts:   gas,
		NetProfit:    net,
		NetProfitRaw: netProfit, // Preserve sign
		NetProfitPct: pct,
		IsProfitable: isProfitable,
	}
}

// NewProfitResultWithFees creates a ProfitResult including exchange fees.
// This is the complete calculation for arbitrage profitability.
func NewProfitResultWithFees(grossProfit, gasCost, exchangeFees decimal.Decimal, quoteAsset *asset.Asset) *ProfitResult {
	totalCosts := gasCost.Add(exchangeFees)
	netProfit := grossProfit.Sub(totalCosts)

	pct := decimal.Zero
	if !grossProfit.IsZero() {
		pct = netProfit.Div(grossProfit).Mul(decimal.NewFromInt(100))
	}

	// Round to asset's decimal places (USD has 2 decimals)
	decimals := int32(quoteAsset.Decimals())
	grossRounded := grossProfit.Abs().Round(decimals)
	gasRounded := gasCost.Abs().Round(decimals)
	feesRounded := exchangeFees.Abs().Round(decimals)
	costsRounded := totalCosts.Abs().Round(decimals)
	netRounded := netProfit.Abs().Round(decimals)

	gross, _ := asset.ParseDecimal(quoteAsset, grossRounded)
	gas, _ := asset.ParseDecimal(quoteAsset, gasRounded)
	fees, _ := asset.ParseDecimal(quoteAsset, feesRounded)
	costs, _ := asset.ParseDecimal(quoteAsset, costsRounded)
	net, _ := asset.ParseDecimal(quoteAsset, netRounded)

	isProfitable := netProfit.IsPositive()

	return &ProfitResult{
		GrossProfit:  gross,
		GasCost:      gas,
		ExchangeFees: fees,
		TotalCosts:   costs,
		NetProfit:    net,
		NetProfitRaw: netProfit.Round(decimals), // Preserve sign for display
		NetProfitPct: pct,
		IsProfitable: isProfitable,
	}
}
