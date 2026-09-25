package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/service"
)

// AnalyticsHandler handles HTTP requests for financial analytics.
type AnalyticsHandler struct {
	analyticsService *service.AnalyticsService
}

// NewAnalyticsHandler creates a new AnalyticsHandler.
func NewAnalyticsHandler(analyticsService *service.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{analyticsService: analyticsService}
}

// CategoryExpenseResponse represents the category expense share in HTTP responses.
type CategoryExpenseResponse struct {
	CategoryID   *string `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Amount       int64   `json:"amount"`
	Percentage   float64 `json:"percentage"`
}

// GetSummary handles GET /api/v1/analytics/summary?month=YYYY-MM.
func (h *AnalyticsHandler) GetSummary(c *gin.Context) {
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

	monthStr := c.Query("month")
	if monthStr == "" {
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

	summary, err := h.analyticsService.GetMonthlySummary(c.Request.Context(), userID, monthStr)
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
				"message": "failed to get monthly summary",
			},
		})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// GetCategories handles GET /api/v1/analytics/categories?month=YYYY-MM.
func (h *AnalyticsHandler) GetCategories(c *gin.Context) {
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

	monthStr := c.Query("month")
	if monthStr == "" {
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

	shares, err := h.analyticsService.GetCategoryExpenses(c.Request.Context(), userID, monthStr)
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
				"message": "failed to get category expenses",
			},
		})
		return
	}

	resp := make([]CategoryExpenseResponse, 0, len(shares))
	for _, s := range shares {
		var catID *string
		if s.CategoryID != nil {
			idStr := s.CategoryID.String()
			catID = &idStr
		}
		resp = append(resp, CategoryExpenseResponse{
			CategoryID:   catID,
			CategoryName: s.CategoryName,
			Amount:       s.Amount,
			Percentage:   s.Percentage,
		})
	}

	c.JSON(http.StatusOK, resp)
}

// GetTrend handles GET /api/v1/analytics/trend?months=6.
func (h *AnalyticsHandler) GetTrend(c *gin.Context) {
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

	months := 6
	if mStr := c.Query("months"); mStr != "" {
		m, err := strconv.Atoi(mStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "months must be an integer",
					"fields": gin.H{
						"months": "months must be an integer",
					},
				},
			})
			return
		}
		if m < 1 || m > 24 {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "VALIDATION_ERROR",
					"message": "months must be between 1 and 24",
					"fields": gin.H{
						"months": "months must be between 1 and 24",
					},
				},
			})
			return
		}
		months = m
	}

	trend, err := h.analyticsService.GetMonthlyTrend(c.Request.Context(), userID, months)
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
				"message": "failed to get monthly trend",
			},
		})
		return
	}

	c.JSON(http.StatusOK, trend)
}
