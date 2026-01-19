# Data Flow Documentation

## Overview

The arbitrage bot follows an event-driven architecture where new Ethereum blocks trigger price analysis across CEX (Binance) and DEX (Uniswap) to detect arbitrage opportunities.

---

## 1. Block Trigger Flow

New blocks from Ethereum trigger the entire detection cycle.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           BLOCK SUBSCRIPTION                                │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   Ethereum Node                                                             │
│       │                                                                     │
│       │ eth_subscribe("newHeads")                                           │
│       ▼                                                                     │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │              SUBSCRIBER (WebSocket Primary)                 │           │
│   │  business/blockchain/infra/ethereum/subscriber.go           │           │
│   │                                                             │           │
│   │  - Maintains WebSocket connection to Ethereum node          │           │
│   │  - Reconnects automatically with exponential backoff        │           │
│   │  - Circuit breaker prevents repeated failures               │           │
│   └─────────────────────────────────────────────────────────────┘           │
│       │                                                                     │
│       │ (if WebSocket fails)                                                │
│       ▼                                                                     │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │              HTTP POLLING (Fallback)                        │           │
│   │                                                             │           │
│   │  - Polls eth_blockNumber every 12 seconds                   │           │
│   │  - Fetches block header with eth_getBlockByNumber           │           │
│   │  - Separate circuit breaker from WebSocket                  │           │
│   └─────────────────────────────────────────────────────────────┘           │
│       │                                                                     │
│       │ Block{Number, Hash, Timestamp, GasLimit, GasUsed}                   │
│       ▼                                                                     │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │              BLOCKS CHANNEL (buffered: 16)                  │           │
│   │                                                             │           │
│   │  chan *domain.Block                                         │           │
│   │  WARNING: Drops blocks if buffer full (logged)              │           │
│   └─────────────────────────────────────────────────────────────┘           │
│       │                                                                     │
│       ▼                                                                     │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │              DETECTOR (Consumer)                            │           │
│   │  business/arbitrage/app/detector.go                         │           │
│   │                                                             │           │
│   │  for block := range blocks {                                │           │
│   │      d.onNewBlock(ctx, block)                               │           │
│   │  }                                                          │           │
│   └─────────────────────────────────────────────────────────────┘           │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Key Files:**
- `business/blockchain/infra/ethereum/subscriber.go` - Block subscription
- `business/arbitrage/app/detector.go:89-101` - Block consumer loop

---

## 2. Price Fetch Flow

On each new block, prices are fetched from both CEX and DEX.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           PRICE FETCHING                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   onNewBlock(block)                                                         │
│       │                                                                     │
│       │ For each configured pair (e.g., ETH/USDC)                           │
│       │ For each trade size (e.g., 1, 10, 100 ETH)                          │
│       ▼                                                                     │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │              PRICING SERVICE                                │           │
│   │  business/pricing/app/service.go                            │           │
│   │                                                             │           │
│   │  GetPriceSnapshot(ctx, pair, tradeSize) *PriceSnapshot      │           │
│   └──────────────────────┬──────────────────────────────────────┘           │
│                          │                                                  │
│          ┌───────────────┴───────────────┐                                  │
│          ▼                               ▼                                  │
│   ┌──────────────────┐          ┌──────────────────┐                        │
│   │  CEX PROVIDER    │          │  DEX PROVIDER    │                        │
│   │  (Binance)       │          │  (Uniswap)       │                        │
│   └────────┬─────────┘          └────────┬─────────┘                        │
│            │                             │                                  │
│            ▼                             ▼                                  │
│   ┌──────────────────┐          ┌──────────────────┐                        │
│   │  ORDERBOOK       │          │  QUOTER V3       │                        │
│   │  (in-memory)     │          │  (contract call) │                        │
│   │                  │          │                  │                        │
│   │  - Top 20 bids   │          │  - Quote for     │                        │
│   │  - Top 20 asks   │          │    exact input   │                        │
│   │  - VWAP calc     │          │  - Tries 4 fee   │                        │
│   │    per size      │          │    tiers         │                        │
│   └────────┬─────────┘          └────────┬─────────┘                        │
│            │                             │                                  │
│            │ CEXAsk, CEXBid              │ DEXQuote                         │
│            └──────────────┬──────────────┘                                  │
│                           ▼                                                 │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │              PRICE SNAPSHOT                                 │           │
│   │                                                             │           │
│   │  {                                                          │           │
│   │    Pair:     "ETH/USDC",                                    │           │
│   │    CEXAsk:   {Rate: 3400.50, Size: 10},                     │           │
│   │    CEXBid:   {Rate: 3400.00, Size: 10},                     │           │
│   │    DEXQuote: {Price: 3395.00, AmountOut: 33950},            │           │
│   │    Timestamp: now()                                         │           │
│   │  }                                                          │           │
│   └─────────────────────────────────────────────────────────────┘           │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Key Files:**
- `business/pricing/app/service.go` - PricingService
- `business/pricing/infra/binance/provider.go` - Binance orderbook (WS + HTTP fallback)
- `business/pricing/infra/binance/http_client.go` - REST API client for fallback
- `business/pricing/infra/uniswap/provider.go` - Uniswap quoter
- `internal/httpclient/client.go` - Instrumented HTTP client

