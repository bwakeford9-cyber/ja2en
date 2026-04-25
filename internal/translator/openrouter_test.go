package translator

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTranslate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		var req chatRequest
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "test/model" {
			t.Errorf("model = %q", req.Model)
		}
		if len(req.Messages) != 2 || req.Messages[0].Role != "system" || req.Messages[1].Role != "user" {
			t.Errorf("unexpected messages: %+v", req.Messages)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"Hello world"}}]}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key", 5*time.Second)
	out, err := c.Translate(context.Background(), "test/model", "translate", "こんにちは世界", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != "Hello world" {
		t.Errorf("output = %q", out)
	}
}

func TestTranslate_401(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid","code":401}}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "bad", time.Second)
	_, err := c.Translate(context.Background(), "m", "p", "u", "")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 error, got %v", err)
	}
}

func TestTranslate_429(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	c := NewClient(server.URL, "k", time.Second)
	_, err := c.Translate(context.Background(), "m", "p", "u", "")
	if err == nil || !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("expected rate-limit error, got %v", err)
	}
}

func TestTranslate_500WithErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"upstream provider down","code":500}}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "k", time.Second)
	_, err := c.Translate(context.Background(), "m", "p", "u", "")
	if err == nil || !strings.Contains(err.Error(), "upstream provider down") {
		t.Errorf("expected upstream error message, got %v", err)
	}
}

func TestTranslate_ReasoningEffortOmittedWhenEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "reasoning_effort") {
			t.Errorf("reasoning_effort should be omitted when empty, got body: %s", string(body))
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"x"}}]}`))
	}))
	defer server.Close()
	c := NewClient(server.URL, "k", time.Second)
	if _, err := c.Translate(context.Background(), "m", "p", "u", ""); err != nil {
		t.Fatal(err)
	}
}

func TestTranslate_ReasoningEffortIncludedWhenSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req chatRequest
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatal(err)
		}
		if req.ReasoningEffort == nil || *req.ReasoningEffort != "minimal" {
			t.Errorf("reasoning_effort = %v, want \"minimal\"", req.ReasoningEffort)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"x"}}]}`))
	}))
	defer server.Close()
	c := NewClient(server.URL, "k", time.Second)
	if _, err := c.Translate(context.Background(), "m", "p", "u", "minimal"); err != nil {
		t.Fatal(err)
	}
}

func TestTranslate_NoChoices(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "k", time.Second)
	_, err := c.Translate(context.Background(), "m", "p", "u", "")
	if err == nil || !strings.Contains(err.Error(), "no choices") {
		t.Errorf("expected no-choices error, got %v", err)
	}
}
