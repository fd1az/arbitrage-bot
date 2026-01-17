package asset

import (
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

// Registry is a thread-safe registry of known assets.
type Registry struct {
	byID     map[AssetID]*Asset
	bySymbol map[string][]*Asset // symbol -> assets (can have multiple on different chains)
	mu       sync.RWMutex
}

// NewRegistry creates a new empty asset registry.
func NewRegistry() *Registry {
	return &Registry{
		byID:     make(map[AssetID]*Asset),
		bySymbol: make(map[string][]*Asset),
	}
}

// Register adds an asset to the registry.
// Panics if an asset with the same ID is already registered.
func (r *Registry) Register(a *Asset) {
	if a == nil {
		panic("asset: cannot register nil asset")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	id := a.ID()
	if _, exists := r.byID[id]; exists {
		panic(fmt.Sprintf("asset: %s already registered", id))
	}

	r.byID[id] = a
	r.bySymbol[a.Symbol()] = append(r.bySymbol[a.Symbol()], a)
}

// Get retrieves an asset by its ID.
func (r *Registry) Get(id AssetID) (*Asset, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	a, ok := r.byID[id]
	return a, ok
}

// MustGet retrieves an asset by its ID, panics if not found.
func (r *Registry) MustGet(id AssetID) *Asset {
	a, ok := r.Get(id)
	if !ok {
		panic(fmt.Sprintf("asset: %s not found in registry", id))
	}
	return a
}

// GetBySymbol retrieves all assets with the given symbol.
// Returns nil if no assets found.
func (r *Registry) GetBySymbol(symbol string) []*Asset {
	r.mu.RLock()
	defer r.mu.RUnlock()

	assets := r.bySymbol[symbol]
	if len(assets) == 0 {
		return nil
	}

	// Return a copy to prevent mutation
	result := make([]*Asset, len(assets))
	copy(result, assets)
	return result
}

// GetBySymbolAndChain retrieves an asset by symbol and chain ID.
func (r *Registry) GetBySymbolAndChain(symbol string, chainID uint64) (*Asset, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	assets := r.bySymbol[symbol]
	for _, a := range assets {
		if a.ChainID() == chainID {
			return a, true
		}
	}
	return nil, false
}

// GetNative retrieves the native coin for a chain.
func (r *Registry) GetNative(chainID uint64) (*Asset, bool) {
	id := NewNativeAssetID(chainID)
	return r.Get(id)
}

// GetToken retrieves a token by chain and address.
func (r *Registry) GetToken(chainID uint64, address common.Address) (*Asset, bool) {
	id := NewTokenAssetID(chainID, address)
	return r.Get(id)
}

// All returns all registered assets.
func (r *Registry) All() []*Asset {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Asset, 0, len(r.byID))
	for _, a := range r.byID {
		result = append(result, a)
	}
	return result
}

// Count returns the number of registered assets.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byID)
}

// Has returns true if an asset with the given ID is registered.
func (r *Registry) Has(id AssetID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.byID[id]
	return ok
}
