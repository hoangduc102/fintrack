package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/repository"
)

// AnalyticsService coordinates calculation and retrieval of financial summaries and analytics.
type AnalyticsService struct {
	analyticsRepo repository.AnalyticsRepository
}

// NewAnalyticsService creates a new AnalyticsService.
func NewAnalyticsService(analyticsRepo repository.AnalyticsRepository) *AnalyticsService {
	return &AnalyticsService{analyticsRepo: analyticsRepo}
}

// GetMonthlySummary validates month format and retrieves high-level financial summary.
func (s *AnalyticsService) GetMonthlySummary(ctx context.Context, userID uuid.UUID, monthStr string) (*domain.MonthlySummary, error) {
	t, err := time.Parse("2006-01", monthStr)
	if err != nil {
		return nil, domain.ValidationError{Field: "month", Message: "month must be in YYYY-MM format"}
	}

	return s.analyticsRepo.GetMonthlySummary(ctx, userID, t.Year(), int(t.Month()))
}

// GetCategoryExpenses validates month format and retrieves category expense distribution.
func (s *AnalyticsService) GetCategoryExpenses(ctx context.Context, userID uuid.UUID, monthStr string) ([]domain.CategoryExpenseShare, error) {
	t, err := time.Parse("2006-01", monthStr)
	if err != nil {
		return nil, domain.ValidationError{Field: "month", Message: "month must be in YYYY-MM format"}
	}

	return s.analyticsRepo.GetCategoryExpenses(ctx, userID, t.Year(), int(t.Month()))
}

// GetMonthlyTrend validates months count and retrieves monthly income/expense trend.
func (s *AnalyticsService) GetMonthlyTrend(ctx context.Context, userID uuid.UUID, months int) ([]domain.MonthlyTrendPoint, error) {
	if months == 0 {
		months = 6
	}
	if months < 1 || months > 24 {
		return nil, domain.ValidationError{Field: "months", Message: "months must be between 1 and 24"}
	}

	return s.analyticsRepo.GetMonthlyTrend(ctx, userID, months)
}
