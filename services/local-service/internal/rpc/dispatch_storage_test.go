package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskinspector"
)

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
		Method:  methodAgentSecurityAuditList,
		Params:  mustMarshal(t, map[string]any{"task_id": "task_001", "limit": 20, "offset": 0}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	items := success.Result.Data.(map[string]any)["items"].([]map[string]any)
	if len(items) != 1 || items[0]["audit_id"] != "audit_001" {
		t.Fatalf("expected stored audit_001, got %+v", items)
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
		Method:  methodAgentSecurityRestorePointsList,
		Params:  mustMarshal(t, map[string]any{"task_id": "task_001", "limit": 20, "offset": 0}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	items := success.Result.Data.(map[string]any)["items"].([]map[string]any)
	if len(items) != 1 || items[0]["recovery_point_id"] != "rp_001" {
		t.Fatalf("expected stored rp_001, got %+v", items)
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
		Method:  methodAgentSecurityRestoreApply,
		Params:  mustMarshal(t, map[string]any{"task_id": taskID, "recovery_point_id": "rp_001"}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	if _, ok := data["applied"].(bool); !ok || data["task"].(map[string]any)["status"] != "waiting_auth" || data["recovery_point"].(map[string]any)["recovery_point_id"] != "rp_001" {
		t.Fatalf("unexpected restore apply result %+v", data)
	}
}

func TestDispatchReturnsNotepadUpdateResult(t *testing.T) {
	server := newTestServer()
	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-notepad-update"`),
		Method:  methodAgentNotepadUpdate,
		Params:  mustMarshal(t, map[string]any{"item_id": "todo_002", "action": "move_upcoming"}),
	})
	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	notepadItem, ok := data["notepad_item"].(map[string]any)
	if !ok || notepadItem["bucket"] != "upcoming" {
		t.Fatalf("expected updated notepad item bucket upcoming, got %+v", data)
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
		Method:  methodAgentTaskArtifactList,
		Params:  mustMarshal(t, map[string]any{"task_id": "task_rpc_001", "limit": 20, "offset": 0}),
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
		Method:  methodAgentTaskArtifactOpen,
		Params:  mustMarshal(t, map[string]any{"task_id": "task_rpc_open_001", "artifact_id": "art_rpc_open_001"}),
	})
	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	if data["open_action"] != "open_file" || data["artifact"].(map[string]any)["artifact_id"] != "art_rpc_open_001" {
		t.Fatalf("expected opened artifact, got %+v", data)
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
		Method:  methodAgentDeliveryOpen,
		Params:  mustMarshal(t, map[string]any{"task_id": "task_delivery_rpc_001", "artifact_id": "art_delivery_rpc_001"}),
	})
	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	if success.Result.Data.(map[string]any)["open_action"] != "open_file" {
		t.Fatalf("expected open_file action, got %+v", success.Result.Data)
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
		Method:  methodAgentDeliveryOpen,
		Params:  mustMarshal(t, map[string]any{"task_id": taskID}),
	})
	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	if success.Result.Data.(map[string]any)["open_action"] != "task_detail" {
		t.Fatalf("expected task_detail action, got %+v", success.Result.Data)
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
				Method:  methodAgentTaskInspectorRun,
				Params:  mustMarshal(t, map[string]any{"target_sources": test.targetSources}),
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

func TestNewServerSkipsDebugHTTPWhenDisabled(t *testing.T) {
	seed := newTestServer()
	server := NewServer(serviceconfig.RPCConfig{
		Transport:        "named_pipe",
		NamedPipeName:    `\\.\pipe\cialloclaw-rpc-disabled`,
		DebugHTTPAddress: "",
	}, seed.orchestrator)

	if server.debugHTTPServer != nil {
		t.Fatal("expected explicit empty debug HTTP address to disable the diagnostics server")
	}
}
