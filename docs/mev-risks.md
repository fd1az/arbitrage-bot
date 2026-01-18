# MEV Risk Analysis

This document explains MEV (Maximal Extractable Value) risks relevant to CEX-DEX arbitrage and how to mitigate them.

## What is MEV?

MEV refers to the profit that block producers (validators/miners) or searchers can extract by reordering, inserting, or censoring transactions within a block. In practice, sophisticated bots monitor the mempool and exploit predictable transactions.

## Risks for CEX-DEX Arbitrage

### 1. Sandwich Attacks

**How it works:**
1. MEV bot detects your pending swap in the mempool
2. Bot places a buy order *before* your transaction (frontrun)
3. Your transaction executes at a worse price
4. Bot places a sell order *after* your transaction (backrun)
5. Bot profits the price difference

**Impact:** You receive fewer tokens than expected. On large trades, losses can exceed the arbitrage profit.

**Example:**
```
Your tx: Swap 10 ETH → USDC at ~$3,200/ETH
MEV bot:
  1. Buy ETH (price moves to $3,210)
  2. Your tx executes at $3,210 (you lose $100)
  3. Sell ETH at $3,210 (bot profits)
```

### 2. Frontrunning

**How it works:**
1. MEV bot sees your arbitrage transaction
2. Bot copies your exact trade with higher gas
3. Bot's transaction confirms first, capturing the opportunity
4. Your transaction either fails or gets worse price

**Impact:** Opportunity disappears before your transaction confirms.

### 3. Backrunning

**How it works:**
1. Your transaction executes successfully
2. MEV bot executes immediately after to capture remaining arbitrage

**Impact:** Usually harmless to you - they capture leftover profit you weren't going to get anyway.

## Risk by Spread Size

| Spread (bps) | MEV Risk | Reasoning |
|--------------|----------|-----------|
| < 20 | Low | Too small for MEV bots to bother (gas costs exceed profit) |
| 20-50 | High | Sweet spot for MEV - profitable and common |
| 50-100 | Medium | Attractive but more competition, faster execution needed |
| > 100 | Variable | Rare opportunities - execute fast before others see |

## Risk by Trade Size

| Trade Size | MEV Risk | Reasoning |
|------------|----------|-----------|
| < 1 ETH | Low | Not worth the gas for sandwich |
| 1-10 ETH | Medium | Moderate target |
| 10-100 ETH | High | Attractive target for sandwich |
| > 100 ETH | Very High | Prime target, will almost certainly be attacked |

## Mitigation Strategies

### 1. Flashbots Protect (Recommended)

**What:** Free RPC endpoint that sends transactions directly to block builders, bypassing the public mempool.

**How to use:**
```bash
# Replace your RPC with Flashbots Protect
ETH_HTTP_URL=https://rpc.flashbots.net
```

**Pros:**
- Free
- No mempool exposure
- Failed transactions don't cost gas

**Cons:**
- Slightly slower inclusion
- Only works on Ethereum mainnet

**Link:** https://protect.flashbots.net

### 2. MEV Blocker

**What:** RPC that rebates some MEV back to users.

**How to use:**
```bash
ETH_HTTP_URL=https://rpc.mevblocker.io
```

**Link:** https://mevblocker.io

### 3. Private Transaction Services (Paid)

For high-value operations:

| Service | Description |
|---------|-------------|
| Bloxroute | Private transaction relay |
| Eden Network | Priority transaction inclusion |
| Flashbots Bundles | Atomic multi-transaction execution |

### 4. Slippage Protection

Set tight slippage tolerance to limit losses:

```go
// In swap execution (if implemented)
maxSlippage := 0.005 // 0.5%
minAmountOut := expectedAmount * (1 - maxSlippage)
```

**Trade-off:** Tighter slippage = more failed transactions but less loss when sandwiched.

### 5. Trade Splitting

Split large trades into smaller chunks:

```go
// Instead of one 100 ETH trade
// Execute 10 x 10 ETH trades across multiple blocks
```

**Trade-off:** More gas costs, but each trade is less attractive to MEV.

## MEV in This Bot's Context

This bot is **detection-only** - it doesn't execute trades. However, understanding MEV is crucial because:

1. **Opportunity Validity:** A detected opportunity may not be capturable due to MEV
2. **Profit Estimation:** Real profit is lower than calculated due to MEV losses
3. **Future Execution:** If you add execution, you must implement MEV protection

## Recommendations for Future Execution

If you plan to add trade execution:

1. **Always use Flashbots Protect** - no reason not to, it's free
2. **Start with small sizes** (< 5 ETH) to minimize MEV exposure
3. **Monitor sandwich attacks** using tools like [EigenPhi](https://eigenphi.io/)
4. **Consider Flashbots Bundles** for atomic execution of multi-step arb
5. **Never use public mempool** for arbitrage transactions

## Resources

- [Flashbots Docs](https://docs.flashbots.net/)
- [MEV Wiki](https://www.mev.wiki/)
- [Ethereum.org MEV](https://ethereum.org/en/developers/docs/mev/)
- [EigenPhi MEV Explorer](https://eigenphi.io/)
