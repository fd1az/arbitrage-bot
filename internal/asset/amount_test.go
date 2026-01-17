package asset_test

import (
	"math/big"
	"testing"

	"github.com/fd1az/arbitrage-bot/internal/asset"
	"github.com/shopspring/decimal"
)

func TestAmount_Basic(t *testing.T) {
	// 1 ETH = 1e18 wei
	oneETH := asset.NewAmount(asset.ETH, big.NewInt(1e18))

	if oneETH.IsZero() {
		t.Error("expected non-zero amount")
	}

	// ToDecimal should return 1.0
	d := oneETH.ToDecimal()
	if !d.Equal(decimal.NewFromInt(1)) {
		t.Errorf("expected 1, got %s", d.String())
	}

	// String should be "1 ETH"
	if oneETH.String() != "1 ETH" {
		t.Errorf("expected '1 ETH', got '%s'", oneETH.String())
	}
}

func TestAmount_Add(t *testing.T) {
	oneETH := asset.NewAmount(asset.ETH, big.NewInt(1e18))
	twoETH := asset.NewAmount(asset.ETH, big.NewInt(2e18))

	sum, err := oneETH.Add(twoETH)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := decimal.NewFromInt(3)
	if !sum.ToDecimal().Equal(expected) {
		t.Errorf("expected 3, got %s", sum.ToDecimal().String())
	}
}

func TestAmount_CannotAddDifferentAssets(t *testing.T) {
	oneETH := asset.NewAmount(asset.ETH, big.NewInt(1e18))
	oneUSDC := asset.NewAmount(asset.USDC, big.NewInt(1e6))

	_, err := oneETH.Add(oneUSDC)
	if err == nil {
		t.Error("expected error when adding different assets")
	}
}

func TestAmount_Sub(t *testing.T) {
	threeETH := asset.NewAmount(asset.ETH, big.NewInt(3e18))
	oneETH := asset.NewAmount(asset.ETH, big.NewInt(1e18))

	diff, err := threeETH.Sub(oneETH)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := decimal.NewFromInt(2)
	if !diff.ToDecimal().Equal(expected) {
		t.Errorf("expected 2, got %s", diff.ToDecimal().String())
	}
}

func TestAmount_SubNegativeError(t *testing.T) {
	oneETH := asset.NewAmount(asset.ETH, big.NewInt(1e18))
	twoETH := asset.NewAmount(asset.ETH, big.NewInt(2e18))

	_, err := oneETH.Sub(twoETH)
	if err == nil {
		t.Error("expected error for negative result")
	}
}

func TestParseDecimal(t *testing.T) {
	// Parse "1.5" ETH
	d := decimal.NewFromFloat(1.5)
	amount, err := asset.ParseDecimal(asset.ETH, d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be 1.5e18 wei
	expected := big.NewInt(0)
	expected.SetString("1500000000000000000", 10)

	if amount.Raw().Cmp(expected) != 0 {
		t.Errorf("expected %s, got %s", expected.String(), amount.Raw().String())
	}
}

func TestParseDecimal_TooManyDecimals(t *testing.T) {
	// USDC has 6 decimals, try to parse 1.1234567 (7 decimals)
	d := decimal.NewFromFloat(1.1234567)
	_, err := asset.ParseDecimal(asset.USDC, d)
	if err == nil {
		t.Error("expected error for too many decimals")
	}
}

func TestPrice_Convert(t *testing.T) {
	// ETH/USDC price = 2000
	price := asset.NewPriceNow(asset.ETH, asset.USDC, decimal.NewFromInt(2000))

	// 1 ETH
	oneETH := asset.NewAmount(asset.ETH, big.NewInt(1e18))

	// Convert to USDC
	usdc, err := price.Convert(oneETH)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be 2000 USDC (2000 * 1e6 = 2e9)
	expectedUSDC := decimal.NewFromInt(2000)
	if !usdc.ToDecimal().Equal(expectedUSDC) {
		t.Errorf("expected %s USDC, got %s", expectedUSDC.String(), usdc.ToDecimal().String())
	}
}

func TestPrice_Invert(t *testing.T) {
	// ETH/USDC = 2000
	price := asset.NewPriceNow(asset.ETH, asset.USDC, decimal.NewFromInt(2000))

	// Invert to USDC/ETH = 0.0005
	inverted := price.Invert()

	expected := decimal.NewFromFloat(0.0005)
	// Allow small precision error
	diff := inverted.Rate().Sub(expected).Abs()
	if diff.GreaterThan(decimal.NewFromFloat(0.0000001)) {
		t.Errorf("expected ~0.0005, got %s", inverted.Rate().String())
	}
}

func TestAssetID_Identity(t *testing.T) {
	// Same token on different chains should have different IDs
	usdcEth := asset.NewTokenAssetID(1, asset.AddrUSDCEthereum)
	usdcEth2 := asset.NewTokenAssetID(1, asset.AddrUSDCEthereum)

	if !usdcEth.Equals(usdcEth2) {
		t.Error("same asset should have equal IDs")
	}

	// Different chains
	usdcPolygon := asset.NewTokenAssetID(137, asset.AddrUSDCEthereum) // hypothetically same address

	if usdcEth.Equals(usdcPolygon) {
		t.Error("different chains should have different IDs")
	}
}

func TestRegistry(t *testing.T) {
	r := asset.DefaultRegistry()

	// Should find ETH
	eth, ok := r.GetNative(asset.ChainIDEthereum)
	if !ok {
		t.Error("ETH not found in registry")
	}
	if eth.Symbol() != "ETH" {
		t.Errorf("expected ETH, got %s", eth.Symbol())
	}

	// Should find USDC by symbol and chain
	usdc, ok := r.GetBySymbolAndChain("USDC", asset.ChainIDEthereum)
	if !ok {
		t.Error("USDC not found in registry")
	}
	if usdc.Decimals() != 6 {
		t.Errorf("expected 6 decimals, got %d", usdc.Decimals())
	}
}
