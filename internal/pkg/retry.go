package pkg

import (
	"context"
	"math"
	"math/rand/v2"
	"time"
)

type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	JitterPct   float64 // 0.0–1.0; e.g. 0.3 means ±30%
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3, // 1 initial + 2 retries
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    2 * time.Second,
		JitterPct:   0.3,
	}
}

// Retry calls fn up to cfg.MaxAttempts times.
// Between each call it sleeps with exponential backoff and jitter.
func Retry[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error), shouldRetry func(error) bool) (T, error) {
	var lastErr error
	var zero T

	for attempt := range cfg.MaxAttempts {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err
		if !shouldRetry(err) {
			return zero, err
		}

		if attempt == cfg.MaxAttempts-1 {
			break
		}

		delay := backoffWithJitter(attempt, cfg)
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(delay):
		}
	}

	return zero, lastErr
}

func backoffWithJitter(attempt int, cfg RetryConfig) time.Duration {
	delay := float64(cfg.BaseDelay) * math.Pow(2, float64(attempt))
	if delay > float64(cfg.MaxDelay) {
		delay = float64(cfg.MaxDelay)
	}

	jitter := delay * cfg.JitterPct
	delay += jitter * (2*rand.Float64() - 1) // ±jitterPct

	return time.Duration(delay)
}
