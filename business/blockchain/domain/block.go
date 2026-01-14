// Package domain contains the core domain types for the blockchain context.
package domain

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// Block represents an Ethereum block header.
type Block struct {
	Number     uint64
	Hash       common.Hash
	ParentHash common.Hash
	Timestamp  time.Time
	GasLimit   uint64
	GasUsed    uint64
	BaseFee    *big.Int
}

// ConnectionState represents the state of a blockchain connection.
type ConnectionState string

const (
	StateDisconnected ConnectionState = "disconnected"
	StateConnecting   ConnectionState = "connecting"
	StateConnected    ConnectionState = "connected"
	StateReconnecting ConnectionState = "reconnecting"
)

// ConnectionStatus contains detailed connection information.
type ConnectionStatus struct {
	State       ConnectionState
	Latency     time.Duration
	LastBlock   uint64
	LastUpdate  time.Time
	Reconnects  int
	UsingHTTP   bool // true if using HTTP fallback
}
