package service_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/service"
)

// fakeCategoryRepository is an in-memory test double for repository.CategoryRepository.
type fakeCategoryRepository struct {
	mu         sync.Mutex
	categories map[uuid.UUID]*domain.Category
}

func newFakeCategoryRepository() *fakeCategoryRepository {
	return &fakeCategoryRepository{
		categories: make(map[uuid.UUID]*domain.Category),
	}
}

func (r *fakeCategoryRepository) Create(ctx context.Context, category *domain.Category) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, c := range r.categories {
		if c.UserID == category.UserID && c.Type == category.Type && strings.EqualFold(c.Name, category.Name) {
			return domain.ErrConflict
		}
	}

	if category.ID == uuid.Nil {
		category.ID = uuid.New()
	}
	now := time.Now().UTC()
	category.CreatedAt = now
	category.UpdatedAt = now

	copied := *category
	r.categories[category.ID] = &copied
	return nil
}

func (r *fakeCategoryRepository) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Category, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cat, exists := r.categories[id]
	if !exists || cat.UserID != userID {
		return nil, domain.ErrNotFound
	}
	copied := *cat
	return &copied, nil
}

func (r *fakeCategoryRepository) FindByUserAndType(ctx context.Context, userID uuid.UUID, categoryType *domain.CategoryType) ([]domain.Category, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var result []domain.Category
	for _, c := range r.categories {
		if c.UserID == userID {
			if categoryType == nil || c.Type == *categoryType {
				result = append(result, *c)
			}
		}
	}
	return result, nil
}

func (r *fakeCategoryRepository) Update(ctx context.Context, category *domain.Category) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cat, exists := r.categories[category.ID]
	if !exists || cat.UserID != category.UserID {
		return domain.ErrNotFound
	}

	for _, c := range r.categories {
		if c.ID != category.ID && c.UserID == category.UserID && c.Type == cat.Type && strings.EqualFold(c.Name, category.Name) {
			return domain.ErrConflict
		}
	}

	cat.Name = category.Name
	cat.UpdatedAt = time.Now().UTC()
	category.UpdatedAt = cat.UpdatedAt
	return nil
}

func (r *fakeCategoryRepository) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cat, exists := r.categories[id]
	if !exists || cat.UserID != userID {
		return domain.ErrNotFound
	}
	delete(r.categories, id)
	return nil
}

func TestCategoryService_CreateCategory_Success(t *testing.T) {
	repo := newFakeCategoryRepository()
	svc := service.NewCategoryService(repo)
	ctx := context.Background()
	userID := uuid.New()

	cat, err := svc.CreateCategory(ctx, service.CreateCategoryInput{
		UserID: userID,
		Name:   "  Groceries  ",
		Type:   domain.CategoryTypeExpense,
	})
	if err != nil {
		t.Fatalf("expected create to succeed, got %v", err)
	}

	if cat.Name != "Groceries" {
		t.Errorf("expected trimmed name 'Groceries', got %q", cat.Name)
	}
	if cat.Type != domain.CategoryTypeExpense {
		t.Errorf("expected type EXPENSE, got %v", cat.Type)
	}
	if cat.UserID != userID {
		t.Errorf("expected userID %v, got %v", userID, cat.UserID)
	}
}

func TestCategoryService_CreateCategory_ValidationErrors(t *testing.T) {
	repo := newFakeCategoryRepository()
	svc := service.NewCategoryService(repo)
	ctx := context.Background()
	userID := uuid.New()

	tests := []struct {
		name          string
		input         service.CreateCategoryInput
		expectedField string
	}{
		{
			name: "empty name",
			input: service.CreateCategoryInput{
				UserID: userID,
				Name:   "",
				Type:   domain.CategoryTypeExpense,
			},
			expectedField: "name",
		},
		{
			name: "whitespace name",
			input: service.CreateCategoryInput{
				UserID: userID,
				Name:   "   ",
				Type:   domain.CategoryTypeExpense,
			},
			expectedField: "name",
		},
		{
			name: "name exceeds 50 chars",
			input: service.CreateCategoryInput{
				UserID: userID,
				Name:   strings.Repeat("a", 51),
				Type:   domain.CategoryTypeExpense,
			},
			expectedField: "name",
		},
		{
			name: "invalid type",
			input: service.CreateCategoryInput{
				UserID: userID,
				Name:   "Investments",
				Type:   domain.CategoryType("INVALID"),
			},
			expectedField: "type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.CreateCategory(ctx, tt.input)
			if err == nil {
				t.Fatalf("expected validation error on field %q, got nil", tt.expectedField)
			}

			var valErr domain.ValidationError
			if !errors.As(err, &valErr) {
				t.Fatalf("expected ValidationError, got %v", err)
			}
			if valErr.Field != tt.expectedField {
				t.Errorf("expected error field %q, got %q", tt.expectedField, valErr.Field)
			}
		})
	}
}

