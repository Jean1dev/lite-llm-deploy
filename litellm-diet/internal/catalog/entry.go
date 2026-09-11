package catalog

import "github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/provider"

type Entry struct {
	Provider        provider.ID `json:"provider"`
	Model           string      `json:"model"`
	InputPrice      float64     `json:"input_price"`
	OutputPrice     float64     `json:"output_price"`
	MaxInputTokens  int         `json:"max_input_tokens"`
	MaxOutputTokens int         `json:"max_output_tokens"`
}

func (e Entry) Name() string {
	return string(e.Provider) + "/" + e.Model
}
