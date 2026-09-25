package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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

type mockAnalyticsRepo struct {
	summaryFn    func(ctx context.Context, userID uuid.UUID, year, month int) (*domain.MonthlySummary, error)
	categoriesFn func(ctx context.Context, userID uuid.UUID, year, month int) ([]domain.CategoryExpenseShare, error)
	trendFn      func(ctx context.Context, userID uuid.UUID, months int) ([]domain.MonthlyTrendPoint, error)
}

func (m *mockAnalyticsRepo) GetMonthlySummary(ctx context.Context, userID uuid.UUID, year, month int) (*domain.MonthlySummary, error) {
	if m.summaryFn != nil {
		return m.summaryFn(ctx, userID, year, month)
	}
	return &domain.MonthlySummary{}, nil
}

func (m *mockAnalyticsRepo) GetCategoryExpenses(ctx context.Context, userID uuid.UUID, year, month int) ([]domain.CategoryExpenseShare, error) {
	if m.categoriesFn != nil {
		return m.categoriesFn(ctx, userID, year, month)
	}
	return nil, nil
}

func (m *mockAnalyticsRepo) GetMonthlyTrend(ctx context.Context, userID uuid.UUID, months int) ([]domain.MonthlyTrendPoint, error) {
	if m.trendFn != nil {
		return m.trendFn(ctx, userID, months)
	}
	return nil, nil
}

func setupAnalyticsTestServer(analyticsSvc *service.AnalyticsService, jwtMgr *auth.JWTManager) *gin.Engine {
	cfg := &config.Config{
		Environment: "test",
		Port:        "8080",
		JWTSecret:   "test-secret-key-12345678901234567890",
	}
	analyticsHandler := handler.NewAnalyticsHandler(analyticsSvc)
	return server.NewRouter(cfg, analyticsHandler, jwtMgr)
}

func TestAnalyticsHandler_AuthProtection(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	repo := &mockAnalyticsRepo{}
	svc := service.NewAnalyticsService(repo)
	router := setupAnalyticsTestServer(svc, jwtMgr)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/analytics/summary?month=2026-09"},
		{http.MethodGet, "/api/v1/analytics/categories?month=2026-09"},
		{http.MethodGet, "/api/v1/analytics/trend?months=6"},
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

func TestAnalyticsHandler_GetSummary(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	userID := uuid.New()
	token, _ := jwtMgr.Generate(userID)

	t.Run("success", func(t *testing.T) {
		repo := &mockAnalyticsRepo{
			summaryFn: func(ctx context.Context, uID uuid.UUID, year, month int) (*domain.MonthlySummary, error) {
				if uID != userID || year != 2026 || month != 9 {
					return nil, errors.New("unexpected params")
				}
				return &domain.MonthlySummary{
					Month:        "2026-09",
					TotalIncome:  10000000,
					TotalExpense: 6000000,
					NetSavings:   4000000,
					BudgetStatus: domain.BudgetStatusOverview{
						TotalBudget: 8000000,
						TotalSpent:  6000000,
						Remaining:   2000000,
						Percentage:  75.0,
					},
				}, nil
			},
		}
		svc := service.NewAnalyticsService(repo)
		router := setupAnalyticsTestServer(svc, jwtMgr)

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/summary?month=2026-09", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var summary domain.MonthlySummary
		if err := json.Unmarshal(w.Body.Bytes(), &summary); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if summary.Month != "2026-09" || summary.TotalIncome != 10000000 || summary.TotalExpense != 6000000 || summary.NetSavings != 4000000 {
			t.Errorf("unexpected summary fields: %+v", summary)
		}
		if summary.BudgetStatus.TotalBudget != 8000000 || summary.BudgetStatus.Percentage != 75.0 {
			t.Errorf("unexpected budget overview: %+v", summary.BudgetStatus)
		}
	})

	t.Run("missing month parameter", func(t *testing.T) {
		repo := &mockAnalyticsRepo{}
		svc := service.NewAnalyticsService(repo)
		router := setupAnalyticsTestServer(svc, jwtMgr)

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/summary", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("invalid month format", func(t *testing.T) {
		repo := &mockAnalyticsRepo{}
		svc := service.NewAnalyticsService(repo)
		router := setupAnalyticsTestServer(svc, jwtMgr)

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/summary?month=invalid", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("internal server error", func(t *testing.T) {
		repo := &mockAnalyticsRepo{
			summaryFn: func(ctx context.Context, uID uuid.UUID, year, month int) (*domain.MonthlySummary, error) {
				return nil, errors.New("db error")
			},
		}
		svc := service.NewAnalyticsService(repo)
		router := setupAnalyticsTestServer(svc, jwtMgr)

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/summary?month=2026-09", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", w.Code)
		}
	})
}

