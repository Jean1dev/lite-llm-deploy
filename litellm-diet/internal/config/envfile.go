package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func loadDotEnv() error {
	for _, path := range []string{".env", "litellm-diet/.env"} {
		if err := applyEnvFile(path); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func applyEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, set := os.LookupEnv(key); set {
			continue
		}
		if err := os.Setenv(key, unquoteEnv(strings.TrimSpace(value))); err != nil {
			return fmt.Errorf("set %s from %s: %w", key, path, err)
		}
	}
	return sc.Err()
}

func unquoteEnv(value string) string {
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}
