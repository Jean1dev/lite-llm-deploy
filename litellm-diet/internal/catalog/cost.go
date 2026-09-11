package catalog

import "fmt"

var ErrUnknownModel = fmt.Errorf("model missing from catalog")

type Usage struct {
	PromptTokens     int
	CompletionTokens int
}

func (c Catalog) Cost(name string, usage Usage) (float64, error) {
	e, ok := c.byName[name]
	if !ok {
		return 0, fmt.Errorf("%s: %w", name, ErrUnknownModel)
	}
	return e.InputPrice*float64(usage.PromptTokens) + e.OutputPrice*float64(usage.CompletionTokens), nil
}
