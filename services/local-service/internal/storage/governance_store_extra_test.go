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

func TestRecoveryPointStoresPreserveUnknownMode(t *testing.T) {
	tests := []struct {
		name     string
		newStore func(t *testing.T) interface {
			WriteRecoveryPoint(ctx context.Context, point checkpoint.RecoveryPoint) error
			GetRecoveryPoint(ctx context.Context, recoveryPointID string) (checkpoint.RecoveryPoint, error)
		}
	}{
		{
			name: "in-memory",
			newStore: func(t *testing.T) interface {
				WriteRecoveryPoint(ctx context.Context, point checkpoint.RecoveryPoint) error
				GetRecoveryPoint(ctx context.Context, recoveryPointID string) (checkpoint.RecoveryPoint, error)
			} {
				t.Helper()
				return newInMemoryRecoveryPointStore()
			},
		},
		{
			name: "sqlite",
			newStore: func(t *testing.T) interface {
				WriteRecoveryPoint(ctx context.Context, point checkpoint.RecoveryPoint) error
				GetRecoveryPoint(ctx context.Context, recoveryPointID string) (checkpoint.RecoveryPoint, error)
			} {
				t.Helper()
				store, err := NewSQLiteRecoveryPointStore(filepath.Join(t.TempDir(), "recovery-points-mode.db"))
				if err != nil {
					t.Fatalf("NewSQLiteRecoveryPointStore returned error: %v", err)
				}
				t.Cleanup(func() { _ = store.Close() })
				return store
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := test.newStore(t)
			if err := store.WriteRecoveryPoint(context.Background(), checkpoint.RecoveryPoint{
				RecoveryPointID: "rp_unknown_mode",
				TaskID:          "task_unknown_mode",
				Summary:         "before unknown mode",
				CreatedAt:       "2026-04-21T10:00:00Z",
				Mode:            "future_mode",
				Objects:         []string{"workspace://snapshot"},
			}); err != nil {
				t.Fatalf("WriteRecoveryPoint returned error: %v", err)
			}
			stored, err := store.GetRecoveryPoint(context.Background(), "rp_unknown_mode")
			if err != nil {
				t.Fatalf("GetRecoveryPoint returned error: %v", err)
			}
			if stored.Mode != "future_mode" {
				t.Fatalf("expected unknown recovery mode to round-trip, got %+v", stored)
			}
		})
	}
}
