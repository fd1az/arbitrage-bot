// Package binance implements the CEXProvider interface for Binance exchange.
package binance

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
)

// WebSocket request/response messages

// WSRequest is a WebSocket subscription request.
type WSRequest struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
	ID     int64    `json:"id"`
}

// WSResponse is a WebSocket subscription response.
type WSResponse struct {
	Result json.RawMessage `json:"result"`
	ID     int64           `json:"id"`
}

// Stream event types
const (
	EventTypeAggTrade     = "aggTrade"
	EventTypeDepthUpdate  = "depthUpdate"
	EventTypeBookTicker   = "bookTicker"
)

// StreamEvent is the base wrapper for all stream messages.
type StreamEvent struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

// AggTradeEvent represents an aggregate trade event.
// Stream: <symbol>@aggTrade
type AggTradeEvent struct {
	EventType    string `json:"e"` // "aggTrade"
	EventTime    int64  `json:"E"` // Event time (ms)
	Symbol       string `json:"s"` // Symbol
	AggTradeID   int64  `json:"a"` // Aggregate trade ID
	Price        string `json:"p"` // Price
	Quantity     string `json:"q"` // Quantity
	FirstTradeID int64  `json:"f"` // First trade ID
	LastTradeID  int64  `json:"l"` // Last trade ID
	TradeTime    int64  `json:"T"` // Trade time (ms)
	IsBuyerMaker bool   `json:"m"` // Is buyer the maker?
}

// ParsePrice parses the price as decimal.
func (e *AggTradeEvent) ParsePrice() (decimal.Decimal, error) {
	return decimal.NewFromString(e.Price)
}

// ParseQuantity parses the quantity as decimal.
func (e *AggTradeEvent) ParseQuantity() (decimal.Decimal, error) {
	return decimal.NewFromString(e.Quantity)
}

// Timestamp returns the trade time as time.Time.
func (e *AggTradeEvent) Timestamp() time.Time {
	return time.UnixMilli(e.TradeTime)
}

// DepthUpdateEvent represents a diff depth update (not used with @depth20).
// Stream: <symbol>@depth@100ms or <symbol>@depth@1000ms
type DepthUpdateEvent struct {
	EventType     string     `json:"e"` // "depthUpdate"
	EventTime     int64      `json:"E"` // Event time (ms)
	Symbol        string     `json:"s"` // Symbol
	FirstUpdateID int64      `json:"U"` // First update ID
	FinalUpdateID int64      `json:"u"` // Final update ID
	Bids          [][]string `json:"b"` // Bids [price, qty]
	Asks          [][]string `json:"a"` // Asks [price, qty]
}

// PartialDepthEvent represents a partial book depth snapshot.
// Stream: <symbol>@depth5, @depth10, @depth20 (with optional @100ms/@1000ms speed)
// This is the format used by @depth20@100ms which sends top 20 levels.
// Note: Symbol is not in the JSON payload - it must be set from the stream name.
type PartialDepthEvent struct {
	LastUpdateID int64      `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"` // Top bids [[price, qty], ...]
	Asks         [][]string `json:"asks"` // Top asks [[price, qty], ...]
	Symbol       string     `json:"-"`    // Set from stream name, not in payload
}

// BookTickerEvent represents best bid/ask update (real-time).
// Stream: <symbol>@bookTicker
type BookTickerEvent struct {
	UpdateID int64  `json:"u"` // Order book updateId
	Symbol   string `json:"s"` // Symbol
	BidPrice string `json:"b"` // Best bid price
	BidQty   string `json:"B"` // Best bid qty
	AskPrice string `json:"a"` // Best ask price
	AskQty   string `json:"A"` // Best ask qty
}

// ParseBidPrice parses the best bid price.
func (e *BookTickerEvent) ParseBidPrice() (decimal.Decimal, error) {
	return decimal.NewFromString(e.BidPrice)
}

// ParseAskPrice parses the best ask price.
func (e *BookTickerEvent) ParseAskPrice() (decimal.Decimal, error) {
	return decimal.NewFromString(e.AskPrice)
}

// ParseBidQty parses the best bid quantity.
func (e *BookTickerEvent) ParseBidQty() (decimal.Decimal, error) {
	return decimal.NewFromString(e.BidQty)
}

// ParseAskQty parses the best ask quantity.
func (e *BookTickerEvent) ParseAskQty() (decimal.Decimal, error) {
	return decimal.NewFromString(e.AskQty)
}

// OrderbookLevel represents a price level in the orderbook.
type OrderbookLevel struct {
	Price    decimal.Decimal
	Quantity decimal.Decimal
}

// ParseOrderbookLevels parses raw orderbook levels from Binance format.
func ParseOrderbookLevels(raw [][]string) ([]OrderbookLevel, error) {
	levels := make([]OrderbookLevel, 0, len(raw))
	for _, r := range raw {
		if len(r) < 2 {
			continue
		}
		price, err := decimal.NewFromString(r[0])
		if err != nil {
			return nil, err
		}
		qty, err := decimal.NewFromString(r[1])
		if err != nil {
			return nil, err
		}
		// Skip zero quantity levels (removed from book)
		if qty.IsZero() {
			continue
		}
		levels = append(levels, OrderbookLevel{Price: price, Quantity: qty})
	}
	return levels, nil
}

// REST API responses (for initial orderbook snapshot)

// OrderbookSnapshot is the REST API response for orderbook.
type OrderbookSnapshot struct {
	LastUpdateID int64      `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"`
	Asks         [][]string `json:"asks"`
}

// Helper to build stream names

// AggTradeStream returns the aggTrade stream name for a symbol.
func AggTradeStream(symbol string) string {
	return lowercase(symbol) + "@aggTrade"
}

// DepthStream returns the partial book depth stream name for a symbol.
// Uses @depth20 which sends the top 20 bid/ask levels (not diff stream).
func DepthStream(symbol string, speedMs int) string {
	return lowercase(symbol) + "@depth20@" + strconv.Itoa(speedMs) + "ms"
}

// BookTickerStream returns the bookTicker stream name for a symbol.
func BookTickerStream(symbol string) string {
	return lowercase(symbol) + "@bookTicker"
}

func lowercase(s string) string {
	// Simple ASCII lowercase for symbols
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 32
		}
	}
	return string(b)
}
