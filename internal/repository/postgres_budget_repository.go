package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
)

// PostgresBudgetRepository implements BudgetRepository using PostgreSQL.
type PostgresBudgetRepository struct {
	db *sql.DB
}

// NewPostgresBudgetRepository creates a new PostgresBudgetRepository.
func NewPostgresBudgetRepository(db *sql.DB) *PostgresBudgetRepository {
	return &PostgresBudgetRepository{db: db}
}

// Upsert creates or updates a budget for (user_id, category_id, year, month).
func (r *PostgresBudgetRepository) Upsert(ctx context.Context, budget *domain.Budget) error {
	if budget.ID == uuid.Nil {
		budget.ID = uuid.New()
	}
	now := time.Now().UTC()
	budget.CreatedAt = now
	budget.UpdatedAt = now

	query := `
		INSERT INTO budgets (id, user_id, category_id, amount, year, month, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, category_id, year, month)
		DO UPDATE SET
			amount = EXCLUDED.amount,
			updated_at = EXCLUDED.updated_at
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRowContext(
		ctx,
		query,
		budget.ID,
		budget.UserID,
		budget.CategoryID,
		budget.Amount,
		budget.Year,
		budget.Month,
		budget.CreatedAt,
		budget.UpdatedAt,
	).Scan(&budget.ID, &budget.CreatedAt, &budget.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to upsert budget: %w", err)
	}
	return nil
}

// FindByID retrieves a budget by ID and user ID.
func (r *PostgresBudgetRepository) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Budget, error) {
	query := `
		SELECT 
			b.id,
			b.user_id,
			b.category_id,
			c.name AS category_name,
			b.amount,
			b.year,
			b.month,
			b.created_at,
			b.updated_at
		FROM budgets b
		JOIN categories c ON c.id = b.category_id
		WHERE b.id = $1 AND b.user_id = $2
	`
	var b domain.Budget
	err := r.db.QueryRowContext(ctx, query, id, userID).Scan(
		&b.ID,
		&b.UserID,
		&b.CategoryID,
		&b.CategoryName,
		&b.Amount,
		&b.Year,
		&b.Month,
		&b.CreatedAt,
		&b.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: budget not found", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("failed to find budget by id: %w", err)
	}
	return &b, nil
}

// FindByCategoryAndMonth retrieves a budget by category ID, year, and month.
func (r *PostgresBudgetRepository) FindByCategoryAndMonth(ctx context.Context, userID uuid.UUID, categoryID uuid.UUID, year int, month int) (*domain.Budget, error) {
	query := `
		SELECT 
			b.id,
			b.user_id,
			b.category_id,
			c.name AS category_name,
			b.amount,
			b.year,
			b.month,
			b.created_at,
			b.updated_at
		FROM budgets b
		JOIN categories c ON c.id = b.category_id
		WHERE b.user_id = $1 AND b.category_id = $2 AND b.year = $3 AND b.month = $4
	`
	var b domain.Budget
	err := r.db.QueryRowContext(ctx, query, userID, categoryID, year, month).Scan(
		&b.ID,
		&b.UserID,
		&b.CategoryID,
		&b.CategoryName,
		&b.Amount,
		&b.Year,
		&b.Month,
		&b.CreatedAt,
		&b.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: budget not found", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("failed to find budget by category and month: %w", err)
	}
	return &b, nil
}

// ListByMonth retrieves all budgets for a user in the given year and month with dynamically computed spent amounts.
func (r *PostgresBudgetRepository) ListByMonth(ctx context.Context, userID uuid.UUID, year int, month int) ([]domain.Budget, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	nextStart := start.AddDate(0, 1, 0)
	monthStart := start.Format("2006-01-02")
	nextMonthStart := nextStart.Format("2006-01-02")

	query := `
		SELECT 
			b.id,
			b.user_id,
			b.category_id,
			c.name AS category_name,
			b.amount,
			b.year,
			b.month,
			COALESCE(SUM(t.amount), 0) AS spent,
			b.created_at,
			b.updated_at
		FROM budgets b
		JOIN categories c ON c.id = b.category_id
		LEFT JOIN transactions t ON t.category_id = b.category_id
			AND t.user_id = b.user_id
			AND t.type = 'EXPENSE'
			AND t.transaction_date >= $4
			AND t.transaction_date < $5
		WHERE b.user_id = $1 AND b.year = $2 AND b.month = $3
		GROUP BY b.id, b.user_id, b.category_id, c.name, b.amount, b.year, b.month, b.created_at, b.updated_at
		ORDER BY c.name ASC, b.id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, userID, year, month, monthStart, nextMonthStart)
	if err != nil {
		return nil, fmt.Errorf("failed to list budgets: %w", err)
	}
	defer rows.Close()

	budgets := make([]domain.Budget, 0)
	for rows.Next() {
		var b domain.Budget
		if err := rows.Scan(
			&b.ID,
			&b.UserID,
			&b.CategoryID,
			&b.CategoryName,
			&b.Amount,
			&b.Year,
			&b.Month,
			&b.Spent,
			&b.CreatedAt,
			&b.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan budget: %w", err)
		}
		budgets = append(budgets, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return budgets, nil
}

// Delete removes a budget by ID and user ID.
func (r *PostgresBudgetRepository) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	query := `
		DELETE FROM budgets
		WHERE id = $1 AND user_id = $2
	`
	res, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete budget: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("%w: budget not found", domain.ErrNotFound)
	}
	return nil
}

// GetSpent dynamically calculates total expense spending for a category in the given month.
func (r *PostgresBudgetRepository) GetSpent(ctx context.Context, userID uuid.UUID, categoryID uuid.UUID, year int, month int) (int64, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	nextStart := start.AddDate(0, 1, 0)
	monthStart := start.Format("2006-01-02")
	nextMonthStart := nextStart.Format("2006-01-02")

	query := `
		SELECT COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE user_id = $1
		  AND category_id = $2
		  AND type = 'EXPENSE'
		  AND transaction_date >= $3
		  AND transaction_date < $4
	`
	var spent int64
	err := r.db.QueryRowContext(ctx, query, userID, categoryID, monthStart, nextMonthStart).Scan(&spent)
	if err != nil {
		return 0, fmt.Errorf("failed to get spent: %w", err)
	}
	return spent, nil
}
