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

func ReadSource(ctx context.Context, pool *pgxpool.Pool) ([]key.Key, []string, error) {
	const q = `
SELECT
	token,
	COALESCE(key_alias, ''),
	COALESCE(models, '{}'),
	COALESCE(spend, 0),
	max_budget,
	COALESCE(budget_duration, ''),
	budget_reset_at,
	COALESCE(last_active, updated_at),
	expires,
	COALESCE(blocked, false),
	COALESCE(created_at, now()),
	COALESCE(updated_at, now()),
	COALESCE(deleted, false)
FROM "LiteLLM_VerificationToken"`

	rows, err := pool.Query(ctx, q)
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

func skipReason(o sourceKey) string {
	if o.Hash == "" {
		return "empty hash"
	}
	return o.Hash[:min(8, len(o.Hash))] + ": deleted at source"
}
