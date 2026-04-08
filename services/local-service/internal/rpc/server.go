// 该文件负责 JSON-RPC 服务端、调试 HTTP 和事件流入口。
package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"
)

type Server struct {
	transport       string
	namedPipeName   string
	debugHTTPServer *http.Server
	handlers        map[string]methodHandler
	orchestrator    *orchestrator.Service
	now             func() time.Time
}

func NewServer(cfg serviceconfig.RPCConfig, orchestrator *orchestrator.Service) *Server {
	server := &Server{
		transport:     cfg.Transport,
		namedPipeName: cfg.NamedPipeName,
		orchestrator:  orchestrator,
		now:           time.Now,
	}

	server.registerHandlers()

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", server.handleHealthz)
	mux.HandleFunc("/rpc", server.handleHTTPRPC)
	mux.HandleFunc("/events", server.handleDebugEvents)
	mux.HandleFunc("/events/stream", server.handleDebugEventStream)

	server.debugHTTPServer = &http.Server{
		Addr:              cfg.DebugHTTPAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return server
}

func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 2)

	if s.debugHTTPServer != nil {
		go func() {
			err := s.debugHTTPServer.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- err
			}
		}()
	}

	if s.transport == "named_pipe" {
		go func() {
			err := serveNamedPipe(ctx, s.namedPipeName, s.handleStreamConn)
			if err != nil && !errors.Is(err, errNamedPipeUnsupported) && ctx.Err() == nil {
				errCh <- err
			}
		}()
	}

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.debugHTTPServer == nil {
		return nil
	}

	if err := s.debugHTTPServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":       "ok",
		"service":      "local-service",
		"transport":    s.transport,
		"named_pipe":   s.namedPipeName,
		"orchestrator": s.orchestrator.Snapshot(),
	})
}

func (s *Server) handleHTTPRPC(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleDebugEvents(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleDebugEventStream(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleStreamConn(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	for {
		var request requestEnvelope
		if err := decoder.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}

			_ = encoder.Encode(newErrorEnvelope(nil, &rpcError{
				Code:    errInvalidParams,
				Message: "INVALID_PARAMS",
				Detail:  "invalid json-rpc payload",
				TraceID: "trace_rpc_decode",
			}))
			return
		}

		response := s.dispatch(request)
		if err := encoder.Encode(response); err != nil {
			return
		}

		for _, taskID := range taskIDsFromResponse(response) {
			notifications, err := s.orchestrator.DrainNotifications(taskID)
			if err != nil {
				continue
			}

			for _, notification := range notifications {
				method := stringValue(notification, "method", "task.updated")
				params := mapValue(notification, "params")
				if err := encoder.Encode(newNotificationEnvelope(method, params)); err != nil {
					return
				}
			}
		}
	}
}

func (s *Server) dispatch(request requestEnvelope) any {
	if request.JSONRPC != "2.0" {
		return newErrorEnvelope(request.ID, &rpcError{
			Code:    errInvalidParams,
			Message: "INVALID_PARAMS",
			Detail:  "jsonrpc version must be 2.0",
			TraceID: "trace_rpc_version",
		})
	}

	handler, ok := s.handlers[request.Method]
	if !ok {
		return newErrorEnvelope(request.ID, &rpcError{
			Code:    errMethodNotFound,
			Message: "JSON_RPC_METHOD_NOT_FOUND",
			Detail:  "method is not registered in the stable stub router",
			TraceID: traceIDFromRequest(request.Params),
		})
	}

	params, rpcErr := decodeParams(request.Params)
	if rpcErr != nil {
		return newErrorEnvelope(request.ID, rpcErr)
	}

	data, handlerErr := handler(params)
	if handlerErr != nil {
		return newErrorEnvelope(request.ID, handlerErr)
	}

	return newSuccessEnvelope(request.ID, data, s.nowRFC3339())
}

func (s *Server) nowRFC3339() string {
	return s.now().Format(time.RFC3339)
}

func taskIDsFromResponse(response any) []string {
	success, ok := response.(successEnvelope)
	if !ok {
		return nil
	}

	ids := map[string]struct{}{}
	collectTaskIDs(success.Result.Data, ids)

	result := make([]string, 0, len(ids))
	for taskID := range ids {
		result = append(result, taskID)
	}

	return result
}

func collectTaskIDs(rawValue any, ids map[string]struct{}) {
	switch value := rawValue.(type) {
	case map[string]any:
		for key, item := range value {
			if strings.HasSuffix(key, "task_id") {
				if taskID, ok := item.(string); ok && taskID != "" {
					ids[taskID] = struct{}{}
				}
			}
			collectTaskIDs(item, ids)
		}
	case []map[string]any:
		for _, item := range value {
			collectTaskIDs(item, ids)
		}
	case []any:
		for _, item := range value {
			collectTaskIDs(item, ids)
		}
	}
}

func marshalSSEData(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return `{}`
	}
	return string(encoded)
}
