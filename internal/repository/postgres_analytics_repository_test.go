package repository_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/repository"
)

func TestPostgresAnalyticsRepository_GetMonthlySummary(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	catRepo := repository.NewPostgresCategoryRepository(db)
	txRepo := repository.NewPostgresTransactionRepository(db)
	budgetRepo := repository.NewPostgresBudgetRepository(db)
	analyticsRepo := repository.NewPostgresAnalyticsRepository(db)
	ctx := context.Background()

	user := createTestUser(t, userRepo)

	catFood := &domain.Category{UserID: user.ID, Name: "Food", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(ctx, catFood)

	catSalary := &domain.Category{UserID: user.ID, Name: "Salary", Type: domain.CategoryTypeIncome}
	_ = catRepo.Create(ctx, catSalary)

	// 1. Transactions in September 2026
	// Income: 25,000,000
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user.ID, Amount: 25000000, Type: domain.TransactionTypeIncome, CategoryID: &catSalary.ID, TransactionDate: "2026-09-01"})
	// Expense: 4,000,000 on Food
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user.ID, Amount: 4000000, Type: domain.TransactionTypeExpense, CategoryID: &catFood.ID, TransactionDate: "2026-09-10"})
	// Expense: 1,000,000 Uncategorized
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user.ID, Amount: 1000000, Type: domain.TransactionTypeExpense, CategoryID: nil, TransactionDate: "2026-09-15"})

	// 2. Transaction outside September (should be excluded)
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user.ID, Amount: 8888888, Type: domain.TransactionTypeIncome, CategoryID: &catSalary.ID, TransactionDate: "2026-10-01"})

	// 3. Budget in September: 8,000,000 on Food
	_ = budgetRepo.Upsert(ctx, &domain.Budget{UserID: user.ID, CategoryID: catFood.ID, Amount: 8000000, Year: 2026, Month: 9})

	summary, err := analyticsRepo.GetMonthlySummary(ctx, user.ID, 2026, 9)
	if err != nil {
		t.Fatalf("failed to get monthly summary: %v", err)
	}

	if summary.Month != "2026-09" {
		t.Errorf("expected month 2026-09, got %s", summary.Month)
	}
	if summary.TotalIncome != 25000000 {
		t.Errorf("expected total_income 25000000, got %d", summary.TotalIncome)
	}
	if summary.TotalExpense != 5000000 {
		t.Errorf("expected total_expense 5000000, got %d", summary.TotalExpense)
	}
	if summary.NetSavings != 20000000 {
		t.Errorf("expected net_savings 20000000, got %d", summary.NetSavings)
	}
	if summary.BudgetStatus.TotalBudget != 8000000 {
		t.Errorf("expected total_budget 8000000, got %d", summary.BudgetStatus.TotalBudget)
	}
	if summary.BudgetStatus.TotalSpent != 4000000 {
		t.Errorf("expected total_spent 4000000, got %d", summary.BudgetStatus.TotalSpent)
	}
	if summary.BudgetStatus.Remaining != 4000000 {
		t.Errorf("expected remaining 4000000, got %d", summary.BudgetStatus.Remaining)
	}
	if summary.BudgetStatus.Percentage != 50.0 {
		t.Errorf("expected percentage 50.0, got %f", summary.BudgetStatus.Percentage)
	}
}

