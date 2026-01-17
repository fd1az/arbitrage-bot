package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/fd1az/arbitrage-bot/business/blockchain/domain"
	"github.com/fd1az/arbitrage-bot/internal/apperror"
	"github.com/fd1az/arbitrage-bot/internal/cache"
	"github.com/fd1az/arbitrage-bot/internal/circuitbreaker"
	"github.com/fd1az/arbitrage-bot/internal/logger"
)

// GasOracleConfig holds configuration for the gas oracle.
type GasOracleConfig struct {
	RPCURL       string        // Ethereum RPC endpoint
	CacheTTL     time.Duration // How long to cache gas prices
	MaxGasPrice  *big.Int      // Maximum acceptable gas price (safety)
	DefaultGas   uint64        // Default gas limit for estimation
}

// DefaultGasOracleConfig returns sensible defaults.
func DefaultGasOracleConfig(rpcURL string) GasOracleConfig {
	maxGas := new(big.Int)
	maxGas.SetString("500000000000", 10) // 500 gwei max

	return GasOracleConfig{
		RPCURL:      rpcURL,
		CacheTTL:    12 * time.Second, // ~1 block
		MaxGasPrice: maxGas,
		DefaultGas:  200000,
	}
}

// gasOracleMetrics holds OTEL metric instruments.
type gasOracleMetrics struct {
	gasPriceFetches metric.Int64Counter
	gasPriceGwei    metric.Float64Gauge
	estimateGas     metric.Int64Counter
	cacheHits       metric.Int64Counter
	cacheMisses     metric.Int64Counter
}

// GasOracle implements the GasOracle interface using go-ethereum.
type GasOracle struct {
	config GasOracleConfig
	logger logger.LoggerInterface

	client   *ethclient.Client
	clientMu sync.RWMutex

	// Caching
	priceCache    *cache.Cache[string, *domain.GasPrice]
	priceCacheTTL time.Duration

	// Circuit breaker
	cb *circuitbreaker.CircuitBreaker[*big.Int]

	// Observability
	tracer  trace.Tracer
	metrics *gasOracleMetrics
}

// NewGasOracle creates a new gas oracle instance.
func NewGasOracle(cfg GasOracleConfig, log logger.LoggerInterface) (*GasOracle, error) {
	g := &GasOracle{
		config:        cfg,
		logger:        log,
		priceCache:    cache.New[string, *domain.GasPrice](5 * time.Minute),
		priceCacheTTL: cfg.CacheTTL,
		tracer:        otel.Tracer(tracerName),
	}

	if err := g.initMetrics(); err != nil {
		return nil, fmt.Errorf("init metrics: %w", err)
	}

	g.initCircuitBreaker()

	return g, nil
}

// initMetrics initializes OTEL metric instruments.
func (g *GasOracle) initMetrics() error {
	meter := otel.Meter(meterName)
	var err error

	g.metrics = &gasOracleMetrics{}

	g.metrics.gasPriceFetches, err = meter.Int64Counter(
		"gas_price_fetches_total",
		metric.WithDescription("Total gas price fetch attempts"),
		metric.WithUnit("{fetch}"),
	)
	if err != nil {
		return err
	}

	g.metrics.gasPriceGwei, err = meter.Float64Gauge(
		"gas_price_gwei",
		metric.WithDescription("Current gas price in gwei"),
		metric.WithUnit("gwei"),
	)
	if err != nil {
		return err
	}

	g.metrics.estimateGas, err = meter.Int64Counter(
		"gas_estimate_total",
		metric.WithDescription("Total gas estimation calls"),
		metric.WithUnit("{estimate}"),
	)
	if err != nil {
		return err
	}

	g.metrics.cacheHits, err = meter.Int64Counter(
		"gas_cache_hits_total",
		metric.WithDescription("Gas price cache hits"),
		metric.WithUnit("{hit}"),
	)
	if err != nil {
		return err
	}

	g.metrics.cacheMisses, err = meter.Int64Counter(
		"gas_cache_misses_total",
		metric.WithDescription("Gas price cache misses"),
		metric.WithUnit("{miss}"),
	)
	if err != nil {
		return err
	}

	return nil
}

// initCircuitBreaker initializes the circuit breaker.
func (g *GasOracle) initCircuitBreaker() {
	cfg := circuitbreaker.DefaultConfig("gas-oracle")
	g.cb = circuitbreaker.New[*big.Int](cfg)
}

// Connect establishes connection to the Ethereum node.
func (g *GasOracle) Connect(ctx context.Context) error {
	ctx, span := g.tracer.Start(ctx, "gas.connect",
		trace.WithAttributes(attribute.String("url", g.config.RPCURL)),
	)
	defer span.End()

	client, err := ethclient.DialContext(ctx, g.config.RPCURL)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "dial failed")
		return apperror.New(apperror.CodeEthereumConnectionFailed,
			apperror.WithCause(err),
			apperror.WithContext("failed to connect gas oracle"))
	}

	g.clientMu.Lock()
	g.client = client
	g.clientMu.Unlock()

	span.SetStatus(codes.Ok, "connected")
	g.logger.Info(ctx, "gas oracle connected", "url", g.config.RPCURL)

	return nil
}

