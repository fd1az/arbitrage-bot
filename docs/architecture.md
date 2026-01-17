# Architecture Overview

## System Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            CEX-DEX ARBITRAGE BOT                            │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  EXTERNAL DATA SOURCES                                                      │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐          │
│  │     Binance      │  │    Ethereum      │  │     Uniswap      │          │
│  │    WebSocket     │  │    WebSocket     │  │   Quoter V3      │          │
│  │   (CEX Prices)   │  │    (Blocks)      │  │   (DEX Quotes)   │          │
│  └────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘          │
│           │                     │                     │                     │
│           │ Book Ticker         │ New Block           │ Quote Request       │
│           │ Depth Updates       │ (trigger)           │ (on-demand)         │
│           ▼                     ▼                     ▼                     │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                        INFRASTRUCTURE LAYER                          │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌────────────┐  │   │
│  │  │  wsconn     │  │  Ethereum   │  │  Binance    │  │  Uniswap   │  │   │
│  │  │  (generic   │  │  Subscriber │  │  Provider   │  │  Provider  │  │   │
│  │  │  WS client) │  │             │  │  (orderbook)│  │  (quoter)  │  │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └────────────┘  │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│                                    ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                         APPLICATION LAYER                            │   │
│  │  ┌─────────────────────────────────────────────────────────────┐    │   │
│  │  │                    PRICING SERVICE                           │    │   │
│  │  │  - Aggregates CEX (Binance) and DEX (Uniswap) prices        │    │   │
│  │  │  - Constructs PriceSnapshot for comparison                   │    │   │
│  │  │  - Manages orderbook state (in-memory cache)                 │    │   │
│  │  └─────────────────────────────────────────────────────────────┘    │   │
│  │                                │                                     │   │
│  │                                ▼                                     │   │
│  │  ┌─────────────────────────────────────────────────────────────┐    │   │
│  │  │                   ARBITRAGE DETECTOR                         │    │   │
│  │  │  - Listens for new blocks (trigger)                         │    │   │
│  │  │  - Fetches price snapshots for configured pairs             │    │   │
│  │  │  - Calculates spread (CEX vs DEX)                           │    │   │
│  │  │  - Estimates gas costs                                       │    │   │
│  │  │  - Computes profit (gross - gas - fees)                     │    │   │
│  │  │  - Identifies profitable opportunities                       │    │   │
│  │  └─────────────────────────────────────────────────────────────┘    │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                    │                                        │
│                                    ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │                         PRESENTATION LAYER                           │   │
│  │  ┌─────────────────────┐        ┌─────────────────────┐             │   │
│  │  │    TUI (default)    │        │    CLI (debug)      │             │   │
│  │  │  - Bubble Tea UI    │        │  - Structured logs  │             │   │
│  │  │  - Real-time prices │        │  - JSON output      │             │   │
│  │  │  - Opportunities    │        │  - Full trace       │             │   │
│  │  │  - Cost breakdown   │        │                     │             │   │
│  │  └─────────────────────┘        └─────────────────────┘             │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Module Structure

```
arbitrage-bot/
├── cmd/
│   └── arbitrage/
│       └── main.go              # Entry point, flag parsing, module init
│
├── business/                    # Domain-driven business logic
│   ├── arbitrage/              # Arbitrage bounded context
│   │   ├── domain/             # Core domain types
│   │   │   ├── opportunity.go  # Opportunity entity
│   │   │   ├── direction.go    # Trade direction (CEX→DEX, DEX→CEX)
│   │   │   └── costs.go        # GasCost, ProfitResult value objects
│   │   ├── app/                # Application services
│   │   │   ├── detector.go     # Main detection orchestrator
│   │   │   ├── calculator.go   # Profit calculator
│   │   │   └── reporter.go     # Reporter interface (TUI/CLI)
│   │   └── di/                 # Dependency injection
│   │       └── container.go    # Service container
│   │
│   ├── pricing/                # Pricing bounded context
│   │   ├── domain/             # Core domain types
│   │   │   ├── price.go        # Price, Quote value objects
│   │   │   ├── spread.go       # Spread calculation
│   │   │   └── pair.go         # Trading pair
│   │   ├── app/                # Application services
│   │   │   └── service.go      # PricingService (aggregates CEX+DEX)
│   │   └── infra/              # Infrastructure adapters
│   │       ├── binance/        # Binance CEX adapter
│   │       │   ├── client.go      # WebSocket client
│   │       │   ├── http_client.go # REST API client (fallback)
│   │       │   └── provider.go    # CEXProvider (WS + HTTP fallback)
│   │       └── uniswap/        # Uniswap DEX adapter
│   │           └── provider.go # DEXProvider implementation
│   │
│   └── blockchain/             # Blockchain bounded context
│       ├── domain/             # Core domain types
│       │   ├── block.go        # Block entity
│       │   └── gas.go          # GasPrice value object
│       ├── app/                # Application services
│       │   └── service.go      # BlockchainService
│       └── infra/              # Infrastructure adapters
│           └── ethereum/       # Ethereum adapter
│               └── subscriber.go # Block subscription (WS + HTTP fallback)
│
├── internal/                   # Shared internal packages
│   ├── asset/                  # Asset/Amount handling
│   ├── config/                 # Configuration loading
│   ├── logger/                 # Structured logging (slog)
│   ├── apm/                    # Tracing (OpenTelemetry)
│   ├── metrics/                # Metrics (Prometheus)
│   ├── apperror/               # Structured error handling
│   ├── wsconn/                 # Generic WebSocket client
│   ├── httpclient/             # Instrumented HTTP client (OTEL)
│   ├── circuitbreaker/         # Circuit breaker pattern
│   ├── ratelimit/              # Rate limiting
│   └── monolith/               # Application container
│
├── pkg/                        # Public packages
│   └── ui/                     # TUI components
│       ├── tui.go              # Main Bubble Tea model
│       ├── messages.go         # TUI message types
│       ├── styles.go           # Lipgloss styles
│       └── components/         # Reusable UI components
│
└── docs/                       # Documentation
    ├── architecture.md         # This file
    ├── dataflow.md             # Data flow diagrams
    └── websocket.md            # WebSocket implementation details
```

