package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAppEnv             = "development"
	defaultPort               = "8080"
	defaultLogLevel           = "info"
	defaultRequestTimeout     = 60 * time.Second
	defaultServerReadTimeout  = 5 * time.Second
	defaultServerWriteTimeout = 60 * time.Second
)

type Config struct {
	AppEnv             string
	Port               string
	DBPath             string
	GitHubToken        string
	GitHubAPIBaseURL   string
	GitHubTimeout      time.Duration
	CacheTTLSingleRepo time.Duration
	CacheTTLUserOrg    time.Duration
	GitHubMaxConc      int
	LogLevel           slog.Level
	RequestTimeout     time.Duration
	ServerReadTimeout  time.Duration
	ServerWriteTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		AppEnv:             getEnv("APP_ENV", defaultAppEnv),
		Port:               getEnv("PORT", defaultPort),
		DBPath:             strings.TrimSpace(os.Getenv("DB_PATH")),
		GitHubToken:        strings.TrimSpace(os.Getenv("GITHUB_TOKEN")),
		GitHubAPIBaseURL:   getEnv("GITHUB_API_BASE_URL", "https://api.github.com"),
		GitHubTimeout:      getDuration("GITHUB_REQUEST_TIMEOUT", 10*time.Second),
		CacheTTLSingleRepo: getDuration("CACHE_TTL_SINGLE_REPO", time.Hour),
		CacheTTLUserOrg:    getDuration("CACHE_TTL_USER_ORG", 3*time.Hour),
		GitHubMaxConc:      getInt("GITHUB_MAX_CONCURRENCY", 1),
		RequestTimeout:     getDuration("REQUEST_TIMEOUT", defaultRequestTimeout),
		ServerReadTimeout:  getDuration("SERVER_READ_TIMEOUT", defaultServerReadTimeout),
		ServerWriteTimeout: getDuration("SERVER_WRITE_TIMEOUT", defaultServerWriteTimeout),
	}

	level, err := parseLogLevel(getEnv("LOG_LEVEL", defaultLogLevel))
	if err != nil {
		return Config{}, err
	}
	cfg.LogLevel = level

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.AppEnv) == "" {
		return errors.New("APP_ENV cannot be empty")
	}
	if c.DBPath == "" {
		return errors.New("DB_PATH is required")
	}

	port, err := strconv.Atoi(c.Port)
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Errorf("invalid PORT: %q", c.Port)
	}

	if c.RequestTimeout <= 0 {
		return errors.New("REQUEST_TIMEOUT must be greater than zero")
	}
	if c.ServerReadTimeout <= 0 {
		return errors.New("SERVER_READ_TIMEOUT must be greater than zero")
	}
	if c.ServerWriteTimeout <= 0 {
		return errors.New("SERVER_WRITE_TIMEOUT must be greater than zero")
	}
	if c.GitHubTimeout <= 0 {
		return errors.New("GITHUB_REQUEST_TIMEOUT must be greater than zero")
	}
	if c.CacheTTLSingleRepo <= 0 {
		return errors.New("CACHE_TTL_SINGLE_REPO must be greater than zero")
	}
	if c.CacheTTLUserOrg <= 0 {
		return errors.New("CACHE_TTL_USER_ORG must be greater than zero")
	}
	if c.GitHubMaxConc <= 0 {
		return errors.New("GITHUB_MAX_CONCURRENCY must be greater than zero")
	}

	return nil
}

func NewLogger(cfg Config) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.LogLevel}
	if cfg.AppEnv == "production" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return duration
}

func getInt(key string, fallback int) int {
	value, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}

	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func parseLogLevel(value string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid LOG_LEVEL: %q", value)
	}
}
