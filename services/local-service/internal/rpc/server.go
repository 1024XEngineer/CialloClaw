// Package rpc hosts the local JSON-RPC server and debug transports.
package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"
)

// Server is the transport entrypoint for local-service.
// It accepts debug HTTP, named-pipe streams, and dispatches stable JSON-RPC
// methods into the orchestrator.
type Server struct {
	transport       string
	namedPipeName   string
	debugHTTPServer *http.Server
	handlers        map[string]methodHandler
	orchestrator    *orchestrator.Service
	now             func() time.Time
}

// NewServer constructs the RPC server and registers debug endpoints.
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

// Start serves every transport enabled by the current config.
// During P0 it intentionally keeps both debug HTTP and named pipe available for
// local integration work.
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

// Shutdown closes the debug HTTP server when it was started.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.debugHTTPServer == nil {
		return nil
	}

	if err := s.debugHTTPServer.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// handleHealthz returns a minimal health snapshot plus orchestrator state.
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

// handleHTTPRPC serves debug-time HTTP JSON-RPC requests.
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

// handleDebugEvents returns buffered notifications for a task.
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

// handleDebugEventStream polls and emits task notifications over SSE for debug
// consumers.
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
	if host != "localhost" && host != "127.0.0.1" {
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
}

// handleStreamConn serves JSON-RPC requests on a long-lived stream and then
// replays buffered notifications on the same connection.
func (s *Server) handleStreamConn(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)
	var writeMu sync.Mutex

	for {
		var request requestEnvelope
		if err := decoder.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}

			writeMu.Lock()
			_ = encoder.Encode(newErrorEnvelope(nil, &rpcError{
				Code:    errInvalidParams,
				Message: "INVALID_PARAMS",
				Detail:  "invalid json-rpc payload",
				TraceID: "trace_rpc_decode",
			}))
			writeMu.Unlock()
			return
		}

		// Each request is handled in its own goroutine so a long-running task
		// confirmation or input-submit flow cannot block unrelated control-panel or
		// dashboard reads that share the same desktop pipe connection.
		go s.handleStreamRequest(request, encoder, &writeMu)
	}

}

// handleStreamRequest owns one decoded request lifecycle on a shared stream.
// Responses may complete out of order, but JSON-RPC ids keep the desktop host
// correlation stable while writeMu preserves frame boundaries on the pipe.
func (s *Server) handleStreamRequest(
	request requestEnvelope,
	encoder *json.Encoder,
	writeMu *sync.Mutex,
) {
	streamedRuntimeCounts := map[string]int{}
	var streamedRuntimeCountsMu sync.Mutex
	requestTaskIDs, requestSessionID, requestTraceID := requestRoutingHints(request)
	var requestTaskMu sync.RWMutex
	addRequestTaskID := func(taskID string) {
		trimmed := strings.TrimSpace(taskID)
		if trimmed == "" {
			return
		}
		requestTaskMu.Lock()
		if requestTaskIDs == nil {
			requestTaskIDs = map[string]bool{}
		}
		requestTaskIDs[trimmed] = true
		requestTaskMu.Unlock()
	}
	hasRequestTaskID := func(taskID string) bool {
		requestTaskMu.RLock()
		defer requestTaskMu.RUnlock()
		return requestTaskIDs != nil && requestTaskIDs[taskID]
	}
	matchesTaskStart := func(sessionID, traceID string) bool {
		switch {
		case requestTraceID != "":
			return requestTraceID == traceID
		case requestSessionID != "":
			return requestSessionID == sessionID
		default:
			return false
		}
	}

	unsubscribeRuntime := func() {}
	if requestTaskIDs != nil || shouldTrackStartedTask(request.Method) {
		unsubscribeRuntime = s.orchestrator.SubscribeRuntimeNotifications(func(taskID string, method string, params map[string]any) {
			if !isLiveRuntimeMethod(method) {
				return
			}
			notificationTaskID := runtimeNotificationTaskID(taskID, params)
			if notificationTaskID == "" || !hasRequestTaskID(notificationTaskID) {
				return
			}
			writeMu.Lock()
			defer writeMu.Unlock()
			if err := encoder.Encode(newNotificationEnvelope(method, params)); err == nil {
				streamedRuntimeCountsMu.Lock()
				streamedRuntimeCounts[notificationKey(method, notificationTaskID, params)]++
				streamedRuntimeCountsMu.Unlock()
			}
		})
	}

	unsubscribeTaskStart := func() {}
	if shouldTrackStartedTask(request.Method) {
		unsubscribeTaskStart = s.orchestrator.SubscribeTaskStarts(func(taskID, sessionID, traceID string) {
			if !matchesTaskStart(sessionID, traceID) {
				return
			}
			addRequestTaskID(taskID)
		})
	}

	response := s.dispatch(request)
	unsubscribeTaskStart()
	unsubscribeRuntime()

	writeMu.Lock()
	err := encoder.Encode(response)
	writeMu.Unlock()
	if err != nil {
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
			key := notificationKey(method, taskID, params)
			streamedRuntimeCountsMu.Lock()
			alreadyStreamed := isLiveRuntimeMethod(method) && streamedRuntimeCounts[key] > 0
			if alreadyStreamed {
				streamedRuntimeCounts[key]--
			}
			streamedRuntimeCountsMu.Unlock()
			if alreadyStreamed {
				continue
			}
			writeMu.Lock()
			err := encoder.Encode(newNotificationEnvelope(method, params))
			writeMu.Unlock()
			if err != nil {
				return
			}
		}
	}
}

