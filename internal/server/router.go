package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hoangduc102/fintrack/internal/config"
	"github.com/hoangduc102/fintrack/internal/handler"
)

// NewRouter initializes and configures a Gin router based on the provided Config and optional handlers.
func NewRouter(cfg *config.Config, authHandlers ...*handler.AuthHandler) *gin.Engine {
	switch cfg.Environment {
	case "production":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()
	router.Use(gin.Recovery())

	// Avoid polluting test output with request access logs
	if cfg.Environment != "test" {
		router.Use(gin.Logger())
	}

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
		})
	})

	var authHandler *handler.AuthHandler
	if len(authHandlers) > 0 && authHandlers[0] != nil {
		authHandler = authHandlers[0]
	}

	if authHandler != nil {
		v1 := router.Group("/api/v1")
		{
			auth := v1.Group("/auth")
			{
				auth.POST("/register", authHandler.Register)
			}
		}
	}

	return router
}
