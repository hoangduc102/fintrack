package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/hoangduc102/fintrack/internal/config"
	"github.com/hoangduc102/fintrack/internal/server"
)

func TestServer_DeterministicGracefulShutdown(t *testing.T) {
	// 1. Bind to an ephemeral port deterministically
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on ephemeral port: %v", err)
	}

	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("failed to split host and port: %v", err)
	}

	cfg := &config.Config{
		Port:        port,
		Environment: "test",
	}

	srv := server.New(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Serve(ctx, ln)
	}()

	// 2. Deterministically verify the server is actively handling requests
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s/health", ln.Addr().String()))
	if err != nil {
		t.Fatalf("failed to perform health check request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status 'ok', got %q", body["status"])
	}

	// 3. Trigger graceful shutdown
	cancel()

	// 4. Assert clean shutdown without timeout or error
	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("expected clean shutdown, got error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for server to shut down cleanly")
	}
}

func TestServer_PortAlreadyInUse(t *testing.T) {
	// Pre-bind a listener to occupy a port
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to bind listener: %v", err)
	}
	defer ln.Close()

	_, port, err := net.SplitHostPort(ln.Addr().String())
	if err != nil {
		t.Fatalf("failed to split host and port: %v", err)
	}

	cfg := &config.Config{
		Port:        port,
		Environment: "test",
	}

	srv := server.New(cfg)

	// Run with a non-cancelled context; Run must return immediately due to port conflict
	errChan := make(chan error, 1)
	go func() {
		errChan <- srv.Run(context.Background())
	}()

	select {
	case err := <-errChan:
		if err == nil {
			t.Fatal("expected error due to port already in use, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out: srv.Run blocked indefinitely instead of returning listener error")
	}
}