// GetGasPrice retrieves the current gas price with caching.
func (g *GasOracle) GetGasPrice(ctx context.Context) (*domain.GasPrice, error) {
	ctx, span := g.tracer.Start(ctx, "gas.get_price")
	defer span.End()

	// Check cache first
	if price, found := g.priceCache.Get(ctx, "current"); found {
		g.metrics.cacheHits.Add(ctx, 1)
		span.AddEvent("cache_hit")
		return price, nil
	}

	g.metrics.cacheMisses.Add(ctx, 1)
	g.metrics.gasPriceFetches.Add(ctx, 1)

	g.clientMu.RLock()
	client := g.client
	g.clientMu.RUnlock()

	if client == nil {
		err := apperror.New(apperror.CodeEthereumConnectionFailed,
			apperror.WithContext("gas oracle not connected"))
		span.RecordError(err)
		return nil, err
	}

	// Fetch through circuit breaker
	wei, err := g.cb.Execute(func() (*big.Int, error) {
		return client.SuggestGasPrice(ctx)
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "fetch failed")
		return nil, apperror.New(apperror.CodeEthereumRPCError,
			apperror.WithCause(err),
			apperror.WithContext("failed to get gas price"))
	}

	// Safety check
	if g.config.MaxGasPrice != nil && wei.Cmp(g.config.MaxGasPrice) > 0 {
		span.AddEvent("gas_price_exceeded_max",
			trace.WithAttributes(attribute.String("wei", wei.String())))
		g.logger.Warn(ctx, "gas price exceeds max", "wei", wei.String())
		wei = g.config.MaxGasPrice
	}

	price := domain.NewGasPrice(wei)

	// Update cache
	g.priceCache.Set(ctx, "current", price, g.priceCacheTTL)

	// Record metric
	g.metrics.gasPriceGwei.Record(ctx, price.Gwei())

	span.SetAttributes(attribute.Float64("gwei", price.Gwei()))
	span.SetStatus(codes.Ok, "fetched")

	return price, nil
}

// GetGasTipCap retrieves the suggested gas tip cap (EIP-1559).
func (g *GasOracle) GetGasTipCap(ctx context.Context) (*big.Int, error) {
	ctx, span := g.tracer.Start(ctx, "gas.get_tip_cap")
	defer span.End()

	g.clientMu.RLock()
	client := g.client
	g.clientMu.RUnlock()

	if client == nil {
		err := apperror.New(apperror.CodeEthereumConnectionFailed,
			apperror.WithContext("gas oracle not connected"))
		span.RecordError(err)
		return nil, err
	}

	tipCap, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "fetch failed")
		return nil, apperror.New(apperror.CodeEthereumRPCError,
			apperror.WithCause(err),
			apperror.WithContext("failed to get gas tip cap"))
	}

	span.SetStatus(codes.Ok, "fetched")
	return tipCap, nil
}

// EstimateGas estimates the gas needed for a transaction.
func (g *GasOracle) EstimateGas(ctx context.Context, data []byte, to string) (uint64, error) {
	ctx, span := g.tracer.Start(ctx, "gas.estimate",
		trace.WithAttributes(
			attribute.String("to", to),
			attribute.Int("data_len", len(data)),
		),
	)
	defer span.End()

	g.metrics.estimateGas.Add(ctx, 1)

	g.clientMu.RLock()
	client := g.client
	g.clientMu.RUnlock()

	if client == nil {
		err := apperror.New(apperror.CodeEthereumConnectionFailed,
			apperror.WithContext("gas oracle not connected"))
		span.RecordError(err)
		return 0, err
	}

	toAddr := common.HexToAddress(to)
	msg := ethereum.CallMsg{
		To:   &toAddr,
		Data: data,
	}

	gas, err := client.EstimateGas(ctx, msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "estimate failed")
		return 0, apperror.New(apperror.CodeGasEstimationFailed,
			apperror.WithCause(err),
			apperror.WithContext(fmt.Sprintf("failed to estimate gas for %s", to)))
	}

	// Add safety margin (10%)
	gas = gas + (gas / 10)

	span.SetAttributes(attribute.Int64("gas", int64(gas)))
	span.SetStatus(codes.Ok, "estimated")

	return gas, nil
}

// GetGasEstimate returns a full gas estimate including price.
func (g *GasOracle) GetGasEstimate(ctx context.Context, data []byte, to string) (*domain.GasEstimate, error) {
	ctx, span := g.tracer.Start(ctx, "gas.full_estimate")
	defer span.End()

	gasPrice, err := g.GetGasPrice(ctx)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	gasLimit, err := g.EstimateGas(ctx, data, to)
	if err != nil {
		// Use default if estimation fails
		gasLimit = g.config.DefaultGas
		span.AddEvent("using_default_gas", trace.WithAttributes(
			attribute.Int64("default", int64(gasLimit))))
	}

	estimate := domain.NewGasEstimate(gasLimit, gasPrice)

	span.SetAttributes(
		attribute.Int64("gas_limit", int64(estimate.GasLimit)),
		attribute.Float64("total_gwei", estimate.TotalGwei()),
	)
	span.SetStatus(codes.Ok, "estimated")

	return estimate, nil
}

// Close closes the gas oracle.
func (g *GasOracle) Close() error {
	g.clientMu.Lock()
	defer g.clientMu.Unlock()

	if g.client != nil {
		g.client.Close()
		g.client = nil
	}

	g.priceCache.Close()

	return nil
}
