package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hoangduc102/fintrack/internal/config"
	"github.com/hoangduc102/fintrack/internal/database"
	"github.com/hoangduc102/fintrack/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var db *sql.DB
	if cfg.DatabaseURL != "" {
		dbCtx, dbCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer dbCancel()

		var err error
		db, err = database.Open(dbCtx, cfg.DatabaseURL)
		if err != nil {
			slog.Error("failed to connect to database", "error", err)
			os.Exit(1)
		}
		slog.Info("connected to database successfully")
	}

	var opts []server.Option
	if db != nil {
		opts = append(opts, server.WithDB(db))
	}

	srv := server.New(cfg, opts...)

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
