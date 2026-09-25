package service_test

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/repository"
	"github.com/hoangduc102/fintrack/internal/service"
)

// fakeTransactionRepository is an in-memory test double for repository.TransactionRepository.
type fakeTransactionRepository struct {
	mu           sync.Mutex
	transactions map[uuid.UUID]*domain.Transaction
}

func newFakeTransactionRepository() *fakeTransactionRepository {
	return &fakeTransactionRepository{
		transactions: make(map[uuid.UUID]*domain.Transaction),
	}
}

func (r *fakeTransactionRepository) Create(ctx context.Context, tx *domain.Transaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if tx.ID == uuid.Nil {
		tx.ID = uuid.New()
	}
	now := time.Now().UTC()
	tx.CreatedAt = now
	tx.UpdatedAt = now

	copied := *tx
	r.transactions[tx.ID] = &copied
	return nil
}

func (r *fakeTransactionRepository) CreateBatch(ctx context.Context, txs []domain.Transaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	for i := range txs {
		tx := txs[i]
		if tx.ID == uuid.Nil {
			tx.ID = uuid.New()
		}
		if tx.CreatedAt.IsZero() {
			tx.CreatedAt = now
		}
		if tx.UpdatedAt.IsZero() {
			tx.UpdatedAt = now
		}
		copied := tx
		r.transactions[tx.ID] = &copied
	}
	return nil
}

func (r *fakeTransactionRepository) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Transaction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tx, exists := r.transactions[id]
	if !exists || tx.UserID != userID {
		return nil, domain.ErrNotFound
	}
	copied := *tx
	return &copied, nil
}

func (r *fakeTransactionRepository) List(ctx context.Context, filter repository.ListTransactionsFilter) (*repository.ListTransactionsResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var matched []domain.Transaction
	for _, tx := range r.transactions {
		if tx.UserID != filter.UserID {
			continue
		}
		if filter.StartDate != nil && *filter.StartDate != "" && tx.TransactionDate < *filter.StartDate {
			continue
		}
		if filter.EndDate != nil && *filter.EndDate != "" && tx.TransactionDate > *filter.EndDate {
			continue
		}
		if filter.Type != nil && *filter.Type != "" && tx.Type != *filter.Type {
			continue
		}
		if filter.Uncategorized {
			if tx.CategoryID != nil {
				continue
			}
		} else if filter.CategoryID != nil && *filter.CategoryID != uuid.Nil {
			if tx.CategoryID == nil || *tx.CategoryID != *filter.CategoryID {
				continue
			}
		}
		matched = append(matched, *tx)
	}

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].TransactionDate != matched[j].TransactionDate {
			return matched[i].TransactionDate > matched[j].TransactionDate
		}
		if !matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].CreatedAt.After(matched[j].CreatedAt)
		}
		return matched[i].ID.String() > matched[j].ID.String()
	})

	totalCount := len(matched)
	offset := filter.Offset
	if offset > totalCount {
		offset = totalCount
	}
	end := offset + filter.Limit
	if end > totalCount {
		end = totalCount
	}

	var paginated []domain.Transaction
	if offset < totalCount {
		paginated = matched[offset:end]
	} else {
		paginated = []domain.Transaction{}
	}

	return &repository.ListTransactionsResult{
		Transactions: paginated,
		TotalCount:   totalCount,
	}, nil
}

func (r *fakeTransactionRepository) Update(ctx context.Context, tx *domain.Transaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.transactions[tx.ID]
	if !exists || existing.UserID != tx.UserID {
		return domain.ErrNotFound
	}

	tx.UpdatedAt = time.Now().UTC()
	copied := *tx
	r.transactions[tx.ID] = &copied
	return nil
}

func (r *fakeTransactionRepository) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.transactions[id]
	if !exists || existing.UserID != userID {
		return domain.ErrNotFound
	}

	delete(r.transactions, id)
	return nil
}

func ptrString(s string) *string {
	return &s
}

