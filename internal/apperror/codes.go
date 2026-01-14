package apperror

// Code represents a unique error code for the application
type Code string

// General error codes
const (
	// General validation
	CodeRequiredField   Code = "REQUIRED_FIELD"
	CodeInvalidInput    Code = "INVALID_INPUT"
	CodeInvalidFormat   Code = "INVALID_FORMAT"
	CodeInvalidState    Code = "INVALID_STATE"
	CodeNotFound        Code = "NOT_FOUND"
	CodeValidationError Code = "VALIDATION_ERROR"

	// Configuration
	CodeConfigurationError Code = "CONFIGURATION_ERROR"

	// External service errors
	CodeExternalServiceError Code = "EXTERNAL_SERVICE_ERROR"
	CodeServiceTimeout       Code = "SERVICE_TIMEOUT"
	CodeServiceUnavailable   Code = "SERVICE_UNAVAILABLE"
	CodeRateLimitExceeded    Code = "RATE_LIMIT_EXCEEDED"

	// System errors
	CodeInternalError Code = "INTERNAL_ERROR"
	CodeUnknownError  Code = "UNKNOWN_ERROR"
)

// Arbitrage-specific error codes
const (
	// Blockchain/Ethereum errors
	CodeEthereumConnectionFailed Code = "ETHEREUM_CONNECTION_FAILED"
	CodeEthereumSubscribeFailed  Code = "ETHEREUM_SUBSCRIBE_FAILED"
	CodeEthereumRPCError         Code = "ETHEREUM_RPC_ERROR"
	CodeBlockNotFound            Code = "BLOCK_NOT_FOUND"
	CodeGasEstimationFailed      Code = "GAS_ESTIMATION_FAILED"

	// WebSocket errors
	CodeWebSocketConnectionError Code = "WEBSOCKET_CONNECTION_ERROR"
	CodeWebSocketReconnecting    Code = "WEBSOCKET_RECONNECTING"
	CodeWebSocketClosed          Code = "WEBSOCKET_CLOSED"
	CodeWebSocketSendError       Code = "WEBSOCKET_SEND_ERROR"

	// CEX (Binance) errors
	CodeBinanceConnectionFailed Code = "BINANCE_CONNECTION_FAILED"
	CodeBinanceAPIError         Code = "BINANCE_API_ERROR"
	CodeBinanceRateLimited      Code = "BINANCE_RATE_LIMITED"
	CodeOrderbookFetchFailed    Code = "ORDERBOOK_FETCH_FAILED"
	CodeInvalidOrderbook        Code = "INVALID_ORDERBOOK"

	// DEX (Uniswap) errors
	CodeUniswapQuoteFailed  Code = "UNISWAP_QUOTE_FAILED"
	CodeUniswapPoolNotFound Code = "UNISWAP_POOL_NOT_FOUND"
	CodeInvalidQuote        Code = "INVALID_QUOTE"
	CodeContractCallFailed  Code = "CONTRACT_CALL_FAILED"

	// Arbitrage detection errors
	CodePriceCalculationFailed Code = "PRICE_CALCULATION_FAILED"
	CodeSpreadCalculationError Code = "SPREAD_CALCULATION_ERROR"
	CodeInsufficientLiquidity  Code = "INSUFFICIENT_LIQUIDITY"
	CodeInvalidTradeSize       Code = "INVALID_TRADE_SIZE"

	// Cache errors
	CodeCacheMiss    Code = "CACHE_MISS"
	CodeCacheExpired Code = "CACHE_EXPIRED"

	// Circuit breaker errors
	CodeCircuitOpen     Code = "CIRCUIT_OPEN"
	CodeCircuitHalfOpen Code = "CIRCUIT_HALF_OPEN"
)
