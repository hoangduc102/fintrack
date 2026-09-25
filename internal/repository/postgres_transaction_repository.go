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
