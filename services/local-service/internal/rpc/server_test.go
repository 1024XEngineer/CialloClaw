// 该测试文件验证 RPC 层的响应与通知流行为。
package rpc

import (
	"encoding/json"
	"net"
	"net/http/httptest"
	"testing"
	"time"

	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/memory"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/orchestrator"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/risk"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

// TestHandleStreamConnEmitsApprovalNotifications 验证HandleStreamConnEmitsApprovalNotifications。
func TestHandleStreamConnEmitsApprovalNotifications(t *testing.T) {
	server := newTestServer()
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()

	go server.handleStreamConn(left)

	encoder := json.NewEncoder(right)
	decoder := json.NewDecoder(right)

	request := requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-1"`),
		Method:  "agent.task.start",
		Params: mustMarshal(t, map[string]any{
			"session_id": "sess_demo",
			"source":     "floating_ball",
			"trigger":    "text_selected_click",
			"input": map[string]any{
				"type": "text_selection",
				"text": "请生成一个文件版本",
			},
			"intent": map[string]any{
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
	for index := 0; index < 4; index++ {
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

// TestHandleDebugEventsReturnsQueuedNotifications 验证HandleDebugEventsReturnsQueuedNotifications。
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

// newTestServer 处理当前模块的相关逻辑。
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

func newTestServer() *Server {
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
		tools.NewRegistry(),
		plugin.NewService(),
	)

	server := NewServer(serviceconfig.RPCConfig{
		Transport:        "named_pipe",
		NamedPipeName:    `\\.\pipe\cialloclaw-rpc-test`,
		DebugHTTPAddress: ":0",
	}, orch)
	server.now = func() time.Time {
		return time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC)
	}
	return server
}

// mustMarshal 处理当前模块的相关逻辑。
func mustMarshal(t *testing.T, value any) json.RawMessage {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal request params: %v", err)
	}
	return encoded
}