func TestCategoryService_CreateCategory_NameBoundaryLengths(t *testing.T) {
	repo := newFakeCategoryRepository()
	svc := service.NewCategoryService(repo)
	ctx := context.Background()
	userID := uuid.New()

	// 1 char name passes
	cat1, err := svc.CreateCategory(ctx, service.CreateCategoryInput{
		UserID: userID,
		Name:   "A",
		Type:   domain.CategoryTypeExpense,
	})
	if err != nil {
		t.Fatalf("expected 1-char name to pass, got %v", err)
	}
	if cat1.Name != "A" {
		t.Errorf("expected 'A', got %q", cat1.Name)
	}

	// Exactly 50 chars name passes
	name50 := strings.Repeat("B", 50)
	cat50, err := svc.CreateCategory(ctx, service.CreateCategoryInput{
		UserID: userID,
		Name:   name50,
		Type:   domain.CategoryTypeIncome,
	})
	if err != nil {
		t.Fatalf("expected 50-char name to pass, got %v", err)
	}
	if cat50.Name != name50 {
		t.Errorf("expected 50-char name, got %q", cat50.Name)
	}
}

func TestCategoryService_CreateCategory_DuplicateConflict(t *testing.T) {
	repo := newFakeCategoryRepository()
	svc := service.NewCategoryService(repo)
	ctx := context.Background()
	userID := uuid.New()

	_, err := svc.CreateCategory(ctx, service.CreateCategoryInput{
		UserID: userID,
		Name:   "Housing",
		Type:   domain.CategoryTypeExpense,
	})
	if err != nil {
		t.Fatalf("expected first create to succeed, got %v", err)
	}

	// Duplicate with different casing
	_, err = svc.CreateCategory(ctx, service.CreateCategoryInput{
		UserID: userID,
		Name:   "housing",
		Type:   domain.CategoryTypeExpense,
	})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("expected domain.ErrConflict, got %v", err)
	}
}

func TestCategoryService_ListCategories(t *testing.T) {
	repo := newFakeCategoryRepository()
	svc := service.NewCategoryService(repo)
	ctx := context.Background()
	user1 := uuid.New()
	user2 := uuid.New()

	_, _ = svc.CreateCategory(ctx, service.CreateCategoryInput{UserID: user1, Name: "Food", Type: domain.CategoryTypeExpense})
	_, _ = svc.CreateCategory(ctx, service.CreateCategoryInput{UserID: user1, Name: "Salary", Type: domain.CategoryTypeIncome})
	_, _ = svc.CreateCategory(ctx, service.CreateCategoryInput{UserID: user2, Name: "Rent", Type: domain.CategoryTypeExpense})

	// List all for user1
	all, err := svc.ListCategories(ctx, service.ListCategoriesInput{UserID: user1})
	if err != nil {
		t.Fatalf("unexpected error listing categories: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 categories for user1, got %d", len(all))
	}

	// Filter by expense
	expenseType := domain.CategoryTypeExpense
	expenses, err := svc.ListCategories(ctx, service.ListCategoriesInput{UserID: user1, Type: &expenseType})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(expenses) != 1 || expenses[0].Name != "Food" {
		t.Errorf("expected 1 expense category 'Food', got %+v", expenses)
	}

	// Invalid type filter
	invalidType := domain.CategoryType("INVALID")
	_, err = svc.ListCategories(ctx, service.ListCategoriesInput{UserID: user1, Type: &invalidType})
	if err == nil {
		t.Fatal("expected validation error for invalid type filter, got nil")
	}
	var valErr domain.ValidationError
	if !errors.As(err, &valErr) || valErr.Field != "type" {
		t.Errorf("expected validation error on field 'type', got %v", err)
	}
}

func TestCategoryService_UpdateCategory(t *testing.T) {
	repo := newFakeCategoryRepository()
	svc := service.NewCategoryService(repo)
	ctx := context.Background()
	user1 := uuid.New()
	user2 := uuid.New()

	cat, err := svc.CreateCategory(ctx, service.CreateCategoryInput{
		UserID: user1,
		Name:   "Dining",
		Type:   domain.CategoryTypeExpense,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Update name succeeds
	updated, err := svc.UpdateCategory(ctx, service.UpdateCategoryInput{
		ID:     cat.ID,
		UserID: user1,
		Name:   "  Dining Out  ",
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if updated.Name != "Dining Out" {
		t.Errorf("expected trimmed name 'Dining Out', got %q", updated.Name)
	}

	// Update validation: empty name
	_, err = svc.UpdateCategory(ctx, service.UpdateCategoryInput{
		ID:     cat.ID,
		UserID: user1,
		Name:   "   ",
	})
	if err == nil {
		t.Fatal("expected validation error on empty name, got nil")
	}

	// Update with wrong user returns ErrNotFound
	_, err = svc.UpdateCategory(ctx, service.UpdateCategoryInput{
		ID:     cat.ID,
		UserID: user2,
		Name:   "Hacked",
	})
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for wrong user, got %v", err)
	}
}

func TestCategoryService_DeleteCategory(t *testing.T) {
	repo := newFakeCategoryRepository()
	svc := service.NewCategoryService(repo)
	ctx := context.Background()
	user1 := uuid.New()
	user2 := uuid.New()

	cat, err := svc.CreateCategory(ctx, service.CreateCategoryInput{
		UserID: user1,
		Name:   "Books",
		Type:   domain.CategoryTypeExpense,
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	// Delete with wrong user returns ErrNotFound
	err = svc.DeleteCategory(ctx, cat.ID, user2)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound for wrong user, got %v", err)
	}

	// Delete with owner succeeds
	err = svc.DeleteCategory(ctx, cat.ID, user1)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Second delete returns ErrNotFound
	err = svc.DeleteCategory(ctx, cat.ID, user1)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound on second delete, got %v", err)
	}
}
