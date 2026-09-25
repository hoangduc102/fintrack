package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
)

// TransactionRepository defines the persistence contract for transactions.
type TransactionRepository interface {
	Create(ctx context.Context, tx *domain.Transaction) error
	FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Transaction, error)
	Update(ctx context.Context, tx *domain.Transaction) error
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
}
