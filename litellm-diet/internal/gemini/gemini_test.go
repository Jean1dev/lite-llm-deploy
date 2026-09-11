package gemini

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConvertRequestTable(t *testing.T) {
	cases := []struct {
		name  string
		input string
		check func(t *testing.T, dst map[string]any)
	}{
		{
			name: "system and sampling",
			input: `{
				"model":"gemini/gemini-2.5-flash",
				"messages":[
					{"role":"system","content":"objetivo"},
					{"role":"user","content":"oi"}
				],
				"temperature":0.1,
				"max_tokens":64
			}`,
			check: func(t *testing.T, dst map[string]any) {
				if dst["system_instruction"] == nil {
					t.Fatal("system_instruction missing")
				}
				cfg := dst["generationConfig"].(map[string]any)
				if cfg["temperature"] != 0.1 {
					t.Errorf("temperature = %v", cfg["temperature"])
				}
				if cfg["maxOutputTokens"] != float64(64) {
					t.Errorf("maxOutputTokens = %v", cfg["maxOutputTokens"])
				}
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := ConvertRequest([]byte(c.input))
			if err != nil {
				t.Fatalf("convert: %v", err)
			}
			var dst map[string]any
			if err := json.Unmarshal(out, &dst); err != nil {
				t.Fatalf("decode: %v", err)
			}
			c.check(t, dst)
		})
	}
}

func TestConvertResponseCaptured(t *testing.T) {
	captured := `{
		"candidates":[{"content":{"parts":[{"text":"resposta"}]},"finishReason":"STOP"}],
		"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":2,"totalTokenCount":10}
	}`
	out, err := ConvertResponse([]byte(captured), "gemini/gemini-2.5-flash")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	var dst map[string]any
	if err := json.Unmarshal(out, &dst); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if dst["model"] != "gemini/gemini-2.5-flash" {
		t.Errorf("model = %v", dst["model"])
	}
	msg := dst["choices"].([]any)[0].(map[string]any)["message"].(map[string]any)
	if msg["content"] != "resposta" {
		t.Errorf("content = %v", msg["content"])
	}
	if dst["choices"].([]any)[0].(map[string]any)["finish_reason"] != "stop" {
		t.Errorf("unexpected finish_reason")
	}
	usage := dst["usage"].(map[string]any)
	if usage["total_tokens"] != float64(10) {
		t.Errorf("usage = %v", usage)
	}
}

func TestConvertRequestTool(t *testing.T) {
	input := `{
		"model":"gemini/gemini-2.5-flash",
		"messages":[{"role":"user","content":"hora"}],
		"tools":[{"type":"function","function":{"name":"hora","parameters":{"type":"object"}}}]
	}`
	out, err := ConvertRequest([]byte(input))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if !strings.Contains(string(out), "functionDeclarations") {
		t.Fatalf("tool missing: %s", out)
	}
}

func TestConvertStream(t *testing.T) {
	stream := `data: {"candidates":[{"content":{"parts":[{"text":"oi"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":1}}` + "\n"
	var n int
	_, _, err := ConvertStream(strings.NewReader(stream), "gemini/x", false, func([]byte) error {
		n++
		return nil
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if n == 0 {
		t.Fatal("no chunk emitted")
	}
}
