package auth_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/auth"
	"github.com/hoangduc102/fintrack/internal/domain"
)

func TestJWTManager_GenerateAndValidate(t *testing.T) {
	secret := "test-secret-key-for-jwt-signing-12345"
	manager := auth.NewJWTManager(secret, 24*time.Hour)

	userID := uuid.New()
	token, err := manager.Generate(userID)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty token string")
	}

	parsedID, err := manager.Validate(token)
	if err != nil {
		t.Fatalf("failed to validate valid token: %v", err)
	}

	if parsedID != userID {
		t.Errorf("expected parsed ID %s, got %s", userID, parsedID)
	}
}

func TestJWTManager_ExpiredToken(t *testing.T) {
	secret := "test-secret-key-for-jwt-signing-12345"
	// Create manager with negative duration to generate already-expired token
	manager := auth.NewJWTManager(secret, -1*time.Minute)

	userID := uuid.New()
	token, err := manager.Generate(userID)
	if err != nil {
		t.Fatalf("failed to generate expired token: %v", err)
	}

	_, err = manager.Validate(token)
	if err == nil {
		t.Fatal("expected error validating expired token, got nil")
	}

	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected domain.ErrUnauthorized, got: %v", err)
	}
}

func TestJWTManager_InvalidSignature(t *testing.T) {
	manager1 := auth.NewJWTManager("secret-key-one-11111111111111111111", 24*time.Hour)
	manager2 := auth.NewJWTManager("secret-key-two-22222222222222222222", 24*time.Hour)

	userID := uuid.New()
	token, err := manager1.Generate(userID)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	_, err = manager2.Validate(token)
	if err == nil {
		t.Fatal("expected error validating token signed with different key, got nil")
	}

	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected domain.ErrUnauthorized, got: %v", err)
	}
}

func TestJWTManager_MalformedToken(t *testing.T) {
	manager := auth.NewJWTManager("test-secret-key-for-jwt-signing-12345", 24*time.Hour)

	_, err := manager.Validate("not.a.valid.jwt.token")
	if err == nil {
		t.Fatal("expected error validating malformed token, got nil")
	}

	if !errors.Is(err, domain.ErrUnauthorized) {
		t.Errorf("expected domain.ErrUnauthorized, got: %v", err)
	}
}