func TestAnalyticsHandler_GetCategories(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	userID := uuid.New()
	token, _ := jwtMgr.Generate(userID)

	t.Run("success", func(t *testing.T) {
		catID := uuid.New()
		repo := &mockAnalyticsRepo{
			categoriesFn: func(ctx context.Context, uID uuid.UUID, year, month int) ([]domain.CategoryExpenseShare, error) {
				return []domain.CategoryExpenseShare{
					{
						CategoryID:   &catID,
						CategoryName: "Food & Dining",
						Amount:       3000000,
						Percentage:   75.0,
					},
					{
						CategoryID:   nil,
						CategoryName: "Uncategorized",
						Amount:       1000000,
						Percentage:   25.0,
					},
				}, nil
			},
		}
		svc := service.NewAnalyticsService(repo)
		router := setupAnalyticsTestServer(svc, jwtMgr)

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/categories?month=2026-09", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var shares []handler.CategoryExpenseResponse
		if err := json.Unmarshal(w.Body.Bytes(), &shares); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(shares) != 2 {
			t.Fatalf("expected 2 items, got %d", len(shares))
		}
		if shares[0].CategoryID == nil || *shares[0].CategoryID != catID.String() {
			t.Errorf("expected category ID %s, got %v", catID.String(), shares[0].CategoryID)
		}
		if shares[0].CategoryName != "Food & Dining" || shares[0].Percentage != 75.0 {
			t.Errorf("unexpected share[0]: %+v", shares[0])
		}
		if shares[1].CategoryID != nil {
			t.Errorf("expected nil category ID for uncategorized, got %v", shares[1].CategoryID)
		}
		if shares[1].CategoryName != "Uncategorized" || shares[1].Percentage != 25.0 {
			t.Errorf("unexpected share[1]: %+v", shares[1])
		}
	})

	t.Run("missing month parameter", func(t *testing.T) {
		repo := &mockAnalyticsRepo{}
		svc := service.NewAnalyticsService(repo)
		router := setupAnalyticsTestServer(svc, jwtMgr)

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/categories", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("invalid month format", func(t *testing.T) {
		repo := &mockAnalyticsRepo{}
		svc := service.NewAnalyticsService(repo)
		router := setupAnalyticsTestServer(svc, jwtMgr)

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/categories?month=2026-13", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("internal server error", func(t *testing.T) {
		repo := &mockAnalyticsRepo{
			categoriesFn: func(ctx context.Context, uID uuid.UUID, year, month int) ([]domain.CategoryExpenseShare, error) {
				return nil, errors.New("db error")
			},
		}
		svc := service.NewAnalyticsService(repo)
		router := setupAnalyticsTestServer(svc, jwtMgr)

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/categories?month=2026-09", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", w.Code)
		}
	})
}

func TestAnalyticsHandler_GetTrend(t *testing.T) {
	jwtMgr := auth.NewJWTManager("test-secret-key-12345678901234567890", 24*time.Hour)
	userID := uuid.New()
	token, _ := jwtMgr.Generate(userID)

	t.Run("success with default months (6)", func(t *testing.T) {
		repo := &mockAnalyticsRepo{
			trendFn: func(ctx context.Context, uID uuid.UUID, months int) ([]domain.MonthlyTrendPoint, error) {
				if months != 6 {
					return nil, fmt.Errorf("expected 6 months, got %d", months)
				}
				return []domain.MonthlyTrendPoint{
					{Month: "2026-04", Income: 1000, Expense: 500, NetSavings: 500},
					{Month: "2026-05", Income: 1000, Expense: 600, NetSavings: 400},
				}, nil
			},
		}
		svc := service.NewAnalyticsService(repo)
		router := setupAnalyticsTestServer(svc, jwtMgr)

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/trend", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var points []domain.MonthlyTrendPoint
		if err := json.Unmarshal(w.Body.Bytes(), &points); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(points) != 2 {
			t.Errorf("expected 2 trend points, got %d", len(points))
		}
	})

	t.Run("success with explicit months", func(t *testing.T) {
		repo := &mockAnalyticsRepo{
			trendFn: func(ctx context.Context, uID uuid.UUID, months int) ([]domain.MonthlyTrendPoint, error) {
				if months != 12 {
					return nil, fmt.Errorf("expected 12 months, got %d", months)
				}
				return []domain.MonthlyTrendPoint{
					{Month: "2025-10", Income: 0, Expense: 0, NetSavings: 0},
				}, nil
			},
		}
		svc := service.NewAnalyticsService(repo)
		router := setupAnalyticsTestServer(svc, jwtMgr)

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/trend?months=12", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("invalid months format (non-integer)", func(t *testing.T) {
		repo := &mockAnalyticsRepo{}
		svc := service.NewAnalyticsService(repo)
		router := setupAnalyticsTestServer(svc, jwtMgr)

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/trend?months=abc", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("invalid months value (out of bounds)", func(t *testing.T) {
		repo := &mockAnalyticsRepo{}
		svc := service.NewAnalyticsService(repo)
		router := setupAnalyticsTestServer(svc, jwtMgr)

		for _, invalidMonths := range []string{"0", "-1", "25"} {
			req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/trend?months="+invalidMonths, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("months=%s: expected status 400, got %d", invalidMonths, w.Code)
			}
		}
	})

	t.Run("internal server error", func(t *testing.T) {
		repo := &mockAnalyticsRepo{
			trendFn: func(ctx context.Context, uID uuid.UUID, months int) ([]domain.MonthlyTrendPoint, error) {
				return nil, errors.New("db error")
			},
		}
		svc := service.NewAnalyticsService(repo)
		router := setupAnalyticsTestServer(svc, jwtMgr)

		req, _ := http.NewRequest(http.MethodGet, "/api/v1/analytics/trend?months=6", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", w.Code)
		}
	})
}
