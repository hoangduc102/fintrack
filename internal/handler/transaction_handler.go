package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/service"
)

// TransactionHandler handles HTTP endpoints for transactions.
type TransactionHandler struct {
	txService *service.TransactionService
}

// NewTransactionHandler creates a new TransactionHandler.
func NewTransactionHandler(txService *service.TransactionService) *TransactionHandler {
	return &TransactionHandler{txService: txService}
}

// CreateTransactionRequest defines payload for POST /api/v1/transactions.
type CreateTransactionRequest struct {
	Amount          int64   `json:"amount"`
	Type            string  `json:"type"`
	CategoryID      *string `json:"category_id"`
	TransactionDate string  `json:"transaction_date"`
	Description     *string `json:"description"`
}

// UpdateTransactionRequest defines payload for PUT /api/v1/transactions/:id.
type UpdateTransactionRequest struct {
	Amount          int64   `json:"amount"`
	Type            string  `json:"type"`
	CategoryID      *string `json:"category_id"`
	TransactionDate string  `json:"transaction_date"`
	Description     *string `json:"description"`
}

// TransactionResponse defines flat JSON response for a transaction.
type TransactionResponse struct {
	ID              string  `json:"id"`
	UserID          string  `json:"user_id"`
	Amount          int64   `json:"amount"`
	Type            string  `json:"type"`
	CategoryID      *string `json:"category_id"`
	TransactionDate string  `json:"transaction_date"`
	Description     *string `json:"description,omitempty"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

func toTransactionResponse(tx *domain.Transaction) TransactionResponse {
	var catID *string
	if tx.CategoryID != nil {
		s := tx.CategoryID.String()
		catID = &s
	}

	return TransactionResponse{
		ID:              tx.ID.String(),
		UserID:          tx.UserID.String(),
		Amount:          tx.Amount,
		Type:            string(tx.Type),
		CategoryID:      catID,
		TransactionDate: tx.TransactionDate,
		Description:     tx.Description,
		CreatedAt:       tx.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       tx.UpdatedAt.Format(time.RFC3339),
	}
}

// Create handles POST /api/v1/transactions.
func (h *TransactionHandler) Create(c *gin.Context) {
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

	var req CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "BAD_REQUEST",
				"message": "invalid request body",
			},
		})
		return
	}

	var catID *uuid.UUID
	if req.CategoryID != nil && *req.CategoryID != "" {
		parsed, err := uuid.Parse(*req.CategoryID)
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
		catID = &parsed
	}

	tx, err := h.txService.CreateTransaction(c.Request.Context(), service.CreateTransactionInput{
		UserID:          userID,
		Amount:          req.Amount,
		Type:            domain.TransactionType(req.Type),
		CategoryID:      catID,
		TransactionDate: req.TransactionDate,
		Description:     req.Description,
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
				"message": "failed to create transaction",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, toTransactionResponse(tx))
}

// GetByID handles GET /api/v1/transactions/:id.
func (h *TransactionHandler) GetByID(c *gin.Context) {
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

	txID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "BAD_REQUEST",
				"message": "invalid transaction id",
			},
		})
		return
	}

	tx, err := h.txService.GetTransaction(c.Request.Context(), txID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "transaction not found",
				},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "failed to retrieve transaction",
			},
		})
		return
	}

	c.JSON(http.StatusOK, toTransactionResponse(tx))
}

// Update handles PUT /api/v1/transactions/:id.
func (h *TransactionHandler) Update(c *gin.Context) {
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

	txID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "BAD_REQUEST",
				"message": "invalid transaction id",
			},
		})
		return
	}

	var req UpdateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "BAD_REQUEST",
				"message": "invalid request body",
			},
		})
		return
	}

	var catID *uuid.UUID
	if req.CategoryID != nil && *req.CategoryID != "" {
		parsed, err := uuid.Parse(*req.CategoryID)
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
		catID = &parsed
	}

	tx, err := h.txService.UpdateTransaction(c.Request.Context(), service.UpdateTransactionInput{
		ID:              txID,
		UserID:          userID,
		Amount:          req.Amount,
		Type:            domain.TransactionType(req.Type),
		CategoryID:      catID,
		TransactionDate: req.TransactionDate,
		Description:     req.Description,
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

		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "transaction not found",
				},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "failed to update transaction",
			},
		})
		return
	}

	c.JSON(http.StatusOK, toTransactionResponse(tx))
}

// Delete handles DELETE /api/v1/transactions/:id.
func (h *TransactionHandler) Delete(c *gin.Context) {
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

	txID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "BAD_REQUEST",
				"message": "invalid transaction id",
			},
		})
		return
	}

	if err := h.txService.DeleteTransaction(c.Request.Context(), txID, userID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "transaction not found",
				},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "failed to delete transaction",
			},
		})
		return
	}

	c.Status(http.StatusNoContent)
}
