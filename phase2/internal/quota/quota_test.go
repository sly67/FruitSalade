package quota

import (
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	rl := NewRateLimiter(nil)

	// 10 requests per minute
	rpm := 10
	userID := 1

	// Should allow up to 10 requests
	for i := 0; i < 10; i++ {
		if !rl.Allow(userID, rpm) {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}

	// 11th should be denied
	if rl.Allow(userID, rpm) {
		t.Error("11th request should be denied")
	}
}

func TestRateLimiterUnlimited(t *testing.T) {
	rl := NewRateLimiter(nil)

	// rpm=0 means unlimited
	for i := 0; i < 1000; i++ {
		if !rl.Allow(1, 0) {
			t.Fatalf("request %d should be allowed (unlimited)", i+1)
		}
	}
}

func TestRateLimiterRefill(t *testing.T) {
	rl := NewRateLimiter(nil)
	userID := 1
	rpm := 60 // 1 token per second

	// Exhaust all tokens
	for i := 0; i < 60; i++ {
		rl.Allow(userID, rpm)
	}

	if rl.Allow(userID, rpm) {
		t.Error("should be rate limited after exhausting tokens")
	}

	// Wait for refill
	time.Sleep(1100 * time.Millisecond)

	if !rl.Allow(userID, rpm) {
		t.Error("should be allowed after refill")
	}
}

func TestRateLimiterRetryAfter(t *testing.T) {
	rl := NewRateLimiter(nil)
	userID := 1
	rpm := 60

	// Exhaust tokens
	for i := 0; i < 60; i++ {
		rl.Allow(userID, rpm)
	}

	retryAfter := rl.RetryAfter(userID, rpm)
	if retryAfter < 1 {
		t.Errorf("expected retry-after >= 1, got %d", retryAfter)
	}
}

func TestRateLimiterMultipleUsers(t *testing.T) {
	rl := NewRateLimiter(nil)

	// User 1: 5 rpm
	for i := 0; i < 5; i++ {
		if !rl.Allow(1, 5) {
			t.Fatalf("user 1 request %d should be allowed", i+1)
		}
	}
	if rl.Allow(1, 5) {
		t.Error("user 1 should be rate limited")
	}

	// User 2 should still have tokens
	if !rl.Allow(2, 5) {
		t.Error("user 2 should not be affected by user 1's rate limit")
	}
}

func TestRateLimiterCleanup(t *testing.T) {
	rl := NewRateLimiter(nil)

	rl.Allow(1, 10)
	rl.Allow(2, 10)

	if len(rl.buckets) != 2 {
		t.Fatalf("expected 2 buckets, got %d", len(rl.buckets))
	}

	// Set bucket 1's lastRefill to the past
	rl.mu.Lock()
	rl.buckets[1].lastRefill = time.Now().Add(-2 * time.Hour)
	rl.mu.Unlock()

	rl.Cleanup(1 * time.Hour)

	rl.mu.Lock()
	count := len(rl.buckets)
	rl.mu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 bucket after cleanup, got %d", count)
	}
}
