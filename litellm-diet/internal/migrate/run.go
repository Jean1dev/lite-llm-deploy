package migrate

import (
	"context"
	"fmt"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/key"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/storage"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Report struct {
	Read    int      `json:"read"`
	Created int      `json:"created"`
	Updated int      `json:"updated"`
	Skipped int      `json:"skipped"`
	Reasons []string `json:"reasons"`
	DryRun  bool     `json:"dry_run"`
}

type Options struct {
	DryRun bool
}

func Run(ctx context.Context, source, dest *pgxpool.Pool, repo *storage.Repository, opt Options) (Report, error) {
	read, reasons, err := ReadSource(ctx, source)
	if err != nil {
		return Report{}, err
	}
	rep := Report{Read: len(read) + len(reasons), Skipped: len(reasons), Reasons: reasons, DryRun: opt.DryRun}

	existing, err := indexDest(ctx, repo)
	if err != nil {
		return Report{}, err
	}

	for _, k := range read {
		_, found := existing[k.Hash]
		if opt.DryRun {
			if found {
				rep.Updated++
			} else {
				rep.Created++
			}
			continue
		}
		if err := repo.Upsert(ctx, k); err != nil {
			return Report{}, err
		}
		if found {
			rep.Updated++
		} else {
			rep.Created++
		}
	}
	return rep, nil
}

func indexDest(ctx context.Context, repo *storage.Repository) (map[string]key.Key, error) {
	list, err := repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list dest: %w", err)
	}
	out := make(map[string]key.Key, len(list))
	for _, k := range list {
		out[k.Hash] = k
	}
	return out, nil
}
