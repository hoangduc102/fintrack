package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hoangduc102/fintrack/internal/auth"
	"github.com/hoangduc102/fintrack/internal/config"
	"github.com/hoangduc102/fintrack/internal/handler"
)

// NewRouter initializes and configures a Gin router based on the provided Config and optional dependencies.
func NewRouter(cfg *config.Config, deps ...interface{}) *gin.Engine {
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
	var categoryHandler *handler.CategoryHandler
	var txHandler *handler.TransactionHandler
	var budgetHandler *handler.BudgetHandler
	var jwtManager *auth.JWTManager

	for _, dep := range deps {
		switch d := dep.(type) {
		case *handler.AuthHandler:
			authHandler = d
		case *handler.CategoryHandler:
			categoryHandler = d
		case *handler.TransactionHandler:
			txHandler = d
		case *handler.BudgetHandler:
			budgetHandler = d
		case *auth.JWTManager:
			jwtManager = d
		}
	}

	v1 := router.Group("/api/v1")
	{
		if authHandler != nil {
			authGroup := v1.Group("/auth")
			{
				authGroup.POST("/register", authHandler.Register)
				authGroup.POST("/login", authHandler.Login)

				if jwtManager != nil {
					authGroup.GET("/me", handler.AuthMiddleware(jwtManager), authHandler.Me)
				}
			}
		}

		if categoryHandler != nil && jwtManager != nil {
			catGroup := v1.Group("/categories", handler.AuthMiddleware(jwtManager))
			{
				catGroup.GET("", categoryHandler.List)
				catGroup.POST("", categoryHandler.Create)
				catGroup.PUT("/:id", categoryHandler.Update)
				catGroup.DELETE("/:id", categoryHandler.Delete)
			}
		}

		if txHandler != nil && jwtManager != nil {
			txGroup := v1.Group("/transactions", handler.AuthMiddleware(jwtManager))
			{
				txGroup.POST("", txHandler.Create)
				txGroup.GET("", txHandler.List)
				txGroup.GET("/:id", txHandler.GetByID)
				txGroup.PUT("/:id", txHandler.Update)
				txGroup.DELETE("/:id", txHandler.Delete)
			}
		}

		if budgetHandler != nil && jwtManager != nil {
			budgetGroup := v1.Group("/budgets", handler.AuthMiddleware(jwtManager))
			{
				budgetGroup.PUT("", budgetHandler.Upsert)
				budgetGroup.GET("", budgetHandler.List)
				budgetGroup.DELETE("/:id", budgetHandler.Delete)
			}
		}
	}

	return router
}
