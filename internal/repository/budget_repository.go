package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
)

// BudgetRepository defines the persistence contract for monthly category budgets.
type BudgetRepository interface {
	Upsert(ctx context.Context, budget *domain.Budget) error
	FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Budget, error)
	FindByCategoryAndMonth(ctx context.Context, userID uuid.UUID, categoryID uuid.UUID, year int, month int) (*domain.Budget, error)
	ListByMonth(ctx context.Context, userID uuid.UUID, year int, month int) ([]domain.Budget, error)
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	GetSpent(ctx context.Context, userID uuid.UUID, categoryID uuid.UUID, year int, month int) (int64, error)
}
