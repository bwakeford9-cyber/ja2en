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

func TestDeepLTranslate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/translate" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "DeepL-Auth-Key test-key:fx" {
			t.Errorf("auth header = %q", got)
		}
		var req deepLRequest
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatal(err)
		}
		if len(req.Text) != 1 || req.Text[0] != "こんにちは世界" {
			t.Errorf("text = %v", req.Text)
		}
		if req.TargetLang != "EN-US" {
			t.Errorf("target_lang = %q, want EN-US", req.TargetLang)
		}
		if req.SourceLang != "JA" {
			t.Errorf("source_lang = %q, want JA", req.SourceLang)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"translations":[{"detected_source_language":"JA","text":"Hello world"}]}`))
	}))
	defer server.Close()

	c := NewDeepLClient(server.URL, "test-key:fx", 5*time.Second)
	out, err := c.Translate(context.Background(), "", "", "こんにちは世界", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if out != "Hello world" {
		t.Errorf("output = %q, want Hello world", out)
	}
}

func TestDeepLTranslate_403(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	c := NewDeepLClient(server.URL, "bad", time.Second)
	_, err := c.Translate(context.Background(), "", "", "u", "")
	if err == nil || !strings.Contains(err.Error(), "invalid DeepL API key") {
		t.Errorf("expected 403/invalid-key error, got %v", err)
	}
}

func TestDeepLTranslate_QuotaExceeded456(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(456)
	}))
	defer server.Close()

	c := NewDeepLClient(server.URL, "k", time.Second)
	_, err := c.Translate(context.Background(), "", "", "u", "")
	if err == nil || !strings.Contains(err.Error(), "quota exceeded") {
		t.Errorf("expected quota-exceeded error, got %v", err)
	}
}

func TestDeepLTranslate_NoTranslations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"translations":[]}`))
	}))
	defer server.Close()

	c := NewDeepLClient(server.URL, "k", time.Second)
	_, err := c.Translate(context.Background(), "", "", "u", "")
	if err == nil || !strings.Contains(err.Error(), "no translations") {
		t.Errorf("expected no-translations error, got %v", err)
	}
}

func TestDeepLTranslate_AutoBaseFreeKey(t *testing.T) {
	c := NewDeepLClient("", "abcd:fx", time.Second)
	if c.apiBase != "https://api-free.deepl.com" {
		t.Errorf("free key apiBase = %q, want api-free.deepl.com", c.apiBase)
	}
}

func TestDeepLTranslate_AutoBasePaidKey(t *testing.T) {
	c := NewDeepLClient("", "abcd-paid", time.Second)
	if c.apiBase != "https://api.deepl.com" {
		t.Errorf("paid key apiBase = %q, want api.deepl.com", c.apiBase)
	}
}

func TestDeepLTranslate_ReasoningEffortIgnored(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), "reasoning_effort") {
			t.Errorf("DeepL request must not contain reasoning_effort, got: %s", string(body))
		}
		_, _ = w.Write([]byte(`{"translations":[{"text":"ok"}]}`))
	}))
	defer server.Close()

	c := NewDeepLClient(server.URL, "k:fx", time.Second)
	if _, err := c.Translate(context.Background(), "ignored-model", "ignored-prompt", "u", "minimal"); err != nil {
		t.Fatal(err)
	}
}
