package app

import (
	"math/big"
	"testing"

	"github.com/fd1az/arbitrage-bot/business/arbitrage/domain"
	pricingDomain "github.com/fd1az/arbitrage-bot/business/pricing/domain"
	"github.com/shopspring/decimal"
)

// Helper to create a GasCost
func makeGasCost(gasLimit uint64, gasPriceGwei int64, ethPriceUSD string) *domain.GasCost {
	gasPriceWei := big.NewInt(gasPriceGwei * 1_000_000_000) // gwei to wei
	ethPrice := decimal.RequireFromString(ethPriceUSD)
	return domain.NewGasCost(gasLimit, gasPriceWei, ethPrice)
}

// Helper to create a Spread
func makeSpread(cexPrice, dexPrice string) pricingDomain.Spread {
	cex := decimal.RequireFromString(cexPrice)
	dex := decimal.RequireFromString(dexPrice)
	return pricingDomain.CalculateSpread(cex, dex)
}

func TestProfitCalculator_Calculate(t *testing.T) {
	tests := []struct {
		name           string
		minProfitBps   string
		minProfitUSD   string
		cexPrice       string
		dexPrice       string
		tradeSize      string
		tradeValueUSD  string
		gasLimit       uint64
		gasPriceGwei   int64
		ethPriceUSD    string
		wantGross      string // Expected gross profit
		wantFees       string // Expected exchange fees
		wantGas        string // Expected gas cost USD
		wantNet        string // Expected net profit
		wantProfitable bool
	}{
		{
			name:           "profitable_large_spread",
			minProfitBps:   "10",
			minProfitUSD:   "50",
			cexPrice:       "3400",
			dexPrice:       "3350", // DEX $50 cheaper per ETH
			tradeSize:      "10",
			tradeValueUSD:  "34000",       // 10 * 3400
			gasLimit:       200_000,
			gasPriceGwei:   25,
			ethPriceUSD:    "3400",
			wantGross:      "500",          // |3350-3400| * 10 = 500
			wantFees:       "136",          // 34000 * 0.004 = 136
			wantGas:        "17",           // 200000 * 25gwei * 3400 / 1e18
			wantNet:        "347",          // 500 - 136 - 17
			wantProfitable: true,
		},
		{
			name:           "unprofitable_small_spread",
			minProfitBps:   "10",
			minProfitUSD:   "50",
			cexPrice:       "3400",
			dexPrice:       "3399", // Only $1 difference
			tradeSize:      "10",
			tradeValueUSD:  "34000",
			gasLimit:       200_000,
			gasPriceGwei:   25,
			ethPriceUSD:    "3400",
			wantGross:      "10",           // |3399-3400| * 10 = 10
			wantFees:       "136",          // 34000 * 0.004
			wantGas:        "17",           // ~17 USD
			wantNet:        "143",          // gross - fees - gas (stored as |loss|)
			wantProfitable: false,          // gross < costs
		},
		{
			name:           "unprofitable_high_gas",
			minProfitBps:   "10",
			minProfitUSD:   "50",
			cexPrice:       "3400",
			dexPrice:       "3350",
			tradeSize:      "10",
			tradeValueUSD:  "34000",
			gasLimit:       200_000,
			gasPriceGwei:   500,            // Very high gas: 500 gwei
			ethPriceUSD:    "3400",
			wantGross:      "500",
			wantFees:       "136",
			wantGas:        "340",          // 200000 * 500gwei * 3400 / 1e18 = 340
			wantNet:        "24",           // 500 - 136 - 340 = 24
			wantProfitable: false,          // Below minProfitUSD (50)
		},
		{
			name:           "profitable_1_eth",
			minProfitBps:   "10",
			minProfitUSD:   "10",
			cexPrice:       "3400",
			dexPrice:       "3366",         // -34 = -100 bps
			tradeSize:      "1",
			tradeValueUSD:  "3400",
			gasLimit:       200_000,
			gasPriceGwei:   10,             // Low gas
			ethPriceUSD:    "3400",
			wantGross:      "34",           // |3366-3400| * 1 = 34
			wantFees:       "13.6",         // 3400 * 0.004 = 13.6
			wantGas:        "6.8",          // 200000 * 10gwei * 3400 / 1e18
			wantNet:        "13.6",         // 34 - 13.6 - 6.8 = 13.6
			wantProfitable: true,
		},
		{
			name:           "profitable_100_eth",
			minProfitBps:   "10",
			minProfitUSD:   "100",
			cexPrice:       "3400",
			dexPrice:       "3366",         // -34 = -100 bps
			tradeSize:      "100",
			tradeValueUSD:  "340000",
			gasLimit:       200_000,
			gasPriceGwei:   25,
			ethPriceUSD:    "3400",
			wantGross:      "3400",         // |3366-3400| * 100
			wantFees:       "1360",         // 340000 * 0.004
			wantGas:        "17",
			wantNet:        "2023",         // 3400 - 1360 - 17
			wantProfitable: true,
		},
		{
			name:           "below_min_bps_threshold",
			minProfitBps:   "100",           // Require 1% spread
			minProfitUSD:   "10",
			cexPrice:       "3400",
			dexPrice:       "3383",          // -17 = -50 bps (0.5%)
			tradeSize:      "10",
			tradeValueUSD:  "34000",
			gasLimit:       200_000,
			gasPriceGwei:   10,
			ethPriceUSD:    "3400",
			wantGross:      "170",
			wantFees:       "136",
			wantGas:        "6.8",
			wantNet:        "27.2",
			wantProfitable: false,           // 50 bps < 100 bps threshold
		},
		{
			name:           "below_min_usd_threshold",
			minProfitBps:   "10",
			minProfitUSD:   "100",           // Require $100 profit
			cexPrice:       "3400",
			dexPrice:       "3366",
			tradeSize:      "1",
			tradeValueUSD:  "3400",
			gasLimit:       200_000,
			gasPriceGwei:   10,
			ethPriceUSD:    "3400",
			wantGross:      "34",
			wantFees:       "13.6",
			wantGas:        "6.8",
			wantNet:        "13.6",
			wantProfitable: false,           // $13.6 < $100 threshold
		},
		{
			name:           "dex_more_expensive_cex_to_dex",
			minProfitBps:   "10",
			minProfitUSD:   "50",
			cexPrice:       "3400",
			dexPrice:       "3450",          // DEX $50 MORE expensive
			tradeSize:      "10",
			tradeValueUSD:  "34000",
			gasLimit:       200_000,
			gasPriceGwei:   25,
			ethPriceUSD:    "3400",
			wantGross:      "500",           // |3450-3400| * 10 = 500
			wantFees:       "136",
			wantGas:        "17",
			wantNet:        "347",
			wantProfitable: true,
		},
		{
			name:           "zero_spread",
			minProfitBps:   "10",
			minProfitUSD:   "10",
			cexPrice:       "3400",
			dexPrice:       "3400",          // Same price
			tradeSize:      "10",
			tradeValueUSD:  "34000",
			gasLimit:       200_000,
			gasPriceGwei:   25,
			ethPriceUSD:    "3400",
			wantGross:      "0",
			wantFees:       "136",
			wantGas:        "17",
			wantNet:        "153",           // Stored as |loss|
			wantProfitable: false,
		},
		{
			name:           "zero_gas_cost",
			minProfitBps:   "10",
			minProfitUSD:   "50",
			cexPrice:       "3400",
			dexPrice:       "3350",
			tradeSize:      "10",
			tradeValueUSD:  "34000",
			gasLimit:       0,               // No gas
			gasPriceGwei:   25,
			ethPriceUSD:    "3400",
			wantGross:      "500",
			wantFees:       "136",
			wantGas:        "0",
			wantNet:        "364",           // 500 - 136 - 0
			wantProfitable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create calculator
			minBps := decimal.RequireFromString(tt.minProfitBps)
			minUSD := decimal.RequireFromString(tt.minProfitUSD)
			calc := NewProfitCalculator(minBps, minUSD)

			// Create inputs
			spread := makeSpread(tt.cexPrice, tt.dexPrice)
			tradeSize := decimal.RequireFromString(tt.tradeSize)
			tradeValueUSD := decimal.RequireFromString(tt.tradeValueUSD)
			gasCost := makeGasCost(tt.gasLimit, tt.gasPriceGwei, tt.ethPriceUSD)

			// Calculate
			result := calc.Calculate(spread, tradeSize, tradeValueUSD, gasCost)

			// Check profitability
			if result.IsProfitable != tt.wantProfitable {
				t.Errorf("IsProfitable = %v, want %v", result.IsProfitable, tt.wantProfitable)
			}

			// Check gross profit (with tolerance)
			wantGross := decimal.RequireFromString(tt.wantGross)
			gotGross := result.GrossProfit.ToDecimal()
			if !gotGross.Round(0).Equal(wantGross.Round(0)) {
				t.Errorf("GrossProfit = %s, want %s", gotGross.Round(2), wantGross)
			}

			// Check exchange fees
			wantFees := decimal.RequireFromString(tt.wantFees)
			gotFees := result.ExchangeFees.ToDecimal()
			if !gotFees.Round(0).Equal(wantFees.Round(0)) {
				t.Errorf("ExchangeFees = %s, want %s", gotFees.Round(2), wantFees)
			}

			// Check gas cost
			wantGas := decimal.RequireFromString(tt.wantGas)
			gotGas := result.GasCost.ToDecimal()
			if !gotGas.Round(0).Equal(wantGas.Round(0)) {
				t.Errorf("GasCost = %s, want %s", gotGas.Round(2), wantGas)
			}

			// Check net profit (with tolerance for rounding)
			wantNet := decimal.RequireFromString(tt.wantNet)
			gotNet := result.NetProfit.ToDecimal()
			tolerance := decimal.NewFromInt(2) // $2 tolerance for rounding
			diff := gotNet.Sub(wantNet).Abs()
			if diff.GreaterThan(tolerance) {
				t.Errorf("NetProfit = %s, want ~%s (diff: %s)", gotNet.Round(2), wantNet, diff)
			}
		})
	}
}