---

## 3. Binance WebSocket Flow (with HTTP Fallback)

Binance prices are continuously updated via WebSocket, with automatic HTTP fallback.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    BINANCE DATA FLOW (WS + HTTP FALLBACK)                   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   wss://stream.binance.com:9443/stream?streams=ethusdc@bookTicker           │     /ethusdc@depth20@100ms
│       │                                                                     │
│       │ JSON Messages (bookTicker + partial depth)                          │
│       ▼                                                                     │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │              WSCONN CLIENT (Generic WebSocket)              │           │
│   │  internal/wsconn/wsconn.go                                  │           │
│   │                                                             │           │
│   │  - Maintains connection with auto-reconnect                 │           │
│   │  - Exponential backoff (1s → 30s max)                       │           │
│   │  - Ping/pong heartbeat every 30s                            │           │
│   │  - OTEL tracing + metrics                                   │           │
│   │  - Message buffer (1024 messages)                           │           │
│   │  - Max message size limit (10MB)                            │           │
│   └─────────────────────────────────────────────────────────────┘           │
│       │                                                                     │
│       │ OnMessage callback                                                  │
│       ▼                                                                     │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │              BINANCE CLIENT                                 │           │
│   │  business/pricing/infra/binance/client.go                   │           │
│   │                                                             │           │
│   │  handleMessage(data []byte)                                 │           │
│   │    ├── Parse JSON                                           │           │
│   │    ├── Route by stream type                                 │           │
│   │    └── Call registered handlers                             │           │
│   └─────────────────────────────────────────────────────────────┘           │
│       │                                                                     │
│       │ BookTickerEvent{Symbol, BidPrice, AskPrice, ...}                    │
│       ▼                                                                     │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │              BINANCE PROVIDER                               │           │
│   │  business/pricing/infra/binance/provider.go                 │           │
│   │                                                             │           │
│   │  handleBookTicker(event)  → Update best bid/ask             │           │
│   │  handleDepthUpdate(event) → Replace top 20 levels           │           │
│   │                                                             │           │
│   │  orderbookState: map[symbol]*OrderbookState                 │           │
│   │    ├── bids: []OrderbookLevel (sorted desc)                 │           │
│   │    ├── asks: []OrderbookLevel (sorted asc)                  │           │
│   │    ├── lastUpdate: time.Time                                │           │
│   │    └── RWMutex protected                                    │           │
│   └─────────────────────────────────────────────────────────────┘           │
│                                                                             │
│   When GetOrderbook(pair) called:                                           │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │  1. Check if WS data is stale (lastUpdate > staleTimeout)   │           │
│   │     ├── Fresh data → Return cached orderbook                │           │
│   │     └── Stale/Empty → Trigger HTTP fallback ──────────────┐ │           │
│   │                                                           │ │           │
│   │  ┌──────────────────────────────────────────────────────┐ │ │           │
│   │  │            HTTP FALLBACK (if enabled)                │ │ │           │
│   │  │  business/pricing/infra/binance/http_client.go       │ │ │           │
│   │  │                                                      │ │ │           │
│   │  │  GET https://api.binance.com/api/v3/depth            │◄─┘ │          │
│   │  │      ?symbol=ETHUSDC&limit=20                        │    │          │
│   │  │                                                      │    │          │
│   │  │  - Instrumented with OTEL tracing                    │    │          │
│   │  │  - Metrics: http_client_requests_total               │    │          │
│   │  │  - Updates local cache on success                    │    │          │
│   │  └──────────────────────────────────────────────────────┘    │          │
│   │                                                              │          │
│   │  2. Calculate VWAP for requested size                        │          │
│   │     - Walk through price levels                              │          │
│   │     - Accumulate cost until size filled                      │          │
│   │     - avgPrice = totalCost / totalFilled                     │          │
│   │  3. Return Price{Rate: VWAP, Size: filled}                   │          │
│   └──────────────────────────────────────────────────────────────┘          │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Fallback Decision Logic:**
```
GetOrderbook(pair)
    │
    ├── WS data fresh (lastUpdate < 5s)?
    │       │
    │       ├── YES → Return cached orderbook
    │       │
    │       └── NO → Is HTTP fallback enabled?
    │               │
    │               ├── YES → Fetch via REST API
    │               │           ├── Success → Update cache, return data
    │               │           └── Error → Return error
    │               │
    │               └── NO → Return "stale data" error
```

