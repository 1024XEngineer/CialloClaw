// RPC server tests verify response envelopes and notification behavior.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http/httptest"
	"path/filepath"
	"strings"
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
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskinspector"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/builtin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/sidecarclient"
)

type stubLoopModelClient struct {
	toolResult       model.ToolCallResult
	generateToolWait chan struct{}
	generateToolSeen chan struct{}
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

type selectiveWaitLoopModelClient struct {
	stubLoopModelClient
	blockedTaskID string
}

func (s *selectiveWaitLoopModelClient) GenerateText(ctx context.Context, request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
	return s.stubLoopModelClient.GenerateText(ctx, request)
}

func (s *selectiveWaitLoopModelClient) GenerateToolCalls(_ context.Context, request model.ToolCallRequest) (model.ToolCallResult, error) {
	if s.generateToolSeen != nil && request.TaskID == s.blockedTaskID {
		select {
		case <-s.generateToolSeen:
		default:
			close(s.generateToolSeen)
		}
	}
	if s.generateToolWait != nil && request.TaskID == s.blockedTaskID {
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
	modelClient := &selectiveWaitLoopModelClient{
		stubLoopModelClient: stubLoopModelClient{
			toolResult: model.ToolCallResult{
				OutputText: "Scoped runtime finished.",
			},
			generateToolWait: make(chan struct{}),
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

	taskA := startTask("sess_loop_scope_a")
	taskB := startTask("sess_loop_scope_b")
	modelClient.blockedTaskID = taskA

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

	if _, err := server.orchestrator.TaskSteer(map[string]any{
		"task_id": taskB,
		"message": "Mention the unrelated steering marker.",
	}); err != nil {
		t.Fatalf("queue steering for unrelated task: %v", err)
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

	select {
	case err := <-confirmDone:
		if err != nil {
			t.Fatalf("confirm unrelated task: %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected unrelated task confirmation to complete")
	}

	close(modelClient.generateToolWait)
}

func TestDispatchTaskStartIgnoresUnsupportedIntentField(t *testing.T) {
	server := newTestServer()

	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-start-ignore-intent"`),
		Method:  "agent.task.start",
		Params: mustMarshal(t, map[string]any{
			"session_id": "sess_ignore_intent",
			"source":     "floating_ball",
			"trigger":    "text_selected_click",
			"input": map[string]any{
				"type": "text_selection",
				"text": "select this content",
			},
			"intent": map[string]any{
				"name": "write_file",
				"arguments": map[string]any{
					"require_authorization": true,
				},
			},
		}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	task := success.Result.Data.(map[string]any)["task"].(map[string]any)
	if task["status"] != "confirming_intent" {
		t.Fatalf("expected task.start to stay in confirming_intent when intent is stripped, got %+v", task)
	}
	intentValue, ok := task["intent"].(map[string]any)
	if !ok || intentValue["name"] != "agent_loop" {
		t.Fatalf("expected task.start to rely on backend suggestion instead of request intent, got %+v", task["intent"])
	}
}

// TestHandleDebugEventsReturnsQueuedNotifications verifies that queued
// notifications can be fetched through the debug events endpoint.
func TestHandleDebugEventsReturnsQueuedNotifications(t *testing.T) {
	server := newTestServer()
	result, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "直接总结这段文字",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task: %v", err)
	}

	taskID := result["task"].(map[string]any)["task_id"].(string)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/events?task_id="+taskID, nil)
	server.handleDebugEvents(recorder, request)

	if recorder.Code != 200 {
		t.Fatalf("expected 200 status, got %d", recorder.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	items := payload["items"].([]any)
	if len(items) == 0 {
		t.Fatal("expected queued notifications to be returned")
	}
}

func TestDispatchTaskDetailGetIncludesActiveApprovalAnchor(t *testing.T) {
	server := newTestServer()

	startResult, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_detail_rpc",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "rpc task detail should expose active approval anchor",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start task: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-detail-anchor"`),
		Method:  "agent.task.detail.get",
		Params: mustMarshal(t, map[string]any{
			"task_id": taskID,
		}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	approvalRequest, ok := data["approval_request"].(map[string]any)
	if !ok {
		t.Fatalf("expected approval_request in rpc result, got %+v", data["approval_request"])
	}
	if approvalRequest["task_id"] != taskID {
		t.Fatalf("expected approval_request task_id %s, got %+v", taskID, approvalRequest)
	}

	securitySummary := data["security_summary"].(map[string]any)
	if numericValue(t, securitySummary["pending_authorizations"]) != 1 {
		t.Fatalf("expected pending_authorizations 1 in rpc result, got %+v", securitySummary["pending_authorizations"])
	}
	if securitySummary["latest_restore_point"] != nil {
		t.Fatalf("expected latest_restore_point nil in rpc result, got %+v", securitySummary["latest_restore_point"])
	}
}

func TestDispatchTaskDetailGetOmitsApprovalAnchorForCompletedTask(t *testing.T) {
	server := newTestServer()

	startResult, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_detail_rpc_done",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "rpc task detail should omit anchor for completed task",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	if _, ok := server.orchestrator.RunEngine().CompleteTask(taskID, map[string]any{"type": "task_detail", "payload": map[string]any{"task_id": taskID}}, map[string]any{"task_id": taskID, "type": "result", "text": "done"}, nil); !ok {
		t.Fatal("expected runtime task completion to succeed")
	}
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-detail-no-anchor"`),
		Method:  "agent.task.detail.get",
		Params: mustMarshal(t, map[string]any{
			"task_id": taskID,
		}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	if data["approval_request"] != nil {
		t.Fatalf("expected approval_request to be nil for completed task, got %+v", data["approval_request"])
	}

	securitySummary := data["security_summary"].(map[string]any)
	if numericValue(t, securitySummary["pending_authorizations"]) != 0 {
		t.Fatalf("expected pending_authorizations 0 in rpc result, got %+v", securitySummary["pending_authorizations"])
	}
	if _, ok := securitySummary["latest_restore_point"].(map[string]any); !ok {
		t.Fatalf("expected latest_restore_point object for completed task, got %+v", securitySummary["latest_restore_point"])
	}
}

func TestDispatchMapsTaskControlStatusErrors(t *testing.T) {
	server := newTestServer()

	startResult, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "still waiting for intent confirmation",
		},
	})
	if err != nil {
		t.Fatalf("start task: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-control-invalid"`),
		Method:  "agent.task.control",
		Params: mustMarshal(t, map[string]any{
			"task_id": taskID,
			"action":  "pause",
		}),
	})

	errEnvelope, ok := response.(errorEnvelope)
	if !ok {
		t.Fatalf("expected error response envelope, got %#v", response)
	}
	if errEnvelope.Error.Code != 1001004 || errEnvelope.Error.Message != "TASK_STATUS_INVALID" {
		t.Fatalf("expected TASK_STATUS_INVALID mapping, got code=%d message=%s", errEnvelope.Error.Code, errEnvelope.Error.Message)
	}
}

func TestDispatchMapsTaskControlFinishedErrors(t *testing.T) {
	server := newTestServer()

	startResult, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "completed task for rpc error mapping",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	if _, ok := server.orchestrator.RunEngine().CompleteTask(taskID, map[string]any{"type": "task_detail", "payload": map[string]any{"task_id": taskID}}, map[string]any{"task_id": taskID, "type": "result", "text": "done"}, nil); !ok {
		t.Fatal("expected runtime task completion to succeed")
	}
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-control-finished"`),
		Method:  "agent.task.control",
		Params: mustMarshal(t, map[string]any{
			"task_id": taskID,
			"action":  "cancel",
		}),
	})

	errEnvelope, ok := response.(errorEnvelope)
	if !ok {
		t.Fatalf("expected error response envelope, got %#v", response)
	}
	if errEnvelope.Error.Code != 1001005 || errEnvelope.Error.Message != "TASK_ALREADY_FINISHED" {
		t.Fatalf("expected TASK_ALREADY_FINISHED mapping, got code=%d message=%s", errEnvelope.Error.Code, errEnvelope.Error.Message)
	}
}

func TestDispatchReturnsSecurityAuditList(t *testing.T) {
	server := newTestServer()
	storageService := storage.NewService(platform.NewLocalStorageAdapter(filepath.Join(t.TempDir(), "audit.db")))
	defer func() { _ = storageService.Close() }()
	server.orchestrator.WithStorage(storageService)
	err := storageService.AuditWriter().WriteAuditRecord(context.Background(), audit.Record{
		AuditID:   "audit_001",
		TaskID:    "task_001",
		Type:      "file",
		Action:    "write_file",
		Summary:   "stored audit record",
		Target:    "workspace/result.md",
		Result:    "success",
		CreatedAt: "2026-04-08T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("write audit record: %v", err)
	}

	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-security-audit-list"`),
		Method:  "agent.security.audit.list",
		Params: mustMarshal(t, map[string]any{
			"task_id": "task_001",
			"limit":   20,
			"offset":  0,
		}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	items := success.Result.Data.(map[string]any)["items"].([]map[string]any)
	if len(items) != 1 {
		t.Fatalf("expected one audit item, got %d", len(items))
	}
	if items[0]["audit_id"] != "audit_001" {
		t.Fatalf("expected stored audit_001, got %+v", items[0])
	}
}

func TestDispatchReturnsSecurityRestorePointsList(t *testing.T) {
	server := newTestServer()
	storageService := storage.NewService(platform.NewLocalStorageAdapter(filepath.Join(t.TempDir(), "restore.db")))
	defer func() { _ = storageService.Close() }()
	server.orchestrator.WithStorage(storageService)
	err := storageService.RecoveryPointWriter().WriteRecoveryPoint(context.Background(), checkpoint.RecoveryPoint{
		RecoveryPointID: "rp_001",
		TaskID:          "task_001",
		Summary:         "stored recovery point",
		CreatedAt:       "2026-04-08T10:00:00Z",
		Objects:         []string{"workspace/result.md"},
	})
	if err != nil {
		t.Fatalf("write recovery point: %v", err)
	}

	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-security-restore-points-list"`),
		Method:  "agent.security.restore_points.list",
		Params: mustMarshal(t, map[string]any{
			"task_id": "task_001",
			"limit":   20,
			"offset":  0,
		}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	items := success.Result.Data.(map[string]any)["items"].([]map[string]any)
	if len(items) != 1 {
		t.Fatalf("expected one recovery point item, got %d", len(items))
	}
	if items[0]["recovery_point_id"] != "rp_001" {
		t.Fatalf("expected stored rp_001, got %+v", items[0])
	}
}

func TestDispatchReturnsSecurityRestoreApplyResult(t *testing.T) {
	server := newTestServer()
	storageService := storage.NewService(platform.NewLocalStorageAdapter(filepath.Join(t.TempDir(), "restore-apply.db")))
	defer func() { _ = storageService.Close() }()
	server.orchestrator.WithStorage(storageService)
	startResult, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_restore",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "restore runtime task",
		},
	})
	if err != nil {
		t.Fatalf("start task: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)

	err = storageService.RecoveryPointWriter().WriteRecoveryPoint(context.Background(), checkpoint.RecoveryPoint{
		RecoveryPointID: "rp_001",
		TaskID:          taskID,
		Summary:         "stored recovery point",
		CreatedAt:       "2026-04-08T10:00:00Z",
		Objects:         []string{"workspace/result.md"},
	})
	if err != nil {
		t.Fatalf("write recovery point: %v", err)
	}

	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-security-restore-apply"`),
		Method:  "agent.security.restore.apply",
		Params: mustMarshal(t, map[string]any{
			"task_id":           taskID,
			"recovery_point_id": "rp_001",
		}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	if _, ok := data["applied"].(bool); !ok {
		t.Fatalf("expected applied flag in restore result, got %+v", data)
	}
	if data["task"].(map[string]any)["status"] != "waiting_auth" {
		t.Fatalf("expected restore apply rpc to enter waiting_auth, got %+v", data)
	}
	if data["recovery_point"].(map[string]any)["recovery_point_id"] != "rp_001" {
		t.Fatalf("expected rp_001 restore result, got %+v", data)
	}
}

func TestDispatchReturnsNotepadUpdateResult(t *testing.T) {
	server := newTestServer()

	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-notepad-update"`),
		Method:  "agent.notepad.update",
		Params: mustMarshal(t, map[string]any{
			"item_id": "todo_002",
			"action":  "move_upcoming",
		}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}

	data := success.Result.Data.(map[string]any)
	notepadItem, ok := data["notepad_item"].(map[string]any)
	if !ok {
		t.Fatalf("expected notepad_item payload, got %+v", data)
	}
	if notepadItem["bucket"] != "upcoming" {
		t.Fatalf("expected updated notepad item bucket upcoming, got %+v", notepadItem)
	}
	refreshGroups := data["refresh_groups"].([]string)
	if len(refreshGroups) != 2 || refreshGroups[0] != "later" || refreshGroups[1] != "upcoming" {
		t.Fatalf("expected refresh_groups to include source and target buckets, got %+v", refreshGroups)
	}
}

func TestDispatchReturnsTaskArtifactList(t *testing.T) {
	server := newTestServer()
	storageService := storage.NewService(platform.NewLocalStorageAdapter(filepath.Join(t.TempDir(), "artifact-list.db")))
	defer func() { _ = storageService.Close() }()
	server.orchestrator.WithStorage(storageService)
	err := storageService.ArtifactStore().SaveArtifacts(context.Background(), []storage.ArtifactRecord{{
		ArtifactID:          "art_rpc_001",
		TaskID:              "task_rpc_001",
		ArtifactType:        "generated_doc",
		Title:               "rpc-artifact.md",
		Path:                "workspace/rpc-artifact.md",
		MimeType:            "text/markdown",
		DeliveryType:        "workspace_document",
		DeliveryPayloadJSON: `{"path":"workspace/rpc-artifact.md","task_id":"task_rpc_001"}`,
		CreatedAt:           "2026-04-14T10:00:00Z",
	}})
	if err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-artifact-list"`),
		Method:  "agent.task.artifact.list",
		Params: mustMarshal(t, map[string]any{
			"task_id": "task_rpc_001",
			"limit":   20,
			"offset":  0,
		}),
	})
	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	items := success.Result.Data.(map[string]any)["items"].([]map[string]any)
	if len(items) != 1 || items[0]["artifact_id"] != "art_rpc_001" {
		t.Fatalf("expected artifact list item, got %+v", items)
	}
}

func TestDispatchReturnsTaskArtifactOpen(t *testing.T) {
	server := newTestServer()
	storageService := storage.NewService(platform.NewLocalStorageAdapter(filepath.Join(t.TempDir(), "artifact-open.db")))
	defer func() { _ = storageService.Close() }()
	server.orchestrator.WithStorage(storageService)
	err := storageService.ArtifactStore().SaveArtifacts(context.Background(), []storage.ArtifactRecord{{
		ArtifactID:          "art_rpc_open_001",
		TaskID:              "task_rpc_open_001",
		ArtifactType:        "generated_doc",
		Title:               "rpc-open.md",
		Path:                "workspace/rpc-open.md",
		MimeType:            "text/markdown",
		DeliveryType:        "open_file",
		DeliveryPayloadJSON: `{"path":"workspace/rpc-open.md","task_id":"task_rpc_open_001"}`,
		CreatedAt:           "2026-04-14T10:05:00Z",
	}})
	if err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-artifact-open"`),
		Method:  "agent.task.artifact.open",
		Params: mustMarshal(t, map[string]any{
			"task_id":     "task_rpc_open_001",
			"artifact_id": "art_rpc_open_001",
		}),
	})
	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	if data["open_action"] != "open_file" {
		t.Fatalf("expected open_file action, got %+v", data)
	}
	if data["artifact"].(map[string]any)["artifact_id"] != "art_rpc_open_001" {
		t.Fatalf("expected opened artifact, got %+v", data)
	}
}

func TestDispatchMapsArtifactNotFoundErrors(t *testing.T) {
	_, rpcErr := wrapOrchestratorResult(nil, orchestrator.ErrArtifactNotFound)
	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}
	if rpcErr.Code != 1005002 || rpcErr.Message != "ARTIFACT_NOT_FOUND" {
		t.Fatalf("expected ARTIFACT_NOT_FOUND mapping, got code=%d message=%s", rpcErr.Code, rpcErr.Message)
	}
}

func TestDispatchReturnsDeliveryOpenForArtifact(t *testing.T) {
	server := newTestServer()
	storageService := storage.NewService(platform.NewLocalStorageAdapter(filepath.Join(t.TempDir(), "delivery-open-artifact.db")))
	defer func() { _ = storageService.Close() }()
	server.orchestrator.WithStorage(storageService)
	err := storageService.ArtifactStore().SaveArtifacts(context.Background(), []storage.ArtifactRecord{{
		ArtifactID:          "art_delivery_rpc_001",
		TaskID:              "task_delivery_rpc_001",
		ArtifactType:        "generated_doc",
		Title:               "delivery-rpc.md",
		Path:                "workspace/delivery-rpc.md",
		MimeType:            "text/markdown",
		DeliveryType:        "open_file",
		DeliveryPayloadJSON: `{"path":"workspace/delivery-rpc.md","task_id":"task_delivery_rpc_001"}`,
		CreatedAt:           "2026-04-14T10:10:00Z",
	}})
	if err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-delivery-open-artifact"`),
		Method:  "agent.delivery.open",
		Params: mustMarshal(t, map[string]any{
			"task_id":     "task_delivery_rpc_001",
			"artifact_id": "art_delivery_rpc_001",
		}),
	})
	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	if data["open_action"] != "open_file" {
		t.Fatalf("expected open_file action, got %+v", data)
	}
}

