package asset

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/shopspring/decimal"
)

// Common errors
var (
	ErrNilAsset        = errors.New("asset: nil asset")
	ErrNilRaw          = errors.New("asset: nil raw value")
	ErrNegativeAmount  = errors.New("asset: negative amount")
	ErrAssetMismatch   = errors.New("asset: cannot operate on different assets")
	ErrNegativeResult  = errors.New("asset: operation would result in negative amount")
	ErrTooManyDecimals = errors.New("asset: too many decimal places for asset")
	ErrDivisionByZero  = errors.New("asset: division by zero")
)

// Amount is an immutable Value Object representing a quantity of an asset.
// The raw value is always in the smallest unit (wei, satoshi, cents, etc).
type Amount struct {
	raw   *big.Int
	asset *Asset
}

// NewAmount creates a new Amount from a raw big.Int value.
// The raw value must be in the smallest unit (wei, satoshi, etc).
func NewAmount(asset *Asset, raw *big.Int) Amount {
	if asset == nil {
		panic(ErrNilAsset)
	}
	if raw == nil {
		panic(ErrNilRaw)
	}
	if raw.Sign() < 0 {
		panic(ErrNegativeAmount)
	}

	return Amount{
		raw:   new(big.Int).Set(raw), // defensive copy
		asset: asset,
	}
}

// Zero creates a zero Amount for the given asset.
func Zero(asset *Asset) Amount {
	return NewAmount(asset, big.NewInt(0))
}

// NewAmountFromInt64 creates an Amount from an int64 raw value.
func NewAmountFromInt64(asset *Asset, raw int64) Amount {
	if raw < 0 {
		panic(ErrNegativeAmount)
	}
	return NewAmount(asset, big.NewInt(raw))
}

// NewAmountFromUint64 creates an Amount from a uint64 raw value.
func NewAmountFromUint64(asset *Asset, raw uint64) Amount {
	return NewAmount(asset, new(big.Int).SetUint64(raw))
}

// Raw returns a copy of the raw big.Int value.
func (a Amount) Raw() *big.Int {
	if a.raw == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(a.raw)
}

// Asset returns the asset this amount is denominated in.
func (a Amount) Asset() *Asset {
	return a.asset
}

// IsZero returns true if the amount is zero.
func (a Amount) IsZero() bool {
	return a.raw == nil || a.raw.Sign() == 0
}

// IsPositive returns true if the amount is greater than zero.
func (a Amount) IsPositive() bool {
	return a.raw != nil && a.raw.Sign() > 0
}

// -----------------------------------------------------------------------------
// Arithmetic Operations (type-safe, same asset only)
// -----------------------------------------------------------------------------

// Add adds two amounts of the same asset.
func (a Amount) Add(b Amount) (Amount, error) {
	if err := a.checkSameAsset(b); err != nil {
		return Amount{}, err
	}

	sum := new(big.Int).Add(a.raw, b.raw)
	return NewAmount(a.asset, sum), nil
}

// MustAdd adds two amounts, panics on error.
func (a Amount) MustAdd(b Amount) Amount {
	result, err := a.Add(b)
	if err != nil {
		panic(err)
	}
	return result
}

// Sub subtracts b from a (same asset only).
func (a Amount) Sub(b Amount) (Amount, error) {
	if err := a.checkSameAsset(b); err != nil {
		return Amount{}, err
	}

	if a.raw.Cmp(b.raw) < 0 {
		return Amount{}, ErrNegativeResult
	}

	diff := new(big.Int).Sub(a.raw, b.raw)
	return NewAmount(a.asset, diff), nil
}

// MustSub subtracts b from a, panics on error.
func (a Amount) MustSub(b Amount) Amount {
	result, err := a.Sub(b)
	if err != nil {
		panic(err)
	}
	return result
}

// Mul multiplies the amount by an integer factor.
func (a Amount) Mul(factor int64) Amount {
	if factor < 0 {
		panic(ErrNegativeAmount)
	}
	result := new(big.Int).Mul(a.raw, big.NewInt(factor))
	return NewAmount(a.asset, result)
}

// MulBig multiplies the amount by a big.Int factor.
func (a Amount) MulBig(factor *big.Int) Amount {
	if factor.Sign() < 0 {
		panic(ErrNegativeAmount)
	}
	result := new(big.Int).Mul(a.raw, factor)
	return NewAmount(a.asset, result)
}

// Div divides the amount by an integer divisor (integer division).
func (a Amount) Div(divisor int64) (Amount, error) {
	if divisor == 0 {
		return Amount{}, ErrDivisionByZero
	}
	if divisor < 0 {
		return Amount{}, ErrNegativeAmount
	}
	result := new(big.Int).Div(a.raw, big.NewInt(divisor))
	return NewAmount(a.asset, result), nil
}

