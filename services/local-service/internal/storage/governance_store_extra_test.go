package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
)

func TestRecoveryPointStoresGetRecoveryPointByID(t *testing.T) {
	inMemory := newInMemoryRecoveryPointStore()
	point := checkpoint.RecoveryPoint{RecoveryPointID: "rp_mem_001", TaskID: "task_mem_001", Summary: "before write", CreatedAt: "2026-04-21T10:00:00Z", Objects: []string{"workspace://snapshot"}}
	if err := inMemory.WriteRecoveryPoint(context.Background(), point); err != nil {
		t.Fatalf("in-memory WriteRecoveryPoint returned error: %v", err)
	}
	stored, err := inMemory.GetRecoveryPoint(context.Background(), "rp_mem_001")
	if err != nil || stored.TaskID != "task_mem_001" {
		t.Fatalf("in-memory GetRecoveryPoint returned point=%+v err=%v", stored, err)
	}

	sqliteStore, err := NewSQLiteRecoveryPointStore(filepath.Join(t.TempDir(), "recovery-points.db"))
	if err != nil {
		t.Fatalf("NewSQLiteRecoveryPointStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()
	if err := sqliteStore.WriteRecoveryPoint(context.Background(), checkpoint.RecoveryPoint{RecoveryPointID: "rp_sql_001", TaskID: "task_sql_001", Summary: "before write", CreatedAt: "2026-04-21T10:00:00Z", Objects: []string{"workspace://snapshot"}}); err != nil {
		t.Fatalf("sqlite WriteRecoveryPoint returned error: %v", err)
	}
	stored, err = sqliteStore.GetRecoveryPoint(context.Background(), "rp_sql_001")
	if err != nil || stored.TaskID != "task_sql_001" {
		t.Fatalf("sqlite GetRecoveryPoint returned point=%+v err=%v", stored, err)
	}
}
