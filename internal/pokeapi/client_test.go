package pokeapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aminsalami/funpokedex/internal/domain"
	"github.com/aminsalami/funpokedex/internal/pkg"
)

// 400 from PokeAPI should not be retried — client errors are not transient.

func TestFetchSpecies_400_IsNotRetried(t *testing.T) {
	var reqCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"detail":"bad request"}`))
	}))
	defer srv.Close()

	client := NewClient(&http.Client{Timeout: 5 * time.Second}, srv.URL)
	client.retryCfg = pkg.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    5 * time.Millisecond,
		JitterPct:   0,
	}

	_, err := client.FetchSpecies(context.Background(), "pikachu")
	if err == nil {
		t.Fatal("expected error for 400 response")
	}

	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != "BAD_REQUEST" {
		t.Fatalf("expected BAD_REQUEST, got: %v", err)
	}

	got := int(reqCount.Load())
	if got != 1 {
		t.Errorf("expected 1 request (no retries for 4xx), got %d", got)
	}
}

// Upstream sends 200 headers then stalls body

func TestFetchSpecies_SlowBodyRead_TimesOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Flush headers, then stall — never write the body.
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(5 * time.Second)
	}))
	defer srv.Close()

	httpClient := &http.Client{Timeout: 100 * time.Millisecond}
	client := NewClient(httpClient, srv.URL)
	client.retryCfg = pkg.RetryConfig{
		MaxAttempts: 1,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    1 * time.Millisecond,
	}

	_, err := client.FetchSpecies(context.Background(), "slowpoke")
	if err == nil {
		t.Fatal("expected timeout error")
	}

	var appErr *domain.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *AppError, got: %T: %v", err, err)
	}
	if appErr.Code != "UPSTREAM_ERROR" {
		t.Errorf("expected UPSTREAM_ERROR code, got %q", appErr.Code)
	}
}

// --- 2c: Malformed/partial JSON from PokeAPI ---

func TestFetchSpecies_MalformedJSON(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantErrMsg string
	}{
		{
			name:       "truncated JSON",
			body:       `{`,
			wantErrMsg: "UPSTREAM_ERROR",
		},
		{
			name: "valid JSON missing all fields produces zero-value pokemon",
			body: `{}`,
		},
		{
			name: "flavor text only in Japanese produces empty description",
			body: `{"name":"pikachu","flavor_text_entries":[{"flavor_text":"ピカチュウ","language":{"name":"ja"}}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			client := NewClient(&http.Client{Timeout: 2 * time.Second}, srv.URL)
			client.retryCfg = pkg.RetryConfig{MaxAttempts: 1}

			poke, err := client.FetchSpecies(context.Background(), "test")

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatal("expected error")
				}
				var appErr *domain.AppError
				if !errors.As(err, &appErr) || appErr.Code != tt.wantErrMsg {
					t.Errorf("expected code %q, got: %v", tt.wantErrMsg, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if poke.Description != "" {
				t.Errorf("expected empty description, got %q", poke.Description)
			}
		})
	}
}

// --- 2d: 404 is NOT retried ---

func TestFetchSpecies_404_NotRetried(t *testing.T) {
	var reqCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqCount.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := NewClient(&http.Client{Timeout: 2 * time.Second}, srv.URL)
	client.retryCfg = pkg.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    5 * time.Millisecond,
	}

	_, err := client.FetchSpecies(context.Background(), "doesnotexist")
	if err == nil {
		t.Fatal("expected error for 404")
	}

	var appErr *domain.AppError
	if !errors.As(err, &appErr) || appErr.Code != "POKEMON_NOT_FOUND" {
		t.Fatalf("expected POKEMON_NOT_FOUND, got: %v", err)
	}

	if got := int(reqCount.Load()); got != 1 {
		t.Errorf("expected exactly 1 request (no retries), got %d", got)
	}
}
