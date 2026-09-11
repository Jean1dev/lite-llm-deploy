package migrate

import (
	"context"
	"fmt"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/key"
	"github.com/jackc/pgx/v5/pgxpool"
)

type sourceKey struct {
	Hash           string
	Alias          string
	Models         []string
	Spend          float64
	MaxBudget      *float64
	BudgetDuration string
	BudgetResetAt  *time.Time
	LastActive     *time.Time
	ExpiresAt      *time.Time
	Blocked        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Deleted        bool
}

type sourceFlags struct {
	deleted    bool
	lastActive bool
}

func ReadSource(ctx context.Context, pool *pgxpool.Pool) ([]key.Key, []string, error) {
	flags, err := inspectSource(ctx, pool)
	if err != nil {
		return nil, nil, err
	}

	rows, err := pool.Query(ctx, sourceQuery(flags))
	if err != nil {
		return nil, nil, fmt.Errorf("read source keys: %w", err)
	}
	defer rows.Close()

	out := make([]key.Key, 0, 64)
	skipped := make([]string, 0)
	for rows.Next() {
		var o sourceKey
		if err := rows.Scan(
			&o.Hash, &o.Alias, &o.Models, &o.Spend, &o.MaxBudget, &o.BudgetDuration,
			&o.BudgetResetAt, &o.LastActive, &o.ExpiresAt, &o.Blocked, &o.CreatedAt, &o.UpdatedAt, &o.Deleted,
		); err != nil {
			return nil, nil, fmt.Errorf("scan source row: %w", err)
		}
		if o.Deleted || o.Hash == "" {
			skipped = append(skipped, skipReason(o))
			continue
		}
		out = append(out, key.Key{
			Hash:           o.Hash,
			Alias:          o.Alias,
			Models:         o.Models,
			Spend:          o.Spend,
			MaxBudget:      o.MaxBudget,
			BudgetDuration: o.BudgetDuration,
			BudgetResetAt:  o.BudgetResetAt,
			LastActive:     o.LastActive,
			ExpiresAt:      o.ExpiresAt,
			Blocked:        o.Blocked,
			CreatedAt:      o.CreatedAt,
			UpdatedAt:      o.UpdatedAt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate source: %w", err)
	}
	return out, skipped, nil
}

func inspectSource(ctx context.Context, pool *pgxpool.Pool) (sourceFlags, error) {
	var flags sourceFlags
	deleted, err := sourceHasColumn(ctx, pool, "deleted")
	if err != nil {
		return sourceFlags{}, err
	}
	flags.deleted = deleted
	lastActive, err := sourceHasColumn(ctx, pool, "last_active")
	if err != nil {
		return sourceFlags{}, err
	}
	flags.lastActive = lastActive
	return flags, nil
}

func sourceHasColumn(ctx context.Context, pool *pgxpool.Pool, name string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_attribute
			WHERE attrelid = '"LiteLLM_VerificationToken"'::regclass
			  AND attname = $1
			  AND attnum > 0
			  AND NOT attisdropped
		)`, name).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("inspect source column %s: %w", name, err)
	}
	return exists, nil
}

func sourceQuery(f sourceFlags) string {
	lastActive := "updated_at"
	if f.lastActive {
		lastActive = "COALESCE(last_active, updated_at)"
	}
	deleted := "false"
	if f.deleted {
		deleted = "COALESCE(deleted, false)"
	}
	return `SELECT
	token,
	COALESCE(key_alias, ''),
	COALESCE(models, '{}'),
	COALESCE(spend, 0),
	max_budget,
	COALESCE(budget_duration, ''),
	budget_reset_at,
	` + lastActive + `,
	expires,
	COALESCE(blocked, false),
	COALESCE(created_at, now()),
	COALESCE(updated_at, now()),
	` + deleted + `
FROM "LiteLLM_VerificationToken"`
}

func skipReason(o sourceKey) string {
	if o.Hash == "" {
		return "empty hash"
	}
	return o.Hash[:min(8, len(o.Hash))] + ": deleted at source"
}
