// RPC server tests verify response envelopes and notification behavior.
package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/execution"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/memory"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/risk"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/builtin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/sidecarclient"
)

type stubLoopModelClient struct {
	toolResult       model.ToolCallResult
	generateToolWait chan struct{}
	generateToolSeen chan struct{}
}

// selectiveWaitLoopModelClient only applies the blocking tool-call gate to one
// task so stream-serialization tests can distinguish per-task locking from
// unrelated concurrent requests.
type selectiveWaitLoopModelClient struct {
	stubLoopModelClient
	blockedTaskID string
}

func (s *stubLoopModelClient) GenerateText(_ context.Context, request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
	return model.GenerateTextResponse{
		TaskID:     request.TaskID,
		RunID:      request.RunID,
		RequestID:  "req_loop_text",
		Provider:   "openai_responses",
		ModelID:    "gpt-5.4",
		OutputText: "loop fallback output",
	}, nil
}

func (s *stubLoopModelClient) GenerateToolCalls(_ context.Context, request model.ToolCallRequest) (model.ToolCallResult, error) {
	if s.generateToolSeen != nil {
		select {
		case <-s.generateToolSeen:
		default:
			close(s.generateToolSeen)
		}
	}
	if s.generateToolWait != nil {
		<-s.generateToolWait
	}
	result := s.toolResult
	if strings.TrimSpace(result.OutputText) == "" && len(result.ToolCalls) == 0 {
		result.OutputText = request.Input
	}
	if result.RequestID == "" {
		result.RequestID = "req_loop_tools"
	}
	if result.Provider == "" {
		result.Provider = "openai_responses"
	}
	if result.ModelID == "" {
		result.ModelID = "gpt-5.4"
	}
	return result, nil
}

func (s *selectiveWaitLoopModelClient) GenerateToolCalls(ctx context.Context, request model.ToolCallRequest) (model.ToolCallResult, error) {
	if strings.TrimSpace(s.blockedTaskID) == "" || request.TaskID == s.blockedTaskID {
		return s.stubLoopModelClient.GenerateToolCalls(ctx, request)
	}

	result := s.toolResult
	if strings.TrimSpace(result.OutputText) == "" && len(result.ToolCalls) == 0 {
		result.OutputText = request.Input
	}
	if result.RequestID == "" {
		result.RequestID = "req_loop_tools"
	}
	if result.Provider == "" {
		result.Provider = "openai_responses"
	}
	if result.ModelID == "" {
		result.ModelID = "gpt-5.4"
	}
	return result, nil
}

type testStorageAdapter struct {
	databasePath string
}

type stubExecutionCapability struct {
	result tools.CommandExecutionResult
	err    error
}

func (s stubExecutionCapability) RunCommand(_ context.Context, _ string, _ []string, _ string) (tools.CommandExecutionResult, error) {
	if s.err != nil {
		return tools.CommandExecutionResult{}, s.err
	}
	return s.result, nil
}

func (a testStorageAdapter) DatabasePath() string {
	return a.databasePath
}

func (a testStorageAdapter) SecretStorePath() string {
	if a.databasePath == "" {
		return ""
	}
	return a.databasePath + ".stronghold"
}

// TestHandleStreamConnEmitsApprovalNotifications verifies that approval notifications
// are emitted on the stream connection after task confirmation enters waiting_auth.
func TestHandleStreamConnEmitsApprovalNotifications(t *testing.T) {
	server := newTestServer()
	startResult, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "请生成一个文件版本",
		},
	})
	if err != nil {
		t.Fatalf("seed task.start: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	if startResult["task"].(map[string]any)["status"] != "confirming_intent" {
		t.Fatalf("expected seeded task to wait for confirm, got %+v", startResult["task"])
	}
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	go server.handleStreamConn(left)

	encoder := json.NewEncoder(right)
	decoder := json.NewDecoder(right)

	request := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-1"`),
		Method:  "agent.task.confirm",
		Params: mustMarshal(t, map[string]any{
			"task_id":   taskID,
			"confirmed": false,
			"corrected_intent": map[string]any{
				"name": "write_file",
				"arguments": map[string]any{
					"require_authorization": true,
					"target_path":           "workspace_document",
				},
			},
		}),
	}

	if err := encoder.Encode(request); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	var response successEnvelope
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Result.Data.(map[string]any)["task"].(map[string]any)["status"] != "waiting_auth" {
		t.Fatalf("expected waiting_auth task status in response")
	}

	if err := right.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	seenApprovalPending := false
	for index := 0; index < 8; index++ {
		var notification notificationEnvelope
		if err := decoder.Decode(&notification); err != nil {
			break
		}
		if notification.Method == "approval.pending" {
			seenApprovalPending = true
		}
	}

	if !seenApprovalPending {
		t.Fatal("expected approval.pending notification to be emitted on stream connection")
	}
}

