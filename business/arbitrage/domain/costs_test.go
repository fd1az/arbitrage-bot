package domain

import (
	"math/big"
	"testing"

	"github.com/fd1az/arbitrage-bot/internal/asset"
	"github.com/shopspring/decimal"
)

func TestNewGasCost(t *testing.T) {
	tests := []struct {
		name        string
		gasLimit    uint64
		gasPriceWei string // in wei
		ethPriceUSD string
		wantTotalETH string
		wantTotalUSD string
	}{
		{
			name:         "standard_gas_25gwei_3400eth",
			gasLimit:     200_000,
			gasPriceWei:  "25000000000",       // 25 gwei
			ethPriceUSD:  "3400",
			wantTotalETH: "0.005",             // 200000 * 25 gwei = 5000000 gwei = 0.005 ETH
			wantTotalUSD: "17",                // 0.005 * 3400 = 17 USD
		},
		{
			name:         "high_gas_100gwei",
			gasLimit:     200_000,
			gasPriceWei:  "100000000000",      // 100 gwei
			ethPriceUSD:  "3400",
			wantTotalETH: "0.02",              // 200000 * 100 gwei = 0.02 ETH
			wantTotalUSD: "68",                // 0.02 * 3400 = 68 USD
		},
		{
			name:         "low_gas_5gwei",
			gasLimit:     200_000,
			gasPriceWei:  "5000000000",        // 5 gwei
			ethPriceUSD:  "3400",
			wantTotalETH: "0.001",             // 200000 * 5 gwei = 0.001 ETH
			wantTotalUSD: "3.4",               // 0.001 * 3400 = 3.4 USD
		},
		{
			name:         "low_eth_price_2000",
			gasLimit:     200_000,
			gasPriceWei:  "25000000000",       // 25 gwei
			ethPriceUSD:  "2000",
			wantTotalETH: "0.005",
			wantTotalUSD: "10",                // 0.005 * 2000 = 10 USD
		},
		{
			name:         "high_eth_price_5000",
			gasLimit:     200_000,
			gasPriceWei:  "25000000000",       // 25 gwei
			ethPriceUSD:  "5000",
			wantTotalETH: "0.005",
			wantTotalUSD: "25",                // 0.005 * 5000 = 25 USD
		},
		{
			name:         "complex_swap_300k_gas",
			gasLimit:     300_000,
			gasPriceWei:  "30000000000",       // 30 gwei
			ethPriceUSD:  "3500",
			wantTotalETH: "0.009",             // 300000 * 30 gwei = 0.009 ETH
			wantTotalUSD: "31.5",              // 0.009 * 3500 = 31.5 USD
		},
		{
			name:         "zero_gas_limit",
			gasLimit:     0,
			gasPriceWei:  "25000000000",
			ethPriceUSD:  "3400",
			wantTotalETH: "0",
			wantTotalUSD: "0",
		},
		{
			name:         "zero_gas_price",
			gasLimit:     200_000,
			gasPriceWei:  "0",
			ethPriceUSD:  "3400",
			wantTotalETH: "0",
			wantTotalUSD: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gasPriceWei := new(big.Int)
			gasPriceWei.SetString(tt.gasPriceWei, 10)
			ethPrice := decimal.RequireFromString(tt.ethPriceUSD)

			gasCost := NewGasCost(tt.gasLimit, gasPriceWei, ethPrice)

			// Check gas limit stored
			if gasCost.GasLimit != tt.gasLimit {
				t.Errorf("GasLimit = %d, want %d", gasCost.GasLimit, tt.gasLimit)
			}

			// Check total ETH
			wantETH := decimal.RequireFromString(tt.wantTotalETH)
			gotETH := gasCost.TotalETH.ToDecimal()
			if !gotETH.Equal(wantETH) {
				t.Errorf("TotalETH = %s, want %s", gotETH, wantETH)
			}

			// Check total USD (with tolerance for rounding)
			wantUSD := decimal.RequireFromString(tt.wantTotalUSD)
			gotUSD := gasCost.TotalUSD.ToDecimal()
			diff := gotUSD.Sub(wantUSD).Abs()
			tolerance := decimal.RequireFromString("0.01") // 1 cent tolerance
			if diff.GreaterThan(tolerance) {
				t.Errorf("TotalUSD = %s, want %s (diff: %s)", gotUSD, wantUSD, diff)
			}
		})
	}
}