func TestProfitCalculator_FeeCalculation(t *testing.T) {
	// Verify that TotalFeeRate = 0.004 (0.4%)
	expected := decimal.NewFromFloat(0.004)
	if !TotalFeeRate.Equal(expected) {
		t.Errorf("TotalFeeRate = %s, want %s", TotalFeeRate, expected)
	}

	// Verify component fees
	wantUniswap := decimal.NewFromFloat(0.003)
	if !UniswapFeeBps.Equal(wantUniswap) {
		t.Errorf("UniswapFeeBps = %s, want %s", UniswapFeeBps, wantUniswap)
	}

	wantBinance := decimal.NewFromFloat(0.001)
	if !BinanceFeeBps.Equal(wantBinance) {
		t.Errorf("BinanceFeeBps = %s, want %s", BinanceFeeBps, wantBinance)
	}

	// Verify sum
	if !UniswapFeeBps.Add(BinanceFeeBps).Equal(TotalFeeRate) {
		t.Error("UniswapFeeBps + BinanceFeeBps != TotalFeeRate")
	}
}

func TestNewProfitCalculator(t *testing.T) {
	minBps := decimal.NewFromInt(10)
	minUSD := decimal.NewFromInt(50)

	calc := NewProfitCalculator(minBps, minUSD)

	if calc.minProfitBps.Cmp(minBps) != 0 {
		t.Errorf("minProfitBps = %s, want %s", calc.minProfitBps, minBps)
	}

	if calc.minProfitUSD.Cmp(minUSD) != 0 {
		t.Errorf("minProfitUSD = %s, want %s", calc.minProfitUSD, minUSD)
	}
}

