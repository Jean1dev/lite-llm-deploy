package storage

import (
	"context"
	"fmt"
	"time"
)

type UsageDelta struct {
	Date             time.Time
	KeyHash          string
	Model            string
	Provider         string
	PromptTokens     int64
	CompletionTokens int64
	Spend            float64
	Requests         int64
	Successes        int64
	Failures         int64
}

type UsageRow = UsageDelta

func (r *Repository) AddUsage(ctx context.Context, items []UsageDelta) error {
	if len(items) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin usage transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const q = `INSERT INTO daily_usage (
		date, key_hash, model, provider, prompt_tokens, completion_tokens,
		spend, api_requests, successful_requests, failed_requests
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	ON CONFLICT (date, key_hash, model) DO UPDATE SET
		prompt_tokens = daily_usage.prompt_tokens + EXCLUDED.prompt_tokens,
		completion_tokens = daily_usage.completion_tokens + EXCLUDED.completion_tokens,
		spend = daily_usage.spend + EXCLUDED.spend,
		api_requests = daily_usage.api_requests + EXCLUDED.api_requests,
		successful_requests = daily_usage.successful_requests + EXCLUDED.successful_requests,
		failed_requests = daily_usage.failed_requests + EXCLUDED.failed_requests`

	for _, item := range items {
		if _, err := tx.Exec(ctx, q,
			item.Date, item.KeyHash, item.Model, item.Provider, item.PromptTokens, item.CompletionTokens,
			item.Spend, item.Requests, item.Successes, item.Failures,
		); err != nil {
			return fmt.Errorf("add usage of %s: %w", item.KeyHash[:min(8, len(item.KeyHash))], err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit usage: %w", err)
	}
	return nil
}

func (r *Repository) DailyUsage(ctx context.Context, from, to time.Time, keyHash, model string) ([]UsageRow, error) {
	const q = `SELECT date, key_hash, model, provider, prompt_tokens, completion_tokens,
		spend, api_requests, successful_requests, failed_requests
		FROM daily_usage
		WHERE date BETWEEN $1 AND $2
		  AND ($3 = '' OR key_hash = $3)
		  AND ($4 = '' OR model = $4)
		ORDER BY date, key_hash, model`

	rows, err := r.pool.Query(ctx, q, from, to, keyHash, model)
	if err != nil {
		return nil, fmt.Errorf("query daily usage: %w", err)
	}
	defer rows.Close()

	out := make([]UsageRow, 0, 64)
	for rows.Next() {
		var u UsageRow
		if err := rows.Scan(
			&u.Date, &u.KeyHash, &u.Model, &u.Provider, &u.PromptTokens, &u.CompletionTokens,
			&u.Spend, &u.Requests, &u.Successes, &u.Failures,
		); err != nil {
			return nil, fmt.Errorf("scan daily usage: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily usage: %w", err)
	}
	return out, nil
}
