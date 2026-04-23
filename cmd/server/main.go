package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	githubadapter "gthanks/internal/adapter/github"
	httpadapter "gthanks/internal/adapter/http"
	sqliteadapter "gthanks/internal/adapter/sqlite"
	"gthanks/internal/config"
	"gthanks/internal/migration"
	"gthanks/internal/usecase"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logger := config.NewLogger(cfg)
	slog.SetDefault(logger)

	db, err := sqliteadapter.Open(context.Background(), cfg.DBPath)
	if err != nil {
		logger.Error("open sqlite", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := migration.Run(db); err != nil {
		logger.Error("run migrations", "error", err)
		os.Exit(1)
	}

	store := sqliteadapter.NewStore(db)
	githubClient := githubadapter.NewClient(cfg)
	service := usecase.NewService(cfg, store, githubClient)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpadapter.NewRouter(cfg, logger, service),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.ServerReadTimeout,
		WriteTimeout:      cfg.ServerWriteTimeout,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", server.Addr, "env", cfg.AppEnv)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		if err != nil {
			logger.Error("http server failed", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}
