package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type nonFlusherResponseWriter struct {
	header http.Header
	body   strings.Builder
	status int
}

func (w *nonFlusherResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *nonFlusherResponseWriter) Write(value []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.body.Write(value)
}

func (w *nonFlusherResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
}

type flushRecorder struct {
	*httptest.ResponseRecorder
	onFlush func()
}

func (r *flushRecorder) Flush() {
	if r.onFlush != nil {
		r.onFlush()
	}
}

func TestHandleStreamConnServesJSONRPCSuccess(t *testing.T) {
	server := newTestServer()
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	go server.handleStreamConn(left)

	request := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-stream-success"`),
		Method:  "agent.settings.get",
		Params:  mustMarshal(t, map[string]any{"scope": "all"}),
	}
	if err := json.NewEncoder(right).Encode(request); err != nil {
		t.Fatalf("encode stream request: %v", err)
	}

	var response successEnvelope
	if err := json.NewDecoder(right).Decode(&response); err != nil {
		t.Fatalf("decode stream response: %v", err)
	}
	if string(response.ID) != `"req-stream-success"` || response.Result.Meta.ServerTime == "" {
		t.Fatalf("expected stream success envelope with request id and server time, got %+v", response)
	}
	if response.Result.Data == nil {
		t.Fatalf("expected settings payload in stream response, got %+v", response)
	}
}

func TestHandleStreamConnReturnsDecodeErrorForMalformedPayload(t *testing.T) {
	server := newTestServer()
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	go server.handleStreamConn(left)

	if _, err := right.Write([]byte("{bad json\n")); err != nil {
		t.Fatalf("write malformed stream payload: %v", err)
	}

	var response errorEnvelope
	if err := json.NewDecoder(right).Decode(&response); err != nil {
		t.Fatalf("decode stream error response: %v", err)
	}
	if response.Error.Code != errInvalidParams || response.Error.Message != "INVALID_PARAMS" {
		t.Fatalf("expected invalid params error envelope, got %+v", response)
	}
	if response.Error.Data.TraceID != "trace_rpc_decode" {
		t.Fatalf("expected decode trace id, got %+v", response.Error.Data)
	}
}

func TestHandleHealthzSupportsCORSAndOptions(t *testing.T) {
	server := newTestServer()

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	request.Header.Set("Origin", "http://localhost:1420")
	recorder := httptest.NewRecorder()
	server.handleHealthz(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected healthz to return 200, got %d", recorder.Code)
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") != "http://localhost:1420" {
		t.Fatalf("expected localhost origin to be allowed, headers=%v", recorder.Header())
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode healthz response: %v", err)
	}
	if payload["status"] != "ok" || payload["service"] != "local-service" {
		t.Fatalf("expected healthz payload, got %+v", payload)
	}

	optionsRecorder := httptest.NewRecorder()
	server.handleHealthz(optionsRecorder, httptest.NewRequest(http.MethodOptions, "/healthz", nil))
	if optionsRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected options healthz to return 204, got %d", optionsRecorder.Code)
	}
}

