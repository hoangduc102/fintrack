package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/auth"
	"github.com/hoangduc102/fintrack/internal/config"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/handler"
	"github.com/hoangduc102/fintrack/internal/server"
	"github.com/hoangduc102/fintrack/internal/service"
)

type mockCategoryRepo struct {
	mu         sync.Mutex
	categories map[uuid.UUID]*domain.Category
}

func newMockCategoryRepo() *mockCategoryRepo {
	return &mockCategoryRepo{
		categories: make(map[uuid.UUID]*domain.Category),
	}
}

func (r *mockCategoryRepo) Create(ctx context.Context, category *domain.Category) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, c := range r.categories {
		if c.UserID == category.UserID && c.Type == category.Type && strings.EqualFold(c.Name, category.Name) {
			return domain.ErrConflict
		}
	}

	if category.ID == uuid.Nil {
		category.ID = uuid.New()
	}
	now := time.Now().UTC()
	category.CreatedAt = now
	category.UpdatedAt = now

	copied := *category
	r.categories[category.ID] = &copied
	return nil
}

func (r *mockCategoryRepo) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Category, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cat, exists := r.categories[id]
	if !exists || cat.UserID != userID {
		return nil, domain.ErrNotFound
	}
	copied := *cat
	return &copied, nil
}

func (r *mockCategoryRepo) FindByUserAndType(ctx context.Context, userID uuid.UUID, categoryType *domain.CategoryType) ([]domain.Category, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var result []domain.Category
	for _, c := range r.categories {
		if c.UserID == userID {
			if categoryType == nil || c.Type == *categoryType {
				result = append(result, *c)
			}
		}
	}
	return result, nil
}

func (r *mockCategoryRepo) Update(ctx context.Context, category *domain.Category) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cat, exists := r.categories[category.ID]
	if !exists || cat.UserID != category.UserID {
		return domain.ErrNotFound
	}

	for _, c := range r.categories {
		if c.ID != category.ID && c.UserID == category.UserID && c.Type == cat.Type && strings.EqualFold(c.Name, category.Name) {
			return domain.ErrConflict
		}
	}

	cat.Name = category.Name
	cat.UpdatedAt = time.Now().UTC()
	category.UpdatedAt = cat.UpdatedAt
	return nil
}

func (r *mockCategoryRepo) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cat, exists := r.categories[id]
	if !exists || cat.UserID != userID {
		return domain.ErrNotFound
	}
	delete(r.categories, id)
	return nil
}

func setupCategoryTestServer(catSvc *service.CategoryService, jwtMgr *auth.JWTManager) *gin.Engine {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
		JWTSecret:   "test-secret-key-12345678901234567890",
	}
	catHandler := handler.NewCategoryHandler(catSvc)
	return server.NewRouter(cfg, catHandler, jwtMgr)
}

func TestCategoryHandler_AuthMiddlewareProtection(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	repo := newMockCategoryRepo()
	catSvc := service.NewCategoryService(repo)
	router := setupCategoryTestServer(catSvc, jwtMgr)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/categories"},
		{http.MethodPost, "/api/v1/categories"},
		{http.MethodPut, fmt.Sprintf("/api/v1/categories/%s", uuid.New().String())},
		{http.MethodDelete, fmt.Sprintf("/api/v1/categories/%s", uuid.New().String())},
	}

	for _, ep := range endpoints {
		t.Run(fmt.Sprintf("%s %s requires auth", ep.method, ep.path), func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(ep.method, ep.path, nil)
			router.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 Unauthorized, got %d", w.Code)
			}
		})
	}
}

func TestCategoryHandler_Create(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	repo := newMockCategoryRepo()
	catSvc := service.NewCategoryService(repo)
	router := setupCategoryTestServer(catSvc, jwtMgr)

	userID := uuid.New()
	token, _ := jwtMgr.Generate(userID)

	t.Run("success creates category", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name": "Groceries",
			"type": "EXPENSE",
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/categories", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
		}

		var res handler.CategoryResponse
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}
		if res.Name != "Groceries" {
			t.Errorf("expected name Groceries, got %s", res.Name)
		}
		if res.Type != "EXPENSE" {
			t.Errorf("expected type EXPENSE, got %s", res.Type)
		}
	})

	t.Run("duplicate returns 409 Conflict", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name": "groceries",
			"type": "EXPENSE",
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/categories", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("expected 409 Conflict, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("validation failure returns 400 Bad Request", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name": "",
			"type": "EXPENSE",
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/categories", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestCategoryHandler_List(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	repo := newMockCategoryRepo()
	catSvc := service.NewCategoryService(repo)
	router := setupCategoryTestServer(catSvc, jwtMgr)

	user1 := uuid.New()
	token1, _ := jwtMgr.Generate(user1)

	// Seed categories
	_, _ = catSvc.CreateCategory(context.Background(), service.CreateCategoryInput{UserID: user1, Name: "Food", Type: domain.CategoryTypeExpense})
	_, _ = catSvc.CreateCategory(context.Background(), service.CreateCategoryInput{UserID: user1, Name: "Salary", Type: domain.CategoryTypeIncome})

	t.Run("list all returns all categories", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/categories", nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var res []handler.CategoryResponse
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to unmarshal: %v", err)
		}
		if len(res) != 2 {
			t.Errorf("expected 2 categories, got %d", len(res))
		}
	})

	t.Run("filter by type=EXPENSE", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/categories?type=EXPENSE", nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var res []handler.CategoryResponse
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if len(res) != 1 || res[0].Name != "Food" {
			t.Errorf("expected 1 Food category, got %+v", res)
		}
	})

	t.Run("filter by invalid type returns 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/categories?type=INVALID", nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

func TestCategoryHandler_Update(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	repo := newMockCategoryRepo()
	catSvc := service.NewCategoryService(repo)
	router := setupCategoryTestServer(catSvc, jwtMgr)

	user1 := uuid.New()
	token1, _ := jwtMgr.Generate(user1)

	cat, _ := catSvc.CreateCategory(context.Background(), service.CreateCategoryInput{UserID: user1, Name: "Dining", Type: domain.CategoryTypeExpense})

	t.Run("success updates category name", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name": "Dining Out",
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/categories/%s", cat.ID.String()), bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token1)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var res handler.CategoryResponse
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Name != "Dining Out" {
			t.Errorf("expected name 'Dining Out', got %s", res.Name)
		}
	})

	t.Run("not found returns 404", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name": "Whatever",
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/categories/%s", uuid.New().String()), bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token1)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", w.Code)
		}
	})

	t.Run("invalid id format returns 400", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"name": "Whatever",
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/categories/not-a-uuid", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token1)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

func TestCategoryHandler_Delete(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	repo := newMockCategoryRepo()
	catSvc := service.NewCategoryService(repo)
	router := setupCategoryTestServer(catSvc, jwtMgr)

	user1 := uuid.New()
	token1, _ := jwtMgr.Generate(user1)

	cat, _ := catSvc.CreateCategory(context.Background(), service.CreateCategoryInput{UserID: user1, Name: "Old Category", Type: domain.CategoryTypeExpense})

	t.Run("success deletes category", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/categories/%s", cat.ID.String()), nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204 No Content, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("second delete returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/categories/%s", cat.ID.String()), nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", w.Code)
		}
	})

	t.Run("invalid id returns 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/categories/invalid-uuid", nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}
