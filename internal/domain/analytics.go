package domain

import "github.com/google/uuid"

// BudgetStatusOverview represents high-level budget progress for a month.
type BudgetStatusOverview struct {
	TotalBudget int64   `json:"total_budget"`
	TotalSpent  int64   `json:"total_spent"`
	Remaining   int64   `json:"remaining"`
	Percentage  float64 `json:"percentage"`
}

// MonthlySummary holds the high-level financial summary for a month.
type MonthlySummary struct {
	Month        string               `json:"month"` // YYYY-MM
	TotalIncome  int64                `json:"total_income"`
	TotalExpense int64                `json:"total_expense"`
	NetSavings   int64                `json:"net_savings"`
	BudgetStatus BudgetStatusOverview `json:"budget_status"`
}

// CategoryExpenseShare represents spending and percentage share for a category in a month.
type CategoryExpenseShare struct {
	CategoryID   *uuid.UUID `json:"category_id"`
	CategoryName string     `json:"category_name"`
	Amount       int64      `json:"amount"`
	Percentage   float64    `json:"percentage"`
}

// MonthlyTrendPoint represents historical monthly income and expense figures.
type MonthlyTrendPoint struct {
	Month      string `json:"month"` // YYYY-MM
	Income     int64  `json:"income"`
	Expense    int64  `json:"expense"`
	NetSavings int64  `json:"net_savings"`
}
