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

func TestPostgresTransactionRepository_List_PaginationAndSorting(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	txRepo := repository.NewPostgresTransactionRepository(db)
	ctx := context.Background()

	user := createTestUser(t, userRepo)

	dates := []string{"2026-09-20", "2026-09-21", "2026-09-22", "2026-09-23", "2026-09-24"}
	for _, d := range dates {
		tx := &domain.Transaction{
			UserID:          user.ID,
			Amount:          10000,
			Type:            domain.TransactionTypeExpense,
			TransactionDate: d,
		}
		if err := txRepo.Create(ctx, tx); err != nil {
			t.Fatalf("failed to seed transaction: %v", err)
		}
	}

	// Page 1: limit 2, offset 0 -> expecting 2026-09-24, 2026-09-23
	res1, err := txRepo.List(ctx, repository.ListTransactionsFilter{
		UserID: user.ID,
		Limit:  2,
		Offset: 0,
	})
	if err != nil {
		t.Fatalf("failed to list page 1: %v", err)
	}
	if res1.TotalCount != 5 {
		t.Errorf("expected TotalCount 5, got %d", res1.TotalCount)
	}
	if len(res1.Transactions) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(res1.Transactions))
	}
	if res1.Transactions[0].TransactionDate != "2026-09-24" {
		t.Errorf("expected first item date 2026-09-24, got %s", res1.Transactions[0].TransactionDate)
	}
	if res1.Transactions[1].TransactionDate != "2026-09-23" {
		t.Errorf("expected second item date 2026-09-23, got %s", res1.Transactions[1].TransactionDate)
	}

	// Page 2: limit 2, offset 2 -> expecting 2026-09-22, 2026-09-21
	res2, err := txRepo.List(ctx, repository.ListTransactionsFilter{
		UserID: user.ID,
		Limit:  2,
		Offset: 2,
	})
	if err != nil {
		t.Fatalf("failed to list page 2: %v", err)
	}
	if res2.TotalCount != 5 {
		t.Errorf("expected TotalCount 5, got %d", res2.TotalCount)
	}
	if len(res2.Transactions) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(res2.Transactions))
	}
	if res2.Transactions[0].TransactionDate != "2026-09-22" {
		t.Errorf("expected item date 2026-09-22, got %s", res2.Transactions[0].TransactionDate)
	}
	if res2.Transactions[1].TransactionDate != "2026-09-21" {
		t.Errorf("expected item date 2026-09-21, got %s", res2.Transactions[1].TransactionDate)
	}

	// Page 3: limit 2, offset 4 -> expecting 2026-09-20
	res3, err := txRepo.List(ctx, repository.ListTransactionsFilter{
		UserID: user.ID,
		Limit:  2,
		Offset: 4,
	})
	if err != nil {
		t.Fatalf("failed to list page 3: %v", err)
	}
	if len(res3.Transactions) != 1 {
		t.Fatalf("expected 1 transaction on page 3, got %d", len(res3.Transactions))
	}
	if res3.Transactions[0].TransactionDate != "2026-09-20" {
		t.Errorf("expected item date 2026-09-20, got %s", res3.Transactions[0].TransactionDate)
	}
}

func TestPostgresTransactionRepository_List_Filters(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	catRepo := repository.NewPostgresCategoryRepository(db)
	txRepo := repository.NewPostgresTransactionRepository(db)
	ctx := context.Background()

	user1 := createTestUser(t, userRepo)
	user2 := createTestUser(t, userRepo)

	catExpense := &domain.Category{UserID: user1.ID, Name: "Dining", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(ctx, catExpense)

	catIncome := &domain.Category{UserID: user1.ID, Name: "Salary", Type: domain.CategoryTypeIncome}
	_ = catRepo.Create(ctx, catIncome)

	// User 1 transactions
	// tx1: Expense, with category, 2026-09-20
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user1.ID, Amount: 50000, Type: domain.TransactionTypeExpense, CategoryID: &catExpense.ID, TransactionDate: "2026-09-20"})
	// tx2: Expense, uncategorized, 2026-09-22
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user1.ID, Amount: 30000, Type: domain.TransactionTypeExpense, CategoryID: nil, TransactionDate: "2026-09-22"})
	// tx3: Income, with category, 2026-09-23
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user1.ID, Amount: 1000000, Type: domain.TransactionTypeIncome, CategoryID: &catIncome.ID, TransactionDate: "2026-09-23"})

	// User 2 transaction
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user2.ID, Amount: 99999, Type: domain.TransactionTypeExpense, TransactionDate: "2026-09-22"})

	// 1. Date Range Filter: 2026-09-21 to 2026-09-23 -> tx2 and tx3 (2 items)
	startDate := "2026-09-21"
	endDate := "2026-09-23"
	resDate, err := txRepo.List(ctx, repository.ListTransactionsFilter{
		UserID:    user1.ID,
		StartDate: &startDate,
		EndDate:   &endDate,
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("failed to list by date range: %v", err)
	}
	if resDate.TotalCount != 2 {
		t.Errorf("expected 2 items for date range, got %d", resDate.TotalCount)
	}

	// 2. Type Filter: INCOME -> tx3 (1 item)
	incType := domain.TransactionTypeIncome
	resType, err := txRepo.List(ctx, repository.ListTransactionsFilter{
		UserID: user1.ID,
		Type:   &incType,
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("failed to list by type: %v", err)
	}
	if resType.TotalCount != 1 || resType.Transactions[0].Type != domain.TransactionTypeIncome {
		t.Errorf("expected 1 income transaction, got %+v", resType)
	}

	// 3. Category Filter: catExpense -> tx1 (1 item)
	resCat, err := txRepo.List(ctx, repository.ListTransactionsFilter{
		UserID:     user1.ID,
		CategoryID: &catExpense.ID,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("failed to list by category: %v", err)
	}
	if resCat.TotalCount != 1 || resCat.Transactions[0].CategoryID == nil || *resCat.Transactions[0].CategoryID != catExpense.ID {
		t.Errorf("expected 1 transaction for catExpense, got %+v", resCat)
	}

	// 4. Uncategorized Filter: true -> tx2 (1 item)
	resUncat, err := txRepo.List(ctx, repository.ListTransactionsFilter{
		UserID:        user1.ID,
		Uncategorized: true,
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("failed to list uncategorized: %v", err)
	}
	if resUncat.TotalCount != 1 || resUncat.Transactions[0].CategoryID != nil {
		t.Errorf("expected 1 uncategorized transaction, got %+v", resUncat)
	}
}

