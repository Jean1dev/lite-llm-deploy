package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/migrate"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/storage"
)

func main() {
	source := flag.String("source", os.Getenv("LITELLM_DATABASE_URL"), "LiteLLM database DSN")
	dest := flag.String("dest", os.Getenv("DATABASE_URL"), "litellm-diet database DSN")
	dry := flag.Bool("dry-run", false, "report only, do not write to dest")
	salt := flag.String("salt", os.Getenv("LITELLM_SALT_KEY"), "optional, decrypt provider credentials")
	flag.Parse()

	if err := run(*source, *dest, *salt, *dry); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

func run(sourceDSN, destDSN, salt string, dry bool) error {
	if sourceDSN == "" || destDSN == "" {
		return fmt.Errorf("provide -source and -dest")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	source, err := storage.Open(ctx, sourceDSN)
	if err != nil {
		return fmt.Errorf("source: %w", err)
	}
	defer source.Close()
	dest, err := storage.Open(ctx, destDSN)
	if err != nil {
		return fmt.Errorf("dest: %w", err)
	}
	defer dest.Close()

	if err := storage.NewMigrator(dest).Apply(ctx); err != nil {
		return err
	}

	rep, err := migrate.Run(ctx, source, dest, storage.NewRepository(dest), migrate.Options{DryRun: dry})
	if err != nil {
		return err
	}
	if salt == "" {
		rep.Reasons = append(rep.Reasons, "provider credentials not extracted: configure them manually")
	}
	out, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize report: %w", err)
	}
	fmt.Println(string(out))
	return nil
}
