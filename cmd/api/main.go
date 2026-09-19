package main

import (
	"log/slog" // for structured logging
	"os"

	"github.com/schak04/crypto-fund-tracer/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if _, err := config.Load(); err != nil {
		return err
	}
	return nil
}
