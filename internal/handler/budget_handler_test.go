package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

type mockBudgetRepo struct {
	mu      sync.Mutex
	budgets map[uuid.UUID]*domain.Budget
	spent   map[string]int64
}

func newMockBudgetRepo() *mockBudgetRepo {
	return &mockBudgetRepo{
		budgets: make(map[uuid.UUID]*domain.Budget),
		spent:   make(map[string]int64),
	}
}

func (r *mockBudgetRepo) Upsert(ctx context.Context, budget *domain.Budget) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.budgets {
		if existing.UserID == budget.UserID &&
			existing.CategoryID == budget.CategoryID &&
			existing.Year == budget.Year &&
			existing.Month == budget.Month {
			existing.Amount = budget.Amount
			existing.UpdatedAt = time.Now().UTC()
			budget.ID = existing.ID
			budget.CreatedAt = existing.CreatedAt
			budget.UpdatedAt = existing.UpdatedAt
			return nil
		}
	}

	if budget.ID == uuid.Nil {
		budget.ID = uuid.New()
	}
	now := time.Now().UTC()
	budget.CreatedAt = now
	budget.UpdatedAt = now
	copied := *budget
	r.budgets[budget.ID] = &copied
	return nil
}

func (r *mockBudgetRepo) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Budget, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	b, exists := r.budgets[id]
	if !exists || b.UserID != userID {
		return nil, domain.ErrNotFound
	}
	copied := *b
	return &copied, nil
}

func (r *mockBudgetRepo) FindByCategoryAndMonth(ctx context.Context, userID uuid.UUID, categoryID uuid.UUID, year int, month int) (*domain.Budget, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, b := range r.budgets {
		if b.UserID == userID && b.CategoryID == categoryID && b.Year == year && b.Month == month {
			copied := *b
			return &copied, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *mockBudgetRepo) ListByMonth(ctx context.Context, userID uuid.UUID, year int, month int) ([]domain.Budget, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var result []domain.Budget
	for _, b := range r.budgets {
		if b.UserID == userID && b.Year == year && b.Month == month {
			copied := *b
			key := fmt.Sprintf("%s:%d:%d", b.CategoryID, year, month)
			copied.Spent = r.spent[key]
			result = append(result, copied)
		}
	}
	return result, nil
}

func (r *mockBudgetRepo) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	b, exists := r.budgets[id]
	if !exists || b.UserID != userID {
		return domain.ErrNotFound
	}
	delete(r.budgets, id)
	return nil
}

func (r *mockBudgetRepo) GetSpent(ctx context.Context, userID uuid.UUID, categoryID uuid.UUID, year int, month int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%d:%d", categoryID, year, month)
	return r.spent[key], nil
}

func setupBudgetTestServer(budgetSvc *service.BudgetService, jwtMgr *auth.JWTManager) *gin.Engine {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
		JWTSecret:   "test-secret-key-12345678901234567890",
	}
	budgetHandler := handler.NewBudgetHandler(budgetSvc)
	return server.NewRouter(cfg, budgetHandler, jwtMgr)
}

