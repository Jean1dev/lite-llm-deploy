package gemini

import (
	"encoding/json"
	"fmt"
)

type openaiReq struct {
	Model               string          `json:"model"`
	Messages            []openaiMsg     `json:"messages"`
	MaxTokens           *int            `json:"max_tokens"`
	MaxCompletionTokens *int            `json:"max_completion_tokens"`
	Temperature         *float64        `json:"temperature"`
	TopP                *float64        `json:"top_p"`
	Stop                json.RawMessage `json:"stop"`
	Tools               []openaiTool    `json:"tools"`
	ToolChoice          json.RawMessage `json:"tool_choice"`
}

type openaiMsg struct {
	Role       string           `json:"role"`
	Content    json.RawMessage  `json:"content"`
	ToolCalls  []openaiToolCall `json:"tool_calls"`
	ToolCallID string           `json:"tool_call_id"`
}

type openaiTool struct {
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

type openaiToolCall struct {
	ID       string `json:"id"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type geminiReq struct {
	SystemInstruction *geminiContent  `json:"system_instruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  map[string]any  `json:"generationConfig,omitempty"`
	Tools             []geminiTools   `json:"tools,omitempty"`
	ToolConfig        any             `json:"toolConfig,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text         string        `json:"text,omitempty"`
	FunctionCall *functionCall `json:"functionCall,omitempty"`
	FunctionResp *functionResp `json:"functionResponse,omitempty"`
	InlineData   *inlineData   `json:"inline_data,omitempty"`
}

type functionCall struct {
	Name string `json:"name"`
	Args any    `json:"args"`
}

type functionResp struct {
	Name     string `json:"name"`
	Response any    `json:"response"`
}

type inlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type geminiTools struct {
	FunctionDeclarations []map[string]any `json:"functionDeclarations"`
}

func ConvertRequest(body []byte) ([]byte, error) {
	var src openaiReq
	if err := json.Unmarshal(body, &src); err != nil {
		return nil, fmt.Errorf("decode openai request: %w", err)
	}

	dst := geminiReq{GenerationConfig: map[string]any{}}
	if src.Temperature != nil {
		dst.GenerationConfig["temperature"] = *src.Temperature
	}
	if src.TopP != nil {
		dst.GenerationConfig["topP"] = *src.TopP
	}
	if src.MaxTokens != nil {
		dst.GenerationConfig["maxOutputTokens"] = *src.MaxTokens
	} else if src.MaxCompletionTokens != nil {
		dst.GenerationConfig["maxOutputTokens"] = *src.MaxCompletionTokens
	}
	if len(src.Stop) > 0 {
		dst.GenerationConfig["stopSequences"] = convertStop(src.Stop)
	}
	if len(dst.GenerationConfig) == 0 {
		dst.GenerationConfig = nil
	}

	var system []geminiPart
	for _, m := range src.Messages {
		switch m.Role {
		case "system":
			parts, err := textParts(m.Content)
			if err != nil {
				return nil, err
			}
			system = append(system, parts...)
		case "user":
			parts, err := userParts(m.Content)
			if err != nil {
				return nil, err
			}
			dst.Contents = append(dst.Contents, geminiContent{Role: "user", Parts: parts})
		case "assistant":
			parts, err := assistantParts(m)
			if err != nil {
				return nil, err
			}
			dst.Contents = append(dst.Contents, geminiContent{Role: "model", Parts: parts})
		case "tool":
			text, _ := textOf(m.Content)
			dst.Contents = append(dst.Contents, geminiContent{
				Role: "user",
				Parts: []geminiPart{{
					FunctionResp: &functionResp{Name: "tool", Response: map[string]any{"content": text}},
				}},
			})
		}
	}
	if len(system) > 0 {
		dst.SystemInstruction = &geminiContent{Parts: system}
	}
	if len(src.Tools) > 0 {
		decls := make([]map[string]any, 0, len(src.Tools))
		for _, t := range src.Tools {
			decls = append(decls, map[string]any{
				"name":        t.Function.Name,
				"description": t.Function.Description,
				"parameters":  t.Function.Parameters,
			})
		}
		dst.Tools = []geminiTools{{FunctionDeclarations: decls}}
	}

	out, err := json.Marshal(dst)
	if err != nil {
		return nil, fmt.Errorf("serialize gemini request: %w", err)
	}
	return out, nil
}

func textParts(raw json.RawMessage) ([]geminiPart, error) {
	text, err := textOf(raw)
	if err != nil {
		return nil, err
	}
	if text == "" {
		return nil, nil
	}
	return []geminiPart{{Text: text}}, nil
}

func userParts(raw json.RawMessage) ([]geminiPart, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return []geminiPart{{Text: text}}, nil
	}
	var parts []map[string]any
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, fmt.Errorf("gemini content: %w", err)
	}
	out := make([]geminiPart, 0, len(parts))
	for _, p := range parts {
		switch p["type"] {
		case "text":
			s, _ := p["text"].(string)
			out = append(out, geminiPart{Text: s})
		case "image_url":
			urlMap, _ := p["image_url"].(map[string]any)
			url, _ := urlMap["url"].(string)
			media, data, ok := splitDataURL(url)
			if ok {
				out = append(out, geminiPart{InlineData: &inlineData{MimeType: media, Data: data}})
			}
		}
	}
	return out, nil
}

func assistantParts(m openaiMsg) ([]geminiPart, error) {
	parts, err := textParts(m.Content)
	if err != nil {
		return nil, err
	}
	for _, tc := range m.ToolCalls {
		var args any = map[string]any{}
		if tc.Function.Arguments != "" {
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		}
		parts = append(parts, geminiPart{FunctionCall: &functionCall{Name: tc.Function.Name, Args: args}})
	}
	return parts, nil
}

func textOf(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}
	var parts []map[string]any
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", err
	}
	var out string
	for _, p := range parts {
		if p["type"] == "text" {
			s, _ := p["text"].(string)
			out += s
		}
	}
	return out, nil
}

func convertStop(raw json.RawMessage) []string {
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

func splitDataURL(url string) (string, string, bool) {
	if len(url) < 6 || url[:5] != "data:" {
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
		}
	}
	return "", "", false
}
