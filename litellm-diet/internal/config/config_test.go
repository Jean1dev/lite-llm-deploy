package config

import (
	"log/slog"
	"testing"
)

func TestParseSize(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		want    int64
		wantErr bool
	}{
		{"bytes no suffix", "1024", 1024, false},
		{"suffix B", "512B", 512, false},
		{"suffix KiB", "8KiB", 8 << 10, false},
		{"suffix MiB", "256MiB", 256 << 20, false},
		{"suffix GiB", "2GiB", 2 << 30, false},
		{"surrounding spaces", "  128MiB  ", 128 << 20, false},
		{"empty", "", 0, true},
		{"negative", "-1MiB", 0, true},
		{"zero", "0MiB", 0, true},
		{"unknown suffix", "10PB", 0, true},
		{"no number", "MiB", 0, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseSize(c.in)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %d", c.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("size = %d, want %d", got, c.want)
			}
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	minEnv(t)

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Port != defaultPort {
		t.Errorf("port = %d, want %d", c.Port, defaultPort)
	}
	if c.LogLevel != slog.LevelInfo {
		t.Errorf("level = %v, want %v", c.LogLevel, slog.LevelInfo)
	}
	if c.MemoryLimit != 256<<20 {
		t.Errorf("memory = %d, want %d", c.MemoryLimit, 256<<20)
	}
}

func TestLoadEnvValues(t *testing.T) {
	minEnv(t)
	t.Setenv("PORT", "8080")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("MEMORY_LIMIT", "512MiB")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Port != 8080 {
		t.Errorf("port = %d, want 8080", c.Port)
	}
	if c.LogLevel != slog.LevelDebug {
		t.Errorf("level = %v, want %v", c.LogLevel, slog.LevelDebug)
	}
	if c.MemoryLimit != 512<<20 {
		t.Errorf("memory = %d, want %d", c.MemoryLimit, 512<<20)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name  string
		vars  map[string]string
		field string
	}{
		{"non numeric port", map[string]string{"PORT": "abc"}, "PORT"},
		{"port out of range", map[string]string{"PORT": "70000"}, "PORT"},
		{"unknown level", map[string]string{"LOG_LEVEL": "verbose"}, "LOG_LEVEL"},
		{"invalid memory", map[string]string{"MEMORY_LIMIT": "lots"}, "MEMORY_LIMIT"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			minEnv(t)
			for k, v := range c.vars {
				t.Setenv(k, v)
			}

			if _, err := Load(); err == nil {
				t.Fatalf("expected error mentioning %s", c.field)
			}
		})
	}
}
