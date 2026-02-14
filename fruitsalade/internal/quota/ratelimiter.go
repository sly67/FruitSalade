package quota

import (
	"sync"
	"time"
)

// RateLimiter implements per-user token bucket rate limiting.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[int]*tokenBucket
	store   *QuotaStore
}

type tokenBucket struct {
	tokens    float64
	maxTokens float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewRateLimiter creates a new per-user rate limiter.
func NewRateLimiter(store *QuotaStore) *RateLimiter {
	return &RateLimiter{
		buckets: make(map[int]*tokenBucket),
		store:   store,
	}
}

// Allow checks if a request from the given user should be allowed.
// Returns true if allowed, false if rate limited.
// rpm=0 means unlimited.
func (rl *RateLimiter) Allow(userID int, rpm int) bool {
	if rpm == 0 {
		return true // Unlimited
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, ok := rl.buckets[userID]
	if !ok {
		bucket = &tokenBucket{
			tokens:     float64(rpm),
			maxTokens:  float64(rpm),
			refillRate: float64(rpm) / 60.0,
			lastRefill: time.Now(),
		}
		rl.buckets[userID] = bucket
	}

	// Update bucket if rpm changed
	if bucket.maxTokens != float64(rpm) {
		bucket.maxTokens = float64(rpm)
		bucket.refillRate = float64(rpm) / 60.0
	}

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens += elapsed * bucket.refillRate
	if bucket.tokens > bucket.maxTokens {
		bucket.tokens = bucket.maxTokens
	}
	bucket.lastRefill = now

	// Check if we have a token
	if bucket.tokens < 1 {
		return false
	}

	bucket.tokens--
	return true
}

// RetryAfter returns the number of seconds until the next token is available.
func (rl *RateLimiter) RetryAfter(userID int, rpm int) int {
	if rpm == 0 {
		return 0
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, ok := rl.buckets[userID]
	if !ok {
		return 0
	}

	if bucket.tokens >= 1 {
		return 0
	}

	// Time until next token
	needed := 1.0 - bucket.tokens
	seconds := needed / bucket.refillRate
	return int(seconds) + 1
}

// Cleanup removes buckets for users that haven't been seen recently.
func (rl *RateLimiter) Cleanup(maxAge time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for userID, bucket := range rl.buckets {
		if bucket.lastRefill.Before(cutoff) {
			delete(rl.buckets, userID)
		}
	}
}
