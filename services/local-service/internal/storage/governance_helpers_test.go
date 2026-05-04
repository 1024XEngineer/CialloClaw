package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestInMemoryGovernanceAndToolStoresCoverHelpers(t *testing.T) {
	approvalStore := newInMemoryApprovalRequestStore()
	if err := approvalStore.WriteApprovalRequest(context.Background(), ApprovalRequestRecord{
		ApprovalID:    "appr_mem_001",
		TaskID:        "task_001",
		OperationName: "write_file",
		Status:        "pending",
		CreatedAt:     "2026-04-17T09:59:00Z",
		UpdatedAt:     "2026-04-17T09:59:00Z",
	}); err != nil {
		t.Fatalf("write in-memory approval request failed: %v", err)
	}
	if err := approvalStore.WriteApprovalRequest(context.Background(), ApprovalRequestRecord{
		ApprovalID:    "appr_mem_002",
		TaskID:        "task_001",
		OperationName: "restore_apply",
		Status:        "approved",
		CreatedAt:     "2026-04-17T10:00:30Z",
		UpdatedAt:     "2026-04-17T10:00:30Z",
	}); err != nil {
		t.Fatalf("write second in-memory approval request failed: %v", err)
	}
	if err := approvalStore.UpdateApprovalRequestStatus(context.Background(), "appr_mem_001", "denied", "2026-04-17T10:01:00Z"); err != nil {
		t.Fatalf("update in-memory approval request failed: %v", err)
	}
	approvalItems, approvalTotal, err := approvalStore.ListApprovalRequests(context.Background(), "task_001", 10, 0)
	if err != nil || approvalTotal != 2 || len(approvalItems) != 2 {
		t.Fatalf("unexpected in-memory approval list: total=%d items=%+v err=%v", approvalTotal, approvalItems, err)
	}
	if approvalItems[1].Status != "denied" || approvalItems[1].UpdatedAt != "2026-04-17T10:01:00Z" {
		t.Fatalf("expected approval status update to persist, got %+v", approvalItems[1])
	}
	pendingApprovalItems, pendingApprovalTotal, err := approvalStore.ListPendingApprovalRequests(context.Background(), 10, 0)
	if err != nil || pendingApprovalTotal != 0 || len(pendingApprovalItems) != 0 {
		t.Fatalf("expected no pending approvals after status updates, got total=%d items=%+v err=%v", pendingApprovalTotal, pendingApprovalItems, err)
	}

	authorizationStore := newInMemoryAuthorizationRecordStore()
	if err := authorizationStore.WriteAuthorizationRecord(context.Background(), AuthorizationRecordRecord{
		AuthorizationRecordID: "auth_mem_001",
		TaskID:                "task_001",
		RunID:                 "run_001",
		ApprovalID:            "appr_mem_001",
		Decision:              "allow_once",
		Operator:              "user",
		RememberRule:          true,
		CreatedAt:             "2026-04-17T10:02:00Z",
	}); err != nil {
		t.Fatalf("write in-memory authorization record failed: %v", err)
	}
	if err := authorizationStore.WriteAuthorizationRecord(context.Background(), AuthorizationRecordRecord{
		AuthorizationRecordID: "auth_mem_002",
		TaskID:                "task_001",
		RunID:                 "run_002",
		ApprovalID:            "appr_mem_002",
		Decision:              "deny_once",
		Operator:              "user",
		CreatedAt:             "2026-04-17T10:03:00Z",
	}); err != nil {
		t.Fatalf("write second in-memory authorization record failed: %v", err)
	}
	authorizationItems, authorizationTotal, err := authorizationStore.ListAuthorizationRecords(context.Background(), "task_001", "", 10, 0)
	if err != nil || authorizationTotal != 2 || len(authorizationItems) != 2 {
		t.Fatalf("unexpected in-memory authorization list: total=%d items=%+v err=%v", authorizationTotal, authorizationItems, err)
	}
	if authorizationItems[0].AuthorizationRecordID != "auth_mem_002" {
		t.Fatalf("expected authorization records sorted by newest first, got %+v", authorizationItems)
	}

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
	auditItems, auditTotal, err := auditStore.ListAuditRecords(context.Background(), "task_001", "", 10, 0)
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

	approvalPaged := pageApprovalRequests([]ApprovalRequestRecord{{ApprovalID: "a1"}, {ApprovalID: "a2"}}, 0, 1)
	if len(approvalPaged) != 1 || approvalPaged[0].ApprovalID != "a2" {
		t.Fatalf("unexpected approval helper page: %+v", approvalPaged)
	}
	if got := pageApprovalRequests([]ApprovalRequestRecord{{ApprovalID: "a1"}}, 1, 4); got != nil {
		t.Fatalf("expected nil approval page when offset exceeds length, got %+v", got)
	}

	authorizationPaged := pageAuthorizationRecords([]AuthorizationRecordRecord{{AuthorizationRecordID: "auth1"}, {AuthorizationRecordID: "auth2"}}, 0, 1)
	if len(authorizationPaged) != 1 || authorizationPaged[0].AuthorizationRecordID != "auth2" {
		t.Fatalf("unexpected authorization helper page: %+v", authorizationPaged)
	}
	if got := pageAuthorizationRecords([]AuthorizationRecordRecord{{AuthorizationRecordID: "auth1"}}, 1, 4); got != nil {
		t.Fatalf("expected nil authorization page when offset exceeds length, got %+v", got)
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

	filtered := filterApprovalRequests([]ApprovalRequestRecord{{ApprovalID: "ap1", TaskID: "task_001", Status: "pending", CreatedAt: "2026-04-17T10:00:00Z"}, {ApprovalID: "ap2", TaskID: "task_001", Status: "approved", CreatedAt: "2026-04-17T10:01:00Z"}, {ApprovalID: "ap3", TaskID: "task_002", Status: "pending", CreatedAt: "2026-04-17T10:02:00Z"}}, "task_001", "pending")
	if len(filtered) != 1 || filtered[0].ApprovalID != "ap1" {
		t.Fatalf("unexpected filtered approvals: %+v", filtered)
	}
}

func TestInMemoryAuthorizationDecisionIsAtomic(t *testing.T) {
	state := &inMemoryGovernanceState{
		approvalRequests:     make([]ApprovalRequestRecord, 0),
		authorizationRecords: make([]AuthorizationRecordRecord, 0),
	}
	approvalStore := newInMemoryApprovalRequestStoreWithState(state)
	authorizationStore := newInMemoryAuthorizationRecordStoreWithState(state)
	if err := approvalStore.WriteApprovalRequest(context.Background(), ApprovalRequestRecord{
		ApprovalID:    "appr_atomic_001",
		TaskID:        "task_atomic_001",
		OperationName: "write_file",
		Status:        "pending",
		CreatedAt:     "2026-04-18T10:00:00Z",
		UpdatedAt:     "2026-04-18T10:00:00Z",
	}); err != nil {
		t.Fatalf("write in-memory approval request failed: %v", err)
	}
	if err := authorizationStore.WriteAuthorizationDecision(context.Background(), AuthorizationRecordRecord{
		AuthorizationRecordID: "auth_atomic_001",
		TaskID:                "task_atomic_001",
		RunID:                 "run_atomic_001",
		ApprovalID:            "appr_atomic_001",
		Decision:              "allow_once",
		Operator:              "user",
		CreatedAt:             "2026-04-18T10:01:00Z",
	}, "approved", "2026-04-18T10:01:00Z"); err != nil {
		t.Fatalf("write in-memory authorization decision failed: %v", err)
	}

	approvalItems, approvalTotal, err := approvalStore.ListApprovalRequests(context.Background(), "task_atomic_001", 10, 0)
	if err != nil || approvalTotal != 1 || len(approvalItems) != 1 {
		t.Fatalf("unexpected in-memory approval decision state: total=%d items=%+v err=%v", approvalTotal, approvalItems, err)
	}
	if approvalItems[0].Status != "approved" {
		t.Fatalf("expected in-memory approval to be updated atomically, got %+v", approvalItems[0])
	}
	authorizationItems, authorizationTotal, err := authorizationStore.ListAuthorizationRecords(context.Background(), "task_atomic_001", "", 10, 0)
	if err != nil || authorizationTotal != 1 || len(authorizationItems) != 1 {
		t.Fatalf("unexpected in-memory authorization decision history: total=%d items=%+v err=%v", authorizationTotal, authorizationItems, err)
	}

	err = authorizationStore.WriteAuthorizationDecision(context.Background(), AuthorizationRecordRecord{
		AuthorizationRecordID: "auth_atomic_002",
		TaskID:                "task_atomic_001",
		RunID:                 "run_atomic_missing",
		ApprovalID:            "appr_missing",
		Decision:              "deny_once",
		Operator:              "user",
		CreatedAt:             "2026-04-18T10:02:00Z",
	}, "denied", "2026-04-18T10:02:00Z")
	if !errors.Is(err, ErrApprovalRequestNotFound) {
		t.Fatalf("expected missing approval to fail atomic write, got %v", err)
	}
	authorizationItems, authorizationTotal, err = authorizationStore.ListAuthorizationRecords(context.Background(), "task_atomic_001", "", 10, 0)
	if err != nil || authorizationTotal != 1 || len(authorizationItems) != 1 {
		t.Fatalf("expected failed in-memory atomic write to leave history unchanged, total=%d items=%+v err=%v", authorizationTotal, authorizationItems, err)
	}
}