func TestDispatchReturnsDeliveryOpenForTaskResult(t *testing.T) {
	server := newTestServer()
	startResult, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_delivery_rpc",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请整理成文档",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-delivery-open-task"`),
		Method:  "agent.delivery.open",
		Params: mustMarshal(t, map[string]any{
			"task_id": taskID,
		}),
	})
	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	if data["open_action"] != "task_detail" {
		t.Fatalf("expected task_detail action, got %+v", data)
	}
}

func TestDispatchMapsSecurityAuditListStorageErrors(t *testing.T) {
	_, rpcErr := wrapOrchestratorResult(nil, orchestrator.ErrStorageQueryFailed)
	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}
	if rpcErr.Code != 1005001 || rpcErr.Message != "SQLITE_WRITE_FAILED" {
		t.Fatalf("expected SQLITE_WRITE_FAILED mapping, got code=%d message=%s", rpcErr.Code, rpcErr.Message)
	}
}

func TestDispatchMapsWrappedStructuredStoreErrors(t *testing.T) {
	_, rpcErr := wrapOrchestratorResult(nil, fmt.Errorf("settings snapshot write failed: %w", storage.ErrStructuredStoreUnavailable))
	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}
	if rpcErr.Code != 1005001 || rpcErr.Message != "SQLITE_WRITE_FAILED" {
		t.Fatalf("expected wrapped structured store error to map to SQLITE_WRITE_FAILED, got code=%d message=%s", rpcErr.Code, rpcErr.Message)
	}
}

