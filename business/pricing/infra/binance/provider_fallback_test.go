package binance

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fd1az/arbitrage-bot/business/pricing/domain"
	"github.com/fd1az/arbitrage-bot/internal/asset"
	"github.com/fd1az/arbitrage-bot/internal/logger"
	"github.com/shopspring/decimal"
)

// mockLogger implements logger.LoggerInterface for testing.
type mockLogger struct{}

func (m *mockLogger) Debug(ctx context.Context, msg string, args ...any)              {}
func (m *mockLogger) Info(ctx context.Context, msg string, args ...any)               {}
func (m *mockLogger) Warn(ctx context.Context, msg string, args ...any)               {}
func (m *mockLogger) Error(ctx context.Context, msg string, args ...any)              {}
func (m *mockLogger) Debugc(ctx context.Context, caller int, msg string, args ...any) {}
func (m *mockLogger) Infoc(ctx context.Context, caller int, msg string, args ...any)  {}
func (m *mockLogger) Warnc(ctx context.Context, caller int, msg string, args ...any)  {}
func (m *mockLogger) Errorc(ctx context.Context, caller int, msg string, args ...any) {}

var _ logger.LoggerInterface = (*mockLogger)(nil)

// TestProvider_FallbackToHTTP tests that the provider falls back to HTTP
// when WebSocket data is stale or unavailable.
func TestProvider_FallbackToHTTP(t *testing.T) {
	// Create mock HTTP server that returns orderbook data
	mockDepthResponse := DepthResponse{
		LastUpdateID: 12345,
		Bids: [][]string{
			{"3400.50", "10.5"},
			{"3400.00", "20.0"},
			{"3399.50", "15.0"},
		},
		Asks: [][]string{
			{"3401.00", "8.0"},
			{"3401.50", "12.0"},
			{"3402.00", "25.0"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request params
		symbol := r.URL.Query().Get("symbol")
		if symbol != "ETHUSDC" {
			t.Errorf("expected symbol ETHUSDC, got %s", symbol)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockDepthResponse)
	}))
	defer server.Close()

	// Create provider with HTTP fallback pointing to mock server
	cfg := ProviderConfig{
		Symbols:        []string{"ETHUSDC"},
		DepthSpeedMs:   100,
		SnapshotDepth:  20,
		StaleTimeout:   100 * time.Millisecond, // Very short for testing
		EnableFallback: true,
		HTTPURL:        server.URL,
	}

	log := &mockLogger{}
	provider, err := NewProvider(cfg, log)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	// Don't connect WebSocket - simulate no WS data scenario
	// The orderbook state will be empty, triggering HTTP fallback

	ctx := context.Background()
	pair := domain.Pair{
		Base:  asset.ETH,
		Quote: asset.USDC,
	}

	// Test 1: No WebSocket data - should fallback to HTTP
	t.Run("fallback_when_no_ws_data", func(t *testing.T) {
		ob, err := provider.GetOrderbook(ctx, pair)
		if err != nil {
			t.Fatalf("expected HTTP fallback to succeed, got error: %v", err)
		}

		// Verify orderbook data from HTTP
		if len(ob.Bids) != 3 {
			t.Errorf("expected 3 bids, got %d", len(ob.Bids))
		}
		if len(ob.Asks) != 3 {
			t.Errorf("expected 3 asks, got %d", len(ob.Asks))
		}

		// Verify prices
		expectedBidPrice := decimal.RequireFromString("3400.50")
		if !ob.Bids[0].Price.Equal(expectedBidPrice) {
			t.Errorf("expected first bid price %s, got %s", expectedBidPrice, ob.Bids[0].Price)
		}

		expectedAskPrice := decimal.RequireFromString("3401.00")
		if !ob.Asks[0].Price.Equal(expectedAskPrice) {
			t.Errorf("expected first ask price %s, got %s", expectedAskPrice, ob.Asks[0].Price)
		}
	})

	// Test 2: Stale WebSocket data - should fallback to HTTP
	t.Run("fallback_when_ws_data_stale", func(t *testing.T) {
		// Manually set stale data in the orderbook state
		provider.booksMu.RLock()
		state := provider.orderbooks["ETHUSDC"]
		provider.booksMu.RUnlock()

		staleAmt, _ := asset.ParseDecimal(asset.ETH, decimal.NewFromInt(1))
		state.mu.Lock()
		state.bids = []domain.OrderbookLevel{
			{Price: decimal.NewFromInt(3000), Amount: staleAmt},
		}
		state.asks = []domain.OrderbookLevel{
			{Price: decimal.NewFromInt(3001), Amount: staleAmt},
		}
		state.lastUpdate = time.Now().Add(-1 * time.Hour) // Very stale
		state.mu.Unlock()

		ob, err := provider.GetOrderbook(ctx, pair)
		if err != nil {
			t.Fatalf("expected HTTP fallback to succeed on stale data, got error: %v", err)
		}

		// Should get fresh HTTP data, not the stale WS data
		expectedBidPrice := decimal.RequireFromString("3400.50")
		if !ob.Bids[0].Price.Equal(expectedBidPrice) {
			t.Errorf("expected HTTP fallback price %s, got stale price %s", expectedBidPrice, ob.Bids[0].Price)
		}
	})

	// Test 3: Fresh WebSocket data - should NOT fallback to HTTP
	t.Run("no_fallback_when_ws_data_fresh", func(t *testing.T) {
		// Set fresh data in the orderbook state
		provider.booksMu.RLock()
		state := provider.orderbooks["ETHUSDC"]
		provider.booksMu.RUnlock()

		freshPrice := decimal.RequireFromString("3500.00")
		freshAmt, _ := asset.ParseDecimal(asset.ETH, decimal.NewFromInt(5))
		state.mu.Lock()
		state.bids = []domain.OrderbookLevel{
			{Price: freshPrice, Amount: freshAmt},
		}
		state.asks = []domain.OrderbookLevel{
			{Price: decimal.RequireFromString("3501.00"), Amount: freshAmt},
		}
		state.lastUpdate = time.Now() // Fresh!
		state.mu.Unlock()

		ob, err := provider.GetOrderbook(ctx, pair)
		if err != nil {
			t.Fatalf("expected success with fresh WS data, got error: %v", err)
		}

		// Should get WS data, not HTTP data
		if !ob.Bids[0].Price.Equal(freshPrice) {
			t.Errorf("expected WS price %s, got %s (HTTP fallback was used incorrectly)", freshPrice, ob.Bids[0].Price)
		}
	})
}

