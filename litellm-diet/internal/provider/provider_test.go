package provider

import (
	"errors"
	"testing"
)

func TestSplitPrefixes(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		id      ID
		model   string
		wantErr bool
	}{
		{name: "openai", in: "openai/gpt-4o", id: OpenAI, model: "gpt-4o"},
		{name: "anthropic", in: "anthropic/claude-haiku-4-5", id: Anthropic, model: "claude-haiku-4-5"},
		{name: "gemini", in: "gemini/gemini-2.5-flash", id: Gemini, model: "gemini-2.5-flash"},
		{name: "unknown", in: "mistral/large", wantErr: true},
		{name: "no slash", in: "gpt-4o", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, model, err := Split(c.in)
			if c.wantErr {
				if !errors.Is(err, ErrUnknownPrefix) {
					t.Fatalf("err = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if id != c.id || model != c.model {
				t.Errorf("got %s %s", id, model)
			}
		})
	}
}
