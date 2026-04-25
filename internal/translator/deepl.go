package translator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DeepLClient calls DeepL's REST translation API.
//
// DeepL is not OpenAI-compatible. It exposes POST /v2/translate with a
// `DeepL-Auth-Key` header, takes JA→EN-US as a fixed pair for ja2en, and
// has no system-prompt concept — formality is the only stylistic knob.
type DeepLClient struct {
	apiBase    string
	apiKey     string
	httpClient *http.Client
}

// NewDeepLClient constructs a DeepL client. If apiBase is empty, the host
// is auto-detected from the key suffix: keys ending in ":fx" target the
// free plan (api-free.deepl.com), all other keys target the paid plan
// (api.deepl.com).
func NewDeepLClient(apiBase, apiKey string, timeout time.Duration) *DeepLClient {
	if apiBase == "" {
		apiBase = autoDeepLBase(apiKey)
	}
	return &DeepLClient{
		apiBase:    strings.TrimRight(apiBase, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
	}
}

func autoDeepLBase(apiKey string) string {
	if strings.HasSuffix(apiKey, ":fx") {
		return "https://api-free.deepl.com"
	}
	return "https://api.deepl.com"
}

type deepLRequest struct {
	Text       []string `json:"text"`
	TargetLang string   `json:"target_lang"`
	SourceLang string   `json:"source_lang,omitempty"`
}

type deepLResponse struct {
	Translations []struct {
		DetectedSourceLanguage string `json:"detected_source_language"`
		Text                   string `json:"text"`
	} `json:"translations"`
	Message string `json:"message"`
}

// Translate sends userText to DeepL and returns the English translation.
// model, systemPrompt, and reasoningEffort are accepted for interface
// compatibility with the OpenAI-compatible Client but ignored — DeepL is
// a translation-only NMT API and supports none of those concepts.
func (c *DeepLClient) Translate(ctx context.Context, _ string, _ string, userText string, _ string) (string, error) {
	body, err := json.Marshal(deepLRequest{
		Text:       []string{userText},
		TargetLang: "EN-US",
		SourceLang: "JA",
	})
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	url := c.apiBase + "/v2/translate"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Authorization", "DeepL-Auth-Key "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ja2en/0.3 (https://github.com/GigiTiti-Kai/ja2en)")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("network failure: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		// continue
	case http.StatusForbidden:
		return "", fmt.Errorf("invalid DeepL API key (HTTP 403). check DEEPL_API_KEY (free keys end in ':fx')")
	case http.StatusNotFound:
		return "", fmt.Errorf("HTTP 404 from %s — likely wrong endpoint for this key (free keys must use api-free.deepl.com)", url)
	case http.StatusRequestEntityTooLarge:
		return "", fmt.Errorf("input too large (HTTP 413). DeepL limits a single request body to ~128 KiB")
	case http.StatusTooManyRequests:
		return "", fmt.Errorf("DeepL rate limit hit (HTTP 429). retry with backoff")
	case 456:
		return "", fmt.Errorf("DeepL monthly character quota exceeded (HTTP 456). free tier: 500,000 chars/month, resets on billing date")
	default:
		return "", fmt.Errorf("DeepL API error (HTTP %d): %s", resp.StatusCode, extractDeepLErrMsg(raw))
	}

	var out deepLResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(out.Translations) == 0 {
		return "", fmt.Errorf("DeepL returned no translations")
	}
	return strings.TrimSpace(out.Translations[0].Text), nil
}

func extractDeepLErrMsg(raw []byte) string {
	var r deepLResponse
	if err := json.Unmarshal(raw, &r); err == nil && r.Message != "" {
		return r.Message
	}
	s := string(raw)
	if len(s) > 500 {
		s = s[:500] + "..."
	}
	return s
}
