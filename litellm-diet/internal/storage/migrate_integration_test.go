//go:build integration

package storage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func openTest(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	pool, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(pool.Close)
	return ctx, pool
}

func resetSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS keys CASCADE`); err != nil {
		t.Fatalf("drop keys: %v", err)
	}
	if _, err := pool.Exec(ctx, `DROP TABLE IF EXISTS schema_migrations CASCADE`); err != nil {
		t.Fatalf("drop schema_migrations: %v", err)
	}
}

func TestMigrationAppliesAndRollsBackOnEmptyDatabase(t *testing.T) {
	ctx, pool := openTest(t)
	resetSchema(t, ctx, pool)

	m := NewMigrator(pool)

	if err := m.Apply(ctx); err != nil {
		t.Fatalf("apply: %v", err)
	}
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM information_schema.tables WHERE table_name = 'keys'
	)`).Scan(&exists); err != nil {
		t.Fatalf("check table: %v", err)
	}
	if !exists {
		t.Fatal("keys table was not created")
	}

	if err := m.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1 FROM information_schema.tables WHERE table_name = 'keys'
	)`).Scan(&exists); err != nil {
		t.Fatalf("check table after rollback: %v", err)
	}
	if exists {
		t.Fatal("keys table remained after rollback")
	}

	if err := m.Apply(ctx); err != nil {
		t.Fatalf("reapply: %v", err)
	}
}
