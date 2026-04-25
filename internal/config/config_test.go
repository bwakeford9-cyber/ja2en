package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolve_BasicSimpleProfile(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "test-key")

	cfg := &Config{
		DefaultProfile: "simple",
		Model:          "google/gemma-3-27b-it:free",
		Profiles: map[string]Profile{
			"simple": {Prompt: "translate to english"},
		},
	}

	r, err := cfg.Resolve("", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Model != "google/gemma-3-27b-it:free" {
		t.Errorf("model = %q, want google/gemma-3-27b-it:free", r.Model)
	}
	if r.Prompt != "translate to english" {
		t.Errorf("prompt = %q, want translate to english", r.Prompt)
	}
	if r.APIBase != "https://openrouter.ai/api/v1" {
		t.Errorf("api_base default = %q", r.APIBase)
	}
	if r.TimeoutSeconds != 30 {
		t.Errorf("timeout default = %d, want 30", r.TimeoutSeconds)
	}
}

func TestResolve_MissingAPIKey(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")

	cfg := &Config{
		DefaultProfile: "simple",
		Model:          "x",
		Profiles:       map[string]Profile{"simple": {Prompt: "p"}},
	}
	if _, err := cfg.Resolve("", "", ""); err == nil {
		t.Fatal("expected error for missing API key")
	}
}

func TestResolve_ProfileNotFound(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "k")

	cfg := &Config{
		DefaultProfile: "simple",
		Model:          "x",
		Profiles:       map[string]Profile{"simple": {Prompt: "p"}},
	}
	_, err := cfg.Resolve("nope", "", "")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected profile-not-found error, got %v", err)
	}
}

func TestResolve_PromptAndPromptFileExclusive(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "k")

	cfg := &Config{
		DefaultProfile: "x",
		Model:          "m",
		Profiles: map[string]Profile{
			"x": {Prompt: "p", PromptFile: "/tmp/foo"},
		},
	}
	_, err := cfg.Resolve("", "", "")
	if err == nil || !strings.Contains(err.Error(), "only one is allowed") {
		t.Fatalf("expected exclusivity error, got %v", err)
	}
}

func TestResolve_ModelOverridePrecedence(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "k")

	cfg := &Config{
		DefaultProfile: "x",
		Model:          "top",
		Profiles: map[string]Profile{
			"x": {Prompt: "p", Model: "profile"},
		},
	}

	tests := []struct {
		name string
		cli  string
		want string
	}{
		{"cli wins", "cli", "cli"},
		{"profile fallback", "", "profile"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := cfg.Resolve("", tt.cli, "")
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if r.Model != tt.want {
				t.Errorf("model = %q, want %q", r.Model, tt.want)
			}
		})
	}
}

func TestResolve_PromptFileOverride(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "k")

	dir := t.TempDir()
	p := filepath.Join(dir, "ad-hoc.md")
	if err := os.WriteFile(p, []byte("ADHOC PROMPT\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := &Config{
		DefaultProfile: "x",
		Model:          "m",
		Profiles:       map[string]Profile{"x": {Prompt: "default"}},
	}
	r, err := cfg.Resolve("", "", p)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Prompt != "ADHOC PROMPT" {
		t.Errorf("prompt = %q, want ADHOC PROMPT", r.Prompt)
	}
}

func TestResolve_ProviderDefaultsToOpenAI(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "k")

	cfg := &Config{
		DefaultProfile: "x",
		Model:          "m",
		Profiles:       map[string]Profile{"x": {Prompt: "p"}},
	}
	r, err := cfg.Resolve("", "", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Provider != "openai" {
		t.Errorf("provider = %q, want openai (default)", r.Provider)
	}
}

func TestResolve_ProviderProfileOverridesTop(t *testing.T) {
	t.Setenv("DEEPL_API_KEY", "deepl-key")

	cfg := &Config{
		DefaultProfile: "deepl",
		Provider:       "openai",
		Model:          "m",
		Profiles: map[string]Profile{
			"deepl": {Provider: "deepl"},
		},
	}
	r, err := cfg.Resolve("", "", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Provider != "deepl" {
		t.Errorf("provider = %q, want deepl (profile override)", r.Provider)
	}
}

func TestResolve_DeepLAllowsNoModel(t *testing.T) {
	t.Setenv("DEEPL_API_KEY", "k:fx")

	cfg := &Config{
		DefaultProfile: "deepl",
		Profiles: map[string]Profile{
			"deepl": {Provider: "deepl"},
		},
	}
	r, err := cfg.Resolve("", "", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Model != "" {
		t.Errorf("model = %q, want empty for deepl", r.Model)
	}
	if r.APIKey != "k:fx" {
		t.Errorf("api_key = %q", r.APIKey)
	}
}

func TestResolve_DeepLAllowsNoPrompt(t *testing.T) {
	t.Setenv("DEEPL_API_KEY", "k")

	cfg := &Config{
		DefaultProfile: "deepl",
		Profiles: map[string]Profile{
			"deepl": {Provider: "deepl"}, // no Prompt or PromptFile
		},
	}
	r, err := cfg.Resolve("", "", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.Prompt != "" {
		t.Errorf("prompt = %q, want empty for deepl", r.Prompt)
	}
}

func TestResolve_OpenAIRequiresPrompt(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "k")

	cfg := &Config{
		DefaultProfile: "x",
		Model:          "m",
		Profiles: map[string]Profile{
			"x": {}, // no Prompt
		},
	}
	_, err := cfg.Resolve("", "", "")
	if err == nil || !strings.Contains(err.Error(), "neither 'prompt'") {
		t.Errorf("expected missing-prompt error, got %v", err)
	}
}

func TestResolve_UnknownProviderRejected(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "k")

	cfg := &Config{
		DefaultProfile: "x",
		Profiles: map[string]Profile{
			"x": {Provider: "anthropic-native", Prompt: "p"},
		},
	}
	_, err := cfg.Resolve("", "", "")
	if err == nil || !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("expected unknown-provider error, got %v", err)
	}
}

