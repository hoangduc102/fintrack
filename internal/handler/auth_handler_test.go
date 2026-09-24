package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/handler"
	"github.com/hoangduc102/fintrack/internal/service"
)

type mockUserRepo struct {
	users map[string]*domain.User
}

func (m *mockUserRepo) CreateWithCategories(ctx context.Context, user *domain.User, categories []domain.Category) error {
	lowerEmail := strings.ToLower(user.Email)
	if _, exists := m.users[lowerEmail]; exists {
		return domain.ErrConflict
	}
	user.ID = uuid.New()
	m.users[lowerEmail] = user
	return nil
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	lowerEmail := strings.ToLower(email)
	if u, exists := m.users[lowerEmail]; exists {
		return u, nil
	}
	return nil, domain.ErrNotFound
}

func (m *mockUserRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, domain.ErrNotFound
}

func setupTestRouter() (*gin.Engine, *mockUserRepo) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	repo := &mockUserRepo{users: make(map[string]*domain.User)}
	authService := service.NewAuthService(repo)
	authHandler := handler.NewAuthHandler(authService)

	v1 := router.Group("/api/v1")
	{
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
		}
	}

	return router, repo
}

func TestAuthHandler_Register_Success(t *testing.T) {
	router, _ := setupTestRouter()

	displayName := "Test User"
	reqBody := handler.RegisterRequest{
		Email:       "test@example.com",
		Password:    "password123",
		DisplayName: &displayName,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(bodyBytes))
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

	if resp.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got %q", resp.Email)
	}
	if resp.ID == "" {
		t.Error("expected non-empty ID in response")
	}
	if resp.DisplayName == nil || *resp.DisplayName != displayName {
		t.Errorf("expected display name %q, got %v", displayName, resp.DisplayName)
	}
}

func TestAuthHandler_Register_ValidationError(t *testing.T) {
	router, _ := setupTestRouter()

	reqBody := handler.RegisterRequest{
		Email:    "invalid-email",
		Password: "short",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	var errResp struct {
		Error struct {
			Code    string            `json:"code"`
			Message string            `json:"message"`
			Fields  map[string]string `json:"fields"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Error.Code != "BAD_REQUEST" {
		t.Errorf("expected code 'BAD_REQUEST', got %q", errResp.Error.Code)
	}
}

func TestAuthHandler_Register_DuplicateEmailConflict(t *testing.T) {
	router, _ := setupTestRouter()

	reqBody := handler.RegisterRequest{
		Email:    "dup@example.com",
		Password: "password123",
	}
	bodyBytes, _ := json.Marshal(reqBody)

	// First request succeeds
	req1, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(bodyBytes))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first registration failed with status %d", rec1.Code)
	}

	// Second request with same email conflicts
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(bodyBytes))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, rec2.Code)
	}

	var errResp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	if errResp.Error.Code != "CONFLICT" {
		t.Errorf("expected code 'CONFLICT', got %q", errResp.Error.Code)
	}
}
