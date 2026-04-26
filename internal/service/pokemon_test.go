package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aminsalami/funpokedex/internal/domain"
)

type stubFetcher struct {
	mu       sync.Mutex
	calls    int
	fn       func(ctx context.Context, name string) (domain.Pokemon, error)
	blocking chan struct{} // closed by caller to unblock FetchSpecies
}

func (f *stubFetcher) FetchSpecies(ctx context.Context, name string) (domain.Pokemon, error) {
	f.mu.Lock()
	f.calls++
	f.mu.Unlock()

	if f.blocking != nil {
		select {
		case <-f.blocking:
		case <-ctx.Done():
			return domain.Pokemon{}, ctx.Err()
		}
	}
	return f.fn(ctx, name)
}

type spyCache struct {
	mu   sync.Mutex
	data map[string]any
}

func newSpyCache() *spyCache {
	return &spyCache{data: make(map[string]any)}
}
func (c *spyCache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.data[key]
	return v, ok
}
func (c *spyCache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}
func (c *spyCache) has(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.data[key]
	return ok
}

type stubTranslator struct {
	name string
	fn   func(ctx context.Context, text string) (string, error)
}

func (t *stubTranslator) Name() string { return t.name }
func (t *stubTranslator) Translate(ctx context.Context, text string) (string, error) {
	return t.fn(ctx, text)
}

var mewtwo = domain.Pokemon{
	Name:        "mewtwo",
	Description: "It was created by a scientist.",
	Habitat:     "rare",
	IsLegendary: true,
}

var zubat = domain.Pokemon{
	Name:        "zubat",
	Description: "Forms colonies in dark places.",
	Habitat:     "cave",
	IsLegendary: false,
}

var pikachu = domain.Pokemon{
	Name:        "pikachu",
	Description: "When several of these Pokemon gather...",
	Habitat:     "forest",
	IsLegendary: false,
}

// Translation fallback must NOT cache the untranslated description ---

func TestGetTranslatedPokemon_TranslationFailure_DoesNotCache(t *testing.T) {
	fetcher := &stubFetcher{fn: func(_ context.Context, _ string) (domain.Pokemon, error) {
		return mewtwo, nil
	}}
	cache := newSpyCache()

	svc := NewPokemonService(fetcher, cache)
	svc.Register(&stubTranslator{
		name: domain.TranslatorYoda,
		fn: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("translation API down")
		},
	})
	svc.Register(&stubTranslator{name: domain.TranslatorShakespeare})

	poke, err := svc.GetTranslatedPokemon(context.Background(), "mewtwo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The service falls back to the original description.
	if poke.Description != mewtwo.Description {
		t.Errorf("expected original description %q, got %q", mewtwo.Description, poke.Description)
	}

	// A failed translation must not be cached — a recovered API must be re-tried.
	if cache.has("translated:mewtwo") {
		t.Fatal("untranslated fallback should not be cached under 'translated:mewtwo'")
	}
}

// chooseTranslator failure returns non-AppError (leaks 500)

func TestGetTranslatedPokemon_NoTranslatorsRegistered_ReturnsPlainError(t *testing.T) {
	fetcher := &stubFetcher{fn: func(_ context.Context, _ string) (domain.Pokemon, error) {
		return pikachu, nil
	}}
	svc := NewPokemonService(fetcher, newSpyCache())
	// Deliberately register NO translators.

	_, err := svc.GetTranslatedPokemon(context.Background(), "pikachu")
	if err == nil {
		t.Fatal("expected error when no translator is registered")
	}

	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		t.Errorf("error should NOT be *AppError (it's a plain fmt.Errorf), but got: %v", appErr)
	}
}

// Singleflight context sharing ---

func TestFetchSpecies_SingleflightSharesFirstRequestContext(t *testing.T) {
	unblock := make(chan struct{})
	fetcher := &stubFetcher{
		blocking: unblock,
		fn: func(ctx context.Context, _ string) (domain.Pokemon, error) {
			return mewtwo, nil
		},
	}
	svc := NewPokemonService(fetcher, newSpyCache())

	ctxA, cancelA := context.WithCancel(context.Background())
	ctxB := context.Background()

	var wg sync.WaitGroup
	var errA, errB error

	// Request A starts first and owns the singleflight slot.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, errA = svc.GetPokemon(ctxA, "mewtwo")
	}()

	// Give A a moment to register with singleflight.
	time.Sleep(30 * time.Millisecond)

	// Request B joins the same singleflight group.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, errB = svc.GetPokemon(ctxB, "mewtwo")
	}()

	time.Sleep(30 * time.Millisecond)

	// Cancel A's context — the inflight fetch will fail for both.
	cancelA()
	close(unblock)

	wg.Wait()

	if errA == nil {
		t.Error("expected errA to be non-nil after context cancellation")
	}
	// B's own context is alive, but it shares A's singleflight result.
	if errB == nil {
		t.Error("expected errB to be non-nil — singleflight shares first caller's context")
	}
}

// Translator selection logic

func TestChooseTranslator_SelectionMatrix(t *testing.T) {
	yoda := &stubTranslator{name: domain.TranslatorYoda}
	shakespeare := &stubTranslator{name: domain.TranslatorShakespeare}

	svc := NewPokemonService(nil, newSpyCache())
	svc.Register(yoda)
	svc.Register(shakespeare)

	tests := []struct {
		desc     string
		pokemon  domain.Pokemon
		wantName string
	}{
		{
			desc:     "cave habitat selects yoda",
			pokemon:  zubat,
			wantName: domain.TranslatorYoda,
		},
		{
			desc:     "legendary non-cave selects yoda",
			pokemon:  mewtwo,
			wantName: domain.TranslatorYoda,
		},
		{
			desc:     "non-legendary non-cave selects shakespeare",
			pokemon:  pikachu,
			wantName: domain.TranslatorShakespeare,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := svc.chooseTranslator(tt.pokemon)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Name() != tt.wantName {
				t.Errorf("expected translator %q, got %q", tt.wantName, got.Name())
			}
		})
	}
}
