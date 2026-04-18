package storage

import (
	"context"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestInMemoryGovernanceAndToolStoresCoverHelpers(t *testing.T) {
	auditStore := newInMemoryAuditStore()
	if err := auditStore.WriteAuditRecord(context.Background(), audit.Record{
		AuditID:   "audit_mem_001",
		TaskID:    "task_001",
		Type:      "file",
		Action:    "write_file",
		Summary:   "write one",
		Target:    "workspace/one.md",
		Result:    "success",
		CreatedAt: "2026-04-17T10:00:00Z",
	}); err != nil {
		t.Fatalf("write in-memory audit failed: %v", err)
	}
	auditItems, auditTotal, err := auditStore.ListAuditRecords(context.Background(), "task_001", 10, 0)
	if err != nil || auditTotal != 1 || len(auditItems) != 1 {
		t.Fatalf("unexpected in-memory audit list: total=%d items=%+v err=%v", auditTotal, auditItems, err)
	}

	recoveryStore := newInMemoryRecoveryPointStore()
	if err := recoveryStore.WriteRecoveryPoint(context.Background(), checkpoint.RecoveryPoint{
		RecoveryPointID: "rp_mem_001",
		TaskID:          "task_001",
		Summary:         "before overwrite",
		CreatedAt:       "2026-04-17T10:01:00Z",
		Objects:         []string{"workspace/one.md"},
	}); err != nil {
		t.Fatalf("write in-memory recovery point failed: %v", err)
	}
	recoveryItems, recoveryTotal, err := recoveryStore.ListRecoveryPoints(context.Background(), "task_001", 10, 0)
	if err != nil || recoveryTotal != 1 || len(recoveryItems) != 1 {
		t.Fatalf("unexpected in-memory recovery point list: total=%d items=%+v err=%v", recoveryTotal, recoveryItems, err)
	}
	point, err := recoveryStore.GetRecoveryPoint(context.Background(), "rp_mem_001")
	if err != nil || point.RecoveryPointID != "rp_mem_001" {
		t.Fatalf("unexpected in-memory recovery point get: point=%+v err=%v", point, err)
	}
	missingPoint, err := recoveryStore.GetRecoveryPoint(context.Background(), "rp_missing")
	if err == nil || missingPoint.RecoveryPointID != "" {
		t.Fatalf("expected missing in-memory recovery point lookup to fail, got point=%+v err=%v", missingPoint, err)
	}

	toolStore := newInMemoryToolCallStore()
	if err := toolStore.SaveToolCall(context.Background(), tools.ToolCallRecord{
		ToolCallID: "tool_mem_001",
		TaskID:     "task_001",
		RunID:      "run_001",
		StepID:     "step_001",
		ToolName:   "read_file",
		Status:     tools.ToolCallStatusSucceeded,
		Input:      map[string]any{"path": "workspace/one.md"},
		Output:     map[string]any{"bytes": 10},
	}); err != nil {
		t.Fatalf("write in-memory tool call failed: %v", err)
	}

	auditPaged := pageAuditRecords([]audit.Record{{AuditID: "a1"}, {AuditID: "a2"}}, 0, 1)
	if len(auditPaged) != 1 || auditPaged[0].AuditID != "a2" {
		t.Fatalf("unexpected audit helper page: %+v", auditPaged)
	}
	if got := pageAuditRecords([]audit.Record{{AuditID: "a1"}}, 1, 4); got != nil {
		t.Fatalf("expected nil audit page when offset exceeds length, got %+v", got)
	}

	recoveryPaged := pageRecoveryPoints([]checkpoint.RecoveryPoint{{RecoveryPointID: "rp1"}, {RecoveryPointID: "rp2"}}, 0, 1)
	if len(recoveryPaged) != 1 || recoveryPaged[0].RecoveryPointID != "rp2" {
		t.Fatalf("unexpected recovery helper page: %+v", recoveryPaged)
	}
	if got := pageRecoveryPoints([]checkpoint.RecoveryPoint{{RecoveryPointID: "rp1"}}, 1, 4); got != nil {
		t.Fatalf("expected nil recovery page when offset exceeds length, got %+v", got)
	}

	if args := firstArg(""); args != nil {
		t.Fatalf("expected nil firstArg for empty task id, got %+v", args)
	}
	if args := firstArg("task_001"); len(args) != 1 || args[0] != "task_001" {
		t.Fatalf("unexpected firstArg result: %+v", args)
	}

	if parsed := parseGovernanceTime("2026-04-17T10:00:00.123456789Z"); parsed.IsZero() {
		t.Fatal("expected RFC3339Nano governance time to parse")
	}
	if parsed := parseGovernanceTime("2026-04-17T10:00:00Z"); parsed.IsZero() {
		t.Fatal("expected RFC3339 governance time to parse")
	}
	if parsed := parseGovernanceTime("not-a-time"); !parsed.Equal(time.Time{}) {
		t.Fatalf("expected invalid governance time to return zero value, got %v", parsed)
	}
}
