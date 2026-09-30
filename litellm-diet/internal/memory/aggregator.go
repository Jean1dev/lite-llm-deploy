package memory

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/key"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/storage"
)

type SpendWriter interface {
	AddSpend(ctx context.Context, items []storage.SpendDelta) error
	AddUsage(ctx context.Context, items []storage.UsageDelta) error
}

type UsageEvent struct {
	Hash       string
	Model      string
	Provider   string
	Prompt     int
	Completion int
	Cost       float64
	Success    bool
	At         time.Time
}

type usageBucket struct {
	date  time.Time
	hash  string
	model string
}

type Aggregator struct {
	mu      sync.Mutex
	pending map[string]storage.SpendDelta
	usage   map[usageBucket]storage.UsageDelta
	writer  SpendWriter
	keys    *Map
}

func NewAggregator(writer SpendWriter, keys *Map) *Aggregator {
	return &Aggregator{
		pending: make(map[string]storage.SpendDelta),
		usage:   make(map[usageBucket]storage.UsageDelta),
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

func (a *Aggregator) RecordUsage(ev UsageEvent) {
	day := ev.At.UTC().Truncate(24 * time.Hour)
	b := usageBucket{date: day, hash: ev.Hash, model: ev.Model}

	a.mu.Lock()
	defer a.mu.Unlock()
	d := a.usage[b]
	d.Date = day
	d.KeyHash = ev.Hash
	d.Model = ev.Model
	d.Provider = ev.Provider
	d.PromptTokens += int64(ev.Prompt)
	d.CompletionTokens += int64(ev.Completion)
	d.Spend += ev.Cost
	d.Requests++
	if ev.Success {
		d.Successes++
	} else {
		d.Failures++
	}
	a.usage[b] = d
}

func (a *Aggregator) Flush(ctx context.Context) error {
	return errors.Join(a.flushSpend(ctx), a.flushUsage(ctx))
}

func (a *Aggregator) flushUsage(ctx context.Context) error {
	a.mu.Lock()
	if len(a.usage) == 0 {
		a.mu.Unlock()
		return nil
	}
	items := make([]storage.UsageDelta, 0, len(a.usage))
	for _, d := range a.usage {
		items = append(items, d)
	}
	a.usage = make(map[usageBucket]storage.UsageDelta, len(items))
	a.mu.Unlock()

	if err := a.writer.AddUsage(ctx, items); err != nil {
		a.mu.Lock()
		for _, item := range items {
			b := usageBucket{date: item.Date, hash: item.KeyHash, model: item.Model}
			cur := a.usage[b]
			cur.Date = item.Date
			cur.KeyHash = item.KeyHash
			cur.Model = item.Model
			cur.Provider = item.Provider
			cur.PromptTokens += item.PromptTokens
			cur.CompletionTokens += item.CompletionTokens
			cur.Spend += item.Spend
			cur.Requests += item.Requests
			cur.Successes += item.Successes
			cur.Failures += item.Failures
			a.usage[b] = cur
		}
		a.mu.Unlock()
		return err
	}
	return nil
}

func (a *Aggregator) flushSpend(ctx context.Context) error {
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
	return len(a.pending) + len(a.usage)
}
