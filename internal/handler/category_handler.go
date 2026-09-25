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

// CategoryHandler handles HTTP requests for categories.
type CategoryHandler struct {
	categoryService *service.CategoryService
}

// NewCategoryHandler creates a new CategoryHandler.
func NewCategoryHandler(categoryService *service.CategoryService) *CategoryHandler {
	return &CategoryHandler{categoryService: categoryService}
}

// CreateCategoryRequest defines the payload for creating a category.
type CreateCategoryRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// UpdateCategoryRequest defines the payload for updating a category.
type UpdateCategoryRequest struct {
	Name string `json:"name"`
}

// CategoryResponse defines the response structure for a category.
type CategoryResponse struct {
	ID        string `json:"id"`
	UserID    string `json:"user_id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func toCategoryResponse(c *domain.Category) CategoryResponse {
	return CategoryResponse{
		ID:        c.ID.String(),
		UserID:    c.UserID.String(),
		Name:      c.Name,
		Type:      string(c.Type),
		CreatedAt: c.CreatedAt.Format(time.RFC3339),
		UpdatedAt: c.UpdatedAt.Format(time.RFC3339),
	}
}

// List handles GET /api/v1/categories (optional ?type=INCOME|EXPENSE).
func (h *CategoryHandler) List(c *gin.Context) {
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

	var catType *domain.CategoryType
	if typeParam := c.Query("type"); typeParam != "" {
		ct := domain.CategoryType(typeParam)
		catType = &ct
	}

	categories, err := h.categoryService.ListCategories(c.Request.Context(), service.ListCategoriesInput{
		UserID: userID,
		Type:   catType,
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
				"message": "failed to list categories",
			},
		})
		return
	}

	resp := make([]CategoryResponse, len(categories))
	for i := range categories {
		resp[i] = toCategoryResponse(&categories[i])
	}

	c.JSON(http.StatusOK, resp)
}

// Create handles POST /api/v1/categories.
func (h *CategoryHandler) Create(c *gin.Context) {
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

	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "BAD_REQUEST",
				"message": "invalid request body",
			},
		})
		return
	}

	cat, err := h.categoryService.CreateCategory(c.Request.Context(), service.CreateCategoryInput{
		UserID: userID,
		Name:   req.Name,
		Type:   domain.CategoryType(req.Type),
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

		if errors.Is(err, domain.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{
				"error": gin.H{
					"code":    "CONFLICT",
					"message": "category already exists",
				},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "failed to create category",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, toCategoryResponse(cat))
}

// Update handles PUT /api/v1/categories/:id.
func (h *CategoryHandler) Update(c *gin.Context) {
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

	catID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "BAD_REQUEST",
				"message": "invalid category id",
			},
		})
		return
	}

	var req UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "BAD_REQUEST",
				"message": "invalid request body",
			},
		})
		return
	}

	cat, err := h.categoryService.UpdateCategory(c.Request.Context(), service.UpdateCategoryInput{
		ID:     catID,
		UserID: userID,
		Name:   req.Name,
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
					"message": "category not found",
				},
			})
			return
		}

		if errors.Is(err, domain.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{
				"error": gin.H{
					"code":    "CONFLICT",
					"message": "category already exists",
				},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "failed to update category",
			},
		})
		return
	}

	c.JSON(http.StatusOK, toCategoryResponse(cat))
}

// Delete handles DELETE /api/v1/categories/:id.
func (h *CategoryHandler) Delete(c *gin.Context) {
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

	catID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "BAD_REQUEST",
				"message": "invalid category id",
			},
		})
		return
	}

	err = h.categoryService.DeleteCategory(c.Request.Context(), catID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "NOT_FOUND",
					"message": "category not found",
				},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "failed to delete category",
			},
		})
		return
	}

	c.Status(http.StatusNoContent)
}
