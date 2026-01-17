package wsconn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// mockWSServer creates a test WebSocket server that echoes messages.
func mockWSServer(t *testing.T, handler func(conn *websocket.Conn)) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("websocket accept error: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		if handler != nil {
			handler(conn)
		}
	}))
}

// echoHandler echoes messages back to the client.
func echoHandler(conn *websocket.Conn) {
	ctx := context.Background()
	for {
		msgType, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if err := conn.Write(ctx, msgType, data); err != nil {
			return
		}
	}
}

func TestClient_Connect_Success(t *testing.T) {
	server := mockWSServer(t, func(conn *websocket.Conn) {
		// Keep connection open briefly
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	cfg := DefaultConfig(wsURL, "test")
	cfg.PingInterval = 0 // Disable ping for this test

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if client.State() != StateConnected {
		t.Errorf("expected state %v, got %v", StateConnected, client.State())
	}

	if !client.IsConnected() {
		t.Error("expected IsConnected() to return true")
	}
}

func TestClient_Connect_Failure(t *testing.T) {
	cfg := DefaultConfig("ws://localhost:59999", "test") // Invalid port
	cfg.PingInterval = 0

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = client.Connect(ctx)
	if err == nil {
		t.Fatal("expected Connect to fail with invalid URL")
	}

	if client.State() != StateDisconnected {
		t.Errorf("expected state %v, got %v", StateDisconnected, client.State())
	}
}

func TestClient_SendJSON(t *testing.T) {
	var received []byte
	var mu sync.Mutex

	server := mockWSServer(t, func(conn *websocket.Conn) {
		ctx := context.Background()
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		mu.Lock()
		received = data
		mu.Unlock()
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	cfg := DefaultConfig(wsURL, "test")
	cfg.PingInterval = 0

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Send a struct as JSON
	payload := map[string]interface{}{
		"method": "SUBSCRIBE",
		"params": []string{"btcusdt@trade"},
		"id":     1,
	}

	if err := client.SendJSON(ctx, payload); err != nil {
		t.Fatalf("SendJSON failed: %v", err)
	}

	// Wait for server to receive
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(received) == 0 {
		t.Fatal("server did not receive message")
	}

	// Verify it's valid JSON (not fmt.Sprintf output)
	var parsed map[string]interface{}
	if err := json.Unmarshal(received, &parsed); err != nil {
		t.Fatalf("received data is not valid JSON: %v\ndata: %s", err, string(received))
	}

	if parsed["method"] != "SUBSCRIBE" {
		t.Errorf("expected method=SUBSCRIBE, got %v", parsed["method"])
	}
}

func TestClient_MessageHandling(t *testing.T) {
	server := mockWSServer(t, echoHandler)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	cfg := DefaultConfig(wsURL, "test")
	cfg.PingInterval = 0

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	var receivedMsg []byte
	var msgMu sync.Mutex
	msgReceived := make(chan struct{})

	client.OnMessage(func(ctx context.Context, msg []byte) {
		msgMu.Lock()
		receivedMsg = msg
		msgMu.Unlock()
		close(msgReceived)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Send a message
	testMsg := []byte(`{"test":"message"}`)
	if err := client.Send(ctx, testMsg); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Wait for echo
	select {
	case <-msgReceived:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for message")
	}

	msgMu.Lock()
	defer msgMu.Unlock()

	if string(receivedMsg) != string(testMsg) {
		t.Errorf("expected %s, got %s", testMsg, receivedMsg)
	}
}

func TestClient_StateChangeHandler(t *testing.T) {
	server := mockWSServer(t, func(conn *websocket.Conn) {
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	cfg := DefaultConfig(wsURL, "test")
	cfg.PingInterval = 0

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	var states []State
	var statesMu sync.Mutex

	client.OnStateChange(func(state State, err error) {
		statesMu.Lock()
		states = append(states, state)
		statesMu.Unlock()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Give time for state changes
	time.Sleep(50 * time.Millisecond)

	statesMu.Lock()
	defer statesMu.Unlock()

	// Should have: Connecting, Connected
	if len(states) < 2 {
		t.Fatalf("expected at least 2 state changes, got %d: %v", len(states), states)
	}

	if states[0] != StateConnecting {
		t.Errorf("expected first state to be Connecting, got %v", states[0])
	}

	if states[1] != StateConnected {
		t.Errorf("expected second state to be Connected, got %v", states[1])
	}
}

func TestClient_GracefulClose(t *testing.T) {
	server := mockWSServer(t, func(conn *websocket.Conn) {
		// Keep reading until closed
		ctx := context.Background()
		for {
			_, _, err := conn.Read(ctx)
			if err != nil {
				return
			}
		}
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	cfg := DefaultConfig(wsURL, "test")
	cfg.PingInterval = 0

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Close should not error
	if err := client.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	if client.State() != StateClosed {
		t.Errorf("expected state %v, got %v", StateClosed, client.State())
	}

	// Second close should be idempotent
	if err := client.Close(); err != nil {
		t.Errorf("second Close should not error: %v", err)
	}
}

func TestClient_ConcurrentSend(t *testing.T) {
	var msgCount atomic.Int32

	server := mockWSServer(t, func(conn *websocket.Conn) {
		ctx := context.Background()
		for {
			_, _, err := conn.Read(ctx)
			if err != nil {
				return
			}
			msgCount.Add(1)
		}
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	cfg := DefaultConfig(wsURL, "test")
	cfg.PingInterval = 0

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Send messages concurrently
	const numGoroutines = 10
	const msgsPerGoroutine = 5
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < msgsPerGoroutine; j++ {
				msg := map[string]int{"goroutine": id, "msg": j}
				if err := client.SendJSON(ctx, msg); err != nil {
					t.Errorf("SendJSON failed: %v", err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(200 * time.Millisecond) // Let server process

	expected := int32(numGoroutines * msgsPerGoroutine)
	if got := msgCount.Load(); got != expected {
		t.Errorf("expected %d messages, server received %d", expected, got)
	}
}

func TestClient_MaxMessageSize(t *testing.T) {
	server := mockWSServer(t, func(conn *websocket.Conn) {
		ctx := context.Background()
		// Send a large message
		largeMsg := make([]byte, 1024*1024) // 1MB
		for i := range largeMsg {
			largeMsg[i] = 'A'
		}
		conn.Write(ctx, websocket.MessageText, largeMsg)
		time.Sleep(100 * time.Millisecond)
	})
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	cfg := DefaultConfig(wsURL, "test")
	cfg.PingInterval = 0
	cfg.MaxMessageSize = 100 // Very small limit

	client, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Wait for the large message to trigger disconnect
	time.Sleep(300 * time.Millisecond)

	// Client should have disconnected or be reconnecting
	state := client.State()
	if state == StateConnected {
		t.Error("expected client to disconnect after receiving oversized message")
	}
}
