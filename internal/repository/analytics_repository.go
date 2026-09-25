package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
)

// AnalyticsRepository defines the persistence contract for financial summaries and expense analytics.
type AnalyticsRepository interface {
	GetMonthlySummary(ctx context.Context, userID uuid.UUID, year int, month int) (*domain.MonthlySummary, error)
	GetCategoryExpenses(ctx context.Context, userID uuid.UUID, year int, month int) ([]domain.CategoryExpenseShare, error)
	GetMonthlyTrend(ctx context.Context, userID uuid.UUID, months int) ([]domain.MonthlyTrendPoint, error)
}
