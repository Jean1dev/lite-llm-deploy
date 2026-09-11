package catalog

import (
	"errors"
	"testing"
)

func TestCostKnownAndMissingModel(t *testing.T) {
	c := fixtureCatalog(t)

	cases := []struct {
		name    string
		model   string
		usage   Usage
		want    float64
		missing bool
	}{
		{
			name:  "known openai model",
			model: "openai/gpt-4o",
			usage: Usage{PromptTokens: 1000, CompletionTokens: 500},
			want:  1000*2.5e-6 + 500*1e-5,
		},
		{
			name:  "known anthropic model",
			model: "anthropic/claude-haiku-4-5",
			usage: Usage{PromptTokens: 200, CompletionTokens: 50},
			want:  200*1e-6 + 50*5e-6,
		},
		{
			name:  "zero usage",
			model: "openai/gpt-4o",
			usage: Usage{},
			want:  0,
		},
		{
			name:    "model missing from catalog",
			model:   "openai/unknown-model",
			usage:   Usage{PromptTokens: 10, CompletionTokens: 10},
			missing: true,
		},
		{
			name:    "known provider new model",
			model:   "gemini/gemini-future",
			usage:   Usage{PromptTokens: 1, CompletionTokens: 1},
			missing: true,
		},
	}

	for _, cso := range cases {
		t.Run(cso.name, func(t *testing.T) {
			got, err := c.Cost(cso.model, cso.usage)
			if cso.missing {
				if !errors.Is(err, ErrUnknownModel) {
					t.Fatalf("err = %v, want %v", err, ErrUnknownModel)
				}
				if got != 0 {
					t.Errorf("cost = %g, want 0 for missing model", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != cso.want {
				t.Errorf("cost = %g, want %g", got, cso.want)
			}
		})
	}
}
