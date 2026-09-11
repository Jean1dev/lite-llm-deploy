package config

import (
	"errors"
	"testing"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/provider"
)

func clearEnv(t *testing.T) {
	t.Helper()

	for _, v := range []string{
		"PORT", "LOG_LEVEL", "MEMORY_LIMIT",
		"OPENAI_API_KEY", "OPENAI_BASE_URL",
		"ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL",
		"GEMINI_API_KEY", "GEMINI_BASE_URL",
		"DATABASE_URL", "LITELLM_DATABASE_URL", "MASTER_KEY",
		"KEY_REFRESH_INTERVAL", "SPEND_FLUSH_INTERVAL",
	} {
		t.Setenv(v, "")
	}
}

func minEnv(t *testing.T) {
	t.Helper()
	clearEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-openai")
	t.Setenv("DATABASE_URL", "postgres://diet:diet@localhost:5432/diet?sslmode=disable")
	t.Setenv("MASTER_KEY", "sk-dev-master")
}

func TestLoadWithoutProviderRejectsStartup(t *testing.T) {
	clearEnv(t)
	t.Setenv("DATABASE_URL", "postgres://diet:diet@localhost:5432/diet?sslmode=disable")
	t.Setenv("MASTER_KEY", "sk-dev-master")

	_, err := Load()
	if !errors.Is(err, ErrNoProvider) {
		t.Fatalf("err = %v, want %v", err, ErrNoProvider)
	}
}

func TestLoadProviderWithoutCredentialIsDisabled(t *testing.T) {
	minEnv(t)

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(c.Providers) != 1 {
		t.Fatalf("providers = %d, want 1", len(c.Providers))
	}
	if _, ok := c.Providers[provider.Anthropic]; ok {
		t.Error("anthropic enabled without credential")
	}
	if _, ok := c.Providers[provider.Gemini]; ok {
		t.Error("gemini enabled without credential")
	}
}

func TestLoadDefaultAndOverrideBaseURL(t *testing.T) {
	minEnv(t)
	t.Setenv("ANTHROPIC_API_KEY", "sk-anthropic")
	t.Setenv("ANTHROPIC_BASE_URL", "https://proxy.interno/anthropic/")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	def, _ := provider.DefaultBaseURL(provider.OpenAI)
	if got := c.Providers[provider.OpenAI].BaseURL; got != def {
		t.Errorf("openai base url = %q, want %q", got, def)
	}
	if got := c.Providers[provider.Anthropic].BaseURL; got != "https://proxy.interno/anthropic" {
		t.Errorf("anthropic base url = %q, want no trailing slash", got)
	}
	if got := c.Providers[provider.OpenAI].Credential; got != "sk-openai" {
		t.Errorf("openai credential = %q", got)
	}
}

func TestLoadInvalidBaseURL(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"missing scheme", "api.openai.com/v1"},
		{"non http scheme", "ftp://api.openai.com/v1"},
		{"missing host", "https://"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			minEnv(t)
			t.Setenv("OPENAI_BASE_URL", c.url)

			if _, err := Load(); err == nil {
				t.Fatalf("expected error for url %q", c.url)
			}
		})
	}
}

func TestLoadIgnoresBlankCredential(t *testing.T) {
	minEnv(t)
	t.Setenv("OPENAI_API_KEY", "   ")
	t.Setenv("GEMINI_API_KEY", "gemini-key")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := c.Providers[provider.OpenAI]; ok {
		t.Error("openai enabled with blank credential")
	}
	if len(c.Providers) != 1 {
		t.Errorf("providers = %d, want 1", len(c.Providers))
	}
}