// dispatch is the single RPC dispatch path that validates protocol shape,
// resolves handlers, decodes params, and rewraps orchestrator output.
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

// nowRFC3339 returns the unified response timestamp format.
func (s *Server) nowRFC3339() string {
	return s.now().Format(time.RFC3339)
}

// taskIDsFromResponse recursively collects task_id values from a success
// response so the transport can replay related notifications afterward.
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

func requestRoutingHints(request requestEnvelope) (map[string]bool, string, string) {
	params, rpcErr := decodeParams(request.Params)
	if rpcErr != nil {
		return nil, "", ""
	}

	ids := map[string]struct{}{}
	collectTaskIDs(params, ids)
	var result map[string]bool
	if len(ids) > 0 {
		result = make(map[string]bool, len(ids))
		for taskID := range ids {
			result[taskID] = true
		}
	}
	return result, stringValue(params, "session_id", ""), stringValue(mapValue(params, "request_meta"), "trace_id", "")
}

func shouldTrackStartedTask(method string) bool {
	return method == "agent.task.start" || method == "agent.input.submit"
}

func isLiveRuntimeMethod(method string) bool {
	return strings.HasPrefix(method, "loop.") || method == "task.steered"
}

func runtimeNotificationTaskID(taskID string, params map[string]any) string {
	if strings.TrimSpace(taskID) != "" {
		return taskID
	}
	if params == nil {
		return ""
	}
	rawTaskID, _ := params["task_id"].(string)
	return strings.TrimSpace(rawTaskID)
}

func notificationKey(method, taskID string, params map[string]any) string {
	encoded, err := json.Marshal(normalizeNotificationKey(method, taskID, params))
	if err != nil {
		return method
	}
	return method + ":" + string(encoded)
}

func normalizeNotificationKey(method, taskID string, params map[string]any) map[string]any {
	if !isLiveRuntimeMethod(method) {
		return map[string]any{
			"task_id": strings.TrimSpace(taskID),
			"params":  params,
		}
	}

	normalizedTaskID := strings.TrimSpace(taskID)
	if normalizedTaskID == "" {
		normalizedTaskID = runtimeNotificationTaskID("", params)
	}

	payload := map[string]any{}
	if event := mapValue(params, "event"); len(event) > 0 {
		payload = mapValue(event, "payload")
	} else {
		for key, value := range params {
			if key == "task_id" {
				continue
			}
			payload[key] = value
		}
	}

	return map[string]any{
		"task_id": normalizedTaskID,
		"type":    method,
		"payload": payload,
	}
}

// collectTaskIDs walks arbitrary decoded response payloads and gathers every
// embedded task_id.
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

// marshalSSEData encodes arbitrary debug payloads into an SSE data field.
func marshalSSEData(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return `{}`
	}
	return string(encoded)
}
