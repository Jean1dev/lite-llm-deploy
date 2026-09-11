package jsonutil

import (
	"bytes"
	"fmt"
	"testing"
)

func TestExtractModel(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		want    string
		wantErr bool
	}{
		{name: "model at start", body: `{"model":"openai/gpt-4o","messages":[{"role":"user","content":"hi"}]}`, want: "openai/gpt-4o"},
		{name: "model after messages", body: `{"messages":[{"role":"user","content":"hi"}],"model":"anthropic/claude-haiku-4-5"}`, want: "anthropic/claude-haiku-4-5"},
		{name: "missing", body: `{"messages":[]}`, wantErr: true},
		{name: "not an object", body: `[]`, wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ExtractModel([]byte(c.body))
			if c.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("model = %q, want %q", got, c.want)
			}
		})
	}
}

func TestRemoveFieldsKeepsOthers(t *testing.T) {
	body := []byte(`{"model":"anthropic/claude","temperature":0.5,"top_p":0.9,"presence_penalty":1,"messages":[{"role":"user","content":"x"}]}`)
	out, err := RemoveFields(body, "top_p", "presence_penalty")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if bytes.Contains(out, []byte(`"top_p"`)) {
		t.Errorf("top_p remained: %s", out)
	}
	if bytes.Contains(out, []byte(`"presence_penalty"`)) {
		t.Errorf("presence_penalty remained: %s", out)
	}
	if !bytes.Contains(out, []byte(`"temperature"`)) {
		t.Errorf("temperature removed: %s", out)
	}
	if !bytes.Contains(out, []byte(`"messages"`)) {
		t.Errorf("messages removed: %s", out)
	}
}

func TestExtractModelAllocDoesNotGrowWithPrompt(t *testing.T) {
	small := testing.Benchmark(func(b *testing.B) {
		body := bodyWithPrompt(2048)
		for i := 0; i < b.N; i++ {
			if _, err := ExtractModel(body); err != nil {
				b.Fatal(err)
			}
		}
	})
	large := testing.Benchmark(func(b *testing.B) {
		body := bodyWithPrompt(256 * 1024)
		for i := 0; i < b.N; i++ {
			if _, err := ExtractModel(body); err != nil {
				b.Fatal(err)
			}
		}
	})
	if large.AllocedBytesPerOp() > small.AllocedBytesPerOp()*3 && large.AllocedBytesPerOp() > 4096 {
		t.Fatalf("allocation grew with prompt: small=%d large=%d", small.AllocedBytesPerOp(), large.AllocedBytesPerOp())
	}
}

func BenchmarkExtractModel(b *testing.B) {
	for _, n := range []int{1024, 64 * 1024, 256 * 1024} {
		body := bodyWithPrompt(n)
		b.Run(fmt.Sprintf("%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := ExtractModel(body); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func bodyWithPrompt(n int) []byte {
	prompt := bytes.Repeat([]byte("a"), n)
	var buf bytes.Buffer
	buf.Grow(n + 64)
	buf.WriteString(`{"messages":[{"role":"user","content":"`)
	buf.Write(prompt)
	buf.WriteString(`"}],"model":"openai/gpt-4o"}`)
	return buf.Bytes()
}
