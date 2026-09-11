package anthropic

import (
	"encoding/json"
	"fmt"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/jsonutil"
)

var droppedFields = []string{"top_p", "presence_penalty", "frequency_penalty", "n", "logit_bias"}

type openaiReq struct {
	Model               string          `json:"model"`
	Messages            []openaiMsg     `json:"messages"`
	MaxTokens           *int            `json:"max_tokens"`
	MaxCompletionTokens *int            `json:"max_completion_tokens"`
	Temperature         *float64        `json:"temperature"`
	Stream              bool            `json:"stream"`
	Stop                json.RawMessage `json:"stop"`
	Tools               []openaiTool    `json:"tools"`
	ToolChoice          json.RawMessage `json:"tool_choice"`
}

type openaiMsg struct {
	Role       string           `json:"role"`
	Content    json.RawMessage  `json:"content"`
	ToolCalls  []openaiToolCall `json:"tool_calls"`
	ToolCallID string           `json:"tool_call_id"`
	Name       string           `json:"name"`
}

type openaiTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

type openaiToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type anthropicReq struct {
	Model       string          `json:"model"`
	Messages    []anthropicMsg  `json:"messages"`
	MaxTokens   int             `json:"max_tokens"`
	System      any             `json:"system,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Stop        any             `json:"stop_sequences,omitempty"`
	Tools       []anthropicTool `json:"tools,omitempty"`
	ToolChoice  any             `json:"tool_choice,omitempty"`
}

type anthropicMsg struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

func ConvertRequest(body []byte, providerModel string) ([]byte, error) {
	filtered, err := jsonutil.RemoveFields(body, droppedFields...)
	if err != nil {
		return nil, err
	}

	var src openaiReq
	if err := json.Unmarshal(filtered, &src); err != nil {
		return nil, fmt.Errorf("decode openai request: %w", err)
	}

	dst := anthropicReq{
		Model:       providerModel,
		Temperature: src.Temperature,
		Stream:      src.Stream,
		MaxTokens:   4096,
	}
	if src.MaxTokens != nil {
		dst.MaxTokens = *src.MaxTokens
	} else if src.MaxCompletionTokens != nil {
		dst.MaxTokens = *src.MaxCompletionTokens
	}

	system, messages, err := convertMessages(src.Messages)
	if err != nil {
		return nil, err
	}
	dst.System = system
	dst.Messages = messages

	if len(src.Tools) > 0 {
		dst.Tools = make([]anthropicTool, 0, len(src.Tools))
		for _, t := range src.Tools {
			dst.Tools = append(dst.Tools, anthropicTool{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				InputSchema: t.Function.Parameters,
			})
		}
	}
	if len(src.ToolChoice) > 0 {
		dst.ToolChoice = convertToolChoice(src.ToolChoice)
	}
	if len(src.Stop) > 0 {
		dst.Stop = convertStop(src.Stop)
	}

	out, err := json.Marshal(dst)
	if err != nil {
		return nil, fmt.Errorf("serialize anthropic request: %w", err)
	}
	return out, nil
}

func convertMessages(msgs []openaiMsg) (any, []anthropicMsg, error) {
	var system []map[string]any
	out := make([]anthropicMsg, 0, len(msgs))

	for _, m := range msgs {
		switch m.Role {
		case "system":
			text, blocks, err := textOrBlocks(m.Content)
			if err != nil {
				return nil, nil, err
			}
			if text != "" {
				system = append(system, map[string]any{"type": "text", "text": text})
			} else {
				system = append(system, blocks...)
			}
		case "user":
			content, err := convertUserContent(m.Content)
			if err != nil {
				return nil, nil, err
			}
			out = append(out, anthropicMsg{Role: "user", Content: content})
		case "assistant":
			content, err := convertAssistantContent(m)
			if err != nil {
				return nil, nil, err
			}
			out = append(out, anthropicMsg{Role: "assistant", Content: content})
		case "tool":
			result, err := convertToolResult(m)
			if err != nil {
				return nil, nil, err
			}
			if n := len(out); n > 0 && out[n-1].Role == "user" {
				if blocks, ok := out[n-1].Content.([]any); ok {
					out[n-1].Content = append(blocks, result)
					continue
				}
			}
			out = append(out, anthropicMsg{Role: "user", Content: []any{result}})
		}
	}

	var sys any
	if len(system) == 1 {
		if text, ok := system[0]["text"].(string); ok && system[0]["type"] == "text" {
			sys = text
		} else {
			sys = system
		}
	} else if len(system) > 1 {
		sys = system
	}
	return sys, out, nil
}

func textOrBlocks(raw json.RawMessage) (string, []map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil, nil
	}
	var parts []map[string]any
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", nil, fmt.Errorf("message content: %w", err)
	}
	return "", parts, nil
}

func convertUserContent(raw json.RawMessage) (any, error) {
	text, parts, err := textOrBlocks(raw)
	if err != nil {
		return nil, err
	}
	if text != "" {
		return text, nil
	}
	blocks := make([]any, 0, len(parts))
	for _, p := range parts {
		switch p["type"] {
		case "text":
			blocks = append(blocks, map[string]any{"type": "text", "text": p["text"]})
		case "image_url":
			block, err := convertImage(p)
			if err != nil {
				return nil, err
			}
			blocks = append(blocks, block)
		default:
			blocks = append(blocks, p)
		}
	}
	return blocks, nil
}

func convertImage(part map[string]any) (map[string]any, error) {
	urlMap, _ := part["image_url"].(map[string]any)
	url, _ := urlMap["url"].(string)
	if url == "" {
		return nil, fmt.Errorf("image_url missing url")
	}
	media, data, ok := splitDataURL(url)
	if ok {
		return map[string]any{
			"type": "image",
			"source": map[string]any{
				"type":       "base64",
				"media_type": media,
				"data":       data,
			},
		}, nil
	}
	return map[string]any{
		"type": "image",
		"source": map[string]any{
			"type": "url",
			"url":  url,
		},
	}, nil
}

func splitDataURL(url string) (string, string, bool) {
	const prefix = "data:"
	if len(url) < 6 || url[:5] != prefix {
		return "", "", false
	}
	rest := url[5:]
	for i := 0; i < len(rest); i++ {
		if rest[i] == ';' {
			media := rest[:i]
			const b64 = ";base64,"
			if i+len(b64) <= len(rest) && rest[i:i+len(b64)] == b64 {
				return media, rest[i+len(b64):], true
			}
			return "", "", false
		}
	}
	return "", "", false
}

func convertAssistantContent(m openaiMsg) (any, error) {
	blocks := make([]any, 0, 2)
	text, parts, err := textOrBlocks(m.Content)
	if err != nil {
		return nil, err
	}
	if text != "" {
		blocks = append(blocks, map[string]any{"type": "text", "text": text})
	}
	for _, p := range parts {
		blocks = append(blocks, p)
	}
	for _, tc := range m.ToolCalls {
		var input any = map[string]any{}
		if tc.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				input = map[string]any{}
			}
		}
		blocks = append(blocks, map[string]any{
			"type":  "tool_use",
			"id":    tc.ID,
			"name":  tc.Function.Name,
			"input": input,
		})
	}
	if len(blocks) == 1 {
		if b, ok := blocks[0].(map[string]any); ok && b["type"] == "text" {
			return b["text"], nil
		}
	}
	return blocks, nil
}

func convertToolResult(m openaiMsg) (map[string]any, error) {
	text, _, err := textOrBlocks(m.Content)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"type":        "tool_result",
		"tool_use_id": m.ToolCallID,
		"content":     text,
	}, nil
}

func convertToolChoice(raw json.RawMessage) any {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch s {
		case "auto":
			return map[string]any{"type": "auto"}
		case "none":
			return map[string]any{"type": "none"}
		case "required":
			return map[string]any{"type": "any"}
		}
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil {
		if fn, ok := obj["function"].(map[string]any); ok {
			return map[string]any{"type": "tool", "name": fn["name"]}
		}
	}
	return nil
}

func convertStop(raw json.RawMessage) any {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []string{s}
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	return nil
}