func TestDispatchMapsRecoveryPointNotFoundErrors(t *testing.T) {
	_, rpcErr := wrapOrchestratorResult(nil, orchestrator.ErrRecoveryPointNotFound)
	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}
	if rpcErr.Code != 1005006 || rpcErr.Message != "RECOVERY_POINT_NOT_FOUND" {
		t.Fatalf("expected RECOVERY_POINT_NOT_FOUND mapping, got code=%d message=%s", rpcErr.Code, rpcErr.Message)
	}
}

func TestDispatchMapsStrongholdErrors(t *testing.T) {
	_, rpcErr := wrapOrchestratorResult(nil, orchestrator.ErrStrongholdAccessFailed)
	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}
	if rpcErr.Code != 1005004 || rpcErr.Message != "STRONGHOLD_ACCESS_FAILED" {
		t.Fatalf("expected STRONGHOLD_ACCESS_FAILED mapping, got code=%d message=%s", rpcErr.Code, rpcErr.Message)
	}
}

func TestDispatchMapsModelProviderErrors(t *testing.T) {
	_, rpcErr := wrapOrchestratorResult(nil, model.ErrModelProviderUnsupported)
	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}
	if rpcErr.Code != 1008001 || rpcErr.Message != "MODEL_PROVIDER_NOT_FOUND" {
		t.Fatalf("expected MODEL_PROVIDER_NOT_FOUND mapping, got code=%d message=%s", rpcErr.Code, rpcErr.Message)
	}
}

