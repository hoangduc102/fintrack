package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/hoangduc102/fintrack/internal/domain"
)

// PostgresCategoryRepository implements CategoryRepository using raw SQL queries with database/sql.
type PostgresCategoryRepository struct {
	db *sql.DB
}

// NewPostgresCategoryRepository creates a new PostgresCategoryRepository.
func NewPostgresCategoryRepository(db *sql.DB) *PostgresCategoryRepository {
	return &PostgresCategoryRepository{db: db}
}

// Create inserts a new category into PostgreSQL and returns domain.ErrConflict on unique constraint violation.
func (r *PostgresCategoryRepository) Create(ctx context.Context, category *domain.Category) error {
	if category.ID == uuid.Nil {
		category.ID = uuid.New()
	}
	now := time.Now().UTC()
	category.CreatedAt = now
	category.UpdatedAt = now

	query := `
		INSERT INTO categories (id, user_id, name, type, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRowContext(ctx, query, category.ID, category.UserID, category.Name, category.Type, category.CreatedAt, category.UpdatedAt).
		Scan(&category.ID, &category.CreatedAt, &category.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%w: category %q with type %s already exists", domain.ErrConflict, category.Name, category.Type)
		}
		return fmt.Errorf("failed to insert category: %w", err)
	}
	return nil
}

// FindByID retrieves a category by its ID ensuring user ownership isolation.
func (r *PostgresCategoryRepository) FindByID(ctx context.Context, id uuid.UUID, userID uuid.UUID) (*domain.Category, error) {
	query := `
		SELECT id, user_id, name, type, created_at, updated_at
		FROM categories
		WHERE id = $1 AND user_id = $2
	`
	var cat domain.Category
	err := r.db.QueryRowContext(ctx, query, id, userID).
		Scan(&cat.ID, &cat.UserID, &cat.Name, &cat.Type, &cat.CreatedAt, &cat.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: category not found", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("failed to find category by id: %w", err)
	}
	return &cat, nil
}

// FindByUserAndType retrieves all categories belonging to the user, optionally filtered by category type.
func (r *PostgresCategoryRepository) FindByUserAndType(ctx context.Context, userID uuid.UUID, categoryType *domain.CategoryType) ([]domain.Category, error) {
	var query string
	var rows *sql.Rows
	var err error

	if categoryType != nil {
		query = `
			SELECT id, user_id, name, type, created_at, updated_at
			FROM categories
			WHERE user_id = $1 AND type = $2
			ORDER BY type ASC, name ASC, id ASC
		`
		rows, err = r.db.QueryContext(ctx, query, userID, *categoryType)
	} else {
		query = `
			SELECT id, user_id, name, type, created_at, updated_at
			FROM categories
			WHERE user_id = $1
			ORDER BY type ASC, name ASC, id ASC
		`
		rows, err = r.db.QueryContext(ctx, query, userID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer rows.Close()

	categories := make([]domain.Category, 0)
	for rows.Next() {
		var cat domain.Category
		if err := rows.Scan(&cat.ID, &cat.UserID, &cat.Name, &cat.Type, &cat.CreatedAt, &cat.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, cat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return categories, nil
}

// Update updates the name of an existing category while verifying user ownership.
func (r *PostgresCategoryRepository) Update(ctx context.Context, category *domain.Category) error {
	now := time.Now().UTC()
	query := `
		UPDATE categories
		SET name = $1, updated_at = $2
		WHERE id = $3 AND user_id = $4
		RETURNING updated_at
	`
	err := r.db.QueryRowContext(ctx, query, category.Name, now, category.ID, category.UserID).
		Scan(&category.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: category not found", domain.ErrNotFound)
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%w: category %q with type %s already exists", domain.ErrConflict, category.Name, category.Type)
		}
		return fmt.Errorf("failed to update category: %w", err)
	}
	return nil
}

// Delete removes a category by ID while verifying user ownership.
func (r *PostgresCategoryRepository) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	query := `
		DELETE FROM categories
		WHERE id = $1 AND user_id = $2
	`
	res, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete category: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("%w: category not found", domain.ErrNotFound)
	}
	return nil
}
