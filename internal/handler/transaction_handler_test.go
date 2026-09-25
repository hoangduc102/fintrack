package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/auth"
	"github.com/hoangduc102/fintrack/internal/config"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/handler"
	"github.com/hoangduc102/fintrack/internal/repository"
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

func (r *mockTransactionRepo) List(ctx context.Context, filter repository.ListTransactionsFilter) (*repository.ListTransactionsResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var matching []*domain.Transaction
	for _, tx := range r.transactions {
		if tx.UserID != filter.UserID {
			continue
		}
		if filter.Type != nil && tx.Type != *filter.Type {
			continue
		}
		if filter.CategoryID != nil {
			if tx.CategoryID == nil || *tx.CategoryID != *filter.CategoryID {
				continue
			}
		}
		if filter.Uncategorized && tx.CategoryID != nil {
			continue
		}
		if filter.StartDate != nil && tx.TransactionDate < *filter.StartDate {
			continue
		}
		if filter.EndDate != nil && tx.TransactionDate > *filter.EndDate {
			continue
		}
		matching = append(matching, tx)
	}

	sort.Slice(matching, func(i, j int) bool {
		if matching[i].TransactionDate != matching[j].TransactionDate {
			return matching[i].TransactionDate > matching[j].TransactionDate
		}
		if !matching[i].CreatedAt.Equal(matching[j].CreatedAt) {
			return matching[i].CreatedAt.After(matching[j].CreatedAt)
		}
		return matching[i].ID.String() > matching[j].ID.String()
	})

	totalCount := len(matching)
	start := filter.Offset
	if start > totalCount {
		start = totalCount
	}
	end := start + filter.Limit
	if end > totalCount {
		end = totalCount
	}

	sliced := matching[start:end]
	res := make([]domain.Transaction, len(sliced))
	for i, tx := range sliced {
		res[i] = *tx
	}

	return &repository.ListTransactionsResult{
		Transactions: res,
		TotalCount:   totalCount,
	}, nil
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
		{http.MethodGet, "/api/v1/transactions"},
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

func TestTransactionHandler_List_DefaultsAndPagination(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	txRepo := newMockTransactionRepo()
	catRepo := newMockCategoryRepo()
	txSvc := service.NewTransactionService(txRepo, catRepo)
	router := setupTransactionTestServer(txSvc, jwtMgr)

	user1 := uuid.New()
	user2 := uuid.New()
	token1, _ := jwtMgr.Generate(user1)
	token2, _ := jwtMgr.Generate(user2)

	// Seed 25 transactions for user1
	for i := 1; i <= 25; i++ {
		date := fmt.Sprintf("2026-09-%02d", i)
		_, err := txSvc.CreateTransaction(context.Background(), service.CreateTransactionInput{
			UserID:          user1,
			Amount:          int64(i * 1000),
			Type:            domain.TransactionTypeExpense,
			TransactionDate: date,
		})
		if err != nil {
			t.Fatalf("failed to seed transaction: %v", err)
		}
	}

	t.Run("default pagination returns first 20 items", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/transactions", nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var res handler.PaginatedTransactionsResponse
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(res.Items) != 20 {
			t.Errorf("expected 20 items, got %d", len(res.Items))
		}
		if res.Pagination.Page != 1 {
			t.Errorf("expected page 1, got %d", res.Pagination.Page)
		}
		if res.Pagination.Limit != 20 {
			t.Errorf("expected limit 20, got %d", res.Pagination.Limit)
		}
		if res.Pagination.TotalItems != 25 {
			t.Errorf("expected total_items 25, got %d", res.Pagination.TotalItems)
		}
		if res.Pagination.TotalPages != 2 {
			t.Errorf("expected total_pages 2, got %d", res.Pagination.TotalPages)
		}
		// First item should be latest date (2026-09-25)
		if res.Items[0].TransactionDate != "2026-09-25" {
			t.Errorf("expected newest item first (2026-09-25), got %s", res.Items[0].TransactionDate)
		}
	})

	t.Run("page 2 returns remaining 5 items", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/transactions?page=2&limit=20", nil)
		req.Header.Set("Authorization", "Bearer "+token1)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var res handler.PaginatedTransactionsResponse
		_ = json.Unmarshal(w.Body.Bytes(), &res)

		if len(res.Items) != 5 {
			t.Errorf("expected 5 items, got %d", len(res.Items))
		}
		if res.Pagination.Page != 2 {
			t.Errorf("expected page 2, got %d", res.Pagination.Page)
		}
	})

	t.Run("user2 has no transactions", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/transactions", nil)
		req.Header.Set("Authorization", "Bearer "+token2)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d: %s", w.Code, w.Body.String())
		}

		var res handler.PaginatedTransactionsResponse
		_ = json.Unmarshal(w.Body.Bytes(), &res)

		if len(res.Items) != 0 {
			t.Errorf("expected 0 items, got %d", len(res.Items))
		}
		if res.Pagination.TotalItems != 0 {
			t.Errorf("expected total_items 0, got %d", res.Pagination.TotalItems)
		}
		if res.Pagination.TotalPages != 0 {
			t.Errorf("expected total_pages 0, got %d", res.Pagination.TotalPages)
		}
	})
}

