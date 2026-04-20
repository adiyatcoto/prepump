// Package ratelimit provides token bucket rate limiting for API calls
package ratelimit

import (
	"context"
	"time"
)

// Limiter implements token bucket rate limiting
type Limiter struct {
	tokens   chan struct{}
	interval time.Duration
}

// New creates a rate limiter allowing `rate` requests per second
func New(rate int) *Limiter {
	l := &Limiter{
		tokens:   make(chan struct{}, rate),
		interval: time.Second / time.Duration(rate),
	}
	// Pre-fill bucket
	for i := 0; i < rate; i++ {
		l.tokens <- struct{}{}
	}
	// Refill tokens
	go func() {
		ticker := time.NewTicker(l.interval)
		defer ticker.Stop()
		for range ticker.C {
			select {
			case l.tokens <- struct{}{}:
			default:
			}
		}
	}()
	return l
}

// Wait blocks until a token is available
func (l *Limiter) Wait(ctx context.Context) error {
	select {
	case <-l.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Do executes fn after acquiring a token
func (l *Limiter) Do(ctx context.Context, fn func() error) error {
	if err := l.Wait(ctx); err != nil {
		return err
	}
	return fn()
}