**Keep-Alive:**
- WebSocket layer: ping/pong every 30 seconds (detects half-open connections)
- Binance layer: `LIST_SUBSCRIPTIONS` every 2 minutes (Binance requires activity every 3 min)

**HTTP Fallback Triggers:**
- WebSocket data older than `stale_timeout` (default: 5s)
- No WebSocket data received yet (cold start)
- WebSocket connection lost (during reconnection)

---

## 4. Analysis Flow

Spread calculation and profit analysis.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           ANALYSIS FLOW                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   analyzeOpportunity(ctx, block, pair, tradeSize, gasPrice)                 │
│       │                                                                     │
│       │ 1. Get Price Snapshot                                               │
│       ▼                                                                     │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │  snapshot = pricing.GetPriceSnapshot(pair, tradeSize)       │           │
│   │                                                             │           │
│   │  CEX Ask: $3,400.50 (buy ETH price on Binance)              │           │
│   │  DEX Quote: $3,395.00 (effective price on Uniswap)          │           │
│   └─────────────────────────────────────────────────────────────┘           │
│       │                                                                     │
│       │ 2. Calculate Spread                                                 │
│       ▼                                                                     │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │  spread = CalculateSpread(cexPrice, dexPrice)               │           │
│   │                                                             │           │
│   │  Absolute: DEX - CEX = -$5.50                               │           │
│   │  BasisPoints: (DEX - CEX) / CEX * 10000 = -16.17 bps        │           │
│   │  Direction: DEX_TO_CEX (DEX cheaper, buy on DEX)            │           │
│   └─────────────────────────────────────────────────────────────┘           │
│       │                                                                     │
│       │ 3. Calculate Gas Cost                                               │
│       ▼                                                                     │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │  gasCost = NewGasCost(gasLimit=200000, gasPrice, ethPrice)  │           │
│   │                                                             │           │
│   │  Gas Limit: 200,000 units (swap estimate)                   │           │
│   │  Gas Price: 25 gwei                                         │           │
│   │  ETH Price: $3,400                                          │           │
│   │  Total: 200000 * 25 * 1e-9 * 3400 = $17.00                  │           │
│   └─────────────────────────────────────────────────────────────┘           │
│       │                                                                     │
│       │ 4. Calculate Profit                                                 │
│       ▼                                                                     │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │  profit = calculator.Calculate(spread, tradeSize, gasCost)  │           │
│   │                                                             │           │
│   │  Trade Value: 10 ETH * $3,400 = $34,000                     │           │
│   │  Gross Profit: |spread| * size = $55.00                     │           │
│   │  Gas Cost: $17.00                                           │           │
│   │  Exchange Fees: $34,000 * 0.4% = $136.00                    │           │
│   │    - Uniswap: 0.3%                                          │           │
│   │    - Binance: 0.1%                                          │           │
│   │  Total Costs: $17 + $136 = $153.00                          │           │
│   │  Net Profit: $55 - $153 = -$98.00 (NOT PROFITABLE)          │           │
│   └─────────────────────────────────────────────────────────────┘           │
│       │                                                                     │
│       │ 5. Build Opportunity                                                │
│       ▼                                                                     │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │  opportunity = &Opportunity{                                │           │
│   │    ID:            "21000000-ETHUSDC-10",                    │           │
│   │    BlockNumber:   21000000,                                 │           │
│   │    Direction:     DEX_TO_CEX,                               │           │
│   │    TradeSize:     10,                                       │           │
│   │    CEXPrice:      3400.50,                                  │           │
│   │    DEXPrice:      3395.00,                                  │           │
│   │    Spread:        {...},                                    │           │
│   │    Profit:        {IsProfitable: false, ...},               │           │
│   │    DEXQuote:      {FeeTier: 3000, ...},  // 0.30% pool      │           │
│   │    RequiredCapital: 34000.00,  // USDC needed               │           │
│   │    ExecutionSteps: [                                        │           │
│   │      {1, "Buy 10 ETH on Binance at $3,400.50"},             │           │
│   │      {2, "Transfer ETH to trading wallet"},                 │           │
│   │      {3, "Execute Uniswap V3 swap via 0.30% pool"},         │           │
│   │      {4, "Receive USDC from swap"},                         │           │
│   │    ],                                                       │           │
│   │    RiskFactors: [                                           │           │
│   │      {Name: "Slippage", Severity: "low"},                   │           │
│   │      {Name: "MEV", Severity: "medium"},                     │           │
│   │      {Name: "Timing", Severity: "low"},                     │           │
│   │    ],                                                       │           │
│   │  }                                                          │           │
│   └─────────────────────────────────────────────────────────────┘           │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Key Files:**
- `business/pricing/domain/spread.go` - Spread calculation
- `business/arbitrage/domain/costs.go` - GasCost, ProfitResult
- `business/arbitrage/app/calculator.go` - ProfitCalculator
- `business/arbitrage/app/detector.go:152-251` - analyzeOpportunity

