package memory

import (
	"context"
	"sync"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/key"
)

type KeyReader interface {
	List(ctx context.Context) ([]key.Key, error)
}

type Map struct {
	mu     sync.RWMutex
	byHash map[string]key.Key
	reader KeyReader
}

func NewMap(reader KeyReader) *Map {
	return &Map{
		byHash: make(map[string]key.Key),
		reader: reader,
	}
}

func (m *Map) Load(ctx context.Context) error {
	list, err := m.reader.List(ctx)
	if err != nil {
		return err
	}

	next := make(map[string]key.Key, len(list))
	for _, k := range list {
		next[k.Hash] = k
	}

	m.mu.Lock()
	m.byHash = next
	m.mu.Unlock()
	return nil
}

func (m *Map) Lookup(hash string) (key.Key, bool) {
	m.mu.RLock()
	k, ok := m.byHash[hash]
	m.mu.RUnlock()
	return k, ok
}

func (m *Map) Replace(k key.Key) {
	m.mu.Lock()
	m.byHash[k.Hash] = k
	m.mu.Unlock()
}

func (m *Map) Remove(hash string) {
	m.mu.Lock()
	delete(m.byHash, hash)
	m.mu.Unlock()
}

func (m *Map) All() []key.Key {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]key.Key, 0, len(m.byHash))
	for _, k := range m.byHash {
		out = append(out, k)
	}
	return out
}

func (m *Map) RefreshPeriodically(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			_ = m.Load(ctx)
		}
	}
}
