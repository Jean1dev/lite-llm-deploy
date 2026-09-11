package httpapi

import "encoding/json"

type responseUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

func extractUsage(body []byte) (responseUsage, bool) {
	var env struct {
		Usage *responseUsage `json:"usage"`
	}
	if err := json.Unmarshal(body, &env); err != nil || env.Usage == nil {
		return responseUsage{}, false
	}
	return *env.Usage, true
}
