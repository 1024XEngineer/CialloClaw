package rpc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// handleHealthz exposes a debug health snapshot without mutating runtime state.
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeDebugCORSHeaders(w)
	setDebugCORSOrigin(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":       "ok",
		"service":      "local-service",
		"transport":    s.transport,
		"named_pipe":   s.namedPipeName,
		"orchestrator": s.orchestrator.Snapshot(),
	})
}

// handleHTTPRPC serves debug JSON-RPC requests through the same dispatch path
// used by stream transports.
func (s *Server) handleHTTPRPC(w http.ResponseWriter, r *http.Request) {
	writeDebugCORSHeaders(w)
	setDebugCORSOrigin(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	defer r.Body.Close()

	request, rpcErr := decodeRequest(r.Body)
	if rpcErr != nil {
		w.Header().Set("content-type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(newErrorEnvelope(nil, rpcErr))
		return
	}

	response := s.dispatch(request)
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(response)
}

// handleDebugEvents returns buffered notifications for one task without
// draining the orchestrator queue.
func (s *Server) handleDebugEvents(w http.ResponseWriter, r *http.Request) {
	writeDebugCORSHeaders(w)
	setDebugCORSOrigin(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "task_id is required"})
		return
	}

	events, err := s.orchestrator.PendingNotifications(taskID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}

	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"task_id": taskID,
		"items":   events,
	})
}

// handleDebugEventStream drains task notifications into an SSE stream for
// local debug consumers.
func (s *Server) handleDebugEventStream(w http.ResponseWriter, r *http.Request) {
	writeDebugCORSHeaders(w)
	setDebugCORSOrigin(w, r)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "task_id is required"})
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "streaming is not supported by this response writer"})
		return
	}

	w.Header().Set("content-type", "text/event-stream")
	w.Header().Set("cache-control", "no-cache")
	w.Header().Set("connection", "keep-alive")

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			notifications, err := s.orchestrator.DrainNotifications(taskID)
			if err != nil {
				_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", marshalSSEData(map[string]any{"error": err.Error()}))
				flusher.Flush()
				return
			}

			for _, notification := range notifications {
				method := stringValue(notification, "method", "task.updated")
				params := mapValue(notification, "params")
				_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", method, marshalSSEData(params))
				flusher.Flush()
			}
		}
	}
}

func writeDebugCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func setDebugCORSOrigin(w http.ResponseWriter, r *http.Request) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return
	}

	parsed, err := url.Parse(origin)
	if err != nil {
		return
	}

	host := strings.ToLower(parsed.Hostname())
	if !isAllowedDebugOriginHost(host) {
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
}

func isAllowedDebugOriginHost(host string) bool {
	normalized := strings.TrimSpace(strings.ToLower(host))
	switch normalized {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return strings.HasSuffix(normalized, ".localhost")
	}
}

// marshalSSEData encodes arbitrary debug payloads into one SSE data field.
func marshalSSEData(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return `{}`
	}
	return string(encoded)
}
