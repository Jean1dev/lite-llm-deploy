package anthropic

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestConvertRequestTable(t *testing.T) {
	cases := []struct {
		name  string
		input string
		check func(t *testing.T, dst map[string]any)
	}{
		{
			name: "messages and system",
			input: `{
				"model":"anthropic/claude-haiku-4-5",
				"messages":[
					{"role":"system","content":"seja breve"},
					{"role":"user","content":"olá"}
				],
				"temperature":0.2,
				"max_tokens":128,
				"top_p":0.9
			}`,
			check: func(t *testing.T, dst map[string]any) {
				if dst["model"] != "claude-haiku-4-5" {
					t.Errorf("model = %v", dst["model"])
				}
				if dst["system"] != "seja breve" {
					t.Errorf("system = %v", dst["system"])
				}
				if dst["max_tokens"] != float64(128) {
					t.Errorf("max_tokens = %v", dst["max_tokens"])
				}
				if dst["temperature"] != 0.2 {
					t.Errorf("temperature = %v", dst["temperature"])
				}
				if _, ok := dst["top_p"]; ok {
					t.Error("top_p should not exist")
				}
				msgs := dst["messages"].([]any)
				if len(msgs) != 1 {
					t.Fatalf("messages = %d", len(msgs))
				}
			},
		},
		{
			name:  "removes presence and frequency penalty",
			input: `{"model":"anthropic/x","messages":[{"role":"user","content":"a"}],"presence_penalty":1,"frequency_penalty":1,"temperature":0.5}`,
			check: func(t *testing.T, dst map[string]any) {
				if _, ok := dst["presence_penalty"]; ok {
					t.Error("presence_penalty remained")
				}
				if dst["temperature"] != 0.5 {
					t.Errorf("temperature = %v", dst["temperature"])
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, err := ConvertRequest([]byte(c.input), "claude-haiku-4-5")
			if err != nil {
				t.Fatalf("convert: %v", err)
			}
			var dst map[string]any
			if err := json.Unmarshal(out, &dst); err != nil {
				t.Fatalf("decode output: %v", err)
			}
			c.check(t, dst)
		})
	}
}

func TestConvertRequestTool(t *testing.T) {
	input := `{
		"model":"anthropic/claude-haiku-4-5",
		"messages":[
			{"role":"user","content":"hora?"},
			{"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"hora","arguments":"{}"}}]},
			{"role":"tool","tool_call_id":"call_1","content":"12:00"}
		],
		"tools":[{"type":"function","function":{"name":"hora","description":"hora atual","parameters":{"type":"object"}}}]
	}`
	out, err := ConvertRequest([]byte(input), "claude-haiku-4-5")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if !bytes.Contains(out, []byte(`"tool_use"`)) {
		t.Fatalf("tool_use missing: %s", out)
	}
	if !bytes.Contains(out, []byte(`"tool_result"`)) {
		t.Fatalf("tool_result missing: %s", out)
	}
}

func TestConvertRequestImage(t *testing.T) {
	input := `{
		"model":"anthropic/claude-haiku-4-5",
		"messages":[{"role":"user","content":[
			{"type":"text","text":"o que é?"},
			{"type":"image_url","image_url":{"url":"data:image/png;base64,iVBORw0KGgo="}}
		]}]
	}`
	out, err := ConvertRequest([]byte(input), "claude-haiku-4-5")
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if !bytes.Contains(out, []byte(`"type":"image"`)) {
		t.Fatalf("image block missing: %s", out)
	}
}
