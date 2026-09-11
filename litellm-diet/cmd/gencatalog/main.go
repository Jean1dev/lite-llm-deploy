package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/catalog"
)

func main() {
	source := flag.String("source", "", "path to the LiteLLM price map")
	out := flag.String("output", "internal/catalog/data/catalog.json", "generated catalog path")
	flag.Parse()

	if err := generate(*source, *out); err != nil {
		fmt.Fprintf(os.Stderr, "gencatalog: %v\n", err)
		os.Exit(1)
	}
}

func generate(source, out string) error {
	if source == "" {
		return fmt.Errorf("provide -source with the LiteLLM price map")
	}

	raw, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read source %q: %w", source, err)
	}

	entries, err := catalog.Prune(raw)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return fmt.Errorf("no chat models found in source")
	}

	body, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("serialize catalog: %w", err)
	}
	body = append(body, '\n')

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(out, body, 0o644); err != nil {
		return fmt.Errorf("write %q: %w", out, err)
	}

	fmt.Printf("catalog generated with %d models at %s\n", len(entries), out)
	return nil
}
