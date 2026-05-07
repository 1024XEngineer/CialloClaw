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
	"sort"
	"strings"
	"sync"
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

// maxPendingStreamRequests caps per-connection in-flight work so a shared
// named-pipe stream applies backpressure before requests pile up behind task
// serialization locks.
const maxPendingStreamRequests = 32

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

type streamEnvelopeWriter struct {
	encoder *json.Encoder
	writeMu sync.Mutex
}

func (w *streamEnvelopeWriter) writeEnvelope(envelope any) error {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	return w.encoder.Encode(envelope)
}

type streamTaskCoordinator struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newStreamTaskCoordinator() *streamTaskCoordinator {
	return &streamTaskCoordinator{locks: map[string]*sync.Mutex{}}
}

func (c *streamTaskCoordinator) withTaskLocks(taskIDs map[string]bool, fn func()) {
	if c == nil || len(taskIDs) == 0 {
		fn()
		return
	}

	orderedTaskIDs := make([]string, 0, len(taskIDs))
	for taskID := range taskIDs {
		orderedTaskIDs = append(orderedTaskIDs, taskID)
	}
	sort.Strings(orderedTaskIDs)

	locks := make([]*sync.Mutex, 0, len(orderedTaskIDs))
	c.mu.Lock()
	for _, taskID := range orderedTaskIDs {
		lock := c.locks[taskID]
		if lock == nil {
			lock = &sync.Mutex{}
			c.locks[taskID] = lock
		}
		locks = append(locks, lock)
	}
	c.mu.Unlock()

	for _, lock := range locks {
		lock.Lock()
	}
	defer func() {
		for index := len(locks) - 1; index >= 0; index-- {
			locks[index].Unlock()
		}
	}()

	fn()
}

func (s *Server) handleStreamConn(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	writer := &streamEnvelopeWriter{encoder: json.NewEncoder(conn)}
	taskCoordinator := newStreamTaskCoordinator()
	pendingRequests := make(chan struct{}, maxPendingStreamRequests)
	var taskStartRequestMu sync.Mutex

	for {
		var request requestEnvelope
		if err := decoder.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}

			_ = writer.writeEnvelope(newErrorEnvelope(nil, &rpcError{
				Code:    errInvalidParams,
				Message: "INVALID_PARAMS",
				Detail:  "invalid json-rpc payload",
				TraceID: "trace_rpc_decode",
			}))
			return
		}

		pendingRequests <- struct{}{}
		go func(request requestEnvelope) {
			defer func() { <-pendingRequests }()
			s.handleStreamRequest(request, writer, taskCoordinator, &taskStartRequestMu)
		}(request)
	}
}

func (s *Server) handleStreamRequest(request requestEnvelope, writer *streamEnvelopeWriter, taskCoordinator *streamTaskCoordinator, taskStartMu *sync.Mutex) {
	if shouldTrackStartedTask(request.Method) {
		// Task-starting requests on one shared desktop stream must stay serialized.
		// Their runtime-notification correlation temporarily learns task ids from
		// task-start events, and parallel submits on the same connection can
		// otherwise cross-wire notifications before that mapping is established.
		taskStartMu.Lock()
		defer taskStartMu.Unlock()
	}

	streamedRuntimeCounts := map[string]int{}
	var streamedRuntimeMu sync.Mutex
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
	markRuntimeNotificationStreamed := func(key string) {
		streamedRuntimeMu.Lock()
		streamedRuntimeCounts[key]++
		streamedRuntimeMu.Unlock()
	}
	consumeStreamedRuntimeNotification := func(key string) bool {
		streamedRuntimeMu.Lock()
		defer streamedRuntimeMu.Unlock()
		if streamedRuntimeCounts[key] == 0 {
			return false
		}
		streamedRuntimeCounts[key]--
		return true
	}

	taskCoordinator.withTaskLocks(requestTaskIDs, func() {
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
				if err := writer.writeEnvelope(newNotificationEnvelope(method, params)); err == nil {
					markRuntimeNotificationStreamed(notificationKey(method, notificationTaskID, params))
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

		if err := writer.writeEnvelope(response); err != nil {
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
				if isLiveRuntimeMethod(method) && consumeStreamedRuntimeNotification(key) {
					continue
				}
				if err := writer.writeEnvelope(newNotificationEnvelope(method, params)); err != nil {
					return
				}
			}
		}
	})
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