func TestDispatchMapsModelConfigurationErrors(t *testing.T) {
	_, rpcErr := wrapOrchestratorResult(nil, model.ErrOpenAIEndpointRequired)
	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}
	if rpcErr.Code != 1008002 || rpcErr.Message != "MODEL_NOT_ALLOWED" {
		t.Fatalf("expected MODEL_NOT_ALLOWED mapping, got code=%d message=%s", rpcErr.Code, rpcErr.Message)
	}
}

func TestDispatchMapsModelRuntimeUnavailableErrors(t *testing.T) {
	_, rpcErr := wrapOrchestratorResult(nil, model.ErrOpenAIRequestTimeout)
	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}
	if rpcErr.Code != 1008003 || rpcErr.Message != "MODEL_RUNTIME_UNAVAILABLE" {
		t.Fatalf("expected MODEL_RUNTIME_UNAVAILABLE mapping, got code=%d message=%s", rpcErr.Code, rpcErr.Message)
	}

	_, rpcErr = wrapOrchestratorResult(nil, &model.OpenAIHTTPStatusError{StatusCode: 503, Message: "upstream unavailable"})
	if rpcErr == nil || rpcErr.Code != 1008003 {
		t.Fatalf("expected 5xx provider failure to map as runtime unavailable, got %+v", rpcErr)
	}
}

