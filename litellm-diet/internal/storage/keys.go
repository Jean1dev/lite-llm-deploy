package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/key"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrKeyNotFound = fmt.Errorf("key not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) List(ctx context.Context) ([]key.Key, error) {
	const q = `SELECT hash, alias, models, spend, max_budget, budget_duration,
		budget_reset_at, last_active, expires_at, blocked, created_at, updated_at
		FROM keys`

	rows, err := r.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list keys: %w", err)
	}
	defer rows.Close()

	out := make([]key.Key, 0, 64)
	for rows.Next() {
		k, err := scanKey(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate keys: %w", err)
	}
	return out, nil
}

func (r *Repository) Get(ctx context.Context, hash string) (key.Key, error) {
	const q = `SELECT hash, alias, models, spend, max_budget, budget_duration,
		budget_reset_at, last_active, expires_at, blocked, created_at, updated_at
		FROM keys WHERE hash = $1`

	k, err := scanKey(r.pool.QueryRow(ctx, q, hash))
	if errors.Is(err, pgx.ErrNoRows) {
		return key.Key{}, fmt.Errorf("get key %s: %w", hash[:min(8, len(hash))], ErrKeyNotFound)
	}
	if err != nil {
		return key.Key{}, fmt.Errorf("get key %s: %w", hash[:min(8, len(hash))], err)
	}
	return k, nil
}

func (r *Repository) Insert(ctx context.Context, k key.Key) error {
	if k.Models == nil {
		k.Models = []string{}
	}
	const q = `INSERT INTO keys (
		hash, alias, models, spend, max_budget, budget_duration,
		budget_reset_at, last_active, expires_at, blocked, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`

	_, err := r.pool.Exec(ctx, q,
		k.Hash, k.Alias, k.Models, k.Spend, k.MaxBudget, k.BudgetDuration,
		k.BudgetResetAt, k.LastActive, k.ExpiresAt, k.Blocked, k.CreatedAt, k.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert key %s: %w", k.Hash[:min(8, len(k.Hash))], err)
	}
	return nil
}

func (r *Repository) Update(ctx context.Context, k key.Key) error {
	if k.Models == nil {
		k.Models = []string{}
	}
	const q = `UPDATE keys SET
		alias = $2, models = $3, spend = $4, max_budget = $5, budget_duration = $6,
		budget_reset_at = $7, last_active = $8, expires_at = $9, blocked = $10, updated_at = $11
		WHERE hash = $1`

	tag, err := r.pool.Exec(ctx, q,
		k.Hash, k.Alias, k.Models, k.Spend, k.MaxBudget, k.BudgetDuration,
		k.BudgetResetAt, k.LastActive, k.ExpiresAt, k.Blocked, k.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update key %s: %w", k.Hash[:min(8, len(k.Hash))], err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update key %s: %w", k.Hash[:min(8, len(k.Hash))], ErrKeyNotFound)
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, hash string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM keys WHERE hash = $1`, hash)
	if err != nil {
		return fmt.Errorf("delete key %s: %w", hash[:min(8, len(hash))], err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("delete key %s: %w", hash[:min(8, len(hash))], ErrKeyNotFound)
	}
	return nil
}

func (r *Repository) Upsert(ctx context.Context, k key.Key) error {
	if k.Models == nil {
		k.Models = []string{}
	}
	const q = `INSERT INTO keys (
		hash, alias, models, spend, max_budget, budget_duration,
		budget_reset_at, last_active, expires_at, blocked, created_at, updated_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	ON CONFLICT (hash) DO UPDATE SET
		alias = EXCLUDED.alias,
		models = EXCLUDED.models,
		spend = EXCLUDED.spend,
		max_budget = EXCLUDED.max_budget,
		budget_duration = EXCLUDED.budget_duration,
		budget_reset_at = EXCLUDED.budget_reset_at,
		last_active = EXCLUDED.last_active,
		expires_at = EXCLUDED.expires_at,
		blocked = EXCLUDED.blocked,
		updated_at = EXCLUDED.updated_at`

	_, err := r.pool.Exec(ctx, q,
		k.Hash, k.Alias, k.Models, k.Spend, k.MaxBudget, k.BudgetDuration,
		k.BudgetResetAt, k.LastActive, k.ExpiresAt, k.Blocked, k.CreatedAt, k.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert key %s: %w", k.Hash[:min(8, len(k.Hash))], err)
	}
	return nil
}

type SpendDelta struct {
	Hash       string
	Spend      float64
	LastActive time.Time
	ResetSpend bool
	ResetAt    *time.Time
}

func (r *Repository) AddSpend(ctx context.Context, items []SpendDelta) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin spend transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	const add = `UPDATE keys SET spend = spend + $2, last_active = $3, updated_at = now() WHERE hash = $1`
	const reset = `UPDATE keys SET spend = $2, last_active = $3, budget_reset_at = $4, updated_at = now() WHERE hash = $1`

	for _, item := range items {
		if item.ResetSpend {
			if _, err := tx.Exec(ctx, reset, item.Hash, item.Spend, item.LastActive, item.ResetAt); err != nil {
				return fmt.Errorf("reset spend of %s: %w", item.Hash[:min(8, len(item.Hash))], err)
			}
			continue
		}
		if _, err := tx.Exec(ctx, add, item.Hash, item.Spend, item.LastActive); err != nil {
			return fmt.Errorf("add spend of %s: %w", item.Hash[:min(8, len(item.Hash))], err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit spend: %w", err)
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanKey(s scanner) (key.Key, error) {
	var k key.Key
	if err := s.Scan(
		&k.Hash, &k.Alias, &k.Models, &k.Spend, &k.MaxBudget, &k.BudgetDuration,
		&k.BudgetResetAt, &k.LastActive, &k.ExpiresAt, &k.Blocked, &k.CreatedAt, &k.UpdatedAt,
	); err != nil {
		return key.Key{}, err
	}
	if k.Models == nil {
		k.Models = []string{}
	}
	return k, nil
}
