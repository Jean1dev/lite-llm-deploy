package anthropic

import (
	"encoding/json"
	"fmt"
)

type anthropicResp struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Model   string `json:"model"`
	Content []struct {
		Type  string          `json:"type"`
		Text  string          `json:"text"`
		ID    string          `json:"id"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func ConvertResponse(body []byte, clientModel string) ([]byte, error) {
	var src anthropicResp
	if err := json.Unmarshal(body, &src); err != nil {
		return nil, fmt.Errorf("decode anthropic response: %w", err)
	}

	text, tools := extractContent(src)
	finish := mapStop(src.StopReason)

	msg := map[string]any{
		"role":    "assistant",
		"content": text,
		"provider_specific_fields": map[string]any{
			"id":            src.ID,
			"type":          src.Type,
			"role":          src.Role,
			"stop_reason":   src.StopReason,
			"stop_sequence": nil,
		},
	}
	if len(tools) > 0 {
		msg["tool_calls"] = tools
		if text == nil || *text == "" {
			msg["content"] = nil
		}
	}

	dst := map[string]any{
		"id":      newChatID(),
		"object":  "chat.completion",
		"created": unixNow(),
		"model":   clientModel,
		"choices": []any{
			map[string]any{
				"index":         0,
				"message":       msg,
				"finish_reason": finish,
				"provider_specific_fields": map[string]any{
					"stop_reason": src.StopReason,
				},
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     src.Usage.InputTokens,
			"completion_tokens": src.Usage.OutputTokens,
			"total_tokens":      src.Usage.InputTokens + src.Usage.OutputTokens,
		},
	}

	out, err := json.Marshal(dst)
	if err != nil {
		return nil, fmt.Errorf("serialize openai response: %w", err)
	}
	return out, nil
}

func extractContent(src anthropicResp) (*string, []map[string]any) {
	var text string
	tools := make([]map[string]any, 0)
	for _, b := range src.Content {
		switch b.Type {
		case "text":
			text += b.Text
		case "tool_use":
			args := "{}"
			if len(b.Input) > 0 {
				args = string(b.Input)
			}
			tools = append(tools, map[string]any{
				"id":   b.ID,
				"type": "function",
				"function": map[string]any{
					"name":      b.Name,
					"arguments": args,
				},
			})
		}
	}
	if text == "" && len(tools) > 0 {
		return nil, tools
	}
	return &text, tools
}

func mapStop(reason string) string {
	switch reason {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		if reason == "" {
			return "stop"
		}
		return reason
	}
}
