package config

import (
	"fmt"
	"os"
	"strconv"
)

const (
	DefaultEnvironment      = "development"
	DefaultHTTPAddr         = ":8080"
	DefaultAnalysisMaxDepth = 3
	MinAnalysisMaxDepth     = 1
	MaxAnalysisMaxDepth     = 5
)

const (
	envEnvironment      = "CFT_ENV"
	envHTTPAddr         = "CFT_HTTP_ADDR"
	envDatabaseURL      = "CFT_DB_URL"
	envAnalysisURL      = "CFT_ANALYSIS_URL"
	envAnalysisMaxDepth = "CFT_ANALYSIS_MAX_DEPTH"
)

type Config struct {
	Environment      string
	HTTPAddr         string
	DatabaseURL      string
	AnalysisURL      string
	AnalysisMaxDepth int
}

func Load() (Config, error) {
	cfg := Config{
		Environment:      envOr(envEnvironment, DefaultEnvironment),
		HTTPAddr:         envOr(envHTTPAddr, DefaultHTTPAddr),
		DatabaseURL:      os.Getenv(envDatabaseURL),
		AnalysisURL:      os.Getenv(envAnalysisURL),
		AnalysisMaxDepth: DefaultAnalysisMaxDepth,
	}

	if !validEnvironment(cfg.Environment) {
		return Config{}, fmt.Errorf("%s must be one of development, test, production, got %q", envEnvironment, cfg.Environment)
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("%s is required", envDatabaseURL)
	}

	if cfg.AnalysisURL == "" {
		return Config{}, fmt.Errorf("%s is required", envAnalysisURL)
	}

	if raw := os.Getenv(envAnalysisMaxDepth); raw != "" {
		depth, err := strconv.Atoi(raw)
		if err != nil {
			return Config{}, fmt.Errorf("%s must be an integer, got %q", envAnalysisMaxDepth, raw)
		}
		cfg.AnalysisMaxDepth = depth
	}

	if cfg.AnalysisMaxDepth < MinAnalysisMaxDepth || cfg.AnalysisMaxDepth > MaxAnalysisMaxDepth {
		return Config{}, fmt.Errorf("%s must be between %d and %d, got %d", envAnalysisMaxDepth, MinAnalysisMaxDepth, MaxAnalysisMaxDepth, cfg.AnalysisMaxDepth)
	}

	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func validEnvironment(environment string) bool {
	switch environment {
	case "development", "test", "production":
		return true
	}
	return false
}