---

## 5. Reporting Flow

Results are sent to the TUI or CLI for display.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           REPORTING FLOW                                    │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   Detector                                                                  │
│       │                                                                     │
│       ├── reporter.UpdatePrices(snapshot)                                   │
│       │       └── ui.Send(PriceUpdateMsg{Snapshot})                         │
│       │                                                                     │
│       ├── reporter.UpdateBlock(blockNumber)                                 │
│       │       └── ui.Send(BlockMsg{Number})                                 │
│       │                                                                     │
│       ├── reporter.UpdateGasPrice(gweiPrice)                                │
│       │       └── ui.Send(GasPriceMsg{GweiPrice})                           │
│       │                                                                     │
│       ├── reporter.UpdateCostBreakdown(breakdown)                           │
│       │       └── ui.Send(CostBreakdownMsg{...})                            │
│       │                                                                     │
│       └── reporter.Report(opportunity)  // if profitable                    │
│               └── ui.Send(OpportunityMsg{Opportunity})                      │
│                                                                             │
│   ┌─────────────────────────────────────────────────────────────┐           │
│   │              TUI (Bubble Tea)                               │           │
│   │  pkg/ui/tui.go                                              │           │
│   │                                                             │           │
│   │  Update(msg) (tea.Model, tea.Cmd)                           │           │
│   │    switch msg.(type) {                                      │           │
│   │    case PriceUpdateMsg:                                     │           │
│   │        m.prices.Update(snapshot)                            │           │
│   │    case BlockMsg:                                           │           │
│   │        m.currentBlock = msg.Number                          │           │
│   │    case OpportunityMsg:                                     │           │
│   │        m.opportunities.Add(opportunity)                     │           │
│   │    case CostBreakdownMsg:                                   │           │
│   │        m.prices.SetCostBreakdown(msg)                       │           │
│   │    }                                                        │           │
│   │                                                             │           │
│   │  View() string                                              │           │
│   │    ├── renderStatusBar()  // Block, Gas, Connections        │           │
│   │    ├── prices.View()      // Price table + Cost breakdown   │           │
│   │    └── opportunities.View() // Opportunity list             │           │
│   └─────────────────────────────────────────────────────────────┘           │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Message Types** (`pkg/ui/messages.go`):
- `PriceUpdateMsg` - Price snapshot update
- `BlockMsg` - New block received
- `GasPriceMsg` - Gas price update
- `CostBreakdownMsg` - Cost analysis (domain-calculated)
- `OpportunityMsg` - Profitable opportunity detected
- `ConnectionStatusMsg` - Connection state change

