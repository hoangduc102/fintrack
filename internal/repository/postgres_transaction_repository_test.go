package repository_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/repository"
)

func ptrString(s string) *string {
	return &s
}

func TestPostgresTransactionRepository_CreateAndFindByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	catRepo := repository.NewPostgresCategoryRepository(db)
	txRepo := repository.NewPostgresTransactionRepository(db)
	ctx := context.Background()

	user1 := createTestUser(t, userRepo)
	user2 := createTestUser(t, userRepo)

	category := &domain.Category{
		UserID: user1.ID,
		Name:   "Dining",
		Type:   domain.CategoryTypeExpense,
	}
	if err := catRepo.Create(ctx, category); err != nil {
		t.Fatalf("failed to create category: %v", err)
	}

	tx := &domain.Transaction{
		UserID:          user1.ID,
		Amount:          150000,
		Type:            domain.TransactionTypeExpense,
		CategoryID:      &category.ID,
		TransactionDate: "2026-09-25",
		Description:     ptrString("Lunch buffet"),
	}

	if err := txRepo.Create(ctx, tx); err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}

	if tx.ID == uuid.Nil {
		t.Fatal("expected transaction ID to be populated, got Nil")
	}

	// FindByID by owner
	found, err := txRepo.FindByID(ctx, tx.ID, user1.ID)
	if err != nil {
		t.Fatalf("failed to find transaction by id: %v", err)
	}
	if found.Amount != 150000 {
		t.Errorf("expected amount 150000, got %d", found.Amount)
	}
	if found.Type != domain.TransactionTypeExpense {
		t.Errorf("expected type EXPENSE, got %v", found.Type)
	}
	if found.CategoryID == nil || *found.CategoryID != category.ID {
		t.Errorf("expected categoryID %v, got %v", category.ID, found.CategoryID)
	}
	if found.TransactionDate != "2026-09-25" {
		t.Errorf("expected transaction_date 2026-09-25, got %s", found.TransactionDate)
	}
	if found.Description == nil || *found.Description != "Lunch buffet" {
		t.Errorf("expected description 'Lunch buffet', got %v", found.Description)
	}

	// FindByID by non-owner returns ErrNotFound
	_, err = txRepo.FindByID(ctx, tx.ID, user2.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound for non-owner, got %v", err)
	}
}

func TestPostgresTransactionRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	txRepo := repository.NewPostgresTransactionRepository(db)
	ctx := context.Background()

	user1 := createTestUser(t, userRepo)
	user2 := createTestUser(t, userRepo)

	tx := &domain.Transaction{
		UserID:          user1.ID,
		Amount:          200000,
		Type:            domain.TransactionTypeIncome,
		CategoryID:      nil,
		TransactionDate: "2026-09-20",
		Description:     ptrString("Side gig"),
	}
	if err := txRepo.Create(ctx, tx); err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}

	// Update fields
	tx.Amount = 250000
	tx.TransactionDate = "2026-09-21"
	tx.Description = ptrString("Bonus payment")
	if err := txRepo.Update(ctx, tx); err != nil {
		t.Fatalf("failed to update transaction: %v", err)
	}

	found, err := txRepo.FindByID(ctx, tx.ID, user1.ID)
	if err != nil {
		t.Fatalf("failed to find transaction: %v", err)
	}
	if found.Amount != 250000 {
		t.Errorf("expected updated amount 250000, got %d", found.Amount)
	}
	if found.TransactionDate != "2026-09-21" {
		t.Errorf("expected updated date 2026-09-21, got %s", found.TransactionDate)
	}

	// Update with wrong user returns ErrNotFound
	txWrongUser := *tx
	txWrongUser.UserID = user2.ID
	err = txRepo.Update(ctx, &txWrongUser)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound when updating with wrong user, got %v", err)
	}
}

func TestPostgresTransactionRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	txRepo := repository.NewPostgresTransactionRepository(db)
	ctx := context.Background()

	user1 := createTestUser(t, userRepo)
	user2 := createTestUser(t, userRepo)

	tx := &domain.Transaction{
		UserID:          user1.ID,
		Amount:          50000,
		Type:            domain.TransactionTypeExpense,
		CategoryID:      nil,
		TransactionDate: "2026-09-22",
	}
	if err := txRepo.Create(ctx, tx); err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}

	// Delete by non-owner returns ErrNotFound
	err := txRepo.Delete(ctx, tx.ID, user2.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound for non-owner delete, got %v", err)
	}

	// Delete by owner succeeds
	err = txRepo.Delete(ctx, tx.ID, user1.ID)
	if err != nil {
		t.Fatalf("failed to delete transaction: %v", err)
	}

	// Second delete returns ErrNotFound
	err = txRepo.Delete(ctx, tx.ID, user1.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected domain.ErrNotFound on second delete, got %v", err)
	}
}

func TestPostgresTransactionRepository_CategoryDeletionNullification(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	catRepo := repository.NewPostgresCategoryRepository(db)
	txRepo := repository.NewPostgresTransactionRepository(db)
	ctx := context.Background()

	user := createTestUser(t, userRepo)

	category := &domain.Category{
		UserID: user.ID,
		Name:   "Subscriptions",
		Type:   domain.CategoryTypeExpense,
	}
	if err := catRepo.Create(ctx, category); err != nil {
		t.Fatalf("failed to create category: %v", err)
	}

	tx := &domain.Transaction{
		UserID:          user.ID,
		Amount:          99000,
		Type:            domain.TransactionTypeExpense,
		CategoryID:      &category.ID,
		TransactionDate: "2026-09-23",
		Description:     ptrString("Streaming"),
	}
	if err := txRepo.Create(ctx, tx); err != nil {
		t.Fatalf("failed to create transaction: %v", err)
	}

	// Verify category_id is set
	found, err := txRepo.FindByID(ctx, tx.ID, user.ID)
	if err != nil || found.CategoryID == nil || *found.CategoryID != category.ID {
		t.Fatalf("expected category to be linked before delete")
	}

	// Delete category
	if err := catRepo.Delete(ctx, category.ID, user.ID); err != nil {
		t.Fatalf("failed to delete category: %v", err)
	}

	// Verify transaction still exists and category_id is now NULL
	foundAfterCatDelete, err := txRepo.FindByID(ctx, tx.ID, user.ID)
	if err != nil {
		t.Fatalf("expected transaction to persist after category deletion: %v", err)
	}
	if foundAfterCatDelete.CategoryID != nil {
		t.Errorf("expected category_id to be NULL (Uncategorized) after category deletion, got %v", foundAfterCatDelete.CategoryID)
	}
}
