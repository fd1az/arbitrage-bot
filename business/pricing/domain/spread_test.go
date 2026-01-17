package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestCalculateSpread(t *testing.T) {
	tests := []struct {
		name          string
		cexPrice      string
		dexPrice      string
		wantAbsolute  string
		wantBPS       string
		wantDirection SpreadDirection
	}{
		{
			name:          "equal_prices_no_spread",
			cexPrice:      "3400.00",
			dexPrice:      "3400.00",
			wantAbsolute:  "0",
			wantBPS:       "0",
			wantDirection: SpreadNone,
		},
		{
			name:          "dex_higher_1pct_buy_on_cex",
			cexPrice:      "3400.00",
			dexPrice:      "3434.00",
			wantAbsolute:  "34",
			wantBPS:       "100", // 34/3400 * 10000 = 100
			wantDirection: SpreadCEXToDEX,
		},
		{
			name:          "dex_lower_1pct_buy_on_dex",
			cexPrice:      "3400.00",
			dexPrice:      "3366.00",
			wantAbsolute:  "-34",
			wantBPS:       "-100", // -34/3400 * 10000 = -100
			wantDirection: SpreadDEXToCEX,
		},
		{
			name:          "zero_cex_price_no_panic",
			cexPrice:      "0",
			dexPrice:      "3400.00",
			wantAbsolute:  "3400",
			wantBPS:       "0", // Division by zero avoided
			wantDirection: SpreadCEXToDEX,
		},
		{
			name:          "zero_dex_price",
			cexPrice:      "3400.00",
			dexPrice:      "0",
			wantAbsolute:  "-3400",
			wantBPS:       "-10000", // -100%
			wantDirection: SpreadDEXToCEX,
		},
		{
			name:          "both_zero",
			cexPrice:      "0",
			dexPrice:      "0",
			wantAbsolute:  "0",
			wantBPS:       "0",
			wantDirection: SpreadNone,
		},
		{
			name:          "tiny_spread_positive",
			cexPrice:      "3400.00",
			dexPrice:      "3400.34",
			wantAbsolute:  "0.34",
			wantBPS:       "1", // 0.34/3400 * 10000 = 1 bps
			wantDirection: SpreadCEXToDEX,
		},
		{
			name:          "tiny_spread_negative",
			cexPrice:      "3400.00",
			dexPrice:      "3399.66",
			wantAbsolute:  "-0.34",
			wantBPS:       "-1",
			wantDirection: SpreadDEXToCEX,
		},
		{
			name:          "large_spread_10pct",
			cexPrice:      "3000.00",
			dexPrice:      "3300.00",
			wantAbsolute:  "300",
			wantBPS:       "1000", // 10%
			wantDirection: SpreadCEXToDEX,
		},
		{
			name:          "large_numbers",
			cexPrice:      "100000.00",
			dexPrice:      "101000.00",
			wantAbsolute:  "1000",
			wantBPS:       "100",
			wantDirection: SpreadCEXToDEX,
		},
		{
			name:          "small_numbers",
			cexPrice:      "0.001",
			dexPrice:      "0.00101",
			wantAbsolute:  "0.00001",
			wantBPS:       "100", // 1%
			wantDirection: SpreadCEXToDEX,
		},
		{
			name:          "high_precision",
			cexPrice:      "3456.789012345678",
			dexPrice:      "3460.245801357913",
			wantAbsolute:  "3.456789012235",
			wantBPS:       "10", // ~0.1% = 10 bps
			wantDirection: SpreadCEXToDEX,
		},
		{
			name:          "negative_spread_profitable_dex",
			cexPrice:      "2000.00",
			dexPrice:      "1980.00",
			wantAbsolute:  "-20",
			wantBPS:       "-100", // -1%
			wantDirection: SpreadDEXToCEX,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cex := decimal.RequireFromString(tt.cexPrice)
			dex := decimal.RequireFromString(tt.dexPrice)

			spread := CalculateSpread(cex, dex)

			// Check CEX price stored correctly
			if !spread.CEXPrice.Equal(cex) {
				t.Errorf("CEXPrice = %s, want %s", spread.CEXPrice, cex)
			}

			// Check DEX price stored correctly
			if !spread.DEXPrice.Equal(dex) {
				t.Errorf("DEXPrice = %s, want %s", spread.DEXPrice, dex)
			}

			// Check absolute spread
			wantAbsolute := decimal.RequireFromString(tt.wantAbsolute)
			if !spread.Absolute.Equal(wantAbsolute) {
				t.Errorf("Absolute = %s, want %s", spread.Absolute, wantAbsolute)
			}

			// Check basis points (with some tolerance for precision)
			wantBPS := decimal.RequireFromString(tt.wantBPS)
			bpsRounded := spread.BasisPoints.Round(0)
			if !bpsRounded.Equal(wantBPS) {
				t.Errorf("BasisPoints = %s (rounded: %s), want %s",
					spread.BasisPoints, bpsRounded, wantBPS)
			}

			// Check direction
			if spread.Direction != tt.wantDirection {
				t.Errorf("Direction = %v, want %v", spread.Direction, tt.wantDirection)
			}
		})
	}
}

func TestSpreadDirection_String(t *testing.T) {
	tests := []struct {
		direction SpreadDirection
		want      string
	}{
		{SpreadCEXToDEX, "CEX_TO_DEX"},
		{SpreadDEXToCEX, "DEX_TO_CEX"},
		{SpreadNone, "NONE"},
	}

	for _, tt := range tests {
		t.Run(string(tt.direction), func(t *testing.T) {
			if got := string(tt.direction); got != tt.want {
				t.Errorf("SpreadDirection = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCalculateSpread_Symmetry(t *testing.T) {
	// When we swap CEX and DEX, the absolute spread should negate
	// and direction should flip
	cex := decimal.RequireFromString("3400")
	dex := decimal.RequireFromString("3434")

	spread1 := CalculateSpread(cex, dex)
	spread2 := CalculateSpread(dex, cex)

	// Absolute values should be negated
	if !spread1.Absolute.Add(spread2.Absolute).IsZero() {
		t.Errorf("Absolutes don't negate: %s + %s != 0",
			spread1.Absolute, spread2.Absolute)
	}

	// Directions should be opposite
	if spread1.Direction == spread2.Direction {
		t.Errorf("Directions should be opposite: both are %v", spread1.Direction)
	}
}

func TestCalculateSpread_BasisPointsFormula(t *testing.T) {
	// Verify: BPS = (DEX - CEX) / CEX * 10000
	cex := decimal.RequireFromString("2500")
	dex := decimal.RequireFromString("2525") // 1% higher

	spread := CalculateSpread(cex, dex)

	// Manual calculation: (2525 - 2500) / 2500 * 10000 = 100 bps
	expected := dex.Sub(cex).Div(cex).Mul(decimal.NewFromInt(10000))

	if !spread.BasisPoints.Equal(expected) {
		t.Errorf("BasisPoints formula incorrect: got %s, want %s",
			spread.BasisPoints, expected)
	}
}

// Benchmark for performance-critical spread calculation
func BenchmarkCalculateSpread(b *testing.B) {
	cex := decimal.RequireFromString("3456.789")
	dex := decimal.RequireFromString("3460.123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateSpread(cex, dex)
	}
}
