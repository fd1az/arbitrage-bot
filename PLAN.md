# CEX-DEX Arbitrage Bot - Implementation Plan

## Summary

Build a **real-time arbitrage detection system** in Go that monitors price discrepancies between Binance (CEX) and Uniswap V3 (DEX) for ETH-USDC. The system will be **production-grade** with WebSocket resilience, caching, circuit breakers, and full observability.

## Challenge Analysis

| Requirement | Priority | Reuse from trd-* |
|-------------|----------|------------------|
| Binance orderbook integration | Core | HTTP client pattern ✓ |
| Ethereum block streaming (WebSocket) | Core | WS pattern (needs hardening) |
| Uniswap V3 QuoterV2 | Core | New |
| Arbitrage detection | Core | New |
| Multi-layer caching | Senior | New (generic cache) |
| WebSocket reconnection w/backoff | Senior | Needs production upgrade |
| Circuit breaker | Senior | New (sony/gobreaker) |
| Rate limiting | Senior | x/time/rate |
| Configuration (YAML/env) | Senior | Viper pattern ✓ |
| Observability (logs + OTEL) | Senior | Full stack ✓ |

## Interface Decision

**CLI + TUI con Bubble Tea**:
- CLI mode: Structured stdout output (as challenge specifies)
- TUI mode: Real-time dashboard con Bubble Tea mostrando:
  - Precios CEX/DEX en tiempo real
  - Estado de conexiones (WS/HTTP)
  - Oportunidades detectadas
  - Métricas de performance
- Seleccionable via flag `--tui` o config

## Architecture

**Patrón**: Hexagonal Architecture / Ports & Adapters (mismo patrón que trd-mcp)

```
arbitrage-bot/
├── cmd/arbitrage/main.go              # Entry point, DI, graceful shutdown
│
├── business/                          # ══════ BOUNDED CONTEXTS ══════
│   │
│   ├── pricing/                       # ─── Pricing Context ───
│   │   ├── app/
│   │   │   ├── service.go             # PricingService (application layer)
│   │   │   └── ports.go               # CEXProvider, DEXProvider interfaces
│   │   ├── domain/
│   │   │   ├── price.go               # Price, Orderbook, Quote value objects
│   │   │   └── spread.go              # Spread calculation logic
│   │   ├── infra/
│   │   │   ├── binance/               # CEXProvider adapter (Binance API)
│   │   │   │   ├── client.go
│   │   │   │   ├── orderbook.go
│   │   │   │   └── mapper.go
│   │   │   └── uniswap/               # DEXProvider adapter (QuoterV2)
│   │   │       ├── quoter.go
│   │   │       ├── abi.go
│   │   │       └── mapper.go
│   │   ├── di/
│   │   │   └── tokens.go              # DI tokens for this module
│   │   └── module.go                  # RegisterServices()
│   │
│   ├── blockchain/                    # ─── Blockchain Context ───
│   │   ├── app/
│   │   │   ├── service.go             # BlockchainService (block streaming)
│   │   │   └── ports.go               # BlockSubscriber, GasOracle interfaces
│   │   ├── domain/
│   │   │   ├── block.go               # Block, BlockHeader value objects
│   │   │   └── gas.go                 # GasPrice, GasEstimate
│   │   ├── infra/
│   │   │   └── ethereum/              # Ethereum adapter (WS + HTTP fallback)
│   │   │       ├── client.go          # Main client with reconnection
│   │   │       ├── subscriber.go      # eth_subscribe("newHeads")
│   │   │       ├── poller.go          # HTTP polling fallback
│   │   │       └── gas.go             # Gas price fetching
│   │   ├── di/
│   │   │   └── tokens.go
│   │   └── module.go
│   │
│   └── arbitrage/                     # ─── Arbitrage Context ───
│       ├── app/
│       │   ├── detector.go            # ArbitrageDetector (orchestrates)
│       │   ├── calculator.go          # ProfitCalculator (spread, costs, net)
│       │   └── ports.go               # Reporter interface
│       ├── domain/
│       │   ├── opportunity.go         # ArbitrageOpportunity entity
│       │   ├── direction.go           # CEXtoDEX, DEXtoCEX
│       │   └── costs.go               # TradeCosts (gas, fees)
│       ├── infra/
│       │   ├── console_reporter.go    # CLI stdout output
│       │   └── tui_reporter.go        # Bubble Tea integration
│       ├── di/
│       │   └── tokens.go
│       └── module.go
│
├── internal/                          # ══════ CROSS-CUTTING CONCERNS ══════
│   ├── config/                        # Viper-based configuration
│   ├── logger/                        # COPY from trd-mcp
│   ├── apm/                           # COPY from trd-mcp
│   ├── metrics/                       # COPY from trd-mcp
│   ├── di/                            # COPY from trd-mcp (service registry)
│   ├── apperror/                      # COPY + extend with arb error codes
│   ├── cache/                         # NEW: Generic TTL cache
│   ├── circuitbreaker/                # NEW: sony/gobreaker wrapper
│   ├── ratelimit/                     # NEW: x/time/rate wrapper
│   └── wsconn/                        # NEW: Production WebSocket client
│
├── pkg/                               # ══════ SHARED/UI ══════
│   └── ui/                            # TUI (Bubble Tea)
│       ├── tui.go                     # Main Bubble Tea model
│       ├── components/
│       │   ├── status.go              # Connection status
│       │   ├── prices.go              # Price table
│       │   ├── opportunities.go       # Opportunities list
│       │   └── stats.go               # Statistics bar
│       ├── styles.go                  # Lipgloss styles
│       └── keys.go                    # Keybindings
│
└── config.yaml
```

