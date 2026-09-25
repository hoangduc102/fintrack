package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/auth"
	"github.com/hoangduc102/fintrack/internal/config"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/handler"
	"github.com/hoangduc102/fintrack/internal/repository"
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

func TestLoginAndMeRoute(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
	}

	repo := &fakeUserRepoForRouter{users: make(map[string]*domain.User)}
	jwtManager := auth.NewJWTManager("test-secret", 24*time.Hour)
	authService := service.NewAuthService(repo, jwtManager)
	authHandler := handler.NewAuthHandler(authService)

	router := server.NewRouter(cfg, authHandler, jwtManager)

	// 1. Register
	regBody := `{"email":"flow@example.com","password":"password123","display_name":"Flow User"}`
	regReq, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	router.ServeHTTP(regRec, regReq)
	if regRec.Code != http.StatusCreated {
		t.Fatalf("register failed: %d", regRec.Code)
	}

	// 2. Login
	loginBody := `{"email":"flow@example.com","password":"password123"}`
	loginReq, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	router.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusOK {
		t.Fatalf("login failed: %d", loginRec.Code)
	}

	var loginResp handler.LoginResponse
	_ = json.Unmarshal(loginRec.Body.Bytes(), &loginResp)
	if loginResp.Token == "" {
		t.Fatal("expected non-empty token")
	}

	// 3. Me
	meReq, _ := http.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginResp.Token)
	meRec := httptest.NewRecorder()
	router.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me failed: %d", meRec.Code)
	}

	var userResp handler.UserResponse
	_ = json.Unmarshal(meRec.Body.Bytes(), &userResp)
	if userResp.Email != "flow@example.com" {
		t.Errorf("expected email 'flow@example.com', got %q", userResp.Email)
	}
	if userResp.DisplayName == nil || *userResp.DisplayName != "Flow User" {
		t.Errorf("expected display name 'Flow User', got %v", userResp.DisplayName)
	}
}

type fakeCategoryRepoForRouter struct {
	categories map[uuid.UUID]*domain.Category
}

func (f *fakeCategoryRepoForRouter) Create(ctx context.Context, category *domain.Category) error {
	category.ID = uuid.New()
	category.CreatedAt = time.Now().UTC()
	category.UpdatedAt = category.CreatedAt
	f.categories[category.ID] = category
	return nil
}

func (f *fakeCategoryRepoForRouter) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Category, error) {
	c, ok := f.categories[id]
	if !ok || c.UserID != userID {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (f *fakeCategoryRepoForRouter) FindByUserAndType(ctx context.Context, userID uuid.UUID, categoryType *domain.CategoryType) ([]domain.Category, error) {
	var result []domain.Category
	for _, c := range f.categories {
		if c.UserID == userID {
			if categoryType == nil || c.Type == *categoryType {
				result = append(result, *c)
			}
		}
	}
	return result, nil
}

func (f *fakeCategoryRepoForRouter) Update(ctx context.Context, category *domain.Category) error {
	c, ok := f.categories[category.ID]
	if !ok || c.UserID != category.UserID {
		return domain.ErrNotFound
	}
	c.Name = category.Name
	c.UpdatedAt = time.Now().UTC()
	return nil
}

func (f *fakeCategoryRepoForRouter) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	c, ok := f.categories[id]
	if !ok || c.UserID != userID {
		return domain.ErrNotFound
	}
	delete(f.categories, id)
	return nil
}

func TestCategoriesRoute(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
	}

	catRepo := &fakeCategoryRepoForRouter{categories: make(map[uuid.UUID]*domain.Category)}
	catService := service.NewCategoryService(catRepo)
	catHandler := handler.NewCategoryHandler(catService)
	jwtManager := auth.NewJWTManager("test-secret", 24*time.Hour)

	router := server.NewRouter(cfg, catHandler, jwtManager)

	userID := uuid.New()
	token, _ := jwtManager.Generate(userID)

	// 1. Create Category
	createBody := `{"name":"Groceries","type":"EXPENSE"}`
	createReq, _ := http.NewRequest(http.MethodPost, "/api/v1/categories", strings.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create category failed: %d, body: %s", createRec.Code, createRec.Body.String())
	}

	var catResp handler.CategoryResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &catResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if catResp.Name != "Groceries" {
		t.Errorf("expected name 'Groceries', got %q", catResp.Name)
	}

	// 2. List Categories
	listReq, _ := http.NewRequest(http.MethodGet, "/api/v1/categories", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list categories failed: %d", listRec.Code)
	}

	var listResp []handler.CategoryResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list: %v", err)
	}
	if len(listResp) != 1 {
		t.Errorf("expected 1 category, got %d", len(listResp))
	}

	// 3. Update Category
	updateBody := `{"name":"Food & Groceries"}`
	updateReq, _ := http.NewRequest(http.MethodPut, "/api/v1/categories/"+catResp.ID, strings.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	router.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update category failed: %d, body: %s", updateRec.Code, updateRec.Body.String())
	}

	// 4. Delete Category
	deleteReq, _ := http.NewRequest(http.MethodDelete, "/api/v1/categories/"+catResp.ID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete category failed: %d", deleteRec.Code)
	}
}

