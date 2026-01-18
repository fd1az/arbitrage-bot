# Memory Profiling with pprof

This document explains how to analyze memory usage in the arbitrage bot using Go's built-in pprof profiler.

## Quick Start

With the bot running, access pprof at `http://localhost:9090/debug/pprof/`

```bash
# Start the bot
make run

# In another terminal, analyze memory
go tool pprof -text http://localhost:9090/debug/pprof/heap
```

## Available Profiles

| Endpoint | Description |
|----------|-------------|
| `/debug/pprof/heap` | Current memory in use |
| `/debug/pprof/allocs` | All allocations since start (including freed) |
| `/debug/pprof/goroutine` | Stack traces of all goroutines |
| `/debug/pprof/profile` | CPU profile (30s by default) |
| `/debug/pprof/trace` | Execution trace |

## Memory Analysis Commands

### Heap (current memory usage)

```bash
# Text output - memory currently in use
go tool pprof -text -inuse_space http://localhost:9090/debug/pprof/heap

# By number of objects (not size)
go tool pprof -text -inuse_objects http://localhost:9090/debug/pprof/heap

# Interactive mode
go tool pprof http://localhost:9090/debug/pprof/heap
# Then use: top, list <func>, web, png, etc.

# Web UI (opens browser)
go tool pprof -http=:8080 http://localhost:9090/debug/pprof/heap
```

### Allocs (total allocations)

```bash
# Total bytes allocated
go tool pprof -text -alloc_space http://localhost:9090/debug/pprof/allocs

# Total objects allocated
go tool pprof -text -alloc_objects http://localhost:9090/debug/pprof/allocs
```

## Reading the Output

```
      flat  flat%   sum%        cum   cum%
   13.89MB 11.01% 11.01%    13.89MB 11.01%  bytes.growSlice
       9MB  7.13% 18.13%    31.89MB 25.26%  encoding/json.Marshal
```

| Column | Meaning |
|--------|---------|
| **flat** | Memory allocated directly by this function |
| **flat%** | Percentage of total |
| **sum%** | Cumulative percentage (sum of rows above) |
| **cum** | Memory allocated by this function + everything it calls |
| **cum%** | Cumulative percentage |

**Key insight**: High `cum` with low `flat` means the function itself doesn't allocate much, but it calls other functions that do.

## Baseline Analysis (2026-01-18)

### Heap Profile (memory in use)

Total: **~7.4 MB**

| Memory | Function | Notes |
|--------|----------|-------|
| 2.0 MB | `runtime.allocm` | Goroutine stacks (normal) |
| 900 KB | `compress/flate.NewWriter` | Gzip compression for WebSocket |
| 810 KB | `bytes.growSlice` | Dynamic buffer growth |
| 528 KB | `trace.NewBatchSpanProcessor` | OpenTelemetry tracing |
| 512 KB | `wsconn.readLoop` | WebSocket message buffers |

### Allocs Profile (total since start)

Total: **~126 MB** allocated over ~2 minutes of runtime

| Allocated | Function | Notes |
|-----------|----------|-------|
| 13.9 MB | `bytes.growSlice` | Slice resizing |
| 31.9 MB (cum) | `encoding/json.Marshal` | JSON serialization |
| 56.6 MB (cum) | `wsconn.readLoop` | WebSocket message processing |
| 24 MB (cum) | `binance.routeStreamEvent` | Binance message routing |
| 18.2 MB (cum) | `zipkin.SpanModels` | Trace export |

### Interpretation

1. **Memory usage is healthy** (~7.4 MB in use) - GC is working well
2. **Main allocation source**: WebSocket message processing (JSON parsing of Binance orderbook updates)
3. **Expected behavior**: High `alloc_space` with low `inuse_space` means objects are short-lived and properly garbage collected
4. **Tracing overhead**: ~18 MB from Zipkin exports - can be reduced by disabling telemetry in production if needed

## Useful pprof Commands (Interactive Mode)

```bash
go tool pprof http://localhost:9090/debug/pprof/heap
```

Inside pprof:

```
top           # Top memory consumers
top --cum     # Sort by cumulative memory
list <func>   # Show source code with allocation info
web           # Open graph in browser (requires graphviz)
png           # Generate PNG graph
tree          # Show call tree
```

## Comparing Profiles

Save profiles at different times and compare:

```bash
# Save baseline
curl -o baseline.prof http://localhost:9090/debug/pprof/heap

# ... run workload ...

# Save after workload
curl -o after.prof http://localhost:9090/debug/pprof/heap

# Compare (shows difference)
go tool pprof -base=baseline.prof after.prof
```

## Detecting Memory Leaks

1. Take heap profile at time T1
2. Wait (let app run under load)
3. Take heap profile at time T2
4. Compare: `go tool pprof -base=t1.prof t2.prof`

If `inuse_space` keeps growing without bounds, you have a leak.

## Resources

- [Go Blog: Profiling Go Programs](https://go.dev/blog/pprof)
- [runtime/pprof documentation](https://pkg.go.dev/runtime/pprof)
- [net/http/pprof documentation](https://pkg.go.dev/net/http/pprof)
