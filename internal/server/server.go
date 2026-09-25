package server

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/hoangduc102/fintrack/internal/auth"
	"github.com/hoangduc102/fintrack/internal/config"
	"github.com/hoangduc102/fintrack/internal/handler"
	"github.com/hoangduc102/fintrack/internal/repository"
	"github.com/hoangduc102/fintrack/internal/service"
)

// Server wraps the HTTP server and configuration.
type Server struct {
	httpServer      *http.Server
	cfg             *config.Config
	db              *sql.DB
	jwtManager      *auth.JWTManager
	authHandler     *handler.AuthHandler
	categoryHandler *handler.CategoryHandler
	txHandler       *handler.TransactionHandler
	budgetHandler   *handler.BudgetHandler
	analyticsHandler *handler.AnalyticsHandler
}

// Option configures Server options.
type Option func(*Server)

// WithDB sets the database pool instance to be closed during graceful shutdown.
func WithDB(db *sql.DB) Option {
	return func(s *Server) {
		s.db = db
	}
}

// WithJWTManager sets the JWTManager instance for token authentication.
func WithJWTManager(jm *auth.JWTManager) Option {
	return func(s *Server) {
		s.jwtManager = jm
	}
}

// WithAuthHandler sets the AuthHandler instance for routing.
func WithAuthHandler(h *handler.AuthHandler) Option {
	return func(s *Server) {
		s.authHandler = h
	}
}

// WithCategoryHandler sets the CategoryHandler instance for routing.
func WithCategoryHandler(h *handler.CategoryHandler) Option {
	return func(s *Server) {
		s.categoryHandler = h
	}
}

// WithTransactionHandler sets the TransactionHandler instance for routing.
func WithTransactionHandler(h *handler.TransactionHandler) Option {
	return func(s *Server) {
		s.txHandler = h
	}
}

// WithBudgetHandler sets the BudgetHandler instance for routing.
func WithBudgetHandler(h *handler.BudgetHandler) Option {
	return func(s *Server) {
		s.budgetHandler = h
	}
}

// WithAnalyticsHandler sets the AnalyticsHandler instance for routing.
func WithAnalyticsHandler(h *handler.AnalyticsHandler) Option {
	return func(s *Server) {
		s.analyticsHandler = h
	}
}

// New creates a configured Server instance.
func New(cfg *config.Config, opts ...Option) *Server {
	s := &Server{
		cfg: cfg,
	}
	for _, opt := range opts {
		opt(s)
	}

	if s.jwtManager == nil {
		s.jwtManager = auth.NewJWTManager(cfg.JWTSecret, 24*time.Hour)
	}

	if s.authHandler == nil && s.db != nil {
		userRepo := repository.NewPostgresUserRepository(s.db)
		authService := service.NewAuthService(userRepo, s.jwtManager)
		s.authHandler = handler.NewAuthHandler(authService)
	}

	if s.categoryHandler == nil && s.db != nil {
		catRepo := repository.NewPostgresCategoryRepository(s.db)
		catService := service.NewCategoryService(catRepo)
		s.categoryHandler = handler.NewCategoryHandler(catService)
	}

	if s.txHandler == nil && s.db != nil {
		txRepo := repository.NewPostgresTransactionRepository(s.db)
		catRepo := repository.NewPostgresCategoryRepository(s.db)
		txService := service.NewTransactionService(txRepo, catRepo)
		s.txHandler = handler.NewTransactionHandler(txService)
	}

	if s.budgetHandler == nil && s.db != nil {
		budgetRepo := repository.NewPostgresBudgetRepository(s.db)
		catRepo := repository.NewPostgresCategoryRepository(s.db)
		budgetService := service.NewBudgetService(budgetRepo, catRepo)
		s.budgetHandler = handler.NewBudgetHandler(budgetService)
	}

	if s.analyticsHandler == nil && s.db != nil {
		analyticsRepo := repository.NewPostgresAnalyticsRepository(s.db)
		analyticsService := service.NewAnalyticsService(analyticsRepo)
		s.analyticsHandler = handler.NewAnalyticsHandler(analyticsService)
	}

	router := NewRouter(cfg, s.authHandler, s.categoryHandler, s.txHandler, s.budgetHandler, s.analyticsHandler, s.jwtManager)
	s.httpServer = &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	return s
}

// Serve starts accepting connections on the given net.Listener and handles graceful shutdown
// when ctx is cancelled. If the server fails while serving, it returns that error immediately.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	errChan := make(chan error, 1)

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
		close(errChan)
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var shutdownErr error
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			shutdownErr = err
		}

		if s.db != nil {
			if err := s.db.Close(); err != nil && shutdownErr == nil {
				shutdownErr = err
			}
		}

		if shutdownErr != nil {
			return shutdownErr
		}

		return <-errChan
	}
}

// Run starts the HTTP server using ListenAndServe and handles graceful shutdown when ctx is cancelled.
// If the server fails to start (e.g. port already in use), the error is returned immediately.
func (s *Server) Run(ctx context.Context) error {
	errChan := make(chan error, 1)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- err
		}
		close(errChan)
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var shutdownErr error
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			shutdownErr = err
		}

		if s.db != nil {
			if err := s.db.Close(); err != nil && shutdownErr == nil {
				shutdownErr = err
			}
		}

		if shutdownErr != nil {
			return shutdownErr
		}

		return <-errChan
	}
}