func TestProfitCalculator_GrossProfit_UsesAbsoluteSpread(t *testing.T) {
	calc := NewProfitCalculator(decimal.Zero, decimal.Zero)
	gasCost := makeGasCost(0, 0, "3400") // Zero gas for simplicity

	// Test with negative spread (DEX cheaper)
	spreadNeg := makeSpread("3400", "3350") // DEX $50 cheaper, spread = -50
	result1 := calc.Calculate(spreadNeg, decimal.NewFromInt(10), decimal.NewFromInt(34000), gasCost)

	// Test with positive spread (DEX more expensive)
	spreadPos := makeSpread("3350", "3400") // DEX $50 more expensive, spread = +50
	result2 := calc.Calculate(spreadPos, decimal.NewFromInt(10), decimal.NewFromInt(34000), gasCost)

	// Both should have same gross profit (|50| * 10 = 500)
	if !result1.GrossProfit.ToDecimal().Equal(result2.GrossProfit.ToDecimal()) {
		t.Errorf("Gross profits should be equal: %s vs %s",
			result1.GrossProfit.ToDecimal(), result2.GrossProfit.ToDecimal())
	}
}

// Benchmark for performance-critical calculation
func BenchmarkProfitCalculator_Calculate(b *testing.B) {
	calc := NewProfitCalculator(decimal.NewFromInt(10), decimal.NewFromInt(50))
	spread := makeSpread("3400", "3350")
	tradeSize := decimal.NewFromInt(10)
	tradeValueUSD := decimal.NewFromInt(34000)
	gasCost := makeGasCost(200_000, 25, "3400")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.Calculate(spread, tradeSize, tradeValueUSD, gasCost)
	}
}