### Hexagonal Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              APPLICATION CORE                                │
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                     arbitrage/app/detector.go                        │    │
│  │  ┌───────────────────────────────────────────────────────────────┐  │    │
│  │  │                    ArbitrageDetector                          │  │    │
│  │  │  - Subscribes to blocks (via BlockSubscriber port)            │  │    │
│  │  │  - Fetches prices (via CEXProvider, DEXProvider ports)        │  │    │
│  │  │  - Calculates profit (ProfitCalculator)                       │  │    │
│  │  │  - Reports opportunities (via Reporter port)                  │  │    │
│  │  └───────────────────────────────────────────────────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────────────┘    │
│                                     │                                        │
│         ┌───────────────────────────┼───────────────────────────┐           │
│         │                           │                           │           │
│         ▼                           ▼                           ▼           │
│  ┌─────────────┐           ┌─────────────┐            ┌─────────────┐       │
│  │ PORTS       │           │ PORTS       │            │ PORTS       │       │
│  │ (interfaces)│           │ (interfaces)│            │ (interfaces)│       │
│  ├─────────────┤           ├─────────────┤            ├─────────────┤       │
│  │CEXProvider  │           │DEXProvider  │            │BlockSubscr. │       │
│  │GetOrderbook │           │GetQuote     │            │Subscribe    │       │
│  └─────────────┘           └─────────────┘            └─────────────┘       │
│         │                           │                           │           │
└─────────┼───────────────────────────┼───────────────────────────┼───────────┘
          │                           │                           │
          ▼                           ▼                           ▼
┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
│    ADAPTERS     │         │    ADAPTERS     │         │    ADAPTERS     │
│ (infra layer)   │         │ (infra layer)   │         │ (infra layer)   │
├─────────────────┤         ├─────────────────┤         ├─────────────────┤
│ pricing/infra/  │         │ pricing/infra/  │         │ blockchain/     │
│ binance/        │         │ uniswap/        │         │ infra/ethereum/ │
│                 │         │                 │         │                 │
│ - HTTP client   │         │ - QuoterV2 ABI  │         │ - WS client     │
│ - Rate limiter  │         │ - eth_call      │         │ - HTTP fallback │
│ - Circuit break │         │ - Cache         │         │ - Reconnection  │
└─────────────────┘         └─────────────────┘         └─────────────────┘
          │                           │                           │
          ▼                           ▼                           ▼
    ┌──────────┐              ┌──────────┐               ┌──────────┐
    │ Binance  │              │ Ethereum │               │ Ethereum │
    │   API    │              │   Node   │               │   Node   │
    └──────────┘              └──────────┘               └──────────┘
```

### Port Interfaces (contracts)

```go
// pricing/app/ports.go
type CEXProvider interface {
    GetOrderbook(ctx context.Context, pair domain.Pair) (*domain.Orderbook, error)
    GetEffectivePrice(ctx context.Context, pair domain.Pair, size decimal.Decimal, side domain.Side) (*domain.Price, error)
}

