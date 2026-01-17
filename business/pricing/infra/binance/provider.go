package binance

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/fd1az/arbitrage-bot/business/pricing/app"
	"github.com/fd1az/arbitrage-bot/business/pricing/domain"
	"github.com/fd1az/arbitrage-bot/internal/apperror"
	"github.com/fd1az/arbitrage-bot/internal/asset"
	"github.com/fd1az/arbitrage-bot/internal/logger"
	"github.com/shopspring/decimal"
)

// Ensure Provider implements CEXProvider.
var _ app.CEXProvider = (*Provider)(nil)

// ProviderConfig holds configuration for the Binance provider.
type ProviderConfig struct {
	WebSocketURL   string        // WebSocket base URL (empty = default)
	HTTPURL        string        // REST API base URL (empty = default)
	Symbols        []string      // Trading symbols (e.g., "ETHUSDC", "BTCUSDC")
	DepthSpeedMs   int           // Depth update speed (100ms recommended)
	SnapshotDepth  int           // Number of orderbook levels to maintain
	StaleTimeout   time.Duration // How long before data is considered stale
	EnableFallback bool          // Enable HTTP fallback when WS data is stale
}

// DefaultProviderConfig returns sensible defaults.
func DefaultProviderConfig(symbols []string) ProviderConfig {
	return ProviderConfig{
		Symbols:        symbols,
		DepthSpeedMs:   100,
		SnapshotDepth:  20,
		StaleTimeout:   5 * time.Second,
		EnableFallback: true, // Enable HTTP fallback by default
	}
}

// orderbookState holds the current orderbook for a symbol.
type orderbookState struct {
	bids       []domain.OrderbookLevel
	asks       []domain.OrderbookLevel
	lastUpdate time.Time
	mu         sync.RWMutex
}

// Provider implements CEXProvider for Binance.
type Provider struct {
	config     ProviderConfig
	logger     logger.LoggerInterface
	client     *Client     // WebSocket client
	httpClient *HTTPClient // HTTP client for fallback

	// Orderbook state per symbol
	orderbooks map[string]*orderbookState
	booksMu    sync.RWMutex

	// Asset registry for conversions
	registry *asset.Registry

	// Observability
	tracer trace.Tracer
}

