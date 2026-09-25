package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/repository"
)

func TestPostgresBudgetRepository_UpsertAndFind(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	catRepo := repository.NewPostgresCategoryRepository(db)
	budgetRepo := repository.NewPostgresBudgetRepository(db)
	ctx := context.Background()

	user := createTestUser(t, userRepo)
	cat := &domain.Category{
		UserID: user.ID,
		Name:   "Groceries",
		Type:   domain.CategoryTypeExpense,
	}
	if err := catRepo.Create(ctx, cat); err != nil {
		t.Fatalf("failed to create category: %v", err)
	}

	// 1. Initial Insert via Upsert
	b := &domain.Budget{
		UserID:     user.ID,
		CategoryID: cat.ID,
		Amount:     5000000,
		Year:       2026,
		Month:      9,
	}
	if err := budgetRepo.Upsert(ctx, b); err != nil {
		t.Fatalf("failed to insert budget: %v", err)
	}
	if b.ID == uuid.Nil {
		t.Fatal("expected non-nil ID after upsert")
	}
	initialID := b.ID

	// 2. FindByID
	found, err := budgetRepo.FindByID(ctx, b.ID, user.ID)
	if err != nil {
		t.Fatalf("failed to find budget by id: %v", err)
	}
	if found.Amount != 5000000 {
		t.Errorf("expected amount 5000000, got %d", found.Amount)
	}
	if found.Year != 2026 || found.Month != 9 {
		t.Errorf("expected 2026-09, got %d-%02d", found.Year, found.Month)
	}
	if found.CategoryName != "Groceries" {
		t.Errorf("expected CategoryName Groceries, got %s", found.CategoryName)
	}

	// 3. Update limit via Upsert (same user, category, year, month)
	updateBudget := &domain.Budget{
		UserID:     user.ID,
		CategoryID: cat.ID,
		Amount:     7500000,
		Year:       2026,
		Month:      9,
	}
	if err := budgetRepo.Upsert(ctx, updateBudget); err != nil {
		t.Fatalf("failed to update budget via upsert: %v", err)
	}
	if updateBudget.ID != initialID {
		t.Errorf("expected same ID %s, got %s", initialID, updateBudget.ID)
	}

	// 4. Verify updated amount
	updatedFound, err := budgetRepo.FindByID(ctx, initialID, user.ID)
	if err != nil {
		t.Fatalf("failed to find updated budget: %v", err)
	}
	if updatedFound.Amount != 7500000 {
		t.Errorf("expected updated amount 7500000, got %d", updatedFound.Amount)
	}
}

func TestPostgresBudgetRepository_ListByMonth_WithSpentCalculation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	catRepo := repository.NewPostgresCategoryRepository(db)
	txRepo := repository.NewPostgresTransactionRepository(db)
	budgetRepo := repository.NewPostgresBudgetRepository(db)
	ctx := context.Background()

	user := createTestUser(t, userRepo)

	catDining := &domain.Category{UserID: user.ID, Name: "Dining", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(ctx, catDining)

	catCinema := &domain.Category{UserID: user.ID, Name: "Cinema", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(ctx, catCinema)

	// Set budgets for 2026-09
	bDining := &domain.Budget{UserID: user.ID, CategoryID: catDining.ID, Amount: 3000000, Year: 2026, Month: 9}
	_ = budgetRepo.Upsert(ctx, bDining)

	bCinema := &domain.Budget{UserID: user.ID, CategoryID: catCinema.ID, Amount: 1000000, Year: 2026, Month: 9}
	_ = budgetRepo.Upsert(ctx, bCinema)

	// Seed transactions
	// tx1: Dining in September -> +500,000
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user.ID, Amount: 500000, Type: domain.TransactionTypeExpense, CategoryID: &catDining.ID, TransactionDate: "2026-09-05"})
	// tx2: Dining in September -> +200,000
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user.ID, Amount: 200000, Type: domain.TransactionTypeExpense, CategoryID: &catDining.ID, TransactionDate: "2026-09-30"})
	// tx3: Dining in October -> NOT counted (>= next_month_start)
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user.ID, Amount: 150000, Type: domain.TransactionTypeExpense, CategoryID: &catDining.ID, TransactionDate: "2026-10-01"})
	// tx4: Dining in August -> NOT counted (< month_start)
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user.ID, Amount: 300000, Type: domain.TransactionTypeExpense, CategoryID: &catDining.ID, TransactionDate: "2026-08-31"})
	// tx5: Dining in September INCOME -> NOT counted (only EXPENSE counted)
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user.ID, Amount: 999999, Type: domain.TransactionTypeIncome, CategoryID: &catDining.ID, TransactionDate: "2026-09-15"})
	// tx6: Cinema in September -> +400,000
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user.ID, Amount: 400000, Type: domain.TransactionTypeExpense, CategoryID: &catCinema.ID, TransactionDate: "2026-09-12"})

	// List budgets for 2026-09
	budgets, err := budgetRepo.ListByMonth(ctx, user.ID, 2026, 9)
	if err != nil {
		t.Fatalf("failed to list budgets: %v", err)
	}

	if len(budgets) != 2 {
		t.Fatalf("expected 2 budgets, got %d", len(budgets))
	}

	budgetMap := make(map[uuid.UUID]domain.Budget)
	for _, b := range budgets {
		budgetMap[b.CategoryID] = b
	}

	diningBudget, ok := budgetMap[catDining.ID]
	if !ok {
		t.Fatal("dining budget not found in list")
	}
	if diningBudget.Spent != 700000 {
		t.Errorf("expected dining spent 700000 (500000+200000), got %d", diningBudget.Spent)
	}
	if diningBudget.Amount != 3000000 {
		t.Errorf("expected dining amount 3000000, got %d", diningBudget.Amount)
	}

	cinemaBudget, ok := budgetMap[catCinema.ID]
	if !ok {
		t.Fatal("cinema budget not found in list")
	}
	if cinemaBudget.Spent != 400000 {
		t.Errorf("expected cinema spent 400000, got %d", cinemaBudget.Spent)
	}
}

