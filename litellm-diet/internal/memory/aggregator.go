package memory

import (
	"context"
	"sync"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/key"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/storage"
)

type SpendWriter interface {
	AddSpend(ctx context.Context, items []storage.SpendDelta) error
}

type Aggregator struct {
	mu      sync.Mutex
	pending map[string]storage.SpendDelta
	writer  SpendWriter
	keys    *Map
}

func NewAggregator(writer SpendWriter, keys *Map) *Aggregator {
	return &Aggregator{
		pending: make(map[string]storage.SpendDelta),
		writer:  writer,
		keys:    keys,
	}
}

func (a *Aggregator) Record(hash string, cost float64, at time.Time) {
	a.mu.Lock()
	d := a.pending[hash]
	d.Hash = hash
	d.Spend += cost
	d.LastActive = at
	a.pending[hash] = d
	a.mu.Unlock()

	if a.keys == nil {
		return
	}
	k, ok := a.keys.Lookup(hash)
	if !ok {
		return
	}
	k = key.ApplyReset(k, at)
	k.Spend += cost
	k.LastActive = &at
	a.keys.Replace(k)
}

func (a *Aggregator) Flush(ctx context.Context) error {
	a.mu.Lock()
	if len(a.pending) == 0 {
		a.mu.Unlock()
		return nil
	}
	items := make([]storage.SpendDelta, 0, len(a.pending))
	for _, d := range a.pending {
		items = append(items, d)
	}
	a.pending = make(map[string]storage.SpendDelta, len(items))
	a.mu.Unlock()

	if err := a.writer.AddSpend(ctx, items); err != nil {
		a.mu.Lock()
		for _, item := range items {
			cur := a.pending[item.Hash]
			cur.Hash = item.Hash
			cur.Spend += item.Spend
			if item.LastActive.After(cur.LastActive) {
				cur.LastActive = item.LastActive
			}
			a.pending[item.Hash] = cur
		}
		a.mu.Unlock()
		return err
	}
	return nil
}

func (a *Aggregator) Shutdown(ctx context.Context) error {
	return a.Flush(ctx)
}

func (a *Aggregator) FlushPeriodically(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = a.Flush(ctx)
		}
	}
}

func (a *Aggregator) Pending() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.pending)
}