func TestDispatchMapsToolOutputInvalidExplicitly(t *testing.T) {
	_, rpcErr := wrapOrchestratorResult(nil, tools.ErrToolOutputInvalid)
	if rpcErr == nil {
		t.Fatal("expected rpc error")
	}
	if rpcErr.Code != 1003004 || rpcErr.Message != "TOOL_OUTPUT_INVALID" {
		t.Fatalf("expected TOOL_OUTPUT_INVALID mapping, got code=%d message=%s", rpcErr.Code, rpcErr.Message)
	}
}

func TestDispatchMapsTaskInspectorSourceErrorsExplicitly(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		code    int
		message string
	}{
		{name: "outside workspace", err: fmt.Errorf("wrapped: %w", taskinspector.ErrInspectionSourceOutsideWorkspace), code: 1004003, message: "WORKSPACE_BOUNDARY_DENIED"},
		{name: "filesystem unavailable", err: fmt.Errorf("wrapped: %w", taskinspector.ErrInspectionFileSystemUnavailable), code: 1007006, message: "INSPECTION_FILESYSTEM_UNAVAILABLE"},
		{name: "source not found", err: fmt.Errorf("wrapped: %w", taskinspector.ErrInspectionSourceNotFound), code: 1007007, message: "INSPECTION_SOURCE_NOT_FOUND"},
		{name: "source unreadable", err: fmt.Errorf("wrapped: %w", taskinspector.ErrInspectionSourceUnreadable), code: 1007008, message: "INSPECTION_SOURCE_UNREADABLE"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, rpcErr := wrapOrchestratorResult(nil, test.err)
			if rpcErr == nil {
				t.Fatal("expected rpc error")
			}
			if rpcErr.Code != test.code || rpcErr.Message != test.message {
				t.Fatalf("expected %s mapping, got code=%d message=%s", test.message, rpcErr.Code, rpcErr.Message)
			}
		})
	}
}

func TestDispatchReturnsFormalTaskInspectorRunSourceErrors(t *testing.T) {
	server := newTestServer()
	pathPolicy, err := platform.NewLocalPathPolicy(filepath.Join(t.TempDir(), "rpc-task-inspector"))
	if err != nil {
		t.Fatalf("NewLocalPathPolicy returned error: %v", err)
	}
	server.orchestrator.WithTaskInspector(taskinspector.NewService(platform.NewLocalFileSystemAdapter(pathPolicy)))
	tests := []struct {
		name          string
		requestID     string
		targetSources []any
		expectCode    int
		expectMessage string
	}{
		{name: "missing source", requestID: "req-task-inspector-missing", targetSources: []any{"workspace/missing"}, expectCode: 1007007, expectMessage: "INSPECTION_SOURCE_NOT_FOUND"},
		{name: "outside workspace", requestID: "req-task-inspector-outside", targetSources: []any{"../outside"}, expectCode: 1004003, expectMessage: "WORKSPACE_BOUNDARY_DENIED"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := server.dispatch(requestEnvelope{
				JSONRPC: "2.0",
				ID:      json.RawMessage(fmt.Sprintf(`"%s"`, test.requestID)),
				Method:  "agent.task_inspector.run",
				Params: mustMarshal(t, map[string]any{
					"target_sources": test.targetSources,
				}),
			})
			errEnvelope, ok := response.(errorEnvelope)
			if !ok {
				t.Fatalf("expected error response envelope, got %#v", response)
			}
			if errEnvelope.Error.Code != test.expectCode || errEnvelope.Error.Message != test.expectMessage {
				t.Fatalf("expected %s to map to formal rpc error, got %+v", test.expectMessage, errEnvelope.Error)
			}
		})
	}
}

