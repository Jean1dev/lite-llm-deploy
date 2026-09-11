package gemini

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"time"
)

func ConvertStream(r io.Reader, clientModel string, includeUsage bool, emit func([]byte) error) (prompt, completion int, err error) {
	id := newChatID()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	roleSent := false
	var promptTokens, completionTokens int
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		line = strings.TrimPrefix(line, "data:")
		line = strings.TrimSpace(line)
		if line == "" || line == "[DONE]" {
			continue
		}
		p, c, err := emitChunk([]byte(line), id, clientModel, &roleSent, includeUsage, emit)
		if err != nil {
			return promptTokens, completionTokens, err
		}
		if p > 0 {
			promptTokens = p
		}
		if c > 0 {
			completionTokens = c
		}
	}
	return promptTokens, completionTokens, sc.Err()
}

func emitChunk(payload []byte, id, model string, roleSent *bool, includeUsage bool, emit func([]byte) error) (int, int, error) {
	var src geminiResp
	if err := json.Unmarshal(payload, &src); err != nil {
		return 0, 0, nil
	}
	if !*roleSent {
		if err := emit(chunk(id, model, map[string]any{"role": "assistant", "content": ""}, nil)); err != nil {
			return 0, 0, err
		}
		*roleSent = true
	}
	if len(src.Candidates) > 0 {
		for _, p := range src.Candidates[0].Content.Parts {
			if p.Text != "" {
				if err := emit(chunk(id, model, map[string]any{"content": p.Text}, nil)); err != nil {
					return 0, 0, err
				}
			}
			if p.FunctionCall != nil {
				args := "{}"
				if len(p.FunctionCall.Args) > 0 {
					args = string(p.FunctionCall.Args)
				}
				delta := map[string]any{
					"tool_calls": []any{
						map[string]any{
							"index": 0,
							"id":    "call_0",
							"type":  "function",
							"function": map[string]any{
								"name":      p.FunctionCall.Name,
								"arguments": args,
							},
						},
					},
				}
				if err := emit(chunk(id, model, delta, nil)); err != nil {
					return 0, 0, err
				}
			}
		}
		if src.Candidates[0].FinishReason != "" {
			finish := mapFinish(src.Candidates[0].FinishReason)
			if err := emit(chunk(id, model, map[string]any{}, &finish)); err != nil {
				return 0, 0, err
			}
		}
	}
	prompt := src.UsageMetadata.PromptTokenCount
	completion := src.UsageMetadata.CandidatesTokenCount
	if includeUsage && (prompt > 0 || completion > 0) {
		body, err := json.Marshal(map[string]any{
			"id":      id,
			"object":  "chat.completion.chunk",
			"created": time.Now().Unix(),
			"model":   model,
			"choices": []any{},
			"usage": map[string]any{
				"prompt_tokens":     prompt,
				"completion_tokens": completion,
				"total_tokens":      prompt + completion,
			},
		})
		if err != nil {
			return prompt, completion, err
		}
		if err := emit(body); err != nil {
			return prompt, completion, err
		}
	}
	return prompt, completion, nil
}

func chunk(id, model string, delta map[string]any, finish *string) []byte {
	choice := map[string]any{"index": 0, "delta": delta, "finish_reason": nil}
	if finish != nil {
		choice["finish_reason"] = *finish
	}
	body, err := json.Marshal(map[string]any{
		"id":      id,
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []any{choice},
	})
	if err != nil {
		return nil
	}
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
