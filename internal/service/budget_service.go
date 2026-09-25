package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/repository"
)

// UpsertBudgetInput holds data for creating or updating a budget.
type UpsertBudgetInput struct {
	UserID     uuid.UUID
	CategoryID uuid.UUID
	Amount     int64
	Year       int
	Month      int
}

// BudgetService coordinates business logic and validations for monthly budgets.
type BudgetService struct {
	budgetRepo repository.BudgetRepository
	catRepo    repository.CategoryRepository
}

// NewBudgetService creates a new BudgetService.
func NewBudgetService(budgetRepo repository.BudgetRepository, catRepo repository.CategoryRepository) *BudgetService {
	return &BudgetService{
		budgetRepo: budgetRepo,
		catRepo:    catRepo,
	}
}

// UpsertBudget validates input and creates or updates a monthly budget limit for a category.
func (s *BudgetService) UpsertBudget(ctx context.Context, input UpsertBudgetInput) (*domain.Budget, error) {
	if input.Amount <= 0 {
		return nil, domain.ValidationError{Field: "amount", Message: "amount must be greater than 0"}
	}
	if input.Year < 2000 || input.Year > 2100 {
		return nil, domain.ValidationError{Field: "year", Message: "year must be between 2000 and 2100"}
	}
	if input.Month < 1 || input.Month > 12 {
		return nil, domain.ValidationError{Field: "month", Message: "month must be between 1 and 12"}
	}
	if input.CategoryID == uuid.Nil {
		return nil, domain.ValidationError{Field: "category_id", Message: "category_id is required"}
	}

	cat, err := s.catRepo.FindByID(ctx, input.CategoryID, input.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, domain.ValidationError{Field: "category_id", Message: "category not found or belongs to another user"}
		}
		return nil, err
	}

	if cat.Type != domain.CategoryTypeExpense {
		return nil, domain.ValidationError{Field: "category_id", Message: "budgets can only be set on expense categories"}
	}

	budget := &domain.Budget{
		UserID:     input.UserID,
		CategoryID: input.CategoryID,
		Amount:     input.Amount,
		Year:       input.Year,
		Month:      input.Month,
	}

	if err := s.budgetRepo.Upsert(ctx, budget); err != nil {
		return nil, err
	}

	spent, err := s.budgetRepo.GetSpent(ctx, input.UserID, input.CategoryID, input.Year, input.Month)
	if err == nil {
		budget.Spent = spent
	}
	budget.CategoryName = cat.Name

	return budget, nil
}

// ListBudgetsByMonth retrieves all budgets for the specified user, year, and month.
func (s *BudgetService) ListBudgetsByMonth(ctx context.Context, userID uuid.UUID, year int, month int) ([]domain.Budget, error) {
	if year < 2000 || year > 2100 {
		return nil, domain.ValidationError{Field: "year", Message: "year must be between 2000 and 2100"}
	}
	if month < 1 || month > 12 {
		return nil, domain.ValidationError{Field: "month", Message: "month must be between 1 and 12"}
	}

	return s.budgetRepo.ListByMonth(ctx, userID, year, month)
}

// DeleteBudget removes a budget by ID and user ID.
func (s *BudgetService) DeleteBudget(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	return s.budgetRepo.Delete(ctx, id, userID)
}
