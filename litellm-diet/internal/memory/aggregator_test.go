package memory

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/key"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/storage"
)

type fakeWriter struct {
	mu      sync.Mutex
	batches [][]storage.SpendDelta
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
