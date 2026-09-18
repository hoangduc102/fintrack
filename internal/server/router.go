package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hoangduc102/fintrack/internal/config"
)

// NewRouter initializes and configures a Gin router based on the provided Config.
func NewRouter(cfg *config.Config) *gin.Engine {
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

	return router
}
