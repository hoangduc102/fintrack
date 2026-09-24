package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/database"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/repository"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://fintrack:fintrack_secret@localhost:5432/fintrack?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	db, err := database.Open(ctx, dbURL)
	if err != nil {
		t.Skipf("skipping postgres integration test: database unavailable: %v", err)
	}

	return db
}

func TestPostgresUserRepository_CreateWithCategories_Success(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewPostgresUserRepository(db)
	ctx := context.Background()

	displayName := "Test User"
	user := &domain.User{
		Email:        fmt.Sprintf("test-%s@example.com", uuid.New().String()[:8]),
		PasswordHash: "$2a$10$hashedpasswordplaceholder",
		DisplayName:  &displayName,
	}

	categories := make([]domain.Category, len(domain.DefaultCategories))
	for i, def := range domain.DefaultCategories {
		categories[i] = domain.Category{
			Name: def.Name,
			Type: def.Type,
		}
	}

	err := repo.CreateWithCategories(ctx, user, categories)
	if err != nil {
		t.Fatalf("failed to create user with categories: %v", err)
	}

	if user.ID == uuid.Nil {
		t.Fatal("expected user.ID to be populated, got Nil")
	}

	// Verify user was persisted
	foundUser, err := repo.FindByEmail(ctx, user.Email)
	if err != nil {
		t.Fatalf("failed to find user by email: %v", err)
	}
	if foundUser.ID != user.ID {
		t.Errorf("expected user ID %s, got %s", user.ID, foundUser.ID)
	}
	if foundUser.Email != user.Email {
		t.Errorf("expected email %s, got %s", user.Email, foundUser.Email)
	}

	// Verify all categories were persisted with user_id
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM categories WHERE user_id = $1", user.ID).Scan(&count)
	if err != nil {
		t.Fatalf("failed to count categories: %v", err)
	}
	if count != len(domain.DefaultCategories) {
		t.Errorf("expected %d categories, got %d", len(domain.DefaultCategories), count)
	}
}

func TestPostgresUserRepository_Create_DuplicateEmailReturnsConflict(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewPostgresUserRepository(db)
	ctx := context.Background()

	email := fmt.Sprintf("dup-%s@example.com", uuid.New().String()[:8])
	user1 := &domain.User{
		Email:        email,
		PasswordHash: "$2a$10$hashedpasswordplaceholder",
	}

	err := repo.CreateWithCategories(ctx, user1, nil)
	if err != nil {
		t.Fatalf("expected initial create to succeed, got %v", err)
	}

	user2 := &domain.User{
		Email:        email,
		PasswordHash: "$2a$10$hashedpasswordplaceholder",
	}
	err = repo.CreateWithCategories(ctx, user2, nil)
	if err == nil {
		t.Fatal("expected duplicate email error, got nil")
	}

	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("expected domain.ErrConflict, got %v", err)
	}
}

func TestPostgresUserRepository_Atomicity_RollbackOnFailure(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := repository.NewPostgresUserRepository(db)
	ctx := context.Background()

	email := fmt.Sprintf("atomic-%s@example.com", uuid.New().String()[:8])
	user := &domain.User{
		Email:        email,
		PasswordHash: "$2a$10$hashedpasswordplaceholder",
	}

	// Include an invalid category with illegal type to force constraint violation
	categories := []domain.Category{
		{Name: "Valid Expense", Type: domain.CategoryTypeExpense},
		{Name: "Invalid Category", Type: domain.CategoryType("INVALID_TYPE")},
	}

	err := repo.CreateWithCategories(ctx, user, categories)
	if err == nil {
		t.Fatal("expected category creation to fail due to CHECK constraint, got nil")
	}

	// Verify that user was NOT created due to rollback
	var exists bool
	err = db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", email).Scan(&exists)
	if err != nil {
		t.Fatalf("failed to check user existence: %v", err)
	}
	if exists {
		t.Fatal("expected user to be rolled back, but user exists in database")
	}
}
