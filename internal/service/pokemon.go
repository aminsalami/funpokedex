package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/sync/singleflight"

	"github.com/aminsalami/funpokedex/internal/domain"
)

type PokemonFetcher interface {
	FetchSpecies(ctx context.Context, name string) (domain.Pokemon, error)
}

type Cache interface {
	Get(key string) (any, bool)
	Set(key string, value any)
}

type PokemonService struct {
	fetcher     PokemonFetcher
	translators map[string]domain.Translator
	cache       Cache
	sfSpecies   singleflight.Group
	sfTransl    singleflight.Group
}

func NewPokemonService(fetcher PokemonFetcher, cache Cache) *PokemonService {
	return &PokemonService{
		fetcher:     fetcher,
		translators: make(map[string]domain.Translator),
		cache:       cache,
	}
}

func (s *PokemonService) Register(t domain.Translator) {
	s.translators[t.Name()] = t
}

func (s *PokemonService) GetPokemon(ctx context.Context, name string) (domain.Pokemon, error) {
	name = normalizeName(name)
	if name == "" {
		return domain.Pokemon{}, domain.ErrBadRequest("pokemon name is required")
	}

	return s.fetchSpecies(ctx, name)
}

func (s *PokemonService) GetTranslatedPokemon(ctx context.Context, name string) (domain.Pokemon, error) {
	name = normalizeName(name)
	if name == "" {
		return domain.Pokemon{}, domain.ErrBadRequest("pokemon name is required")
	}

	cacheKey := "translated:" + name
	if cached, ok := s.cache.Get(cacheKey); ok {
		return cached.(domain.Pokemon), nil
	}

	poke, err := s.fetchSpecies(ctx, name)
	if err != nil {
		return domain.Pokemon{}, err
	}

	t, err := s.chooseTranslator(poke)
	if err != nil {
		slog.Warn("translation failed, no matching translator", "pokemon", name, "error", err)
		return poke, err
	}

	translated, err := s.translateDescription(ctx, name, poke.Description, t)
	if err != nil {
		slog.Warn("translation failed, using standard description",
			"pokemon", name,
			"translator", t.Name(),
			"error", err,
		)
	} else {
		poke.Description = translated
	}

	s.cache.Set(cacheKey, poke)
	return poke, nil
}

func (s *PokemonService) fetchSpecies(ctx context.Context, name string) (domain.Pokemon, error) {
	cacheKey := "species:" + name
	if cached, ok := s.cache.Get(cacheKey); ok {
		return cached.(domain.Pokemon), nil
	}

	result, err, _ := s.sfSpecies.Do(cacheKey, func() (any, error) {
		poke, err := s.fetcher.FetchSpecies(ctx, name)
		if err != nil {
			return nil, err
		}
		poke.Description = domain.CleanFlavorText(poke.Description)
		s.cache.Set(cacheKey, poke)
		return poke, nil
	})
	if err != nil {
		return domain.Pokemon{}, err
	}

	return result.(domain.Pokemon), nil
}

func (s *PokemonService) chooseTranslator(p domain.Pokemon) (domain.Translator, error) {
	name := domain.TranslatorShakespeare
	if p.Habitat == "cave" || p.IsLegendary {
		name = domain.TranslatorYoda
	}

	t, ok := s.translators[name]
	if !ok {
		return nil, fmt.Errorf("unknown translator: %s", name)
	}
	return t, nil
}

func (s *PokemonService) translateDescription(ctx context.Context, pokemonName, text string, t domain.Translator) (string, error) {
	sfKey := fmt.Sprintf("translate:%s:%s", pokemonName, t.Name())
	result, err, _ := s.sfTransl.Do(sfKey, func() (any, error) {
		return t.Translate(ctx, text)
	})
	if err != nil {
		return "", err
	}

	return result.(string), nil
}

func normalizeName(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}
