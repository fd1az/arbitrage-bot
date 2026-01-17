package asset

import "github.com/ethereum/go-ethereum/common"

// Asset represents the metadata of a crypto or fiat asset.
// It is a reference entity with stable identity (AssetID).
// The symbol is NOT identity - just metadata for display.
type Asset struct {
	id       AssetID
	symbol   string
	name     string
	decimals uint8
}

// NewAsset creates a new Asset with the given parameters.
func NewAsset(id AssetID, symbol string, decimals uint8) *Asset {
	if symbol == "" {
		panic("asset: empty symbol")
	}
	if decimals > 30 {
		panic("asset: suspicious decimals (>30)")
	}

	return &Asset{
		id:       id,
		symbol:   symbol,
		decimals: decimals,
	}
}

// NewAssetWithName creates a new Asset with a human-readable name.
func NewAssetWithName(id AssetID, symbol, name string, decimals uint8) *Asset {
	a := NewAsset(id, symbol, decimals)
	a.name = name
	return a
}

// ID returns the unique identifier for this asset.
func (a *Asset) ID() AssetID {
	return a.id
}

// Symbol returns the ticker symbol (e.g., "ETH", "USDC").
func (a *Asset) Symbol() string {
	return a.symbol
}

// Name returns the human-readable name (e.g., "Ethereum", "USD Coin").
func (a *Asset) Name() string {
	if a.name == "" {
		return a.symbol
	}
	return a.name
}

// Decimals returns the number of decimal places.
func (a *Asset) Decimals() uint8 {
	return a.decimals
}

// ChainID returns the chain ID (0 for fiat).
func (a *Asset) ChainID() uint64 {
	return a.id.ChainID()
}

// IsNative returns true if this is a native coin.
func (a *Asset) IsNative() bool {
	return a.id.IsNative()
}

// IsToken returns true if this is an ERC20 token.
func (a *Asset) IsToken() bool {
	return a.id.IsToken()
}

// IsFiat returns true if this is a fiat currency.
func (a *Asset) IsFiat() bool {
	return a.id.IsFiat()
}

// String returns a human-readable representation.
func (a *Asset) String() string {
	return a.symbol
}

// Equals compares two Assets by their ID.
func (a *Asset) Equals(other *Asset) bool {
	if a == nil || other == nil {
		return a == other
	}
	return a.id.Equals(other.id)
}

// Address returns the token contract address (zero for native coins).
func (a *Asset) Address() common.Address {
	return a.id.Address()
}
