package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/schak04/crypto-fund-tracer/internal/config"
	"github.com/schak04/crypto-fund-tracer/internal/database"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

// Inits the app dependencies and performs startup checks.
// Keeping startup logic here makes main responsible only for handling the
// final success or failure of the application.
func run() error {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("loading .env: %w", err)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dbCancel()

	pool, err := database.Connect(dbCtx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	defer pool.Close()

	slog.Info("connected to database", "host", pool.Config().ConnConfig.Host, "database", pool.Config().ConnConfig.Database)

	return nil
}
