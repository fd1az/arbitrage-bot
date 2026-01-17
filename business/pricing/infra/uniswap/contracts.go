package uniswap

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// Fee tiers in Uniswap V3 (in hundredths of a bip)
const (
	FeeTier001 = 100   // 0.01%
	FeeTier005 = 500   // 0.05%
	FeeTier030 = 3000  // 0.30%
	FeeTier100 = 10000 // 1.00%
)

// QuoterV2ABI is the ABI for the Uniswap V3 QuoterV2 contract.
// Only includes quoteExactInputSingle which we use for quotes.
const QuoterV2ABI = `[
	{
		"inputs": [
			{
				"components": [
					{"internalType": "address", "name": "tokenIn", "type": "address"},
					{"internalType": "address", "name": "tokenOut", "type": "address"},
					{"internalType": "uint256", "name": "amountIn", "type": "uint256"},
					{"internalType": "uint24", "name": "fee", "type": "uint24"},
					{"internalType": "uint160", "name": "sqrtPriceLimitX96", "type": "uint160"}
				],
				"internalType": "struct IQuoterV2.QuoteExactInputSingleParams",
				"name": "params",
				"type": "tuple"
			}
		],
		"name": "quoteExactInputSingle",
		"outputs": [
			{"internalType": "uint256", "name": "amountOut", "type": "uint256"},
			{"internalType": "uint160", "name": "sqrtPriceX96After", "type": "uint160"},
			{"internalType": "uint32", "name": "initializedTicksCrossed", "type": "uint32"},
			{"internalType": "uint256", "name": "gasEstimate", "type": "uint256"}
		],
		"stateMutability": "nonpayable",
		"type": "function"
	}
]`

// QuoteExactInputSingleParams represents the input params for quoteExactInputSingle.
type QuoteExactInputSingleParams struct {
	TokenIn           common.Address
	TokenOut          common.Address
	AmountIn          *big.Int
	Fee               *big.Int // uint24
	SqrtPriceLimitX96 *big.Int // uint160, 0 for no limit
}

// QuoteResult represents the output of quoteExactInputSingle.
type QuoteResult struct {
	AmountOut               *big.Int
	SqrtPriceX96After       *big.Int
	InitializedTicksCrossed uint32
	GasEstimate             *big.Int
}