func TestBudgetHandler_AuthProtection(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	budgetRepo := newMockBudgetRepo()
	catRepo := newMockCategoryRepo()
	budgetSvc := service.NewBudgetService(budgetRepo, catRepo)
	router := setupBudgetTestServer(budgetSvc, jwtMgr)

	randomID := uuid.New().String()
	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/api/v1/budgets"},
		{http.MethodGet, "/api/v1/budgets?month=2026-09"},
		{http.MethodDelete, "/api/v1/budgets/" + randomID},
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

func TestBudgetHandler_Upsert(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	budgetRepo := newMockBudgetRepo()
	catRepo := newMockCategoryRepo()
	budgetSvc := service.NewBudgetService(budgetRepo, catRepo)
	router := setupBudgetTestServer(budgetSvc, jwtMgr)

	user1 := uuid.New()
	token1, _ := jwtMgr.Generate(user1)

	expenseCat := &domain.Category{UserID: user1, Name: "Groceries", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(context.Background(), expenseCat)

	incomeCat := &domain.Category{UserID: user1, Name: "Bonus", Type: domain.CategoryTypeIncome}
	_ = catRepo.Create(context.Background(), incomeCat)

	t.Run("success creates budget on expense category", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"category_id": expenseCat.ID.String(),
			"amount":      5000000,
			"year":        2026,
			"month":       9,
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/budgets", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token1)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var res handler.BudgetResponse
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if res.Amount != 5000000 {
			t.Errorf("expected amount 5000000, got %d", res.Amount)
		}
		if res.CategoryID != expenseCat.ID.String() {
			t.Errorf("expected category_id %s, got %s", expenseCat.ID.String(), res.CategoryID)
		}
		if res.Year != 2026 || res.Month != 9 {
			t.Errorf("expected 2026-09, got %d-%02d", res.Year, res.Month)
		}
	})

	t.Run("supports YYYY-MM month string in request body", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"category_id": expenseCat.ID.String(),
			"amount":      6000000,
			"month":       "2026-10",
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/budgets", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token1)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var res handler.BudgetResponse
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Year != 2026 || res.Month != 10 {
			t.Errorf("expected 2026-10, got %d-%02d", res.Year, res.Month)
		}
	})

	t.Run("rejects budget on income category with 400 Bad Request", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"category_id": incomeCat.ID.String(),
			"amount":      10000000,
			"year":        2026,
			"month":       9,
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/budgets", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token1)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
		}

		var errRes map[string]interface{}
		_ = json.Unmarshal(w.Body.Bytes(), &errRes)
		errObj := errRes["error"].(map[string]interface{})
		if errObj["code"] != "VALIDATION_ERROR" {
			t.Errorf("expected VALIDATION_ERROR, got %v", errObj["code"])
		}
	})

	t.Run("validation error on negative amount returns 400", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"category_id": expenseCat.ID.String(),
			"amount":      -50000,
			"year":        2026,
			"month":       9,
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/budgets", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token1)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("invalid category_id UUID returns 400", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"category_id": "not-a-uuid",
			"amount":      100000,
			"year":        2026,
			"month":       9,
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/budgets", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token1)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

func TestBudgetHandler_List(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	budgetRepo := newMockBudgetRepo()
	catRepo := newMockCategoryRepo()
	budgetSvc := service.NewBudgetService(budgetRepo, catRepo)
	router := setupBudgetTestServer(budgetSvc, jwtMgr)

	user := uuid.New()
	token, _ := jwtMgr.Generate(user)

	cat := &domain.Category{UserID: user, Name: "Dining", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(context.Background(), cat)

	_, _ = budgetSvc.UpsertBudget(context.Background(), service.UpsertBudgetInput{
		UserID:     user,
		CategoryID: cat.ID,
		Amount:     3000000,
		Year:       2026,
		Month:      9,
	})

	t.Run("lists budgets with valid month", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/budgets?month=2026-09", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var res []handler.BudgetResponse
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(res) != 1 {
			t.Errorf("expected 1 budget, got %d", len(res))
		}
		if res[0].Amount != 3000000 {
			t.Errorf("expected amount 3000000, got %d", res[0].Amount)
		}
	})

	t.Run("missing month parameter returns 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/budgets", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})

	t.Run("malformed month parameter returns 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/budgets?month=2026/09", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

func TestBudgetHandler_Delete(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	budgetRepo := newMockBudgetRepo()
	catRepo := newMockCategoryRepo()
	budgetSvc := service.NewBudgetService(budgetRepo, catRepo)
	router := setupBudgetTestServer(budgetSvc, jwtMgr)

	user := uuid.New()
	token, _ := jwtMgr.Generate(user)

	cat := &domain.Category{UserID: user, Name: "Dining", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(context.Background(), cat)

	b, _ := budgetSvc.UpsertBudget(context.Background(), service.UpsertBudgetInput{
		UserID:     user,
		CategoryID: cat.ID,
		Amount:     3000000,
		Year:       2026,
		Month:      9,
	})

	t.Run("owner deletes budget successfully", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/budgets/"+b.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204 No Content, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("second delete returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/budgets/"+b.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", w.Code)
		}
	})

	t.Run("invalid id format returns 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/budgets/invalid-id", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}
