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

type PostgresUserRepository struct {
	db *sql.DB
}

func NewPostgresUserRepository(db *sql.DB) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) CreateWithCategories(ctx context.Context, user *domain.User, categories []domain.Category) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	now := time.Now().UTC()
	user.CreatedAt = now
	user.UpdatedAt = now

	queryUser := `
		INSERT INTO users (id, email, password_hash, display_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`
	err = tx.QueryRowContext(ctx, queryUser, user.ID, user.Email, user.PasswordHash, user.DisplayName, user.CreatedAt, user.UpdatedAt).
		Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%w: email %q already exists", domain.ErrConflict, user.Email)
		}
		return fmt.Errorf("failed to insert user: %w", err)
	}

	if len(categories) > 0 {
		queryCategory := `
			INSERT INTO categories (id, user_id, name, type, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`
		stmt, err := tx.PrepareContext(ctx, queryCategory)
		if err != nil {
			return fmt.Errorf("failed to prepare category insert: %w", err)
		}
		defer stmt.Close()

		for i := range categories {
			catID := uuid.New()
			_, err = stmt.ExecContext(ctx, catID, user.ID, categories[i].Name, categories[i].Type, now, now)
			if err != nil {
				return fmt.Errorf("failed to insert category %q: %w", categories[i].Name, err)
			}
			categories[i].ID = catID
			categories[i].UserID = user.ID
			categories[i].CreatedAt = now
			categories[i].UpdatedAt = now
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (r *PostgresUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, display_name, created_at, updated_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`
	var user domain.User
	err := r.db.QueryRowContext(ctx, query, email).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: user not found", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("failed to find user by email: %w", err)
	}
	return &user, nil
}

func (r *PostgresUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, display_name, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var user domain.User
	err := r.db.QueryRowContext(ctx, query, id).
		Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DisplayName, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: user not found", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("failed to find user by id: %w", err)
	}
	return &user, nil
}
