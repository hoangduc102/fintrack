package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"github.com/hoangduc102/fintrack/internal/auth"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/repository"
)

// RegisterInput holds the data required to register a new user.
type RegisterInput struct {
	Email       string
	Password    string
	DisplayName *string
}

// LoginInput holds the credentials required for user authentication.
type LoginInput struct {
	Email    string
	Password string
}

// LoginOutput contains the JWT access token and authenticated user details.
type LoginOutput struct {
	Token string
	User  *domain.User
}

// AuthService coordinates user registration, authentication, and password management.
type AuthService struct {
	userRepo   repository.UserRepository
	jwtManager *auth.JWTManager
}

// NewAuthService creates a new AuthService instance.
func NewAuthService(userRepo repository.UserRepository, jwtManagers ...*auth.JWTManager) *AuthService {
	var jm *auth.JWTManager
	if len(jwtManagers) > 0 && jwtManagers[0] != nil {
		jm = jwtManagers[0]
	}
	return &AuthService{
		userRepo:   userRepo,
		jwtManager: jm,
	}
}

// Register validates user input, hashes the password, checks uniqueness,
// and atomically persists the user with default seeded categories.
func (s *AuthService) Register(ctx context.Context, input RegisterInput) (*domain.User, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(input.Email))
	if normalizedEmail == "" {
		return nil, domain.ValidationError{Field: "email", Message: "email is required"}
	}

	addr, err := mail.ParseAddress(normalizedEmail)
	if err != nil || addr.Address != normalizedEmail {
		return nil, domain.ValidationError{Field: "email", Message: "invalid email format"}
	}

	atIdx := strings.LastIndex(normalizedEmail, "@")
	if atIdx <= 0 || atIdx >= len(normalizedEmail)-1 {
		return nil, domain.ValidationError{Field: "email", Message: "invalid email format"}
	}
	domainPart := normalizedEmail[atIdx+1:]
	if !strings.Contains(domainPart, ".") {
		return nil, domain.ValidationError{Field: "email", Message: "invalid email format"}
	}

	if len(input.Password) < 8 {
		return nil, domain.ValidationError{Field: "password", Message: "password must be at least 8 characters"}
	}
	if len([]byte(input.Password)) > 72 {
		return nil, domain.ValidationError{Field: "password", Message: "password cannot exceed 72 bytes"}
	}

	// Application-level uniqueness check
	existing, err := s.userRepo.FindByEmail(ctx, normalizedEmail)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("%w: email %q is already registered", domain.ErrConflict, normalizedEmail)
	}
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	var displayName *string
	if input.DisplayName != nil {
		trimmed := strings.TrimSpace(*input.DisplayName)
		if trimmed != "" {
			displayName = &trimmed
		}
	}

	user := &domain.User{
		Email:        normalizedEmail,
		PasswordHash: string(hashedPassword),
		DisplayName:  displayName,
	}

	categories := make([]domain.Category, len(domain.DefaultCategories))
	for i, def := range domain.DefaultCategories {
		categories[i] = domain.Category{
			Name: def.Name,
			Type: def.Type,
		}
	}

	if err := s.userRepo.CreateWithCategories(ctx, user, categories); err != nil {
		return nil, err
	}

	return user, nil
}

// Login verifies credentials against stored bcrypt hash and returns a 24-hour JWT token.
func (s *AuthService) Login(ctx context.Context, input LoginInput) (*LoginOutput, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(input.Email))
	if normalizedEmail == "" || input.Password == "" {
		return nil, domain.ErrInvalidCredentials
	}

	user, err := s.userRepo.FindByEmail(ctx, normalizedEmail)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, domain.ErrInvalidCredentials
	}

	if s.jwtManager == nil {
		return nil, errors.New("jwt manager is not configured")
	}

	token, err := s.jwtManager.Generate(user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &LoginOutput{
		Token: token,
		User:  user,
	}, nil
}

// GetProfile retrieves user account details by ID.
func (s *AuthService) GetProfile(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.userRepo.FindByID(ctx, userID)
}
