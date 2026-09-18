package server

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/hoangduc102/fintrack/internal/config"
)

// Server wraps the HTTP server and configuration.
type Server struct {
	httpServer *http.Server
	cfg        *config.Config
}

// New creates a configured Server instance.
func New(cfg *config.Config) *Server {
	router := NewRouter(cfg)
	return &Server{
		httpServer: &http.Server{
			Addr:    ":" + cfg.Port,
			Handler: router,
		},
		cfg: cfg,
	}
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

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return err
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

		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}

		return <-errChan
	}
}
