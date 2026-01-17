// Package ethereum provides Ethereum blockchain infrastructure adapters.
package ethereum

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/sony/gobreaker/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/fd1az/arbitrage-bot/business/blockchain/domain"
	"github.com/fd1az/arbitrage-bot/internal/apperror"
	"github.com/fd1az/arbitrage-bot/internal/circuitbreaker"
	"github.com/fd1az/arbitrage-bot/internal/logger"
)

const (
	tracerName = "github.com/fd1az/arbitrage-bot/business/blockchain/infra/ethereum"
	meterName  = "github.com/fd1az/arbitrage-bot/business/blockchain/infra/ethereum"
)

// SubscriberConfig holds configuration for the Ethereum subscriber.
type SubscriberConfig struct {
	WSURL          string        // WebSocket endpoint (primary)
	HTTPURL        string        // HTTP endpoint (fallback)
	PollInterval   time.Duration // Polling interval for HTTP fallback
	ReconnectDelay time.Duration // Delay before reconnecting WS
	BufferSize     int           // Block channel buffer size
}

// DefaultSubscriberConfig returns sensible defaults.
func DefaultSubscriberConfig(wsURL, httpURL string) SubscriberConfig {
	return SubscriberConfig{
		WSURL:          wsURL,
		HTTPURL:        httpURL,
		PollInterval:   12 * time.Second, // ~1 block time
		ReconnectDelay: 5 * time.Second,
		BufferSize:     16,
	}
}

// subscriberMetrics holds OTEL metric instruments.
type subscriberMetrics struct {
	blocksReceived   metric.Int64Counter
	subscribeErrors  metric.Int64Counter
	connectionState  metric.Int64Gauge
	blockLatency     metric.Float64Histogram
	httpFallbackUsed metric.Int64Counter
}

// Subscriber implements BlockSubscriber using go-ethereum client.
// It uses WebSocket as primary with HTTP polling as fallback.
type Subscriber struct {
	config SubscriberConfig
	logger logger.LoggerInterface

	// Clients
	wsClient   *ethclient.Client
	httpClient *ethclient.Client
	clientMu   sync.RWMutex

	// State
	state      domain.ConnectionState
	stateMu    sync.RWMutex
	usingHTTP  atomic.Bool
	lastBlock  atomic.Uint64
	reconnects atomic.Int32

	// Channels
	blocks     chan *domain.Block
	done       chan struct{}
	closeMu    sync.Mutex
	closed     atomic.Bool

	// Circuit breakers
	wsCB   *circuitbreaker.CircuitBreaker[*types.Header]
	httpCB *circuitbreaker.CircuitBreaker[*types.Header]

	// Observability
	tracer  trace.Tracer
	metrics *subscriberMetrics
}

// NewSubscriber creates a new Ethereum block subscriber.
func NewSubscriber(cfg SubscriberConfig, log logger.LoggerInterface) (*Subscriber, error) {
	s := &Subscriber{
		config: cfg,
		logger: log,
		state:  domain.StateDisconnected,
		blocks: make(chan *domain.Block, cfg.BufferSize),
		done:   make(chan struct{}),
		tracer: otel.Tracer(tracerName),
	}

	if err := s.initMetrics(); err != nil {
		return nil, fmt.Errorf("init metrics: %w", err)
	}

	s.initCircuitBreakers()

	return s, nil
}