func TestResolve_ReasoningEffortPrecedence(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "k")

	cfg := &Config{
		DefaultProfile:  "x",
		Model:           "m",
		ReasoningEffort: "high",
		APIKeyEnv:       "GEMINI_API_KEY",
		Profiles: map[string]Profile{
			"x": {Prompt: "p", ReasoningEffort: "minimal"},
		},
	}
	r, err := cfg.Resolve("", "", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.ReasoningEffort != "minimal" {
		t.Errorf("reasoning_effort = %q, want minimal (profile wins)", r.ReasoningEffort)
	}
}

func TestResolve_ReasoningEffortFallsBackToTop(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "k")

	cfg := &Config{
		DefaultProfile:  "x",
		Model:           "m",
		ReasoningEffort: "none",
		APIKeyEnv:       "GEMINI_API_KEY",
		Profiles: map[string]Profile{
			"x": {Prompt: "p"}, // no ReasoningEffort
		},
	}
	r, err := cfg.Resolve("", "", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.ReasoningEffort != "none" {
		t.Errorf("reasoning_effort = %q, want none (top-level fallback)", r.ReasoningEffort)
	}
}

func TestResolve_DeepLAPIKeyEnvDefault(t *testing.T) {
	// When provider=deepl and no api_key_env is set anywhere, it should
	// default to DEEPL_API_KEY rather than OPENROUTER_API_KEY.
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("DEEPL_API_KEY", "deepl-key:fx")

	cfg := &Config{
		DefaultProfile: "deepl",
		Profiles: map[string]Profile{
			"deepl": {Provider: "deepl"},
		},
	}
	r, err := cfg.Resolve("", "", "")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if r.APIKey != "deepl-key:fx" {
		t.Errorf("api_key = %q, want deepl-key:fx", r.APIKey)
	}
}

func TestExpandTilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	tests := []struct {
		in, want string
	}{
		{"~", home},
		{"~/foo", filepath.Join(home, "foo")},
		{"/abs/path", "/abs/path"},
		{"relative", "relative"},
	}
	for _, tt := range tests {
		got, err := expandTilde(tt.in)
		if err != nil {
			t.Errorf("expandTilde(%q) err: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("expandTilde(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestResolve_CustomAPIKeyEnv(t *testing.T) {
	// Make sure the default env var is empty so we can prove the custom one is used.
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "gemini-test-key")

	cfg := &Config{
		DefaultProfile: "simple",
		Model:          "gemini-3.1-flash-lite-preview",
		APIBase:        "https://generativelanguage.googleapis.com/v1beta/openai",
		APIKeyEnv:      "GEMINI_API_KEY",
		Profiles:       map[string]Profile{"simple": {Prompt: "translate"}},
	}
	r, err := cfg.Resolve("", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.APIKey != "gemini-test-key" {
		t.Errorf("api_key = %q, want gemini-test-key", r.APIKey)
	}
	if r.APIBase != "https://generativelanguage.googleapis.com/v1beta/openai" {
		t.Errorf("api_base = %q", r.APIBase)
	}
}

func TestResolve_ProfileLevelAPIKeyEnv(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "or-key")
	t.Setenv("GEMINI_API_KEY", "gem-key")

	cfg := &Config{
		DefaultProfile: "openrouter",
		Model:          "fallback",
		APIKeyEnv:      "GEMINI_API_KEY", // top-level says Gemini
		Profiles: map[string]Profile{
			"openrouter": {
				Prompt:    "translate",
				APIKeyEnv: "OPENROUTER_API_KEY", // profile overrides to OpenRouter
				APIBase:   "https://openrouter.ai/api/v1",
				Model:     "anthropic/claude-haiku-4.5",
			},
		},
	}
	r, err := cfg.Resolve("openrouter", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.APIKey != "or-key" {
		t.Errorf("api_key = %q, want or-key (profile-level override)", r.APIKey)
	}
	if r.APIBase != "https://openrouter.ai/api/v1" {
		t.Errorf("api_base = %q", r.APIBase)
	}
	if r.Model != "anthropic/claude-haiku-4.5" {
		t.Errorf("model = %q", r.Model)
	}
}

func TestResolve_MissingCustomAPIKey_MentionsActualVar(t *testing.T) {
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")

	cfg := &Config{
		DefaultProfile: "simple",
		Model:          "x",
		APIKeyEnv:      "GEMINI_API_KEY",
		Profiles:       map[string]Profile{"simple": {Prompt: "p"}},
	}
	_, err := cfg.Resolve("", "", "")
	if err == nil || !strings.Contains(err.Error(), "GEMINI_API_KEY") {
		t.Fatalf("expected error mentioning GEMINI_API_KEY, got %v", err)
	}
}

func TestLoad_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.toml")
	body := `
default_profile = "simple"
model = "google/gemma-3-27b-it:free"
api_base = "https://openrouter.ai/api/v1"
timeout_seconds = 15

[profiles.simple]
prompt = "translate jp to en"

[profiles.detailed]
prompt_file = "/tmp/x"
model = "anthropic/claude-3-haiku"
`
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.DefaultProfile != "simple" {
		t.Errorf("default_profile = %q", cfg.DefaultProfile)
	}
	if cfg.TimeoutSeconds != 15 {
		t.Errorf("timeout = %d", cfg.TimeoutSeconds)
	}
	if cfg.Profiles["detailed"].Model != "anthropic/claude-3-haiku" {
		t.Errorf("detailed.model = %q", cfg.Profiles["detailed"].Model)
	}
}
