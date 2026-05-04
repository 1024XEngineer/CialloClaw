package rpc

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

func TestDispatchReturnsSettingsRuntimePaths(t *testing.T) {
	server := newTestServer()
	storageService := storage.NewService(platform.NewLocalStorageAdapter(filepath.Join(t.TempDir(), "runtime-paths.db")))
	defer func() { _ = storageService.Close() }()
	server.orchestrator.WithStorage(storageService)

	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-settings-runtime-paths"`),
		Method:  "agent.settings.runtime_paths.get",
		Params:  mustMarshal(t, map[string]any{}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	data := success.Result.Data.(map[string]any)
	if data["database_path"] != filepath.ToSlash(storageService.DatabasePath()) || data["secret_store_path"] != filepath.ToSlash(storageService.SecretStorePath()) {
		t.Fatalf("expected runtime path payload to reflect storage paths, got %+v", data)
	}
}

func TestDispatchReturnsExecutionLogList(t *testing.T) {
	server := newTestServer()
	storageService := storage.NewService(platform.NewLocalStorageAdapter(filepath.Join(t.TempDir(), "execution-log.db")))
	defer func() { _ = storageService.Close() }()
	server.orchestrator.WithStorage(storageService)
	if err := storageService.LoopRuntimeStore().SaveEvents(context.Background(), []storage.EventRecord{{
		EventID:     "event_001",
		RunID:       "run_001",
		TaskID:      "task_001",
		Type:        "loop.failed",
		Level:       "error",
		PayloadJSON: `{"message":"failed"}`,
		CreatedAt:   "2026-04-18T10:01:00Z",
	}}); err != nil {
		t.Fatalf("SaveEvents returned error: %v", err)
	}

	response := server.dispatch(requestEnvelope{
		JSONRPC: "2.0",
		ID:      json.RawMessage(`"req-log-execution-list"`),
		Method:  "agent.log.execution.list",
		Params: mustMarshal(t, map[string]any{
			"task_id": "task_001",
			"source":  "event",
			"limit":   20,
			"offset":  0,
		}),
	})

	success, ok := response.(successEnvelope)
	if !ok {
		t.Fatalf("expected success response envelope, got %#v", response)
	}
	items := success.Result.Data.(map[string]any)["items"].([]map[string]any)
	if len(items) != 1 || items[0]["log_id"] != "event:event_001" {
		t.Fatalf("expected filtered execution log payload, got %+v", items)
	}
}
