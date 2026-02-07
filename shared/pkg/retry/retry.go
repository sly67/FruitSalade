// Package retry provides retry logic with exponential backoff.
package retry

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// Config holds retry configuration.
type Config struct {
	MaxAttempts int           // Maximum number of attempts (0 = infinite)
	InitialWait time.Duration // Initial wait time
	MaxWait     time.Duration // Maximum wait time
	Multiplier  float64       // Backoff multiplier
	Jitter      float64       // Jitter factor (0-1)
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxAttempts: 3,
		InitialWait: 100 * time.Millisecond,
		MaxWait:     10 * time.Second,
		Multiplier:  2.0,
		Jitter:      0.1,
	}
}

// RetryableError wraps an error that should be retried.
type RetryableError struct {
	Err error
}

func (e RetryableError) Error() string {
	return e.Err.Error()
}

func (e RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable returns true if the error should be retried.
func IsRetryable(err error) bool {
	var retryable RetryableError
	return errors.As(err, &retryable)
}

// Retryable wraps an error to mark it as retryable.
func Retryable(err error) error {
	if err == nil {
		return nil
	}
	return RetryableError{Err: err}
}

// Do executes fn with retries.
func Do(ctx context.Context, cfg Config, fn func() error) error {
	var lastErr error

	for attempt := 1; cfg.MaxAttempts == 0 || attempt <= cfg.MaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryable(err) {
			return err
		}

		// Check context
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Calculate wait time with exponential backoff
		wait := float64(cfg.InitialWait) * math.Pow(cfg.Multiplier, float64(attempt-1))
		if wait > float64(cfg.MaxWait) {
			wait = float64(cfg.MaxWait)
		}

		// Add jitter
		if cfg.Jitter > 0 {
			jitter := wait * cfg.Jitter * (rand.Float64()*2 - 1)
			wait += jitter
		}

		// Wait or context cancel
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(wait)):
			// Continue to next attempt
		}
	}

	return lastErr
}

// DoWithResult executes fn with retries and returns a result.
func DoWithResult[T any](ctx context.Context, cfg Config, fn func() (T, error)) (T, error) {
	var result T
	var lastErr error

	for attempt := 1; cfg.MaxAttempts == 0 || attempt <= cfg.MaxAttempts; attempt++ {
		r, err := fn()
		if err == nil {
			return r, nil
		}

		lastErr = err

		if !IsRetryable(err) {
			return result, err
		}

		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		wait := float64(cfg.InitialWait) * math.Pow(cfg.Multiplier, float64(attempt-1))
		if wait > float64(cfg.MaxWait) {
			wait = float64(cfg.MaxWait)
		}

		if cfg.Jitter > 0 {
			jitter := wait * cfg.Jitter * (rand.Float64()*2 - 1)
			wait += jitter
		}

		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(time.Duration(wait)):
		}
	}

	return result, lastErr
}
