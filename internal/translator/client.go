package translator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/aminsalami/funpokedex/internal/domain"
	"github.com/aminsalami/funpokedex/internal/pkg"
)

type FunTranslator struct {
	name       string
	httpClient *http.Client
	baseURL    string
	path       string
	retryCfg   pkg.RetryConfig
}

func NewYodaTranslator(httpClient *http.Client) *FunTranslator {
	return newFunTranslator(domain.TranslatorYoda, httpClient, "https://api.funtranslations.mercxry.me/v1/translate", "yoda")
}

func NewShakespeareTranslator(httpClient *http.Client) *FunTranslator {
	return newFunTranslator(domain.TranslatorShakespeare, httpClient, "https://api.funtranslations.mercxry.me/v1/translate", "shakespeare")
}

func newFunTranslator(name string, httpClient *http.Client, baseURL, path string) *FunTranslator {
	return &FunTranslator{
		name:       name,
		httpClient: httpClient,
		baseURL:    baseURL,
		path:       path,
		retryCfg:   pkg.DefaultRetryConfig(),
	}
}

func (t *FunTranslator) Name() string {
	return t.name
}

func (t *FunTranslator) Translate(ctx context.Context, text string) (string, error) {
	return pkg.Retry(ctx, t.retryCfg, func() (string, error) {
		return t.translateOnce(ctx, text)
	}, func(_ error) bool { return true })
}

func (t *FunTranslator) translateOnce(ctx context.Context, text string) (string, error) {
	endpoint := fmt.Sprintf("%s/%s", t.baseURL, t.path)

	body, err := json.Marshal(translationRequest{Text: text})
	if err != nil {
		return "", fmt.Errorf("translator: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("translator: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("translator: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp translationErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp.Error.Message != "" {
			return "", fmt.Errorf("translator: %d: %s", errResp.Error.Code, errResp.Error.Message)
		}
		return "", fmt.Errorf("translator: unexpected status %d", resp.StatusCode)
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

type translationRequest struct {
	Text string `json:"text"`
}

type translationResponse struct {
	Contents translationContents `json:"contents"`
}

type translationContents struct {
	Translated  string `json:"translated"`
	Text        string `json:"text"`
	Translation string `json:"translation"`
}

type translationErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	RetryAfter int `json:"retry_after"`
}
