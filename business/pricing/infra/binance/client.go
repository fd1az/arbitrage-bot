package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/fd1az/arbitrage-bot/internal/apperror"
	"github.com/fd1az/arbitrage-bot/internal/logger"
	"github.com/fd1az/arbitrage-bot/internal/wsconn"
)

// Ensure interface compliance
var _ logger.LoggerInterface = (*logger.Logger)(nil)

const (
	tracerName = "binance"
	meterName  = "binance"

	// Binance WebSocket endpoints
	BaseWSURL     = "wss://stream.binance.com:9443"
	BaseWSURLAlt  = "wss://stream.binance.com:443"
	DataStreamURL = "wss://data-stream.binance.vision"
	// Binance US endpoint (for users in USA)
	BaseWSURLUS = "wss://stream.binance.us:9443"

	// Keep-alive interval (Binance requires message every 3 min)
	keepAliveInterval = 2 * time.Minute
)

// ClientConfig holds configuration for the Binance client.
type ClientConfig struct {
	BaseURL      string        // WebSocket base URL
	Symbols      []string      // Symbols to subscribe (e.g., "ETHUSDC")
	DepthSpeedMs int           // Depth update speed (100 or 1000)
	ReadTimeout  time.Duration // Read timeout
	WriteTimeout time.Duration // Write timeout
}

