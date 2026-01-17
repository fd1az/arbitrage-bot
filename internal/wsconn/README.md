# wsconn - Instrumented WebSocket Client

Production-grade WebSocket client with automatic reconnection, exponential backoff, and full OpenTelemetry instrumentation.

## Features

- **Automatic Reconnection** - Exponential backoff with jitter
- **OTEL Tracing** - Spans for connect, reconnect, send, receive, close
- **OTEL Metrics** - Connection state, message counts, latency, bytes transferred
- **Thread-Safe** - Safe for concurrent use
- **Context Support** - Full context propagation for tracing
- **Configurable Timeouts** - Read/write timeouts, ping intervals

## Usage

### Basic Usage

```go
import "github.com/fd1az/arbitrage-bot/internal/wsconn"

// Create client with default config
config := wsconn.DefaultConfig("wss://example.com/ws", "my-service")
client, err := wsconn.New(config)
if err != nil {
    log.Fatal(err)
}
defer client.Close()

// Connect with automatic retry
ctx := context.Background()
if err := client.ConnectWithRetry(ctx); err != nil {
    log.Fatal(err)
}

// Send message
if err := client.Send(ctx, []byte(`{"action":"subscribe"}`)); err != nil {
    log.Error(err)
}

// Receive messages
for msg := range client.Messages() {
    fmt.Println("Received:", string(msg))
}
```

### With Message Handler

```go
client, _ := wsconn.New(config)

// Set message handler (called for each message)
client.OnMessage(func(ctx context.Context, msg []byte) {
    log.Info("received message", "size", len(msg))
    // Process message...
})

// Set state change handler
client.OnStateChange(func(state wsconn.State, err error) {
    log.Info("connection state changed", "state", state, "error", err)
})

client.ConnectWithRetry(ctx)
```

### Custom Configuration

```go
config := wsconn.Config{
    URL:            "wss://stream.binance.com:9443/ws",
    Name:           "binance-orderbook",  // Used in metrics/traces
    InitialBackoff: 1 * time.Second,
    MaxBackoff:     30 * time.Second,
    MaxReconnects:  0,                    // 0 = infinite
    PingInterval:   30 * time.Second,
    ReadTimeout:    60 * time.Second,
    WriteTimeout:   10 * time.Second,
    BufferSize:     256,                  // Message channel buffer
}
```

## Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `URL` | required | WebSocket endpoint URL |
| `Name` | required | Identifier for metrics/tracing |
| `InitialBackoff` | 1s | Initial retry delay |
| `MaxBackoff` | 30s | Maximum retry delay |
| `MaxReconnects` | 0 | Max reconnection attempts (0 = infinite) |
| `PingInterval` | 30s | Keep-alive ping interval |
| `ReadTimeout` | 60s | Read operation timeout |
| `WriteTimeout` | 10s | Write operation timeout |
| `BufferSize` | 256 | Message channel buffer size |

## Connection States

```go
const (
    StateDisconnected State = "disconnected"
    StateConnecting   State = "connecting"
    StateConnected    State = "connected"
    StateReconnecting State = "reconnecting"
    StateClosed       State = "closed"
)
```

## OTEL Instrumentation

### Traces (Spans)

| Span Name | Description |
|-----------|-------------|
| `ws.connect` | Connection attempt |
| `ws.connect_with_retry` | Connection with retry loop |
| `ws.reconnect` | Reconnection attempt |
| `ws.message.send` | Message sent |
| `ws.message.recv` | Message received |
| `ws.disconnect` | Disconnection event |
| `ws.close` | Client close |

### Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `ws_connection_state` | Gauge | Current state (0-4) |
| `ws_messages_received_total` | Counter | Messages received |
| `ws_messages_sent_total` | Counter | Messages sent |
| `ws_bytes_received_total` | Counter | Bytes received |
| `ws_bytes_sent_total` | Counter | Bytes sent |
| `ws_reconnects_total` | Counter | Reconnection attempts |
| `ws_message_latency_ms` | Histogram | Message processing latency |

All metrics are tagged with `ws.name` attribute.

## Reconnection Strategy

Uses exponential backoff with jitter:

```
delay = min(initialBackoff * 2^attempt, maxBackoff) + random(0, delay/2)
```

Example progression (with 1s initial, 30s max):
- Attempt 1: ~1-1.5s
- Attempt 2: ~2-3s
- Attempt 3: ~4-6s
- Attempt 4: ~8-12s
- Attempt 5: ~16-24s
- Attempt 6+: ~30-45s (capped)

## Thread Safety

The client is safe for concurrent use:
- `Send()` can be called from multiple goroutines
- `Messages()` channel can be consumed by one goroutine
- State changes are atomic
- Connection management is mutex-protected

## Dependencies

- `github.com/coder/websocket` - WebSocket implementation
- `go.opentelemetry.io/otel` - Tracing and metrics
