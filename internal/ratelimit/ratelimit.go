// Package ratelimit provides a wrapper around golang.org/x/time/rate.
package ratelimit

import (
	"context"
	"time"

	"golang.org/x/time/rate"
)

// Limiter wraps rate.Limiter with convenience methods.
type Limiter struct {
	limiter *rate.Limiter
}

// New creates a new rate limiter.
// requestsPerMinute specifies how many requests are allowed per minute.
func New(requestsPerMinute int) *Limiter {
	// Convert requests per minute to rate per second
	rps := float64(requestsPerMinute) / 60.0
	burst := requestsPerMinute / 10 // Allow burst of 10% of rate limit
	if burst < 1 {
		burst = 1
	}

	return &Limiter{
		limiter: rate.NewLimiter(rate.Limit(rps), burst),
	}
}

// NewWithBurst creates a new rate limiter with explicit burst.
func NewWithBurst(requestsPerSecond float64, burst int) *Limiter {
	return &Limiter{
		limiter: rate.NewLimiter(rate.Limit(requestsPerSecond), burst),
	}
}

// Wait blocks until a token is available or the context is cancelled.
func (l *Limiter) Wait(ctx context.Context) error {
	return l.limiter.Wait(ctx)
}

// Allow reports whether an event may happen now.
func (l *Limiter) Allow() bool {
	return l.limiter.Allow()
}

// Reserve returns a Reservation that indicates how long the caller must wait.
func (l *Limiter) Reserve() *rate.Reservation {
	return l.limiter.Reserve()
}

// Tokens returns the current number of available tokens.
func (l *Limiter) Tokens() float64 {
	return l.limiter.Tokens()
}

// WaitN blocks until n tokens are available.
func (l *Limiter) WaitN(ctx context.Context, n int) error {
	return l.limiter.WaitN(ctx, n)
}

// SetLimit updates the rate limit.
func (l *Limiter) SetLimit(requestsPerMinute int) {
	rps := float64(requestsPerMinute) / 60.0
	l.limiter.SetLimit(rate.Limit(rps))
}

// SetBurst updates the burst limit.
func (l *Limiter) SetBurst(burst int) {
	l.limiter.SetBurst(burst)
}

// WaitWithTimeout is a convenience method that waits with a timeout.
func (l *Limiter) WaitWithTimeout(timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return l.limiter.Wait(ctx)
}
