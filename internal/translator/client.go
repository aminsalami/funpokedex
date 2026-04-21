package translator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

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

func (c *Client) Translate(ctx context.Context, text string, typ domain.TranslatorType) (string, error) {
	return pkg.Retry(ctx, c.retryCfg, func() (string, error) {
		return c.translateOnce(ctx, text, typ)
	}, func(_ error) bool { return true })
}

func (c *Client) translateOnce(ctx context.Context, text string, typ domain.TranslatorType) (string, error) {
	endpoint := fmt.Sprintf("%s/translate/%s", c.baseURL, string(typ))

	form := url.Values{"text": {text}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("translator: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("translator: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("translator: returned %d: %s", resp.StatusCode, body)
	}

	var result translationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("translator: decode response: %w", err)
	}

	if result.Contents.Translated == "" {
		return "", fmt.Errorf("translator: empty translated text")
	}

	return result.Contents.Translated, nil
}

type translationResponse struct {
	Contents translationContents `json:"contents"`
}

type translationContents struct {
	Translated string `json:"translated"`
}
