package config_test

import (
	"reflect"
	"testing"

	"github.com/hoangduc102/fintrack/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any relevant environment variables for clean test state
	t.Setenv("PORT", "")
	t.Setenv("APP_ENV", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "")

	cfg := config.Load()

	if cfg.Port != "8080" {
		t.Errorf("expected default Port '8080', got %q", cfg.Port)
	}

	if cfg.Environment != "development" {
		t.Errorf("expected default Environment 'development', got %q", cfg.Environment)
	}

	expectedOrigins := []string{"http://localhost:5173"}
	if !reflect.DeepEqual(cfg.CORSAllowedOrigins, expectedOrigins) {
		t.Errorf("expected CORSAllowedOrigins %v, got %v", expectedOrigins, cfg.CORSAllowedOrigins)
	}

	if cfg.DatabaseURL != "" {
		t.Errorf("expected empty DatabaseURL by default, got %q", cfg.DatabaseURL)
	}

	if cfg.JWTSecret != "" {
		t.Errorf("expected empty JWTSecret by default, got %q", cfg.JWTSecret)
	}
}

func TestLoad_CustomEnvironment(t *testing.T) {
	t.Setenv("PORT", "3000")
	t.Setenv("APP_ENV", "production")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:3000, https://app.fintrack.vn ")
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/fintrack")
	t.Setenv("JWT_SECRET", "super-secret-key")

	cfg := config.Load()

	if cfg.Port != "3000" {
		t.Errorf("expected Port '3000', got %q", cfg.Port)
	}

	if cfg.Environment != "production" {
		t.Errorf("expected Environment 'production', got %q", cfg.Environment)
	}

	expectedOrigins := []string{"http://localhost:3000", "https://app.fintrack.vn"}
	if !reflect.DeepEqual(cfg.CORSAllowedOrigins, expectedOrigins) {
		t.Errorf("expected CORSAllowedOrigins %v, got %v", expectedOrigins, cfg.CORSAllowedOrigins)
	}

	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/fintrack" {
		t.Errorf("expected custom DatabaseURL, got %q", cfg.DatabaseURL)
	}

	if cfg.JWTSecret != "super-secret-key" {
		t.Errorf("expected custom JWTSecret, got %q", cfg.JWTSecret)
	}
}
