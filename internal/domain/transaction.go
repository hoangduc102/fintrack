package domain

import (
	"time"

	"github.com/google/uuid"
)

// TransactionType defines whether a transaction is an Income or Expense.
type TransactionType string

const (
	TransactionTypeIncome  TransactionType = "INCOME"
	TransactionTypeExpense TransactionType = "EXPENSE"
)

// Transaction represents a record of financial inflow or outflow occurring on a calendar date.
type Transaction struct {
	ID              uuid.UUID       `json:"id"`
	UserID          uuid.UUID       `json:"user_id"`
	Amount          int64           `json:"amount"`
	Type            TransactionType `json:"type"`
	CategoryID      *uuid.UUID      `json:"category_id"`
	TransactionDate string          `json:"transaction_date"` // YYYY-MM-DD
	Description     *string         `json:"description"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}
