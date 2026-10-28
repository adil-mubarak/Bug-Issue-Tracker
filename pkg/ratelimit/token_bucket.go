// Package ratelimit provides a token-bucket rate limiter.
package ratelimit

import (
	"sync"
	"time"
)

type Bucket struct {
	mu               sync.Mutex
	capacity, tokens float64
	rate             float64
	last             time.Time
}

func New(capacity int, refillPerSecond float64) *Bucket {
	if capacity < 1 || refillPerSecond <= 0 {
		panic("ratelimit: capacity and refill rate must be positive")
	}
	return &Bucket{capacity: float64(capacity), tokens: float64(capacity), rate: refillPerSecond, last: time.Now()}
}
func (b *Bucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.tokens += now.Sub(b.last).Seconds() * b.rate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