func TestTransactionService_Create_Success(t *testing.T) {
	catRepo := newFakeCategoryRepository()
	txRepo := newFakeTransactionRepository()
	svc := service.NewTransactionService(txRepo, catRepo)
	ctx := context.Background()
	userID := uuid.New()

	// Seed user category
	cat := &domain.Category{
		UserID: userID,
		Name:   "Food",
		Type:   domain.CategoryTypeExpense,
	}
	_ = catRepo.Create(ctx, cat)

	tx, err := svc.CreateTransaction(ctx, service.CreateTransactionInput{
		UserID:          userID,
		Amount:          50000,
		Type:            domain.TransactionTypeExpense,
		CategoryID:      &cat.ID,
		TransactionDate: "2026-09-25",
		Description:     ptrString("Lunch"),
	})
	if err != nil {
		t.Fatalf("expected create to succeed, got %v", err)
	}

	if tx.Amount != 50000 {
		t.Errorf("expected amount 50000, got %d", tx.Amount)
	}
	if tx.CategoryID == nil || *tx.CategoryID != cat.ID {
		t.Errorf("expected category ID %v, got %v", cat.ID, tx.CategoryID)
	}
}

func TestTransactionService_Create_Uncategorized_Success(t *testing.T) {
	catRepo := newFakeCategoryRepository()
	txRepo := newFakeTransactionRepository()
	svc := service.NewTransactionService(txRepo, catRepo)
	ctx := context.Background()
	userID := uuid.New()

	tx, err := svc.CreateTransaction(ctx, service.CreateTransactionInput{
		UserID:          userID,
		Amount:          100000,
		Type:            domain.TransactionTypeIncome,
		CategoryID:      nil,
		TransactionDate: "2026-09-25",
	})
	if err != nil {
		t.Fatalf("expected uncategorized create to succeed, got %v", err)
	}

	if tx.CategoryID != nil {
		t.Errorf("expected nil CategoryID for uncategorized transaction, got %v", tx.CategoryID)
	}
}

func TestTransactionService_Create_ValidationErrors(t *testing.T) {
	catRepo := newFakeCategoryRepository()
	txRepo := newFakeTransactionRepository()
	svc := service.NewTransactionService(txRepo, catRepo)
	ctx := context.Background()
	user1 := uuid.New()
	user2 := uuid.New()

	// Seed categories for testing
	catExpenseUser1 := &domain.Category{UserID: user1, Name: "Food", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(ctx, catExpenseUser1)

	catIncomeUser1 := &domain.Category{UserID: user1, Name: "Salary", Type: domain.CategoryTypeIncome}
	_ = catRepo.Create(ctx, catIncomeUser1)

	catUser2 := &domain.Category{UserID: user2, Name: "User2 Cat", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(ctx, catUser2)

	nonExistentCatID := uuid.New()

	tests := []struct {
		name          string
		input         service.CreateTransactionInput
		expectedField string
	}{
		{
			name: "zero amount",
			input: service.CreateTransactionInput{
				UserID:          user1,
				Amount:          0,
				Type:            domain.TransactionTypeExpense,
				TransactionDate: "2026-09-25",
			},
			expectedField: "amount",
		},
		{
			name: "negative amount",
			input: service.CreateTransactionInput{
				UserID:          user1,
				Amount:          -50000,
				Type:            domain.TransactionTypeExpense,
				TransactionDate: "2026-09-25",
			},
			expectedField: "amount",
		},
		{
			name: "invalid type",
			input: service.CreateTransactionInput{
				UserID:          user1,
				Amount:          50000,
				Type:            domain.TransactionType("INVALID"),
				TransactionDate: "2026-09-25",
			},
			expectedField: "type",
		},
		{
			name: "invalid date format",
			input: service.CreateTransactionInput{
				UserID:          user1,
				Amount:          50000,
				Type:            domain.TransactionTypeExpense,
				TransactionDate: "25-09-2026",
			},
			expectedField: "transaction_date",
		},
		{
			name: "empty date",
			input: service.CreateTransactionInput{
				UserID:          user1,
				Amount:          50000,
				Type:            domain.TransactionTypeExpense,
				TransactionDate: "",
			},
			expectedField: "transaction_date",
		},
		{
			name: "category belongs to another user",
			input: service.CreateTransactionInput{
				UserID:          user1,
				Amount:          50000,
				Type:            domain.TransactionTypeExpense,
				CategoryID:      &catUser2.ID,
				TransactionDate: "2026-09-25",
			},
			expectedField: "category_id",
		},
		{
			name: "non-existent category",
			input: service.CreateTransactionInput{
				UserID:          user1,
				Amount:          50000,
				Type:            domain.TransactionTypeExpense,
				CategoryID:      &nonExistentCatID,
				TransactionDate: "2026-09-25",
			},
			expectedField: "category_id",
		},
		{
			name: "category type mismatch (expense tx with income category)",
			input: service.CreateTransactionInput{
				UserID:          user1,
				Amount:          50000,
				Type:            domain.TransactionTypeExpense,
				CategoryID:      &catIncomeUser1.ID,
				TransactionDate: "2026-09-25",
			},
			expectedField: "category_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateTransaction(ctx, tt.input)
			if err == nil {
				t.Fatalf("expected validation error on field %q, got nil", tt.expectedField)
			}
			var valErr domain.ValidationError
			if !errors.As(err, &valErr) {
				t.Fatalf("expected domain.ValidationError, got %v", err)
			}
			if valErr.Field != tt.expectedField {
				t.Errorf("expected error field %q, got %q", tt.expectedField, valErr.Field)
			}
		})
	}
}

func TestTransactionService_GetTransaction(t *testing.T) {
	catRepo := newFakeCategoryRepository()
	txRepo := newFakeTransactionRepository()
	svc := service.NewTransactionService(txRepo, catRepo)
	ctx := context.Background()
	user1 := uuid.New()
	user2 := uuid.New()

	tx, _ := svc.CreateTransaction(ctx, service.CreateTransactionInput{
		UserID:          user1,
		Amount:          20000,
		Type:            domain.TransactionTypeExpense,
		TransactionDate: "2026-09-25",
	})

	// Get by owner
	found, err := svc.GetTransaction(ctx, tx.ID, user1)
	if err != nil {
		t.Fatalf("expected get to succeed, got %v", err)
	}
	if found.ID != tx.ID {
		t.Errorf("expected ID %v, got %v", tx.ID, found.ID)
	}

	// Get by non-owner returns ErrNotFound
	_, err = svc.GetTransaction(ctx, tx.ID, user2)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound for non-owner, got %v", err)
	}
}