func TestHandlerWrappersCoverRecommendationInspectorDashboardAndSecurityMethods(t *testing.T) {
	server := newTestServer()
	handlerCalls := []struct {
		name   string
		invoke func() (any, *rpcError)
	}{
		{name: "recommendation.get", invoke: func() (any, *rpcError) { return server.handleAgentRecommendationGet(map[string]any{}) }},
		{name: "recommendation.feedback.submit", invoke: func() (any, *rpcError) { return server.handleAgentRecommendationFeedbackSubmit(map[string]any{}) }},
		{name: "task_inspector.config.get", invoke: func() (any, *rpcError) { return server.handleAgentTaskInspectorConfigGet(nil) }},
		{name: "task_inspector.config.update", invoke: func() (any, *rpcError) {
			return server.handleAgentTaskInspectorConfigUpdate(map[string]any{"task_sources": []any{"D:/workspace/todos"}, "inspection_interval": map[string]any{"unit": "minute", "value": 10}})
		}},
		{name: "task_inspector.run", invoke: func() (any, *rpcError) { return server.handleAgentTaskInspectorRun(map[string]any{}) }},
		{name: "notepad.list", invoke: func() (any, *rpcError) { return server.handleAgentNotepadList(map[string]any{}) }},
		{name: "notepad.convert_to_task", invoke: func() (any, *rpcError) {
			return server.handleAgentNotepadConvertToTask(map[string]any{"item_id": "missing"})
		}},
		{name: "dashboard.overview.get", invoke: func() (any, *rpcError) { return server.handleAgentDashboardOverviewGet(map[string]any{}) }},
		{name: "dashboard.module.get", invoke: func() (any, *rpcError) { return server.handleAgentDashboardModuleGet(map[string]any{"module": "task"}) }},
		{name: "mirror.overview.get", invoke: func() (any, *rpcError) { return server.handleAgentMirrorOverviewGet(map[string]any{}) }},
		{name: "security.summary.get", invoke: func() (any, *rpcError) { return server.handleAgentSecuritySummaryGet(nil) }},
		{name: "security.pending.list", invoke: func() (any, *rpcError) { return server.handleAgentSecurityPendingList(map[string]any{}) }},
		{name: "security.respond", invoke: func() (any, *rpcError) {
			return server.handleAgentSecurityRespond(map[string]any{"task_id": "missing", "decision": "approve"})
		}},
		{name: "settings.model.validate", invoke: func() (any, *rpcError) { return server.handleAgentSettingsModelValidate(map[string]any{}) }},
	}
	for _, call := range handlerCalls {
		data, rpcErr := call.invoke()
		if data == nil && rpcErr == nil {
			t.Fatalf("expected %s handler to return either data or rpc error", call.name)
		}
		if rpcErr != nil && rpcErr.TraceID == "" {
			t.Fatalf("expected %s handler rpc error to include trace id, got %+v", call.name, rpcErr)
		}
	}
}

func TestJSONRPCHandlerWrappersCoverPrimitiveDecodersAndTracingHelpers(t *testing.T) {
	req := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`1`),
		Method:  "agent.settings.update",
		Params:  mustMarshal(t, map[string]any{"request_meta": map[string]any{"trace_id": "trace_rpc_helpers"}, "enabled": true, "count": 3, "labels": []string{"a", "b"}}),
	}
	decoded, err := decodeRequest(strings.NewReader(string(mustMarshal(t, req))))
	if err != nil {
		t.Fatalf("decodeRequest returned error: %v", err)
	}
	params, err := decodeParams(decoded.Params)
	if err != nil {
		t.Fatalf("decodeParams returned error: %v", err)
	}
	if requestTraceID(params) != "trace_rpc_helpers" || traceIDFromRequest(decoded.Params) != "trace_rpc_helpers" {
		t.Fatalf("expected trace helpers to extract request trace ids, params=%+v", params)
	}
	if !boolValue(params, "enabled", false) || intValue(params, "count", 0) != 3 {
		t.Fatalf("expected primitive decoders to round-trip values, params=%+v", params)
	}
	labels := stringSliceValue(params["labels"])
	if len(labels) != 2 || labels[1] != "b" {
		t.Fatalf("expected stringSliceValue to decode labels, got %+v", labels)
	}
	if boolValue(nil, "enabled", false) || intValue(nil, "count", 0) != 0 || len(stringSliceValue(map[string]any{"labels": 7}["labels"])) != 0 {
		t.Fatal("expected primitive decoders to handle nil and invalid inputs")
	}
	if _, err := decodeRequest(strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"agent.settings.get","params":`)); err == nil {
		t.Fatal("expected malformed request to fail decodeRequest")
	}
	if _, err := decodeParams(json.RawMessage(`{"broken":`)); err == nil {
		t.Fatal("expected malformed params to fail decodeParams")
	}
}

func TestDispatchReturnsSettingsGet(t *testing.T) {
	server := newTestServer()
	storageService := storage.NewService(testStorageAdapter{databasePath: filepath.Join(t.TempDir(), "settings-get.db")})
	defer func() { _ = storageService.Close() }()
	server.orchestrator.WithStorage(storageService)
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-settings-get"`),
		Method:  "agent.settings.get",
		Params:  mustMarshal(t, map[string]any{"scope": "all"}),
	})
	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	models := success.Result.Data.(map[string]any)["settings"].(map[string]any)["models"].(map[string]any)
	credentials := models["credentials"].(map[string]any)
	if _, ok := credentials["stronghold"].(map[string]any); !ok {
		t.Fatalf("expected settings get to include stronghold status, got %+v", credentials)
	}
}

func TestDispatchReturnsSettingsUpdate(t *testing.T) {
	server := newTestServer()
	storageService := storage.NewService(testStorageAdapter{databasePath: filepath.Join(t.TempDir(), "settings-update.db")})
	defer func() { _ = storageService.Close() }()
	server.orchestrator.WithStorage(storageService)
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-settings-update"`),
		Method:  "agent.settings.update",
		Params: mustMarshal(t, map[string]any{
			"models": map[string]any{
				"provider": "openai",
				"api_key":  "rpc-secret-key",
			},
		}),
	})
	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	models := success.Result.Data.(map[string]any)["effective_settings"].(map[string]any)["models"].(map[string]any)
	if models["provider_api_key_configured"] != true {
		t.Fatalf("expected settings update to mark provider key configured, got %+v", models)
	}
	if _, exists := models["api_key"]; exists {
		t.Fatalf("expected settings update response to keep api_key redacted, got %+v", models)
	}
	if success.Result.Data.(map[string]any)["apply_mode"] != "next_task_effective" || success.Result.Data.(map[string]any)["need_restart"] != false {
		t.Fatalf("expected model settings update to be next_task_effective, got %+v", success.Result.Data)
	}
}

