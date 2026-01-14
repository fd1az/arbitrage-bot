package apperror

// messages maps error codes to human-readable messages
var messages = map[Code]string{
	// General validation
	CodeRequiredField:   "Required field is missing",
	CodeInvalidInput:    "Invalid input provided",
	CodeInvalidFormat:   "Invalid data format",
	CodeInvalidState:    "Invalid state for this operation",
	CodeNotFound:        "Resource not found",
	CodeValidationError: "Validation error",

	// Configuration
	CodeConfigurationError: "Configuration error",

	// External service errors
	CodeExternalServiceError: "External service error",
	CodeServiceTimeout:       "Service request timeout",
	CodeServiceUnavailable:   "Service temporarily unavailable",
	CodeRateLimitExceeded:    "Rate limit exceeded",

	// System errors
	CodeInternalError: "Internal server error",
	CodeUnknownError:  "An unknown error occurred",

	// Blockchain/Ethereum errors
	CodeEthereumConnectionFailed: "Failed to connect to Ethereum node",
	CodeEthereumSubscribeFailed:  "Failed to subscribe to Ethereum events",
	CodeEthereumRPCError:         "Ethereum RPC call failed",
	CodeBlockNotFound:            "Block not found",
	CodeGasEstimationFailed:      "Gas estimation failed",

	// WebSocket errors
	CodeWebSocketConnectionError: "WebSocket connection error",
	CodeWebSocketReconnecting:    "WebSocket reconnecting",
	CodeWebSocketClosed:          "WebSocket connection closed",
	CodeWebSocketSendError:       "Failed to send WebSocket message",

	// CEX (Binance) errors
	CodeBinanceConnectionFailed: "Failed to connect to Binance API",
	CodeBinanceAPIError:         "Binance API error",
	CodeBinanceRateLimited:      "Binance rate limit exceeded",
	CodeOrderbookFetchFailed:    "Failed to fetch orderbook",
	CodeInvalidOrderbook:        "Invalid orderbook data",

	// DEX (Uniswap) errors
	CodeUniswapQuoteFailed:  "Failed to get Uniswap quote",
	CodeUniswapPoolNotFound: "Uniswap pool not found",
	CodeInvalidQuote:        "Invalid quote data",
	CodeContractCallFailed:  "Smart contract call failed",

	// Arbitrage detection errors
	CodePriceCalculationFailed: "Price calculation failed",
	CodeSpreadCalculationError: "Spread calculation error",
	CodeInsufficientLiquidity:  "Insufficient liquidity for trade size",
	CodeInvalidTradeSize:       "Invalid trade size",

	// Cache errors
	CodeCacheMiss:    "Cache miss",
	CodeCacheExpired: "Cache entry expired",

	// Circuit breaker errors
	CodeCircuitOpen:     "Circuit breaker is open",
	CodeCircuitHalfOpen: "Circuit breaker is half-open",
}