func TestTransactionService_UpdateTransaction(t *testing.T) {
	catRepo := newFakeCategoryRepository()
	txRepo := newFakeTransactionRepository()
	svc := service.NewTransactionService(txRepo, catRepo)
	ctx := context.Background()
	user1 := uuid.New()
	user2 := uuid.New()

	tx, _ := svc.CreateTransaction(ctx, service.CreateTransactionInput{
		UserID:          user1,
		Amount:          20000,
		Type:            domain.TransactionTypeExpense,
		TransactionDate: "2026-09-25",
	})

	// Update by owner
	updated, err := svc.UpdateTransaction(ctx, service.UpdateTransactionInput{
		ID:              tx.ID,
		UserID:          user1,
		Amount:          30000,
		Type:            domain.TransactionTypeExpense,
		TransactionDate: "2026-09-26",
		Description:     ptrString("Updated description"),
	})
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}
	if updated.Amount != 30000 {
		t.Errorf("expected amount 30000, got %d", updated.Amount)
	}

	// Update with wrong user returns ErrNotFound
	_, err = svc.UpdateTransaction(ctx, service.UpdateTransactionInput{
		ID:              tx.ID,
		UserID:          user2,
		Amount:          40000,
		Type:            domain.TransactionTypeExpense,
		TransactionDate: "2026-09-26",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for wrong user, got %v", err)
	}
}

func TestTransactionService_DeleteTransaction(t *testing.T) {
	catRepo := newFakeCategoryRepository()
	txRepo := newFakeTransactionRepository()
	svc := service.NewTransactionService(txRepo, catRepo)
	ctx := context.Background()
	user1 := uuid.New()
	user2 := uuid.New()

	tx, _ := svc.CreateTransaction(ctx, service.CreateTransactionInput{
		UserID:          user1,
		Amount:          20000,
		Type:            domain.TransactionTypeExpense,
		TransactionDate: "2026-09-25",
	})

	// Delete by non-owner returns ErrNotFound
	err := svc.DeleteTransaction(ctx, tx.ID, user2)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound for non-owner delete, got %v", err)
	}

	// Delete by owner succeeds
	err = svc.DeleteTransaction(ctx, tx.ID, user1)
	if err != nil {
		t.Fatalf("expected delete to succeed, got %v", err)
	}

	// Second delete returns ErrNotFound
	err = svc.DeleteTransaction(ctx, tx.ID, user1)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound on second delete, got %v", err)
	}
}