// initMetrics initializes OTEL metric instruments.
func (s *Subscriber) initMetrics() error {
	meter := otel.Meter(meterName)
	var err error

	s.metrics = &subscriberMetrics{}

	s.metrics.blocksReceived, err = meter.Int64Counter(
		"eth_blocks_received_total",
		metric.WithDescription("Total Ethereum blocks received"),
		metric.WithUnit("{block}"),
	)
	if err != nil {
		return err
	}

	s.metrics.subscribeErrors, err = meter.Int64Counter(
		"eth_subscribe_errors_total",
		metric.WithDescription("Total Ethereum subscription errors"),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		return err
	}

	s.metrics.connectionState, err = meter.Int64Gauge(
		"eth_connection_state",
		metric.WithDescription("Ethereum connection state (0=disconnected, 1=connecting, 2=connected, 3=reconnecting)"),
		metric.WithUnit("{state}"),
	)
	if err != nil {
		return err
	}

	s.metrics.blockLatency, err = meter.Float64Histogram(
		"eth_block_latency_ms",
		metric.WithDescription("Latency from block timestamp to receipt"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return err
	}

	s.metrics.httpFallbackUsed, err = meter.Int64Counter(
		"eth_http_fallback_total",
		metric.WithDescription("Times HTTP fallback was used"),
		metric.WithUnit("{fallback}"),
	)
	if err != nil {
		return err
	}

	return nil
}

// initCircuitBreakers initializes circuit breakers for WS and HTTP.
func (s *Subscriber) initCircuitBreakers() {
	wsCfg := circuitbreaker.DefaultConfig("eth-ws")
	wsCfg.OnStateChange = func(name string, from, to gobreaker.State) {
		s.logger.Info(context.Background(), "circuit breaker state change",
			"breaker", name, "from", from.String(), "to", to.String())
	}
	s.wsCB = circuitbreaker.New[*types.Header](wsCfg)

	httpCfg := circuitbreaker.DefaultConfig("eth-http")
	httpCfg.OnStateChange = func(name string, from, to gobreaker.State) {
		s.logger.Info(context.Background(), "circuit breaker state change",
			"breaker", name, "from", from.String(), "to", to.String())
	}
	s.httpCB = circuitbreaker.New[*types.Header](httpCfg)
}

// Subscribe starts listening for new blocks and returns a channel.
func (s *Subscriber) Subscribe(ctx context.Context) (<-chan *domain.Block, error) {
	ctx, span := s.tracer.Start(ctx, "eth.subscribe",
		trace.WithAttributes(
			attribute.String("ws_url", s.config.WSURL),
			attribute.String("http_url", s.config.HTTPURL),
		),
	)
	defer span.End()

	if s.closed.Load() {
		err := errors.New("subscriber is closed")
		span.RecordError(err)
		return nil, err
	}

	s.setState(domain.StateConnecting)

	// Try WebSocket first
	if err := s.connectWS(ctx); err != nil {
		s.logger.Warn(ctx, "ws connection failed, trying http fallback", "error", err)
		span.AddEvent("ws_failed_trying_http")

		// Fall back to HTTP
		if err := s.connectHTTP(ctx); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "both connections failed")
			s.setState(domain.StateDisconnected)
			return nil, apperror.New(apperror.CodeEthereumConnectionFailed,
				apperror.WithCause(err),
				apperror.WithContext("failed to connect via WS and HTTP"))
		}

		s.usingHTTP.Store(true)
		go s.runHTTPPoller(ctx)
	} else {
		go s.runWSSubscription(ctx)
	}

	s.setState(domain.StateConnected)
	span.SetStatus(codes.Ok, "subscribed")

	return s.blocks, nil
}

// connectWS establishes a WebSocket connection to the Ethereum node.
func (s *Subscriber) connectWS(ctx context.Context) error {
	ctx, span := s.tracer.Start(ctx, "eth.connect.ws",
		trace.WithAttributes(attribute.String("url", s.config.WSURL)),
	)
	defer span.End()

	if s.config.WSURL == "" {
		return errors.New("ws url not configured")
	}

	client, err := ethclient.DialContext(ctx, s.config.WSURL)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "dial failed")
		return fmt.Errorf("dial ws: %w", err)
	}

	s.clientMu.Lock()
	s.wsClient = client
	s.clientMu.Unlock()

	span.SetStatus(codes.Ok, "connected")
	return nil
}

// connectHTTP establishes an HTTP connection to the Ethereum node.
func (s *Subscriber) connectHTTP(ctx context.Context) error {
	ctx, span := s.tracer.Start(ctx, "eth.connect.http",
		trace.WithAttributes(attribute.String("url", s.config.HTTPURL)),
	)
	defer span.End()

	if s.config.HTTPURL == "" {
		return errors.New("http url not configured")
	}

	client, err := ethclient.DialContext(ctx, s.config.HTTPURL)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "dial failed")
		return fmt.Errorf("dial http: %w", err)
	}

	s.clientMu.Lock()
	s.httpClient = client
	s.clientMu.Unlock()

	span.SetStatus(codes.Ok, "connected")
	return nil
}

// runWSSubscription runs the WebSocket subscription loop.
func (s *Subscriber) runWSSubscription(ctx context.Context) {
	headers := make(chan *types.Header, s.config.BufferSize)

	for {
		select {
		case <-s.done:
			return
		case <-ctx.Done():
			return
		default:
		}

		s.clientMu.RLock()
		client := s.wsClient
		s.clientMu.RUnlock()

		if client == nil {
			s.handleWSDisconnect(ctx)
			return
		}

		// Subscribe to new heads
		sub, err := client.SubscribeNewHead(ctx, headers)
		if err != nil {
			s.logger.Error(ctx, "subscribe new head failed", "error", err)
			s.metrics.subscribeErrors.Add(ctx, 1)
			s.handleWSDisconnect(ctx)
			return
		}

		s.logger.Info(ctx, "subscribed to new heads via ws")

		// Process headers until error
		s.processWSHeaders(ctx, headers, sub)

		// If we get here, subscription ended - try to reconnect
		sub.Unsubscribe()
		s.handleWSDisconnect(ctx)
		return
	}
}

