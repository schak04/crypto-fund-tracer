package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultMaxConns          = 25
	defaultMinConns          = 2
	defaultMaxConnLifetime   = 1 * time.Hour
	defaultMaxConnIdleTime   = 15 * time.Minute
	defaultHealthCheckPeriod = 1 * time.Minute
)

type Config struct {
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

func DefaultConfig() Config {
	return Config{
		MaxConns:          defaultMaxConns,
		MinConns:          defaultMinConns,
		MaxConnLifetime:   defaultMaxConnLifetime,
		MaxConnIdleTime:   defaultMaxConnIdleTime,
		HealthCheckPeriod: defaultHealthCheckPeriod,
	}
}

// Parses the PostgreSQL DSN, configures a connection pool,
// verifies database connectivity, and returns a ready-to-use pool.
// This ensures callers receive a pool that has already passed a connectivity check.
func Connect(ctx context.Context, dsn string, opts ...Config) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parsing database connection string: %w", err)
	}

	opt := DefaultConfig()
	if len(opts) > 0 {
		opt = opts[0]
	}

	if opt.MaxConns > 0 {
		cfg.MaxConns = opt.MaxConns
	}
	if opt.MinConns > 0 {
		cfg.MinConns = opt.MinConns
	}
	if opt.MaxConnLifetime > 0 {
		cfg.MaxConnLifetime = opt.MaxConnLifetime
	}
	if opt.MaxConnIdleTime > 0 {
		cfg.MaxConnIdleTime = opt.MaxConnIdleTime
	}
	if opt.HealthCheckPeriod > 0 {
		cfg.HealthCheckPeriod = opt.HealthCheckPeriod
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("creating database pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return pool, nil
}
