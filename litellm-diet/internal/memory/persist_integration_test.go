//go:build integration

package memory

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/key"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/storage"
)

func testRepo(t *testing.T) (context.Context, *storage.Repository) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	pool, err := storage.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS keys`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS daily_usage`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS schema_migrations`); err != nil {
		t.Fatal(err)
	}
	if err := storage.NewMigrator(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	return ctx, storage.NewRepository(pool)
}

func TestMapReflectsChangeAfterInterval(t *testing.T) {
	ctx, repo := testRepo(t)
	now := time.Now().UTC()
	k := key.Key{Hash: "hash-refresh", Alias: "before", CreatedAt: now, UpdatedAt: now}
	if err := repo.Insert(ctx, k); err != nil {
		t.Fatalf("insert: %v", err)
	}

	m := NewMap(repo)
	if err := m.Load(ctx); err != nil {
		t.Fatalf("load: %v", err)
	}
	got, ok := m.Lookup("hash-refresh")
	if !ok || got.Alias != "before" {
		t.Fatalf("initial map = %+v", got)
	}

	k.Alias = "after"
	k.UpdatedAt = time.Now().UTC()
	if err := repo.Update(ctx, k); err != nil {
		t.Fatalf("update: %v", err)
	}

	loop, cancel := context.WithCancel(ctx)
	defer cancel()
	go m.RefreshPeriodically(loop, 50*time.Millisecond)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		got, ok = m.Lookup("hash-refresh")
		if ok && got.Alias == "after" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("database change did not reach the map within the interval")
}

func TestAggregatorPersistsOneWriteForNRequests(t *testing.T) {
	ctx, repo := testRepo(t)
	now := time.Now().UTC()
	if err := repo.Insert(ctx, key.Key{Hash: "hash-agg", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	counter := &writeCounter{inner: repo}
	m := NewMap(repo)
	if err := m.Load(ctx); err != nil {
		t.Fatalf("load: %v", err)
	}
	ag := NewAggregator(counter, m)
	at := time.Now().UTC()
	for i := 0; i < 5; i++ {
		ag.Record("hash-agg", 0.2, at)
	}
	if err := ag.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if counter.n != 1 {
		t.Fatalf("writes = %d, want 1", counter.n)
	}

	persisted, err := repo.Get(ctx, "hash-agg")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if persisted.Spend < 0.99 || persisted.Spend > 1.01 {
		t.Errorf("persisted spend = %g, want 1", persisted.Spend)
	}
	if persisted.LastActive == nil {
		t.Error("last_active not persisted")
	}
}

type writeCounter struct {
	inner *storage.Repository
	n     int
}

func (c *writeCounter) AddSpend(ctx context.Context, items []storage.SpendDelta) error {
	c.n++
	return c.inner.AddSpend(ctx, items)
}

func (c *writeCounter) AddUsage(ctx context.Context, items []storage.UsageDelta) error {
	return c.inner.AddUsage(ctx, items)
}

func TestUsageAccumulatesAcrossFlushes(t *testing.T) {
	ctx, repo := testRepo(t)
	day := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	ag := NewAggregator(repo, nil)

	for i := 0; i < 2; i++ {
		ag.RecordUsage(UsageEvent{Hash: "h1", Model: "openai/gpt-4o", Provider: "openai", Prompt: 10, Completion: 5, Cost: 0.5, Success: true, At: day.Add(time.Hour)})
		ag.RecordUsage(UsageEvent{Hash: "h1", Model: "openai/gpt-4o", Provider: "openai", Success: false, At: day.Add(2 * time.Hour)})
		if err := ag.Flush(ctx); err != nil {
			t.Fatalf("flush %d: %v", i, err)
		}
	}
	ag.RecordUsage(UsageEvent{Hash: "h2", Model: "anthropic/claude-haiku-4-5", Provider: "anthropic", Prompt: 1, Success: true, At: day.AddDate(0, 0, -10)})
	if err := ag.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	rows, err := repo.DailyUsage(ctx, day.AddDate(0, 0, -6), day, "", "")
	if err != nil {
		t.Fatalf("daily usage: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1 (out-of-range row must be excluded)", len(rows))
	}
	r := rows[0]
	if r.PromptTokens != 20 || r.CompletionTokens != 10 || r.Requests != 4 || r.Successes != 2 || r.Failures != 2 {
		t.Errorf("unexpected accumulated row: %+v", r)
	}
	if r.Spend < 0.99 || r.Spend > 1.01 {
		t.Errorf("spend = %g, want 1", r.Spend)
	}
	if !r.Date.Equal(day) {
		t.Errorf("date = %v, want %v", r.Date, day)
	}

	rows, err = repo.DailyUsage(ctx, day.AddDate(0, 0, -29), day, "h2", "")
	if err != nil {
		t.Fatalf("daily usage filtered: %v", err)
	}
	if len(rows) != 1 || rows[0].KeyHash != "h2" {
		t.Fatalf("key filter returned %+v", rows)
	}
	rows, err = repo.DailyUsage(ctx, day.AddDate(0, 0, -29), day, "", "openai/gpt-4o")
	if err != nil {
		t.Fatalf("daily usage by model: %v", err)
	}
	if len(rows) != 1 || rows[0].Model != "openai/gpt-4o" {
		t.Fatalf("model filter returned %+v", rows)
	}
}
