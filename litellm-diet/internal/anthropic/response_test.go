package anthropic

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConvertResponseCaptured(t *testing.T) {
	captured := `{
		"id":"msg_abc123",
		"type":"message",
		"role":"assistant",
		"model":"claude-haiku-4-5",
		"content":[{"type":"text","text":"olá"}],
		"stop_reason":"end_turn",
		"usage":{"input_tokens":12,"output_tokens":3}
	}`
	out, err := ConvertResponse([]byte(captured), "anthropic/claude-haiku-4-5")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	var dst map[string]any
	if err := json.Unmarshal(out, &dst); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(dst["id"].(string), "chatcmpl-") {
		t.Errorf("id = %v", dst["id"])
	}
	if dst["model"] != "anthropic/claude-haiku-4-5" {
		t.Errorf("model = %v", dst["model"])
	}
	if dst["object"] != "chat.completion" {
		t.Errorf("object = %v", dst["object"])
	}
	choices := dst["choices"].([]any)
	msg := choices[0].(map[string]any)["message"].(map[string]any)
	if msg["content"] != "olá" {
		t.Errorf("content = %v", msg["content"])
	}
	if choices[0].(map[string]any)["finish_reason"] != "stop" {
		t.Errorf("finish_reason = %v", choices[0].(map[string]any)["finish_reason"])
	}
	usage := dst["usage"].(map[string]any)
	if usage["prompt_tokens"] != float64(12) || usage["completion_tokens"] != float64(3) {
		t.Errorf("usage = %v", usage)
	}
	if _, ok := msg["provider_specific_fields"]; !ok {
		t.Error("provider_specific_fields missing")
	}
}

func TestConvertStreamSequence(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_1","usage":{"input_tokens":5,"output_tokens":0}}}`,
		`data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"oi"}}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`,
		"",
	}, "\n")

	var chunks []map[string]any
	prompt, completion, err := ConvertStream(strings.NewReader(stream), "anthropic/claude-haiku-4-5", false, func(b []byte) error {
		var obj map[string]any
		if err := json.Unmarshal(b, &obj); err != nil {
			return err
		}
		chunks = append(chunks, obj)
		return nil
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(chunks) < 3 {
		t.Fatalf("chunks = %d, want at least 3", len(chunks))
	}
	if prompt != 5 || completion != 1 {
		t.Errorf("usage = %d/%d", prompt, completion)
	}
	last := chunks[len(chunks)-1]
	if last["choices"].([]any)[0].(map[string]any)["finish_reason"] != "stop" {
		t.Errorf("last chunk = %v", last)
	}
	for _, c := range chunks {
		if _, ok := c["usage"]; ok {
			t.Fatal("usage chunk emitted without client request")
		}
	}
}

func TestConvertStreamWithUsage(t *testing.T) {
	stream := strings.Join([]string{
		`data: {"type":"message_start","message":{"usage":{"input_tokens":2,"output_tokens":0}}}`,
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":4}}`,
		"",
	}, "\n")
	var hasUsage bool
	_, _, err := ConvertStream(strings.NewReader(stream), "anthropic/x", true, func(b []byte) error {
		var obj map[string]any
		if err := json.Unmarshal(b, &obj); err != nil {
			return err
		}
		if _, ok := obj["usage"]; ok {
			hasUsage = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if !hasUsage {
		t.Fatal("usage not emitted")
	}
}
