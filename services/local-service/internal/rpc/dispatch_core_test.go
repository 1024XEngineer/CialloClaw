package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestDispatchTaskStartIgnoresUnsupportedIntentField(t *testing.T) {
	server := newTestServer()

	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-start-ignore-intent"`),
		Method:  methodAgentTaskStart,
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

func TestDispatchTaskStartFileInstructionSkipsIntentConfirmation(t *testing.T) {
	server := newTestServerWithModelClient(&stubLoopModelClient{})

	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-start-file-instruction"`),
		Method:  methodAgentTaskStart,
		Params: mustMarshal(t, map[string]any{
			"session_id": "sess_file_instruction_rpc",
			"source":     "floating_ball",
			"trigger":    "file_drop",
			"input": map[string]any{
				"type":  "file",
				"text":  "帮我看看这里面有什么",
				"files": []string{"workspace/MyToDos_Vue"},
			},
			"options": map[string]any{
				"confirm_required": false,
			},
			"delivery": map[string]any{
				"preferred": "bubble",
				"fallback":  "task_detail",
			},
		}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	result := success.Result.Data.(map[string]any)
	task := result["task"].(map[string]any)
	if task["status"] == "confirming_intent" || task["current_step"] == "intent_confirmation" {
		t.Fatalf("expected instructed file start to skip intent confirmation, got %+v", task)
	}
	if task["source_type"] != "dragged_file" {
		t.Fatalf("expected dragged_file source type, got %+v", task)
	}
	intentValue, ok := task["intent"].(map[string]any)
	if !ok || intentValue["name"] != "agent_loop" {
		t.Fatalf("expected task.start to keep backend agent_loop suggestion, got %+v", task["intent"])
	}
	if result["delivery_result"] == nil {
		t.Fatal("expected instructed file start to return delivery_result")
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
		Method:  methodAgentTaskDetailGet,
		Params:  mustMarshal(t, map[string]any{"task_id": taskID}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	approvalRequest, ok := data["approval_request"].(map[string]any)
	if !ok || approvalRequest["task_id"] != taskID {
		t.Fatalf("expected approval_request task_id %s, got %+v", taskID, data["approval_request"])
	}
	securitySummary := data["security_summary"].(map[string]any)
	if numericValue(t, securitySummary["pending_authorizations"]) != 1 {
		t.Fatalf("expected pending_authorizations 1, got %+v", securitySummary["pending_authorizations"])
	}
	if securitySummary["latest_restore_point"] != nil {
		t.Fatalf("expected latest_restore_point nil, got %+v", securitySummary["latest_restore_point"])
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
		Method:  methodAgentTaskDetailGet,
		Params:  mustMarshal(t, map[string]any{"task_id": taskID}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	if data["approval_request"] != nil {
		t.Fatalf("expected approval_request to be nil, got %+v", data["approval_request"])
	}
	securitySummary := data["security_summary"].(map[string]any)
	if numericValue(t, securitySummary["pending_authorizations"]) != 0 {
		t.Fatalf("expected pending_authorizations 0, got %+v", securitySummary["pending_authorizations"])
	}
	if _, ok := securitySummary["latest_restore_point"].(map[string]any); !ok {
		t.Fatalf("expected latest_restore_point object, got %+v", securitySummary["latest_restore_point"])
	}
}

func TestDispatchTaskControlValidationAndStatusErrors(t *testing.T) {
	server := newTestServer()

	completedTask, err := server.orchestrator.StartTask(map[string]any{
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
	completedTaskID := completedTask["task"].(map[string]any)["task_id"].(string)
	if _, ok := server.orchestrator.RunEngine().CompleteTask(completedTaskID, map[string]any{"type": "task_detail", "payload": map[string]any{"task_id": completedTaskID}}, map[string]any{"task_id": completedTaskID, "type": "result", "text": "done"}, nil); !ok {
		t.Fatal("expected runtime task completion to succeed")
	}

	waitingTask, err := server.orchestrator.StartTask(map[string]any{
		"session_id": "sess_waiting_task",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "still waiting for intent confirmation",
		},
	})
	if err != nil {
		t.Fatalf("start waiting task: %v", err)
	}
	waitingTaskID := waitingTask["task"].(map[string]any)["task_id"].(string)

	tests := []struct {
		name          string
		params        map[string]any
		expectedCode  int
		expectedError string
	}{
		{name: "missing task id", params: map[string]any{"action": "pause"}, expectedCode: errInvalidParams, expectedError: "INVALID_PARAMS"},
		{name: "unsupported action", params: map[string]any{"task_id": completedTaskID, "action": "skip"}, expectedCode: errInvalidParams, expectedError: "INVALID_PARAMS"},
		{name: "finished task", params: map[string]any{"task_id": completedTaskID, "action": "cancel"}, expectedCode: 1001005, expectedError: "TASK_ALREADY_FINISHED"},
		{name: "status invalid", params: map[string]any{"task_id": waitingTaskID, "action": "pause"}, expectedCode: 1001004, expectedError: "TASK_STATUS_INVALID"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := server.dispatch(requestEnvelope{
				JSONRPC: "2.0",
				ID:      json.RawMessage(`"req-task-control"`),
				Method:  methodAgentTaskControl,
				Params:  mustMarshal(t, test.params),
			})
			errEnvelope, ok := response.(errorEnvelope)
			if !ok {
				t.Fatalf("expected error response envelope, got %#v", response)
			}
			if errEnvelope.Error.Code != test.expectedCode || errEnvelope.Error.Message != test.expectedError {
				t.Fatalf("expected %s mapping, got code=%d message=%s", test.expectedError, errEnvelope.Error.Code, errEnvelope.Error.Message)
			}
		})
	}
}

func TestDispatchTaskConfirmRejectsMalformedCorrectionText(t *testing.T) {
	server := newTestServer()

	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-task-confirm-invalid-correction-text"`),
		Method:  methodAgentTaskConfirm,
		Params: mustMarshal(t, map[string]any{
			"task_id":         "task_confirm_invalid_correction_text",
			"confirmed":       false,
			"correction_text": 42,
		}),
	})

	errEnvelope, ok := response.(errorEnvelope)
	if !ok {
		t.Fatalf("expected error response envelope, got %#v", response)
	}
	if errEnvelope.Error.Code != errInvalidParams || errEnvelope.Error.Message != "INVALID_PARAMS" {
		t.Fatalf("expected INVALID_PARAMS for malformed correction_text, got %+v", errEnvelope.Error)
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
		Method:  methodAgentTaskList,
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
	if numericValue(t, page["limit"]) != 20 || numericValue(t, page["offset"]) != 0 || page["has_more"] != true || numericValue(t, page["total"]) != 25 {
		t.Fatalf("unexpected clamped page %+v", page)
	}
}

func TestDispatchTaskEventsListReturnsLoopEvents(t *testing.T) {
	storageService := storage.NewService(testStorageAdapter{databasePath: filepath.Join(t.TempDir(), "rpc-loop-events.db")})
	defer func() { _ = storageService.Close() }()
	server := newTestServerWithStorage(storageService)
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
		Method:  methodAgentTaskEventsList,
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
	items := success.Result.Data.(map[string]any)["items"].([]map[string]any)
	if len(items) != 1 || items[0]["type"] != "loop.completed" {
		t.Fatalf("expected rpc task events list to return loop.completed, got %+v", items)
	}
}

func TestDispatchTaskToolCallsListReturnsPersistedToolCalls(t *testing.T) {
	storageService := storage.NewService(testStorageAdapter{databasePath: filepath.Join(t.TempDir(), "rpc-tool-calls.db")})
	defer func() { _ = storageService.Close() }()
	server := newTestServerWithStorage(storageService)
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
		Method:  methodAgentTaskToolCallsList,
		Params:  mustMarshal(t, map[string]any{"task_id": "task_rpc_tool_001", "limit": 20, "offset": 0}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	items := success.Result.Data.(map[string]any)["items"].([]map[string]any)
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
		Method:  methodAgentTaskSteer,
		Params:  mustMarshal(t, map[string]any{"task_id": taskID, "message": "Also include a short summary section."}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	if success.Result.Data.(map[string]any)["task"].(map[string]any)["task_id"] != taskID {
		t.Fatalf("expected rpc task steer to keep task id, got %+v", success.Result.Data)
	}
}

func TestDispatchReturnsSettingsGet(t *testing.T) {
	storageService := storage.NewService(testStorageAdapter{databasePath: filepath.Join(t.TempDir(), "settings-get.db")})
	defer func() { _ = storageService.Close() }()
	server := newTestServerWithStorage(storageService)
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-settings-get"`),
		Method:  methodAgentSettingsGet,
		Params:  mustMarshal(t, map[string]any{"scope": "all"}),
	})
	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	credentials := success.Result.Data.(map[string]any)["settings"].(map[string]any)["models"].(map[string]any)["credentials"].(map[string]any)
	if _, ok := credentials["stronghold"].(map[string]any); !ok {
		t.Fatalf("expected settings get to include stronghold status, got %+v", credentials)
	}
}

func TestDispatchReturnsSettingsUpdate(t *testing.T) {
	storageService := storage.NewService(testStorageAdapter{databasePath: filepath.Join(t.TempDir(), "settings-update.db")})
	defer func() { _ = storageService.Close() }()
	server := newTestServerWithStorage(storageService)
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-settings-update"`),
		Method:  methodAgentSettingsUpdate,
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
		Method:  methodAgentSettingsModelValidate,
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
		Method:  methodAgentPluginRuntimeList,
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
		Method:  methodAgentPluginList,
		Params:  mustMarshal(t, map[string]any{"query": "ocr", "page": map[string]any{"limit": 10, "offset": 0}}),
	})
	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	items := success.Result.Data.(map[string]any)["items"].([]map[string]any)
	if len(items) != 1 || items[0]["plugin_id"] != "ocr" {
		t.Fatalf("expected plugin list query to return filtered ocr plugin, got %+v", success.Result.Data)
	}
}

func TestDispatchReturnsPluginDetail(t *testing.T) {
	server, toolRegistry, pluginService := newTestServerWithDependencies(nil, nil, nil)
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-plugin-detail"`),
		Method:  methodAgentPluginDetailGet,
		Params:  mustMarshal(t, map[string]any{"plugin_id": "ocr"}),
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
		t.Fatalf("expected one contract per declared capability, got %+v", data)
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
