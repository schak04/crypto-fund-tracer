package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/schak04/crypto-fund-tracer/internal/api"
	"github.com/schak04/crypto-fund-tracer/internal/config"
	"github.com/schak04/crypto-fund-tracer/internal/database"
	"github.com/schak04/crypto-fund-tracer/internal/repository"
	"github.com/schak04/crypto-fund-tracer/internal/service"
)

func main() {
	if err := run(); err != nil {
		slog.Error("startup failed", "error", err)
		os.Exit(1)
	}
}

// inits the app dependencies, wires the composition root, and manages server lifecycle
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

	repo := repository.New(pool)
	svc := service.New(repo)
	handler := api.NewHandler(svc)

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      handler.Routes(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("starting HTTP server", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("http server failed: %w", err)
	case <-shutdownCtx.Done():
		slog.Info("shutting down HTTP server")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}
