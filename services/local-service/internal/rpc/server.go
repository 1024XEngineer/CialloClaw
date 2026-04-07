package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"
)

type Server struct {
	address    string
	httpServer *http.Server
}

func NewServer(address string, orchestrator *orchestrator.Service) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":       "ok",
			"service":      "local-service",
			"orchestrator": orchestrator.Snapshot(),
		})
	})
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      "bootstrap",
			"error": map[string]any{
				"code":    1002005,
				"message": "task-centric json-rpc scaffold not wired yet",
				"data": map[string]any{
					"type":     "JSON_RPC_METHOD_NOT_FOUND",
					"trace_id": "trace_bootstrap_rpc",
				},
			},
		})
	})

	return &Server{
		address: address,
		httpServer: &http.Server{
			Addr:              address,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		},
	}
}

func (s *Server) Start() error {
	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}

	return err
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
