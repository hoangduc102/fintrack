package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/repository"
)

// CreateTransactionInput holds the data required to create a transaction.
type CreateTransactionInput struct {
	UserID          uuid.UUID
	Amount          int64
	Type            domain.TransactionType
	CategoryID      *uuid.UUID
	TransactionDate string // YYYY-MM-DD
	Description     *string
}

// UpdateTransactionInput holds the data required to update an existing transaction.
type UpdateTransactionInput struct {
	ID              uuid.UUID
	UserID          uuid.UUID
	Amount          int64
	Type            domain.TransactionType
	CategoryID      *uuid.UUID
	TransactionDate string // YYYY-MM-DD
	Description     *string
}

// ListTransactionsInput holds parameters for querying and paginating transactions.
type ListTransactionsInput struct {
	UserID        uuid.UUID
	Page          int
	Limit         int
	StartDate     *string
	EndDate       *string
	Type          *domain.TransactionType
	CategoryID    *uuid.UUID
	Uncategorized bool
}

// PaginationInfo holds pagination metadata.
type PaginationInfo struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

// PaginatedTransactionsOutput contains the slice of transactions and pagination metadata.
type PaginatedTransactionsOutput struct {
	Items      []domain.Transaction `json:"items"`
	Pagination PaginationInfo       `json:"pagination"`
}

// TransactionService coordinates business logic and validations for transactions.
type TransactionService struct {
	txRepo       repository.TransactionRepository
	categoryRepo repository.CategoryRepository
}

// NewTransactionService creates a new TransactionService.
func NewTransactionService(txRepo repository.TransactionRepository, categoryRepo repository.CategoryRepository) *TransactionService {
	return &TransactionService{
		txRepo:       txRepo,
		categoryRepo: categoryRepo,
	}
}

func (s *TransactionService) validateTransaction(ctx context.Context, userID uuid.UUID, amount int64, txType domain.TransactionType, categoryID *uuid.UUID, dateStr string) error {
	if amount <= 0 {
		return domain.ValidationError{Field: "amount", Message: "amount must be greater than zero"}
	}

	if txType != domain.TransactionTypeIncome && txType != domain.TransactionTypeExpense {
		return domain.ValidationError{Field: "type", Message: "type must be INCOME or EXPENSE"}
	}

	if strings.TrimSpace(dateStr) == "" {
		return domain.ValidationError{Field: "transaction_date", Message: "transaction_date is required"}
	}

	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		return domain.ValidationError{Field: "transaction_date", Message: "transaction_date must be in YYYY-MM-DD format"}
	}

	if categoryID != nil && *categoryID != uuid.Nil {
		cat, err := s.categoryRepo.FindByID(ctx, *categoryID, userID)
		if err != nil {
			return domain.ValidationError{Field: "category_id", Message: "category not found or not owned by user"}
		}
		if cat.Type != domain.CategoryType(txType) {
			return domain.ValidationError{Field: "category_id", Message: "category type does not match transaction type"}
		}
	}

	return nil
}

// CreateTransaction validates input and creates a new transaction.
func (s *TransactionService) CreateTransaction(ctx context.Context, input CreateTransactionInput) (*domain.Transaction, error) {
	if err := s.validateTransaction(ctx, input.UserID, input.Amount, input.Type, input.CategoryID, input.TransactionDate); err != nil {
		return nil, err
	}

	var desc *string
	if input.Description != nil {
		trimmed := strings.TrimSpace(*input.Description)
		if trimmed != "" {
			desc = &trimmed
		}
	}

	tx := &domain.Transaction{
		UserID:          input.UserID,
		Amount:          input.Amount,
		Type:            input.Type,
		CategoryID:      input.CategoryID,
		TransactionDate: input.TransactionDate,
		Description:     desc,
	}

	if err := s.txRepo.Create(ctx, tx); err != nil {
		return nil, err
	}

	return tx, nil
}

