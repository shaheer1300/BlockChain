// Package api implements the HTTP API server. Handlers in this package
// are thin adapters: parse the request, call service methods, write JSON.
// No consensus logic belongs here.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/shaheer1300/BlockChain/utxo-blockchain-node/internal/config"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 15 * time.Second
	idleTimeout       = 60 * time.Second
)

// Server owns the HTTP listener and all route handlers.
type Server struct {
	cfg  *config.Config
	log  *slog.Logger
	http *http.Server
}

// New constructs a Server and registers all routes. The HTTP listener
// is not started until Start is called.
func New(cfg *config.Config, log *slog.Logger) *Server {
	s := &Server{cfg: cfg, log: log}
	mux := http.NewServeMux()
	s.registerRoutes(mux)
	s.http = &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
	return s
}

// Start begins accepting connections. It returns immediately; the listener
// runs in its own goroutine.
func (s *Server) Start() error {
	s.log.Info("API server starting", "addr", s.cfg.HTTPAddr)
	go func() {
		if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.log.Error("API server error", "err", err)
		}
	}()
	return nil
}

// Shutdown gracefully drains active connections. It blocks until all
// connections are idle or ctx is cancelled.
func (s *Server) Shutdown(ctx context.Context) error {
	s.log.Info("API server shutting down")
	return s.http.Shutdown(ctx)
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", s.handleHealth)
}

// healthResponse is the JSON payload returned by GET /health.
type healthResponse struct {
	Status  string `json:"status"`
	NodeID  string `json:"node_id"`
	Network string `json:"network"`
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		Status:  "ok",
		NodeID:  s.cfg.NodeID,
		Network: s.cfg.NetworkID,
	})
}

// writeJSON sets Content-Type, writes the status code, and encodes v as
// JSON. Any encode error after the header is sent is silently discarded.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
