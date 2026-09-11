package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/catalog"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/config"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/httpapi"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/memory"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/migrate"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		newLogger(slog.LevelInfo, os.Stdout).Error("invalid configuration: "+err.Error(), slog.String("err", err.Error()))
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel, os.Stdout)
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg, logger); err != nil {
		logger.Error("exiting after failure: "+err.Error(), slog.String("err", err.Error()))
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	debug.SetMemoryLimit(cfg.MemoryLimit)

	logger.Info("starting server",
		slog.Int("port", cfg.Port),
		slog.String("log_level", cfg.LogLevel.String()),
		slog.Int64("memory_limit_bytes", debug.SetMemoryLimit(-1)),
	)

	cat, err := catalog.LoadEmbedded()
	if err != nil {
		return err
	}

	pool, err := storage.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := storage.NewMigrator(pool).Apply(ctx); err != nil {
		return err
	}

	repo := storage.NewRepository(pool)
	keys := memory.NewMap(repo)
	if err := keys.Load(ctx); err != nil {
		return err
	}
	ag := memory.NewAggregator(repo, keys)

	go keys.RefreshPeriodically(ctx, cfg.KeyRefreshInterval)
	go ag.FlushPeriodically(ctx, cfg.SpendFlushInterval)

	srv, err := httpapi.NewServer(httpapi.Dependencies{
		Port:              cfg.Port,
		MasterKey:         cfg.MasterKey,
		Providers:         cfg.Providers,
		Catalog:           cat,
		Map:               keys,
		Aggregator:        ag,
		Repo:              repo,
		Ready:             func(c context.Context) error { return pool.Ping(c) },
		Log:               logger,
		SourceDatabaseURL: cfg.SourceDatabaseURL,
		Importer:          migrate.Importer{SourceDSN: cfg.SourceDatabaseURL, Dest: pool, Repo: repo},
	})
	if err != nil {
		return err
	}

	if err := srv.Run(ctx); err != nil {
		return err
	}

	logger.Info("server stopped")
	return nil
}

func newLogger(level slog.Level, out io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(out, &slog.HandlerOptions{
		Level:     level,
		AddSource: level <= slog.LevelDebug,
	}))
}
