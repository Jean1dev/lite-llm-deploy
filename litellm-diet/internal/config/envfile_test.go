package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyEnvFileDoesNotOverrideExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("OPENAI_API_KEY=from-file\n# comment\nMASTER_KEY=from-file\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENAI_API_KEY", "from-env")
	os.Unsetenv("MASTER_KEY")

	if err := applyEnvFile(path); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := os.Getenv("OPENAI_API_KEY"); got != "from-env" {
		t.Errorf("openai key = %q, want from-env", got)
	}
	if got := os.Getenv("MASTER_KEY"); got != "from-file" {
		t.Errorf("master key = %q, want from-file", got)
	}
}

func TestApplyEnvFileMissingIsError(t *testing.T) {
	if err := applyEnvFile(filepath.Join(t.TempDir(), "missing.env")); !os.IsNotExist(err) {
		t.Fatalf("err = %v, want not exist", err)
	}
}