func TestHandleStreamConnEmitsLoopLifecycleNotifications(t *testing.T) {
	server := newTestServer()
	startResult, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_loop_notify",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "Inspect the workspace and answer.",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("seed task.start: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	if _, ok := server.orchestrator.RunEngine().EmitRuntimeNotification(taskID, "loop.round.completed", map[string]any{
		"loop_round":  1,
		"stop_reason": "completed",
	}); !ok {
		t.Fatal("expected runtime notification injection to succeed")
	}
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	go server.handleStreamConn(left)

	encoder := json.NewEncoder(right)
	decoder := json.NewDecoder(right)

	request := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-detail"`),
		Method:  "agent.task.detail.get",
		Params: mustMarshal(t, map[string]any{
			"task_id": taskID,
		}),
	}

	if err := encoder.Encode(request); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	var response successEnvelope
	if err := decoder.Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Result.Data.(map[string]any)["task"].(map[string]any)["task_id"] != taskID {
		t.Fatalf("expected task detail response for %s, got %+v", taskID, response)
	}

	if err := right.SetReadDeadline(time.Now().Add(300 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	seenLoopNotification := false
	for index := 0; index < 12; index++ {
		var notification notificationEnvelope
		if err := decoder.Decode(&notification); err != nil {
			break
		}
		if strings.HasPrefix(notification.Method, "loop.") {
			seenLoopNotification = true
			break
		}
	}
	if !seenLoopNotification {
		t.Fatal("expected loop.* notification to be emitted on stream connection")
	}
}

func TestHandleStreamConnStreamsLoopLifecycleNotificationsBeforeResponse(t *testing.T) {
	modelClient := &stubLoopModelClient{
		toolResult: model.ToolCallResult{
			OutputText: "Loop runtime finished in-flight.",
		},
		generateToolWait: make(chan struct{}),
		generateToolSeen: make(chan struct{}),
	}
	server := newTestServerWithModelClient(modelClient)
	startResult, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_loop_stream",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "inspect this workspace",
		},
	})
	if err != nil {
		t.Fatalf("seed task.start: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	if startResult["task"].(map[string]any)["status"] != "confirming_intent" {
		t.Fatalf("expected seeded task to wait for confirm, got %+v", startResult["task"])
	}
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	go server.handleStreamConn(left)

	encoder := json.NewEncoder(right)
	decoder := json.NewDecoder(right)
	request := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-loop-stream"`),
		Method:  "agent.task.confirm",
		Params: mustMarshal(t, map[string]any{
			"task_id":   taskID,
			"confirmed": false,
			"corrected_intent": map[string]any{
				"name":      "agent_loop",
				"arguments": map[string]any{},
			},
		}),
	}

	if err := encoder.Encode(request); err != nil {
		t.Fatalf("encode request: %v", err)
	}
	if err := right.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	var firstEnvelope map[string]any
	if err := decoder.Decode(&firstEnvelope); err != nil {
		t.Fatalf("decode first envelope: %v", err)
	}
	if method, _ := firstEnvelope["method"].(string); !strings.HasPrefix(method, "loop.") {
		t.Fatalf("expected first streamed envelope to be loop.* notification, got %+v", firstEnvelope)
	}
	if err := right.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("clear read deadline: %v", err)
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
		result, ok := envelope["result"].(map[string]any)
		if !ok {
			t.Fatalf("expected success result envelope, got %+v", envelope)
		}
		data, ok := result["data"].(map[string]any)
		if !ok {
			t.Fatalf("expected response data payload, got %+v", envelope)
		}
		task, ok := data["task"].(map[string]any)
		if !ok || task["status"] != "completed" {
			t.Fatalf("expected completed task response, got %+v", envelope)
		}
		responseSeen = true
		break
	}
	if !responseSeen {
		t.Fatal("expected final response after streamed loop notifications")
	}
}

func TestHandleStreamConnStreamsLoopLifecycleNotificationsBeforeResponseForSubmitInput(t *testing.T) {
	modelClient := &stubLoopModelClient{
		toolResult: model.ToolCallResult{
			OutputText: "Loop runtime finished from input.submit.",
		},
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
		ID:      json.RawMessage(`"req-input-submit-loop-stream"`),
		Method:  "agent.input.submit",
		Params: mustMarshal(t, map[string]any{
			"session_id": "sess_input_submit_loop_stream",
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
		t.Fatalf("encode request: %v", err)
	}
	if err := right.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	var firstEnvelope map[string]any
	if err := decoder.Decode(&firstEnvelope); err != nil {
		t.Fatalf("decode first envelope: %v", err)
	}
	if method, _ := firstEnvelope["method"].(string); !strings.HasPrefix(method, "loop.") {
		t.Fatalf("expected first streamed envelope to be loop.* notification, got %+v", firstEnvelope)
	}
	if err := right.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("clear read deadline: %v", err)
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
		result, ok := envelope["result"].(map[string]any)
		if !ok {
			t.Fatalf("expected success result envelope, got %+v", envelope)
		}
		data, ok := result["data"].(map[string]any)
		if !ok {
			t.Fatalf("expected response data payload, got %+v", envelope)
		}
		task, ok := data["task"].(map[string]any)
		if !ok || task["status"] != "completed" {
			t.Fatalf("expected completed task response, got %+v", envelope)
		}
		responseSeen = true
		break
	}
	if !responseSeen {
		t.Fatal("expected final response after streamed loop notifications")
	}
}

func TestHandleStreamConnDoesNotReplayStreamedRuntimeNotificationsAfterResponse(t *testing.T) {
	modelClient := &stubLoopModelClient{
		toolResult: model.ToolCallResult{
			OutputText: "Loop runtime should not replay live events.",
		},
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
		ID:      json.RawMessage(`"req-loop-no-replay"`),
		Method:  "agent.input.submit",
		Params: mustMarshal(t, map[string]any{
			"session_id": "sess_input_submit_no_replay",
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
		t.Fatalf("encode request: %v", err)
	}
	if err := right.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("set first notification deadline: %v", err)
	}

	var firstEnvelope notificationEnvelope
	if err := decoder.Decode(&firstEnvelope); err != nil {
		t.Fatalf("decode first notification: %v", err)
	}
	if !strings.HasPrefix(firstEnvelope.Method, "loop.") {
		t.Fatalf("expected first streamed envelope to be loop.* notification, got %+v", firstEnvelope)
	}
	if err := right.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("clear read deadline: %v", err)
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
		t.Fatal("expected final response after streamed loop notifications")
	}

	if err := right.SetReadDeadline(time.Now().Add(250 * time.Millisecond)); err != nil {
		t.Fatalf("set replay deadline: %v", err)
	}
	for {
		var envelope notificationEnvelope
		if err := decoder.Decode(&envelope); err != nil {
			break
		}
		if isLiveRuntimeMethod(envelope.Method) {
			t.Fatalf("expected streamed runtime notifications to be skipped after response, got %+v", envelope)
		}
	}
	if err := right.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("clear replay deadline: %v", err)
	}
}

