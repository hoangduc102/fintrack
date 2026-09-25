package service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/service"
)

type fakeAnalyticsRepository struct {
	summary  map[string]*domain.MonthlySummary       // key: "userID:year:month"
	shares   map[string][]domain.CategoryExpenseShare // key: "userID:year:month"
	trendPts []domain.MonthlyTrendPoint
}

func newFakeAnalyticsRepository() *fakeAnalyticsRepository {
	return &fakeAnalyticsRepository{
		summary: make(map[string]*domain.MonthlySummary),
		shares:  make(map[string][]domain.CategoryExpenseShare),
	}
}

func (f *fakeAnalyticsRepository) GetMonthlySummary(ctx context.Context, userID uuid.UUID, year int, month int) (*domain.MonthlySummary, error) {
	key := fmt.Sprintf("%s:%d:%d", userID, year, month)
	if s, ok := f.summary[key]; ok {
		return s, nil
	}
	return &domain.MonthlySummary{
		Month: fmt.Sprintf("%04d-%02d", year, month),
	}, nil
}

func (f *fakeAnalyticsRepository) GetCategoryExpenses(ctx context.Context, userID uuid.UUID, year int, month int) ([]domain.CategoryExpenseShare, error) {
	key := fmt.Sprintf("%s:%d:%d", userID, year, month)
	if s, ok := f.shares[key]; ok {
		return s, nil
	}
	return []domain.CategoryExpenseShare{}, nil
}

func (f *fakeAnalyticsRepository) GetMonthlyTrend(ctx context.Context, userID uuid.UUID, months int) ([]domain.MonthlyTrendPoint, error) {
	if len(f.trendPts) > 0 {
		return f.trendPts, nil
	}
	res := make([]domain.MonthlyTrendPoint, months)
	for i := 0; i < months; i++ {
		res[i] = domain.MonthlyTrendPoint{
			Month: fmt.Sprintf("2026-%02d", i+1),
		}
	}
	return res, nil
}

func TestAnalyticsService_GetMonthlySummary(t *testing.T) {
	fakeRepo := newFakeAnalyticsRepository()
	svc := service.NewAnalyticsService(fakeRepo)
	ctx := context.Background()
	userID := uuid.New()

	fakeRepo.summary[fmt.Sprintf("%s:2026:9", userID)] = &domain.MonthlySummary{
		Month:        "2026-09",
		TotalIncome:  20000000,
		TotalExpense: 8000000,
		NetSavings:   12000000,
		BudgetStatus: domain.BudgetStatusOverview{
			TotalBudget: 10000000,
			TotalSpent:  8000000,
			Remaining:   2000000,
			Percentage:  80.0,
		},
	}

	t.Run("success returns summary", func(t *testing.T) {
		res, err := svc.GetMonthlySummary(ctx, userID, "2026-09")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.TotalIncome != 20000000 {
			t.Errorf("expected income 20000000, got %d", res.TotalIncome)
		}
		if res.NetSavings != 12000000 {
			t.Errorf("expected net savings 12000000, got %d", res.NetSavings)
		}
	})

	t.Run("invalid month format returns validation error", func(t *testing.T) {
		_, err := svc.GetMonthlySummary(ctx, userID, "invalid-month")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var valErr domain.ValidationError
		if !errors.As(err, &valErr) || valErr.Field != "month" {
			t.Errorf("expected ValidationError on month, got %v", err)
		}
	})
}

func TestAnalyticsService_GetCategoryExpenses(t *testing.T) {
	fakeRepo := newFakeAnalyticsRepository()
	svc := service.NewAnalyticsService(fakeRepo)
	ctx := context.Background()
	userID := uuid.New()

	catID := uuid.New()
	fakeRepo.shares[fmt.Sprintf("%s:2026:9", userID)] = []domain.CategoryExpenseShare{
		{
			CategoryID:   &catID,
			CategoryName: "Dining",
			Amount:       5000000,
			Percentage:   100.0,
		},
	}

	t.Run("success returns category shares", func(t *testing.T) {
		res, err := svc.GetCategoryExpenses(ctx, userID, "2026-09")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 share, got %d", len(res))
		}
		if res[0].Amount != 5000000 {
			t.Errorf("expected amount 5000000, got %d", res[0].Amount)
		}
	})

	t.Run("invalid month returns validation error", func(t *testing.T) {
		_, err := svc.GetCategoryExpenses(ctx, userID, "2026/09")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		var valErr domain.ValidationError
		if !errors.As(err, &valErr) || valErr.Field != "month" {
			t.Errorf("expected ValidationError on month, got %v", err)
		}
	})
}

func TestAnalyticsService_GetMonthlyTrend(t *testing.T) {
	fakeRepo := newFakeAnalyticsRepository()
	svc := service.NewAnalyticsService(fakeRepo)
	ctx := context.Background()
	userID := uuid.New()

	t.Run("default months when 0 is passed", func(t *testing.T) {
		res, err := svc.GetMonthlyTrend(ctx, userID, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 6 {
			t.Errorf("expected default 6 months, got %d", len(res))
		}
	})

	t.Run("custom months within limit", func(t *testing.T) {
		res, err := svc.GetMonthlyTrend(ctx, userID, 12)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res) != 12 {
			t.Errorf("expected 12 months, got %d", len(res))
		}
	})

	t.Run("months exceeding 24 returns validation error", func(t *testing.T) {
		_, err := svc.GetMonthlyTrend(ctx, userID, 25)
		if err == nil {
			t.Fatal("expected error for months > 24, got nil")
		}
		var valErr domain.ValidationError
		if !errors.As(err, &valErr) || valErr.Field != "months" {
			t.Errorf("expected ValidationError on months, got %v", err)
		}
	})
}
