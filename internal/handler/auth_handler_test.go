package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/auth"
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
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
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

func setupTestRouter() (*gin.Engine, *mockUserRepo, *auth.JWTManager) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	repo := &mockUserRepo{users: make(map[string]*domain.User)}
	jwtManager := auth.NewJWTManager("test-secret", 24*time.Hour)
	authService := service.NewAuthService(repo, jwtManager)
	authHandler := handler.NewAuthHandler(authService)

	v1 := router.Group("/api/v1")
	{
		authGroup := v1.Group("/auth")
		{
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
			authGroup.GET("/me", handler.AuthMiddleware(jwtManager), authHandler.Me)
		}
	}

	return router, repo, jwtManager
}

func TestAuthHandler_Register_Success(t *testing.T) {
	router, _, _ := setupTestRouter()

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
	router, _, _ := setupTestRouter()

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
	router, _, _ := setupTestRouter()

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

func TestAuthHandler_Login_Success(t *testing.T) {
	router, _, _ := setupTestRouter()

	// 1. Register user
	regBody := handler.RegisterRequest{
		Email:    "loginuser@example.com",
		Password: "password123",
	}
	regBytes, _ := json.Marshal(regBody)
	regReq, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regBytes))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	router.ServeHTTP(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("failed to register test user: %d", regRec.Code)
	}

	// 2. Login
	loginBody := handler.LoginRequest{
		Email:    "loginuser@example.com",
		Password: "password123",
	}
	loginBytes, _ := json.Marshal(loginBody)
	loginReq, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBytes))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", loginRec.Code, loginRec.Body.String())
	}

	var loginResp handler.LoginResponse
	if err := json.Unmarshal(loginRec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	if loginResp.Token == "" {
		t.Error("expected non-empty JWT token in login response")
	}
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	router, _, _ := setupTestRouter()

	// Register user
	regBody := handler.RegisterRequest{
		Email:    "valid@example.com",
		Password: "correctpassword",
	}
	regBytes, _ := json.Marshal(regBody)
	regReq, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regBytes))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	router.ServeHTTP(regRec, regReq)

	// Attempt login with wrong password
	loginBody := handler.LoginRequest{
		Email:    "valid@example.com",
		Password: "wrongpassword",
	}
	loginBytes, _ := json.Marshal(loginBody)
	loginReq, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBytes))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	if loginRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, loginRec.Code)
	}

	var errResp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(loginRec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if errResp.Error.Code != "UNAUTHORIZED" {
		t.Errorf("expected code 'UNAUTHORIZED', got %q", errResp.Error.Code)
	}
}

func TestAuthMiddleware_MissingOrInvalidToken(t *testing.T) {
	router, _, jwtManager := setupTestRouter()

	t.Run("missing authorization header", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
		}
	})

	t.Run("invalid header format without bearer", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Basic some-token")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
		}
	})

	t.Run("expired token returns 401", func(t *testing.T) {
		expiredManager := auth.NewJWTManager("test-secret", -1*time.Minute)
		expiredToken, _ := expiredManager.Generate(uuid.New())

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+expiredToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
		}
	})

	t.Run("tampered token returns 401", func(t *testing.T) {
		validToken, _ := jwtManager.Generate(uuid.New())
		tamperedToken := validToken + "tampered"

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.Header.Set("Authorization", "Bearer "+tamperedToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rec.Code)
		}
	})
}

func TestAuthHandler_Me_Success(t *testing.T) {
	router, _, _ := setupTestRouter()

	// 1. Register user
	displayName := "Me Tester"
	regBody := handler.RegisterRequest{
		Email:       "me@example.com",
		Password:    "password123",
		DisplayName: &displayName,
	}
	regBytes, _ := json.Marshal(regBody)
	regReq, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(regBytes))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	router.ServeHTTP(regRec, regReq)

	// 2. Login to get token
	loginBody := handler.LoginRequest{
		Email:    "me@example.com",
		Password: "password123",
	}
	loginBytes, _ := json.Marshal(loginBody)
	loginReq, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBytes))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)

	var loginResp handler.LoginResponse
	_ = json.Unmarshal(loginRec.Body.Bytes(), &loginResp)

	// 3. Call GET /api/v1/auth/me with Bearer token
	meReq, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginResp.Token)
	meRec := httptest.NewRecorder()
	router.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", meRec.Code, meRec.Body.String())
	}

	var userResp handler.UserResponse
	if err := json.Unmarshal(meRec.Body.Bytes(), &userResp); err != nil {
		t.Fatalf("failed to decode me response: %v", err)
	}

	if userResp.Email != "me@example.com" {
		t.Errorf("expected email 'me@example.com', got %q", userResp.Email)
	}
	if userResp.DisplayName == nil || *userResp.DisplayName != displayName {
		t.Errorf("expected display name %q, got %v", displayName, userResp.DisplayName)
	}
}