// processWSHeaders processes incoming block headers from WebSocket.
func (s *Subscriber) processWSHeaders(ctx context.Context, headers <-chan *types.Header, sub interface{ Err() <-chan error }) {
	for {
		select {
		case <-s.done:
			return
		case <-ctx.Done():
			return
		case err := <-sub.Err():
			if err != nil {
				s.logger.Error(ctx, "subscription error", "error", err)
				s.metrics.subscribeErrors.Add(ctx, 1)
			}
			return
		case header := <-headers:
			if header == nil {
				continue
			}
			s.processHeader(ctx, header, false)
		}
	}
}

// handleWSDisconnect handles WebSocket disconnection and fallback.
func (s *Subscriber) handleWSDisconnect(ctx context.Context) {
	if s.closed.Load() {
		return
	}

	s.setState(domain.StateReconnecting)
	s.reconnects.Add(1)

	// Try to reconnect WS
	time.Sleep(s.config.ReconnectDelay)

	if s.closed.Load() {
		return
	}

	if err := s.connectWS(ctx); err != nil {
		s.logger.Warn(ctx, "ws reconnect failed, switching to http", "error", err)

		// Switch to HTTP fallback
		if s.httpClient == nil {
			if err := s.connectHTTP(ctx); err != nil {
				s.logger.Error(ctx, "http fallback connection failed", "error", err)
				s.setState(domain.StateDisconnected)
				return
			}
		}

		s.usingHTTP.Store(true)
		s.metrics.httpFallbackUsed.Add(ctx, 1)
		s.setState(domain.StateConnected)
		go s.runHTTPPoller(ctx)
		return
	}

	s.usingHTTP.Store(false)
	s.setState(domain.StateConnected)
	go s.runWSSubscription(ctx)
}

// runHTTPPoller runs the HTTP polling loop as fallback.
func (s *Subscriber) runHTTPPoller(ctx context.Context) {
	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	s.logger.Info(ctx, "starting http polling fallback", "interval", s.config.PollInterval)

	for {
		select {
		case <-s.done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.pollLatestBlock(ctx)
		}
	}
}

// pollLatestBlock fetches the latest block via HTTP.
func (s *Subscriber) pollLatestBlock(ctx context.Context) {
	ctx, span := s.tracer.Start(ctx, "eth.poll.block")
	defer span.End()

	s.clientMu.RLock()
	client := s.httpClient
	s.clientMu.RUnlock()

	if client == nil {
		span.AddEvent("no_http_client")
		return
	}

	// Execute through circuit breaker
	header, err := s.httpCB.Execute(func() (*types.Header, error) {
		return client.HeaderByNumber(ctx, nil) // nil = latest
	})

	if err != nil {
		span.RecordError(err)
		s.logger.Error(ctx, "http poll failed", "error", err)
		s.metrics.subscribeErrors.Add(ctx, 1)
		return
	}

	// Check if this is a new block
	if header.Number.Uint64() <= s.lastBlock.Load() {
		span.AddEvent("duplicate_block")
		return
	}

	s.processHeader(ctx, header, true)
	span.SetStatus(codes.Ok, "polled")
}

// processHeader converts and emits a block header.
func (s *Subscriber) processHeader(ctx context.Context, header *types.Header, fromHTTP bool) {
	ctx, span := s.tracer.Start(ctx, "eth.process.header",
		trace.WithAttributes(
			attribute.Int64("block_number", int64(header.Number.Uint64())),
			attribute.Bool("from_http", fromHTTP),
		),
	)
	defer span.End()

	block := s.headerToBlock(header)

	// Calculate latency
	latency := time.Since(block.Timestamp)
	s.metrics.blockLatency.Record(ctx, float64(latency.Milliseconds()))

	// Update last block
	s.lastBlock.Store(block.Number)

	// Emit block (non-blocking)
	select {
	case s.blocks <- block:
		s.metrics.blocksReceived.Add(ctx, 1)
		s.logger.Debug(ctx, "block received",
			"number", block.Number,
			"hash", block.Hash.Hex()[:10],
			"latency_ms", latency.Milliseconds())
	default:
		span.AddEvent("block_dropped_buffer_full")
		s.logger.Warn(ctx, "block dropped, buffer full", "number", block.Number)
	}

	span.SetStatus(codes.Ok, "processed")
}

