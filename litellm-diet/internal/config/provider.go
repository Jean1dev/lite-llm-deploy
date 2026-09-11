package config

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/provider"
)

var ErrNoProvider = fmt.Errorf("no provider configured")

type Provider struct {
	ID         provider.ID
	Credential string
	BaseURL    string
}

var providerEnv = map[provider.ID]struct {
	credential string
	baseURL    string
}{
	provider.OpenAI:    {"OPENAI_API_KEY", "OPENAI_BASE_URL"},
	provider.Anthropic: {"ANTHROPIC_API_KEY", "ANTHROPIC_BASE_URL"},
	provider.Gemini:    {"GEMINI_API_KEY", "GEMINI_BASE_URL"},
}

func readProviders() (map[provider.ID]Provider, error) {
	providers := make(map[provider.ID]Provider, len(provider.Supported))

	for _, id := range provider.Supported {
		vars := providerEnv[id]

		credential := strings.TrimSpace(os.Getenv(vars.credential))
		if credential == "" {
			continue
		}

		baseURL, err := readBaseURL(id, vars.baseURL)
		if err != nil {
			return nil, err
		}

		providers[id] = Provider{ID: id, Credential: credential, BaseURL: baseURL}
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("%w: set at least one of OPENAI_API_KEY, ANTHROPIC_API_KEY, GEMINI_API_KEY", ErrNoProvider)
	}
	return providers, nil
}

func readBaseURL(id provider.ID, envName string) (string, error) {
	raw := strings.TrimSpace(os.Getenv(envName))
	if raw == "" {
		u, ok := provider.DefaultBaseURL(id)
		if !ok {
			return "", fmt.Errorf("provider %q has no default base url", id)
		}
		return u, nil
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%s %q invalid: %w", envName, raw, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%s %q must use http or https", envName, raw)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("%s %q missing host", envName, raw)
	}
	return strings.TrimSuffix(raw, "/"), nil
}
