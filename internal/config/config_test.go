package config

import "testing"

func TestLoadAppliesDefaults(t *testing.T) {
	t.Setenv(envEnvironment, "")
	t.Setenv(envHTTPAddr, "")
	t.Setenv(envAnalysisMaxDepth, "")
	t.Setenv(envDatabaseURL, "postgres://cft:cft@localhost:5432/cft")
	t.Setenv(envAnalysisURL, "http://localhost:9090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Environment != DefaultEnvironment {
		t.Errorf("Environment = %q, want %q", cfg.Environment, DefaultEnvironment)
	}
	if cfg.HTTPAddr != DefaultHTTPAddr {
		t.Errorf("HTTPAddr = %q, want %q", cfg.HTTPAddr, DefaultHTTPAddr)
	}
	if cfg.DatabaseURL != "postgres://cft:cft@localhost:5432/cft" {
		t.Errorf("DatabaseURL = %q, want postgres://cft:cft@localhost:5432/cft", cfg.DatabaseURL)
	}
	if cfg.AnalysisURL != "http://localhost:9090" {
		t.Errorf("AnalysisURL = %q, want http://localhost:9090", cfg.AnalysisURL)
	}
	if cfg.AnalysisMaxDepth != DefaultAnalysisMaxDepth {
		t.Errorf("AnalysisMaxDepth = %d, want %d", cfg.AnalysisMaxDepth, DefaultAnalysisMaxDepth)
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv(envEnvironment, "production")
	t.Setenv(envHTTPAddr, ":9090")
	t.Setenv(envDatabaseURL, "postgres://cft@db:5432/cft")
	t.Setenv(envAnalysisURL, "http://analysis:8080")
	t.Setenv(envAnalysisMaxDepth, "5")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Environment != "production" {
		t.Errorf("Environment = %q, want production", cfg.Environment)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Errorf("HTTPAddr = %q, want :9090", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL != "postgres://cft@db:5432/cft" {
		t.Errorf("DatabaseURL = %q, want postgres://cft@db:5432/cft", cfg.DatabaseURL)
	}
	if cfg.AnalysisURL != "http://analysis:8080" {
		t.Errorf("AnalysisURL = %q, want http://analysis:8080", cfg.AnalysisURL)
	}
	if cfg.AnalysisMaxDepth != 5 {
		t.Errorf("AnalysisMaxDepth = %d, want 5", cfg.AnalysisMaxDepth)
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv(envAnalysisURL, "http://localhost:9090")
	t.Setenv(envDatabaseURL, "")

	if _, err := Load(); err == nil {
		t.Error("Load() returned nil error with missing DatabaseURL")
	}
}

func TestLoadRequiresAnalysisURL(t *testing.T) {
	t.Setenv(envDatabaseURL, "postgres://cft:cft@localhost:5432/cft")
	t.Setenv(envAnalysisURL, "")

	if _, err := Load(); err == nil {
		t.Error("Load() returned nil error with missing AnalysisURL")
	}
}

func TestLoadRejectsInvalidEnvironment(t *testing.T) {
	t.Setenv(envEnvironment, "prod")
	t.Setenv(envDatabaseURL, "postgres://cft:cft@localhost:5432/cft")
	t.Setenv(envAnalysisURL, "http://localhost:9090")

	if _, err := Load(); err == nil {
		t.Error("Load() returned nil error with invalid environment")
	}
}

func TestLoadRejectsOutOfRangeMaxDepth(t *testing.T) {
	t.Setenv(envDatabaseURL, "postgres://cft:cft@localhost:5432/cft")
	t.Setenv(envAnalysisURL, "http://localhost:9090")
	t.Setenv(envAnalysisMaxDepth, "10")

	if _, err := Load(); err == nil {
		t.Error("Load() returned nil error with out-of-range max depth")
	}
}

func TestLoadRejectsNonIntegerMaxDepth(t *testing.T) {
	t.Setenv(envDatabaseURL, "postgres://cft:cft@localhost:5432/cft")
	t.Setenv(envAnalysisURL, "http://localhost:9090")
	t.Setenv(envAnalysisMaxDepth, "deep")

	if _, err := Load(); err == nil {
		t.Error("Load() returned nil error with non-integer max depth")
	}
}