---

## Key Design Patterns

### 1. Hexagonal Architecture (Ports & Adapters)

```
                    ┌─────────────────────────────────┐
                    │         APPLICATION CORE        │
                    │  ┌───────────────────────────┐  │
                    │  │       DOMAIN LAYER        │  │
                    │  │  - Opportunity            │  │
                    │  │  - Spread                 │  │
     DRIVING        │  │  - ProfitResult          │  │        DRIVEN
     ADAPTERS       │  └───────────────────────────┘  │       ADAPTERS
  (Primary Ports)   │  ┌───────────────────────────┐  │   (Secondary Ports)
        │           │  │    APPLICATION LAYER      │  │           │
        │           │  │  - Detector               │  │           │
        ▼           │  │  - PricingService         │  │           ▼
  ┌──────────┐      │  │  - Calculator             │  │     ┌──────────┐
  │   TUI    │◄────►│  └───────────────────────────┘  │◄───►│ Binance  │
  │   CLI    │      │                                 │     │ Uniswap  │
  └──────────┘      └─────────────────────────────────┘     │ Ethereum │
                                                            └──────────┘
```

### 2. Domain-Driven Design

- **Bounded Contexts**: `arbitrage`, `pricing`, `blockchain`
- **Entities**: `Opportunity`, `Block`
- **Value Objects**: `Spread`, `GasCost`, `ProfitResult`, `Price`, `Amount`, `Quote`
- **Domain Types**: `ExecutionStep`, `RiskFactor`, `Direction`
- **Domain Services**: `ProfitCalculator`, `SpreadCalculator`
- **Application Services**: `Detector`, `PricingService`, `BlockchainService`

### 3. Event-Driven Architecture

```
Block Event (Ethereum)
        │
        ▼
    Detector
        │
        ├──► Price Fetch (Binance + Uniswap)
        │
        ├──► Spread Calculation
        │
        ├──► Profit Calculation
        │
        └──► Opportunity Event ──► Reporter ──► TUI/CLI
```

---

## Configuration

```yaml
# config.yaml
app:
  name: "arbitrage-bot"
  environment: "development"  # development, staging, production
  log_level: "info"           # debug, info, warn, error

ethereum:
  rpc_url: "wss://eth-mainnet.g.alchemy.com/v2/..."
  http_url: "https://eth-mainnet.g.alchemy.com/v2/..."  # fallback

binance:
  ws_url: "wss://stream.binance.com:9443"
  api_url: "https://api.binance.com"        # HTTP fallback URL
  enable_fallback: true                      # Auto-fallback when WS stale
  stale_timeout: 5s                          # Data staleness threshold

uniswap:
  quoter_address: "0x61fFE014bA17989E743c5F6cB21bF9697530B21e"
  pool_factory: "0x1F98431c8aD98523631AE4a59f267346ea31F984"

arbitrage:
  pairs:
    - base: "ETH"
      quote: "USDC"
  trade_sizes: [1, 10, 100]   # ETH amounts to analyze
  min_profit_bps: 10          # Minimum spread in basis points
  min_profit_usd: 50          # Minimum profit in USD

telemetry:
  enabled: false
  service_name: "arbitrage-bot"
  otlp_endpoint: ""
  prometheus_port: 9090
```

---

## Technology Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| Language | Go 1.21+ | High performance, concurrency |
| TUI | Bubble Tea + Lipgloss | Terminal UI framework |
| WebSocket | coder/websocket | WS connections (nhooyr fork) |
| Ethereum | go-ethereum | Blockchain interaction |
| Decimals | shopspring/decimal | Precise financial math |
| Logging | log/slog | Structured logging |
| Tracing | OpenTelemetry | Distributed tracing |
| Metrics | Prometheus | Metrics collection |
| Config | YAML + env vars | Configuration management |

---

## Security Considerations

1. **No private keys in code** - Uses read-only RPC endpoints
2. **Rate limiting** - Respects API rate limits
3. **Circuit breakers** - Prevents cascade failures
4. **Input validation** - All external data validated
5. **Error handling** - Structured errors with context, no panics

---

## Deployment

### Running Locally
```bash
# Build
go build -o arbitrage-bot ./cmd/arbitrage

# Run with TUI (default)
./arbitrage-bot

# Run with CLI (debug mode)
./arbitrage-bot -cli

# Run with custom config
./arbitrage-bot -config /path/to/config.yaml
```

### Environment Variables
```bash
export ETH_RPC_URL="wss://..."
export BINANCE_WS_URL="wss://stream.binance.com:9443"
export OTEL_EXPORTER_OTLP_ENDPOINT="https://..."
export OTEL_SERVICE_NAME="arbitrage-bot"
```
