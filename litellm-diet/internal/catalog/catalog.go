package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/provider"
)

//go:embed data/catalog.json
var embeddedCatalog []byte

type Catalog struct {
	byName     map[string]Entry
	byProvider map[provider.ID][]Entry
}

func LoadEmbedded() (Catalog, error) {
	return Load(embeddedCatalog)
}

func Load(raw []byte) (Catalog, error) {
	var entries []Entry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return Catalog{}, fmt.Errorf("decode catalog: %w", err)
	}
	if len(entries) == 0 {
		return Catalog{}, fmt.Errorf("empty catalog")
	}

	c := Catalog{
		byName:     make(map[string]Entry, len(entries)),
		byProvider: make(map[provider.ID][]Entry, len(provider.Supported)),
	}
	for _, e := range entries {
		c.byName[e.Name()] = e
		c.byProvider[e.Provider] = append(c.byProvider[e.Provider], e)
	}
	return c, nil
}

func (c Catalog) Entry(name string) (Entry, bool) {
	e, ok := c.byName[name]
	return e, ok
}

func (c Catalog) ByProvider(id provider.ID) []Entry {
	return c.byProvider[id]
}

func (c Catalog) Expand(ids []provider.ID) []Entry {
	total := 0
	for _, id := range ids {
		total += len(c.byProvider[id])
	}

	out := make([]Entry, 0, total)
	for _, id := range ids {
		out = append(out, c.byProvider[id]...)
	}
	return out
}

func (c Catalog) Count(id provider.ID) int {
	return len(c.byProvider[id])
}
