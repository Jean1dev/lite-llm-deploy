//go:build integration

package migrate

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/storage"
)

func TestMigrationIdempotentAndDryRun(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := storage.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS keys`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS schema_migrations`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS "LiteLLM_VerificationToken"`); err != nil {
		t.Fatal(err)
	}

	dir, err := storage.MigrationsDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.NewMigrator(pool, dir).Apply(ctx); err != nil {
		t.Fatal(err)
	}

	const createSource = `
CREATE TABLE "LiteLLM_VerificationToken" (
	token TEXT PRIMARY KEY,
	key_alias TEXT,
	models TEXT[],
	spend DOUBLE PRECISION,
	max_budget DOUBLE PRECISION,
	budget_duration TEXT,
	budget_reset_at TIMESTAMPTZ,
	last_active TIMESTAMPTZ,
	expires TIMESTAMPTZ,
	blocked BOOLEAN,
	created_at TIMESTAMPTZ,
	updated_at TIMESTAMPTZ,
	deleted BOOLEAN
)`
	if _, err := pool.Exec(ctx, createSource); err != nil {
		t.Fatalf("create source: %v", err)
	}
	if _, err := pool.Exec(ctx, `
INSERT INTO "LiteLLM_VerificationToken"
(token, key_alias, models, spend, max_budget, budget_duration, budget_reset_at, last_active, expires, blocked, created_at, updated_at, deleted)
VALUES
('hash-ok', 'prod', '{openai/*}', 12.5, 100, '30d', now() + interval '10 days', now(), now() + interval '1 year', false, now(), now(), false),
('hash-del', 'removed', '{}', 0, NULL, '', NULL, NULL, NULL, false, now(), now(), true)
`); err != nil {
		t.Fatalf("insert source: %v", err)
	}

	repo := storage.NewRepository(pool)
	dry, err := Run(ctx, pool, pool, repo, Options{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	list, err := repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("dry-run wrote %d keys", len(list))
	}
	if dry.Created != 1 || dry.Skipped != 1 {
		t.Fatalf("dry-run report = %+v", dry)
	}

	first, err := Run(ctx, pool, pool, repo, Options{})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if first.Created != 1 {
		t.Fatalf("created = %d", first.Created)
	}
	second, err := Run(ctx, pool, pool, repo, Options{})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if second.Created != 0 {
		t.Fatalf("second created a duplicate: %+v", second)
	}
	list, err = repo.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Hash != "hash-ok" || list[0].Spend != 12.5 {
		t.Fatalf("dest = %+v", list)
	}
}