type DEXProvider interface {
    GetQuote(ctx context.Context, tokenIn, tokenOut common.Address, amountIn *big.Int) (*domain.Quote, error)
}

// blockchain/app/ports.go
type BlockSubscriber interface {
    Subscribe(ctx context.Context) (<-chan *domain.Block, error)
    LatestBlock(ctx context.Context) (*domain.Block, error)
}

type GasOracle interface {
    GetGasPrice(ctx context.Context) (*domain.GasPrice, error)
    EstimateGas(ctx context.Context, tx *domain.Transaction) (uint64, error)
}

// arbitrage/app/ports.go
type Reporter interface {
    Start(ctx context.Context) error
    Report(opp *domain.ArbitrageOpportunity)
    UpdatePrices(prices *domain.PriceSnapshot)
    UpdateConnectionStatus(name string, status *domain.ConnectionStatus)
    Stop() error
}
```

## Key Components

### 1. WebSocket Client (`internal/wsconn/`) - HIGH PRIORITY

Production-grade WebSocket client with:
- **Exponential backoff with jitter** (not just linear)
- **Heartbeat/ping mechanism** (30s ping, 10s pong timeout)
- **Connection state tracking** (disconnected, connecting, connected, reconnecting)
- **Last processed block tracking** (resume without gaps)
- **Graceful fallback to HTTP polling**

```go
type Client interface {
    Connect(ctx context.Context) error
    Send(ctx context.Context, msg []byte) error
    Messages() <-chan []byte
    State() State
    Close() error
}
```

**Library**: `github.com/coder/websocket` (formerly nhooyr.io/websocket - mejor context support que gorilla)

### 2. Cache (`internal/cache/`) - MEDIUM

**¿Por qué in-memory y no Redis?**
- **Latencia**: Redis agrega ~1-2ms por op. En arbitraje cada ms cuenta.
- **Datos efímeros**: Precios/orderbooks válidos por segundos, no vale persistir.
- **Single process**: Bot corre como proceso único, no necesita cache distribuido.
- **Challenge requirement**: Pide específicamente "L1: In-memory cache".

Generic in-memory cache with TTL (custom implementation con generics):
- Thread-safe (sync.RWMutex)
- TTL-based expiration
- Background cleanup goroutine
- Cache stats para metrics
- Warm-up support

```go
type Cache[K comparable, V any] interface {
    Get(ctx context.Context, key K) (V, bool)
    Set(ctx context.Context, key K, value V, ttl time.Duration)
    Stats() CacheStats
}
```

**Uso específico**:
- Orderbook cache: TTL ~100ms (se actualiza constantemente via WS)
- Gas price cache: TTL ~12s (1 bloque)
- Uniswap quotes: Invalidar en cada nuevo bloque
- Block headers: Keep last N blocks

### 3. Ethereum Client (`business/blockchain/infra/ethereum/`)

Block subscription with resilience (implements `BlockSubscriber` port):
- WebSocket primary via `eth_subscribe("newHeads")`
- HTTP polling fallback (12s interval)
- Automatic switchover on WS failure
- Track `lastBlockNumber` to detect gaps

### 4. Uniswap Quoter (`business/pricing/infra/uniswap/`)

QuoterV2 contract integration (implements `DEXProvider` port):
- Direct ABI pack/unpack (no codegen)
- `quoteExactInputSingle` for price quotes
- Cache quotes per block (very short TTL)

Key addresses:
- QuoterV2: `0x61fFE014bA17989E743c5F6cB21bF9697530B21e`
- WETH: `0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2`
- USDC: `0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48`
- ETH-USDC Pool (0.3%): `0x88e6A0c2dDD26FEEb64F039a2c41296FcB3f5640`

### 5. Binance Client (`business/pricing/infra/binance/`)

Orderbook fetching (implements `CEXProvider` port):
- Rate limiting (1200 req/min)
- Circuit breaker for API failures
- Cache orderbooks with short TTL
- Calculate effective price with slippage

### 6. Arbitrage Detector (`business/arbitrage/app/`)

Main detection loop:
1. Subscribe to new Ethereum blocks
2. On each block:
   - Fetch Binance orderbook (snapshot)
   - Query Uniswap QuoterV2 (at block)
   - Calculate effective prices for trade sizes [1, 10, 100] ETH
   - Detect opportunities where spread > costs
3. Emit to reporter channel

## Concurrency Model

```
main()
├── ethereum.SubscribeNewBlocks() [goroutine]
│   ├── wsClient.listen() [goroutine]
│   └── pollBlocks() [goroutine - fallback]
├── detector.Start() [goroutine]
│   └── onNewBlock() → checkPair() [per block]
├── reporter.Start() [goroutine]
└── metrics.ServePrometheus() [goroutine]
```

**Backpressure**: Non-blocking channel sends with drop + log.

## Configuration

```yaml
ethereum:
  websocket_url: "wss://mainnet.infura.io/ws/v3/${INFURA_API_KEY}"
  http_url: "https://mainnet.infura.io/v3/${INFURA_API_KEY}"
  max_reconnects: 0  # infinite
  initial_backoff: "1s"
  max_backoff: "30s"

