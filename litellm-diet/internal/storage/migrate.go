package storage

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationsDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version INT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`

type Migrator struct {
	pool *pgxpool.Pool
	fs   fs.FS
}

func NewMigrator(pool *pgxpool.Pool) *Migrator {
	return &Migrator{pool: pool, fs: migrations.FS}
}

func (m *Migrator) Apply(ctx context.Context) error {
	if err := m.ensureTable(ctx); err != nil {
		return err
	}
	ups, err := m.files(".up.sql")
	if err != nil {
		return err
	}
	current, err := m.version(ctx)
	if err != nil {
		return err
	}
	for _, f := range ups {
		if f.version <= current {
			continue
		}
		if _, err := m.pool.Exec(ctx, f.body); err != nil {
			return fmt.Errorf("apply migration %d: %w", f.version, err)
		}
		if _, err := m.pool.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, f.version); err != nil {
			return fmt.Errorf("record migration %d: %w", f.version, err)
		}
	}
	return nil
}

func (m *Migrator) Rollback(ctx context.Context) error {
	if err := m.ensureTable(ctx); err != nil {
		return err
	}
	current, err := m.version(ctx)
	if err != nil {
		return err
	}
	if current == 0 {
		return nil
	}
	downs, err := m.files(".down.sql")
	if err != nil {
		return err
	}
	var target migrationFile
	for _, f := range downs {
		if f.version == current {
			target = f
			break
		}
	}
	if target.body == "" {
		return fmt.Errorf("down file for version %d missing", current)
	}
	if _, err := m.pool.Exec(ctx, target.body); err != nil {
		return fmt.Errorf("rollback migration %d: %w", current, err)
	}
	if _, err := m.pool.Exec(ctx, `DELETE FROM schema_migrations WHERE version = $1`, current); err != nil {
		return fmt.Errorf("forget migration %d: %w", current, err)
	}
	return nil
}

func (m *Migrator) ensureTable(ctx context.Context) error {
	if _, err := m.pool.Exec(ctx, migrationsDDL); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}
	return nil
}

func (m *Migrator) version(ctx context.Context) (int, error) {
	var current *int
	if err := m.pool.QueryRow(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&current); err != nil {
		return 0, fmt.Errorf("read migration version: %w", err)
	}
	if current == nil {
		return 0, nil
	}
	return *current, nil
}

type migrationFile struct {
	version int
	name    string
	body    string
}

func (m *Migrator) files(suffix string) ([]migrationFile, error) {
	entries, err := fs.ReadDir(m.fs, ".")
	if err != nil {
		return nil, fmt.Errorf("read embedded migrations: %w", err)
	}
	out := make([]migrationFile, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		version, err := strconv.Atoi(strings.SplitN(e.Name(), "_", 2)[0])
		if err != nil {
			return nil, fmt.Errorf("version of %s: %w", e.Name(), err)
		}
		raw, err := fs.ReadFile(m.fs, e.Name())
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", e.Name(), err)
		}
		out = append(out, migrationFile{version: version, name: e.Name(), body: string(raw)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}
