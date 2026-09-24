package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/config"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/handler"
	"github.com/hoangduc102/fintrack/internal/server"
	"github.com/hoangduc102/fintrack/internal/service"
)

type fakeUserRepoForRouter struct {
	users map[string]*domain.User
}

func (f *fakeUserRepoForRouter) CreateWithCategories(ctx context.Context, user *domain.User, categories []domain.Category) error {
	user.ID = uuid.New()
	f.users[user.Email] = user
	return nil
}

func (f *fakeUserRepoForRouter) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	if u, ok := f.users[email]; ok {
		return u, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakeUserRepoForRouter) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, domain.ErrNotFound
}

func TestHealthCheck(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
	}

	router := server.NewRouter(cfg)

	req, err := http.NewRequest(http.MethodGet, "/health", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to parse JSON response: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}
}

func TestNotFound(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
	}

	router := server.NewRouter(cfg)

	req, err := http.NewRequest(http.MethodGet, "/non-existent-route", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, recorder.Code)
	}
}

func TestRegisterRoute(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
	}

	repo := &fakeUserRepoForRouter{users: make(map[string]*domain.User)}
	authService := service.NewAuthService(repo)
	authHandler := handler.NewAuthHandler(authService)

	router := server.NewRouter(cfg, authHandler)

	reqBody := `{"email":"router@example.com","password":"password123","display_name":"Router Test"}`
	req, err := http.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d. Body: %s", http.StatusCreated, recorder.Code, recorder.Body.String())
	}

	var resp handler.UserResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Email != "router@example.com" {
		t.Errorf("expected email 'router@example.com', got %q", resp.Email)
	}
	if resp.ID == "" {
		t.Error("expected non-empty ID")
	}
	if resp.DisplayName == nil || *resp.DisplayName != "Router Test" {
		t.Errorf("expected display name 'Router Test', got %v", resp.DisplayName)
	}
}
