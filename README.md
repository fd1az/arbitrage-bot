# CEX-DEX Arbitrage Bot

A real-time arbitrage opportunity detector that monitors price spreads between centralized exchanges (Binance) and decentralized exchanges (Uniswap V3) on Ethereum.

![Bot Running](images/BotRunning2.png)

## Features

- **Real-time price monitoring**: WebSocket connections to Binance and Ethereum nodes
- **HTTP fallback**: Automatic REST API fallback when WebSocket data is stale or unavailable
- **VWAP calculation**: Volume-weighted average price for accurate large trade pricing
- **Multi-size analysis**: Analyzes opportunities for different trade sizes (1, 10, 100 ETH)
- **Cost-aware**: Includes gas costs and exchange fees in profit calculations
- **Execution planning**: Generates step-by-step execution plans for each opportunity
- **Risk assessment**: Identifies risk factors (slippage, MEV, timing) with severity levels
- **Pool fee detection**: Automatically selects best Uniswap V3 fee tier (0.01%, 0.05%, 0.30%, 1%)
- **Production-ready**: OpenTelemetry tracing, Prometheus metrics, structured logging
- **TUI interface**: Beautiful terminal UI with Bubble Tea showing prices, costs, and opportunities

## Architecture

The bot follows a hexagonal (ports & adapters) architecture with domain-driven design principles.

```
┌─────────────────────────────────────────────────────────────────┐
│                     CEX-DEX ARBITRAGE BOT                       │
├─────────────────────────────────────────────────────────────────┤
│  EXTERNAL SOURCES                                               │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                       │
│  │ Binance  │  │ Ethereum │  │ Uniswap  │                       │
│  │ WS + HTTP│  │ WebSocket│  │ Quoter   │                       │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘                       │
│       │             │             │                             │
│       ▼             ▼             ▼                             │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │              INFRASTRUCTURE LAYER                       │    │
│  │  wsconn │ httpclient │ Binance │ Uniswap │ Ethereum     │    │
│  └─────────────────────────────────────────────────────────┘    │
│                          │                                      │
│                          ▼                                      │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │              APPLICATION LAYER                           │   │
│  │  Pricing Service │ Arbitrage Detector │ Profit Calculator│   │
│  └─────────────────────────────────────────────────────────┘    │
│                          │                                      │
│                          ▼                                      │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │              PRESENTATION LAYER                         │    │
│  │         TUI (Bubble Tea)  │  CLI (debug logs)           │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

For detailed documentation see:
- [Architecture](docs/architecture.md) - System design and module structure
- [Data Flow](docs/dataflow.md) - How data flows through the system
- [Profiling](docs/profiling.md) - Memory profiling with pprof
- [MEV Risks](docs/mev-risks.md) - MEV attack vectors and mitigations

## Quick Start

### Prerequisites

- Go 1.21+
- Ethereum RPC endpoint (Infura, Alchemy, etc.)

### Environment Variables

Create a `.env` file:

```bash
# Required - Ethereum RPC URLs (Infura, Alchemy, etc.)
ETH_WS_URL=wss://mainnet.infura.io/ws/v3/your_api_key_here
ETH_HTTP_URL=https://mainnet.infura.io/v3/your_api_key_here

# Optional (for telemetry)
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
OTEL_SERVICE_NAME=arbitrage-bot
```

### Running

```bash
# Build and run (TUI mode by default)
make run

# Run in TUI mode explicitly
make run-tui

# Run with CLI mode (debug logs)
./bin/arbitrage-bot --cli

# Run with custom config
./bin/arbitrage-bot --config /path/to/config.yaml

# Development mode with hot reload
make dev
```

### Sample Output (CLI Mode)

When an opportunity is detected, the bot outputs detailed analysis:

```
================================================================================
ARBITRAGE OPPORTUNITY DETECTED
================================================================================
Block:          #21234567
Timestamp:      2026-01-17T10:30:45Z
Pair:           ETH-USDC
Direction:      CEX → DEX (Buy on Binance, Sell on Uniswap)
--------------------------------------------------------------------------------
PRICES
  CEX (Binance):  $3,245.30
  DEX (Uniswap):  $3,268.75
  Spread:         72.15 bps
  Pool Fee Tier:  0.30%
--------------------------------------------------------------------------------
TRADE DETAILS
  Size:           10.0000 ETH
  Gas Cost:       0.005432 ETH ($17.64)
  Required Capital: $32,453.00
--------------------------------------------------------------------------------
PROFIT
  Gross:          $234.50
  Net:            $112.15 (47.81%)
--------------------------------------------------------------------------------
EXECUTION STEPS
  1. Buy 10.0000 ETH on Binance at $3,245.30
  2. Transfer ETH to trading wallet
  3. Execute Uniswap V3 swap: ETH → USDC via 0.30% pool
  4. Receive ~32,687.50 USDC from swap
  5. Transfer USDC back to Binance for next cycle
--------------------------------------------------------------------------------
RISK FACTORS
  - Slippage Risk (low): Price movement during execution
  - MEV Risk (medium): Potential sandwich attacks from MEV bots
  - Timing Risk (low): Block confirmation delays
