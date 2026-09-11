package gemini

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type geminiResp struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text         string `json:"text"`
				FunctionCall *struct {
					Name string          `json:"name"`
					Args json.RawMessage `json:"args"`
				} `json:"functionCall"`
			} `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

func ConvertResponse(body []byte, clientModel string) ([]byte, error) {
	var src geminiResp
	if err := json.Unmarshal(body, &src); err != nil {
		return nil, fmt.Errorf("decode gemini response: %w", err)
	}

	var text string
	var tools []map[string]any
	finish := "stop"
	if len(src.Candidates) > 0 {
		finish = mapFinish(src.Candidates[0].FinishReason)
		for i, p := range src.Candidates[0].Content.Parts {
			if p.Text != "" {
				text += p.Text
			}
			if p.FunctionCall != nil {
				args := "{}"
				if len(p.FunctionCall.Args) > 0 {
					args = string(p.FunctionCall.Args)
				}
				tools = append(tools, map[string]any{
					"id":   fmt.Sprintf("call_%d", i),
					"type": "function",
					"function": map[string]any{
						"name":      p.FunctionCall.Name,
						"arguments": args,
					},
				})
				finish = "tool_calls"
			}
		}
	}

	msg := map[string]any{"role": "assistant", "content": text}
	if len(tools) > 0 {
		msg["tool_calls"] = tools
		if text == "" {
			msg["content"] = nil
		}
	}

	prompt := src.UsageMetadata.PromptTokenCount
	completion := src.UsageMetadata.CandidatesTokenCount
	total := src.UsageMetadata.TotalTokenCount
	if total == 0 {
		total = prompt + completion
	}

	dst := map[string]any{
		"id":      newChatID(),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   clientModel,
		"choices": []any{
			map[string]any{
				"index":         0,
				"message":       msg,
				"finish_reason": finish,
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     prompt,
			"completion_tokens": completion,
			"total_tokens":      total,
		},
	}
	out, err := json.Marshal(dst)
	if err != nil {
		return nil, fmt.Errorf("serialize openai response: %w", err)
	}
	return out, nil
}

func mapFinish(reason string) string {
	switch reason {
	case "STOP", "":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY":
		return "content_filter"
	default:
		return "stop"
	}
}

func newChatID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("chatcmpl-%d", time.Now().UnixNano())
	}
	return "chatcmpl-" + hex.EncodeToString(b)
}