func TestTransactionService_ListTransactions_DefaultsAndPagination(t *testing.T) {
	catRepo := newFakeCategoryRepository()
	txRepo := newFakeTransactionRepository()
	svc := service.NewTransactionService(txRepo, catRepo)
	ctx := context.Background()
	userID := uuid.New()

	for i := 1; i <= 25; i++ {
		_, _ = svc.CreateTransaction(ctx, service.CreateTransactionInput{
			UserID:          userID,
			Amount:          int64(i * 1000),
			Type:            domain.TransactionTypeExpense,
			TransactionDate: "2026-09-25",
		})
	}

	// Default page=0, limit=0 should default to page=1, limit=20
	out1, err := svc.ListTransactions(ctx, service.ListTransactionsInput{
		UserID: userID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out1.Pagination.Page != 1 {
		t.Errorf("expected page 1, got %d", out1.Pagination.Page)
	}
	if out1.Pagination.Limit != 20 {
		t.Errorf("expected limit 20, got %d", out1.Pagination.Limit)
	}
	if out1.Pagination.TotalItems != 25 {
		t.Errorf("expected total_items 25, got %d", out1.Pagination.TotalItems)
	}
	if out1.Pagination.TotalPages != 2 {
		t.Errorf("expected total_pages 2, got %d", out1.Pagination.TotalPages)
	}
	if len(out1.Items) != 20 {
		t.Errorf("expected 20 items on page 1, got %d", len(out1.Items))
	}

	// Page 2: limit 20
	out2, err := svc.ListTransactions(ctx, service.ListTransactionsInput{
		UserID: userID,
		Page:   2,
		Limit:  20,
	})
	if err != nil {
		t.Fatalf("unexpected error on page 2: %v", err)
	}
	if len(out2.Items) != 5 {
		t.Errorf("expected 5 items on page 2, got %d", len(out2.Items))
	}
}

func TestTransactionService_ListTransactions_ValidationErrors(t *testing.T) {
	catRepo := newFakeCategoryRepository()
	txRepo := newFakeTransactionRepository()
	svc := service.NewTransactionService(txRepo, catRepo)
	ctx := context.Background()
	userID := uuid.New()

	catID := uuid.New()
	invalidDate := "invalid-date"
	startDate := "2026-09-25"
	endDateBeforeStart := "2026-09-20"
	invalidType := domain.TransactionType("INVALID")

	tests := []struct {
		name          string
		input         service.ListTransactionsInput
		expectedField string
	}{
		{
			name: "limit exceeds 100",
			input: service.ListTransactionsInput{
				UserID: userID,
				Limit:  101,
			},
			expectedField: "limit",
		},
		{
			name: "mutually exclusive category_id and uncategorized",
			input: service.ListTransactionsInput{
				UserID:        userID,
				CategoryID:    &catID,
				Uncategorized: true,
			},
			expectedField: "uncategorized",
		},
		{
			name: "invalid start_date format",
			input: service.ListTransactionsInput{
				UserID:    userID,
				StartDate: &invalidDate,
			},
			expectedField: "start_date",
		},
		{
			name: "invalid end_date format",
			input: service.ListTransactionsInput{
				UserID:  userID,
				EndDate: &invalidDate,
			},
			expectedField: "end_date",
		},
		{
			name: "start_date greater than end_date",
			input: service.ListTransactionsInput{
				UserID:    userID,
				StartDate: &startDate,
				EndDate:   &endDateBeforeStart,
			},
			expectedField: "start_date",
		},
		{
			name: "invalid type",
			input: service.ListTransactionsInput{
				UserID: userID,
				Type:   &invalidType,
			},
			expectedField: "type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.ListTransactions(ctx, tt.input)
			if err == nil {
				t.Fatalf("expected validation error on %s, got nil", tt.expectedField)
			}
			var valErr domain.ValidationError
			if !errors.As(err, &valErr) || valErr.Field != tt.expectedField {
				t.Errorf("expected validation error on %s, got %v", tt.expectedField, err)
			}
		})
	}
}