================================================================================
```

### Configuration

See `config.yaml.example` for all options:

```yaml
arbitrage:
  pairs:
    - base: "ETH"
      quote: "USDC"
  trade_sizes: [1, 10, 100]  # ETH amounts to analyze
  min_profit_bps: 10         # Minimum spread in basis points
  min_profit_usd: 50         # Minimum profit in USD

binance:
  depth_speed_ms: 100        # Orderbook update speed (100 or 1000)
  enable_fallback: true      # Enable HTTP fallback when WS stale (default: true)
  stale_timeout: 5s          # Time before data is considered stale
```

## Make Commands

```bash
make help              # Show all available commands

# Build & Run
make build             # Build the binary
make run               # Build and run
make run-tui           # Run in TUI mode
make dev               # Hot reload development mode

# Testing
make test              # Run all tests with race detector
make test-coverage     # Generate coverage report
make test-short        # Run short tests only
make bench             # Run benchmarks

# Code Quality
make fmt               # Format code
make vet               # Run go vet
make lint              # Run golangci-lint
make check             # Run all quality checks (fmt, vet, lint)

# Setup & Dependencies
make setup             # Initial project setup (tools + deps)
make deps              # Download dependencies
make tidy              # Tidy Go modules
make install-tools     # Install dev tools (air, golangci-lint, mockery)

# Other
make clean             # Remove build artifacts
make mocks             # Generate test mocks
make docker-build      # Build Docker image
make docker-run        # Run in Docker
```

```bash
# Health check (when running)
curl http://localhost:8081/health

# Prometheus metrics
curl http://localhost:9090/metrics
```

## Observability

### Metrics (Prometheus)

The bot exposes metrics on `:9090/metrics`.

**Arbitrage Detection:**

| Metric | Type | Description |
|--------|------|-------------|
| `arbitrage_opportunities_analyzed_total` | Counter | Total opportunities analyzed |
| `arbitrage_opportunities_profitable_total` | Counter | Profitable opportunities detected |
| `arbitrage_spread_bps` | Histogram | Spread distribution in basis points |
| `arbitrage_net_profit_usd` | Histogram | Net profit distribution in USD |
| `arbitrage_analysis_latency_ms` | Histogram | Time to analyze each opportunity |

**Binance (CEX):**

| Metric | Type | Description |
|--------|------|-------------|
| `binance_messages_total` | Counter | WebSocket messages received |
| `binance_depth_updates_total` | Counter | Orderbook depth updates |
| `binance_trades_total` | Counter | Trade messages received |
| `binance_parse_errors_total` | Counter | JSON parse errors |

**Uniswap (DEX):**

| Metric | Type | Description |
|--------|------|-------------|
| `uniswap_quotes_total` | Counter | Quote requests made |
| `uniswap_quote_latency_ms` | Histogram | Quote response time |
| `uniswap_quote_errors_total` | Counter | Failed quotes |

**Blockchain:**

| Metric | Type | Description |
|--------|------|-------------|
| `blocks_received_total` | Counter | Ethereum blocks processed |
| `gas_price_gwei` | Gauge | Current gas price |
| `block_latency_ms` | Histogram | Block processing latency |
| `http_fallback_used_total` | Counter | HTTP fallback activations |

**WebSocket:**

| Metric | Type | Description |
|--------|------|-------------|
| `ws_connection_state` | Gauge | Connection state (0=disconnected, 2=connected) |
| `ws_messages_received_total` | Counter | Messages received |
| `ws_messages_dropped_total` | Counter | Messages dropped (buffer full) |
| `ws_reconnects_total` | Counter | Reconnection attempts |
| `ws_message_latency_ms` | Histogram | Message processing latency |
| `ws_pings_total` | Counter | Successful ping/pong heartbeats |

**Useful PromQL queries:**

```promql
# Message rate per second
rate(binance_messages_total[1m])

# Dropped messages (should be 0)
increase(ws_messages_dropped_total[5m])

# Connection uptime
ws_connection_state == 2

# HTTP fallback usage (should be low)
rate(http_client_requests_total{provider="binance"}[5m])

# P95 message latency
histogram_quantile(0.95, rate(ws_message_latency_ms_milliseconds_bucket[5m]))
```

### Tracing (OpenTelemetry)

Enable OTLP exporter in config:

```yaml
telemetry:
  enabled: true
  otlp_endpoint: "http://localhost:4317"
```

**Useful trace queries (Jaeger/Tempo):**

```
# All arbitrage detection spans
service.name="arbitrage-bot" AND operation="arbitrage.detect"

# Slow Uniswap quotes (>200ms)
service.name="arbitrage-bot" AND operation="uniswap.quote" AND duration>200ms

