// Package config loads and resolves the ja2en TOML configuration file.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// Config maps the on-disk TOML structure of ~/.config/ja2en/config.toml.
type Config struct {
	DefaultProfile  string             `toml:"default_profile"`
	Provider        string             `toml:"provider"`
	Model           string             `toml:"model"`
	APIBase         string             `toml:"api_base"`
	APIKeyEnv       string             `toml:"api_key_env"`
	ReasoningEffort string             `toml:"reasoning_effort"`
	TimeoutSeconds  int                `toml:"timeout_seconds"`
	Profiles        map[string]Profile `toml:"profiles"`
}

// Profile holds per-profile overrides. Each field falls back to the
// top-level Config value when empty.
//
// Provider selects the translation backend: "openai" (OpenAI-compatible
// chat/completions, used for OpenRouter, Google AI Studio, Cerebras, Groq,
// etc.) or "deepl" (DeepL REST API). Default is "openai".
type Profile struct {
	Prompt          string `toml:"prompt"`
	PromptFile      string `toml:"prompt_file"`
	Provider        string `toml:"provider"`
	Model           string `toml:"model"`
	APIBase         string `toml:"api_base"`
	APIKeyEnv       string `toml:"api_key_env"`
	ReasoningEffort string `toml:"reasoning_effort"`
}

// Resolved is the runtime-ready config after merging CLI flags and env vars.
type Resolved struct {
	Prompt          string
	Provider        string
	Model           string
	APIBase         string
	APIKey          string
	ReasoningEffort string
	TimeoutSeconds  int
}

// Path returns the absolute path to config.toml, honoring XDG_CONFIG_HOME.
func Path() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "ja2en", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "ja2en", "config.toml"), nil
}

// Load parses the TOML file at path into a Config.
func Load(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return &cfg, nil
}

// Resolve merges Config with CLI overrides and environment variables,
// returning a runtime-ready Resolved value. Precedence for each field
// (highest to lowest): CLI override, profile-level value, top-level
// Config value, hard-coded default. The api_key_env field names which
// environment variable holds the API key; it defaults to
// OPENROUTER_API_KEY for backward compatibility.
func (c *Config) Resolve(profileName, modelOverride, promptFileOverride string) (*Resolved, error) {
	if profileName == "" {
		profileName = c.DefaultProfile
	}
	if profileName == "" {
		return nil, fmt.Errorf("no profile specified and no default_profile in config")
	}

	profile, ok := c.Profiles[profileName]
	if !ok {
		names := make([]string, 0, len(c.Profiles))
		for k := range c.Profiles {
			names = append(names, k)
		}
		return nil, fmt.Errorf("profile %q not found. available: %v", profileName, names)
	}

	provider := pickFirst(profile.Provider, c.Provider, "openai")
	if provider != "openai" && provider != "deepl" {
		return nil, fmt.Errorf("profile %q: unknown provider %q (must be \"openai\" or \"deepl\")", profileName, provider)
	}

	prompt, err := resolvePrompt(profile, promptFileOverride, provider)
	if err != nil {
		return nil, fmt.Errorf("profile %q: %w", profileName, err)
	}

	// DeepL is a translation-only API; it has no concept of "model".
	model := pickFirst(modelOverride, profile.Model, c.Model)
	if model == "" && provider != "deepl" {
		return nil, fmt.Errorf("no model specified at any level (cli/profile/top)")
	}

	apiBase := pickFirst(profile.APIBase, c.APIBase)
	if apiBase == "" && provider == "openai" {
		apiBase = "https://openrouter.ai/api/v1"
	}
	// For DeepL, an empty apiBase signals "auto-detect from key suffix" — the
	// translator package picks api-free.deepl.com (`:fx` suffix) or api.deepl.com.

	keyEnv := pickFirst(profile.APIKeyEnv, c.APIKeyEnv)
	if keyEnv == "" {
		if provider == "deepl" {
			keyEnv = "DEEPL_API_KEY"
		} else {
			keyEnv = "OPENROUTER_API_KEY"
		}
	}
	apiKey := strings.TrimSpace(os.Getenv(keyEnv))
	if apiKey == "" {
		return nil, fmt.Errorf("environment variable %s is required (referenced by api_key_env)", keyEnv)
	}

	timeout := c.TimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}

	reasoningEffort := pickFirst(profile.ReasoningEffort, c.ReasoningEffort)

	return &Resolved{
		Prompt:          prompt,
		Provider:        provider,
		Model:           model,
		APIBase:         apiBase,
		APIKey:          apiKey,
		ReasoningEffort: reasoningEffort,
		TimeoutSeconds:  timeout,
	}, nil
}

// pickFirst returns the first non-empty string from the candidates.
func pickFirst(candidates ...string) string {
	for _, s := range candidates {
		if s != "" {
			return s
		}
	}
	return ""
}

func resolvePrompt(p Profile, override, provider string) (string, error) {
	if override != "" {
		return readPromptFile(override)
	}
	if p.Prompt != "" && p.PromptFile != "" {
		return "", fmt.Errorf("both 'prompt' and 'prompt_file' set; only one is allowed")
	}
	if p.PromptFile != "" {
		return readPromptFile(p.PromptFile)
	}
	if p.Prompt != "" {
		return strings.TrimSpace(p.Prompt), nil
	}
	// DeepL is a translation-only API and has no system prompt concept;
	// running without a prompt is normal for the deepl provider.
	if provider == "deepl" {
		return "", nil
	}
	return "", fmt.Errorf("neither 'prompt' nor 'prompt_file' is set")
}

func readPromptFile(path string) (string, error) {
	expanded, err := expandTilde(path)
	if err != nil {
		return "", err
	}
	// User-supplied path is the whole point of --prompt-file; reading it is intentional.
	data, err := os.ReadFile(expanded) // #nosec G304
	if err != nil {
		return "", fmt.Errorf("read prompt file %s: %w", expanded, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func expandTilde(path string) (string, error) {
	if !strings.HasPrefix(path, "~/") && path != "~" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if path == "~" {
		return home, nil
	}
	return filepath.Join(home, path[2:]), nil
}
