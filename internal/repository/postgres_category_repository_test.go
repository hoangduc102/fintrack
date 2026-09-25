package repository_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/repository"
)

func createTestUser(t *testing.T, userRepo repository.UserRepository) *domain.User {
	t.Helper()
	user := &domain.User{
		Email:        fmt.Sprintf("cat-user-%s@example.com", uuid.New().String()[:8]),
		PasswordHash: "$2a$10$hashedpasswordplaceholder",
	}
	err := userRepo.CreateWithCategories(context.Background(), user, nil)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return user
}

func TestPostgresCategoryRepository_CreateAndFindByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	catRepo := repository.NewPostgresCategoryRepository(db)
	ctx := context.Background()

	user1 := createTestUser(t, userRepo)
	user2 := createTestUser(t, userRepo)

	category := &domain.Category{
		UserID: user1.ID,
		Name:   "Coffee",
		Type:   domain.CategoryTypeExpense,
	}

	err := catRepo.Create(ctx, category)
	if err != nil {
		t.Fatalf("failed to create category: %v", err)
	}

	if category.ID == uuid.Nil {
		t.Fatal("expected category ID to be populated, got Nil")
	}

	// FindByID with owner succeeds
	found, err := catRepo.FindByID(ctx, category.ID, user1.ID)
	if err != nil {
		t.Fatalf("failed to find category by id: %v", err)
	}
	if found.Name != "Coffee" {
		t.Errorf("expected name 'Coffee', got %q", found.Name)
	}
	if found.Type != domain.CategoryTypeExpense {
		t.Errorf("expected type EXPENSE, got %v", found.Type)
	}

	// FindByID with another user returns ErrNotFound
	_, err = catRepo.FindByID(ctx, category.ID, user2.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound when querying with non-owner, got %v", err)
	}
}

func TestPostgresCategoryRepository_UniquenessPerUserAndType(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	catRepo := repository.NewPostgresCategoryRepository(db)
	ctx := context.Background()

	user1 := createTestUser(t, userRepo)
	user2 := createTestUser(t, userRepo)

	cat1 := &domain.Category{
		UserID: user1.ID,
		Name:   "Investment",
		Type:   domain.CategoryTypeIncome,
	}
	if err := catRepo.Create(ctx, cat1); err != nil {
		t.Fatalf("failed to create cat1: %v", err)
	}

	// Duplicate case-insensitive name for same user and type -> ErrConflict
	cat2 := &domain.Category{
		UserID: user1.ID,
		Name:   "investment",
		Type:   domain.CategoryTypeIncome,
	}
	err := catRepo.Create(ctx, cat2)
	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("expected domain.ErrConflict for case-insensitive duplicate, got %v", err)
	}

	// Same name but different type for same user -> OK
	cat3 := &domain.Category{
		UserID: user1.ID,
		Name:   "Investment",
		Type:   domain.CategoryTypeExpense,
	}
	if err := catRepo.Create(ctx, cat3); err != nil {
		t.Errorf("expected create to succeed for different type, got %v", err)
	}

	// Same name and type but different user -> OK
	cat4 := &domain.Category{
		UserID: user2.ID,
		Name:   "Investment",
		Type:   domain.CategoryTypeIncome,
	}
	if err := catRepo.Create(ctx, cat4); err != nil {
		t.Errorf("expected create to succeed for different user, got %v", err)
	}
}

func TestPostgresCategoryRepository_FindByUserAndType(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	catRepo := repository.NewPostgresCategoryRepository(db)
	ctx := context.Background()

	user1 := createTestUser(t, userRepo)
	user2 := createTestUser(t, userRepo)

	_ = catRepo.Create(ctx, &domain.Category{UserID: user1.ID, Name: "A Expense", Type: domain.CategoryTypeExpense})
	_ = catRepo.Create(ctx, &domain.Category{UserID: user1.ID, Name: "B Income", Type: domain.CategoryTypeIncome})
	_ = catRepo.Create(ctx, &domain.Category{UserID: user2.ID, Name: "User2 Cat", Type: domain.CategoryTypeExpense})

	// List all for user1
	all, err := catRepo.FindByUserAndType(ctx, user1.ID, nil)
	if err != nil {
		t.Fatalf("failed to list categories: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 categories for user1, got %d", len(all))
	}

	// List only EXPENSE for user1
	expenseType := domain.CategoryTypeExpense
	expenses, err := catRepo.FindByUserAndType(ctx, user1.ID, &expenseType)
	if err != nil {
		t.Fatalf("failed to list expense categories: %v", err)
	}
	if len(expenses) != 1 || expenses[0].Name != "A Expense" {
		t.Errorf("expected 1 expense category 'A Expense', got %+v", expenses)
	}

	// List only INCOME for user1
	incomeType := domain.CategoryTypeIncome
	incomes, err := catRepo.FindByUserAndType(ctx, user1.ID, &incomeType)
	if err != nil {
		t.Fatalf("failed to list income categories: %v", err)
	}
	if len(incomes) != 1 || incomes[0].Name != "B Income" {
		t.Errorf("expected 1 income category 'B Income', got %+v", incomes)
	}
}

func TestPostgresCategoryRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	catRepo := repository.NewPostgresCategoryRepository(db)
	ctx := context.Background()

	user1 := createTestUser(t, userRepo)
	user2 := createTestUser(t, userRepo)

	cat := &domain.Category{
		UserID: user1.ID,
		Name:   "Groceries",
		Type:   domain.CategoryTypeExpense,
	}
	if err := catRepo.Create(ctx, cat); err != nil {
		t.Fatalf("failed to create category: %v", err)
	}

	// Update name succeeds
	cat.Name = "Supermarket"
	if err := catRepo.Update(ctx, cat); err != nil {
		t.Fatalf("failed to update category: %v", err)
	}

	found, err := catRepo.FindByID(ctx, cat.ID, user1.ID)
	if err != nil {
		t.Fatalf("failed to find category: %v", err)
	}
	if found.Name != "Supermarket" {
		t.Errorf("expected name 'Supermarket', got %q", found.Name)
	}

	// Update with wrong user returns ErrNotFound
	catWrongUser := &domain.Category{
		ID:     cat.ID,
		UserID: user2.ID,
		Name:   "Hacked",
		Type:   cat.Type,
	}
	err = catRepo.Update(ctx, catWrongUser)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound when updating category of another user, got %v", err)
	}
}

func TestPostgresCategoryRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	catRepo := repository.NewPostgresCategoryRepository(db)
	ctx := context.Background()

	user1 := createTestUser(t, userRepo)
	user2 := createTestUser(t, userRepo)

	cat := &domain.Category{
		UserID: user1.ID,
		Name:   "Utilities",
		Type:   domain.CategoryTypeExpense,
	}
	if err := catRepo.Create(ctx, cat); err != nil {
		t.Fatalf("failed to create category: %v", err)
	}

	// Delete by non-owner returns ErrNotFound
	err := catRepo.Delete(ctx, cat.ID, user2.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound when deleting category of another user, got %v", err)
	}

	// Delete by owner succeeds
	err = catRepo.Delete(ctx, cat.ID, user1.ID)
	if err != nil {
		t.Fatalf("failed to delete category: %v", err)
	}

	// Second delete returns ErrNotFound
	err = catRepo.Delete(ctx, cat.ID, user1.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound on second delete, got %v", err)
	}
}