# WebSocket reconnection events
service.name="arbitrage-bot" AND operation="ws.reconnect"
```

### Grafana Dashboard

Import the dashboard from `observability/grafana/arbitrage-bot.json` or create panels for:

1. **Connection Status**: `ws_connection_state`
2. **Message Throughput**: `rate(binance_messages_total[1m])`
3. **Dropped Messages**: `ws_messages_dropped_total`
4. **Gas Price**: `gas_price_gwei`
5. **Latency Histogram**: `ws_message_latency_ms_milliseconds`

## Domain Decisions

### Why Block-Triggered Analysis?

Instead of continuously polling prices, we trigger analysis on new Ethereum blocks (~12s). This ensures:
- DEX prices are fresh (Uniswap state changes per block)
- Reduced RPC calls
- Natural rate limiting

### Why VWAP for CEX Prices?

Best bid/ask is insufficient for large trades. We maintain top 20 orderbook levels and calculate Volume-Weighted Average Price (VWAP) for accurate execution price estimates.

### Fee Assumptions

| Exchange | Fee |
|----------|-----|
| Uniswap V3 | 0.01% - 1% (auto-detected best pool) |
| Binance | 0.1% (taker) |

The bot automatically queries all Uniswap V3 fee tiers (0.01%, 0.05%, 0.30%, 1%) and selects the pool with best execution price. The selected pool fee tier is shown in opportunity reports.

Opportunities typically need >40-60 bps spread to overcome fees + gas, depending on pool fee tier.

### Why No Execution?

This bot is **detection-only**. Execution requires:
- Private key management
- MEV protection (Flashbots)
- Atomic execution (flash loans)
- Slippage protection

These are out of scope for this monitoring tool.

## Deployment

### Current: Single Binary

```bash
./arbitrage  # Runs detection + TUI in one process
```

### Future: Client-Server Architecture

The codebase is designed to eventually support a separated architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                    DETECTION SERVER                         |
│  - Subscribes to blocks, fetches prices                     │
│  - Calculates opportunities                                 |
│  - Exposes API (HTTP/WebSocket/gRPC)                        │
│  - Prometheus metrics                                       |
└─────────────────────────────────────────────────────────────┘
                              │
                              │ API
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                       CLIENTS                               │
│  - TUI (current)                                            │
│  - Web dashboard                                            │
│  - Mobile app                                               │
│  - Alerts/notifications                                     │
└─────────────────────────────────────────────────────────────┘
```

This separation is not yet implemented but the domain layer is decoupled to support it.

### Observability Stack (Docker Compose)

```bash
# Start observability stack
docker-compose up -d

# Services:
# - Zipkin:     http://localhost:9411  (tracing UI)
# - Prometheus: http://localhost:9091  (metrics)
# - Grafana:    http://localhost:3000  (dashboards, admin/admin)

# Run the bot
./arbitrage  # Exports metrics to :9090, traces to Zipkin
```

## Future Extensions

The following features are documented for future implementation when trade execution is added.

### Confidence Scoring

A weighted scoring system to evaluate opportunity quality before execution:

```go
type ConfidenceScore struct {
    Score   float64            // 0.0 - 1.0
    Factors map[string]float64 // Breakdown by factor
}

// Weights for confidence calculation
weights := map[string]float64{
    "spread_magnitude":   0.25, // Higher spread = more confidence
    "liquidity_depth":    0.20, // More liquidity = more confidence
    "data_freshness":     0.20, // Fresher data = more confidence
    "historical_success": 0.15, // Past success rate
    "volatility":         0.10, // Lower volatility = more confidence
    "gas_stability":      0.10, // Stable gas = more confidence
}
```

**Implementation path:**
1. Create `business/arbitrage/domain/confidence.go`
2. Add `ConfidenceCalculator` with configurable weights
3. Integrate with `Detector` behind a feature flag
4. Add confidence score to opportunity output

### MEV Risk Model

Quantify MEV exposure for each opportunity:

```go
type MEVRisk struct {
    Level       string  // "low", "medium", "high", "critical"
    Score       float64 // 0-1
    Explanation string
}

// Risk increases with:
// - Smaller spreads (more competition)
// - Larger trade sizes (more profitable to sandwich)
// - Higher gas prices (more MEV activity)
```

**Risk matrix:**

| Spread | Size | Risk Level |
|--------|------|------------|
| < 20 bps | Any | Low (not worth MEV bot's gas) |
| 20-50 bps | > 10 ETH | High |
| 50-100 bps | < 10 ETH | Medium |
| > 100 bps | Any | Variable (execute fast) |

**Implementation path:**
1. Create `business/arbitrage/domain/mev.go`
2. Add `CalculateMEVRisk(spread, size, gasPrice)` function
3. Include MEV risk in opportunity reports
4. See [MEV Risks](docs/mev-risks.md) for detailed analysis

### Feature Flags

Simple feature flag system for gradual rollout:

```bash
# Enable via environment variables
FF_CONFIDENCE_SCORING=true
FF_MEV_RISK_DISPLAY=true
```

```go
// internal/features/flags.go
func Enabled(name string) bool {
    return os.Getenv("FF_" + strings.ToUpper(name)) == "true"
}

// Usage
if features.Enabled("confidence_scoring") {
    opp.Confidence = calculator.Calculate(opp)
}
```

## License

MIT
