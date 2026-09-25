package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/service"
)

// BudgetHandler handles HTTP requests for monthly budgets.
type BudgetHandler struct {
	budgetService *service.BudgetService
}

// NewBudgetHandler creates a new BudgetHandler.
func NewBudgetHandler(budgetService *service.BudgetService) *BudgetHandler {
	return &BudgetHandler{budgetService: budgetService}
}

// UpsertBudgetRequest defines payload for PUT /api/v1/budgets.
type UpsertBudgetRequest struct {
	CategoryID string `json:"category_id"`
	Amount     int64  `json:"amount"`
	Year       int    `json:"year"`
	Month      int    `json:"month"`
}

// UnmarshalJSON supports both integer month and YYYY-MM string format.
func (r *UpsertBudgetRequest) UnmarshalJSON(data []byte) error {
	type Alias UpsertBudgetRequest
	var aux struct {
		Alias
		MonthRaw interface{} `json:"month"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*r = UpsertBudgetRequest(aux.Alias)
	switch v := aux.MonthRaw.(type) {
	case float64:
		r.Month = int(v)
	case string:
		if t, err := time.Parse("2006-01", v); err == nil {
			r.Year = t.Year()
			r.Month = int(t.Month())
		} else if m, err := strconv.Atoi(v); err == nil {
			r.Month = m
		}
	}
	return nil
}

// BudgetResponse defines the JSON response structure for a budget.
type BudgetResponse struct {
	ID           string `json:"id"`
	UserID       string `json:"user_id"`
	CategoryID   string `json:"category_id"`
	CategoryName string `json:"category_name,omitempty"`
	Amount       int64  `json:"amount"`
	Spent        int64  `json:"spent"`
	Year         int    `json:"year"`
	Month        int    `json:"month"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

func toBudgetResponse(b *domain.Budget) BudgetResponse {
	return BudgetResponse{
		ID:           b.ID.String(),
		UserID:       b.UserID.String(),
		CategoryID:   b.CategoryID.String(),
		CategoryName: b.CategoryName,
		Amount:       b.Amount,
		Spent:        b.Spent,
		Year:         b.Year,
		Month:        b.Month,
		CreatedAt:    b.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    b.UpdatedAt.Format(time.RFC3339),
	}
}

// Upsert handles PUT /api/v1/budgets.
func (h *BudgetHandler) Upsert(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "unauthorized",
			},
		})
		return
	}

	var req UpsertBudgetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "BAD_REQUEST",
				"message": "invalid request body",
			},
		})
		return
	}

	catID, err := uuid.Parse(req.CategoryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "VALIDATION_ERROR",
				"message": "invalid category_id UUID",
				"fields": gin.H{
					"category_id": "invalid category_id UUID",
				},
			},
		})
		return
	}

	budget, err := h.budgetService.UpsertBudget(c.Request.Context(), service.UpsertBudgetInput{
		UserID:     userID,
		CategoryID: catID,
		Amount:     req.Amount,
		Year:       req.Year,
		Month:      req.Month,
	})
	if err != nil {
		var valErr domain.ValidationError
		if errors.As(err, &valErr) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": valErr.Message,
					"fields": gin.H{
						valErr.Field: valErr.Message,
					},
				},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "failed to upsert budget",
			},
		})
		return
	}

	c.JSON(http.StatusOK, toBudgetResponse(budget))
}

// List handles GET /api/v1/budgets?month=YYYY-MM.
func (h *BudgetHandler) List(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "unauthorized",
			},
		})
		return
	}

	monthParam := c.Query("month")
	if monthParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "VALIDATION_ERROR",
				"message": "month query parameter is required (format: YYYY-MM)",
				"fields": gin.H{
					"month": "month query parameter is required (format: YYYY-MM)",
				},
			},
		})
		return
	}

	t, err := time.Parse("2006-01", monthParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "VALIDATION_ERROR",
				"message": "month must be in YYYY-MM format",
				"fields": gin.H{
					"month": "month must be in YYYY-MM format",
				},
			},
		})
		return
	}

	budgets, err := h.budgetService.ListBudgetsByMonth(c.Request.Context(), userID, t.Year(), int(t.Month()))
	if err != nil {
		var valErr domain.ValidationError
		if errors.As(err, &valErr) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": valErr.Message,
					"fields": gin.H{
						valErr.Field: valErr.Message,
					},
				},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "failed to list budgets",
			},
		})
		return
	}

	resp := make([]BudgetResponse, 0, len(budgets))
	for i := range budgets {
		resp = append(resp, toBudgetResponse(&budgets[i]))
	}

	c.JSON(http.StatusOK, resp)
}

// Delete handles DELETE /api/v1/budgets/:id.
func (h *BudgetHandler) Delete(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{
				"code":    "UNAUTHORIZED",
				"message": "unauthorized",
			},
		})
		return
	}

	budgetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "BAD_REQUEST",
				"message": "invalid budget id",
			},
		})
		return
	}

	if err := h.budgetService.DeleteBudget(c.Request.Context(), budgetID, userID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "budget not found",
				},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "failed to delete budget",
			},
		})
		return
	}

	c.Status(http.StatusNoContent)
}
