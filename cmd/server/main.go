package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/hoangduc102/fintrack/internal/config"
	"github.com/hoangduc102/fintrack/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := server.New(cfg)

	slog.Info("starting FinTrack HTTP server",
		"port", cfg.Port,
		"env", cfg.Environment,
	)

	if err := srv.Run(ctx); err != nil {
		slog.Error("server error during run/shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server exited cleanly")
}