---

## 6. Complete Flow Sequence

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        COMPLETE DETECTION CYCLE                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  Time ──────────────────────────────────────────────────────────────►       │
│                                                                             │
│  T+0ms    Ethereum emits new block #21000000                                │
│     │                                                                       │
│     ▼                                                                       │
│  T+50ms   Subscriber receives block via WebSocket                           │
│     │     └── Sends to blocks channel                                       │
│     ▼                                                                       │
│  T+51ms   Detector.onNewBlock() triggered                                   │
│     │     ├── Updates block in reporter                                     │
│     │     └── Fetches current gas price                                     │
│     ▼                                                                       │
│  T+52ms   Detector.processPair(ETH/USDC) for each trade size                │
│     │                                                                       │
│     │     ┌─────────────────────────────────────────────┐                   │
│     │     │  Trade Size: 1 ETH                          │                   │
│     │     │  T+52ms  Get Binance price (from cache)     │                   │
│     │     │  T+53ms  Get Uniswap quote (RPC call)       │                   │
│     │     │  T+150ms Calculate spread, gas, profit      │                   │
│     │     │  T+151ms Report to TUI                      │                   │
│     │     └─────────────────────────────────────────────┘                   │
│     │                                                                       │
│     │     ┌─────────────────────────────────────────────┐                   │
│     │     │  Trade Size: 10 ETH                         │                   │
│     │     │  T+152ms Get Binance price (from cache)     │                   │
│     │     │  T+153ms Get Uniswap quote (RPC call)       │                   │
│     │     │  T+250ms Calculate spread, gas, profit      │                   │
│     │     │  T+251ms Report to TUI                      │                   │
│     │     └─────────────────────────────────────────────┘                   │
│     │                                                                       │
│     │     ┌─────────────────────────────────────────────┐                   │
│     │     │  Trade Size: 100 ETH                        │                   │
│     │     │  T+252ms Get Binance price (from cache)     │                   │
│     │     │  T+253ms Get Uniswap quote (RPC call)       │                   │
│     │     │  T+350ms Calculate spread, gas, profit      │                   │
│     │     │  T+351ms Report to TUI                      │                   │
│     │     │  T+352ms Send best cost breakdown           │                   │
│     │     └─────────────────────────────────────────────┘                   │
│     │                                                                       │
│     ▼                                                                       │
│  T+400ms  Cycle complete, wait for next block (~12 seconds)                 │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Latency Breakdown:**
- Block propagation: ~50ms
- Binance price fetch: <1ms (from cache)
- Uniswap quote: ~100ms (RPC call)
- Calculations: <1ms
- **Total per trade size: ~100-150ms**
- **Total per block: ~300-500ms** (3 trade sizes)
