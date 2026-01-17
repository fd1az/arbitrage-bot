package asset

import "github.com/ethereum/go-ethereum/common"

// Chain IDs
const (
	ChainIDEthereum = 1
	ChainIDGoerli   = 5
	ChainIDSepolia  = 11155111
	ChainIDPolygon  = 137
	ChainIDArbitrum = 42161
	ChainIDOptimism = 10
	ChainIDBase     = 8453
	ChainIDBSC      = 56
	ChainIDFiat     = 0 // Off-chain / fiat
)

// Well-known token addresses on Ethereum Mainnet
var (
	// Stablecoins
	AddrUSDCEthereum = common.HexToAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")
	AddrUSDTEthereum = common.HexToAddress("0xdAC17F958D2ee523a2206206994597C13D831ec7")
	AddrDAIEthereum  = common.HexToAddress("0x6B175474E89094C44Da98b954EescdeCB5dC3f38")

	// Wrapped
	AddrWETHEthereum = common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
	AddrWBTCEthereum = common.HexToAddress("0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599")
)

// Well-known AssetIDs
var (
	// Ethereum Mainnet
	IDEthereumETH  = NewNativeAssetID(ChainIDEthereum)
	IDEthereumUSDC = NewTokenAssetID(ChainIDEthereum, AddrUSDCEthereum)
	IDEthereumUSDT = NewTokenAssetID(ChainIDEthereum, AddrUSDTEthereum)
	IDEthereumWETH = NewTokenAssetID(ChainIDEthereum, AddrWETHEthereum)
	IDEthereumWBTC = NewTokenAssetID(ChainIDEthereum, AddrWBTCEthereum)

	// Fiat
	IDUSD = NewFiatAssetID("USD")
	IDEUR = NewFiatAssetID("EUR")
	IDARS = NewFiatAssetID("ARS")
)

// Well-known Assets (pre-created instances)
var (
	// Ethereum Mainnet
	ETH  = NewAssetWithName(IDEthereumETH, "ETH", "Ethereum", 18)
	USDC = NewAssetWithName(IDEthereumUSDC, "USDC", "USD Coin", 6)
	USDT = NewAssetWithName(IDEthereumUSDT, "USDT", "Tether USD", 6)
	WETH = NewAssetWithName(IDEthereumWETH, "WETH", "Wrapped Ether", 18)
	WBTC = NewAssetWithName(IDEthereumWBTC, "WBTC", "Wrapped Bitcoin", 8)

	// Fiat
	USD = NewAssetWithName(IDUSD, "USD", "US Dollar", 2)
	EUR = NewAssetWithName(IDEUR, "EUR", "Euro", 2)
	ARS = NewAssetWithName(IDARS, "ARS", "Argentine Peso", 2)
)

// DefaultRegistry returns a registry pre-populated with well-known assets.
func DefaultRegistry() *Registry {
	r := NewRegistry()

	// Ethereum Mainnet
	r.Register(ETH)
	r.Register(USDC)
	r.Register(USDT)
	r.Register(WETH)
	r.Register(WBTC)

	// Fiat
	r.Register(USD)
	r.Register(EUR)
	r.Register(ARS)

	return r
}

// MustNewToken creates a new ERC20 token asset with the given parameters.
// This is a convenience function for registering custom tokens.
func MustNewToken(chainID uint64, address common.Address, symbol, name string, decimals uint8) *Asset {
	id := NewTokenAssetID(chainID, address)
	return NewAssetWithName(id, symbol, name, decimals)
}

// MustNewNative creates a new native coin asset.
func MustNewNative(chainID uint64, symbol, name string, decimals uint8) *Asset {
	id := NewNativeAssetID(chainID)
	return NewAssetWithName(id, symbol, name, decimals)
}
