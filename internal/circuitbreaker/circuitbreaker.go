// Package circuitbreaker provides a wrapper around sony/gobreaker.
package circuitbreaker

import (
	"time"

	"github.com/sony/gobreaker/v2"
)

// Config holds circuit breaker configuration.
type Config struct {
	Name          string
	MaxRequests   uint32        // max requests in half-open state
	Interval      time.Duration // cyclic period of closed state
	Timeout       time.Duration // period of open state
	ReadyToTrip   func(counts gobreaker.Counts) bool
	OnStateChange func(name string, from, to gobreaker.State)
}

// DefaultConfig returns a sensible default configuration.
func DefaultConfig(name string) Config {
	return Config{
		Name:        name,
		MaxRequests: 3,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= 5 && failureRatio >= 0.5
		},
	}
}

// CircuitBreaker wraps gobreaker.CircuitBreaker.
type CircuitBreaker[T any] struct {
	cb *gobreaker.CircuitBreaker[T]
}

// New creates a new CircuitBreaker with the given configuration.
func New[T any](cfg Config) *CircuitBreaker[T] {
	settings := gobreaker.Settings{
		Name:          cfg.Name,
		MaxRequests:   cfg.MaxRequests,
		Interval:      cfg.Interval,
		Timeout:       cfg.Timeout,
		ReadyToTrip:   cfg.ReadyToTrip,
		OnStateChange: cfg.OnStateChange,
	}

	return &CircuitBreaker[T]{
		cb: gobreaker.NewCircuitBreaker[T](settings),
	}
}

// Execute runs the given function through the circuit breaker.
func (c *CircuitBreaker[T]) Execute(fn func() (T, error)) (T, error) {
	return c.cb.Execute(fn)
}

// State returns the current state of the circuit breaker.
func (c *CircuitBreaker[T]) State() gobreaker.State {
	return c.cb.State()
}

// Counts returns the internal counts of the circuit breaker.
func (c *CircuitBreaker[T]) Counts() gobreaker.Counts {
	return c.cb.Counts()
}
