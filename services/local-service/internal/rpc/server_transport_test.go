package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

func TestHandleStreamConnSkipsBufferedLiveRuntimeReplay(t *testing.T) {
	modelClient := &stubLoopModelClient{
		generateToolWait: make(chan struct{}),
		generateToolSeen: make(chan struct{}),
	}
	server := newTestServerWithModelClient(modelClient)
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	go server.handleStreamConn(left)

	encoder := json.NewEncoder(right)
	decoder := json.NewDecoder(right)
	request := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-stream-runtime-no-replay"`),
		Method:  "agent.input.submit",
		Params: mustMarshal(t, map[string]any{
			"session_id": "sess_stream_runtime_no_replay",
			"input": map[string]any{
				"type": "text",
				"text": "inspect this workspace and answer directly",
			},
			"options": map[string]any{
				"confirm_required": false,
			},
		}),
	}

	if err := encoder.Encode(request); err != nil {
		t.Fatalf("encode stream request: %v", err)
	}
	if err := right.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("set live notification deadline: %v", err)
	}

	var liveNotification notificationEnvelope
	if err := decoder.Decode(&liveNotification); err != nil {
		t.Fatalf("decode live runtime notification: %v", err)
	}
	if !isLiveRuntimeMethod(liveNotification.Method) {
		t.Fatalf("expected live runtime notification before response, got %+v", liveNotification)
	}
	if err := right.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("clear live notification deadline: %v", err)
	}

	close(modelClient.generateToolWait)

	if err := right.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("set response deadline: %v", err)
	}
	responseSeen := false
	for index := 0; index < 8; index++ {
		var envelope map[string]any
		if err := decoder.Decode(&envelope); err != nil {
			t.Fatalf("decode response envelope: %v", err)
		}
		if envelope["id"] == nil {
			continue
		}
		responseSeen = true
		break
	}
	if !responseSeen {
		t.Fatal("expected final response after live runtime notification")
	}

	if err := right.SetReadDeadline(time.Now().Add(250 * time.Millisecond)); err != nil {
		t.Fatalf("set replay deadline: %v", err)
	}
	for {
		var replayed notificationEnvelope
		if err := decoder.Decode(&replayed); err != nil {
			break
		}
		if isLiveRuntimeMethod(replayed.Method) {
			t.Fatalf("expected drain replay to skip already streamed runtime notification, got %+v", replayed)
		}
	}
	if err := right.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("clear replay deadline: %v", err)
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

func TestServerShutdownClosesActiveStreamHandlers(t *testing.T) {
	server := newTestServer()
	left, right := net.Pipe()
	defer right.Close()

	done := make(chan struct{})
	go func() {
		server.handleStreamConn(left)
		close(done)
	}()

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		server.streamMu.Lock()
		tracked := len(server.streamConns)
		server.streamMu.Unlock()
		if tracked == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expected stream handler to register active connection")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown active stream handler: %v", err)
	}

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected shutdown to wait for active stream handler exit")
	}
}

func TestServerShutdownWaitsForInFlightStreamRequests(t *testing.T) {
	server := newTestServer()
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	requestFinished := make(chan struct{})
	server.handlers["test.blocking"] = func(_ map[string]any) (any, *rpcError) {
		close(requestStarted)
		<-releaseRequest
		close(requestFinished)
		return map[string]any{"status": "released"}, nil
	}

	left, right := net.Pipe()
	defer right.Close()

	streamDone := make(chan struct{})
	go func() {
		server.handleStreamConn(left)
		close(streamDone)
	}()

	request := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-blocking-shutdown"`),
		Method:  "test.blocking",
		Params:  mustMarshal(t, map[string]any{}),
	}
	if err := json.NewEncoder(right).Encode(request); err != nil {
		t.Fatalf("encode blocking stream request: %v", err)
	}

	select {
	case <-requestStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected blocking stream request to start running")
	}

	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- server.Shutdown(context.Background())
	}()

	select {
	case <-streamDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected shutdown to close the outer stream loop promptly")
	}

	select {
	case err := <-shutdownDone:
		t.Fatalf("expected shutdown to keep waiting for the in-flight request, got %v", err)
	case <-time.After(250 * time.Millisecond):
	}

	select {
	case <-requestFinished:
		t.Fatal("expected blocking stream request to remain in flight until explicitly released")
	default:
	}

	close(releaseRequest)

	select {
	case <-requestFinished:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected blocking stream request to finish after release")
	}

	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("shutdown in-flight stream request: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected shutdown to finish after the in-flight request exits")
	}
}