func TestTransactionHandler_List_Filters(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	txRepo := newMockTransactionRepo()
	catRepo := newMockCategoryRepo()
	txSvc := service.NewTransactionService(txRepo, catRepo)
	router := setupTransactionTestServer(txSvc, jwtMgr)

	user := uuid.New()
	token, _ := jwtMgr.Generate(user)

	catSalary := &domain.Category{UserID: user, Name: "Salary", Type: domain.CategoryTypeIncome}
	_ = catRepo.Create(context.Background(), catSalary)
	catFood := &domain.Category{UserID: user, Name: "Food", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(context.Background(), catFood)

	// 1: Income categorized
	_, _ = txSvc.CreateTransaction(context.Background(), service.CreateTransactionInput{
		UserID:          user,
		Amount:          20000000,
		Type:            domain.TransactionTypeIncome,
		CategoryID:      &catSalary.ID,
		TransactionDate: "2026-09-01",
	})
	// 2: Expense categorized
	_, _ = txSvc.CreateTransaction(context.Background(), service.CreateTransactionInput{
		UserID:          user,
		Amount:          50000,
		Type:            domain.TransactionTypeExpense,
		CategoryID:      &catFood.ID,
		TransactionDate: "2026-09-10",
	})
	// 3: Expense uncategorized
	_, _ = txSvc.CreateTransaction(context.Background(), service.CreateTransactionInput{
		UserID:          user,
		Amount:          15000,
		Type:            domain.TransactionTypeExpense,
		CategoryID:      nil,
		TransactionDate: "2026-09-20",
	})

	t.Run("filter by type=INCOME", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/transactions?type=INCOME", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var res handler.PaginatedTransactionsResponse
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Pagination.TotalItems != 1 {
			t.Errorf("expected 1 item, got %d", res.Pagination.TotalItems)
		}
		if len(res.Items) != 1 || res.Items[0].Type != "INCOME" {
			t.Errorf("expected INCOME item, got %v", res.Items)
		}
	})

	t.Run("filter by category_id", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/transactions?category_id="+catFood.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var res handler.PaginatedTransactionsResponse
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Pagination.TotalItems != 1 {
			t.Errorf("expected 1 item, got %d", res.Pagination.TotalItems)
		}
		if res.Items[0].Amount != 50000 {
			t.Errorf("expected amount 50000, got %d", res.Items[0].Amount)
		}
	})

	t.Run("filter by uncategorized=true", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/transactions?uncategorized=true", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var res handler.PaginatedTransactionsResponse
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Pagination.TotalItems != 1 {
			t.Errorf("expected 1 item, got %d", res.Pagination.TotalItems)
		}
		if res.Items[0].Amount != 15000 {
			t.Errorf("expected amount 15000, got %d", res.Items[0].Amount)
		}
	})

	t.Run("filter by date range", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/api/v1/transactions?start_date=2026-09-05&end_date=2026-09-15", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", w.Code)
		}

		var res handler.PaginatedTransactionsResponse
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Pagination.TotalItems != 1 {
			t.Errorf("expected 1 item, got %d", res.Pagination.TotalItems)
		}
		if res.Items[0].TransactionDate != "2026-09-10" {
			t.Errorf("expected date 2026-09-10, got %s", res.Items[0].TransactionDate)
		}
	})
}

func TestTransactionHandler_List_ValidationErrors(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	txRepo := newMockTransactionRepo()
	catRepo := newMockCategoryRepo()
	txSvc := service.NewTransactionService(txRepo, catRepo)
	router := setupTransactionTestServer(txSvc, jwtMgr)

	user := uuid.New()
	token, _ := jwtMgr.Generate(user)

	tests := []struct {
		name       string
		query      string
		fieldCheck string
	}{
		{
			name:       "limit exceeds 100",
			query:      "?limit=101",
			fieldCheck: "limit",
		},
		{
			name:       "invalid limit integer",
			query:      "?limit=abc",
			fieldCheck: "limit",
		},
		{
			name:       "invalid page integer",
			query:      "?page=xyz",
			fieldCheck: "page",
		},
		{
			name:       "invalid category_id UUID",
			query:      "?category_id=not-a-uuid",
			fieldCheck: "category_id",
		},
		{
			name:       "invalid uncategorized boolean",
			query:      "?uncategorized=maybe",
			fieldCheck: "uncategorized",
		},
		{
			name:       "mutually exclusive category_id and uncategorized",
			query:      fmt.Sprintf("?category_id=%s&uncategorized=true", uuid.New().String()),
			fieldCheck: "uncategorized",
		},
		{
			name:       "invalid start_date format",
			query:      "?start_date=2026/09/01",
			fieldCheck: "start_date",
		},
		{
			name:       "start_date after end_date",
			query:      "?start_date=2026-09-20&end_date=2026-09-10",
			fieldCheck: "start_date",
		},
		{
			name:       "invalid type value",
			query:      "?type=INVESTMENT",
			fieldCheck: "type",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(http.MethodGet, "/api/v1/transactions"+tc.query, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 Bad Request, got %d: %s", w.Code, w.Body.String())
			}

			var res map[string]interface{}
			_ = json.Unmarshal(w.Body.Bytes(), &res)
			errObj, ok := res["error"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected error object, got %v", res)
			}
			if errObj["code"] != "VALIDATION_ERROR" {
				t.Errorf("expected error code VALIDATION_ERROR, got %v", errObj["code"])
			}
		})
	}
}