func TestGasCost_TotalWei(t *testing.T) {
	gasPriceWei := big.NewInt(25_000_000_000) // 25 gwei
	ethPrice := decimal.NewFromInt(3400)

	gasCost := NewGasCost(200_000, gasPriceWei, ethPrice)

	// TotalWei = gasLimit * gasPrice = 200000 * 25 gwei = 5000000 gwei = 5e15 wei
	expectedWei := new(big.Int).Mul(big.NewInt(200_000), gasPriceWei)

	if gasCost.TotalWei().Cmp(expectedWei) != 0 {
		t.Errorf("TotalWei = %s, want %s", gasCost.TotalWei(), expectedWei)
	}
}

func TestNewProfitResultWithFees(t *testing.T) {
	usd := asset.USD

	tests := []struct {
		name         string
		grossProfit  string
		gasCost      string
		exchangeFees string
		wantNet      string
		wantPct      string // Net profit percentage
		wantProfit   bool
	}{
		{
			name:         "profitable_after_costs",
			grossProfit:  "100.00",
			gasCost:      "17.00",
			exchangeFees: "40.00",
			wantNet:      "43.00", // 100 - 17 - 40
			wantPct:      "43",    // 43/100 * 100
			wantProfit:   true,
		},
		{
			name:         "unprofitable_high_gas",
			grossProfit:  "50.00",
			gasCost:      "60.00",
			exchangeFees: "10.00",
			wantNet:      "20.00", // |50 - 60 - 10| = 20 (stored as positive but isProfitable = false)
			wantPct:      "-40",   // -20/50 * 100
			wantProfit:   false,
		},
		{
			name:         "unprofitable_high_fees",
			grossProfit:  "50.00",
			gasCost:      "10.00",
			exchangeFees: "50.00",
			wantNet:      "10.00", // |50 - 10 - 50| = 10
			wantPct:      "-20",   // -10/50 * 100
			wantProfit:   false,
		},
		{
			name:         "breakeven",
			grossProfit:  "100.00",
			gasCost:      "50.00",
			exchangeFees: "50.00",
			wantNet:      "0.00",
			wantPct:      "0",
			wantProfit:   false, // Zero profit is not profitable
		},
		{
			name:         "large_profit",
			grossProfit:  "1000.00",
			gasCost:      "20.00",
			exchangeFees: "80.00",
			wantNet:      "900.00",
			wantPct:      "90",
			wantProfit:   true,
		},
		{
			name:         "tiny_profit",
			grossProfit:  "10.00",
			gasCost:      "5.00",
			exchangeFees: "4.00",
			wantNet:      "1.00",
			wantPct:      "10",
			wantProfit:   true,
		},
		{
			name:         "zero_gross_profit",
			grossProfit:  "0.00",
			gasCost:      "10.00",
			exchangeFees: "5.00",
			wantNet:      "15.00", // Stored as positive loss
			wantPct:      "0",     // Can't calculate % of zero
			wantProfit:   false,
		},
		{
			name:         "rounding_test_cents",
			grossProfit:  "55.555",
			gasCost:      "17.123",
			exchangeFees: "20.456",
			wantNet:      "17.98",  // Rounded to 2 decimals
			wantPct:      "32.32",  // ~32.32%
			wantProfit:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gross := decimal.RequireFromString(tt.grossProfit)
			gas := decimal.RequireFromString(tt.gasCost)
			fees := decimal.RequireFromString(tt.exchangeFees)

			result := NewProfitResultWithFees(gross, gas, fees, usd)

			// Check IsProfitable
			if result.IsProfitable != tt.wantProfit {
				t.Errorf("IsProfitable = %v, want %v", result.IsProfitable, tt.wantProfit)
			}

			// Check TotalCosts = gas + fees
			wantCosts := gas.Add(fees)
			gotCosts := result.TotalCosts.ToDecimal()
			if !gotCosts.Round(2).Equal(wantCosts.Round(2)) {
				t.Errorf("TotalCosts = %s, want %s", gotCosts, wantCosts.Round(2))
			}

			// Check NetProfit (stored as absolute value)
			wantNet := decimal.RequireFromString(tt.wantNet)
			gotNet := result.NetProfit.ToDecimal()
			if !gotNet.Round(2).Equal(wantNet) {
				t.Errorf("NetProfit = %s, want %s", gotNet.Round(2), wantNet)
			}

			// Check GrossProfit stored correctly
			gotGross := result.GrossProfit.ToDecimal()
			if !gotGross.Round(2).Equal(gross.Abs().Round(2)) {
				t.Errorf("GrossProfit = %s, want %s", gotGross, gross.Abs().Round(2))
			}

			// Check ExchangeFees stored correctly
			gotFees := result.ExchangeFees.ToDecimal()
			if !gotFees.Round(2).Equal(fees.Abs().Round(2)) {
				t.Errorf("ExchangeFees = %s, want %s", gotFees, fees.Abs().Round(2))
			}

			// Check NetProfitRaw preserves the sign
			expectedRaw := gross.Sub(gas).Sub(fees).Round(2)
			if !result.NetProfitRaw.Equal(expectedRaw) {
				t.Errorf("NetProfitRaw = %s, want %s", result.NetProfitRaw, expectedRaw)
			}
		})
	}
}

