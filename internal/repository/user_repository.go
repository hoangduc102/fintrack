package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
)

// UserRepository defines the persistence contract for user entities and their initial seeds.
type UserRepository interface {
	CreateWithCategories(ctx context.Context, user *domain.User, categories []domain.Category) error
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}
