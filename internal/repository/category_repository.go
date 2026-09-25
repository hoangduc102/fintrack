package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
)

// CategoryRepository defines the persistence contract for category entities.
type CategoryRepository interface {
	Create(ctx context.Context, category *domain.Category) error
	FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Category, error)
	FindByUserAndType(ctx context.Context, userID uuid.UUID, categoryType *domain.CategoryType) ([]domain.Category, error)
	Update(ctx context.Context, category *domain.Category) error
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
}
