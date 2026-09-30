package main

import (
	"errors"
	"fmt"
	"log/slog" // for structured logging
	"os"

	"github.com/joho/godotenv"
	"github.com/schak04/crypto-fund-tracer/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("loading .env: %w", err)
	}

	if _, err := config.Load(); err != nil {
		return err
	}
	return nil
}
