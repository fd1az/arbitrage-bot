// Package asset provides a type-safe model for crypto and fiat assets.
// The core uses big.Int for exact on-chain representation.
// decimal.Decimal is only used at boundaries (UI, parsing, display).
package asset

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// AssetID uniquely identifies an asset by chain and contract address.
// For native coins (ETH, MATIC), address is zero.
// This is the TRUE identity - not the symbol.
type AssetID struct {
	chainID uint64
	address common.Address // zero = native coin
}

// NewNativeAssetID creates an AssetID for a native coin (ETH, MATIC, etc).
func NewNativeAssetID(chainID uint64) AssetID {
	return AssetID{
		chainID: chainID,
		address: common.Address{},
	}
}

// NewTokenAssetID creates an AssetID for an ERC20 token.
func NewTokenAssetID(chainID uint64, addr common.Address) AssetID {
	if addr == (common.Address{}) {
		panic("token address cannot be zero - use NewNativeAssetID for native coins")
	}
	return AssetID{
		chainID: chainID,
		address: addr,
	}
}

// NewFiatAssetID creates an AssetID for fiat currencies.
// Uses chainID 0 to represent off-chain/fiat.
func NewFiatAssetID(symbol string) AssetID {
	// Use a deterministic address derived from symbol for uniqueness
	hash := common.BytesToAddress(common.RightPadBytes([]byte(symbol), 20))
	return AssetID{
		chainID: 0, // 0 = fiat/off-chain
		address: hash,
	}
}

// ChainID returns the chain ID (0 for fiat).
func (id AssetID) ChainID() uint64 {
	return id.chainID
}

// Address returns the token contract address (zero for native coins).
func (id AssetID) Address() common.Address {
	return id.address
}

// IsNative returns true if this is a native coin (not an ERC20 token).
func (id AssetID) IsNative() bool {
	return id.chainID != 0 && id.address == (common.Address{})
}

// IsToken returns true if this is an ERC20 token.
func (id AssetID) IsToken() bool {
	return id.chainID != 0 && id.address != (common.Address{})
}

// IsFiat returns true if this is a fiat currency.
func (id AssetID) IsFiat() bool {
	return id.chainID == 0
}

// IsOnChain returns true if this asset exists on a blockchain.
func (id AssetID) IsOnChain() bool {
	return id.chainID != 0
}

// String returns a human-readable representation.
func (id AssetID) String() string {
	if id.IsFiat() {
		return fmt.Sprintf("fiat:%s", id.address.Hex()[:10])
	}
	if id.IsNative() {
		return fmt.Sprintf("chain:%d/native", id.chainID)
	}
	return fmt.Sprintf("chain:%d/%s", id.chainID, id.address.Hex())
}

// Equals compares two AssetIDs for equality.
func (id AssetID) Equals(other AssetID) bool {
	return id.chainID == other.chainID && id.address == other.address
}
