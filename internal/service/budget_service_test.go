package service_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/service"
)

type fakeBudgetRepository struct {
	mu      sync.Mutex
	budgets map[uuid.UUID]*domain.Budget
	spent   map[string]int64 // key: fmt.Sprintf("%s:%d:%d", catID, year, month)
}

func newFakeBudgetRepository() *fakeBudgetRepository {
	return &fakeBudgetRepository{
		budgets: make(map[uuid.UUID]*domain.Budget),
		spent:   make(map[string]int64),
	}
}

func (r *fakeBudgetRepository) Upsert(ctx context.Context, budget *domain.Budget) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.budgets {
		if existing.UserID == budget.UserID &&
			existing.CategoryID == budget.CategoryID &&
			existing.Year == budget.Year &&
			existing.Month == budget.Month {
			existing.Amount = budget.Amount
			existing.UpdatedAt = time.Now().UTC()
			budget.ID = existing.ID
			budget.CreatedAt = existing.CreatedAt
			budget.UpdatedAt = existing.UpdatedAt
			return nil
		}
	}

	if budget.ID == uuid.Nil {
		budget.ID = uuid.New()
	}
	now := time.Now().UTC()
	budget.CreatedAt = now
	budget.UpdatedAt = now
	copied := *budget
	r.budgets[budget.ID] = &copied
	return nil
}

func (r *fakeBudgetRepository) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Budget, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	b, exists := r.budgets[id]
	if !exists || b.UserID != userID {
		return nil, domain.ErrNotFound
	}
	copied := *b
	return &copied, nil
}

