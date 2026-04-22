package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestToolCallStoresListAndDecodeRecords(t *testing.T) {
	record := tools.ToolCallRecord{
		ToolCallID: "tool_call_001",
		RunID:      "run_001",
		TaskID:     "task_001",
		StepID:     "step_001",
		ToolName:   "read_file",
		Status:     tools.ToolCallStatusSucceeded,
		Input:      map[string]any{"path": "README.md"},
		Output:     map[string]any{"ok": true},
		DurationMS: 42,
	}

	inMemory := newInMemoryToolCallStore()
	if err := inMemory.SaveToolCall(context.Background(), record); err != nil {
		t.Fatalf("in-memory SaveToolCall returned error: %v", err)
	}
	updatedRecord := record
	updatedRecord.Status = tools.ToolCallStatusTimeout
	updatedRecord.DurationMS = 84
	updatedRecord.Output = map[string]any{"ok": true, "summary_output": map[string]any{"path": "README.md"}}
	if err := inMemory.SaveToolCall(context.Background(), updatedRecord); err != nil {
		t.Fatalf("in-memory SaveToolCall update returned error: %v", err)
	}
	items, total, err := inMemory.ListToolCalls(context.Background(), "task_001", "run_001", 10, 0)
	if err != nil || total != 1 || len(items) != 1 || items[0].ToolName != "read_file" {
		t.Fatalf("in-memory ListToolCalls returned total=%d items=%+v err=%v", total, items, err)
	}
	summaryOutput, ok := items[0].Output["summary_output"].(map[string]any)
	if items[0].DurationMS != 84 || items[0].Status != tools.ToolCallStatusFailed || !ok || summaryOutput["path"] != "README.md" {
		t.Fatalf("expected in-memory tool call store to upsert latest record, got %+v", items[0])
	}

	sqliteStore, err := NewSQLiteToolCallStore(filepath.Join(t.TempDir(), "tool-calls.db"))
	if err != nil {
		t.Fatalf("NewSQLiteToolCallStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()
	if err := sqliteStore.SaveToolCall(context.Background(), record); err != nil {
		t.Fatalf("sqlite SaveToolCall returned error: %v", err)
	}
	items, total, err = sqliteStore.ListToolCalls(context.Background(), "task_001", "run_001", 10, 0)
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("sqlite ListToolCalls returned total=%d items=%+v err=%v", total, items, err)
	}
	if items[0].Status != tools.ToolCallStatusSucceeded || items[0].Input["path"] != "README.md" {
		t.Fatalf("expected decoded tool call record, got %+v", items[0])
	}
	if normalizeToolCallStatus(tools.ToolCallStatusTimeout) != "failed" {
		t.Fatalf("expected timeout status to normalize as failed")
	}
	if denormalizeToolCallStatus("unknown") != tools.ToolCallStatusStarted {
		t.Fatalf("expected unknown status to denormalize to started")
	}
	if paged, total, err := sqliteStore.ListToolCalls(context.Background(), "task_001", "run_001", 1, 5); err != nil || total != 1 || len(paged) != 0 {
		t.Fatalf("expected paged tool-call query overflow to return empty slice, total=%d items=%+v err=%v", total, paged, err)
	}
	if unfiltered, total, err := sqliteStore.ListToolCalls(context.Background(), "", "", 0, 0); err != nil || total != 1 || len(unfiltered) != 1 {
		t.Fatalf("expected unfiltered tool-call query to return stored record, total=%d items=%+v err=%v", total, unfiltered, err)
	}

	if normalizeToolCallStatus(tools.ToolCallStatus("unexpected")) != "pending" {
		t.Fatalf("expected unknown normalized status to fall back to pending")
	}
	if denormalizeToolCallStatus("running") != tools.ToolCallStatusStarted || denormalizeToolCallStatus("succeeded") != tools.ToolCallStatusSucceeded || denormalizeToolCallStatus("failed") != tools.ToolCallStatusFailed {
		t.Fatal("expected denormalizeToolCallStatus to map known persisted statuses")
	}

	if _, err := NewSQLiteToolCallStore(""); err == nil {
		t.Fatal("expected sqlite tool call constructor to reject empty path")
	}
	var nilSQLiteStore SQLiteToolCallStore
	if err := nilSQLiteStore.Close(); err != nil {
		t.Fatalf("expected nil sqlite tool call store close to succeed, got %v", err)
	}

	badInput := record
	badInput.ToolCallID = "tool_call_bad_input"
	badInput.Input = map[string]any{"bad": make(chan int)}
	if err := sqliteStore.SaveToolCall(context.Background(), badInput); err == nil || !strings.Contains(err.Error(), "marshal tool call input") {
		t.Fatalf("expected input marshal error, got %v", err)
	}
	badOutput := record
	badOutput.ToolCallID = "tool_call_bad_output"
	badOutput.Output = map[string]any{"bad": func() {}}
	if err := sqliteStore.SaveToolCall(context.Background(), badOutput); err == nil || !strings.Contains(err.Error(), "marshal tool call output") {
		t.Fatalf("expected output marshal error, got %v", err)
	}
}

func TestSQLiteToolCallStoreConstructorAndDecodeFailurePaths(t *testing.T) {
	dirPath := filepath.Join(t.TempDir(), "tool-call-dir")
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		t.Fatalf("prepare directory path failed: %v", err)
	}
	if _, err := NewSQLiteToolCallStore(dirPath); err == nil || !strings.Contains(err.Error(), "ping sqlite database") {
		t.Fatalf("expected directory path constructor to fail during ping, got %v", err)
	}

	store, err := NewSQLiteToolCallStore(filepath.Join(t.TempDir(), "tool-call-decode.db"))
	if err != nil {
		t.Fatalf("NewSQLiteToolCallStore returned error: %v", err)
	}
	defer func() { _ = store.Close() }()
	if _, err := store.db.Exec(`INSERT INTO tool_calls (tool_call_id, run_id, task_id, step_id, tool_name, status, input_json, output_json, error_code, duration_ms) VALUES ('tool_bad_input', 'run_decode', 'task_bad_input', 'step_001', 'read_file', 'running', '{bad-json', '{}', NULL, 1)`); err != nil {
		t.Fatalf("insert invalid input row failed: %v", err)
	}
	if _, _, err := store.ListToolCalls(context.Background(), "task_bad_input", "", 10, 0); err == nil || !strings.Contains(err.Error(), "decode tool call input") {
		t.Fatalf("expected invalid input json to surface decode error, got %v", err)
	}
	if _, err := store.db.Exec(`DELETE FROM tool_calls WHERE task_id = 'task_bad_input'`); err != nil {
		t.Fatalf("delete invalid input row failed: %v", err)
	}
	if _, err := store.db.Exec(`INSERT INTO tool_calls (tool_call_id, run_id, task_id, step_id, tool_name, status, input_json, output_json, error_code, duration_ms) VALUES ('tool_bad_output', 'run_decode', 'task_bad_output', 'step_001', 'read_file', 'failed', '{}', '{bad-json', 17, 1)`); err != nil {
		t.Fatalf("insert invalid output row failed: %v", err)
	}
	if _, _, err := store.ListToolCalls(context.Background(), "task_bad_output", "", 10, 0); err == nil || !strings.Contains(err.Error(), "decode tool call output") {
		t.Fatalf("expected invalid output json to surface decode error, got %v", err)
	}

	closedDB, err := sql.Open(sqliteDriverName, filepath.Join(t.TempDir(), "tool-call-closed.db"))
	if err != nil {
		t.Fatalf("open closed-db fixture failed: %v", err)
	}
	_ = closedDB.Close()
	closedStore := &SQLiteToolCallStore{db: closedDB}
	if err := closedStore.initialize(context.Background()); err == nil || !strings.Contains(err.Error(), "enable sqlite wal mode") {
		t.Fatalf("expected initialize on closed db to fail, got %v", err)
	}
}
