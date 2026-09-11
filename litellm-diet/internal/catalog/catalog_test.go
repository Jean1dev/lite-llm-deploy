package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/provider"
)

func fixtureCatalog(t *testing.T) Catalog {
	t.Helper()

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
	c, err := Load(raw)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return c
}

func TestExpandCountMatchesCatalog(t *testing.T) {
	c := fixtureCatalog(t)

	cases := []struct {
		id   provider.ID
		want int
	}{
		{provider.OpenAI, 1},
		{provider.Anthropic, 1},
		{provider.Gemini, 1},
	}
	for _, cso := range cases {
		t.Run(string(cso.id), func(t *testing.T) {
			if got := c.Count(cso.id); got != cso.want {
				t.Errorf("count = %d, want %d", got, cso.want)
			}
			expanded := c.Expand([]provider.ID{cso.id})
			if len(expanded) != cso.want {
				t.Errorf("expanded = %d, want %d", len(expanded), cso.want)
			}
			for _, e := range expanded {
				if e.Provider != cso.id {
					t.Errorf("entry %s has provider %s", e.Name(), e.Provider)
				}
			}
		})
	}

	all := c.Expand(provider.Supported)
	if len(all) != 3 {
		t.Errorf("full expand = %d, want 3", len(all))
	}
}

func TestExpandOmitsUndeclaredProvider(t *testing.T) {
	c := fixtureCatalog(t)

	expanded := c.Expand([]provider.ID{provider.OpenAI})
	if len(expanded) != 1 {
		t.Fatalf("expanded = %d, want 1", len(expanded))
	}
	if expanded[0].Name() != "openai/gpt-4o" {
		t.Errorf("name = %q", expanded[0].Name())
	}
}

func TestLoadEmbeddedHasEntriesPerProvider(t *testing.T) {
	c, err := LoadEmbedded()
	if err != nil {
		t.Fatalf("load embedded: %v", err)
	}
	for _, id := range provider.Supported {
		if c.Count(id) == 0 {
			t.Errorf("embedded catalog has no models for %s", id)
		}
		if got := len(c.Expand([]provider.ID{id})); got != c.Count(id) {
			t.Errorf("%s: expand %d != catalog %d", id, got, c.Count(id))
		}
	}
}
