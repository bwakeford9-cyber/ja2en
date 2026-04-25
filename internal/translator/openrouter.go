// Package translator provides an OpenRouter (OpenAI-compatible) chat client.
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

// Client wraps an http.Client for OpenRouter chat completion calls.
type Client struct {
	apiBase    string
	apiKey     string
	httpClient *http.Client
}

// NewClient constructs a Client targeting apiBase with the given key and timeout.
func NewClient(apiBase, apiKey string, timeout time.Duration) *Client {
	return &Client{
		apiBase:    strings.TrimRight(apiBase, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: timeout},
	}
}

type chatRequest struct {
	Model           string    `json:"model"`
	Messages        []message `json:"messages"`
	Temperature     *float64  `json:"temperature,omitempty"`
	ReasoningEffort *string   `json:"reasoning_effort,omitempty"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
	Error *apiError `json:"error,omitempty"`
}

type apiError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// Translate sends a chat completion request and returns the assistant's text.
// Temperature is fixed at 0 for deterministic translation output.
//
// reasoningEffort maps to the OpenAI-compatible `reasoning_effort` field,
// supported by GPT-5.x ("none" / "low" / "medium" / "high" / "xhigh"; some
// older 5.x routes also accept "minimal", but 5.4-nano does not) and Gemini
// 2.5 ("none" disables thinking). Pass an empty string to omit the field
// (provider defaults apply). Critical for Gemini 2.5 — without it, the
// model runs in thinking mode and consumes the project's 250K shared TPM
// per request, throttling RPD to ~20.
func (c *Client) Translate(ctx context.Context, model, systemPrompt, userText, reasoningEffort string) (string, error) {
	temp := 0.0
	payload := chatRequest{
		Model: model,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userText},
		},
		Temperature: &temp,
	}
	if reasoningEffort != "" {
		payload.ReasoningEffort = &reasoningEffort
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	url := c.apiBase + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	// OpenRouter recommends these for usage attribution
	req.Header.Set("HTTP-Referer", "https://github.com/GigiTiti-Kai/ja2en")
	req.Header.Set("X-Title", "ja2en")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("network failure: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		// continue
	case http.StatusUnauthorized:
		return "", fmt.Errorf("invalid API key (HTTP 401). check OPENROUTER_API_KEY")
	case http.StatusTooManyRequests:
		return "", fmt.Errorf("rate limit exceeded (HTTP 429). free tier: 20 RPM, 50-1000 req/day")
	default:
		return "", fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, extractErrMsg(raw))
	}

	var out chatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("API error: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("API returned no choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func extractErrMsg(raw []byte) string {
	var r chatResponse
	if err := json.Unmarshal(raw, &r); err == nil && r.Error != nil {
		return r.Error.Message
	}
	s := string(raw)
	if len(s) > 500 {
		s = s[:500] + "..."
	}
	return s
}
