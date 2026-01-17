// Package uniswap implements the DEXProvider interface for Uniswap V3.
package uniswap

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/fd1az/arbitrage-bot/business/pricing/app"
	"github.com/fd1az/arbitrage-bot/business/pricing/domain"
	"github.com/fd1az/arbitrage-bot/internal/apperror"
	"github.com/fd1az/arbitrage-bot/internal/asset"
	"github.com/fd1az/arbitrage-bot/internal/circuitbreaker"
	"github.com/fd1az/arbitrage-bot/internal/config"
	"github.com/fd1az/arbitrage-bot/internal/logger"
)

const (
	tracerName = "uniswap"
	meterName  = "uniswap"
)

// Ensure Provider implements DEXProvider.
var _ app.DEXProvider = (*Provider)(nil)

// providerMetrics holds OTEL metric instruments.
type providerMetrics struct {
	quotesTotal   metric.Int64Counter
	quoteLatency  metric.Float64Histogram
	quoteErrors   metric.Int64Counter
}

// Provider implements DEXProvider for Uniswap V3.
type Provider struct {
	client   *ethclient.Client
	quoter   common.Address
	quoterABI abi.ABI
	feeTiers []int

	registry *asset.Registry
	logger   logger.LoggerInterface
	cb       *circuitbreaker.CircuitBreaker[[]byte]

	tracer  trace.Tracer
	metrics *providerMetrics
}

// NewProvider creates a new Uniswap V3 provider.
func NewProvider(client *ethclient.Client, cfg config.UniswapConfig, log logger.LoggerInterface) (*Provider, error) {
	// Parse QuoterV2 ABI
	parsedABI, err := abi.JSON(strings.NewReader(QuoterV2ABI))
	if err != nil {
		return nil, fmt.Errorf("failed to parse quoter ABI: %w", err)
	}

	p := &Provider{
		client:    client,
		quoter:    cfg.QuoterAddressHex(),
		quoterABI: parsedABI,
		feeTiers:  []int{cfg.DefaultFeeTier, FeeTier005, FeeTier030, FeeTier100},
		registry:  asset.DefaultRegistry(),
		logger:    log,
		tracer:    otel.Tracer(tracerName),
	}

	// Initialize circuit breaker
	cbCfg := circuitbreaker.DefaultConfig("uniswap-quoter")
	p.cb = circuitbreaker.New[[]byte](cbCfg)

	if err := p.initMetrics(); err != nil {
		return nil, fmt.Errorf("failed to init metrics: %w", err)
	}

	return p, nil
}