func (r *fakeBudgetRepository) FindByCategoryAndMonth(ctx context.Context, userID uuid.UUID, categoryID uuid.UUID, year int, month int) (*domain.Budget, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, b := range r.budgets {
		if b.UserID == userID && b.CategoryID == categoryID && b.Year == year && b.Month == month {
			copied := *b
			return &copied, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (r *fakeBudgetRepository) ListByMonth(ctx context.Context, userID uuid.UUID, year int, month int) ([]domain.Budget, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var result []domain.Budget
	for _, b := range r.budgets {
		if b.UserID == userID && b.Year == year && b.Month == month {
			copied := *b
			key := fmt.Sprintf("%s:%d:%d", b.CategoryID, year, month)
			copied.Spent = r.spent[key]
			result = append(result, copied)
		}
	}
	return result, nil
}

func (r *fakeBudgetRepository) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	b, exists := r.budgets[id]
	if !exists || b.UserID != userID {
		return domain.ErrNotFound
	}
	delete(r.budgets, id)
	return nil
}

func (r *fakeBudgetRepository) GetSpent(ctx context.Context, userID uuid.UUID, categoryID uuid.UUID, year int, month int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := fmt.Sprintf("%s:%d:%d", categoryID, year, month)
	return r.spent[key], nil
}

func TestBudgetService_Upsert_Success(t *testing.T) {
	catRepo := newFakeCategoryRepository()
	budgetRepo := newFakeBudgetRepository()
	svc := service.NewBudgetService(budgetRepo, catRepo)
	ctx := context.Background()

	userID := uuid.New()
	cat := &domain.Category{
		ID:     uuid.New(),
		UserID: userID,
		Name:   "Dining",
		Type:   domain.CategoryTypeExpense,
	}
	_ = catRepo.Create(ctx, cat)

	b, err := svc.UpsertBudget(ctx, service.UpsertBudgetInput{
		UserID:     userID,
		CategoryID: cat.ID,
		Amount:     5000000,
		Year:       2026,
		Month:      9,
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if b.Amount != 5000000 {
		t.Errorf("expected amount 5000000, got %d", b.Amount)
	}
	if b.CategoryName != "Dining" {
		t.Errorf("expected category name Dining, got %s", b.CategoryName)
	}
}

func TestBudgetService_Upsert_RejectsIncomeCategory(t *testing.T) {
	catRepo := newFakeCategoryRepository()
	budgetRepo := newFakeBudgetRepository()
	svc := service.NewBudgetService(budgetRepo, catRepo)
	ctx := context.Background()

	userID := uuid.New()
	incomeCat := &domain.Category{
		ID:     uuid.New(),
		UserID: userID,
		Name:   "Salary",
		Type:   domain.CategoryTypeIncome,
	}
	_ = catRepo.Create(ctx, incomeCat)

	_, err := svc.UpsertBudget(ctx, service.UpsertBudgetInput{
		UserID:     userID,
		CategoryID: incomeCat.ID,
		Amount:     20000000,
		Year:       2026,
		Month:      9,
	})
	if err == nil {
		t.Fatal("expected error when setting budget on INCOME category, got nil")
	}

	var valErr domain.ValidationError
	if !errors.As(err, &valErr) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if valErr.Field != "category_id" {
		t.Errorf("expected error field category_id, got %s", valErr.Field)
	}
}

func TestBudgetService_Upsert_CategoryBelongsToAnotherUser(t *testing.T) {
	catRepo := newFakeCategoryRepository()
	budgetRepo := newFakeBudgetRepository()
	svc := service.NewBudgetService(budgetRepo, catRepo)
	ctx := context.Background()

	user1 := uuid.New()
	user2 := uuid.New()

	cat := &domain.Category{
		ID:     uuid.New(),
		UserID: user1,
		Name:   "Dining",
		Type:   domain.CategoryTypeExpense,
	}
	_ = catRepo.Create(ctx, cat)

	// User 2 tries to set budget on User 1's category
	_, err := svc.UpsertBudget(ctx, service.UpsertBudgetInput{
		UserID:     user2,
		CategoryID: cat.ID,
		Amount:     3000000,
		Year:       2026,
		Month:      9,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var valErr domain.ValidationError
	if !errors.As(err, &valErr) || valErr.Field != "category_id" {
		t.Errorf("expected ValidationError on category_id, got %v", err)
	}
}

func TestBudgetService_Upsert_ValidationErrors(t *testing.T) {
	catRepo := newFakeCategoryRepository()
	budgetRepo := newFakeBudgetRepository()
	svc := service.NewBudgetService(budgetRepo, catRepo)
	ctx := context.Background()

	userID := uuid.New()
	cat := &domain.Category{
		ID:     uuid.New(),
		UserID: userID,
		Name:   "Groceries",
		Type:   domain.CategoryTypeExpense,
	}
	_ = catRepo.Create(ctx, cat)

	tests := []struct {
		name          string
		input         service.UpsertBudgetInput
		expectedField string
	}{
		{
			name: "zero amount",
			input: service.UpsertBudgetInput{
				UserID:     userID,
				CategoryID: cat.ID,
				Amount:     0,
				Year:       2026,
				Month:      9,
			},
			expectedField: "amount",
		},
		{
			name: "negative amount",
			input: service.UpsertBudgetInput{
				UserID:     userID,
				CategoryID: cat.ID,
				Amount:     -1000,
				Year:       2026,
				Month:      9,
			},
			expectedField: "amount",
		},
		{
			name: "year before 2000",
			input: service.UpsertBudgetInput{
				UserID:     userID,
				CategoryID: cat.ID,
				Amount:     100000,
				Year:       1999,
				Month:      9,
			},
			expectedField: "year",
		},
		{
			name: "year after 2100",
			input: service.UpsertBudgetInput{
				UserID:     userID,
				CategoryID: cat.ID,
				Amount:     100000,
				Year:       2101,
				Month:      9,
			},
			expectedField: "year",
		},
		{
			name: "month 0",
			input: service.UpsertBudgetInput{
				UserID:     userID,
				CategoryID: cat.ID,
				Amount:     100000,
				Year:       2026,
				Month:      0,
			},
			expectedField: "month",
		},
		{
			name: "month 13",
			input: service.UpsertBudgetInput{
				UserID:     userID,
				CategoryID: cat.ID,
				Amount:     100000,
				Year:       2026,
				Month:      13,
			},
			expectedField: "month",
		},
		{
			name: "nil category",
			input: service.UpsertBudgetInput{
				UserID:     userID,
				CategoryID: uuid.Nil,
				Amount:     100000,
				Year:       2026,
				Month:      9,
			},
			expectedField: "category_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.UpsertBudget(ctx, tt.input)
			if err == nil {
				t.Fatalf("expected error on %s, got nil", tt.expectedField)
			}
			var valErr domain.ValidationError
			if !errors.As(err, &valErr) || valErr.Field != tt.expectedField {
				t.Errorf("expected ValidationError on %s, got %v", tt.expectedField, err)
			}
		})
	}
}

func TestBudgetService_ListByMonth(t *testing.T) {
	catRepo := newFakeCategoryRepository()
	budgetRepo := newFakeBudgetRepository()
	svc := service.NewBudgetService(budgetRepo, catRepo)
	ctx := context.Background()

	userID := uuid.New()
	cat1 := &domain.Category{ID: uuid.New(), UserID: userID, Name: "Dining", Type: domain.CategoryTypeExpense}
	cat2 := &domain.Category{ID: uuid.New(), UserID: userID, Name: "Cinema", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(ctx, cat1)
	_ = catRepo.Create(ctx, cat2)

	_, _ = svc.UpsertBudget(ctx, service.UpsertBudgetInput{UserID: userID, CategoryID: cat1.ID, Amount: 3000000, Year: 2026, Month: 9})
	_, _ = svc.UpsertBudget(ctx, service.UpsertBudgetInput{UserID: userID, CategoryID: cat2.ID, Amount: 1000000, Year: 2026, Month: 9})

	// Inject spent into fake
	budgetRepo.spent[fmt.Sprintf("%s:2026:9", cat1.ID)] = 750000

	budgets, err := svc.ListBudgetsByMonth(ctx, userID, 2026, 9)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(budgets) != 2 {
		t.Errorf("expected 2 budgets, got %d", len(budgets))
	}
}

func TestBudgetService_DeleteBudget(t *testing.T) {
	catRepo := newFakeCategoryRepository()
	budgetRepo := newFakeBudgetRepository()
	svc := service.NewBudgetService(budgetRepo, catRepo)
	ctx := context.Background()

	userID := uuid.New()
	cat := &domain.Category{ID: uuid.New(), UserID: userID, Name: "Dining", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(ctx, cat)

	b, _ := svc.UpsertBudget(ctx, service.UpsertBudgetInput{UserID: userID, CategoryID: cat.ID, Amount: 3000000, Year: 2026, Month: 9})

	if err := svc.DeleteBudget(ctx, b.ID, userID); err != nil {
		t.Fatalf("failed to delete budget: %v", err)
	}

	if err := svc.DeleteBudget(ctx, b.ID, userID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound on second delete, got %v", err)
	}
}