func TestDispatchReturnsSettingsModelValidate(t *testing.T) {
	server := newTestServer()
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-settings-model-validate"`),
		Method:  "agent.settings.model.validate",
		Params: mustMarshal(t, map[string]any{
			"models": map[string]any{
				"provider": "openai",
				"base_url": "https://api.example.com/v1",
				"model":    "gpt-4.1-mini",
			},
		}),
	})
	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	if data["ok"] != false || data["status"] != "missing_api_key" {
		t.Fatalf("expected structured validation failure result, got %+v", data)
	}
}

func TestDispatchReturnsPluginRuntimeList(t *testing.T) {
	server := newTestServer()
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-plugin-runtime-list"`),
		Method:  "agent.plugin.runtime.list",
		Params:  mustMarshal(t, map[string]any{}),
	})
	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	items := data["items"].([]map[string]any)
	if len(items) == 0 {
		t.Fatalf("expected plugin runtime query to return declared runtimes, got %+v", data)
	}
	manifest, ok := items[0]["manifest"].(map[string]any)
	if !ok || manifest["plugin_id"] == nil || manifest["source"] == nil {
		t.Fatalf("expected plugin runtime items to include formal manifest linkage, got %+v", items[0])
	}
	metrics := data["metrics"].([]map[string]any)
	if len(metrics) == 0 {
		t.Fatalf("expected plugin runtime query to return metric snapshots, got %+v", data)
	}
}

func TestDispatchReturnsPluginList(t *testing.T) {
	server := newTestServer()
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-plugin-list"`),
		Method:  "agent.plugin.list",
		Params: mustMarshal(t, map[string]any{
			"query": "ocr",
			"page":  map[string]any{"limit": 10, "offset": 0},
		}),
	})
	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	items := data["items"].([]map[string]any)
	if len(items) != 1 || items[0]["plugin_id"] != "ocr" {
		t.Fatalf("expected plugin list query to return filtered ocr plugin, got %+v", data)
	}
}

func TestDispatchReturnsPluginDetail(t *testing.T) {
	server, toolRegistry, pluginService := newTestServerWithDependencies(nil)
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-plugin-detail"`),
		Method:  "agent.plugin.detail.get",
		Params: mustMarshal(t, map[string]any{
			"plugin_id": "ocr",
		}),
	})
	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	if data["plugin"].(map[string]any)["plugin_id"] != "ocr" {
		t.Fatalf("expected plugin detail query to resolve ocr plugin, got %+v", data)
	}
	runtime, ok := pluginService.RuntimeState(plugin.RuntimeKindWorker, "ocr_worker")
	if !ok {
		t.Fatalf("expected ocr worker runtime to exist")
	}
	toolItems := data["tools"].([]map[string]any)
	if len(toolItems) != len(runtime.Capabilities) {
		t.Fatalf("expected plugin detail query to return one contract per declared capability, got %+v", data)
	}
	for _, item := range toolItems {
		toolName := item["tool_name"].(string)
		tool, err := toolRegistry.Get(toolName)
		if err != nil {
			t.Fatalf("expected tool %q to exist in registry: %v", toolName, err)
		}
		metadata := tool.Metadata()
		if item["display_name"] != metadata.DisplayName || item["source"] != string(metadata.Source) {
			t.Fatalf("expected plugin detail payload to mirror registry metadata for %q, got %+v", toolName, item)
		}
	}
}

func TestDispatchMapsTaskControlInvalidActionToInvalidParams(t *testing.T) {
	server := newTestServer()

	startResult, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "task for invalid action",
		},
	})
	if err != nil {
		t.Fatalf("start task: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-control-invalid-action"`),
		Method:  "agent.task.control",
		Params: mustMarshal(t, map[string]any{
			"task_id": taskID,
			"action":  "skip",
		}),
	})

	errEnvelope, ok := response.(errorEnvelope)
	if !ok {
		t.Fatalf("expected error response envelope, got %#v", response)
	}
	if errEnvelope.Error.Code != 1002001 || errEnvelope.Error.Message != "INVALID_PARAMS" {
		t.Fatalf("expected INVALID_PARAMS mapping for unsupported action, got code=%d message=%s", errEnvelope.Error.Code, errEnvelope.Error.Message)
	}
}

func TestDispatchMapsTaskControlMissingTaskIDToInvalidParams(t *testing.T) {
	server := newTestServer()

	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-control-missing-task-id"`),
		Method:  "agent.task.control",
		Params: mustMarshal(t, map[string]any{
			"action": "pause",
		}),
	})

	errEnvelope, ok := response.(errorEnvelope)
	if !ok {
		t.Fatalf("expected error response envelope, got %#v", response)
	}
	if errEnvelope.Error.Code != 1002001 || errEnvelope.Error.Message != "INVALID_PARAMS" {
		t.Fatalf("expected INVALID_PARAMS mapping for missing task_id, got code=%d message=%s", errEnvelope.Error.Code, errEnvelope.Error.Message)
	}
}

func TestDispatchMapsTaskControlMissingActionToInvalidParams(t *testing.T) {
	server := newTestServer()

	startResult, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "task control rpc validation",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start task: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-control-missing-action"`),
		Method:  "agent.task.control",
		Params: mustMarshal(t, map[string]any{
			"task_id": taskID,
		}),
	})

	errEnvelope, ok := response.(errorEnvelope)
	if !ok {
		t.Fatalf("expected error response envelope, got %#v", response)
	}
	if errEnvelope.Error.Code != 1002001 || errEnvelope.Error.Message != "INVALID_PARAMS" {
		t.Fatalf("expected INVALID_PARAMS mapping for missing action, got code=%d message=%s", errEnvelope.Error.Code, errEnvelope.Error.Message)
	}
}

