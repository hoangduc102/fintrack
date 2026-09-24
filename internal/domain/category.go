package domain

import (
	"time"

	"github.com/google/uuid"
)

// CategoryType defines whether a category is for Income or Expense transactions.
type CategoryType string

const (
	CategoryTypeIncome  CategoryType = "INCOME"
	CategoryTypeExpense CategoryType = "EXPENSE"
)

// Category represents a classification assigned to a transaction, partitioned by type and scoped per user.
type Category struct {
	ID        uuid.UUID    `json:"id"`
	UserID    uuid.UUID    `json:"user_id"`
	Name      string       `json:"name"`
	Type      CategoryType `json:"type"`
	CreatedAt time.Time    `json:"created_at"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// DefaultCategories lists the standard default categories seeded upon user registration.
var DefaultCategories = []struct {
	Name string
	Type CategoryType
}{
	{Name: "Food & Dining", Type: CategoryTypeExpense},
	{Name: "Transportation", Type: CategoryTypeExpense},
	{Name: "Housing", Type: CategoryTypeExpense},
	{Name: "Entertainment", Type: CategoryTypeExpense},
	{Name: "Salary", Type: CategoryTypeIncome},
	{Name: "Freelance", Type: CategoryTypeIncome},
}
