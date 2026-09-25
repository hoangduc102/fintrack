package domain

import (
	"time"

	"github.com/google/uuid"
)

// Budget represents a monthly spending limit set exclusively for an expense category.
type Budget struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	CategoryID   uuid.UUID `json:"category_id"`
	CategoryName string    `json:"category_name,omitempty"`
	Amount       int64     `json:"amount"`
	Spent        int64     `json:"spent"`
	Year         int       `json:"year"`
	Month        int       `json:"month"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
