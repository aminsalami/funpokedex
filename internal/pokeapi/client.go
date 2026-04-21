package pokeapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/aminsalami/funpokedex/internal/domain"
	"github.com/aminsalami/funpokedex/internal/pkg"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	retryCfg   pkg.RetryConfig
}

func NewClient(httpClient *http.Client, baseURL string) *Client {
	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
		retryCfg:   pkg.DefaultRetryConfig(),
	}
}

func (c *Client) FetchSpecies(ctx context.Context, name string) (domain.Pokemon, error) {
	return pkg.Retry(ctx, c.retryCfg, func() (domain.Pokemon, error) {
		return c.fetchSpeciesOnce(ctx, name)
	}, isRetryable)
}

func (c *Client) fetchSpeciesOnce(ctx context.Context, name string) (domain.Pokemon, error) {
	url := fmt.Sprintf("%s/api/v2/pokemon-species/%s", c.baseURL, name)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return domain.Pokemon{}, domain.ErrUpstream(err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.Pokemon{}, domain.ErrUpstream(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return domain.Pokemon{}, domain.ErrNotFound(name)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return domain.Pokemon{}, domain.ErrUpstream(fmt.Errorf("pokeapi returned %d: %s", resp.StatusCode, body))
	}

	var species speciesResponse
	if err := json.NewDecoder(resp.Body).Decode(&species); err != nil {
		return domain.Pokemon{}, domain.ErrUpstream(fmt.Errorf("failed to decode pokeapi response: %w", err))
	}

	description := extractEnglishFlavorText(species.FlavorTextEntries)

	var habitat string
	if species.Habitat != nil {
		habitat = species.Habitat.Name
	}

	return domain.Pokemon{
		Name:        species.Name,
		Description: description,
		Habitat:     habitat,
		IsLegendary: species.IsLegendary,
	}, nil
}

// isRetryable returns true for transient upstream errors (5xx, timeouts).
// 404 and 400-level client errors are not retried.
func isRetryable(err error) bool {
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		return appErr.Code == "UPSTREAM_ERROR"
	}
	return false
}

func extractEnglishFlavorText(entries []flavorTextEntry) string {
	for _, e := range entries {
		if e.Language.Name == "en" {
			return e.FlavorText
		}
	}
	return ""
}

type speciesResponse struct {
	Name              string            `json:"name"`
	IsLegendary       bool              `json:"is_legendary"`
	Habitat           *namedResource    `json:"habitat"`
	FlavorTextEntries []flavorTextEntry `json:"flavor_text_entries"`
}

type namedResource struct {
	Name string `json:"name"`
}

type flavorTextEntry struct {
	FlavorText string        `json:"flavor_text"`
	Language   namedResource `json:"language"`
}