func TestNewProfitResult(t *testing.T) {
	// Create test amounts
	grossAmt, _ := asset.ParseDecimal(asset.USD, decimal.NewFromInt(100))
	gasAmt, _ := asset.ParseDecimal(asset.USD, decimal.NewFromInt(30))

	result, err := NewProfitResult(grossAmt, gasAmt)
	if err != nil {
		t.Fatalf("NewProfitResult error: %v", err)
	}

	// Net = Gross - Gas = 100 - 30 = 70
	wantNet := decimal.NewFromInt(70)
	gotNet := result.NetProfit.ToDecimal()
	if !gotNet.Equal(wantNet) {
		t.Errorf("NetProfit = %s, want %s", gotNet, wantNet)
	}

	// Should be profitable
	if !result.IsProfitable {
		t.Error("Expected profitable result")
	}
}

func TestNewProfitResult_DifferentAssets(t *testing.T) {
	// Try to subtract USD from ETH - should handle gracefully
	grossETH, _ := asset.ParseDecimal(asset.ETH, decimal.NewFromInt(1))
	gasUSD, _ := asset.ParseDecimal(asset.USD, decimal.NewFromInt(30))

	result, _ := NewProfitResult(grossETH, gasUSD)

	// Should return zero net profit due to different assets
	if result.NetProfit.ToDecimal().IsPositive() {
		t.Error("Expected zero net profit for different asset types")
	}
}

func TestNewProfitResultFromDecimals(t *testing.T) {
	gross := decimal.RequireFromString("100")
	gas := decimal.RequireFromString("30")

	result := NewProfitResultFromDecimals(gross, gas, asset.USD)

	// Net = 100 - 30 = 70
	wantNet := decimal.NewFromInt(70)
	gotNet := result.NetProfit.ToDecimal()
	if !gotNet.Equal(wantNet) {
		t.Errorf("NetProfit = %s, want %s", gotNet, wantNet)
	}

	// Percentage = 70/100 * 100 = 70%
	wantPct := decimal.NewFromInt(70)
	if !result.NetProfitPct.Equal(wantPct) {
		t.Errorf("NetProfitPct = %s, want %s", result.NetProfitPct, wantPct)
	}

	if !result.IsProfitable {
		t.Error("Expected profitable")
	}
}

func TestNewProfitResultFromDecimals_Negative(t *testing.T) {
	gross := decimal.RequireFromString("30")
	gas := decimal.RequireFromString("100")

	result := NewProfitResultFromDecimals(gross, gas, asset.USD)

	// Should not be profitable
	if result.IsProfitable {
		t.Error("Expected unprofitable")
	}

	// NetProfit (asset.Amount) should be zero when negative (can't store negative)
	if !result.NetProfit.ToDecimal().IsZero() {
		t.Errorf("Expected zero NetProfit for negative result, got %s", result.NetProfit.ToDecimal())
	}

	// NetProfitRaw should preserve the negative value for display
	wantRaw := decimal.NewFromInt(-70) // 30 - 100 = -70
	if !result.NetProfitRaw.Equal(wantRaw) {
		t.Errorf("NetProfitRaw = %s, want %s", result.NetProfitRaw, wantRaw)
	}
}

// Benchmark for performance
func BenchmarkNewProfitResultWithFees(b *testing.B) {
	gross := decimal.RequireFromString("100.50")
	gas := decimal.RequireFromString("17.25")
	fees := decimal.RequireFromString("40.30")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewProfitResultWithFees(gross, gas, fees, asset.USD)
	}
}