func (p *Provider) initMetrics() error {
	meter := otel.Meter(meterName)
	var err error

	p.metrics = &providerMetrics{}

	p.metrics.quotesTotal, err = meter.Int64Counter(
		"uniswap_quotes_total",
		metric.WithDescription("Total quote requests"),
	)
	if err != nil {
		return err
	}

	p.metrics.quoteLatency, err = meter.Float64Histogram(
		"uniswap_quote_latency_ms",
		metric.WithDescription("Quote request latency in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return err
	}

	p.metrics.quoteErrors, err = meter.Int64Counter(
		"uniswap_quote_errors_total",
		metric.WithDescription("Total quote errors"),
	)
	if err != nil {
		return err
	}

	return nil
}

// GetQuote retrieves a price quote for swapping tokens on Uniswap V3.
func (p *Provider) GetQuote(ctx context.Context, tokenIn, tokenOut common.Address, amountIn *big.Int) (*domain.Quote, error) {
	ctx, span := p.tracer.Start(ctx, "uniswap.get_quote",
		trace.WithAttributes(
			attribute.String("token_in", tokenIn.Hex()),
			attribute.String("token_out", tokenOut.Hex()),
			attribute.String("amount_in", amountIn.String()),
		),
	)
	defer span.End()

	start := time.Now()
	p.metrics.quotesTotal.Add(ctx, 1)

	// Try each fee tier to find the best quote
	var bestQuote *QuoteResult
	var bestFeeTier int

	for _, feeTier := range p.feeTiers {
		quote, err := p.getQuoteForFeeTier(ctx, tokenIn, tokenOut, amountIn, feeTier)
		if err != nil {
			span.AddEvent("fee_tier_failed",
				trace.WithAttributes(
					attribute.Int("fee_tier", feeTier),
					attribute.String("error", err.Error()),
				),
			)
			continue
		}

		// Keep the best (highest output) quote
		if bestQuote == nil || quote.AmountOut.Cmp(bestQuote.AmountOut) > 0 {
			bestQuote = quote
			bestFeeTier = feeTier
		}
	}

	latency := float64(time.Since(start).Milliseconds())
	p.metrics.quoteLatency.Record(ctx, latency)

	if bestQuote == nil {
		p.metrics.quoteErrors.Add(ctx, 1)
		span.SetStatus(codes.Error, "no valid quote")
		return nil, apperror.New(apperror.CodeUniswapQuoteFailed,
			apperror.WithContext("no pool found for token pair"))
	}

	// Build domain.Quote
	assetIn := p.resolveAsset(tokenIn)
	assetOut := p.resolveAsset(tokenOut)

	amtIn := asset.NewAmount(assetIn, amountIn)
	amtOut := asset.NewAmount(assetOut, bestQuote.AmountOut)

	result := domain.NewQuote(assetIn, assetOut, amtIn, amtOut, bestQuote.GasEstimate.Uint64(), bestFeeTier)

	span.SetAttributes(
		attribute.String("amount_out", bestQuote.AmountOut.String()),
		attribute.Int("fee_tier", bestFeeTier),
		attribute.Int64("gas_estimate", bestQuote.GasEstimate.Int64()),
	)
	span.SetStatus(codes.Ok, "quote received")

	p.logger.Debug(ctx, "uniswap quote",
		"token_in", tokenIn.Hex(),
		"token_out", tokenOut.Hex(),
		"amount_in", amountIn.String(),
		"amount_out", bestQuote.AmountOut.String(),
		"fee_tier", bestFeeTier,
	)

	return &result, nil
}

// getQuoteForFeeTier calls QuoterV2.quoteExactInputSingle for a specific fee tier.
func (p *Provider) getQuoteForFeeTier(ctx context.Context, tokenIn, tokenOut common.Address, amountIn *big.Int, feeTier int) (*QuoteResult, error) {
	// Encode call data for quoteExactInputSingle
	callData, err := p.quoterABI.Pack("quoteExactInputSingle", QuoteExactInputSingleParams{
		TokenIn:           tokenIn,
		TokenOut:          tokenOut,
		AmountIn:          amountIn,
		Fee:               big.NewInt(int64(feeTier)),
		SqrtPriceLimitX96: big.NewInt(0), // No price limit
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode call: %w", err)
	}

	// Execute call through circuit breaker
	result, err := p.cb.Execute(func() ([]byte, error) {
		return p.client.CallContract(ctx, ethereum.CallMsg{
			To:   &p.quoter,
			Data: callData,
		}, nil)
	})
	if err != nil {
		return nil, apperror.New(apperror.CodeContractCallFailed,
			apperror.WithCause(err),
			apperror.WithContext(fmt.Sprintf("quoter call failed for fee tier %d", feeTier)))
	}

	// Decode result
	outputs, err := p.quoterABI.Unpack("quoteExactInputSingle", result)
	if err != nil {
		return nil, fmt.Errorf("failed to decode result: %w", err)
	}

	if len(outputs) < 4 {
		return nil, fmt.Errorf("unexpected output length: %d", len(outputs))
	}

	return &QuoteResult{
		AmountOut:               outputs[0].(*big.Int),
		SqrtPriceX96After:       outputs[1].(*big.Int),
		InitializedTicksCrossed: outputs[2].(uint32),
		GasEstimate:             outputs[3].(*big.Int),
	}, nil
}

// resolveAsset attempts to find the asset in the registry.
func (p *Provider) resolveAsset(addr common.Address) *asset.Asset {
	if a, ok := p.registry.GetToken(asset.ChainIDEthereum, addr); ok {
		return a
	}
	// Return a generic ERC20 if not found
	return asset.NewAsset(
		asset.NewTokenAssetID(asset.ChainIDEthereum, addr),
		addr.Hex()[:8],
		18, // Assume 18 decimals
	)
}