binance:
  base_url: "https://api.binance.com"
  http_rate_limit: 1200

arbitrage:
  pairs: [{base: "ETH", quote: "USDC"}]
  trade_sizes: [1, 10, 100]
  min_profit_bps: 10
  min_profit_usd: 10

telemetry:
  enabled: true
  prometheus_port: 9090
```

## Files to Copy from Existing Projects

| Source | Destination | Changes |
|--------|-------------|---------|
| `trd-mcp/internal/logger/` | `internal/logger/` | None |
| `trd-mcp/internal/apm/` | `internal/apm/` | None |
| `trd-mcp/internal/metrics/` | `internal/metrics/` | None |
| `trd-mcp/internal/di/` | `internal/di/` | None |
| `trd-mcp/internal/apperror/` | `internal/apperror/` | Add arb error codes |
| `trd-octopus/internal/waiter/` | `internal/waiter/` | None |

## Files to Create New

### Cross-Cutting (`internal/`)

| Path | Complexity | Notes |
|------|------------|-------|
| `internal/config/` | Low | Viper-based |
| `internal/cache/` | Medium | Generic with TTL |
| `internal/circuitbreaker/` | Low | Wrap sony/gobreaker |
| `internal/ratelimit/` | Low | Wrap x/time/rate |
| `internal/wsconn/` | **High** | Production WS client |

### Pricing Context (`business/pricing/`)

| Path | Complexity | Notes |
|------|------------|-------|
| `business/pricing/app/ports.go` | Low | CEXProvider, DEXProvider interfaces |
| `business/pricing/app/service.go` | Medium | PricingService (coordinates CEX/DEX) |
| `business/pricing/domain/` | Low | Price, Orderbook, Quote, Spread |
| `business/pricing/infra/binance/` | Medium | CEXProvider adapter |
| `business/pricing/infra/uniswap/` | Medium | DEXProvider adapter (QuoterV2) |
| `business/pricing/di/tokens.go` | Low | DI tokens |
| `business/pricing/module.go` | Low | Module registration |

### Blockchain Context (`business/blockchain/`)

| Path | Complexity | Notes |
|------|------------|-------|
| `business/blockchain/app/ports.go` | Low | BlockSubscriber, GasOracle interfaces |
| `business/blockchain/app/service.go` | Medium | BlockchainService |
| `business/blockchain/domain/` | Low | Block, GasPrice |
| `business/blockchain/infra/ethereum/` | **High** | WS + HTTP fallback subscriber |
| `business/blockchain/di/tokens.go` | Low | DI tokens |
| `business/blockchain/module.go` | Low | Module registration |

### Arbitrage Context (`business/arbitrage/`)

| Path | Complexity | Notes |
|------|------------|-------|
| `business/arbitrage/app/ports.go` | Low | Reporter interface |
| `business/arbitrage/app/detector.go` | **High** | Main orchestrator |
| `business/arbitrage/app/calculator.go` | Medium | Profit calculation |
| `business/arbitrage/domain/` | Low | Opportunity, Direction, Costs |
| `business/arbitrage/infra/console_reporter.go` | Low | CLI output |
| `business/arbitrage/infra/tui_reporter.go` | Medium | Bubble Tea integration |
| `business/arbitrage/di/tokens.go` | Low | DI tokens |
| `business/arbitrage/module.go` | Low | Module registration |

### UI & Entry Point

| Path | Complexity | Notes |
|------|------------|-------|
| `pkg/ui/` | Medium | Bubble Tea TUI |
| `cmd/arbitrage/main.go` | Medium | Entry point + DI wiring |

## Dependencies

```go
// Essential
"github.com/ethereum/go-ethereum"
"github.com/coder/websocket"          // formerly nhooyr.io/websocket
"github.com/sony/gobreaker/v2"
"golang.org/x/time/rate"
"github.com/spf13/viper"
"github.com/go-resty/resty/v2"