func TestPostgresAnalyticsRepository_GetCategoryExpenses(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	catRepo := repository.NewPostgresCategoryRepository(db)
	txRepo := repository.NewPostgresTransactionRepository(db)
	analyticsRepo := repository.NewPostgresAnalyticsRepository(db)
	ctx := context.Background()

	user := createTestUser(t, userRepo)

	catFood := &domain.Category{UserID: user.ID, Name: "Food", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(ctx, catFood)

	catTransport := &domain.Category{UserID: user.ID, Name: "Transport", Type: domain.CategoryTypeExpense}
	_ = catRepo.Create(ctx, catTransport)

	// Food: 6,000,000 (60%)
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user.ID, Amount: 6000000, Type: domain.TransactionTypeExpense, CategoryID: &catFood.ID, TransactionDate: "2026-09-05"})
	// Transport: 2,000,000 (20%)
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user.ID, Amount: 2000000, Type: domain.TransactionTypeExpense, CategoryID: &catTransport.ID, TransactionDate: "2026-09-12"})
	// Uncategorized: 2,000,000 (20%)
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user.ID, Amount: 2000000, Type: domain.TransactionTypeExpense, CategoryID: nil, TransactionDate: "2026-09-20"})

	shares, err := analyticsRepo.GetCategoryExpenses(ctx, user.ID, 2026, 9)
	if err != nil {
		t.Fatalf("failed to get category expenses: %v", err)
	}

	if len(shares) != 3 {
		t.Fatalf("expected 3 category shares, got %d", len(shares))
	}

	// First should be Food with 60%
	if shares[0].CategoryName != "Food" || shares[0].Amount != 6000000 || shares[0].Percentage != 60.0 {
		t.Errorf("unexpected top share: %+v", shares[0])
	}
}

func TestPostgresAnalyticsRepository_GetMonthlyTrend(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	txRepo := repository.NewPostgresTransactionRepository(db)
	analyticsRepo := repository.NewPostgresAnalyticsRepository(db)
	ctx := context.Background()

	user := createTestUser(t, userRepo)

	now := time.Now().UTC()
	currentMonthStr := now.Format("2006-01")
	prevMonth := now.AddDate(0, -1, 0)
	prevMonthStr := prevMonth.Format("2006-01")

	// Seed current month
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user.ID, Amount: 10000000, Type: domain.TransactionTypeIncome, TransactionDate: fmt.Sprintf("%s-05", currentMonthStr)})
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user.ID, Amount: 3000000, Type: domain.TransactionTypeExpense, TransactionDate: fmt.Sprintf("%s-10", currentMonthStr)})

	// Seed previous month
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user.ID, Amount: 8000000, Type: domain.TransactionTypeIncome, TransactionDate: fmt.Sprintf("%s-05", prevMonthStr)})
	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user.ID, Amount: 4000000, Type: domain.TransactionTypeExpense, TransactionDate: fmt.Sprintf("%s-10", prevMonthStr)})

	trend, err := analyticsRepo.GetMonthlyTrend(ctx, user.ID, 6)
	if err != nil {
		t.Fatalf("failed to get monthly trend: %v", err)
	}

	if len(trend) != 6 {
		t.Fatalf("expected 6 trend points, got %d", len(trend))
	}

	// Last item should be current month
	last := trend[5]
	if last.Month != currentMonthStr {
		t.Errorf("expected last month %s, got %s", currentMonthStr, last.Month)
	}
	if last.Income != 10000000 || last.Expense != 3000000 || last.NetSavings != 7000000 {
		t.Errorf("unexpected current month figures: %+v", last)
	}

	// Second to last item should be previous month
	prev := trend[4]
	if prev.Month != prevMonthStr {
		t.Errorf("expected prev month %s, got %s", prevMonthStr, prev.Month)
	}
	if prev.Income != 8000000 || prev.Expense != 4000000 || prev.NetSavings != 4000000 {
		t.Errorf("unexpected prev month figures: %+v", prev)
	}
}

func TestPostgresAnalyticsRepository_Isolation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	userRepo := repository.NewPostgresUserRepository(db)
	txRepo := repository.NewPostgresTransactionRepository(db)
	analyticsRepo := repository.NewPostgresAnalyticsRepository(db)
	ctx := context.Background()

	user1 := createTestUser(t, userRepo)
	user2 := createTestUser(t, userRepo)

	_ = txRepo.Create(ctx, &domain.Transaction{UserID: user1.ID, Amount: 50000000, Type: domain.TransactionTypeIncome, TransactionDate: "2026-09-01"})

	summary2, err := analyticsRepo.GetMonthlySummary(ctx, user2.ID, 2026, 9)
	if err != nil {
		t.Fatalf("failed to get summary for user2: %v", err)
	}
	if summary2.TotalIncome != 0 {
		t.Errorf("expected 0 total income for user2, got %d", summary2.TotalIncome)
	}
}
