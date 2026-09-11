package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/provider"
)

func TestPruneKeepsChatModelsOfThreeProviders(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("testdata", "sample_prices.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	entries, err := Prune(source)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}

	byName := make(map[string]Entry, len(entries))
	for _, e := range entries {
		if e.InputPrice <= 0 || e.OutputPrice <= 0 {
			t.Errorf("%s missing input or output price", e.Name())
		}
		if e.MaxInputTokens <= 0 {
			t.Errorf("%s missing context limit", e.Name())
		}
		byName[e.Name()] = e
	}

	cases := []struct {
		name        string
		provider    provider.ID
		inputPrice  float64
		outputPrice float64
		maxIn       int
		maxOut      int
	}{
		{"openai/gpt-4o", provider.OpenAI, 2.5e-6, 1e-5, 128000, 16384},
		{"anthropic/claude-haiku-4-5", provider.Anthropic, 1e-6, 5e-6, 200000, 64000},
		{"gemini/gemini-2.5-flash", provider.Gemini, 3e-7, 2.5e-6, 1048576, 65536},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e, ok := byName[c.name]
			if !ok {
				t.Fatalf("missing from pruned catalog")
			}
			if e.Provider != c.provider {
				t.Errorf("provider = %q, want %q", e.Provider, c.provider)
			}
			if e.InputPrice != c.inputPrice {
				t.Errorf("input price = %g, want %g", e.InputPrice, c.inputPrice)
			}
			if e.OutputPrice != c.outputPrice {
				t.Errorf("output price = %g, want %g", e.OutputPrice, c.outputPrice)
			}
			if e.MaxInputTokens != c.maxIn {
				t.Errorf("max input = %d, want %d", e.MaxInputTokens, c.maxIn)
			}
			if e.MaxOutputTokens != c.maxOut {
				t.Errorf("max output = %d, want %d", e.MaxOutputTokens, c.maxOut)
			}
		})
	}
}

func TestPruneDeduplicatesPrefixedKey(t *testing.T) {
	source := []byte(`{
		"gpt-4o": {
			"litellm_provider": "openai",
			"mode": "chat",
			"input_cost_per_token": 2.5e-6,
			"output_cost_per_token": 1e-5,
			"max_input_tokens": 128000,
			"max_output_tokens": 16384
		},
		"openai/gpt-4o": {
			"litellm_provider": "openai",
			"mode": "chat",
			"input_cost_per_token": 2.5e-6,
			"output_cost_per_token": 1e-5,
			"max_input_tokens": 128000,
			"max_output_tokens": 16384
		}
	}`)

	entries, err := Prune(source)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].Name() != "openai/gpt-4o" {
		t.Errorf("name = %q", entries[0].Name())
	}
}

func TestPruneRejectsInvalidJSON(t *testing.T) {
	if _, err := Prune([]byte(`{`)); err == nil {
		t.Fatal("expected error for incomplete JSON")
	}
}

func TestPruneSerializesPricesAndLimits(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("testdata", "sample_prices.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	entries, err := Prune(source)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}

	raw, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var back []map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("decode artifact: %v", err)
	}
	for _, item := range back {
		for _, field := range []string{"input_price", "output_price", "max_input_tokens", "max_output_tokens"} {
			if _, ok := item[field]; !ok {
				t.Errorf("artifact missing %s on %v", field, item["model"])
			}
		}
	}
}
