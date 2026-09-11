package catalog

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/provider"
)

var sourceProviders = map[string]provider.ID{
	"openai":    provider.OpenAI,
	"anthropic": provider.Anthropic,
	"gemini":    provider.Gemini,
}

type sourceModel struct {
	Provider   string   `json:"litellm_provider"`
	Mode       string   `json:"mode"`
	InputCost  *float64 `json:"input_cost_per_token"`
	OutputCost *float64 `json:"output_cost_per_token"`
	MaxInput   *float64 `json:"max_input_tokens"`
	MaxOutput  *float64 `json:"max_output_tokens"`
	MaxTokens  *float64 `json:"max_tokens"`
}

func Prune(source []byte) ([]Entry, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(source, &raw); err != nil {
		return nil, fmt.Errorf("decode price map: %w", err)
	}

	byName := make(map[string]Entry, 512)
	for key, blob := range raw {
		if key == "sample_spec" {
			continue
		}

		var m sourceModel
		if err := json.Unmarshal(blob, &m); err != nil {
			return nil, fmt.Errorf("decode model %q: %w", key, err)
		}

		id, ok := sourceProviders[m.Provider]
		if !ok || m.Mode != "chat" {
			continue
		}
		if m.InputCost == nil || m.OutputCost == nil {
			continue
		}

		inTokens := firstPositive(m.MaxInput, m.MaxTokens)
		outTokens := firstPositive(m.MaxOutput, m.MaxTokens)
		if inTokens == 0 {
			continue
		}

		model := modelName(key, id)
		if model == "" {
			continue
		}

		e := Entry{
			Provider:        id,
			Model:           model,
			InputPrice:      *m.InputCost,
			OutputPrice:     *m.OutputCost,
			MaxInputTokens:  inTokens,
			MaxOutputTokens: outTokens,
		}
		byName[e.Name()] = e
	}

	out := make([]Entry, 0, len(byName))
	for _, e := range byName {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Provider != out[j].Provider {
			return out[i].Provider < out[j].Provider
		}
		return out[i].Model < out[j].Model
	})
	return out, nil
}

func modelName(key string, id provider.ID) string {
	prefix := string(id) + "/"
	if strings.HasPrefix(key, prefix) {
		return strings.TrimPrefix(key, prefix)
	}
	if strings.Contains(key, "/") {
		return ""
	}
	return key
}

func firstPositive(values ...*float64) int {
	for _, v := range values {
		if v != nil && *v > 0 {
			return int(*v)
		}
	}
	return 0
}
