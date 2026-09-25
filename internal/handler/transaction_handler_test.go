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

type mockTransactionRepo struct {
	mu           sync.Mutex
	transactions map[uuid.UUID]*domain.Transaction
}

func newMockTransactionRepo() *mockTransactionRepo {
	return &mockTransactionRepo{
		transactions: make(map[uuid.UUID]*domain.Transaction),
	}
}

func (r *mockTransactionRepo) Create(ctx context.Context, tx *domain.Transaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tx.ID == uuid.Nil {
		tx.ID = uuid.New()
	}
	now := time.Now().UTC()
	tx.CreatedAt = now
	tx.UpdatedAt = now

	copied := *tx
	r.transactions[tx.ID] = &copied
	return nil
}

func (r *mockTransactionRepo) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Transaction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tx, exists := r.transactions[id]
	if !exists || tx.UserID != userID {
		return nil, domain.ErrNotFound
	}
	copied := *tx
	return &copied, nil
}

func (r *mockTransactionRepo) Update(ctx context.Context, tx *domain.Transaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.transactions[tx.ID]
	if !exists || existing.UserID != tx.UserID {
		return domain.ErrNotFound
	}

	tx.UpdatedAt = time.Now().UTC()
	copied := *tx
	r.transactions[tx.ID] = &copied
	return nil
}

func (r *mockTransactionRepo) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.transactions[id]
	if !exists || existing.UserID != userID {
		return domain.ErrNotFound
	}

	delete(r.transactions, id)
	return nil
}

func setupTransactionTestServer(txSvc *service.TransactionService, jwtMgr *auth.JWTManager) *gin.Engine {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
		JWTSecret:   "test-secret-key-12345678901234567890",
	}
	txHandler := handler.NewTransactionHandler(txSvc)
	return server.NewRouter(cfg, txHandler, jwtMgr)
}

func TestTransactionHandler_AuthProtection(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	txRepo := newMockTransactionRepo()
	catRepo := newMockCategoryRepo()
	txSvc := service.NewTransactionService(txRepo, catRepo)
	router := setupTransactionTestServer(txSvc, jwtMgr)

	randomID := uuid.New().String()
	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/transactions"},
		{http.MethodGet, "/api/v1/transactions/" + randomID},
		{http.MethodPut, "/api/v1/transactions/" + randomID},
		{http.MethodDelete, "/api/v1/transactions/" + randomID},
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

func TestTransactionHandler_Create(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	txRepo := newMockTransactionRepo()
	catRepo := newMockCategoryRepo()
	txSvc := service.NewTransactionService(txRepo, catRepo)
	router := setupTransactionTestServer(txSvc, jwtMgr)

	user1 := uuid.New()
	token1, _ := jwtMgr.Generate(user1)

	// Seed category for user1
	cat := &domain.Category{UserID: user1, Name: "Food", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(context.Background(), cat)

	t.Run("success creates transaction", func(t *testing.T) {
		catIDStr := cat.ID.String()
		body, _ := json.Marshal(map[string]interface{}{
			"amount":           50000,
			"type":             "EXPENSE",
			"category_id":      catIDStr,
			"transaction_date": "2026-09-25",
			"description":      "Lunch at restaurant",
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/transactions", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token1)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created, got %d: %s", w.Code, w.Body.String())
		}

		var res handler.TransactionResponse
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if res.Amount != 50000 {
			t.Errorf("expected amount 50000, got %d", res.Amount)
		}
		if res.CategoryID == nil || *res.CategoryID != catIDStr {
			t.Errorf("expected category_id %s, got %v", catIDStr, res.CategoryID)
		}
		if res.TransactionDate != "2026-09-25" {
			t.Errorf("expected transaction_date 2026-09-25, got %s", res.TransactionDate)
		}
	})

	t.Run("validation error on negative amount returns 400", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"amount":           -5000,
			"type":             "EXPENSE",
			"transaction_date": "2026-09-25",
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPost, "/api/v1/transactions", bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token1)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestTransactionHandler_GetByID(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	txRepo := newMockTransactionRepo()
	catRepo := newMockCategoryRepo()
	txSvc := service.NewTransactionService(txRepo, catRepo)
	router := setupTransactionTestServer(txSvc, jwtMgr)

	user1 := uuid.New()
	user2 := uuid.New()
	token1, _ := jwtMgr.Generate(user1)
	token2, _ := jwtMgr.Generate(user2)

	tx, _ := txSvc.CreateTransaction(context.Background(), service.CreateTransactionInput{
		UserID:          user1,
		Amount:          75000,
		Type:            domain.TransactionTypeExpense,
		TransactionDate: "2026-09-25",
	})

	t.Run("owner gets transaction successfully", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/transactions/"+tx.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var res handler.TransactionResponse
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.ID != tx.ID.String() {
			t.Errorf("expected ID %s, got %s", tx.ID.String(), res.ID)
		}
	})

	t.Run("non-owner receives 404 Not Found", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/transactions/"+tx.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+token2)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", w.Code)
		}
	})

	t.Run("invalid uuid returns 400", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/transactions/not-a-uuid", nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request, got %d", w.Code)
		}
	})
}

func TestTransactionHandler_Update(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	txRepo := newMockTransactionRepo()
	catRepo := newMockCategoryRepo()
	txSvc := service.NewTransactionService(txRepo, catRepo)
	router := setupTransactionTestServer(txSvc, jwtMgr)

	user1 := uuid.New()
	token1, _ := jwtMgr.Generate(user1)

	tx, _ := txSvc.CreateTransaction(context.Background(), service.CreateTransactionInput{
		UserID:          user1,
		Amount:          50000,
		Type:            domain.TransactionTypeExpense,
		TransactionDate: "2026-09-25",
	})

	t.Run("owner updates transaction successfully", func(t *testing.T) {
		desc := "Dinner with family"
		body, _ := json.Marshal(map[string]interface{}{
			"amount":           120000,
			"type":             "EXPENSE",
			"transaction_date": "2026-09-26",
			"description":      desc,
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/transactions/"+tx.ID.String(), bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token1)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var res handler.TransactionResponse
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Amount != 120000 {
			t.Errorf("expected amount 120000, got %d", res.Amount)
		}
	})

	t.Run("update non-existent returns 404", func(t *testing.T) {
		body, _ := json.Marshal(map[string]interface{}{
			"amount":           10000,
			"type":             "EXPENSE",
			"transaction_date": "2026-09-26",
		})
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodPut, "/api/v1/transactions/"+uuid.New().String(), bytes.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token1)
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", w.Code)
		}
	})
}

func TestTransactionHandler_Delete(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	txRepo := newMockTransactionRepo()
	catRepo := newMockCategoryRepo()
	txSvc := service.NewTransactionService(txRepo, catRepo)
	router := setupTransactionTestServer(txSvc, jwtMgr)

	user1 := uuid.New()
	token1, _ := jwtMgr.Generate(user1)

	tx, _ := txSvc.CreateTransaction(context.Background(), service.CreateTransactionInput{
		UserID:          user1,
		Amount:          50000,
		Type:            domain.TransactionTypeExpense,
		TransactionDate: "2026-09-25",
	})

	t.Run("owner deletes transaction successfully", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/transactions/"+tx.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("expected 204 No Content, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("second delete returns 404", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodDelete, "/api/v1/transactions/"+tx.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 Not Found, got %d", w.Code)
		}
	})
}