// DivBig divides the amount by a big.Int divisor.
func (a Amount) DivBig(divisor *big.Int) (Amount, error) {
	if divisor.Sign() == 0 {
		return Amount{}, ErrDivisionByZero
	}
	if divisor.Sign() < 0 {
		return Amount{}, ErrNegativeAmount
	}
	result := new(big.Int).Div(a.raw, divisor)
	return NewAmount(a.asset, result), nil
}

// -----------------------------------------------------------------------------
// Comparison Operations
// -----------------------------------------------------------------------------

// Cmp compares two amounts of the same asset.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func (a Amount) Cmp(b Amount) (int, error) {
	if err := a.checkSameAsset(b); err != nil {
		return 0, err
	}
	return a.raw.Cmp(b.raw), nil
}

// Equals returns true if both amounts are equal (same asset and value).
func (a Amount) Equals(b Amount) bool {
	if !a.asset.ID().Equals(b.asset.ID()) {
		return false
	}
	return a.raw.Cmp(b.raw) == 0
}

// GreaterThan returns true if a > b.
func (a Amount) GreaterThan(b Amount) (bool, error) {
	cmp, err := a.Cmp(b)
	if err != nil {
		return false, err
	}
	return cmp > 0, nil
}

// GreaterThanOrEqual returns true if a >= b.
func (a Amount) GreaterThanOrEqual(b Amount) (bool, error) {
	cmp, err := a.Cmp(b)
	if err != nil {
		return false, err
	}
	return cmp >= 0, nil
}

// LessThan returns true if a < b.
func (a Amount) LessThan(b Amount) (bool, error) {
	cmp, err := a.Cmp(b)
	if err != nil {
		return false, err
	}
	return cmp < 0, nil
}

// LessThanOrEqual returns true if a <= b.
func (a Amount) LessThanOrEqual(b Amount) (bool, error) {
	cmp, err := a.Cmp(b)
	if err != nil {
		return false, err
	}
	return cmp <= 0, nil
}

// -----------------------------------------------------------------------------
// Boundary Functions (decimal conversion - UI/display only)
// -----------------------------------------------------------------------------

// ToDecimal converts the amount to decimal.Decimal for display.
// This is a BOUNDARY function - use only for UI/display, not calculations.
func (a Amount) ToDecimal() decimal.Decimal {
	if a.raw == nil || a.asset == nil {
		return decimal.Zero
	}
	return decimal.NewFromBigInt(a.raw, -int32(a.asset.Decimals()))
}

// ToFloat64 converts the amount to float64 for display.
// WARNING: Use only for display/logging, NOT for calculations.
func (a Amount) ToFloat64() float64 {
	f, _ := a.ToDecimal().Float64()
	return f
}

// ParseDecimal creates an Amount from a decimal value.
// This is a BOUNDARY function - use for parsing user input.
func ParseDecimal(asset *Asset, d decimal.Decimal) (Amount, error) {
	if asset == nil {
		return Amount{}, ErrNilAsset
	}
	if d.IsNegative() {
		return Amount{}, ErrNegativeAmount
	}

	// Scale up by decimals
	scaled := d.Shift(int32(asset.Decimals()))

	// Check if result is an integer (no fractional part lost)
	if !scaled.Equal(scaled.Truncate(0)) {
		return Amount{}, ErrTooManyDecimals
	}

	return NewAmount(asset, scaled.BigInt()), nil
}

// ParseString creates an Amount from a string decimal value.
func ParseString(asset *Asset, s string) (Amount, error) {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return Amount{}, fmt.Errorf("asset: invalid decimal string: %w", err)
	}
	return ParseDecimal(asset, d)
}

// ParseFloat64 creates an Amount from a float64 value.
// WARNING: May lose precision - prefer ParseString or ParseDecimal.
func ParseFloat64(asset *Asset, f float64) (Amount, error) {
	return ParseDecimal(asset, decimal.NewFromFloat(f))
}

// -----------------------------------------------------------------------------
// Display
// -----------------------------------------------------------------------------

// String returns a human-readable representation (e.g., "1.5 ETH").
func (a Amount) String() string {
	if a.asset == nil {
		return "0 ???"
	}
	return fmt.Sprintf("%s %s", a.ToDecimal().String(), a.asset.Symbol())
}

// StringFixed returns a string with fixed decimal places.
func (a Amount) StringFixed(places int32) string {
	if a.asset == nil {
		return "0 ???"
	}
	return fmt.Sprintf("%s %s", a.ToDecimal().StringFixed(places), a.asset.Symbol())
}

// -----------------------------------------------------------------------------
// Internal helpers
// -----------------------------------------------------------------------------

func (a Amount) checkSameAsset(b Amount) error {
	if a.asset == nil || b.asset == nil {
		return ErrNilAsset
	}
	if !a.asset.ID().Equals(b.asset.ID()) {
		return fmt.Errorf("%w: %s vs %s", ErrAssetMismatch, a.asset.Symbol(), b.asset.Symbol())
	}
	return nil
}