// DefaultClientConfig returns sensible defaults.
func DefaultClientConfig(symbols []string) ClientConfig {
	return ClientConfig{
		BaseURL:      BaseWSURL,
		Symbols:      symbols,
		DepthSpeedMs: 100,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
}

// clientMetrics holds OTEL metric instruments.
type clientMetrics struct {
	messagesReceived metric.Int64Counter
	tradesReceived   metric.Int64Counter
	depthUpdates     metric.Int64Counter
	subscriptions    metric.Int64UpDownCounter
	parseErrors      metric.Int64Counter
}

// Client is a Binance WebSocket client.
type Client struct {
	config ClientConfig
	logger logger.LoggerInterface

	conn   *wsconn.Client
	connMu sync.RWMutex

	// Message handlers
	onAggTrade    func(*AggTradeEvent)
	onDepthUpdate func(*PartialDepthEvent) // Uses PartialDepthEvent for @depth20 streams
	onBookTicker  func(*BookTickerEvent)
	handlersMu    sync.RWMutex

	// Subscription management
	subscriptions map[string]struct{}
	subsMu        sync.RWMutex
	nextID        atomic.Int64

	// Keep-alive
	stopKeepAlive chan struct{}

	// Observability
	tracer  trace.Tracer
	metrics *clientMetrics

	// State
	running atomic.Bool
}

// NewClient creates a new Binance WebSocket client.
func NewClient(cfg ClientConfig, log logger.LoggerInterface) (*Client, error) {
	c := &Client{
		config:        cfg,
		logger:        log,
		subscriptions: make(map[string]struct{}),
		stopKeepAlive: make(chan struct{}),
		tracer:        otel.Tracer(tracerName),
	}

	if err := c.initMetrics(); err != nil {
		return nil, fmt.Errorf("init metrics: %w", err)
	}

	return c, nil
}

func (c *Client) initMetrics() error {
	meter := otel.Meter(meterName)
	var err error

	c.metrics = &clientMetrics{}

	c.metrics.messagesReceived, err = meter.Int64Counter(
		"binance_messages_total",
		metric.WithDescription("Total messages received"),
	)
	if err != nil {
		return err
	}

	c.metrics.tradesReceived, err = meter.Int64Counter(
		"binance_trades_total",
		metric.WithDescription("Total trades received"),
	)
	if err != nil {
		return err
	}

	c.metrics.depthUpdates, err = meter.Int64Counter(
		"binance_depth_updates_total",
		metric.WithDescription("Total depth updates received"),
	)
	if err != nil {
		return err
	}

	c.metrics.subscriptions, err = meter.Int64UpDownCounter(
		"binance_subscriptions",
		metric.WithDescription("Active subscriptions"),
	)
	if err != nil {
		return err
	}

	c.metrics.parseErrors, err = meter.Int64Counter(
		"binance_parse_errors_total",
		metric.WithDescription("Message parse errors"),
	)
	if err != nil {
		return err
	}

	return nil
}

// OnAggTrade registers a handler for aggregate trade events.
func (c *Client) OnAggTrade(handler func(*AggTradeEvent)) {
	c.handlersMu.Lock()
	c.onAggTrade = handler
	c.handlersMu.Unlock()
}

// OnDepthUpdate registers a handler for partial depth events (@depth20 streams).
func (c *Client) OnDepthUpdate(handler func(*PartialDepthEvent)) {
	c.handlersMu.Lock()
	c.onDepthUpdate = handler
	c.handlersMu.Unlock()
}

// OnBookTicker registers a handler for book ticker events.
func (c *Client) OnBookTicker(handler func(*BookTickerEvent)) {
	c.handlersMu.Lock()
	c.onBookTicker = handler
	c.handlersMu.Unlock()
}

// Connect establishes the WebSocket connection and subscribes to streams.
func (c *Client) Connect(ctx context.Context) error {
	ctx, span := c.tracer.Start(ctx, "binance.connect",
		trace.WithAttributes(
			attribute.StringSlice("symbols", c.config.Symbols),
		),
	)
	defer span.End()

	// Build combined streams URL
	wsURL, err := c.buildStreamURL()
	if err != nil {
		return err
	}

	// Create wsconn configuration
	wsCfg := wsconn.DefaultConfig(wsURL, "binance")
	wsCfg.ReadTimeout = c.config.ReadTimeout
	wsCfg.WriteTimeout = c.config.WriteTimeout

	// Create connection
	conn, err := wsconn.New(wsCfg)
	if err != nil {
		return apperror.New(apperror.CodeBinanceConnectionFailed,
			apperror.WithCause(err),
			apperror.WithContext("failed to create wsconn"))
	}

	// Set message handler
	conn.OnMessage(c.handleMessage)

	// Connect
	if err := conn.ConnectWithRetry(ctx); err != nil {
		return apperror.New(apperror.CodeBinanceConnectionFailed,
			apperror.WithCause(err),
			apperror.WithContext("failed to connect to Binance"))
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	// Mark streams as subscribed (combined URL auto-subscribes)
	c.subsMu.Lock()
	for _, sym := range c.config.Symbols {
		c.subscriptions[BookTickerStream(sym)] = struct{}{}
		c.subscriptions[DepthStream(sym, c.config.DepthSpeedMs)] = struct{}{}
	}
	c.subsMu.Unlock()

	c.metrics.subscriptions.Add(ctx, int64(len(c.config.Symbols)*2))

	// Start keep-alive
	c.running.Store(true)
	go c.keepAlive(ctx)

	c.logger.Info(ctx, "binance client connected",
		"url", wsURL,
		"symbols", c.config.Symbols)

	return nil
}

// buildStreamURL constructs the combined streams WebSocket URL.
func (c *Client) buildStreamURL() (string, error) {
	if len(c.config.Symbols) == 0 {
		return "", apperror.New(apperror.CodeConfigurationError,
			apperror.WithContext("no symbols configured"))
	}

	// Build stream list - bookTicker + depth for VWAP calculations
	streams := make([]string, 0, len(c.config.Symbols)*2)
	for _, sym := range c.config.Symbols {
		// Book ticker for best bid/ask
		bookTickerStream := BookTickerStream(sym)
		streams = append(streams, bookTickerStream)

		// Depth stream for VWAP calculations on larger trade sizes
		depthStream := DepthStream(sym, c.config.DepthSpeedMs)
		streams = append(streams, depthStream)
	}

	// Combined streams URL: /stream?streams=stream1/stream2/...
	u, err := url.Parse(c.config.BaseURL)
	if err != nil {
		return "", err
	}
	u.Path = "/stream"
	u.RawQuery = "streams=" + strings.Join(streams, "/")

	finalURL := u.String()
	c.logger.Info(context.Background(), "built websocket URL", "url", finalURL, "streams", streams)

	return finalURL, nil
}

// handleMessage processes incoming WebSocket messages.
func (c *Client) handleMessage(ctx context.Context, data []byte) {
	c.metrics.messagesReceived.Add(ctx, 1)

	// Parse stream wrapper
	var event StreamEvent
	if err := json.Unmarshal(data, &event); err != nil {
		// Might be a subscription response
		var resp WSResponse
		if json.Unmarshal(data, &resp) == nil {
			c.logger.Debug(ctx, "subscription response received")
			return // Ignore subscription confirmations
		}
		c.metrics.parseErrors.Add(ctx, 1)
		c.logger.Debug(ctx, "failed to parse message", "error", err, "data", string(data[:min(len(data), 500)]))
		return
	}

	// Route by stream type
	c.routeStreamEvent(ctx, &event)
}

// routeStreamEvent routes the event to the appropriate handler.
func (c *Client) routeStreamEvent(ctx context.Context, event *StreamEvent) {
	stream := event.Stream

	switch {
	case strings.HasSuffix(stream, "@bookTicker"):
		var ticker BookTickerEvent
		if err := json.Unmarshal(event.Data, &ticker); err != nil {
			c.metrics.parseErrors.Add(ctx, 1)
			return
		}
		c.handlersMu.RLock()
		handler := c.onBookTicker
		c.handlersMu.RUnlock()
		if handler != nil {
			handler(&ticker)
		}

	case strings.Contains(stream, "@depth"):
		var depth PartialDepthEvent
		if err := json.Unmarshal(event.Data, &depth); err != nil {
			c.metrics.parseErrors.Add(ctx, 1)
			c.logger.Warn(ctx, "failed to parse partial depth", "error", err, "data", string(event.Data[:min(len(event.Data), 200)]))
			return
		}
		// Extract symbol from stream name (e.g., "ethusdc@depth20@100ms" -> "ETHUSDC")
		depth.Symbol = extractSymbolFromStream(stream)
		c.metrics.depthUpdates.Add(ctx, 1)
		c.handlersMu.RLock()
		handler := c.onDepthUpdate
		c.handlersMu.RUnlock()
		if handler != nil {
			handler(&depth)
		}

	case strings.HasSuffix(stream, "@aggTrade"):
		var trade AggTradeEvent
		if err := json.Unmarshal(event.Data, &trade); err != nil {
			c.metrics.parseErrors.Add(ctx, 1)
			return
		}
		c.metrics.tradesReceived.Add(ctx, 1)
		c.handlersMu.RLock()
		handler := c.onAggTrade
		c.handlersMu.RUnlock()
		if handler != nil {
			handler(&trade)
		}
	}
}

// Subscribe adds a new subscription (if connected).
func (c *Client) Subscribe(ctx context.Context, streams ...string) error {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil {
		return apperror.New(apperror.CodeBinanceConnectionFailed,
			apperror.WithContext("not connected"))
	}

	req := WSRequest{
		Method: "SUBSCRIBE",
		Params: streams,
		ID:     c.nextID.Add(1),
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	if err := conn.Send(ctx, data); err != nil {
		return apperror.New(apperror.CodeBinanceConnectionFailed,
			apperror.WithCause(err),
			apperror.WithContext("failed to subscribe"))
	}

	c.subsMu.Lock()
	for _, s := range streams {
		c.subscriptions[s] = struct{}{}
	}
	c.subsMu.Unlock()

	c.metrics.subscriptions.Add(ctx, int64(len(streams)))

	return nil
}

// Unsubscribe removes subscriptions.
func (c *Client) Unsubscribe(ctx context.Context, streams ...string) error {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil {
		return nil
	}

	req := WSRequest{
		Method: "UNSUBSCRIBE",
		Params: streams,
		ID:     c.nextID.Add(1),
	}

	data, err := json.Marshal(req)
	if err != nil {
		return err
	}

	if err := conn.Send(ctx, data); err != nil {
		return err
	}

	c.subsMu.Lock()
	for _, s := range streams {
		delete(c.subscriptions, s)
	}
	c.subsMu.Unlock()

	c.metrics.subscriptions.Add(ctx, -int64(len(streams)))

	return nil
}

// keepAlive sends periodic pings to keep the connection alive.
func (c *Client) keepAlive(ctx context.Context) {
	ticker := time.NewTicker(keepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopKeepAlive:
			return
		case <-ticker.C:
			if !c.running.Load() {
				return
			}

			c.connMu.RLock()
			conn := c.conn
			c.connMu.RUnlock()

			if conn != nil {
				// Send a list_subscriptions request as keep-alive
				req := WSRequest{
					Method: "LIST_SUBSCRIPTIONS",
					ID:     c.nextID.Add(1),
				}
				data, _ := json.Marshal(req)
				if err := conn.Send(ctx, data); err != nil {
					c.logger.Warn(ctx, "keep-alive failed", "error", err)
				}
			}
		}
	}
}

// Close closes the client connection.
func (c *Client) Close() error {
	c.running.Store(false)
	close(c.stopKeepAlive)

	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsConnected returns whether the client is connected.
func (c *Client) IsConnected() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()
	return c.conn != nil && c.conn.IsConnected()
}

// extractSymbolFromStream extracts the symbol from a stream name.
// Example: "ethusdc@depth20@100ms" -> "ETHUSDC"
func extractSymbolFromStream(stream string) string {
	// Stream format: <symbol>@<stream_type>[@speed]
	idx := strings.Index(stream, "@")
	if idx > 0 {
		// Convert to uppercase (Binance uses lowercase in streams)
		return strings.ToUpper(stream[:idx])
	}
	return stream
}
