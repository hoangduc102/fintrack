package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
)

// PostgresAnalyticsRepository implements AnalyticsRepository using PostgreSQL.
type PostgresAnalyticsRepository struct {
	db *sql.DB
}

// NewPostgresAnalyticsRepository creates a new PostgresAnalyticsRepository.
func NewPostgresAnalyticsRepository(db *sql.DB) *PostgresAnalyticsRepository {
	return &PostgresAnalyticsRepository{db: db}
}

// GetMonthlySummary computes the high-level financial summary and budget status for the given month.
func (r *PostgresAnalyticsRepository) GetMonthlySummary(ctx context.Context, userID uuid.UUID, year int, month int) (*domain.MonthlySummary, error) {
	monthStr := fmt.Sprintf("%04d-%02d", year, month)
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	nextStart := start.AddDate(0, 1, 0)
	monthStart := start.Format("2006-01-02")
	nextMonthStart := nextStart.Format("2006-01-02")

	// 1. Compute total income and total expense in month
	txQuery := `
		SELECT 
			COALESCE(SUM(CASE WHEN type = 'INCOME' THEN amount ELSE 0 END), 0) AS total_income,
			COALESCE(SUM(CASE WHEN type = 'EXPENSE' THEN amount ELSE 0 END), 0) AS total_expense
		FROM transactions
		WHERE user_id = $1 AND transaction_date >= $2 AND transaction_date < $3
	`
	var totalIncome, totalExpense int64
	if err := r.db.QueryRowContext(ctx, txQuery, userID, monthStart, nextMonthStart).Scan(&totalIncome, &totalExpense); err != nil {
		return nil, fmt.Errorf("failed to query monthly transactions: %w", err)
	}

	netSavings := totalIncome - totalExpense

	// 2. Compute total budget and total spent on budgeted categories in month
	budgetQuery := `
		SELECT 
			COALESCE(SUM(b.amount), 0) AS total_budget,
			COALESCE(SUM(t.amount), 0) AS total_spent
		FROM budgets b
		LEFT JOIN transactions t ON t.category_id = b.category_id
			AND t.user_id = b.user_id
			AND t.type = 'EXPENSE'
			AND t.transaction_date >= $4
			AND t.transaction_date < $5
		WHERE b.user_id = $1 AND b.year = $2 AND b.month = $3
	`
	var totalBudget, totalSpent int64
	if err := r.db.QueryRowContext(ctx, budgetQuery, userID, year, month, monthStart, nextMonthStart).Scan(&totalBudget, &totalSpent); err != nil {
		return nil, fmt.Errorf("failed to query monthly budget status: %w", err)
	}

	remaining := totalBudget - totalSpent
	var percentage float64
	if totalBudget > 0 {
		percentage = math.Round((float64(totalSpent)/float64(totalBudget)*100)*100) / 100
	}

	return &domain.MonthlySummary{
		Month:        monthStr,
		TotalIncome:  totalIncome,
		TotalExpense: totalExpense,
		NetSavings:   netSavings,
		BudgetStatus: domain.BudgetStatusOverview{
			TotalBudget: totalBudget,
			TotalSpent:  totalSpent,
			Remaining:   remaining,
			Percentage:  percentage,
		},
	}, nil
}

// GetCategoryExpenses computes expense distribution across categories for the given month.
func (r *PostgresAnalyticsRepository) GetCategoryExpenses(ctx context.Context, userID uuid.UUID, year int, month int) ([]domain.CategoryExpenseShare, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	nextStart := start.AddDate(0, 1, 0)
	monthStart := start.Format("2006-01-02")
	nextMonthStart := nextStart.Format("2006-01-02")

	query := `
		SELECT 
			c.id AS category_id,
			COALESCE(c.name, 'Uncategorized') AS category_name,
			SUM(t.amount) AS amount
		FROM transactions t
		LEFT JOIN categories c ON c.id = t.category_id AND c.user_id = t.user_id
		WHERE t.user_id = $1 
		  AND t.type = 'EXPENSE'
		  AND t.transaction_date >= $2 
		  AND t.transaction_date < $3
		GROUP BY c.id, c.name
		ORDER BY amount DESC, category_name ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID, monthStart, nextMonthStart)
	if err != nil {
		return nil, fmt.Errorf("failed to query category expenses: %w", err)
	}
	defer rows.Close()

	type rawShare struct {
		categoryID   *uuid.UUID
		categoryName string
		amount       int64
	}

	var rawShares []rawShare
	var totalExpense int64

	for rows.Next() {
		var s rawShare
		if err := rows.Scan(&s.categoryID, &s.categoryName, &s.amount); err != nil {
			return nil, fmt.Errorf("failed to scan category expense row: %w", err)
		}
		rawShares = append(rawShares, s)
		totalExpense += s.amount
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	result := make([]domain.CategoryExpenseShare, 0, len(rawShares))
	for _, s := range rawShares {
		var pct float64
		if totalExpense > 0 {
			pct = math.Round((float64(s.amount)/float64(totalExpense)*100)*100) / 100
		}
		result = append(result, domain.CategoryExpenseShare{
			CategoryID:   s.categoryID,
			CategoryName: s.categoryName,
			Amount:       s.amount,
			Percentage:   pct,
		})
	}

	return result, nil
}

// GetMonthlyTrend computes monthly income and expense trend points for the past N months.
func (r *PostgresAnalyticsRepository) GetMonthlyTrend(ctx context.Context, userID uuid.UUID, months int) ([]domain.MonthlyTrendPoint, error) {
	if months <= 0 {
		months = 6
	}

	now := time.Now().UTC()
	currentYear, currentMonth, _ := now.Date()
	currentMonthStart := time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, time.UTC)
	startMonth := currentMonthStart.AddDate(0, -(months - 1), 0)
	nextMonthStart := currentMonthStart.AddDate(0, 1, 0)

	monthMap := make(map[string]*domain.MonthlyTrendPoint)
	orderedMonths := make([]string, 0, months)
	cur := startMonth
	for !cur.After(currentMonthStart) {
		mStr := cur.Format("2006-01")
		orderedMonths = append(orderedMonths, mStr)
		monthMap[mStr] = &domain.MonthlyTrendPoint{
			Month:      mStr,
			Income:     0,
			Expense:    0,
			NetSavings: 0,
		}
		cur = cur.AddDate(0, 1, 0)
	}

	query := `
		SELECT 
			TO_CHAR(transaction_date, 'YYYY-MM') AS month_str,
			COALESCE(SUM(CASE WHEN type = 'INCOME' THEN amount ELSE 0 END), 0) AS income,
			COALESCE(SUM(CASE WHEN type = 'EXPENSE' THEN amount ELSE 0 END), 0) AS expense
		FROM transactions
		WHERE user_id = $1 
		  AND transaction_date >= $2 
		  AND transaction_date < $3
		GROUP BY TO_CHAR(transaction_date, 'YYYY-MM')
		ORDER BY month_str ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID, startMonth.Format("2006-01-02"), nextMonthStart.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("failed to query monthly trend: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var monthStr string
		var income, expense int64
		if err := rows.Scan(&monthStr, &income, &expense); err != nil {
			return nil, fmt.Errorf("failed to scan trend row: %w", err)
		}
		if p, ok := monthMap[monthStr]; ok {
			p.Income = income
			p.Expense = expense
			p.NetSavings = income - expense
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	result := make([]domain.MonthlyTrendPoint, 0, len(orderedMonths))
	for _, m := range orderedMonths {
		result = append(result, *monthMap[m])
	}

	return result, nil
}
