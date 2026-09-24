package handler

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hoangduc102/fintrack/internal/domain"
	"github.com/hoangduc102/fintrack/internal/service"
)

// AuthHandler handles HTTP requests for user authentication and registration.
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// RegisterRequest defines the input payload for POST /api/v1/auth/register.
type RegisterRequest struct {
	Email       string  `json:"email"`
	Password    string  `json:"password"`
	DisplayName *string `json:"display_name"`
}

// UserResponse defines the flat JSON response for a registered user.
type UserResponse struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	DisplayName *string `json:"display_name,omitempty"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// Register handles user registration.
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "BAD_REQUEST",
				"message": "invalid request body",
			},
		})
		return
	}

	user, err := h.authService.Register(c.Request.Context(), service.RegisterInput{
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		var valErr domain.ValidationError
		if errors.As(err, &valErr) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": gin.H{
					"code":    "BAD_REQUEST",
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
					"message": "email already exists",
				},
			})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "failed to register user",
			},
		})
		return
	}

	c.JSON(http.StatusCreated, UserResponse{
		ID:          user.ID.String(),
		Email:       user.Email,
		DisplayName: user.DisplayName,
		CreatedAt:   user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   user.UpdatedAt.Format(time.RFC3339),
	})
}
