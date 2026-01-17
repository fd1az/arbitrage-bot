// Package wsconn provides a production-grade WebSocket client with reconnection,
// exponential backoff, and full OTEL instrumentation.
package wsconn

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerName = "github.com/fd1az/arbitrage-bot/internal/wsconn"
	meterName  = "github.com/fd1az/arbitrage-bot/internal/wsconn"
)

// State represents the connection state.
type State string

const (
	StateDisconnected State = "disconnected"
	StateConnecting   State = "connecting"
	StateConnected    State = "connected"
	StateReconnecting State = "reconnecting"
	StateClosed       State = "closed"
)

// Config holds WebSocket client configuration.
type Config struct {
	URL            string
	Name           string        // Identifier for metrics/tracing
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	MaxReconnects  int           // 0 = infinite
	PingInterval   time.Duration
	ReadTimeout    time.Duration
	WriteTimeout   time.Duration
	BufferSize     int
	MaxMessageSize int64 // Max message size in bytes (0 = no limit)
}

// DefaultConfig returns sensible defaults.
func DefaultConfig(url string, name string) Config {
	return Config{
		URL:            url,
		Name:           name,
		InitialBackoff: 1 * time.Second,
		MaxBackoff:     30 * time.Second,
		MaxReconnects:  0, // infinite
		PingInterval:   30 * time.Second,
		ReadTimeout:    60 * time.Second,
		WriteTimeout:   10 * time.Second,
		BufferSize:     1024,             // Increased from 256 to reduce message drops
		MaxMessageSize: 10 * 1024 * 1024, // 10MB
	}
}

// MessageHandler is called when a message is received.
type MessageHandler func(ctx context.Context, msg []byte)

// StateChangeHandler is called when connection state changes.
type StateChangeHandler func(state State, err error)

// metrics holds OTEL metric instruments.
type metrics struct {
	connectionState  metric.Int64Gauge
	messagesReceived metric.Int64Counter
	messagesSent     metric.Int64Counter
	reconnectsTotal  metric.Int64Counter
	droppedMessages  metric.Int64Counter
	messageLatency   metric.Float64Histogram
	bytesReceived    metric.Int64Counter
	bytesSent        metric.Int64Counter
	pingsTotal       metric.Int64Counter
	pingsFailed      metric.Int64Counter
}

// Client is a production-grade WebSocket client with OTEL instrumentation.
type Client struct {
	config Config
	conn   *websocket.Conn
	connMu sync.RWMutex

	state   State
	stateMu sync.RWMutex

	messages chan []byte
	done     chan struct{}
	closeMu  sync.Mutex
	closed   atomic.Bool

	reconnects   int
	reconnectsMu sync.Mutex

	tracer  trace.Tracer
	metrics *metrics

	handlersMu    sync.RWMutex
	onMessage     MessageHandler
	onStateChange StateChangeHandler

	connectedAt time.Time
	stopPing    chan struct{}
}

// New creates a new WebSocket client with OTEL instrumentation.
func New(config Config) (*Client, error) {
	c := &Client{
		config:   config,
		state:    StateDisconnected,
		messages: make(chan []byte, config.BufferSize),
		done:     make(chan struct{}),
		stopPing: make(chan struct{}),
		tracer:   otel.Tracer(tracerName),
	}

	if err := c.initMetrics(); err != nil {
		return nil, fmt.Errorf("failed to init metrics: %w", err)
	}

	return c, nil
}

