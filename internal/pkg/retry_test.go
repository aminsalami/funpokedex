package pkg

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

var fastCfg = RetryConfig{
	MaxAttempts: 5,
	BaseDelay:   50 * time.Millisecond,
	MaxDelay:    200 * time.Millisecond,
	JitterPct:   0,
}

// --- 4a: Context cancelled during backoff returns ctx.Err() ---

func TestRetry_ContextCancelledDuringBackoff(t *testing.T) {
	alwaysFail := errors.New("transient")
	var attempts atomic.Int32

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// Cancel after enough time for the first attempt + start of backoff,
		// but before the second attempt's backoff completes.
		time.Sleep(80 * time.Millisecond)
		cancel()
	}()

	_, err := Retry(ctx, fastCfg, func() (string, error) {
		attempts.Add(1)
		return "", alwaysFail
	}, func(_ error) bool { return true })

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}

	if got := int(attempts.Load()); got >= fastCfg.MaxAttempts {
		t.Errorf("should NOT have exhausted all %d attempts, ran %d", fastCfg.MaxAttempts, got)
	}
}

// --- 4b: Succeeds on second attempt, no extra calls ---

func TestRetry_SucceedsOnSecondAttempt(t *testing.T) {
	var attempts atomic.Int32

	result, err := Retry(context.Background(), fastCfg, func() (string, error) {
		n := attempts.Add(1)
		if n == 1 {
			return "", errors.New("first try fails")
		}
		return "ok", nil
	}, func(_ error) bool { return true })

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
	if got := int(attempts.Load()); got != 2 {
		t.Errorf("expected exactly 2 attempts, got %d", got)
	}
}