// TUI
"github.com/charmbracelet/bubbletea"
"github.com/charmbracelet/lipgloss"
"github.com/charmbracelet/bubbles"    // components (table, spinner, etc.)

// Observability
"go.opentelemetry.io/otel"
"github.com/prometheus/client_golang"

// Math
"github.com/shopspring/decimal"       // or use big.Float
```

## TUI Design (Bubble Tea)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  🔗 CEX-DEX ARBITRAGE BOT                                    [Block: 18234567]│
├─────────────────────────────────────────────────────────────────────────────┤
│  CONNECTIONS                                                                │
│  ├─ Ethereum WS: ● Connected (12ms latency)                                │
│  ├─ Binance API: ● Connected (45ms latency)                                │
│  └─ Last Block:  18234567 (2s ago)                                         │
├─────────────────────────────────────────────────────────────────────────────┤
│  PRICES (ETH-USDC)                                          Updated: 14:23:45│
│  ┌─────────────┬──────────────┬──────────────┬──────────────┐              │
│  │ Trade Size  │ Binance (CEX)│ Uniswap (DEX)│   Spread     │              │
│  ├─────────────┼──────────────┼──────────────┼──────────────┤              │
│  │     1 ETH   │   $2,245.30  │   $2,247.80  │   +11.1 bps  │              │
│  │    10 ETH   │   $2,245.10  │   $2,248.20  │   +13.8 bps  │              │
│  │   100 ETH   │   $2,244.50  │   $2,250.10  │   +24.9 bps  │              │
│  └─────────────┴──────────────┴──────────────┴──────────────┘              │
├─────────────────────────────────────────────────────────────────────────────┤
│  OPPORTUNITIES (last 10)                                                    │
│  ┌─────────┬────────┬───────────┬─────────┬──────────┬─────────────────────┐│
│  │  Block  │  Size  │ Direction │ Spread  │  Profit  │      Status        ││
│  ├─────────┼────────┼───────────┼─────────┼──────────┼─────────────────────┤│
│  │18234567 │  10ETH │  CEX→DEX  │ +13.8bp │   $28.50 │ ✓ Profitable       ││
│  │18234560 │ 100ETH │  DEX→CEX  │ +18.2bp │  $182.00 │ ✓ Profitable       ││
│  │18234555 │   1ETH │  CEX→DEX  │  +8.1bp │   -$2.30 │ ✗ Gas > Profit     ││
│  └─────────┴────────┴───────────┴─────────┴──────────┴─────────────────────┘│
├─────────────────────────────────────────────────────────────────────────────┤
│  STATS                                                                      │
│  Blocks processed: 1,234  │  Opportunities: 47  │  Profitable: 23 (48.9%)  │
│  Avg latency: 156ms       │  Cache hit rate: 94.2%  │  Errors: 3           │
├─────────────────────────────────────────────────────────────────────────────┤
│  [q] Quit  [p] Pause  [c] Clear  [l] Logs  [m] Metrics                     │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Arquitectura TUI** (shared component in `pkg/ui/`):
```
pkg/ui/
├── tui.go              # Main Bubble Tea model
├── components/
│   ├── status.go       # Connection status component
│   ├── prices.go       # Price table component
│   ├── opportunities.go # Opportunities list
│   └── stats.go        # Statistics bar
├── styles.go           # Lipgloss styles
└── keys.go             # Keybindings
```

**Reporter Implementations** (in `business/arbitrage/infra/`):
```go
// business/arbitrage/app/ports.go - interface definition
type Reporter interface {
    Start(ctx context.Context) error
    Report(opp *domain.ArbitrageOpportunity)
    UpdatePrices(prices *domain.PriceSnapshot)
    UpdateConnectionStatus(name string, status *domain.ConnectionStatus)
    Stop() error
}