// initMetrics initializes OTEL metric instruments.
func (c *Client) initMetrics() error {
	meter := otel.Meter(meterName)

	var err error

	c.metrics = &metrics{}

	c.metrics.connectionState, err = meter.Int64Gauge(
		"ws_connection_state",
		metric.WithDescription("WebSocket connection state (0=disconnected, 1=connecting, 2=connected, 3=reconnecting, 4=closed)"),
		metric.WithUnit("{state}"),
	)
	if err != nil {
		return err
	}

	c.metrics.messagesReceived, err = meter.Int64Counter(
		"ws_messages_received_total",
		metric.WithDescription("Total number of WebSocket messages received"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return err
	}

	c.metrics.messagesSent, err = meter.Int64Counter(
		"ws_messages_sent_total",
		metric.WithDescription("Total number of WebSocket messages sent"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return err
	}

	c.metrics.reconnectsTotal, err = meter.Int64Counter(
		"ws_reconnects_total",
		metric.WithDescription("Total number of WebSocket reconnection attempts"),
		metric.WithUnit("{attempt}"),
	)
	if err != nil {
		return err
	}

	c.metrics.droppedMessages, err = meter.Int64Counter(
		"ws_messages_dropped_total",
		metric.WithDescription("Total number of WebSocket messages dropped due to full buffer"),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		return err
	}

	c.metrics.messageLatency, err = meter.Float64Histogram(
		"ws_message_latency_ms",
		metric.WithDescription("WebSocket message processing latency in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return err
	}

	c.metrics.bytesReceived, err = meter.Int64Counter(
		"ws_bytes_received_total",
		metric.WithDescription("Total bytes received over WebSocket"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return err
	}

	c.metrics.bytesSent, err = meter.Int64Counter(
		"ws_bytes_sent_total",
		metric.WithDescription("Total bytes sent over WebSocket"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return err
	}

	c.metrics.pingsTotal, err = meter.Int64Counter(
		"ws_pings_total",
		metric.WithDescription("Total WebSocket ping attempts"),
		metric.WithUnit("{ping}"),
	)
	if err != nil {
		return err
	}

	c.metrics.pingsFailed, err = meter.Int64Counter(
		"ws_pings_failed_total",
		metric.WithDescription("Total WebSocket ping failures"),
		metric.WithUnit("{ping}"),
	)
	if err != nil {
		return err
	}

	return nil
}

// OnMessage sets the message handler.
func (c *Client) OnMessage(handler MessageHandler) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.onMessage = handler
}

// OnStateChange sets the state change handler.
func (c *Client) OnStateChange(handler StateChangeHandler) {
	c.handlersMu.Lock()
	defer c.handlersMu.Unlock()
	c.onStateChange = handler
}

// Connect establishes the WebSocket connection.
func (c *Client) Connect(ctx context.Context) error {
	ctx, span := c.tracer.Start(ctx, "ws.connect",
		trace.WithAttributes(
			attribute.String("ws.url", c.config.URL),
			attribute.String("ws.name", c.config.Name),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	c.setState(StateConnecting)

	conn, _, err := websocket.Dial(ctx, c.config.URL, &websocket.DialOptions{
		CompressionMode: websocket.CompressionContextTakeover,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "connection failed")
		c.setState(StateDisconnected)
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	// Set max message size limit to prevent OOM from malicious/large messages
	if c.config.MaxMessageSize > 0 {
		conn.SetReadLimit(c.config.MaxMessageSize)
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	c.connectedAt = time.Now()
	c.setState(StateConnected)
	span.SetStatus(codes.Ok, "connected")
	span.AddEvent("connection established")

	// Start read loop with background context (not tied to connection context)
	go c.readLoop(context.Background())

	// Start ping loop for heartbeat
	go c.startPingLoop(context.Background())

	return nil
}

// startPingLoop sends periodic pings to detect half-open connections.
func (c *Client) startPingLoop(ctx context.Context) {
	if c.config.PingInterval <= 0 {
		return
	}

	ticker := time.NewTicker(c.config.PingInterval)
	defer ticker.Stop()

	attrs := metric.WithAttributes(attribute.String("ws.name", c.config.Name))

	for {
		select {
		case <-c.done:
			return
		case <-c.stopPing:
			return
		case <-ticker.C:
			c.connMu.RLock()
			conn := c.conn
			c.connMu.RUnlock()

			if conn == nil {
				return
			}

			pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := conn.Ping(pingCtx)
			cancel()

			if err != nil {
				c.metrics.pingsFailed.Add(ctx, 1, attrs)
				c.handleDisconnect(ctx, fmt.Errorf("ping failed: %w", err))
				return
			}
			c.metrics.pingsTotal.Add(ctx, 1, attrs)
		}
	}
}

// ConnectWithRetry establishes connection with exponential backoff retry.
func (c *Client) ConnectWithRetry(ctx context.Context) error {
	ctx, span := c.tracer.Start(ctx, "ws.connect_with_retry",
		trace.WithAttributes(
			attribute.String("ws.url", c.config.URL),
			attribute.String("ws.name", c.config.Name),
			attribute.Int("ws.max_reconnects", c.config.MaxReconnects),
		),
		trace.WithSpanKind(trace.SpanKindClient),
	)
	defer span.End()

	backoff := c.config.InitialBackoff
	attempts := 0

	for {
		select {
		case <-ctx.Done():
			span.RecordError(ctx.Err())
			span.SetStatus(codes.Error, "context cancelled")
			return ctx.Err()
		default:
		}

		if c.closed.Load() {
			return errors.New("client is closed")
		}

		err := c.Connect(ctx)
		if err == nil {
			span.SetStatus(codes.Ok, "connected")
			span.SetAttributes(attribute.Int("ws.connect_attempts", attempts+1))
			return nil
		}

		attempts++
		if c.config.MaxReconnects > 0 && attempts >= c.config.MaxReconnects {
			span.RecordError(err)
			span.SetStatus(codes.Error, "max reconnects exceeded")
			return fmt.Errorf("max reconnects (%d) exceeded: %w", c.config.MaxReconnects, err)
		}

		// Calculate backoff with jitter
		jitter := time.Duration(rand.Int63n(int64(backoff) / 2))
		sleepDuration := backoff + jitter

		span.AddEvent("reconnect scheduled",
			trace.WithAttributes(
				attribute.Int("attempt", attempts),
				attribute.String("backoff", sleepDuration.String()),
			),
		)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleepDuration):
		}

		// Exponential backoff
		backoff *= 2
		if backoff > c.config.MaxBackoff {
			backoff = c.config.MaxBackoff
		}
	}
}

// readLoop continuously reads messages from the WebSocket.
func (c *Client) readLoop(ctx context.Context) {
	attrs := []attribute.KeyValue{
		attribute.String("ws.name", c.config.Name),
	}

	for {
		select {
		case <-c.done:
			return
		case <-ctx.Done():
			return
		default:
		}

		c.connMu.RLock()
		conn := c.conn
		c.connMu.RUnlock()

		if conn == nil {
			return
		}

		// Set read deadline
		readCtx := ctx
		var cancel context.CancelFunc
		if c.config.ReadTimeout > 0 {
			readCtx, cancel = context.WithTimeout(ctx, c.config.ReadTimeout)
		}

		start := time.Now()
		msgType, data, err := conn.Read(readCtx)
		latency := float64(time.Since(start).Milliseconds())

		// Cancel context immediately after use (not defer - would leak in loop)
		if cancel != nil {
			cancel()
		}

		if err != nil {
			if c.closed.Load() {
				return
			}

			// Handle reconnection
			if websocket.CloseStatus(err) != -1 || errors.Is(err, context.DeadlineExceeded) {
				c.handleDisconnect(ctx, err)
				return
			}

			_, span := c.tracer.Start(ctx, "ws.read_error",
				trace.WithAttributes(attrs...),
			)
			span.RecordError(err)
			span.SetStatus(codes.Error, "read failed")
			span.End()

			c.handleDisconnect(ctx, err)
			return
		}

		if msgType == websocket.MessageText || msgType == websocket.MessageBinary {
			_, span := c.tracer.Start(ctx, "ws.message.recv",
				trace.WithAttributes(
					append(attrs,
						attribute.Int("ws.message.size", len(data)),
						attribute.String("ws.message.type", msgType.String()),
					)...,
				),
			)

			c.metrics.messagesReceived.Add(ctx, 1, metric.WithAttributes(attrs...))
			c.metrics.bytesReceived.Add(ctx, int64(len(data)), metric.WithAttributes(attrs...))
			c.metrics.messageLatency.Record(ctx, latency, metric.WithAttributes(attrs...))

			// Send to channel (non-blocking to prevent read loop stall)
			select {
			case c.messages <- data:
			default:
				// Buffer full - drop message but track it
				c.metrics.droppedMessages.Add(ctx, 1, metric.WithAttributes(attrs...))
				span.AddEvent("message dropped - buffer full",
					trace.WithAttributes(attribute.Int("buffer_size", c.config.BufferSize)))
			}

			// Call handler if set (with mutex protection)
			c.handlersMu.RLock()
			handler := c.onMessage
			c.handlersMu.RUnlock()
			if handler != nil {
				handler(ctx, data)
			}

			span.SetStatus(codes.Ok, "message received")
			span.End()
		}
	}
}

// handleDisconnect handles connection loss and initiates reconnection.
func (c *Client) handleDisconnect(ctx context.Context, err error) {
	if c.closed.Load() {
		return
	}

	ctx, span := c.tracer.Start(ctx, "ws.disconnect",
		trace.WithAttributes(
			attribute.String("ws.name", c.config.Name),
		),
	)
	defer span.End()

	if err != nil {
		span.RecordError(err)
	}

	c.setState(StateReconnecting)

	c.connMu.Lock()
	if c.conn != nil {
		c.conn.Close(websocket.StatusGoingAway, "reconnecting")
		c.conn = nil
	}
	c.connMu.Unlock()

	// Attempt reconnection
	go c.reconnect(ctx)
}

// reconnect attempts to reconnect with exponential backoff.
func (c *Client) reconnect(ctx context.Context) {
	c.reconnectsMu.Lock()
	c.reconnects++
	attempt := c.reconnects
	c.reconnectsMu.Unlock()

	ctx, span := c.tracer.Start(ctx, "ws.reconnect",
		trace.WithAttributes(
			attribute.String("ws.name", c.config.Name),
			attribute.Int("ws.reconnect.attempt", attempt),
		),
	)
	defer span.End()

	c.metrics.reconnectsTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("ws.name", c.config.Name),
	))

	backoff := c.config.InitialBackoff
	for i := 1; i < attempt; i++ {
		backoff *= 2
		if backoff > c.config.MaxBackoff {
			backoff = c.config.MaxBackoff
			break
		}
	}

	// Add jitter
	jitter := time.Duration(rand.Int63n(int64(backoff) / 2))
	sleepDuration := backoff + jitter

	span.AddEvent("waiting before reconnect",
		trace.WithAttributes(
			attribute.String("backoff", sleepDuration.String()),
		),
	)

	select {
	case <-ctx.Done():
		span.RecordError(ctx.Err())
		return
	case <-c.done:
		return
	case <-time.After(sleepDuration):
	}

	if c.closed.Load() {
		return
	}

	if c.config.MaxReconnects > 0 && attempt > c.config.MaxReconnects {
		span.SetStatus(codes.Error, "max reconnects exceeded")
		c.setState(StateDisconnected)
		c.handlersMu.RLock()
		stateHandler := c.onStateChange
		c.handlersMu.RUnlock()
		if stateHandler != nil {
			stateHandler(StateDisconnected, errors.New("max reconnects exceeded"))
		}
		return
	}

	err := c.Connect(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "reconnect failed")
		// Try again
		go c.reconnect(ctx)
		return
	}

	// Reset reconnect counter on successful connection
	c.reconnectsMu.Lock()
	c.reconnects = 0
	c.reconnectsMu.Unlock()

	span.SetStatus(codes.Ok, "reconnected")
}

// Send sends a message through the WebSocket.
func (c *Client) Send(ctx context.Context, msg []byte) error {
	ctx, span := c.tracer.Start(ctx, "ws.message.send",
		trace.WithAttributes(
			attribute.String("ws.name", c.config.Name),
			attribute.Int("ws.message.size", len(msg)),
		),
	)
	defer span.End()

	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil {
		err := errors.New("not connected")
		span.RecordError(err)
		span.SetStatus(codes.Error, "not connected")
		return err
	}

	writeCtx := ctx
	if c.config.WriteTimeout > 0 {
		var cancel context.CancelFunc
		writeCtx, cancel = context.WithTimeout(ctx, c.config.WriteTimeout)
		defer cancel()
	}

	start := time.Now()
	err := conn.Write(writeCtx, websocket.MessageText, msg)
	latency := float64(time.Since(start).Milliseconds())

	attrs := metric.WithAttributes(attribute.String("ws.name", c.config.Name))

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "send failed")
		return fmt.Errorf("websocket write failed: %w", err)
	}

	c.metrics.messagesSent.Add(ctx, 1, attrs)
	c.metrics.bytesSent.Add(ctx, int64(len(msg)), attrs)
	c.metrics.messageLatency.Record(ctx, latency, attrs)

	span.SetStatus(codes.Ok, "sent")
	return nil
}

// SendJSON sends a JSON message through the WebSocket.
func (c *Client) SendJSON(ctx context.Context, v interface{}) error {
	c.connMu.RLock()
	conn := c.conn
	c.connMu.RUnlock()

	if conn == nil {
		return errors.New("not connected")
	}

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	return c.Send(ctx, data)
}

// Messages returns the channel for receiving messages.
func (c *Client) Messages() <-chan []byte {
	return c.messages
}

// State returns the current connection state.
func (c *Client) State() State {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.state
}

// IsConnected returns true if the client is connected.
func (c *Client) IsConnected() bool {
	return c.State() == StateConnected
}

// Close gracefully closes the WebSocket connection.
func (c *Client) Close() error {
	c.closeMu.Lock()
	defer c.closeMu.Unlock()

	if c.closed.Load() {
		return nil
	}

	_, span := c.tracer.Start(context.Background(), "ws.close",
		trace.WithAttributes(
			attribute.String("ws.name", c.config.Name),
		),
	)
	defer span.End()

	c.closed.Store(true)
	close(c.done)

	c.connMu.Lock()
	conn := c.conn
	c.conn = nil
	c.connMu.Unlock()

	if conn != nil {
		if err := conn.Close(websocket.StatusNormalClosure, "client closing"); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "close error")
			return err
		}
	}

	c.setState(StateClosed)
	span.SetStatus(codes.Ok, "closed")

	return nil
}

// setState updates the connection state and records metrics.
func (c *Client) setState(state State) {
	c.stateMu.Lock()
	oldState := c.state
	c.state = state
	c.stateMu.Unlock()

	if oldState == state {
		return
	}

	// Record state as metric
	stateValue := int64(0)
	switch state {
	case StateDisconnected:
		stateValue = 0
	case StateConnecting:
		stateValue = 1
	case StateConnected:
		stateValue = 2
	case StateReconnecting:
		stateValue = 3
	case StateClosed:
		stateValue = 4
	}

	c.metrics.connectionState.Record(context.Background(), stateValue,
		metric.WithAttributes(attribute.String("ws.name", c.config.Name)),
	)

	// Call handler if set (with mutex protection)
	c.handlersMu.RLock()
	stateHandler := c.onStateChange
	c.handlersMu.RUnlock()
	if stateHandler != nil {
		stateHandler(state, nil)
	}
}

// ReconnectCount returns the current reconnect attempt count.
func (c *Client) ReconnectCount() int {
	c.reconnectsMu.Lock()
	defer c.reconnectsMu.Unlock()
	return c.reconnects
}
