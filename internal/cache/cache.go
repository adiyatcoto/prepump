// Package cache provides in-memory caching for OHLCV data
package cache

import (
	"sync"
	"time"

	"github.com/you/prepump/internal/deepcoin"
)

// Key represents a cache entry key
type Key struct {
	Base      string
	Bar       string
	HasFutures bool
}

// Entry holds cached candles
type Entry struct {
	Candles []deepcoin.Candle
	TS      time.Time
}

// Cache provides thread-safe candle caching
type Cache struct {
	mu      sync.RWMutex
	data    map[Key]Entry
	ttl     time.Duration
	max     int
}

// New creates a cache with specified TTL and max entries
func New(ttl time.Duration, max int) *Cache {
	return &Cache{
		data: make(map[Key]Entry),
		ttl:  ttl,
		max:  max,
	}
}

// Get retrieves cached candles if not expired
func (c *Cache) Get(k Key) ([]deepcoin.Candle, bool) {
	c.mu.RLock()
	entry, ok := c.data[k]
	c.mu.RUnlock()
	
	if !ok || time.Since(entry.TS) > c.ttl {
		return nil, false
	}
	return entry.Candles, true
}

// Set stores candles in cache
func (c *Cache) Set(k Key, candles []deepcoin.Candle) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Simple eviction policy: clear if too large
	if len(c.data) >= c.max {
		c.data = make(map[Key]Entry)
	}
	
	c.data[k] = Entry{
		Candles: candles,
		TS:      time.Now(),
	}
}

// Invalidate removes a specific entry
func (c *Cache) Invalidate(k Key) {
	c.mu.Lock()
	delete(c.data, k)
	c.mu.Unlock()
}

// Clear removes all entries
func (c *Cache) Clear() {
	c.mu.Lock()
	c.data = make(map[Key]Entry)
	c.mu.Unlock()
}

// Stats returns cache stats
func (c *Cache) Stats() (entries int, oldest time.Time) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entries = len(c.data)
	for _, e := range c.data {
		if oldest.IsZero() || e.TS.Before(oldest) {
			oldest = e.TS
		}
	}
	return
}
