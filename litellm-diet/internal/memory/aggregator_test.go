package memory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/key"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/storage"
)

type fakeWriter struct {
	mu       sync.Mutex
	batches  [][]storage.SpendDelta
	usage    [][]storage.UsageDelta
	usageErr error
}

func (e *fakeWriter) AddUsage(_ context.Context, items []storage.UsageDelta) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.usageErr != nil {
		return e.usageErr
	}
	cp := make([]storage.UsageDelta, len(items))
	copy(cp, items)
	e.usage = append(e.usage, cp)
	return nil
}

func (e *fakeWriter) AddSpend(_ context.Context, items []storage.SpendDelta) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	cp := make([]storage.SpendDelta, len(items))
	copy(cp, items)
	e.batches = append(e.batches, cp)
	return nil
}

func (e *fakeWriter) writes() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.batches)
}

func TestAggregatorNRequestsProduceOneWrite(t *testing.T) {
	writer := &fakeWriter{}
	m := NewMap(fakeReader{keys: []key.Key{{Hash: "abc"}}})
	if err := m.Load(context.Background()); err != nil {
		t.Fatalf("load: %v", err)
	}
	ag := NewAggregator(writer, m)
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)

	for i := 0; i < 7; i++ {
		ag.Record("abc", 0.01, now.Add(time.Duration(i)*time.Second))
	}
	if writer.writes() != 0 {
		t.Fatalf("premature writes: %d", writer.writes())
	}

	if err := ag.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if writer.writes() != 1 {
		t.Fatalf("writes = %d, want 1", writer.writes())
	}
	if got := writer.batches[0][0].Spend; got != 0.07 {
		t.Errorf("aggregated spend = %g, want 0.07", got)
	}

	k, ok := m.Lookup("abc")
	if !ok {
		t.Fatal("key missing from map")
	}
	if k.Spend != 0.07 {
		t.Errorf("in-memory spend = %g, want 0.07", k.Spend)
	}
}

func TestAggregatorShutdownKeepsPendingSpend(t *testing.T) {
	writer := &fakeWriter{}
	ag := NewAggregator(writer, nil)
	ag.Record("abc", 1.5, time.Now())

	if err := ag.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if writer.writes() != 1 {
		t.Fatalf("writes = %d, want 1", writer.writes())
	}
	if ag.Pending() != 0 {
		t.Errorf("pending after shutdown: %d", ag.Pending())
	}
}

type fakeReader struct {
	keys []key.Key
}

func (l fakeReader) List(_ context.Context) ([]key.Key, error) {
	return l.keys, nil
}

func TestAggregatorBucketsUsageByDayKeyAndModel(t *testing.T) {
	writer := &fakeWriter{}
	ag := NewAggregator(writer, nil)
	day := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)

	for i := 0; i < 3; i++ {
		ag.RecordUsage(UsageEvent{Hash: "abc", Model: "openai/gpt-4o", Provider: "openai", Prompt: 10, Completion: 2, Cost: 0.1, Success: true, At: day})
	}
	ag.RecordUsage(UsageEvent{Hash: "abc", Model: "openai/gpt-4o", Provider: "openai", Success: false, At: day.Add(time.Hour)})
	ag.RecordUsage(UsageEvent{Hash: "abc", Model: "openai/gpt-4o-mini", Provider: "openai", Prompt: 1, Success: true, At: day})
	ag.RecordUsage(UsageEvent{Hash: "abc", Model: "openai/gpt-4o", Provider: "openai", Prompt: 1, Success: true, At: day.Add(-24 * time.Hour)})

	if err := ag.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(writer.usage) != 1 || len(writer.usage[0]) != 3 {
		t.Fatalf("usage batches = %+v, want one batch of 3 buckets", writer.usage)
	}
	var found bool
	for _, d := range writer.usage[0] {
		if d.Model != "openai/gpt-4o" || !d.Date.Equal(time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)) {
			continue
		}
		found = true
		if d.Requests != 4 || d.Successes != 3 || d.Failures != 1 || d.PromptTokens != 30 || d.CompletionTokens != 6 {
			t.Errorf("unexpected bucket: %+v", d)
		}
	}
	if !found {
		t.Fatal("bucket for 2026-09-30 openai/gpt-4o missing")
	}
}

func TestAggregatorKeepsUsageWhenWriteFails(t *testing.T) {
	writer := &fakeWriter{usageErr: errors.New("db down")}
	ag := NewAggregator(writer, nil)
	at := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	ag.RecordUsage(UsageEvent{Hash: "abc", Model: "m", Provider: "p", Prompt: 5, Success: true, At: at})

	if err := ag.Flush(context.Background()); err == nil {
		t.Fatal("expected flush error")
	}
	ag.RecordUsage(UsageEvent{Hash: "abc", Model: "m", Provider: "p", Prompt: 5, Success: true, At: at})

	writer.usageErr = nil
	if err := ag.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(writer.usage) != 1 || len(writer.usage[0]) != 1 {
		t.Fatalf("usage batches = %+v", writer.usage)
	}
	if d := writer.usage[0][0]; d.Requests != 2 || d.PromptTokens != 10 {
		t.Errorf("merged bucket = %+v, want 2 requests / 10 prompt tokens", d)
	}
	if ag.Pending() != 0 {
		t.Errorf("pending = %d, want 0", ag.Pending())
	}
}