func TestHandleHTTPRPCCoversMethodDecodeAndSuccess(t *testing.T) {
	server := newTestServer()

	optionsRecorder := httptest.NewRecorder()
	server.handleHTTPRPC(optionsRecorder, httptest.NewRequest(http.MethodOptions, "/rpc", nil))
	if optionsRecorder.Code != http.StatusNoContent {
		t.Fatalf("expected rpc options to return 204, got %d", optionsRecorder.Code)
	}

	methodRecorder := httptest.NewRecorder()
	server.handleHTTPRPC(methodRecorder, httptest.NewRequest(http.MethodGet, "/rpc", nil))
	if methodRecorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected rpc get to return 405, got %d", methodRecorder.Code)
	}

	decodeRecorder := httptest.NewRecorder()
	decodeRequest := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"agent.settings.get","params":`))
	server.handleHTTPRPC(decodeRecorder, decodeRequest)
	if decodeRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected malformed rpc body to return 400, got %d", decodeRecorder.Code)
	}

	successRecorder := httptest.NewRecorder()
	successRequest := httptest.NewRequest(http.MethodPost, "/rpc", strings.NewReader(`{"jsonrpc":"2.0","id":"req-http-rpc","method":"agent.settings.get","params":{"scope":"all"}}`))
	server.handleHTTPRPC(successRecorder, successRequest)
	if successRecorder.Code != http.StatusOK {
		t.Fatalf("expected rpc post to return 200, got %d", successRecorder.Code)
	}
	var envelope successEnvelope
	if err := json.Unmarshal(successRecorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode success envelope: %v", err)
	}
	if envelope.Result.Data == nil || envelope.Result.Meta.ServerTime == "" {
		t.Fatalf("expected rpc success envelope to include data and meta, got %+v", envelope)
	}
}

func TestHandleDebugEventsCoversValidationAndSuccess(t *testing.T) {
	server := newTestServer()

	missingRecorder := httptest.NewRecorder()
	server.handleDebugEvents(missingRecorder, httptest.NewRequest(http.MethodGet, "/events", nil))
	if missingRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected missing task_id to return 400, got %d", missingRecorder.Code)
	}

	notFoundRecorder := httptest.NewRecorder()
	server.handleDebugEvents(notFoundRecorder, httptest.NewRequest(http.MethodGet, "/events?task_id=missing", nil))
	if notFoundRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected unknown task_id to return 404, got %d", notFoundRecorder.Code)
	}

	startResult, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_debug_events",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "review my notes",
		},
	})
	if err != nil {
		t.Fatalf("seed task.start: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)

	successRecorder := httptest.NewRecorder()
	server.handleDebugEvents(successRecorder, httptest.NewRequest(http.MethodGet, "/events?task_id="+taskID, nil))
	if successRecorder.Code != http.StatusOK {
		t.Fatalf("expected debug events to return 200, got %d", successRecorder.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(successRecorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode debug events response: %v", err)
	}
	items, ok := payload["items"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("expected buffered debug events, got %+v", payload)
	}
}

func TestHandleDebugEventStreamCoversValidationSuccessAndError(t *testing.T) {
	server := newTestServer()

	missingRecorder := httptest.NewRecorder()
	server.handleDebugEventStream(missingRecorder, httptest.NewRequest(http.MethodGet, "/events/stream", nil))
	if missingRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected missing task_id to return 400, got %d", missingRecorder.Code)
	}

	nonFlusher := &nonFlusherResponseWriter{}
	server.handleDebugEventStream(nonFlusher, httptest.NewRequest(http.MethodGet, "/events/stream?task_id=task_001", nil))
	if nonFlusher.status != http.StatusInternalServerError {
		t.Fatalf("expected non-flusher response writer to return 500, got %d", nonFlusher.status)
	}

	errorCtx, errorCancel := context.WithCancel(context.Background())
	defer errorCancel()
	errorRecorder := &flushRecorder{ResponseRecorder: httptest.NewRecorder(), onFlush: errorCancel}
	errorRequest := httptest.NewRequest(http.MethodGet, "/events/stream?task_id=missing", nil).WithContext(errorCtx)
	errorDone := make(chan struct{})
	go func() {
		server.handleDebugEventStream(errorRecorder, errorRequest)
		close(errorDone)
	}()
	select {
	case <-errorDone:
	case <-time.After(2 * time.Second):
		t.Fatal("expected stream error path to exit after flushing error event")
	}
	if !strings.Contains(errorRecorder.Body.String(), "event: error") {
		t.Fatalf("expected SSE error event, got %q", errorRecorder.Body.String())
	}

	startResult, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_stream_success",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "stream my notifications",
		},
	})
	if err != nil {
		t.Fatalf("seed task.start: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	notifications, err := server.orchestrator.PendingNotifications(taskID)
	if err != nil || len(notifications) == 0 {
		t.Fatalf("expected task start to enqueue notifications, err=%v notifications=%+v", err, notifications)
	}

	successCtx, successCancel := context.WithCancel(context.Background())
	defer successCancel()
	successRecorder := &flushRecorder{ResponseRecorder: httptest.NewRecorder(), onFlush: successCancel}
	successRequest := httptest.NewRequest(http.MethodGet, "/events/stream?task_id="+taskID, nil).WithContext(successCtx)
	successDone := make(chan struct{})
	go func() {
		server.handleDebugEventStream(successRecorder, successRequest)
		close(successDone)
	}()
	select {
	case <-successDone:
	case <-time.After(2 * time.Second):
		t.Fatal("expected stream success path to exit after first flushed event")
	}
	if !strings.Contains(successRecorder.Body.String(), "event: ") || !strings.Contains(successRecorder.Body.String(), "data: ") {
		t.Fatalf("expected SSE success payload, got %q", successRecorder.Body.String())
	}
}

func TestServerStartHandlesShutdownAndImmediateListenErrors(t *testing.T) {
	server := newTestServer()
	server.transport = ""
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("expected graceful start shutdown path, got %v", err)
	}

	errorServer := newTestServer()
	errorServer.transport = ""
	errorServer.debugHTTPServer.Addr = "bad::addr"
	errorCtx, errorCancel := context.WithCancel(context.Background())
	defer errorCancel()
	if err := errorServer.Start(errorCtx); err == nil {
		t.Fatal("expected invalid listen address to surface start error")
	}
}

func TestTransportSupervisorCancelsShutdownAndJoinsWorkers(t *testing.T) {
	transportErr := errors.New("listen failed")
	supervisor := newTransportSupervisor(context.Background(), 2)
	workerExited := make(chan struct{})

	supervisor.Go(func(ctx context.Context) error {
		<-ctx.Done()
		close(workerExited)
		return nil
	})
	supervisor.Go(func(context.Context) error {
		return transportErr
	})

	shutdownCalled := false
	err := supervisor.Wait(func() error {
		shutdownCalled = true
		return nil
	})
	if !errors.Is(err, transportErr) {
		t.Fatalf("expected transport error to win, got %v", err)
	}
	if !shutdownCalled {
		t.Fatal("expected shutdown to run before Wait returns")
	}
	select {
	case <-workerExited:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected sibling worker to exit before Wait returns")
	}
}

func TestServerHelperUtilitiesCoverFallbackBranches(t *testing.T) {
	server := newTestServer()
	recorder := httptest.NewRecorder()

	setDebugCORSOrigin(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if recorder.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected empty origin to be ignored, headers=%v", recorder.Header())
	}

	invalidOriginRequest := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	invalidOriginRequest.Header.Set("Origin", "://bad-origin")
	setDebugCORSOrigin(recorder, invalidOriginRequest)
	if recorder.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected invalid origin to be ignored, headers=%v", recorder.Header())
	}

	remoteOriginRequest := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	remoteOriginRequest.Header.Set("Origin", "https://example.com")
	setDebugCORSOrigin(recorder, remoteOriginRequest)
	if recorder.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected remote origin to be ignored, headers=%v", recorder.Header())
	}

	if traceIDFromRequest(json.RawMessage(`{"broken":`)) != "trace_rpc_unknown" {
		t.Fatal("expected invalid trace payload to fall back to unknown trace id")
	}
	if runtimeNotificationTaskID("task_direct", nil) != "task_direct" {
		t.Fatal("expected explicit runtime task id to win")
	}
	if runtimeNotificationTaskID("", map[string]any{"task_id": " task_from_params "}) != "task_from_params" {
		t.Fatal("expected runtime task id to fall back to params")
	}
	if notificationKey("loop.round", "task_001", map[string]any{"bad": make(chan int)}) != "loop.round" {
		t.Fatal("expected notificationKey marshal failure to fall back to method")
	}
	if marshalSSEData(map[string]any{"ok": true}) != `{"ok":true}` {
		t.Fatal("expected marshalSSEData to encode json payload")
	}
	if marshalSSEData(make(chan int)) != `{}` {
		t.Fatal("expected marshalSSEData to fall back for invalid payloads")
	}
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("expected shutdown after graceful stop to be idempotent, got %v", err)
	}
}