func TestPostgresBudgetRepository_CascadeOnCategoryDeletion(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	catRepo := repository.NewPostgresCategoryRepository(db)
	budgetRepo := repository.NewPostgresBudgetRepository(db)
	ctx := context.Background()

	user := createTestUser(t, userRepo)
	cat := &domain.Category{UserID: user.ID, Name: "Gym", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(ctx, cat)

	b := &domain.Budget{UserID: user.ID, CategoryID: cat.ID, Amount: 1000000, Year: 2026, Month: 9}
	if err := budgetRepo.Upsert(ctx, b); err != nil {
		t.Fatalf("failed to create budget: %v", err)
	}

	// Delete Category
	if err := catRepo.Delete(ctx, cat.ID, user.ID); err != nil {
		t.Fatalf("failed to delete category: %v", err)
	}

	// Verify budget is cascaded and deleted
	_, err := budgetRepo.FindByID(ctx, b.ID, user.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound after category deletion (cascade), got %v", err)
	}
}

func TestPostgresBudgetRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	catRepo := repository.NewPostgresCategoryRepository(db)
	budgetRepo := repository.NewPostgresBudgetRepository(db)
	ctx := context.Background()

	user := createTestUser(t, userRepo)
	cat := &domain.Category{UserID: user.ID, Name: "Books", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(ctx, cat)

	b := &domain.Budget{UserID: user.ID, CategoryID: cat.ID, Amount: 500000, Year: 2026, Month: 9}
	_ = budgetRepo.Upsert(ctx, b)

	if err := budgetRepo.Delete(ctx, b.ID, user.ID); err != nil {
		t.Fatalf("failed to delete budget: %v", err)
	}

	// Second delete should return ErrNotFound
	if err := budgetRepo.Delete(ctx, b.ID, user.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound on second delete, got %v", err)
	}
}

func TestPostgresBudgetRepository_Isolation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	catRepo := repository.NewPostgresCategoryRepository(db)
	budgetRepo := repository.NewPostgresBudgetRepository(db)
	ctx := context.Background()

	user1 := createTestUser(t, userRepo)
	user2 := createTestUser(t, userRepo)

	cat := &domain.Category{UserID: user1.ID, Name: "Gas", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(ctx, cat)

	b := &domain.Budget{UserID: user1.ID, CategoryID: cat.ID, Amount: 800000, Year: 2026, Month: 9}
	_ = budgetRepo.Upsert(ctx, b)

	// User 2 cannot FindByID user 1's budget
	_, err := budgetRepo.FindByID(ctx, b.ID, user2.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound when user2 queries user1's budget, got %v", err)
	}

	// User 2 lists budgets for 2026-09 -> empty
	user2Budgets, err := budgetRepo.ListByMonth(ctx, user2.ID, 2026, 9)
	if err != nil {
		t.Fatalf("failed to list budgets for user 2: %v", err)
	}
	if len(user2Budgets) != 0 {
		t.Errorf("expected 0 budgets for user 2, got %d", len(user2Budgets))
	}
}