func TestHandleStreamConnSkipsQueuedRequestsAfterClientDisconnect(t *testing.T) {
	server := newTestServer()
	taskID := "task_disconnect_queue"
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	var (
		callMu    sync.Mutex
		callCount int
	)

	server.handlers["test.same.task.queue"] = func(_ map[string]any) (any, *rpcError) {
		callMu.Lock()
		callCount++
		currentCall := callCount
		callMu.Unlock()

		if currentCall == 1 {
			close(firstStarted)
			<-releaseFirst
		}

		return map[string]any{
			"task": map[string]any{
				"task_id": taskID,
			},
		}, nil
	}

	left, right := net.Pipe()
	streamDone := make(chan struct{})
	go func() {
		server.handleStreamConn(left)
		close(streamDone)
	}()

	encoder := json.NewEncoder(right)
	firstRequest := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-disconnect-first"`),
		Method:  "test.same.task.queue",
		Params: mustMarshal(t, map[string]any{
			"task_id": taskID,
		}),
	}
	secondRequest := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-disconnect-second"`),
		Method:  "test.same.task.queue",
		Params: mustMarshal(t, map[string]any{
			"task_id": taskID,
		}),
	}

	if err := encoder.Encode(firstRequest); err != nil {
		t.Fatalf("encode first queued request: %v", err)
	}
	select {
	case <-firstStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected first queued request to start running")
	}

	if err := encoder.Encode(secondRequest); err != nil {
		t.Fatalf("encode second queued request: %v", err)
	}
	if err := right.Close(); err != nil {
		t.Fatalf("close client stream: %v", err)
	}

	select {
	case <-streamDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected outer stream loop to stop after client disconnect")
	}

	close(releaseFirst)
	time.Sleep(150 * time.Millisecond)

	callMu.Lock()
	defer callMu.Unlock()
	if callCount != 1 {
		t.Fatalf("expected queued request backlog to stop before dispatch after disconnect, got %d calls", callCount)
	}
}

func TestServerShutdownCancelsNamedPipeRunWithoutParentContextCancellation(t *testing.T) {
	server := newTestServer()
	server.transport = "named_pipe"
	server.debugHTTPServer = nil

	listenerStarted := make(chan struct{})
	listenerStopped := make(chan struct{})
	server.serveNamedPipe = func(ctx context.Context, pipeName string, handler func(net.Conn)) error {
		close(listenerStarted)
		<-ctx.Done()
		close(listenerStopped)
		return nil
	}

	startErr := make(chan error, 1)
	go func() {
		startErr <- server.Start(context.Background())
	}()

	select {
	case <-listenerStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected named-pipe listener to start")
	}

	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown named-pipe listener: %v", err)
	}

	select {
	case <-listenerStopped:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected shutdown to cancel the named-pipe listener context")
	}

	select {
	case err := <-startErr:
		if err != nil {
			t.Fatalf("expected Start to return cleanly after direct shutdown, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected Start to return after direct shutdown")
	}
}

func TestServerShutdownStopsStreamsBeforeDebugHTTPDrainCompletes(t *testing.T) {
	server := newTestServer()

	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	server.debugHTTPServer = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(requestStarted)
			<-releaseRequest
			w.WriteHeader(http.StatusNoContent)
		}),
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen debug http test server: %v", err)
	}
	defer listener.Close()

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.debugHTTPServer.Serve(listener)
	}()

	httpErr := make(chan error, 1)
	go func() {
		response, err := http.Get("http://" + listener.Addr().String())
		if response != nil {
			response.Body.Close()
		}
		httpErr <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected blocking debug HTTP request to start")
	}

	left, right := net.Pipe()
	defer right.Close()

	streamDone := make(chan struct{})
	go func() {
		server.handleStreamConn(left)
		close(streamDone)
	}()

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		server.streamMu.Lock()
		tracked := len(server.streamConns)
		server.streamMu.Unlock()
		if tracked == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("expected stream handler to register active connection")
		}
		time.Sleep(10 * time.Millisecond)
	}

	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- server.Shutdown(context.Background())
	}()

	select {
	case <-streamDone:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected shutdown to stop active streams before debug HTTP drain completes")
	}

	close(releaseRequest)

	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("shutdown with blocking debug HTTP request: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected shutdown to finish after debug HTTP drain releases")
	}

	select {
	case err := <-serveDone:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Fatalf("serve debug http test server: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected debug HTTP server to stop after shutdown")
	}

	select {
	case err := <-httpErr:
		if err != nil {
			t.Fatalf("debug HTTP request after shutdown: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected debug HTTP client to finish after shutdown")
	}
}