func TestDispatchTaskListClampsPagingParams(t *testing.T) {
	server := newTestServer()

	for index := 0; index < 25; index++ {
		_, err := server.orchestrator.StartTask(map[string]any{
			"session_id": fmt.Sprintf("sess_rpc_task_list_%02d", index),
			"source":     "floating_ball",
			"trigger":    "hover_text_input",
			"input": map[string]any{
				"type": "text",
				"text": fmt.Sprintf("rpc task list clamp %02d", index),
			},
			"intent": map[string]any{
				"name": "write_file",
				"arguments": map[string]any{
					"require_authorization": true,
				},
			},
		})
		if err != nil {
			t.Fatalf("start task %d: %v", index, err)
		}
	}

	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-list-clamp"`),
		Method:  "agent.task.list",
		Params: mustMarshal(t, map[string]any{
			"group":      "unfinished",
			"limit":      0,
			"offset":     -5,
			"sort_by":    "updated_at",
			"sort_order": "desc",
		}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	items := data["items"].([]map[string]any)
	if len(items) != 20 {
		t.Fatalf("expected rpc task.list to clamp zero limit to 20 items, got %d", len(items))
	}
	page := data["page"].(map[string]any)
	if numericValue(t, page["limit"]) != 20 {
		t.Fatalf("expected clamped rpc page limit 20, got %+v", page)
	}
	if numericValue(t, page["offset"]) != 0 {
		t.Fatalf("expected clamped rpc page offset 0, got %+v", page)
	}
	if page["has_more"] != true {
		t.Fatalf("expected rpc page has_more to remain true after clamping, got %+v", page)
	}
	if numericValue(t, page["total"]) != 25 {
		t.Fatalf("expected rpc page total 25, got %+v", page)
	}
}

func TestDispatchTaskEventsListReturnsLoopEvents(t *testing.T) {
	server := newTestServer()
	storageService := storage.NewService(testStorageAdapter{databasePath: filepath.Join(t.TempDir(), "rpc-loop-events.db")})
	defer func() { _ = storageService.Close() }()
	server.orchestrator.WithStorage(storageService)
	if err := storageService.LoopRuntimeStore().SaveEvents(context.Background(), []storage.EventRecord{{
		EventID:     "evt_rpc_loop_001",
		RunID:       "run_rpc_loop_001",
		TaskID:      "task_rpc_loop_001",
		StepID:      "step_rpc_loop_001",
		Type:        "loop.completed",
		Level:       "info",
		PayloadJSON: `{"stop_reason":"completed"}`,
		CreatedAt:   "2026-04-17T10:00:00Z",
	}}); err != nil {
		t.Fatalf("save loop events failed: %v", err)
	}

	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-events-list"`),
		Method:  "agent.task.events.list",
		Params: mustMarshal(t, map[string]any{
			"task_id": "task_rpc_loop_001",
			"run_id":  "run_rpc_loop_001",
			"type":    "loop.completed",
			"limit":   20,
			"offset":  0,
		}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	items := data["items"].([]map[string]any)
	if len(items) != 1 || items[0]["type"] != "loop.completed" {
		t.Fatalf("expected rpc task events list to return loop.completed, got %+v", items)
	}
}

func TestDispatchTaskToolCallsListReturnsPersistedToolCalls(t *testing.T) {
	server := newTestServer()
	storageService := storage.NewService(testStorageAdapter{databasePath: filepath.Join(t.TempDir(), "rpc-tool-calls.db")})
	defer func() { _ = storageService.Close() }()
	server.orchestrator.WithStorage(storageService)
	if err := storageService.ToolCallStore().SaveToolCall(context.Background(), tools.ToolCallRecord{
		ToolCallID: "tool_call_rpc_001",
		RunID:      "run_rpc_tool_001",
		TaskID:     "task_rpc_tool_001",
		ToolName:   "read_file",
		Status:     tools.ToolCallStatusSucceeded,
		Input:      map[string]any{"path": "notes/source.txt"},
		Output:     map[string]any{"path": "notes/source.txt"},
		DurationMS: 9,
	}); err != nil {
		t.Fatalf("save tool call failed: %v", err)
	}

	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-tool-calls-list"`),
		Method:  "agent.task.tool_calls.list",
		Params: mustMarshal(t, map[string]any{
			"task_id": "task_rpc_tool_001",
			"limit":   20,
			"offset":  0,
		}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	items := data["items"].([]map[string]any)
	if len(items) != 1 || items[0]["tool_name"] != "read_file" {
		t.Fatalf("expected rpc task tool calls list to return read_file, got %+v", items)
	}
}

func TestDispatchTaskSteerReturnsUpdatedTask(t *testing.T) {
	server := newTestServer()
	startResult, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_rpc_task_steer",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "Please write this into a file after authorization.",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start task: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)

	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-steer"`),
		Method:  "agent.task.steer",
		Params: mustMarshal(t, map[string]any{
			"task_id": taskID,
			"message": "Also include a short summary section.",
		}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	if data["task"].(map[string]any)["task_id"] != taskID {
		t.Fatalf("expected rpc task steer to keep task id, got %+v", data)
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
