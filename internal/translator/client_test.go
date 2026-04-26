package translator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aminsalami/funpokedex/internal/pkg"
)

func fastRetryConfig() pkg.RetryConfig {
	return pkg.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    5 * time.Millisecond,
		JitterPct:   0,
	}
}

// shouldRetry retries non-transient 400 errors

func TestTranslate_400_RetriedThreeTimes(t *testing.T) {
	var reqCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"code":400,"message":"bad text"}}`))
	}))
	defer srv.Close()

	tr := &FunTranslator{
		name:       "yoda",
		httpClient: &http.Client{Timeout: 2 * time.Second},
		endpoint:   srv.URL,
		retryCfg:   fastRetryConfig(),
	}

	_, err := tr.Translate(context.Background(), "some text")
	if err == nil {
		t.Fatal("expected error for 400 response")
	}

	if got := int(reqCount.Load()); got != 3 {
		t.Errorf("expected 3 requests (all retried due to always-true predicate), got %d", got)
	}
}

// 200 with empty translated text

func TestTranslate_EmptyTranslatedText_ReturnsError(t *testing.T) {
	var reqCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"contents":{"translated":"","text":"hello","translation":"yoda"}}`))
	}))
	defer srv.Close()

	tr := &FunTranslator{
		name:       "yoda",
		httpClient: &http.Client{Timeout: 2 * time.Second},
		endpoint:   srv.URL,
		retryCfg:   fastRetryConfig(),
	}

	_, err := tr.Translate(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for empty translation")
	}

	// Because shouldRetry is always-true, this also retries 3 times
	// against a server that will never return a non-empty translation.
	if got := int(reqCount.Load()); got != 3 {
		t.Errorf("expected 3 requests (empty translation retried), got %d", got)
	}
}

// Translation API unreachable (connection refused)

func TestTranslate_ConnectionRefused_FailsAfterRetries(t *testing.T) {
	// Start and immediately close server to get a port that refuses connections.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	closedURL := srv.URL
	srv.Close()

	tr := &FunTranslator{
		name:       "shakespeare",
		httpClient: &http.Client{Timeout: 1 * time.Second},
		endpoint:   closedURL,
		retryCfg:   fastRetryConfig(),
	}

	start := time.Now()
	_, err := tr.Translate(context.Background(), "To be or not to be")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for connection refused")
	}

	// Should complete quickly (retries with ~1ms backoff), not hang.
	if elapsed > 3*time.Second {
		t.Errorf("expected fast failure, took %v", elapsed)
	}
}