func TestHandleStreamConnFiltersRuntimeNotificationsToRequestTask(t *testing.T) {
	modelClient := &stubLoopModelClient{
		toolResult: model.ToolCallResult{
			OutputText: "Scoped runtime finished.",
		},
		generateToolWait: make(chan struct{}),
	}
	server := newTestServerWithModelClient(modelClient)

	startTask := func(sessionID string) string {
		t.Helper()
		result, err := server.orchestrator.StartTask(map[string]any{
			"session_id": sessionID,
			"source":     "floating_ball",
			"trigger":    "text_selected_click",
			"input": map[string]any{
				"type": "text_selection",
				"text": "inspect this workspace",
			},
		})
		if err != nil {
			t.Fatalf("seed task.start for %s: %v", sessionID, err)
		}
		return result["task"].(map[string]any)["task_id"].(string)
	}

	taskA := startTask("sess_loop_scope_a")
	taskB := startTask("sess_loop_scope_b")

	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	go server.handleStreamConn(left)

	encoder := json.NewEncoder(right)
	decoder := json.NewDecoder(right)
	request := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-loop-scope"`),
		Method:  "agent.task.confirm",
		Params: mustMarshal(t, map[string]any{
			"task_id":   taskA,
			"confirmed": false,
			"corrected_intent": map[string]any{
				"name":      "agent_loop",
				"arguments": map[string]any{},
			},
		}),
	}

	if err := encoder.Encode(request); err != nil {
		t.Fatalf("encode request: %v", err)
	}
	if err := right.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("set first notification deadline: %v", err)
	}

	var firstEnvelope notificationEnvelope
	if err := decoder.Decode(&firstEnvelope); err != nil {
		t.Fatalf("decode first notification: %v", err)
	}
	if !strings.HasPrefix(firstEnvelope.Method, "loop.") {
		t.Fatalf("expected first streamed envelope to be loop.* notification, got %+v", firstEnvelope)
	}

	confirmDone := make(chan error, 1)
	go func() {
		_, err := server.orchestrator.ConfirmTask(map[string]any{
			"task_id":   taskB,
			"confirmed": false,
			"corrected_intent": map[string]any{
				"name":      "agent_loop",
				"arguments": map[string]any{},
			},
		})
		confirmDone <- err
	}()

	if err := right.SetReadDeadline(time.Now().Add(250 * time.Millisecond)); err != nil {
		t.Fatalf("set scoped notification deadline: %v", err)
	}
	for {
		var envelope notificationEnvelope
		if err := decoder.Decode(&envelope); err != nil {
			break
		}
		params, ok := envelope.Params.(map[string]any)
		if !ok {
			t.Fatalf("expected notification params map, got %+v", envelope)
		}
		taskID := stringValue(params, "task_id", "")
		if taskID == taskB {
			t.Fatalf("expected stream to suppress unrelated runtime notification for task %s, got %+v", taskB, envelope)
		}
	}
	if err := right.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("clear read deadline: %v", err)
	}

	close(modelClient.generateToolWait)

	select {
	case err := <-confirmDone:
		if err != nil {
			t.Fatalf("confirm unrelated task: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected unrelated task confirmation to complete")
	}
}

func TestHandleStreamConnAllowsSettingsReadWhileTaskConfirmWaits(t *testing.T) {
	server := newTestServer()
	blockingSeen := make(chan struct{})
	releaseBlocking := make(chan struct{})
	releasedBlocking := false
	defer func() {
		if !releasedBlocking {
			close(releaseBlocking)
		}
	}()

	server.handlers["test.blocking"] = func(_ map[string]any) (any, *rpcError) {
		select {
		case <-blockingSeen:
		default:
			close(blockingSeen)
		}
		<-releaseBlocking
		return map[string]any{"status": "released"}, nil
	}
	server.handlers["test.fast"] = func(_ map[string]any) (any, *rpcError) {
		return map[string]any{"status": "fast"}, nil
	}

	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	go server.handleStreamConn(left)

	encoder := json.NewEncoder(right)
	decoder := json.NewDecoder(right)
	blockingRequest := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-blocking"`),
		Method:  "test.blocking",
		Params:  mustMarshal(t, map[string]any{}),
	}
	if err := encoder.Encode(blockingRequest); err != nil {
		t.Fatalf("encode blocked request: %v", err)
	}

	select {
	case <-blockingSeen:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected blocking request to start running")
	}

	settingsRequest := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-fast"`),
		Method:  "test.fast",
		Params:  mustMarshal(t, map[string]any{}),
	}
	if err := encoder.Encode(settingsRequest); err != nil {
		t.Fatalf("encode fast request: %v", err)
	}

	if err := right.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}

	seenSettingsResponse := false
	for !seenSettingsResponse {
		var envelope map[string]any
		if err := decoder.Decode(&envelope); err != nil {
			t.Fatalf("decode concurrent envelope: %v", err)
		}

		id, _ := envelope["id"].(string)
		if id != "req-fast" {
			continue
		}

		result, ok := envelope["result"].(map[string]any)
		if !ok {
			t.Fatalf("expected fast response result envelope, got %+v", envelope)
		}
		data, ok := result["data"].(map[string]any)
		if !ok {
			t.Fatalf("expected fast response data payload, got %+v", result)
		}
		if stringValue(data, "status", "") != "fast" {
			t.Fatalf("expected fast request result payload, got %+v", data)
		}
		seenSettingsResponse = true
	}

	if err := right.SetReadDeadline(time.Time{}); err != nil {
		t.Fatalf("clear read deadline: %v", err)
	}

	close(releaseBlocking)
	releasedBlocking = true
}

