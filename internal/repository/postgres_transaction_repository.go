package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
)

// PostgresTransactionRepository implements TransactionRepository using raw SQL.
type PostgresTransactionRepository struct {
	db *sql.DB
}

// NewPostgresTransactionRepository creates a new PostgresTransactionRepository.
func NewPostgresTransactionRepository(db *sql.DB) *PostgresTransactionRepository {
	return &PostgresTransactionRepository{db: db}
}

// Create inserts a new transaction into PostgreSQL.
func (r *PostgresTransactionRepository) Create(ctx context.Context, tx *domain.Transaction) error {
	if tx.ID == uuid.Nil {
		tx.ID = uuid.New()
	}
	now := time.Now().UTC()
	tx.CreatedAt = now
	tx.UpdatedAt = now

	query := `
		INSERT INTO transactions (id, user_id, amount, type, category_id, transaction_date, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRowContext(ctx, query, tx.ID, tx.UserID, tx.Amount, tx.Type, tx.CategoryID, tx.TransactionDate, tx.Description, tx.CreatedAt, tx.UpdatedAt).
		Scan(&tx.ID, &tx.CreatedAt, &tx.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert transaction: %w", err)
	}
	return nil
}

// CreateBatch inserts multiple transactions inside a single database transaction using a prepared statement.
func (r *PostgresTransactionRepository) CreateBatch(ctx context.Context, txs []domain.Transaction) error {
	if len(txs) == 0 {
		return nil
	}

	dbtx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer dbtx.Rollback()

	query := `
		INSERT INTO transactions (id, user_id, amount, type, category_id, transaction_date, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	stmt, err := dbtx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare batch insert statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().UTC()
	for i := range txs {
		t := &txs[i]
		if t.ID == uuid.Nil {
			t.ID = uuid.New()
		}
		if t.CreatedAt.IsZero() {
			t.CreatedAt = now
		}
		if t.UpdatedAt.IsZero() {
			t.UpdatedAt = now
		}

		_, err := stmt.ExecContext(ctx, t.ID, t.UserID, t.Amount, t.Type, t.CategoryID, t.TransactionDate, t.Description, t.CreatedAt, t.UpdatedAt)
		if err != nil {
			return fmt.Errorf("failed to insert transaction at index %d: %w", i, err)
		}
	}

	if err := dbtx.Commit(); err != nil {
		return fmt.Errorf("failed to commit batch transaction: %w", err)
	}

	return nil
}

// FindByID retrieves a transaction by ID ensuring user ownership isolation.
func (r *PostgresTransactionRepository) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Transaction, error) {
	query := `
		SELECT id, user_id, amount, type, category_id, TO_CHAR(transaction_date, 'YYYY-MM-DD'), description, created_at, updated_at
		FROM transactions
		WHERE id = $1 AND user_id = $2
	`
	var tx domain.Transaction
	err := r.db.QueryRowContext(ctx, query, id, userID).
		Scan(&tx.ID, &tx.UserID, &tx.Amount, &tx.Type, &tx.CategoryID, &tx.TransactionDate, &tx.Description, &tx.CreatedAt, &tx.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: transaction not found", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("failed to find transaction by id: %w", err)
	}
	return &tx, nil
}

// List retrieves transactions for a user according to the provided filters, limit, and offset, along with total count.
func (r *PostgresTransactionRepository) List(ctx context.Context, filter ListTransactionsFilter) (*ListTransactionsResult, error) {
	whereClauses := []string{"user_id = $1"}
	args := []interface{}{filter.UserID}
	argIdx := 2

	if filter.StartDate != nil && *filter.StartDate != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("transaction_date >= $%d", argIdx))
		args = append(args, *filter.StartDate)
		argIdx++
	}

	if filter.EndDate != nil && *filter.EndDate != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("transaction_date <= $%d", argIdx))
		args = append(args, *filter.EndDate)
		argIdx++
	}

	if filter.Type != nil && *filter.Type != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, *filter.Type)
		argIdx++
	}

	if filter.Uncategorized {
		whereClauses = append(whereClauses, "category_id IS NULL")
	} else if filter.CategoryID != nil && *filter.CategoryID != uuid.Nil {
		whereClauses = append(whereClauses, fmt.Sprintf("category_id = $%d", argIdx))
		args = append(args, *filter.CategoryID)
		argIdx++
	}

	whereSQL := strings.Join(whereClauses, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM transactions WHERE %s", whereSQL)
	var totalCount int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, fmt.Errorf("failed to count transactions: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	itemsQuery := fmt.Sprintf(`
		SELECT id, user_id, amount, type, category_id, TO_CHAR(transaction_date, 'YYYY-MM-DD'), description, created_at, updated_at
		FROM transactions
		WHERE %s
		ORDER BY transaction_date DESC, created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, whereSQL, argIdx, argIdx+1)

	itemsArgs := append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, itemsQuery, itemsArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	transactions := make([]domain.Transaction, 0)
	for rows.Next() {
		var tx domain.Transaction
		if err := rows.Scan(&tx.ID, &tx.UserID, &tx.Amount, &tx.Type, &tx.CategoryID, &tx.TransactionDate, &tx.Description, &tx.CreatedAt, &tx.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan transaction: %w", err)
		}
		transactions = append(transactions, tx)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return &ListTransactionsResult{
		Transactions: transactions,
		TotalCount:   totalCount,
	}, nil
}

// Update updates an existing transaction while verifying user ownership.
func (r *PostgresTransactionRepository) Update(ctx context.Context, tx *domain.Transaction) error {
	now := time.Now().UTC()
	query := `
		UPDATE transactions
		SET amount = $1, type = $2, category_id = $3, transaction_date = $4, description = $5, updated_at = $6
		WHERE id = $7 AND user_id = $8
		RETURNING updated_at
	`
	err := r.db.QueryRowContext(ctx, query, tx.Amount, tx.Type, tx.CategoryID, tx.TransactionDate, tx.Description, now, tx.ID, tx.UserID).
		Scan(&tx.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: transaction not found", domain.ErrNotFound)
		}
		return fmt.Errorf("failed to update transaction: %w", err)
	}
	return nil
}

// Delete removes a transaction by ID while verifying user ownership.
func (r *PostgresTransactionRepository) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	query := `
		DELETE FROM transactions
		WHERE id = $1 AND user_id = $2
	`
	res, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete transaction: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("%w: transaction not found", domain.ErrNotFound)
	}
	return nil
}