// headerToBlock converts an Ethereum header to domain Block.
func (s *Subscriber) headerToBlock(header *types.Header) *domain.Block {
	return &domain.Block{
		Number:     header.Number.Uint64(),
		Hash:       header.Hash(),
		ParentHash: header.ParentHash,
		Timestamp:  time.Unix(int64(header.Time), 0),
		GasLimit:   header.GasLimit,
		GasUsed:    header.GasUsed,
		BaseFee:    header.BaseFee,
	}
}

// LatestBlock retrieves the most recent block.
func (s *Subscriber) LatestBlock(ctx context.Context) (*domain.Block, error) {
	ctx, span := s.tracer.Start(ctx, "eth.latest_block")
	defer span.End()

	// Try WS client first, then HTTP
	s.clientMu.RLock()
	wsClient := s.wsClient
	httpClient := s.httpClient
	s.clientMu.RUnlock()

	var header *types.Header
	var err error

	if wsClient != nil && !s.usingHTTP.Load() {
		header, err = s.wsCB.Execute(func() (*types.Header, error) {
			return wsClient.HeaderByNumber(ctx, nil)
		})
	}

	if header == nil && httpClient != nil {
		header, err = s.httpCB.Execute(func() (*types.Header, error) {
			return httpClient.HeaderByNumber(ctx, nil)
		})
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "fetch failed")
		return nil, apperror.New(apperror.CodeBlockNotFound,
			apperror.WithCause(err),
			apperror.WithContext("failed to fetch latest block"))
	}

	if header == nil {
		err := errors.New("no client available")
		span.RecordError(err)
		return nil, apperror.New(apperror.CodeEthereumConnectionFailed,
			apperror.WithContext("no ethereum client connected"))
	}

	span.SetStatus(codes.Ok, "fetched")
	return s.headerToBlock(header), nil
}

// State returns the current connection state.
func (s *Subscriber) State() domain.ConnectionState {
	s.stateMu.RLock()
	defer s.stateMu.RUnlock()
	return s.state
}

// Status returns detailed connection status.
func (s *Subscriber) Status() domain.ConnectionStatus {
	return domain.ConnectionStatus{
		State:      s.State(),
		LastBlock:  s.lastBlock.Load(),
		LastUpdate: time.Now(),
		Reconnects: int(s.reconnects.Load()),
		UsingHTTP:  s.usingHTTP.Load(),
	}
}

// Close gracefully closes the subscriber.
func (s *Subscriber) Close() error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	if s.closed.Load() {
		return nil
	}

	s.logger.Info(context.Background(), "closing ethereum subscriber")

	s.closed.Store(true)
	close(s.done)

	s.clientMu.Lock()
	if s.wsClient != nil {
		s.wsClient.Close()
		s.wsClient = nil
	}
	if s.httpClient != nil {
		s.httpClient.Close()
		s.httpClient = nil
	}
	s.clientMu.Unlock()

	close(s.blocks)
	s.setState(domain.StateDisconnected)

	return nil
}

// setState updates the connection state and records metrics.
func (s *Subscriber) setState(state domain.ConnectionState) {
	s.stateMu.Lock()
	s.state = state
	s.stateMu.Unlock()

	stateValue := int64(0)
	switch state {
	case domain.StateDisconnected:
		stateValue = 0
	case domain.StateConnecting:
		stateValue = 1
	case domain.StateConnected:
		stateValue = 2
	case domain.StateReconnecting:
		stateValue = 3
	}

	s.metrics.connectionState.Record(context.Background(), stateValue)
}

// BlockNumber returns the current block number from the last received block.
func (s *Subscriber) BlockNumber() uint64 {
	return s.lastBlock.Load()
}

// GetChainID returns the chain ID from the connected client.
func (s *Subscriber) GetChainID(ctx context.Context) (*big.Int, error) {
	ctx, span := s.tracer.Start(ctx, "eth.chain_id")
	defer span.End()

	s.clientMu.RLock()
	wsClient := s.wsClient
	httpClient := s.httpClient
	s.clientMu.RUnlock()

	var client *ethclient.Client
	if wsClient != nil && !s.usingHTTP.Load() {
		client = wsClient
	} else if httpClient != nil {
		client = httpClient
	}

	if client == nil {
		return nil, apperror.New(apperror.CodeEthereumConnectionFailed,
			apperror.WithContext("no ethereum client connected"))
	}

	chainID, err := client.ChainID(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, apperror.New(apperror.CodeEthereumRPCError,
			apperror.WithCause(err),
			apperror.WithContext("failed to get chain id"))
	}

	span.SetStatus(codes.Ok, "fetched")
	return chainID, nil
}