func TestHandleStreamConnAppliesBackpressureWhenPendingQueueFills(t *testing.T) {
	server := newTestServer()
	startedSignals := make(chan struct{}, maxPendingStreamRequests+1)
	releaseBlocking := make(chan struct{})
	releasedBlocking := false
	defer func() {
		if !releasedBlocking {
			close(releaseBlocking)
		}
	}()

	var startedMu sync.Mutex
	startedCount := 0
	server.handlers["test.blocking"] = func(_ map[string]any) (any, *rpcError) {
		startedMu.Lock()
		startedCount++
		startedMu.Unlock()

		startedSignals <- struct{}{}
		<-releaseBlocking
		return map[string]any{"status": "released"}, nil
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen loopback: %v", err)
	}
	defer listener.Close()

	acceptDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptDone <- err
			return
		}
		server.handleStreamConn(conn)
		acceptDone <- nil
	}()

	right, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial loopback: %v", err)
	}
	defer func() {
		_ = right.Close()
		select {
		case err := <-acceptDone:
			if err != nil && !errors.Is(err, net.ErrClosed) {
				t.Fatalf("accept loopback: %v", err)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("expected loopback stream to shut down")
		}
	}()

	encoder := json.NewEncoder(right)
	for index := 0; index < maxPendingStreamRequests; index++ {
		request := requestEnvelope{
			JSONRPC: "2.0",
			ID:      json.RawMessage(fmt.Sprintf(`"req-blocking-%d"`, index)),
			Method:  "test.blocking",
			Params:  mustMarshal(t, map[string]any{}),
		}
		if err := encoder.Encode(request); err != nil {
			t.Fatalf("encode blocking request %d: %v", index, err)
		}
	}

	for index := 0; index < maxPendingStreamRequests; index++ {
		select {
		case <-startedSignals:
		case <-time.After(2 * time.Second):
			t.Fatalf("expected request %d to start before the queue filled", index)
		}
	}

	startedMu.Lock()
	if startedCount != maxPendingStreamRequests {
		startedMu.Unlock()
		t.Fatalf("expected exactly %d started requests before backpressure, got %d", maxPendingStreamRequests, startedCount)
	}
	startedMu.Unlock()

	extraRequestDone := make(chan error, 1)
	go func() {
		extraRequestDone <- encoder.Encode(requestEnvelope{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"req-blocking-overflow"`),
			Method:  "test.blocking",
			Params:  mustMarshal(t, map[string]any{}),
		})
	}()

	select {
	case <-startedSignals:
		t.Fatal("expected overflow request to wait until a pending slot is released")
	case <-time.After(250 * time.Millisecond):
	}

	close(releaseBlocking)
	releasedBlocking = true

	select {
	case <-startedSignals:
	case <-time.After(2 * time.Second):
		t.Fatal("expected overflow request to start after pending capacity became available")
	}

	select {
	case err := <-extraRequestDone:
		if err != nil {
			t.Fatalf("encode overflow request: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected overflow request write to complete after backpressure released")
	}
}

func TestHandleStreamConnDropsOverflowRequestAfterDisconnectWithFullPendingQueue(t *testing.T) {
	server := newTestServer()
	startedSignals := make(chan struct{}, maxPendingStreamRequests+1)
	releaseBlocking := make(chan struct{})
	releasedBlocking := false
	defer func() {
		if !releasedBlocking {
			close(releaseBlocking)
		}
	}()

	var startedMu sync.Mutex
	startedCount := 0
	server.handlers["test.blocking"] = func(_ map[string]any) (any, *rpcError) {
		startedMu.Lock()
		startedCount++
		startedMu.Unlock()

		startedSignals <- struct{}{}
		<-releaseBlocking
		return map[string]any{"status": "released"}, nil
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen loopback: %v", err)
	}
	defer listener.Close()

	acceptDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptDone <- err
			return
		}
		server.handleStreamConn(conn)
		acceptDone <- nil
	}()

	right, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial loopback: %v", err)
	}

	encoder := json.NewEncoder(right)
	for index := 0; index < maxPendingStreamRequests; index++ {
		request := requestEnvelope{
			JSONRPC: "2.0",
			ID:      json.RawMessage(fmt.Sprintf(`"req-disconnect-blocking-%d"`, index)),
			Method:  "test.blocking",
			Params:  mustMarshal(t, map[string]any{}),
		}
		if err := encoder.Encode(request); err != nil {
			t.Fatalf("encode blocking request %d: %v", index, err)
		}
	}

	for index := 0; index < maxPendingStreamRequests; index++ {
		select {
		case <-startedSignals:
		case <-time.After(2 * time.Second):
			t.Fatalf("expected request %d to start before the queue filled", index)
		}
	}

	startedMu.Lock()
	if startedCount != maxPendingStreamRequests {
		startedMu.Unlock()
		t.Fatalf("expected exactly %d started requests before the disconnect race, got %d", maxPendingStreamRequests, startedCount)
	}
	startedMu.Unlock()

	extraRequestDone := make(chan error, 1)
	go func() {
		extraRequestDone <- encoder.Encode(requestEnvelope{
			JSONRPC: "2.0",
			ID:      json.RawMessage(`"req-disconnect-overflow"`),
			Method:  "test.blocking",
			Params:  mustMarshal(t, map[string]any{}),
		})
	}()

	select {
	case <-startedSignals:
		t.Fatal("expected overflow request to remain queued before disconnect")
	case <-time.After(250 * time.Millisecond):
	}

	if err := right.Close(); err != nil {
		t.Fatalf("close client stream: %v", err)
	}

	select {
	case <-extraRequestDone:
	case <-time.After(2 * time.Second):
		t.Fatal("expected overflow request write to exit after client disconnect")
	}

	close(releaseBlocking)
	releasedBlocking = true

	select {
	case <-startedSignals:
		t.Fatal("expected disconnected overflow request not to start after pending capacity frees")
	case <-time.After(300 * time.Millisecond):
	}

	startedMu.Lock()
	if startedCount != maxPendingStreamRequests {
		startedMu.Unlock()
		t.Fatalf("expected disconnect to prevent stale overflow dispatch, got %d calls", startedCount)
	}
	startedMu.Unlock()

	select {
	case err := <-acceptDone:
		if err != nil && !errors.Is(err, net.ErrClosed) {
			t.Fatalf("accept loopback: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected loopback stream to shut down after pending workers released")
	}
}

func TestHandleStreamConnDropsDecodedSameTaskBacklogAfterDisconnect(t *testing.T) {
	server := newTestServer()
	taskID := "task_disconnect_same_task_backlog"
	startedSignals := make(chan int, maxPendingStreamRequests)
	releaseFirst := make(chan struct{})

	var startedMu sync.Mutex
	startedCount := 0
	server.handlers["test.same.task.blocking"] = func(params map[string]any) (any, *rpcError) {
		startedMu.Lock()
		startedCount++
		callIndex := startedCount
		startedMu.Unlock()

		startedSignals <- callIndex
		if callIndex == 1 {
			<-releaseFirst
		}

		return map[string]any{
			"task": map[string]any{
				"task_id": stringValue(params, "task_id", ""),
			},
		}, nil
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen loopback: %v", err)
	}
	defer listener.Close()

	acceptDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptDone <- err
			return
		}
		server.handleStreamConn(conn)
		acceptDone <- nil
	}()

	right, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial loopback: %v", err)
	}

	encoder := json.NewEncoder(right)
	for index := 0; index < maxPendingStreamRequests; index++ {
		request := requestEnvelope{
			JSONRPC: "2.0",
			ID:      json.RawMessage(fmt.Sprintf(`"req-same-task-disconnect-%d"`, index)),
			Method:  "test.same.task.blocking",
			Params: mustMarshal(t, map[string]any{
				"task_id": taskID,
			}),
		}
		if err := encoder.Encode(request); err != nil {
			t.Fatalf("encode same-task request %d: %v", index, err)
		}
	}

	select {
	case callIndex := <-startedSignals:
		if callIndex != 1 {
			t.Fatalf("expected the first same-task request to start first, got call %d", callIndex)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected the first same-task request to start")
	}

	select {
	case callIndex := <-startedSignals:
		t.Fatalf("expected same-task backlog to stay queued behind the first request, got call %d", callIndex)
	case <-time.After(250 * time.Millisecond):
	}

	if err := right.Close(); err != nil {
		t.Fatalf("close client stream: %v", err)
	}

	close(releaseFirst)

	select {
	case callIndex := <-startedSignals:
		t.Fatalf("expected disconnected same-task backlog not to start after release, got call %d", callIndex)
	case <-time.After(500 * time.Millisecond):
	}

	startedMu.Lock()
	if startedCount != 1 {
		startedMu.Unlock()
		t.Fatalf("expected only the first same-task request to dispatch before disconnect, got %d", startedCount)
	}
	startedMu.Unlock()

	select {
	case err := <-acceptDone:
		if err != nil && !errors.Is(err, net.ErrClosed) {
			t.Fatalf("accept loopback: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected loopback stream to shut down after same-task backlog release")
	}
}

func TestHandleStreamConnKeepsHealthyIdleSameTaskBacklogAlive(t *testing.T) {
	server := newTestServer()
	taskID := "task_idle_same_task_backlog"
	startedSignals := make(chan int, maxPendingStreamRequests)
	releaseFirst := make(chan struct{})

	var startedMu sync.Mutex
	startedCount := 0
	server.handlers["test.same.task.healthy"] = func(params map[string]any) (any, *rpcError) {
		startedMu.Lock()
		startedCount++
		callIndex := startedCount
		startedMu.Unlock()

		startedSignals <- callIndex
		if callIndex == 1 {
			<-releaseFirst
		}

		return map[string]any{
			"task": map[string]any{
				"task_id": stringValue(params, "task_id", ""),
			},
		}, nil
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen loopback: %v", err)
	}
	defer listener.Close()

	acceptDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptDone <- err
			return
		}
		server.handleStreamConn(conn)
		acceptDone <- nil
	}()

	right, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial loopback: %v", err)
	}
	defer right.Close()

	encoder := json.NewEncoder(right)
	decoder := json.NewDecoder(right)
	for index := 0; index < maxPendingStreamRequests; index++ {
		request := requestEnvelope{
			JSONRPC: "2.0",
			ID:      json.RawMessage(fmt.Sprintf(`"req-same-task-healthy-%d"`, index)),
			Method:  "test.same.task.healthy",
			Params: mustMarshal(t, map[string]any{
				"task_id": taskID,
			}),
		}
		if err := encoder.Encode(request); err != nil {
			t.Fatalf("encode same-task request %d: %v", index, err)
		}
	}

	select {
	case callIndex := <-startedSignals:
		if callIndex != 1 {
			t.Fatalf("expected the first same-task request to start first, got call %d", callIndex)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected the first same-task request to start")
	}

	select {
	case callIndex := <-startedSignals:
		t.Fatalf("expected same-task backlog to stay queued behind the first request, got call %d", callIndex)
	case <-time.After(250 * time.Millisecond):
	}

	close(releaseFirst)

	if err := right.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	defer func() {
		if err := right.SetReadDeadline(time.Time{}); err != nil {
			t.Fatalf("clear read deadline: %v", err)
		}
	}()

	var firstEnvelope map[string]any
	if err := decoder.Decode(&firstEnvelope); err != nil {
		t.Fatalf("decode first same-task response: %v", err)
	}
	if firstEnvelope["id"] == nil {
		t.Fatalf("expected the first same-task response envelope, got %+v", firstEnvelope)
	}
	if firstEnvelope["error"] != nil {
		t.Fatalf("expected first same-task response to succeed, got %+v", firstEnvelope)
	}

	select {
	case callIndex := <-startedSignals:
		if callIndex != 2 {
			t.Fatalf("expected the second same-task request to dispatch next, got call %d", callIndex)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected the second same-task request to dispatch after the first response")
	}

	var secondEnvelope map[string]any
	if err := decoder.Decode(&secondEnvelope); err != nil {
		t.Fatalf("decode second same-task response: %v", err)
	}
	if secondEnvelope["id"] == nil {
		t.Fatalf("expected the second same-task response envelope, got %+v", secondEnvelope)
	}
	if secondEnvelope["error"] != nil {
		t.Fatalf("expected same-task backlog to stay on the healthy shared stream, got %+v", secondEnvelope)
	}

	select {
	case err := <-acceptDone:
		if err != nil && !errors.Is(err, net.ErrClosed) {
			t.Fatalf("accept loopback: %v", err)
		}
	case <-time.After(250 * time.Millisecond):
	}
}

func TestHandleStreamConnSerializesTaskStartingRequestsOnSharedConnection(t *testing.T) {
	testCases := []struct {
		name         string
		method       string
		firstParams  map[string]any
		secondParams map[string]any
	}{
		{
			name:   "input submit",
			method: "agent.input.submit",
			firstParams: map[string]any{
				"session_id": "sess_serialized_submit",
				"input": map[string]any{
					"type": "text",
					"text": "first submit",
				},
			},
			secondParams: map[string]any{
				"session_id": "sess_serialized_submit",
				"input": map[string]any{
					"type": "text",
					"text": "second submit",
				},
			},
		},
		{
			name:   "task start",
			method: "agent.task.start",
			firstParams: map[string]any{
				"session_id": "sess_serialized_start",
				"source":     "floating_ball",
				"trigger":    "text_selected_click",
				"input": map[string]any{
					"type": "text_selection",
					"text": "first selection",
				},
			},
			secondParams: map[string]any{
				"session_id": "sess_serialized_start",
				"source":     "floating_ball",
				"trigger":    "text_selected_click",
				"input": map[string]any{
					"type": "text_selection",
					"text": "second selection",
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			server := newTestServer()
			firstStarted := make(chan struct{})
			releaseFirst := make(chan struct{})
			var callCount int
			var callMu sync.Mutex

			server.handlers[testCase.method] = func(_ map[string]any) (any, *rpcError) {
				callMu.Lock()
				callCount++
				currentCall := callCount
				callMu.Unlock()

				if currentCall == 1 {
					select {
					case <-firstStarted:
					default:
						close(firstStarted)
					}
					<-releaseFirst
				}

				return map[string]any{
					"task": map[string]any{
						"task_id": fmt.Sprintf("task_serial_%d", currentCall),
					},
				}, nil
			}

			left, right := net.Pipe()
			defer left.Close()
			defer right.Close()

			go server.handleStreamConn(left)

			encoder := json.NewEncoder(right)
			decoder := json.NewDecoder(right)
			firstRequest := requestEnvelope{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`"req-task-starting-1"`),
				Method:  testCase.method,
				Params:  mustMarshal(t, testCase.firstParams),
			}
			secondRequest := requestEnvelope{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`"req-task-starting-2"`),
				Method:  testCase.method,
				Params:  mustMarshal(t, testCase.secondParams),
			}

			if err := encoder.Encode(firstRequest); err != nil {
				t.Fatalf("encode first request: %v", err)
			}

			select {
			case <-firstStarted:
			case <-time.After(500 * time.Millisecond):
				t.Fatal("expected first task-starting request to begin running")
			}

			if err := encoder.Encode(secondRequest); err != nil {
				t.Fatalf("encode second request: %v", err)
			}

			type decodeResult struct {
				envelope map[string]any
				err      error
			}
			firstResponseCh := make(chan decodeResult, 1)
			go func() {
				var envelope map[string]any
				err := decoder.Decode(&envelope)
				firstResponseCh <- decodeResult{envelope: envelope, err: err}
			}()

			select {
			case result := <-firstResponseCh:
				if result.err != nil {
					t.Fatalf("expected no response before the first task-starting request finishes, got %v", result.err)
				}
				t.Fatalf("expected second task-starting request to stay queued until the first finishes, got %+v", result.envelope)
			case <-time.After(250 * time.Millisecond):
			}

			close(releaseFirst)

			seenResponses := map[string]bool{}
			select {
			case result := <-firstResponseCh:
				if result.err != nil {
					t.Fatalf("decode first serialized response envelope: %v", result.err)
				}
				id, _ := result.envelope["id"].(string)
				if id == "req-task-starting-1" || id == "req-task-starting-2" {
					seenResponses[id] = true
				}
			case <-time.After(1 * time.Second):
				t.Fatal("expected first queued task-starting request to finish after release")
			}

			if err := right.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
				t.Fatalf("set response deadline: %v", err)
			}
			for len(seenResponses) < 2 {
				var envelope map[string]any
				if err := decoder.Decode(&envelope); err != nil {
					t.Fatalf("decode serialized response envelope: %v", err)
				}
				id, _ := envelope["id"].(string)
				if id == "req-task-starting-1" || id == "req-task-starting-2" {
					seenResponses[id] = true
				}
			}
			if err := right.SetReadDeadline(time.Time{}); err != nil {
				t.Fatalf("clear response deadline: %v", err)
			}
		})
	}
}

func TestHandleStreamConnReplaysLateTaskNotificationsBeforeQueuedSameTaskFollowUp(t *testing.T) {
	server := newTestServer()
	startResult, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_late_task_replay",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "queue notifications for shared stream replay",
		},
	})
	if err != nil {
		t.Fatalf("seed task.start: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	notifications, err := server.orchestrator.PendingNotifications(taskID)
	if err != nil || len(notifications) == 0 {
		t.Fatalf("expected seeded task to queue notifications, err=%v notifications=%+v", err, notifications)
	}

	firstReturned := make(chan struct{})
	server.handlers["agent.task.start"] = func(_ map[string]any) (any, *rpcError) {
		select {
		case <-firstReturned:
		default:
			close(firstReturned)
		}
		return map[string]any{
			"task": map[string]any{
				"task_id": taskID,
			},
		}, nil
	}
	server.handlers["test.followup.task"] = func(params map[string]any) (any, *rpcError) {
		return map[string]any{
			"task": map[string]any{
				"task_id": stringValue(params, "task_id", ""),
			},
		}, nil
	}

	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	go server.handleStreamConn(left)

	encoder := json.NewEncoder(right)
	decoder := json.NewDecoder(right)
	firstRequest := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-late-task-starter"`),
		Method:  "agent.task.start",
		Params: mustMarshal(t, map[string]any{
			"session_id": "sess_late_task_response_owner",
			"source":     "floating_ball",
			"trigger":    "text_selected_click",
			"input": map[string]any{
				"type": "text_selection",
				"text": "start a task on the shared stream",
			},
		}),
	}
	if err := encoder.Encode(firstRequest); err != nil {
		t.Fatalf("encode first late-task response request: %v", err)
	}

	select {
	case <-firstReturned:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected first task-starting request to finish dispatch")
	}

	secondRequest := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-late-task-followup"`),
		Method:  "test.followup.task",
		Params: mustMarshal(t, map[string]any{
			"task_id": taskID,
		}),
	}
	if err := encoder.Encode(secondRequest); err != nil {
		t.Fatalf("encode same-task follow-up request: %v", err)
	}

	if err := right.SetReadDeadline(time.Now().Add(1500 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	defer func() {
		if err := right.SetReadDeadline(time.Time{}); err != nil {
			t.Fatalf("clear read deadline: %v", err)
		}
	}()

	var firstEnvelope map[string]any
	if err := decoder.Decode(&firstEnvelope); err != nil {
		t.Fatalf("decode first response envelope: %v", err)
	}
	if firstID, _ := firstEnvelope["id"].(string); firstID != "req-late-task-starter" {
		t.Fatalf("expected first envelope to be the starter response, got %+v", firstEnvelope)
	}

	notificationBeforeFollowUp := false
	followUpResponseSeen := false
	for index := 0; index < 12; index++ {
		var envelope map[string]any
		if err := decoder.Decode(&envelope); err != nil {
			t.Fatalf("decode shared stream envelope: %v", err)
		}
		if envelopeID, _ := envelope["id"].(string); envelopeID == "req-late-task-followup" {
			followUpResponseSeen = true
			break
		}
		if method, _ := envelope["method"].(string); method != "" {
			notificationBeforeFollowUp = true
		}
	}
	if !followUpResponseSeen {
		t.Fatal("expected follow-up response to arrive on the shared stream")
	}
	if !notificationBeforeFollowUp {
		t.Fatal("expected buffered notifications for the started task to replay before the queued same-task follow-up response")
	}
}

func TestHandleStreamConnTaskListDoesNotStealBufferedNotifications(t *testing.T) {
	server := newTestServer()
	startResult, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_task_list_replay_owner",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "queue notifications for task.list replay ownership",
		},
	})
	if err != nil {
		t.Fatalf("seed task.start: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	notifications, err := server.orchestrator.PendingNotifications(taskID)
	if err != nil || len(notifications) == 0 {
		t.Fatalf("expected seeded task to queue notifications, err=%v notifications=%+v", err, notifications)
	}

	listStarted := make(chan struct{})
	allowListReturn := make(chan struct{})
	server.handlers["agent.task.list"] = func(_ map[string]any) (any, *rpcError) {
		select {
		case <-listStarted:
		default:
			close(listStarted)
		}
		<-allowListReturn
		return map[string]any{
			"items": []any{
				map[string]any{"task_id": taskID},
			},
		}, nil
	}
	server.handlers["test.followup.task"] = func(params map[string]any) (any, *rpcError) {
		return map[string]any{
			"task": map[string]any{
				"task_id": stringValue(params, "task_id", ""),
			},
		}, nil
	}

	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	go server.handleStreamConn(left)

	encoder := json.NewEncoder(right)
	decoder := json.NewDecoder(right)
	if err := encoder.Encode(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-list-owned"`),
		Method:  "agent.task.list",
		Params:  mustMarshal(t, map[string]any{}),
	}); err != nil {
		t.Fatalf("encode task.list request: %v", err)
	}

	select {
	case <-listStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected task.list request to start dispatch")
	}

	if err := encoder.Encode(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-list-followup"`),
		Method:  "test.followup.task",
		Params: mustMarshal(t, map[string]any{
			"task_id": taskID,
		}),
	}); err != nil {
		t.Fatalf("encode follow-up request: %v", err)
	}
	close(allowListReturn)

	if err := right.SetReadDeadline(time.Now().Add(1500 * time.Millisecond)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	defer func() {
		if err := right.SetReadDeadline(time.Time{}); err != nil {
			t.Fatalf("clear read deadline: %v", err)
		}
	}()

	taskListResponseSeen := false
	followUpResponseSeen := false
	notificationAfterFollowUp := false
	for index := 0; index < 16; index++ {
		var envelope map[string]any
		if err := decoder.Decode(&envelope); err != nil {
			t.Fatalf("decode shared stream envelope: %v", err)
		}
		if envelopeID, _ := envelope["id"].(string); envelopeID == "req-task-list-owned" {
			taskListResponseSeen = true
		} else if envelopeID, _ := envelope["id"].(string); envelopeID == "req-task-list-followup" {
			followUpResponseSeen = true
		} else if method, _ := envelope["method"].(string); method != "" {
			if !followUpResponseSeen {
				t.Fatalf("expected task.list not to replay %s before the follow-up response, got %+v", method, envelope)
			}
			notificationAfterFollowUp = true
		}
		if taskListResponseSeen && followUpResponseSeen && notificationAfterFollowUp {
			break
		}
	}
	if !taskListResponseSeen {
		t.Fatal("expected task.list response to arrive on the shared stream")
	}
	if !followUpResponseSeen {
		t.Fatal("expected follow-up response to arrive on the shared stream")
	}
	if !notificationAfterFollowUp {
		t.Fatal("expected the owning follow-up request to replay buffered notifications after its response")
	}
}

