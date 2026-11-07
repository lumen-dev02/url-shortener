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

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/lumen-dev02/url-shortener/internal/config"
	"github.com/lumen-dev02/url-shortener/internal/handler"
	"github.com/lumen-dev02/url-shortener/internal/repository"
	"github.com/lumen-dev02/url-shortener/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		logger.Error("ping database", "error", err)
		os.Exit(1)
	}

	repo := repository.NewPostgres(pool)
	shortener := service.New(repo, cfg.BaseURL)
	api := handler.New(shortener, logger)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("HTTP server started", "addr", cfg.HTTPAddr)
		serverErrors <- server.ListenAndServe()
	}()

	stopCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server", "error", err)
			os.Exit(1)
		}
	case <-stopCtx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTP server shutdown", "error", err)
		}
	}
}
