package translator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/aminsalami/funpokedex/internal/pkg"
)

type FunTranslator struct {
	httpClient *http.Client
	baseURL    string
	path       string
	retryCfg   pkg.RetryConfig
}

func NewYodaTranslator(httpClient *http.Client, baseURL string) *FunTranslator {
	return newFunTranslator(httpClient, baseURL, "yoda")
}

func NewShakespeareTranslator(httpClient *http.Client, baseURL string) *FunTranslator {
	return newFunTranslator(httpClient, baseURL, "shakespeare")
}

func newFunTranslator(httpClient *http.Client, baseURL, path string) *FunTranslator {
	return &FunTranslator{
		httpClient: httpClient,
		baseURL:    baseURL,
		path:       path,
		retryCfg:   pkg.DefaultRetryConfig(),
	}
}

func (t *FunTranslator) Translate(ctx context.Context, text string) (string, error) {
	return pkg.Retry(ctx, t.retryCfg, func() (string, error) {
		return t.translateOnce(ctx, text)
	}, func(_ error) bool { return true })
}

func (t *FunTranslator) translateOnce(ctx context.Context, text string) (string, error) {
	endpoint := fmt.Sprintf("%s/translate/%s", t.baseURL, t.path)

	form := url.Values{"text": {text}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("translator: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := t.httpClient.Do(req)
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