func TestStreamTaskCoordinatorReleasesIdleTaskLocks(t *testing.T) {
	coordinator := newStreamTaskCoordinator()
	coordinator.withTaskLocks(map[string]bool{
		"task_cleanup": true,
	}, func() {})

	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	if len(coordinator.locks) != 0 {
		t.Fatalf("expected idle task locks to be released, got %+v", coordinator.locks)
	}
}

func TestHandleStreamConnKeepsQueuedReadsResponsiveWhileLoopTaskRuns(t *testing.T) {
	modelClient := &selectiveWaitLoopModelClient{
		stubLoopModelClient: stubLoopModelClient{
			toolResult: model.ToolCallResult{
				OutputText: "Concurrent stream finished.",
			},
			generateToolWait: make(chan struct{}),
			generateToolSeen: make(chan struct{}),
		},
	}
	server := newTestServerWithModelClient(modelClient)

	startTask := func(sessionID string) string {
		t.Helper()
		result, err := server.orchestrator.StartTask(map[string]any{
			"session_id": sessionID,
			"source":     "floating_ball",
			"trigger":    "text_selected_click",
			"input": map[string]any{
				"type": "text_selection",
				"text": "inspect this workspace",
			},
		})
		if err != nil {
			t.Fatalf("seed task.start for %s: %v", sessionID, err)
		}
		return result["task"].(map[string]any)["task_id"].(string)
	}

	taskA := startTask("sess_pipe_queue_a")
	taskB := startTask("sess_pipe_queue_b")
	modelClient.blockedTaskID = taskA

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen loopback: %v", err)
	}
	defer listener.Close()

	acceptDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptDone <- err
			return
		}
		server.handleStreamConn(conn)
		acceptDone <- nil
	}()

	right, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial loopback: %v", err)
	}
	defer func() {
		_ = right.Close()
		select {
		case err := <-acceptDone:
			if err != nil && !errors.Is(err, net.ErrClosed) {
				t.Fatalf("accept loopback: %v", err)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("expected loopback stream to shut down")
		}
	}()

	encoder := json.NewEncoder(right)
	decoder := json.NewDecoder(right)
	confirmRequest := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-loop-blocked"`),
		Method:  "agent.task.confirm",
		Params: mustMarshal(t, map[string]any{
			"task_id":   taskA,
			"confirmed": false,
			"corrected_intent": map[string]any{
				"name":      "agent_loop",
				"arguments": map[string]any{},
			},
		}),
	}
	if err := encoder.Encode(confirmRequest); err != nil {
		t.Fatalf("encode blocked confirm request: %v", err)
	}

	select {
	case <-modelClient.generateToolSeen:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected blocked loop task to start model execution")
	}

	detailRequest := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-detail-queued"`),
		Method:  "agent.task.detail.get",
		Params: mustMarshal(t, map[string]any{
			"task_id": taskB,
		}),
	}
	if err := encoder.Encode(detailRequest); err != nil {
		t.Fatalf("encode queued detail request: %v", err)
	}

	if err := right.SetReadDeadline(time.Now().Add(750 * time.Millisecond)); err != nil {
		t.Fatalf("set queued response deadline: %v", err)
	}
	defer func() {
		if err := right.SetReadDeadline(time.Time{}); err != nil {
			t.Fatalf("clear queued response deadline: %v", err)
		}
	}()

	queuedResponseSeen := false
	for index := 0; index < 12; index++ {
		var envelope map[string]any
		if err := decoder.Decode(&envelope); err != nil {
			break
		}
		responseID, _ := envelope["id"].(string)
		if responseID != "req-task-detail-queued" {
			continue
		}
		result, ok := envelope["result"].(map[string]any)
		if !ok {
			t.Fatalf("expected queued detail success envelope, got %+v", envelope)
		}
		data, ok := result["data"].(map[string]any)
		if !ok {
			t.Fatalf("expected queued detail response payload, got %+v", envelope)
		}
		task, ok := data["task"].(map[string]any)
		if !ok || task["task_id"] != taskB {
			t.Fatalf("expected queued detail response for %s, got %+v", taskB, envelope)
		}
		queuedResponseSeen = true
		break
	}

	close(modelClient.generateToolWait)

	if !queuedResponseSeen {
		t.Fatal("expected queued task detail request to complete before the blocked loop response")
	}
}

