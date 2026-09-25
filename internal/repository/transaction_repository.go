package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
)

// ListTransactionsFilter defines criteria for filtering and paginating transactions.
type ListTransactionsFilter struct {
	UserID        uuid.UUID
	StartDate     *string
	EndDate       *string
	Type          *domain.TransactionType
	CategoryID    *uuid.UUID
	Uncategorized bool
	Limit         int
	Offset        int
}

// ListTransactionsResult contains the slice of transactions and total matching count.
type ListTransactionsResult struct {
	Transactions []domain.Transaction
	TotalCount   int
}

// TransactionRepository defines the persistence contract for transactions.
type TransactionRepository interface {
	Create(ctx context.Context, tx *domain.Transaction) error
	CreateBatch(ctx context.Context, txs []domain.Transaction) error
	FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Transaction, error)
	List(ctx context.Context, filter ListTransactionsFilter) (*ListTransactionsResult, error)
	Update(ctx context.Context, tx *domain.Transaction) error
	Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
}

