package orchestrator

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestServiceSettingsRuntimePathsGetReturnsResolvedPaths(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecution(t, "paths")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	result, err := service.SettingsRuntimePathsGet(nil)
	if err != nil {
		t.Fatalf("SettingsRuntimePathsGet returned error: %v", err)
	}
	if result["workspace_path"] != filepath.ToSlash(workspaceRoot) {
		t.Fatalf("expected workspace path %q, got %+v", filepath.ToSlash(workspaceRoot), result)
	}
	if result["database_path"] != filepath.ToSlash(service.storage.DatabasePath()) {
		t.Fatalf("expected database path %q, got %+v", filepath.ToSlash(service.storage.DatabasePath()), result)
	}
	if result["secret_store_path"] != filepath.ToSlash(service.storage.SecretStorePath()) {
		t.Fatalf("expected secret store path %q, got %+v", filepath.ToSlash(service.storage.SecretStorePath()), result)
	}
	if result["runtime_root"] == "" {
		t.Fatalf("expected runtime root, got %+v", result)
	}
}

func TestServiceLogExecutionListAggregatesEventsToolCallsAndAudits(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "logs")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	ctx := context.Background()
	if err := service.storage.LoopRuntimeStore().SaveEvents(ctx, []storage.EventRecord{{
		EventID:     "event_001",
		RunID:       "run_001",
		TaskID:      "task_001",
		Type:        "loop.round.completed",
		Level:       "info",
		PayloadJSON: `{"round":1}`,
		CreatedAt:   "2026-04-18T10:01:00Z",
	}}); err != nil {
		t.Fatalf("SaveEvents returned error: %v", err)
	}
	if err := service.storage.ToolCallStore().SaveToolCall(ctx, tools.ToolCallRecord{
		ToolCallID: "tool_001",
		RunID:      "run_001",
		TaskID:     "task_001",
		CreatedAt:  "2026-04-18T10:02:00Z",
		ToolName:   "read_file",
		Status:     tools.ToolCallStatusSucceeded,
		DurationMS: 11,
	}); err != nil {
		t.Fatalf("SaveToolCall returned error: %v", err)
	}
	if err := service.storage.AuditWriter().WriteAuditRecord(ctx, audit.Record{
		AuditID:   "audit_001",
		TaskID:    "task_001",
		Type:      "file",
		Action:    "write_file",
		Summary:   "stored audit",
		Target:    "workspace/output.md",
		Result:    "success",
		CreatedAt: "2026-04-18T10:03:00Z",
	}); err != nil {
		t.Fatalf("WriteAuditRecord returned error: %v", err)
	}

	result, err := service.LogExecutionList(map[string]any{"task_id": "task_001", "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("LogExecutionList returned error: %v", err)
	}
	items := result["items"].([]map[string]any)
	if len(items) != 3 {
		t.Fatalf("expected three execution log items, got %+v", items)
	}
	if items[0]["source"] != "audit" || items[1]["source"] != "tool_call" || items[2]["source"] != "event" {
		t.Fatalf("expected aggregated descending log order, got %+v", items)
	}
}

func TestServiceLogErrorListFiltersFailures(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "errors")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	ctx := context.Background()
	if err := service.storage.LoopRuntimeStore().SaveEvents(ctx, []storage.EventRecord{
		{
			EventID:     "event_ok",
			RunID:       "run_001",
			TaskID:      "task_001",
			Type:        "loop.round.completed",
			Level:       "info",
			PayloadJSON: `{"round":1}`,
			CreatedAt:   "2026-04-18T10:01:00Z",
		},
		{
			EventID:     "event_failed",
			RunID:       "run_001",
			TaskID:      "task_001",
			Type:        "loop.failed",
			Level:       "error",
			PayloadJSON: `{"message":"failed"}`,
			CreatedAt:   "2026-04-18T10:04:00Z",
		},
	}); err != nil {
		t.Fatalf("SaveEvents returned error: %v", err)
	}
	if err := service.storage.ToolCallStore().SaveToolCall(ctx, tools.ToolCallRecord{
		ToolCallID: "tool_failed",
		RunID:      "run_001",
		TaskID:     "task_001",
		CreatedAt:  "2026-04-18T10:03:00Z",
		ToolName:   "run_command",
		Status:     tools.ToolCallStatusFailed,
		ErrorCode:  intPtr(17),
		DurationMS: 22,
	}); err != nil {
		t.Fatalf("SaveToolCall returned error: %v", err)
	}
	if err := service.storage.ToolCallStore().SaveToolCall(ctx, tools.ToolCallRecord{
		ToolCallID: "tool_ok",
		RunID:      "run_001",
		TaskID:     "task_001",
		CreatedAt:  "2026-04-18T10:02:00Z",
		ToolName:   "read_file",
		Status:     tools.ToolCallStatusSucceeded,
		DurationMS: 8,
	}); err != nil {
		t.Fatalf("SaveToolCall returned error: %v", err)
	}
	if err := service.storage.AuditWriter().WriteAuditRecord(ctx, audit.Record{
		AuditID:   "audit_failed",
		TaskID:    "task_001",
		Type:      "command",
		Action:    "run_command",
		Summary:   "command failed",
		Target:    "workspace/script.sh",
		Result:    "failed",
		CreatedAt: "2026-04-18T10:05:00Z",
	}); err != nil {
		t.Fatalf("WriteAuditRecord returned error: %v", err)
	}

	result, err := service.LogErrorList(map[string]any{"task_id": "task_001", "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("LogErrorList returned error: %v", err)
	}
	items := result["items"].([]map[string]any)
	if len(items) != 3 {
		t.Fatalf("expected three error log items, got %+v", items)
	}
	if items[0]["log_id"] != "audit:audit_failed" || items[1]["log_id"] != "event:event_failed" || items[2]["log_id"] != "tool_call:tool_failed" {
		t.Fatalf("expected only failing log entries in descending order, got %+v", items)
	}
	if items[2]["error_code"] != "17" {
		t.Fatalf("expected tool-call error code to be preserved, got %+v", items[2])
	}
}