func TestHandleStreamConnSerializesConcurrentRequestsForSameTask(t *testing.T) {
	modelClient := &selectiveWaitLoopModelClient{
		stubLoopModelClient: stubLoopModelClient{
			toolResult: model.ToolCallResult{
				OutputText: "Same-task stream finished.",
			},
			generateToolWait: make(chan struct{}),
			generateToolSeen: make(chan struct{}),
		},
	}
	server := newTestServerWithModelClient(modelClient)

	result, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_pipe_same_task",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "inspect this workspace",
		},
	})
	if err != nil {
		t.Fatalf("seed task.start: %v", err)
	}
	taskID := result["task"].(map[string]any)["task_id"].(string)
	modelClient.blockedTaskID = taskID

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen loopback: %v", err)
	}
	defer listener.Close()

	acceptDone := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptDone <- err
			return
		}
		server.handleStreamConn(conn)
		acceptDone <- nil
	}()

	right, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial loopback: %v", err)
	}
	defer func() {
		_ = right.Close()
		select {
		case err := <-acceptDone:
			if err != nil && !errors.Is(err, net.ErrClosed) {
				t.Fatalf("accept loopback: %v", err)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("expected loopback stream to shut down")
		}
	}()

	encoder := json.NewEncoder(right)
	decoder := json.NewDecoder(right)
	envelopeCh := make(chan map[string]any, 32)
	decodeErrCh := make(chan error, 1)
	go func() {
		for {
			var envelope map[string]any
			if err := decoder.Decode(&envelope); err != nil {
				decodeErrCh <- err
				return
			}
			envelopeCh <- envelope
		}
	}()
	confirmRequest := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-loop-same-task"`),
		Method:  "agent.task.confirm",
		Params: mustMarshal(t, map[string]any{
			"task_id":   taskID,
			"confirmed": false,
			"corrected_intent": map[string]any{
				"name":      "agent_loop",
				"arguments": map[string]any{},
			},
		}),
	}
	if err := encoder.Encode(confirmRequest); err != nil {
		t.Fatalf("encode blocked confirm request: %v", err)
	}

	select {
	case <-modelClient.generateToolSeen:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected blocked same-task loop to start model execution")
	}

	detailRequest := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-detail-same-task"`),
		Method:  "agent.task.detail.get",
		Params: mustMarshal(t, map[string]any{
			"task_id": taskID,
		}),
	}
	if err := encoder.Encode(detailRequest); err != nil {
		t.Fatalf("encode same-task detail request: %v", err)
	}

	detailResponseSeenEarly := false
	earlyWindow := time.After(250 * time.Millisecond)
	for !detailResponseSeenEarly {
		select {
		case envelope := <-envelopeCh:
			responseID, _ := envelope["id"].(string)
			if responseID == "req-task-detail-same-task" {
				detailResponseSeenEarly = true
			}
		case err := <-decodeErrCh:
			t.Fatalf("decode same-task early envelope: %v", err)
		case <-earlyWindow:
			goto afterEarlyWindow
		}
	}