// NewProvider creates a new Binance CEX provider.
func NewProvider(cfg ProviderConfig, log logger.LoggerInterface) (*Provider, error) {
	// Use custom URL if provided, otherwise default
	wsURL := cfg.WebSocketURL
	if wsURL == "" {
		wsURL = BaseWSURL
	}

	clientCfg := ClientConfig{
		BaseURL:      wsURL,
		Symbols:      cfg.Symbols,
		DepthSpeedMs: cfg.DepthSpeedMs,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	client, err := NewClient(clientCfg, log)
	if err != nil {
		return nil, err
	}

	// Create HTTP client for fallback (optional)
	var httpClient *HTTPClient
	if cfg.EnableFallback {
		httpCfg := HTTPClientConfig{
			BaseURL: cfg.HTTPURL, // Empty = default
		}
		httpClient, err = NewHTTPClient(httpCfg, log)
		if err != nil {
			log.Warn(context.Background(), "failed to create HTTP fallback client", "error", err)
			// Continue without HTTP fallback
		}
	}

	p := &Provider{
		config:     cfg,
		logger:     log,
		client:     client,
		httpClient: httpClient,
		orderbooks: make(map[string]*orderbookState),
		registry:   asset.DefaultRegistry(),
		tracer:     otel.Tracer(tracerName),
	}

	// Initialize orderbook state for each symbol
	for _, sym := range cfg.Symbols {
		p.orderbooks[sym] = &orderbookState{
			bids: make([]domain.OrderbookLevel, 0, cfg.SnapshotDepth),
			asks: make([]domain.OrderbookLevel, 0, cfg.SnapshotDepth),
		}
	}

	// Register handlers
	client.OnBookTicker(p.handleBookTicker)
	client.OnDepthUpdate(p.handleDepthUpdate)

	return p, nil
}

// Connect establishes connection to Binance.
func (p *Provider) Connect(ctx context.Context) error {
	return p.client.Connect(ctx)
}

// Close closes the provider.
func (p *Provider) Close() error {
	return p.client.Close()
}

// GetOrderbook retrieves the current orderbook for a trading pair.
func (p *Provider) GetOrderbook(ctx context.Context, pair domain.Pair) (*domain.Orderbook, error) {
	ctx, span := p.tracer.Start(ctx, "binance.get_orderbook",
		trace.WithAttributes(attribute.String("pair", pair.String())),
	)
	defer span.End()

	symbol := pairToSymbol(pair)

	p.booksMu.RLock()
	state, ok := p.orderbooks[symbol]
	p.booksMu.RUnlock()

	if !ok {
		return nil, apperror.New(apperror.CodeNotFound,
			apperror.WithContext(fmt.Sprintf("symbol %s not subscribed", symbol)))
	}

	state.mu.RLock()
	isStale := time.Since(state.lastUpdate) > p.config.StaleTimeout
	bidsLen := len(state.bids)
	asksLen := len(state.asks)
	state.mu.RUnlock()

	// Check staleness - try HTTP fallback if available
	if isStale {
		span.SetAttributes(attribute.Bool("stale", true))

		// Try HTTP fallback
		if p.httpClient != nil {
			p.logger.Debug(ctx, "orderbook stale, using HTTP fallback", "symbol", symbol)
			return p.getOrderbookViaHTTP(ctx, pair, symbol, span)
		}

		return nil, apperror.New(apperror.CodeCacheExpired,
			apperror.WithContext(fmt.Sprintf("orderbook stale for %s", symbol)))
	}

	// Check if we have any data
	if bidsLen == 0 || asksLen == 0 {
		// Try HTTP fallback if no WebSocket data yet
		if p.httpClient != nil {
			p.logger.Debug(ctx, "no WS data yet, using HTTP fallback", "symbol", symbol)
			return p.getOrderbookViaHTTP(ctx, pair, symbol, span)
		}
		return nil, apperror.New(apperror.CodeInvalidOrderbook,
			apperror.WithContext(fmt.Sprintf("no orderbook data for %s", symbol)))
	}

	state.mu.RLock()
	defer state.mu.RUnlock()

	// Copy the orderbook
	ob := &domain.Orderbook{
		Pair:      pair,
		Bids:      make([]domain.OrderbookLevel, len(state.bids)),
		Asks:      make([]domain.OrderbookLevel, len(state.asks)),
		Timestamp: state.lastUpdate,
	}
	copy(ob.Bids, state.bids)
	copy(ob.Asks, state.asks)

	span.SetAttributes(
		attribute.Int("bids", len(ob.Bids)),
		attribute.Int("asks", len(ob.Asks)),
		attribute.String("source", "websocket"),
	)

	p.logger.Debug(ctx, "orderbook retrieved", "symbol", symbol, "bids", len(ob.Bids), "asks", len(ob.Asks))

	return ob, nil
}

// getOrderbookViaHTTP fetches the orderbook via REST API fallback.
func (p *Provider) getOrderbookViaHTTP(ctx context.Context, pair domain.Pair, symbol string, span trace.Span) (*domain.Orderbook, error) {
	depth, err := p.httpClient.GetDepth(ctx, symbol, p.config.SnapshotDepth)
	if err != nil {
		return nil, err
	}

	baseAsset := p.guessBaseAsset(symbol)

	// Parse levels
	bidLevels, err := ParseOrderbookLevels(depth.Bids)
	if err != nil {
		return nil, apperror.New(apperror.CodeInvalidOrderbook,
			apperror.WithCause(err),
			apperror.WithContext("failed to parse bid levels"))
	}
	askLevels, err := ParseOrderbookLevels(depth.Asks)
	if err != nil {
		return nil, apperror.New(apperror.CodeInvalidOrderbook,
			apperror.WithCause(err),
			apperror.WithContext("failed to parse ask levels"))
	}

	// Convert to domain levels
	bids := make([]domain.OrderbookLevel, 0, len(bidLevels))
	for _, level := range bidLevels {
		amt, _ := asset.ParseDecimal(baseAsset, level.Quantity)
		bids = append(bids, domain.OrderbookLevel{Price: level.Price, Amount: amt})
	}

	asks := make([]domain.OrderbookLevel, 0, len(askLevels))
	for _, level := range askLevels {
		amt, _ := asset.ParseDecimal(baseAsset, level.Quantity)
		asks = append(asks, domain.OrderbookLevel{Price: level.Price, Amount: amt})
	}

	// Update the cached state with HTTP data
	p.booksMu.RLock()
	state, ok := p.orderbooks[symbol]
	p.booksMu.RUnlock()
	if ok {
		state.mu.Lock()
		state.bids = bids
		state.asks = asks
		state.lastUpdate = time.Now()
		state.mu.Unlock()
	}

	ob := &domain.Orderbook{
		Pair:      pair,
		Bids:      bids,
		Asks:      asks,
		Timestamp: time.Now(),
	}

	span.SetAttributes(
		attribute.Int("bids", len(ob.Bids)),
		attribute.Int("asks", len(ob.Asks)),
		attribute.String("source", "http_fallback"),
	)

	p.logger.Info(ctx, "orderbook retrieved via HTTP fallback", "symbol", symbol, "bids", len(ob.Bids), "asks", len(ob.Asks))

	return ob, nil
}

// GetEffectivePrice calculates the effective price for a given trade size.
func (p *Provider) GetEffectivePrice(ctx context.Context, pair domain.Pair, size decimal.Decimal, side domain.Side) (*domain.Price, error) {
	ctx, span := p.tracer.Start(ctx, "binance.get_effective_price",
		trace.WithAttributes(
			attribute.String("pair", pair.String()),
			attribute.String("size", size.String()),
			attribute.String("side", string(side)),
		),
	)
	defer span.End()

	ob, err := p.GetOrderbook(ctx, pair)
	if err != nil {
		return nil, err
	}

	// Calculate volume-weighted average price
	var levels []domain.OrderbookLevel
	if side == domain.SideBuy {
		levels = ob.Asks // Buy from asks
	} else {
		levels = ob.Bids // Sell into bids
	}

	if len(levels) == 0 {
		return nil, apperror.New(apperror.CodeInvalidOrderbook,
			apperror.WithContext("no liquidity"))
	}

	// VWAP calculation
	remaining := size
	totalCost := decimal.Zero
	totalFilled := decimal.Zero

	for _, level := range levels {
		if remaining.IsZero() {
			break
		}

		fillQty := decimal.Min(remaining, level.Amount.ToDecimal())
		fillCost := fillQty.Mul(level.Price)

		totalCost = totalCost.Add(fillCost)
		totalFilled = totalFilled.Add(fillQty)
		remaining = remaining.Sub(fillQty)
	}

	if totalFilled.IsZero() {
		return nil, apperror.New(apperror.CodeInvalidOrderbook,
			apperror.WithContext("could not fill any quantity"))
	}

	// Average price = total cost / total filled
	avgPrice := totalCost.Div(totalFilled)

	// Warn if not fully filled
	if remaining.IsPositive() {
		p.logger.Warn(ctx, "partial fill in effective price calculation",
			"requested", size.String(),
			"filled", totalFilled.String(),
			"remaining", remaining.String())
	}

	// Build Price with asset types
	baseAsset, quoteAsset := pairToAssets(pair, p.registry)
	sizeAmount, _ := asset.ParseDecimal(baseAsset, totalFilled)
	rate := asset.NewPriceNow(baseAsset, quoteAsset, avgPrice)

	price := domain.NewPrice(rate, sizeAmount, side, "binance")

	span.SetAttributes(
		attribute.String("effective_price", avgPrice.String()),
		attribute.String("filled", totalFilled.String()),
	)

	return &price, nil
}

// handleBookTicker processes book ticker updates (best bid/ask).
func (p *Provider) handleBookTicker(event *BookTickerEvent) {
	ctx := context.Background()

	p.logger.Debug(ctx, "received book ticker", "symbol", event.Symbol, "bid", event.BidPrice, "ask", event.AskPrice)

	p.booksMu.RLock()
	state, ok := p.orderbooks[event.Symbol]
	p.booksMu.RUnlock()

	if !ok {
		p.logger.Debug(ctx, "symbol not in orderbooks", "symbol", event.Symbol, "available", p.getOrderbookKeys())
		return
	}

	bidPrice, err := event.ParseBidPrice()
	if err != nil {
		p.logger.Debug(ctx, "failed to parse bid price", "error", err)
		return
	}
	bidQty, _ := event.ParseBidQty()
	askPrice, _ := event.ParseAskPrice()
	askQty, _ := event.ParseAskQty()

	// Get assets for amounts
	baseAsset := p.guessBaseAsset(event.Symbol)

	state.mu.Lock()
	// Update top of book
	if len(state.bids) > 0 {
		state.bids[0].Price = bidPrice
		state.bids[0].Amount, _ = asset.ParseDecimal(baseAsset, bidQty)
	} else {
		amt, _ := asset.ParseDecimal(baseAsset, bidQty)
		state.bids = []domain.OrderbookLevel{{Price: bidPrice, Amount: amt}}
	}
	if len(state.asks) > 0 {
		state.asks[0].Price = askPrice
		state.asks[0].Amount, _ = asset.ParseDecimal(baseAsset, askQty)
	} else {
		amt, _ := asset.ParseDecimal(baseAsset, askQty)
		state.asks = []domain.OrderbookLevel{{Price: askPrice, Amount: amt}}
	}
	state.lastUpdate = time.Now()
	state.mu.Unlock()
}

// handleDepthUpdate processes partial book depth updates from @depth20 streams.
// This replaces the entire orderbook with the snapshot received.
func (p *Provider) handleDepthUpdate(event *PartialDepthEvent) {
	ctx := context.Background()

	p.booksMu.RLock()
	state, ok := p.orderbooks[event.Symbol]
	p.booksMu.RUnlock()

	if !ok {
		p.logger.Debug(ctx, "depth update for unknown symbol", "symbol", event.Symbol)
		return
	}

	baseAsset := p.guessBaseAsset(event.Symbol)

	// Parse levels from partial book snapshot
	bidLevels, err := ParseOrderbookLevels(event.Bids)
	if err != nil {
		p.logger.Debug(ctx, "failed to parse bid levels", "error", err)
	}
	askLevels, err := ParseOrderbookLevels(event.Asks)
	if err != nil {
		p.logger.Debug(ctx, "failed to parse ask levels", "error", err)
	}

	p.logger.Debug(ctx, "received partial depth",
		"symbol", event.Symbol,
		"bids", len(bidLevels),
		"asks", len(askLevels))

	// Convert to domain levels
	bids := make([]domain.OrderbookLevel, 0, len(bidLevels))
	for _, level := range bidLevels {
		amt, _ := asset.ParseDecimal(baseAsset, level.Quantity)
		bids = append(bids, domain.OrderbookLevel{Price: level.Price, Amount: amt})
	}

	asks := make([]domain.OrderbookLevel, 0, len(askLevels))
	for _, level := range askLevels {
		amt, _ := asset.ParseDecimal(baseAsset, level.Quantity)
		asks = append(asks, domain.OrderbookLevel{Price: level.Price, Amount: amt})
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	// Replace entire orderbook (partial book sends complete snapshot)
	state.bids = bids
	state.asks = asks
	state.lastUpdate = time.Now()
}

// applyOrderbookUpdates merges updates into the current orderbook.
func applyOrderbookUpdates(current []domain.OrderbookLevel, updates []OrderbookLevel, baseAsset *asset.Asset, isBid bool, maxDepth int) []domain.OrderbookLevel {
	// Build map for efficient updates
	priceMap := make(map[string]domain.OrderbookLevel)
	for _, level := range current {
		priceMap[level.Price.String()] = level
	}

	// Apply updates
	for _, upd := range updates {
		key := upd.Price.String()
		if upd.Quantity.IsZero() {
			delete(priceMap, key) // Remove level
		} else {
			amt, _ := asset.ParseDecimal(baseAsset, upd.Quantity)
			priceMap[key] = domain.OrderbookLevel{Price: upd.Price, Amount: amt}
		}
	}

	// Convert back to slice
	result := make([]domain.OrderbookLevel, 0, len(priceMap))
	for _, level := range priceMap {
		result = append(result, level)
	}

	// Sort: bids descending, asks ascending
	if isBid {
		sort.Slice(result, func(i, j int) bool {
			return result[i].Price.GreaterThan(result[j].Price)
		})
	} else {
		sort.Slice(result, func(i, j int) bool {
			return result[i].Price.LessThan(result[j].Price)
		})
	}

	// Truncate to max depth
	if len(result) > maxDepth {
		result = result[:maxDepth]
	}

	return result
}

// guessBaseAsset attempts to determine the base asset from symbol.
func (p *Provider) guessBaseAsset(symbol string) *asset.Asset {
	// Common quote assets
	quotes := []string{"USDC", "USDT", "BUSD", "USD"}
	for _, q := range quotes {
		if len(symbol) > len(q) && symbol[len(symbol)-len(q):] == q {
			baseSymbol := symbol[:len(symbol)-len(q)]
			if a, ok := p.registry.GetBySymbolAndChain(baseSymbol, asset.ChainIDEthereum); ok {
				return a
			}
		}
	}
	// Default to ETH if unknown
	return asset.ETH
}

// pairToSymbol converts a domain.Pair to Binance symbol format.
func pairToSymbol(pair domain.Pair) string {
	return pair.Base.Symbol() + pair.Quote.Symbol()
}

// pairToAssets extracts assets from a pair.
func pairToAssets(pair domain.Pair, registry *asset.Registry) (*asset.Asset, *asset.Asset) {
	return pair.Base, pair.Quote
}

// getOrderbookKeys returns the keys of the orderbooks map for debugging.
func (p *Provider) getOrderbookKeys() []string {
	p.booksMu.RLock()
	defer p.booksMu.RUnlock()
	keys := make([]string, 0, len(p.orderbooks))
	for k := range p.orderbooks {
		keys = append(keys, k)
	}
	return keys
}
