package anthropic

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

type anthropicEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta struct {
		Type        string `json:"type"`
		Text        string `json:"text"`
		PartialJSON string `json:"partial_json"`
		StopReason  string `json:"stop_reason"`
	} `json:"delta"`
	ContentBlock struct {
		Type string `json:"type"`
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"content_block"`
	Message struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	} `json:"message"`
	Usage struct {
		OutputTokens int `json:"output_tokens"`
		InputTokens  int `json:"input_tokens"`
	} `json:"usage"`
}

func ConvertStream(r io.Reader, clientModel string, includeUsage bool, emit func([]byte) error) (prompt, completion int, err error) {
	id := newChatID()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	roleSent := false
	var promptTokens, completionTokens int
	var toolID, toolName string

	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		var ev anthropicEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue
		}

		switch ev.Type {
		case "message_start":
			promptTokens = ev.Message.Usage.InputTokens
			if !roleSent {
				if err := emit(chunkOpenAI(id, clientModel, map[string]any{"role": "assistant", "content": ""}, nil)); err != nil {
					return 0, 0, err
				}
				roleSent = true
			}
		case "content_block_start":
			if ev.ContentBlock.Type == "tool_use" {
				toolID = ev.ContentBlock.ID
				toolName = ev.ContentBlock.Name
				delta := map[string]any{
					"tool_calls": []any{
						map[string]any{
							"index": ev.Index,
							"id":    toolID,
							"type":  "function",
							"function": map[string]any{
								"name":      toolName,
								"arguments": "",
							},
						},
					},
				}
				if err := emit(chunkOpenAI(id, clientModel, delta, nil)); err != nil {
					return 0, 0, err
				}
			}
		case "content_block_delta":
			switch ev.Delta.Type {
			case "text_delta":
				if err := emit(chunkOpenAI(id, clientModel, map[string]any{"content": ev.Delta.Text}, nil)); err != nil {
					return 0, 0, err
				}
			case "input_json_delta":
				delta := map[string]any{
					"tool_calls": []any{
						map[string]any{
							"index": ev.Index,
							"function": map[string]any{
								"arguments": ev.Delta.PartialJSON,
							},
						},
					},
				}
				if err := emit(chunkOpenAI(id, clientModel, delta, nil)); err != nil {
					return 0, 0, err
				}
			}
		case "message_delta":
			if ev.Usage.OutputTokens > 0 {
				completionTokens = ev.Usage.OutputTokens
			}
			if ev.Usage.InputTokens > 0 {
				promptTokens = ev.Usage.InputTokens
			}
			finish := mapStop(ev.Delta.StopReason)
			if err := emit(chunkOpenAI(id, clientModel, map[string]any{}, &finish)); err != nil {
				return 0, 0, err
			}
			if includeUsage {
				if err := emit(chunkUsage(id, clientModel, promptTokens, completionTokens)); err != nil {
					return 0, 0, err
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return promptTokens, completionTokens, err
	}
	return promptTokens, completionTokens, nil
}

func chunkOpenAI(id, model string, delta map[string]any, finish *string) []byte {
	choice := map[string]any{
		"index": 0,
		"delta": delta,
	}
	if finish != nil {
		choice["finish_reason"] = *finish
	} else {
		choice["finish_reason"] = nil
	}
	body, _ := json.Marshal(map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": unixNow(),
		"model":   model,
		"choices": []any{choice},
	})
	return body
}

func chunkUsage(id, model string, prompt, completion int) []byte {
	body, _ := json.Marshal(map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": unixNow(),
		"model":   model,
		"choices": []any{},
		"usage": map[string]any{
			"prompt_tokens":     prompt,
			"completion_tokens": completion,
			"total_tokens":      prompt + completion,
		},
	})
	return body
}

func WrapSSE(obj []byte) []byte {
	var buf bytes.Buffer
	buf.Grow(len(obj) + 8)
	buf.WriteString("data: ")
	buf.Write(obj)
	buf.WriteByte('\n')
	buf.WriteByte('\n')
	return buf.Bytes()
}
