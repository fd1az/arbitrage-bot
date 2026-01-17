package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/fd1az/arbitrage-bot/internal/apperror"
	"github.com/fd1az/arbitrage-bot/internal/httpclient"
	"github.com/fd1az/arbitrage-bot/internal/logger"
)

const (
	// Binance REST API endpoints
	BaseAPIURL   = "https://api.binance.com"
	BaseAPIURLUS = "https://api.binance.us"

	// Endpoints
	depthEndpoint = "/api/v3/depth"

	// Default HTTP client settings
	httpTimeout = 10 * time.Second
)

// HTTPClientConfig holds configuration for the Binance HTTP client.
type HTTPClientConfig struct {
	BaseURL string        // API base URL (empty = default)
	Timeout time.Duration // Request timeout
}

// DefaultHTTPClientConfig returns sensible defaults.
func DefaultHTTPClientConfig() HTTPClientConfig {
	return HTTPClientConfig{
		BaseURL: BaseAPIURL,
		Timeout: httpTimeout,
	}
}

// HTTPClient provides Binance REST API access for fallback scenarios.
type HTTPClient struct {
	client httpclient.Client
	config HTTPClientConfig
	logger logger.LoggerInterface
	tracer trace.Tracer
}

// NewHTTPClient creates a new Binance HTTP client.
func NewHTTPClient(cfg HTTPClientConfig, log logger.LoggerInterface) (*HTTPClient, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = BaseAPIURL
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = httpTimeout
	}

	tracer := otel.Tracer(tracerName)

	client, err := httpclient.NewInstrumentedClient(
		httpclient.WithProviderName("binance"),
		httpclient.WithBaseURL(baseURL),
		httpclient.WithRequestTimeout(timeout),
		httpclient.WithTraceOptions(tracer, httpclient.TraceRequest, httpclient.TraceResponse),
		httpclient.WithHeaders(map[string]string{
			"Accept": "application/json",
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	return &HTTPClient{
		client: client,
		config: cfg,
		logger: log,
		tracer: tracer,
	}, nil
}

// DepthResponse is the REST API response for orderbook depth.
type DepthResponse struct {
	LastUpdateID int64      `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"` // [[price, qty], ...]
	Asks         [][]string `json:"asks"` // [[price, qty], ...]
}

// GetDepth fetches the orderbook depth for a symbol via REST API.
// This is used as a fallback when WebSocket data is stale or unavailable.
func (c *HTTPClient) GetDepth(ctx context.Context, symbol string, limit int) (*DepthResponse, error) {
	ctx, span := c.tracer.Start(ctx, "binance.http.get_depth",
		trace.WithAttributes(
			attribute.String("symbol", symbol),
			attribute.Int("limit", limit),
		),
	)
	defer span.End()

	// Validate limit (Binance accepts: 5, 10, 20, 50, 100, 500, 1000, 5000)
	validLimits := map[int]bool{5: true, 10: true, 20: true, 50: true, 100: true, 500: true, 1000: true, 5000: true}
	if !validLimits[limit] {
		limit = 20 // Default to 20 levels
	}

	var result DepthResponse
	resp, err := c.client.NewRequestWithOptions(
		httpclient.WithLabels(
			httpclient.NewLabel("endpoint", "depth"),
			httpclient.NewLabel("symbol", symbol),
		),
		httpclient.WithResponseErrorHandler(binanceErrorHandler),
	).
		SetQueryParam("symbol", symbol).
		SetQueryParam("limit", strconv.Itoa(limit)).
		SetResult(&result).
		Get(ctx, depthEndpoint)

	if err != nil {
		span.RecordError(err)
		return nil, apperror.New(apperror.CodeBinanceConnectionFailed,
			apperror.WithCause(err),
			apperror.WithContext("failed to fetch depth from REST API"))
	}

	if resp.IsError() {
		return nil, apperror.New(apperror.CodeBinanceConnectionFailed,
			apperror.WithContext(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.String())))
	}

	span.SetAttributes(
		attribute.Int("bids", len(result.Bids)),
		attribute.Int("asks", len(result.Asks)),
		attribute.Int64("last_update_id", result.LastUpdateID),
	)

	c.logger.Debug(ctx, "fetched depth via HTTP",
		"symbol", symbol,
		"bids", len(result.Bids),
		"asks", len(result.Asks))

	return &result, nil
}

// ToPartialDepthEvent converts a DepthResponse to a PartialDepthEvent.
// This allows the HTTP response to be processed the same way as WebSocket data.
func (d *DepthResponse) ToPartialDepthEvent(symbol string) *PartialDepthEvent {
	return &PartialDepthEvent{
		LastUpdateID: d.LastUpdateID,
		Bids:         d.Bids,
		Asks:         d.Asks,
		Symbol:       symbol,
	}
}

// BinanceAPIError represents an error response from Binance API.
type BinanceAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
}

func (e *BinanceAPIError) Error() string {
	return fmt.Sprintf("binance API error %d: %s", e.Code, e.Message)
}

// binanceErrorHandler parses Binance API error responses.
func binanceErrorHandler(statusCode int, body []byte) error {
	if statusCode >= 400 {
		var apiErr BinanceAPIError
		if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Code != 0 {
			return &apiErr
		}
		return fmt.Errorf("HTTP %d: %s", statusCode, string(body))
	}
	return nil
}