type fakeTransactionRepoForRouter struct {
	transactions map[uuid.UUID]*domain.Transaction
}

func (f *fakeTransactionRepoForRouter) Create(ctx context.Context, tx *domain.Transaction) error {
	tx.ID = uuid.New()
	tx.CreatedAt = time.Now().UTC()
	tx.UpdatedAt = tx.CreatedAt
	f.transactions[tx.ID] = tx
	return nil
}

func (f *fakeTransactionRepoForRouter) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Transaction, error) {
	tx, ok := f.transactions[id]
	if !ok || tx.UserID != userID {
		return nil, domain.ErrNotFound
	}
	return tx, nil
}

func (f *fakeTransactionRepoForRouter) Update(ctx context.Context, tx *domain.Transaction) error {
	existing, ok := f.transactions[tx.ID]
	if !ok || existing.UserID != tx.UserID {
		return domain.ErrNotFound
	}
	tx.UpdatedAt = time.Now().UTC()
	f.transactions[tx.ID] = tx
	return nil
}

func (f *fakeTransactionRepoForRouter) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	existing, ok := f.transactions[id]
	if !ok || existing.UserID != userID {
		return domain.ErrNotFound
	}
	delete(f.transactions, id)
	return nil
}

func (f *fakeTransactionRepoForRouter) List(ctx context.Context, filter repository.ListTransactionsFilter) (*repository.ListTransactionsResult, error) {
	var matching []domain.Transaction
	for _, tx := range f.transactions {
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
		matching = append(matching, *tx)
	}

	totalCount := len(matching)
	start := filter.Offset
	if start > totalCount {
		start = totalCount
	}
	end := start + filter.Limit
	if end > totalCount {
		end = totalCount
	}

	return &repository.ListTransactionsResult{
		Transactions: matching[start:end],
		TotalCount:   totalCount,
	}, nil
}

func TestTransactionsRoute(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
	}

	catRepo := &fakeCategoryRepoForRouter{categories: make(map[uuid.UUID]*domain.Category)}
	txRepo := &fakeTransactionRepoForRouter{transactions: make(map[uuid.UUID]*domain.Transaction)}
	txService := service.NewTransactionService(txRepo, catRepo)
	txHandler := handler.NewTransactionHandler(txService)
	jwtManager := auth.NewJWTManager("test-secret", 24*time.Hour)

	router := server.NewRouter(cfg, txHandler, jwtManager)

	userID := uuid.New()
	token, _ := jwtManager.Generate(userID)

	// 1. Create Transaction
	createBody := `{"amount":75000,"type":"EXPENSE","transaction_date":"2026-09-25","description":"Lunch"}`
	createReq, _ := http.NewRequest(http.MethodPost, "/api/v1/transactions", strings.NewReader(createBody))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create transaction failed: %d, body: %s", createRec.Code, createRec.Body.String())
	}

	var txResp handler.TransactionResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &txResp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if txResp.Amount != 75000 {
		t.Errorf("expected amount 75000, got %d", txResp.Amount)
	}

	// 2. List Transactions
	listReq, _ := http.NewRequest(http.MethodGet, "/api/v1/transactions", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list transactions failed: %d, body: %s", listRec.Code, listRec.Body.String())
	}
	var listResp handler.PaginatedTransactionsResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if len(listResp.Items) != 1 {
		t.Errorf("expected 1 item in list, got %d", len(listResp.Items))
	}
	if listResp.Pagination.TotalItems != 1 {
		t.Errorf("expected total_items 1, got %d", listResp.Pagination.TotalItems)
	}

	// 3. Get Transaction
	getReq, _ := http.NewRequest(http.MethodGet, "/api/v1/transactions/"+txResp.ID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	router.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get transaction failed: %d", getRec.Code)
	}

	// 4. Update Transaction
	updateBody := `{"amount":80000,"type":"EXPENSE","transaction_date":"2026-09-25","description":"Lunch + Coffee"}`
	updateReq, _ := http.NewRequest(http.MethodPut, "/api/v1/transactions/"+txResp.ID, strings.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()
	router.ServeHTTP(updateRec, updateReq)
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update transaction failed: %d", updateRec.Code)
	}

	// 4. Delete Transaction
	deleteReq, _ := http.NewRequest(http.MethodDelete, "/api/v1/transactions/"+txResp.ID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRec := httptest.NewRecorder()
	router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete transaction failed: %d", deleteRec.Code)
	}
}