// TestProvider_FallbackDisabled tests that HTTP fallback is not used when disabled.
func TestProvider_FallbackDisabled(t *testing.T) {
	cfg := ProviderConfig{
		Symbols:        []string{"ETHUSDC"},
		DepthSpeedMs:   100,
		SnapshotDepth:  20,
		StaleTimeout:   100 * time.Millisecond,
		EnableFallback: false, // Disabled!
	}

	log := &mockLogger{}
	provider, err := NewProvider(cfg, log)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}

	ctx := context.Background()
	pair := domain.Pair{
		Base:  asset.ETH,
		Quote: asset.USDC,
	}

	// With no WS data and fallback disabled, should get error
	_, err = provider.GetOrderbook(ctx, pair)
	if err == nil {
		t.Error("expected error when no WS data and fallback disabled, got nil")
	}
}

// TestHTTPClient_GetDepth tests the HTTP client depth endpoint.
func TestHTTPClient_GetDepth(t *testing.T) {
	mockResponse := DepthResponse{
		LastUpdateID: 99999,
		Bids: [][]string{
			{"2500.00", "100.0"},
			{"2499.50", "200.0"},
		},
		Asks: [][]string{
			{"2500.50", "150.0"},
			{"2501.00", "250.0"},
		},
	}

	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		// Verify endpoint
		if r.URL.Path != "/api/v3/depth" {
			t.Errorf("expected path /api/v3/depth, got %s", r.URL.Path)
		}

		// Verify params
		symbol := r.URL.Query().Get("symbol")
		limit := r.URL.Query().Get("limit")
		if symbol != "BTCUSDC" {
			t.Errorf("expected symbol BTCUSDC, got %s", symbol)
		}
		if limit != "20" {
			t.Errorf("expected limit 20, got %s", limit)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	cfg := HTTPClientConfig{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	}

	client, err := NewHTTPClient(cfg, &mockLogger{})
	if err != nil {
		t.Fatalf("failed to create HTTP client: %v", err)
	}

	ctx := context.Background()
	depth, err := client.GetDepth(ctx, "BTCUSDC", 20)
	if err != nil {
		t.Fatalf("GetDepth failed: %v", err)
	}

	// Verify response
	if depth.LastUpdateID != 99999 {
		t.Errorf("expected lastUpdateId 99999, got %d", depth.LastUpdateID)
	}
	if len(depth.Bids) != 2 {
		t.Errorf("expected 2 bids, got %d", len(depth.Bids))
	}
	if len(depth.Asks) != 2 {
		t.Errorf("expected 2 asks, got %d", len(depth.Asks))
	}
	if requestCount != 1 {
		t.Errorf("expected 1 HTTP request, got %d", requestCount)
	}
}

// TestHTTPClient_GetDepth_Error tests HTTP client error handling.
func TestHTTPClient_GetDepth_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"code": -1121,
			"msg":  "Invalid symbol.",
		})
	}))
	defer server.Close()

	cfg := HTTPClientConfig{
		BaseURL: server.URL,
	}

	client, err := NewHTTPClient(cfg, &mockLogger{})
	if err != nil {
		t.Fatalf("failed to create HTTP client: %v", err)
	}

	ctx := context.Background()
	_, err = client.GetDepth(ctx, "INVALID", 20)
	if err == nil {
		t.Error("expected error for invalid symbol, got nil")
	}

	// Verify it's a Binance API error
	if apiErr, ok := err.(*BinanceAPIError); ok {
		if apiErr.Code != -1121 {
			t.Errorf("expected error code -1121, got %d", apiErr.Code)
		}
	}
}

// TestDepthResponse_ToPartialDepthEvent tests conversion helper.
func TestDepthResponse_ToPartialDepthEvent(t *testing.T) {
	resp := &DepthResponse{
		LastUpdateID: 12345,
		Bids:         [][]string{{"100", "10"}},
		Asks:         [][]string{{"101", "20"}},
	}

	event := resp.ToPartialDepthEvent("ETHUSDC")

	if event.Symbol != "ETHUSDC" {
		t.Errorf("expected symbol ETHUSDC, got %s", event.Symbol)
	}
	if event.LastUpdateID != 12345 {
		t.Errorf("expected lastUpdateId 12345, got %d", event.LastUpdateID)
	}
	if len(event.Bids) != 1 || len(event.Asks) != 1 {
		t.Errorf("expected 1 bid and 1 ask, got %d bids and %d asks", len(event.Bids), len(event.Asks))
	}
}