// business/arbitrage/infra/console_reporter.go - CLI output
// business/arbitrage/infra/tui_reporter.go - Bubble Tea integration (uses pkg/ui/)
```

## Output Format (CLI mode - as challenge specifies)

```
================================================================================
ARBITRAGE OPPORTUNITY DETECTED
================================================================================
Block:          #18234567
Timestamp:      2024-01-15 14:23:45 UTC
Pair:           ETH-USDC
Direction:      CEX → DEX (Buy on Binance, Sell on Uniswap)
--------------------------------------------------------------------------------
PRICES
  CEX (Binance):  $2,245.30
  DEX (Uniswap):  $2,267.80
  Spread:         100.00 bps
--------------------------------------------------------------------------------
TRADE DETAILS
  Size:           10.0000 ETH ($22,453.00)
  Gas Price:      50.00 Gwei
  Gas Cost:       0.010000 ETH ($22.45)
--------------------------------------------------------------------------------
PROFIT
  Gross:          $225.00
  Net:            $192.55 (0.86%)
================================================================================
```

## Verification Plan

1. **Unit Tests**: Cache, calculator, domain types
2. **Integration Tests**: Binance API, Ethereum node connection
3. **Manual Testing**:
   - Run with `INFURA_API_KEY` set
   - Observe block subscription logs
   - Verify orderbook fetches
   - Check Uniswap quotes
   - Watch for opportunity detection

```bash
# Run the bot (CLI mode)
go run ./cmd/arbitrage

# Run with TUI
go run ./cmd/arbitrage --tui

# Check metrics
curl http://localhost:9090/metrics
```

## Implementation Order

### Phase 1: Foundation
1. **Setup**: Project structure (`business/`, `internal/`, `pkg/`), copy infra from trd-mcp
2. **Config**: Viper-based configuration loading (`internal/config/`)
3. **Cache**: Generic TTL cache (`internal/cache/`) - test separately
4. **WebSocket Client**: Production-grade wsconn (`internal/wsconn/`)

### Phase 2: Blockchain Context
5. **Blockchain Domain**: Block, GasPrice types (`business/blockchain/domain/`)
6. **Blockchain Ports**: BlockSubscriber, GasOracle interfaces (`business/blockchain/app/ports.go`)
7. **Ethereum Adapter**: WS + HTTP fallback (`business/blockchain/infra/ethereum/`)
8. **Blockchain Module**: DI + registration (`business/blockchain/module.go`)

### Phase 3: Pricing Context
9. **Pricing Domain**: Price, Orderbook, Quote, Spread (`business/pricing/domain/`)
10. **Pricing Ports**: CEXProvider, DEXProvider interfaces (`business/pricing/app/ports.go`)
11. **Binance Adapter**: Orderbook + effective price (`business/pricing/infra/binance/`)
12. **Uniswap Adapter**: QuoterV2 integration (`business/pricing/infra/uniswap/`)
13. **Pricing Module**: DI + registration (`business/pricing/module.go`)

### Phase 4: Arbitrage Context
14. **Arbitrage Domain**: Opportunity, Direction, Costs (`business/arbitrage/domain/`)
15. **Arbitrage Ports**: Reporter interface (`business/arbitrage/app/ports.go`)
16. **Calculator**: Profit calculation (`business/arbitrage/app/calculator.go`)
17. **Detector**: Main orchestrator (`business/arbitrage/app/detector.go`)
18. **Console Reporter**: CLI output (`business/arbitrage/infra/console_reporter.go`)
19. **Arbitrage Module**: DI + registration (`business/arbitrage/module.go`)

### Phase 5: UI & Integration
20. **TUI Components**: Bubble Tea UI (`pkg/ui/`)
21. **TUI Reporter**: Bubble Tea integration (`business/arbitrage/infra/tui_reporter.go`)
22. **Main**: Wire everything with --tui flag (`cmd/arbitrage/main.go`)

### Phase 6: Polish
23. **Tests**: Unit tests for domain, integration tests for adapters
24. **Error Handling**: Extend apperror codes
25. **Observability**: Ensure metrics/tracing work

## Run Modes

```bash
# CLI mode (default) - structured output to stdout
./arbitrage-bot

# TUI mode - interactive dashboard
./arbitrage-bot --tui

# With custom config
./arbitrage-bot --config ./config.yaml --tui
```