afterEarlyWindow:

	if detailResponseSeenEarly {
		t.Fatal("expected same-task detail request to wait until the blocked loop request finishes")
	}

	close(modelClient.generateToolWait)
	detailResponseSeen := false
	postUnblockDeadline := time.After(3 * time.Second)
	for !detailResponseSeen {
		select {
		case envelope := <-envelopeCh:
			responseID, _ := envelope["id"].(string)
			if responseID == "req-task-detail-same-task" {
				detailResponseSeen = true
			}
		case err := <-decodeErrCh:
			t.Fatalf("decode same-task post-unblock envelope: %v", err)
		case <-postUnblockDeadline:
			t.Fatal("expected same-task detail response after the blocked loop request completed")
		}
	}
}

func newTestServer() *Server {
	server, _, _ := newTestServerWithDependencies(nil)
	return server
}

func newTestServerWithModelClient(client model.Client) *Server {
	server, _, _ := newTestServerWithDependencies(client)
	return server
}

func newTestServerWithDependencies(client model.Client) (*Server, *tools.Registry, *plugin.Service) {
	toolRegistry := tools.NewRegistry()
	_ = builtin.RegisterBuiltinTools(toolRegistry)
	_ = sidecarclient.RegisterPlaywrightTools(toolRegistry)
	_ = sidecarclient.RegisterOCRTools(toolRegistry)
	_ = sidecarclient.RegisterMediaTools(toolRegistry)
	toolExecutor := tools.NewToolExecutor(toolRegistry)
	pathPolicy, _ := platform.NewLocalPathPolicy(filepath.Join("workspace", "rpc-test"))
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	pluginService := plugin.NewService()
	executionService := execution.NewService(
		fileSystem,
		stubExecutionCapability{result: tools.CommandExecutionResult{Stdout: "ok", ExitCode: 0}},
		sidecarclient.NewNoopPlaywrightSidecarClient(),
		sidecarclient.NewNoopOCRWorkerClient(),
		sidecarclient.NewNoopMediaWorkerClient(),
		sidecarclient.NewNoopScreenCaptureClient(),
		model.NewService(serviceconfig.ModelConfig{Provider: "openai_responses", ModelID: "gpt-5.4", Endpoint: "https://api.openai.com/v1/responses"}, client),
		audit.NewService(),
		checkpoint.NewService(),
		delivery.NewService(),
		toolRegistry,
		toolExecutor,
		pluginService,
	)
	orch := orchestrator.NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(serviceconfig.ModelConfig{
			Provider: "openai_responses",
			ModelID:  "gpt-5.4",
			Endpoint: "https://api.openai.com/v1/responses",
		}),
		toolRegistry,
		pluginService,
	).WithExecutor(executionService)

	server := NewServer(serviceconfig.RPCConfig{
		Transport:        "named_pipe",
		NamedPipeName:    `\\.\pipe\cialloclaw-rpc-test`,
		DebugHTTPAddress: ":0",
	}, orch)
	server.now = func() time.Time {
		return time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC)
	}
	return server, toolRegistry, pluginService
}

func mustMarshal(t *testing.T, value any) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal request params: %v", err)
	}
	return encoded
}

func numericValue(t *testing.T, value any) int {
	t.Helper()
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		t.Fatalf("expected numeric value, got %#v", value)
		return 0
	}
}