type fakeBudgetRepoForRouter struct {
	budgets map[uuid.UUID]*domain.Budget
}

func (f *fakeBudgetRepoForRouter) Upsert(ctx context.Context, budget *domain.Budget) error {
	for _, b := range f.budgets {
		if b.UserID == budget.UserID && b.CategoryID == budget.CategoryID && b.Year == budget.Year && b.Month == budget.Month {
			b.Amount = budget.Amount
			b.UpdatedAt = time.Now().UTC()
			budget.ID = b.ID
			budget.CreatedAt = b.CreatedAt
			budget.UpdatedAt = b.UpdatedAt
			return nil
		}
	}
	if budget.ID == uuid.Nil {
		budget.ID = uuid.New()
	}
	budget.CreatedAt = time.Now().UTC()
	budget.UpdatedAt = budget.CreatedAt
	copied := *budget
	f.budgets[budget.ID] = &copied
	return nil
}

func (f *fakeBudgetRepoForRouter) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Budget, error) {
	b, ok := f.budgets[id]
	if !ok || b.UserID != userID {
		return nil, domain.ErrNotFound
	}
	return b, nil
}

func (f *fakeBudgetRepoForRouter) FindByCategoryAndMonth(ctx context.Context, userID uuid.UUID, categoryID uuid.UUID, year int, month int) (*domain.Budget, error) {
	for _, b := range f.budgets {
		if b.UserID == userID && b.CategoryID == categoryID && b.Year == year && b.Month == month {
			return b, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeBudgetRepoForRouter) ListByMonth(ctx context.Context, userID uuid.UUID, year int, month int) ([]domain.Budget, error) {
	var result []domain.Budget
	for _, b := range f.budgets {
		if b.UserID == userID && b.Year == year && b.Month == month {
			result = append(result, *b)
		}
	}
	return result, nil
}

func (f *fakeBudgetRepoForRouter) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	b, ok := f.budgets[id]
	if !ok || b.UserID != userID {
		return domain.ErrNotFound
	}
	delete(f.budgets, id)
	return nil
}

func (f *fakeBudgetRepoForRouter) GetSpent(ctx context.Context, userID uuid.UUID, categoryID uuid.UUID, year int, month int) (int64, error) {
	return 0, nil
}

func TestBudgetsRoute(t *testing.T) {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
	}

	catRepo := &fakeCategoryRepoForRouter{categories: make(map[uuid.UUID]*domain.Category)}
	budgetRepo := &fakeBudgetRepoForRouter{budgets: make(map[uuid.UUID]*domain.Budget)}
	budgetService := service.NewBudgetService(budgetRepo, catRepo)
	budgetHandler := handler.NewBudgetHandler(budgetService)
	jwtManager := auth.NewJWTManager("test-secret", 24*time.Hour)

	router := server.NewRouter(cfg, budgetHandler, jwtManager)

	userID := uuid.New()
	token, _ := jwtManager.Generate(userID)

	cat := &domain.Category{
		ID:     uuid.New(),
		UserID: userID,
		Name:   "Dining",
		Type:   domain.CategoryTypeExpense,
	}
	catRepo.categories[cat.ID] = cat

	// 1. Create / Upsert Budget
	upsertBody := fmt.Sprintf(`{"category_id":"%s","amount":5000000,"year":2026,"month":9}`, cat.ID.String())
	req1, _ := http.NewRequest(http.MethodPut, "/api/v1/budgets", strings.NewReader(upsertBody))
	req1.Header.Set("Authorization", "Bearer "+token)
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("upsert budget failed: %d, body: %s", rec1.Code, rec1.Body.String())
	}

	var budgetResp handler.BudgetResponse
	if err := json.Unmarshal(rec1.Body.Bytes(), &budgetResp); err != nil {
		t.Fatalf("failed to decode budget response: %v", err)
	}
	if budgetResp.Amount != 5000000 {
		t.Errorf("expected amount 5000000, got %d", budgetResp.Amount)
	}

	// 2. List Budgets
	req2, _ := http.NewRequest(http.MethodGet, "/api/v1/budgets?month=2026-09", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("list budgets failed: %d", rec2.Code)
	}
	var listResp []handler.BudgetResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if len(listResp) != 1 {
		t.Errorf("expected 1 budget in list, got %d", len(listResp))
	}

	// 3. Delete Budget
	req3, _ := http.NewRequest(http.MethodDelete, "/api/v1/budgets/"+budgetResp.ID, nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	rec3 := httptest.NewRecorder()
	router.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusNoContent {
		t.Fatalf("delete budget failed: %d", rec3.Code)
	}
}
