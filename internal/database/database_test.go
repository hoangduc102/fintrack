package database_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hoangduc102/fintrack/internal/database"
)

func TestDefaultConfig(t *testing.T) {
	rawURL := "postgres://fintrack:fintrack_secret@localhost:5432/fintrack?sslmode=disable"
	cfg := database.DefaultConfig(rawURL)

	if cfg.URL != rawURL {
		t.Errorf("expected URL %q, got %q", rawURL, cfg.URL)
	}
	if cfg.MaxOpenConns != 25 {
		t.Errorf("expected MaxOpenConns 25, got %d", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 25 {
		t.Errorf("expected MaxIdleConns 25, got %d", cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime != 15*time.Minute {
		t.Errorf("expected ConnMaxLifetime 15m, got %v", cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime != 5*time.Minute {
		t.Errorf("expected ConnMaxIdleTime 5m, got %v", cfg.ConnMaxIdleTime)
	}
}

func TestOpen_EmptyURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := database.Open(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty database URL, got nil")
	}
}

func TestOpen_UnreachableHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Connecting to an unreachable port should fail PingContext within timeout
	_, err := database.Open(ctx, "postgres://invalid:invalid@127.0.0.1:54321/fintrack?sslmode=disable&connect_timeout=1")
	if err == nil {
		t.Fatal("expected error connecting to unreachable database, got nil")
	}
}

func TestOpen_LiveDatabase(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("skipping live database test: DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := database.Open(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}

	stats := db.Stats()
	if stats.MaxOpenConnections != 25 {
		t.Errorf("expected MaxOpenConnections 25, got %d", stats.MaxOpenConnections)
	}
}