// GetTransaction retrieves a transaction by ID while ensuring user ownership.
func (s *TransactionService) GetTransaction(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Transaction, error) {
	return s.txRepo.FindByID(ctx, id, userID)
}

// UpdateTransaction validates input, verifies user ownership, and updates the transaction.
func (s *TransactionService) UpdateTransaction(ctx context.Context, input UpdateTransactionInput) (*domain.Transaction, error) {
	if err := s.validateTransaction(ctx, input.UserID, input.Amount, input.Type, input.CategoryID, input.TransactionDate); err != nil {
		return nil, err
	}

	existing, err := s.txRepo.FindByID(ctx, input.ID, input.UserID)
	if err != nil {
		return nil, err
	}

	var desc *string
	if input.Description != nil {
		trimmed := strings.TrimSpace(*input.Description)
		if trimmed != "" {
			desc = &trimmed
		}
	}

	existing.Amount = input.Amount
	existing.Type = input.Type
	existing.CategoryID = input.CategoryID
	existing.TransactionDate = input.TransactionDate
	existing.Description = desc

	if err := s.txRepo.Update(ctx, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

// DeleteTransaction removes a transaction while verifying user ownership.
func (s *TransactionService) DeleteTransaction(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	return s.txRepo.Delete(ctx, id, userID)
}

// ListTransactions validates filter parameters, executes paginated query, and returns pagination metadata.
func (s *TransactionService) ListTransactions(ctx context.Context, input ListTransactionsInput) (*PaginatedTransactionsOutput, error) {
	page := input.Page
	if page <= 0 {
		page = 1
	}

	limit := input.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		return nil, domain.ValidationError{Field: "limit", Message: "limit must not exceed 100"}
	}

	if input.CategoryID != nil && input.Uncategorized {
		return nil, domain.ValidationError{Field: "uncategorized", Message: "cannot combine category_id and uncategorized"}
	}

	if input.Type != nil && *input.Type != "" {
		if *input.Type != domain.TransactionTypeIncome && *input.Type != domain.TransactionTypeExpense {
			return nil, domain.ValidationError{Field: "type", Message: "type must be INCOME or EXPENSE"}
		}
	}

	var startDate, endDate *time.Time
	if input.StartDate != nil && *input.StartDate != "" {
		t, err := time.Parse("2006-01-02", *input.StartDate)
		if err != nil {
			return nil, domain.ValidationError{Field: "start_date", Message: "start_date must be in YYYY-MM-DD format"}
		}
		startDate = &t
	}

	if input.EndDate != nil && *input.EndDate != "" {
		t, err := time.Parse("2006-01-02", *input.EndDate)
		if err != nil {
			return nil, domain.ValidationError{Field: "end_date", Message: "end_date must be in YYYY-MM-DD format"}
		}
		endDate = &t
	}

	if startDate != nil && endDate != nil {
		if startDate.After(*endDate) {
			return nil, domain.ValidationError{Field: "start_date", Message: "start_date must be before or equal to end_date"}
		}
	}

	offset := (page - 1) * limit

	repoFilter := repository.ListTransactionsFilter{
		UserID:        input.UserID,
		StartDate:     input.StartDate,
		EndDate:       input.EndDate,
		Type:          input.Type,
		CategoryID:    input.CategoryID,
		Uncategorized: input.Uncategorized,
		Limit:         limit,
		Offset:        offset,
	}

	res, err := s.txRepo.List(ctx, repoFilter)
	if err != nil {
		return nil, err
	}

	totalPages := 0
	if res.TotalCount > 0 {
		totalPages = (res.TotalCount + limit - 1) / limit
	}

	return &PaginatedTransactionsOutput{
		Items: res.Transactions,
		Pagination: PaginationInfo{
			Page:       page,
			Limit:      limit,
			TotalItems: res.TotalCount,
			TotalPages: totalPages,
		},
	}, nil
}
