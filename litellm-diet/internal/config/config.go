package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/provider"
)

const (
	DefaultMemoryLimit = "256MiB"

	defaultPort       = 4000
	defaultKeyRefresh = 10 * time.Second
	defaultSpendFlush = 5 * time.Second
)

type Config struct {
	Port               int
	LogLevel           slog.Level
	MemoryLimit        int64
	Providers          map[provider.ID]Provider
	DatabaseURL        string
	SourceDatabaseURL  string
	MasterKey          string
	KeyRefreshInterval time.Duration
	SpendFlushInterval time.Duration
}

func Load() (Config, error) {
	if err := loadDotEnv(); err != nil {
		return Config{}, err
	}

	port, err := readPort()
	if err != nil {
		return Config{}, err
	}

	level, err := readLogLevel()
	if err != nil {
		return Config{}, err
	}

	mem, err := readMemoryLimit()
	if err != nil {
		return Config{}, err
	}

	providers, err := readProviders()
	if err != nil {
		return Config{}, err
	}

	databaseURL, err := requireEnv("DATABASE_URL")
	if err != nil {
		return Config{}, err
	}
	masterKey, err := requireEnv("MASTER_KEY")
	if err != nil {
		return Config{}, err
	}
	sourceDatabaseURL := strings.TrimSpace(os.Getenv("LITELLM_DATABASE_URL"))
	keyRefresh, err := readDuration("KEY_REFRESH_INTERVAL", defaultKeyRefresh)
	if err != nil {
		return Config{}, err
	}
	spendFlush, err := readDuration("SPEND_FLUSH_INTERVAL", defaultSpendFlush)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Port:               port,
		LogLevel:           level,
		MemoryLimit:        mem,
		Providers:          providers,
		DatabaseURL:        databaseURL,
		SourceDatabaseURL:  sourceDatabaseURL,
		MasterKey:          masterKey,
		KeyRefreshInterval: keyRefresh,
		SpendFlushInterval: spendFlush,
	}, nil
}

func requireEnv(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("%s not set", name)
	}
	return value, nil
}

func readDuration(name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s %q invalid: %w", name, raw, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive", name)
	}
	return d, nil
}

func readPort() (int, error) {
	raw, ok := os.LookupEnv("PORT")
	if !ok || raw == "" {
		return defaultPort, nil
	}

	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("PORT %q invalid: %w", raw, err)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("PORT %d out of range 1-65535", port)
	}
	return port, nil
}

func readLogLevel() (slog.Level, error) {
	raw, ok := os.LookupEnv("LOG_LEVEL")
	if !ok || raw == "" {
		return slog.LevelInfo, nil
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(raw)); err != nil {
		return 0, fmt.Errorf("LOG_LEVEL %q invalid: %w", raw, err)
	}
	return level, nil
}

func readMemoryLimit() (int64, error) {
	raw, ok := os.LookupEnv("MEMORY_LIMIT")
	if !ok || raw == "" {
		raw = DefaultMemoryLimit
	}

	limit, err := ParseSize(raw)
	if err != nil {
		return 0, fmt.Errorf("MEMORY_LIMIT: %w", err)
	}
	return limit, nil
}
