// Package health provides HTTP health check endpoints.
package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Status represents the health check response.
type Status struct {
	Status    string            `json:"status"`
	Checks    map[string]Check  `json:"checks"`
	Version   string            `json:"version,omitempty"`
	Timestamp string            `json:"timestamp"`
}

// Check represents an individual health check.
type Check struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

// CheckFunc is a function that performs a health check.
type CheckFunc func(ctx context.Context) (bool, string)

// Server provides health check HTTP endpoints.
type Server struct {
	port    int
	version string
	checks  map[string]CheckFunc
	mu      sync.RWMutex
	server  *http.Server
}

// NewServer creates a new health check server.
func NewServer(port int, version string) *Server {
	return &Server{
		port:    port,
		version: version,
		checks:  make(map[string]CheckFunc),
	}
}

// RegisterCheck registers a health check function.
func (s *Server) RegisterCheck(name string, check CheckFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checks[name] = check
}

// Start starts the health check server.
func (s *Server) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/ready", s.handleReady)
	mux.HandleFunc("/live", s.handleLive)

	s.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Log error but don't crash - health endpoint is optional
		}
	}()

	return nil
}

// Stop gracefully stops the health check server.
func (s *Server) Stop(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// handleHealth returns full health status with all checks.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	s.mu.RLock()
	checks := make(map[string]CheckFunc, len(s.checks))
	for k, v := range s.checks {
		checks[k] = v
	}
	s.mu.RUnlock()

	status := Status{
		Status:    "ok",
		Checks:    make(map[string]Check),
		Version:   s.version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	allHealthy := true
	for name, check := range checks {
		healthy, msg := check(ctx)
		status.Checks[name] = Check{
			Healthy: healthy,
			Message: msg,
		}
		if !healthy {
			allHealthy = false
		}
	}

	if !allHealthy {
		status.Status = "degraded"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleReady returns whether the service is ready to receive traffic.
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	s.mu.RLock()
	checks := make(map[string]CheckFunc, len(s.checks))
	for k, v := range s.checks {
		checks[k] = v
	}
	s.mu.RUnlock()

	for _, check := range checks {
		if healthy, _ := check(ctx); !healthy {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("not ready"))
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ready"))
}

// handleLive returns whether the service is alive (simple liveness probe).
func (s *Server) handleLive(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("alive"))
}
