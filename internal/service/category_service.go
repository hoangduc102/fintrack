package service

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/repository"
)

// CreateCategoryInput holds the parameters required to create a new category.
type CreateCategoryInput struct {
	UserID uuid.UUID
	Name   string
	Type   domain.CategoryType
}

// UpdateCategoryInput holds the parameters required to update an existing category.
type UpdateCategoryInput struct {
	ID     uuid.UUID
	UserID uuid.UUID
	Name   string
}

// ListCategoriesInput holds parameters for listing categories, with an optional type filter.
type ListCategoriesInput struct {
	UserID uuid.UUID
	Type   *domain.CategoryType
}

// CategoryService handles business logic and validation for categories.
type CategoryService struct {
	categoryRepo repository.CategoryRepository
}

// NewCategoryService creates a new CategoryService.
func NewCategoryService(categoryRepo repository.CategoryRepository) *CategoryService {
	return &CategoryService{categoryRepo: categoryRepo}
}

// CreateCategory validates input and creates a category.
func (s *CategoryService) CreateCategory(ctx context.Context, input CreateCategoryInput) (*domain.Category, error) {
	trimmedName := strings.TrimSpace(input.Name)
	if trimmedName == "" {
		return nil, domain.ValidationError{Field: "name", Message: "category name is required"}
	}
	if len([]rune(trimmedName)) > 50 {
		return nil, domain.ValidationError{Field: "name", Message: "category name must not exceed 50 characters"}
	}

	if input.Type != domain.CategoryTypeIncome && input.Type != domain.CategoryTypeExpense {
		return nil, domain.ValidationError{Field: "type", Message: "type must be INCOME or EXPENSE"}
	}

	cat := &domain.Category{
		UserID: input.UserID,
		Name:   trimmedName,
		Type:   input.Type,
	}

	if err := s.categoryRepo.Create(ctx, cat); err != nil {
		return nil, err
	}

	return cat, nil
}

// ListCategories retrieves categories for a user with optional type filtering.
func (s *CategoryService) ListCategories(ctx context.Context, input ListCategoriesInput) ([]domain.Category, error) {
	if input.Type != nil {
		if *input.Type != domain.CategoryTypeIncome && *input.Type != domain.CategoryTypeExpense {
			return nil, domain.ValidationError{Field: "type", Message: "type must be INCOME or EXPENSE"}
		}
	}

	return s.categoryRepo.FindByUserAndType(ctx, input.UserID, input.Type)
}

// UpdateCategory validates input, verifies user ownership, and updates the category name.
func (s *CategoryService) UpdateCategory(ctx context.Context, input UpdateCategoryInput) (*domain.Category, error) {
	trimmedName := strings.TrimSpace(input.Name)
	if trimmedName == "" {
		return nil, domain.ValidationError{Field: "name", Message: "category name is required"}
	}
	if len([]rune(trimmedName)) > 50 {
		return nil, domain.ValidationError{Field: "name", Message: "category name must not exceed 50 characters"}
	}

	existing, err := s.categoryRepo.FindByID(ctx, input.ID, input.UserID)
	if err != nil {
		return nil, err
	}

	existing.Name = trimmedName
	if err := s.categoryRepo.Update(ctx, existing); err != nil {
		return nil, err
	}

	return existing, nil
}

// DeleteCategory deletes a category ensuring user ownership isolation.
func (s *CategoryService) DeleteCategory(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	return s.categoryRepo.Delete(ctx, id, userID)
}
