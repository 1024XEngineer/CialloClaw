package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestTaskStoresSupportGetAndListBySession(t *testing.T) {
	inMemory := newInMemoryTaskStore()
	if err := inMemory.WriteTask(context.Background(), TaskRecord{TaskID: "task_mem_001", SessionID: "sess_001", RunID: "run_mem_001", RequestSource: "floating_ball", RequestTrigger: "hover_text_input", Title: "Task A", UpdatedAt: "2026-04-21T10:02:00Z", StartedAt: "2026-04-21T10:00:00Z"}); err != nil {
		t.Fatalf("in-memory WriteTask returned error: %v", err)
	}
	if err := inMemory.WriteTask(context.Background(), TaskRecord{TaskID: "task_mem_002", SessionID: "sess_001", PrimaryRunID: "run_mem_002", Title: "Task B", UpdatedAt: "2026-04-21T10:03:00Z", StartedAt: "2026-04-21T10:01:00Z"}); err != nil {
		t.Fatalf("in-memory WriteTask returned error: %v", err)
	}
	record, err := inMemory.GetTask(context.Background(), "task_mem_001")
	if err != nil || record.Title != "Task A" || record.PrimaryRunID != "run_mem_001" || record.RequestSource != "floating_ball" || record.RequestTrigger != "hover_text_input" {
		t.Fatalf("in-memory GetTask returned record=%+v err=%v", record, err)
	}
	items, total, err := inMemory.ListTasksBySession(context.Background(), "sess_001", 10, 0)
	if err != nil || total != 2 || len(items) != 2 || items[0].TaskID != "task_mem_002" {
		t.Fatalf("in-memory ListTasksBySession returned total=%d items=%+v err=%v", total, items, err)
	}
	if _, err := inMemory.GetTask(context.Background(), "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected missing in-memory task to return sql.ErrNoRows, got %v", err)
	}

	sqliteStore, err := NewSQLiteTaskStore(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatalf("NewSQLiteTaskStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()
	if err := sqliteStore.WriteTask(context.Background(), TaskRecord{TaskID: "task_sql_001", SessionID: "sess_sql", RunID: "run_sql_001", RequestSource: "floating_ball", RequestTrigger: "hover_text_input", Title: "Task SQL", Status: "processing", UpdatedAt: "2026-04-21T10:05:00Z", StartedAt: "2026-04-21T10:00:00Z"}); err != nil {
		t.Fatalf("sqlite WriteTask returned error: %v", err)
	}
	record, err = sqliteStore.GetTask(context.Background(), "task_sql_001")
	if err != nil || record.Title != "Task SQL" || record.PrimaryRunID != "run_sql_001" || record.RequestSource != "floating_ball" || record.RequestTrigger != "hover_text_input" {
		t.Fatalf("sqlite GetTask returned record=%+v err=%v", record, err)
	}
	items, total, err = sqliteStore.ListTasksBySession(context.Background(), "sess_sql", 10, 0)
	if err != nil || total != 1 || len(items) != 1 || items[0].TaskID != "task_sql_001" {
		t.Fatalf("sqlite ListTasksBySession returned total=%d items=%+v err=%v", total, items, err)
	}
}
