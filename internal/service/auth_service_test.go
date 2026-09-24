package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/service"
)

// fakeUserRepository is an in-memory implementation of repository.UserRepository for unit tests.
type fakeUserRepository struct {
	users      map[string]*domain.User // keyed by lower(email)
	categories map[uuid.UUID][]domain.Category
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		users:      make(map[string]*domain.User),
		categories: make(map[uuid.UUID][]domain.Category),
	}
}

func (f *fakeUserRepository) CreateWithCategories(ctx context.Context, user *domain.User, categories []domain.Category) error {
	lowerEmail := strings.ToLower(user.Email)
	if _, exists := f.users[lowerEmail]; exists {
		return domain.ErrConflict
	}

	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}

	f.users[lowerEmail] = user
	f.categories[user.ID] = categories
	return nil
}

func (f *fakeUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	lowerEmail := strings.ToLower(email)
	if u, exists := f.users[lowerEmail]; exists {
		return u, nil
	}
	return nil, domain.ErrNotFound
}

func (f *fakeUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	for _, u := range f.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, domain.ErrNotFound
}

func TestAuthService_Register_Success(t *testing.T) {
	repo := newFakeUserRepository()
	authService := service.NewAuthService(repo)

	displayName := "Duc Hoang"
	input := service.RegisterInput{
		Email:       "  User@Example.COM ",
		Password:    "password123",
		DisplayName: &displayName,
	}

	user, err := authService.Register(context.Background(), input)
	if err != nil {
		t.Fatalf("expected successful registration, got: %v", err)
	}

	// 1. Assert email normalization
	if user.Email != "user@example.com" {
		t.Errorf("expected normalized email 'user@example.com', got %q", user.Email)
	}

	// 2. Assert display name
	if user.DisplayName == nil || *user.DisplayName != displayName {
		t.Errorf("expected display name %q, got %v", displayName, user.DisplayName)
	}

	// 3. Assert password hashed with bcrypt
	if user.PasswordHash == "" || user.PasswordHash == "password123" {
		t.Errorf("password was not hashed: %q", user.PasswordHash)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("password123")); err != nil {
		t.Errorf("password hash verification failed: %v", err)
	}

	// 4. Assert default categories seeded
	cats := repo.categories[user.ID]
	if len(cats) != len(domain.DefaultCategories) {
		t.Fatalf("expected %d default categories, got %d", len(domain.DefaultCategories), len(cats))
	}

	expectedCats := map[string]domain.CategoryType{
		"Food & Dining":  domain.CategoryTypeExpense,
		"Transportation": domain.CategoryTypeExpense,
		"Housing":        domain.CategoryTypeExpense,
		"Entertainment":  domain.CategoryTypeExpense,
		"Salary":         domain.CategoryTypeIncome,
		"Freelance":      domain.CategoryTypeIncome,
	}

	for _, c := range cats {
		expectedType, ok := expectedCats[c.Name]
		if !ok {
			t.Errorf("unexpected category name seeded: %s", c.Name)
		}
		if c.Type != expectedType {
			t.Errorf("category %s: expected type %s, got %s", c.Name, expectedType, c.Type)
		}
	}
}

func TestAuthService_Register_ValidationErrors(t *testing.T) {
	tests := []struct {
		name          string
		input         service.RegisterInput
		expectedField string
	}{
		{
			name: "empty email",
			input: service.RegisterInput{
				Email:    "",
				Password: "password123",
			},
			expectedField: "email",
		},
		{
			name: "whitespace-only email",
			input: service.RegisterInput{
				Email:    "   ",
				Password: "password123",
			},
			expectedField: "email",
		},
		{
			name: "invalid email format without at-sign",
			input: service.RegisterInput{
				Email:    "invalid-email",
				Password: "password123",
			},
			expectedField: "email",
		},
		{
			name: "invalid email format without domain",
			input: service.RegisterInput{
				Email:    "user@",
				Password: "password123",
			},
			expectedField: "email",
		},
		{
			name: "password less than 8 characters",
			input: service.RegisterInput{
				Email:    "user@example.com",
				Password: "1234567",
			},
			expectedField: "password",
		},
		{
			name: "password exceeding 72 bytes",
			input: service.RegisterInput{
				Email:    "user@example.com",
				Password: strings.Repeat("a", 73),
			},
			expectedField: "password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newFakeUserRepository()
			authService := service.NewAuthService(repo)

			_, err := authService.Register(context.Background(), tt.input)
			if err == nil {
				t.Fatalf("expected validation error for %s, got nil", tt.name)
			}

			var valErr domain.ValidationError
			if !errors.As(err, &valErr) {
				t.Fatalf("expected domain.ValidationError, got: %T (%v)", err, err)
			}

			if valErr.Field != tt.expectedField {
				t.Errorf("expected validation field %q, got %q", tt.expectedField, valErr.Field)
			}
		})
	}
}

func TestAuthService_Register_PasswordBoundaryLengths(t *testing.T) {
	repo := newFakeUserRepository()
	authService := service.NewAuthService(repo)

	// Exactly 8 characters
	t.Run("exactly 8 chars passes", func(t *testing.T) {
		input := service.RegisterInput{
			Email:    "len8@example.com",
			Password: "12345678",
		}
		_, err := authService.Register(context.Background(), input)
		if err != nil {
			t.Errorf("expected 8 chars password to succeed, got %v", err)
		}
	})

	// Exactly 72 bytes
	t.Run("exactly 72 bytes passes", func(t *testing.T) {
		input := service.RegisterInput{
			Email:    "len72@example.com",
			Password: strings.Repeat("x", 72),
		}
		_, err := authService.Register(context.Background(), input)
		if err != nil {
			t.Errorf("expected 72 bytes password to succeed, got %v", err)
		}
	})
}

func TestAuthService_Register_DuplicateEmailConflict(t *testing.T) {
	repo := newFakeUserRepository()
	authService := service.NewAuthService(repo)

	input := service.RegisterInput{
		Email:    "existing@example.com",
		Password: "password123",
	}

	_, err := authService.Register(context.Background(), input)
	if err != nil {
		t.Fatalf("first registration failed: %v", err)
	}

	// Try registering with same email (case-insensitive)
	inputDuplicate := service.RegisterInput{
		Email:    "EXISTING@example.com",
		Password: "password456",
	}
	_, err = authService.Register(context.Background(), inputDuplicate)
	if err == nil {
		t.Fatal("expected conflict error for duplicate email, got nil")
	}

	if !errors.Is(err, domain.ErrConflict) {
		t.Errorf("expected domain.ErrConflict, got: %v", err)
	}
}
