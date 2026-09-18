package config

import (
	"os"
	"strings"
)

// Config holds all configuration settings for the application.
type Config struct {
	Port               string
	Environment        string
	CORSAllowedOrigins []string
	DatabaseURL        string
	JWTSecret          string
}

// Load populates and returns a Config struct from environment variables,
// falling back to sensible defaults where applicable.
func Load() *Config {
	return &Config{
		Port:               getEnv("PORT", "8080"),
		Environment:        getEnv("APP_ENV", "development"),
		CORSAllowedOrigins: parseCommaSeparated(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
	}
}

// getEnv returns the environment variable value for the given key,
// or fallback if the key is unset or empty.
func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

// parseCommaSeparated splits a comma-delimited string and trims whitespace from each element.
func parseCommaSeparated(val string) []string {
	if strings.TrimSpace(val) == "" {
		return nil
	}

	parts := strings.Split(val, ",")
	var result []string
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