func TestHandleStreamConnRejectsNewHandlersDuringShutdown(t *testing.T) {
	server := newTestServer()
	server.shuttingDown = true
	left, right := net.Pipe()
	defer right.Close()

	done := make(chan struct{})
	go func() {
		server.handleStreamConn(left)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected handler to exit immediately once shutdown begins")
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
	err := supervisor.Wait(time.Second, func(context.Context) error {
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

func TestTransportSupervisorReturnsTimeoutWhenWorkerDoesNotExit(t *testing.T) {
	supervisor := newTransportSupervisor(context.Background(), 2)
	releaseWorker := make(chan struct{})
	workerExited := make(chan struct{})

	supervisor.Go(func(context.Context) error {
		<-releaseWorker
		close(workerExited)
		return nil
	})
	supervisor.Go(func(context.Context) error {
		return errors.New("listen failed")
	})

	err := supervisor.Wait(25*time.Millisecond, func(context.Context) error {
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected bounded shutdown timeout, got %v", err)
	}

	close(releaseWorker)
	select {
	case <-workerExited:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected stalled worker to exit once released")
	}
}

func TestServerStartRejectsRestartAfterTransportTimeout(t *testing.T) {
	server := newTestServer()
	server.transport = "named_pipe"
	server.debugHTTPServer = nil
	server.transportShutdownTimeout = 25 * time.Millisecond

	startCalls := make(chan struct{}, 2)
	releaseListener := make(chan struct{})
	defer close(releaseListener)
	server.serveNamedPipe = func(context.Context, string, func(net.Conn)) error {
		startCalls <- struct{}{}
		<-releaseListener
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	startErr := make(chan error, 1)
	go func() {
		startErr <- server.Start(ctx)
	}()

	select {
	case <-startCalls:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected named-pipe listener to start")
	}
	cancel()

	select {
	case err := <-startErr:
		if !errors.Is(err, errTransportShutdownIncomplete) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("expected incomplete shutdown timeout to fence server reuse, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected Start to return once transport shutdown times out")
	}

	reuseErr := make(chan error, 1)
	go func() {
		reuseErr <- server.Start(context.Background())
	}()
	select {
	case err := <-reuseErr:
		if !errors.Is(err, errServerAlreadyStarted) {
			t.Fatalf("expected one-shot server to reject restart after timeout, got %v", err)
		}
	case <-startCalls:
		t.Fatal("expected terminal server to reject reuse before starting transports")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected terminal server reuse to return without blocking")
	}
}

func TestServerStartRejectsConcurrentServeRun(t *testing.T) {
	server := newTestServer()
	server.transport = "named_pipe"
	server.debugHTTPServer = nil

	started := make(chan struct{}, 1)
	listenerReleased := make(chan struct{})
	server.serveNamedPipe = func(ctx context.Context, pipeName string, handler func(net.Conn)) error {
		started <- struct{}{}
		<-ctx.Done()
		<-listenerReleased
		return nil
	}

	firstStartErr := make(chan error, 1)
	go func() {
		firstStartErr <- server.Start(context.Background())
	}()

	select {
	case <-started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected first serve run to start")
	}

	secondStartErr := make(chan error, 1)
	go func() {
		secondStartErr <- server.Start(context.Background())
	}()

	select {
	case err := <-secondStartErr:
		if !errors.Is(err, errServerAlreadyStarted) {
			t.Fatalf("expected one-shot server to reject concurrent Start, got %v", err)
		}
	case <-started:
		t.Fatal("expected second Start to fail before opening another listener")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected concurrent Start to return without blocking")
	}

	close(listenerReleased)
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown first serve run: %v", err)
	}

	select {
	case err := <-firstStartErr:
		if err != nil {
			t.Fatalf("expected first serve run to stop cleanly, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected first serve run to exit after shutdown")
	}
}

func TestServerStartRejectsRestartAfterCleanShutdown(t *testing.T) {
	server := newTestServer()
	server.transport = "named_pipe"
	server.debugHTTPServer = nil

	started := make(chan struct{}, 1)
	listenerReleased := make(chan struct{}, 1)
	server.serveNamedPipe = func(ctx context.Context, pipeName string, handler func(net.Conn)) error {
		started <- struct{}{}
		<-ctx.Done()
		<-listenerReleased
		return nil
	}

	firstStartErr := make(chan error, 1)
	go func() {
		firstStartErr <- server.Start(context.Background())
	}()

	select {
	case <-started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected first serve run to start")
	}

	listenerReleased <- struct{}{}
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown first serve run: %v", err)
	}

	select {
	case err := <-firstStartErr:
		if err != nil {
			t.Fatalf("expected first serve run to stop cleanly, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected first serve run to exit after shutdown")
	}

	secondStartErr := make(chan error, 1)
	go func() {
		secondStartErr <- server.Start(context.Background())
	}()

	select {
	case err := <-secondStartErr:
		if !errors.Is(err, errServerAlreadyStarted) {
			t.Fatalf("expected one-shot server to reject restart after clean shutdown, got %v", err)
		}
	case <-started:
		t.Fatal("expected one-shot server to reject restart before opening another listener")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected second Start to return without blocking")
	}
}

func TestServerShutdownBeforeStartDoesNotPoisonFutureServeRun(t *testing.T) {
	server := newTestServer()
	server.transport = "named_pipe"
	server.debugHTTPServer = nil

	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown idle server: %v", err)
	}

	started := make(chan struct{}, 1)
	listenerReleased := make(chan struct{}, 1)
	server.serveNamedPipe = func(ctx context.Context, pipeName string, handler func(net.Conn)) error {
		started <- struct{}{}
		<-ctx.Done()
		<-listenerReleased
		return nil
	}

	startErr := make(chan error, 1)
	go func() {
		startErr <- server.Start(context.Background())
	}()

	select {
	case <-started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected idle shutdown not to block a later serve run")
	}

	listenerReleased <- struct{}{}
	if err := server.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown later serve run: %v", err)
	}

	select {
	case err := <-startErr:
		if err != nil {
			t.Fatalf("expected later serve run to stop cleanly, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected later serve run to exit after shutdown")
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
