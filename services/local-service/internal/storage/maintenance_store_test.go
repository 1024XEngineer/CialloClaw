package storage

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestServiceDeleteAllMemory(t *testing.T) {
	tests := []struct {
		name     string
		newStore func(t *testing.T) *Service
	}{
		{
			name: "in-memory",
			newStore: func(t *testing.T) *Service {
				t.Helper()
				return NewService(nil)
			},
		},
		{
			name: "sqlite",
			newStore: func(t *testing.T) *Service {
				t.Helper()
				service := NewService(platform.NewLocalStorageAdapter(filepath.Join(t.TempDir(), "memory-cleanup.db")))
				t.Cleanup(func() { _ = service.Close() })
				return service
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := test.newStore(t)
			ctx := context.Background()
			if err := service.MemoryStore().SaveSummary(ctx, MemorySummaryRecord{
				MemorySummaryID: "mem_001",
				TaskID:          "task_001",
				RunID:           "run_001",
				Summary:         "prefer concise output",
				CreatedAt:       "2026-04-18T10:00:00Z",
			}); err != nil {
				t.Fatalf("SaveSummary returned error: %v", err)
			}
			if err := service.MemoryStore().SaveRetrievalHits(ctx, []MemoryRetrievalRecord{{
				RetrievalHitID: "hit_001",
				TaskID:         "task_001",
				RunID:          "run_001",
				MemoryID:       "mem_001",
				Score:          0.9,
				Source:         "fts",
				Summary:        "prefer concise output",
				CreatedAt:      "2026-04-18T10:00:01Z",
			}}); err != nil {
				t.Fatalf("SaveRetrievalHits returned error: %v", err)
			}

			if err := service.DeleteAllMemory(ctx); err != nil {
				t.Fatalf("DeleteAllMemory returned error: %v", err)
			}
			summaries, err := service.MemoryStore().ListRecentSummaries(ctx, 10)
			if err != nil {
				t.Fatalf("ListRecentSummaries returned error: %v", err)
			}
			if len(summaries) != 0 {
				t.Fatalf("expected cleared memory summaries, got %+v", summaries)
			}
			hits, err := service.MemoryStore().SearchSummaries(ctx, "", "", "concise", 10)
			if err != nil {
				t.Fatalf("SearchSummaries returned error: %v", err)
			}
			if len(hits) != 0 {
				t.Fatalf("expected cleared retrieval hits, got %+v", hits)
			}
		})
	}
}

func TestServiceDeleteAllTaskHistory(t *testing.T) {
	tests := []struct {
		name     string
		newStore func(t *testing.T) *Service
	}{
		{
			name: "in-memory",
			newStore: func(t *testing.T) *Service {
				t.Helper()
				return NewService(nil)
			},
		},
		{
			name: "sqlite",
			newStore: func(t *testing.T) *Service {
				t.Helper()
				service := NewService(platform.NewLocalStorageAdapter(filepath.Join(t.TempDir(), "task-history-cleanup.db")))
				t.Cleanup(func() { _ = service.Close() })
				return service
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := test.newStore(t)
			ctx := context.Background()
			if err := service.SessionStore().WriteSession(ctx, SessionRecord{SessionID: "session_001", Title: "Session", Status: "active", CreatedAt: "2026-04-18T10:00:00Z", UpdatedAt: "2026-04-18T10:00:00Z"}); err != nil {
				t.Fatalf("WriteSession returned error: %v", err)
			}
			if err := service.TaskStore().WriteTask(ctx, TaskRecord{TaskID: "task_001", SessionID: "session_001", RunID: "run_001", Title: "Task", SourceType: "hover_input", Status: "completed", StartedAt: "2026-04-18T10:00:00Z", UpdatedAt: "2026-04-18T10:01:00Z"}); err != nil {
				t.Fatalf("WriteTask returned error: %v", err)
			}
			if err := service.TaskStepStore().ReplaceTaskSteps(ctx, "task_001", []TaskStepRecord{{StepID: "step_001", TaskID: "task_001", Name: "collect", Status: "completed", OrderIndex: 0, CreatedAt: "2026-04-18T10:00:00Z", UpdatedAt: "2026-04-18T10:01:00Z"}}); err != nil {
				t.Fatalf("ReplaceTaskSteps returned error: %v", err)
			}
			if err := service.LoopRuntimeStore().SaveRun(ctx, RunRecord{RunID: "run_001", TaskID: "task_001", SessionID: "session_001", SourceType: "hover_input", Status: "completed", IntentName: "agent_loop", StartedAt: "2026-04-18T10:00:00Z", UpdatedAt: "2026-04-18T10:01:00Z"}); err != nil {
				t.Fatalf("SaveRun returned error: %v", err)
			}
			if err := service.LoopRuntimeStore().SaveEvents(ctx, []EventRecord{{EventID: "event_001", RunID: "run_001", TaskID: "task_001", Type: "loop.completed", Level: "info", PayloadJSON: `{}`, CreatedAt: "2026-04-18T10:01:00Z"}}); err != nil {
				t.Fatalf("SaveEvents returned error: %v", err)
			}
			if err := service.LoopRuntimeStore().SaveDeliveryResult(ctx, DeliveryResultRecord{DeliveryResultID: "delivery_001", TaskID: "task_001", Type: "bubble", Title: "Done", PayloadJSON: `{}`, CreatedAt: "2026-04-18T10:01:01Z"}); err != nil {
				t.Fatalf("SaveDeliveryResult returned error: %v", err)
			}
			if err := service.LoopRuntimeStore().ReplaceTaskCitations(ctx, "task_001", []CitationRecord{{CitationID: "citation_001", TaskID: "task_001", SourceType: "artifact", SourceRef: "artifact_001", Label: "evidence"}}); err != nil {
				t.Fatalf("ReplaceTaskCitations returned error: %v", err)
			}
			if err := service.ToolCallStore().SaveToolCall(ctx, tools.ToolCallRecord{ToolCallID: "tool_001", RunID: "run_001", TaskID: "task_001", CreatedAt: "2026-04-18T10:00:30Z", ToolName: "read_file", Status: tools.ToolCallStatusSucceeded, DurationMS: 5}); err != nil {
				t.Fatalf("SaveToolCall returned error: %v", err)
			}
			if err := service.ArtifactStore().SaveArtifacts(ctx, []ArtifactRecord{{ArtifactID: "artifact_001", TaskID: "task_001", ArtifactType: "generated_doc", Title: "Artifact", Path: "workspace/output.md", MimeType: "text/markdown", DeliveryType: "bubble", DeliveryPayloadJSON: `{}`, CreatedAt: "2026-04-18T10:01:02Z"}}); err != nil {
				t.Fatalf("SaveArtifacts returned error: %v", err)
			}
			if err := service.AuditWriter().WriteAuditRecord(ctx, audit.Record{AuditID: "audit_001", TaskID: "task_001", Type: "file", Action: "write_file", Summary: "audit", Target: "workspace/output.md", Result: "success", CreatedAt: "2026-04-18T10:01:03Z"}); err != nil {
				t.Fatalf("WriteAuditRecord returned error: %v", err)
			}
			if err := service.RecoveryPointWriter().WriteRecoveryPoint(ctx, checkpoint.RecoveryPoint{RecoveryPointID: "rp_001", TaskID: "task_001", Summary: "before cleanup", CreatedAt: "2026-04-18T10:01:04Z", Objects: []string{"workspace/output.md"}}); err != nil {
				t.Fatalf("WriteRecoveryPoint returned error: %v", err)
			}
			if err := service.ApprovalRequestStore().WriteApprovalRequest(ctx, ApprovalRequestRecord{ApprovalID: "appr_001", TaskID: "task_001", OperationName: "task_history_delete", RiskLevel: "red", TargetObject: "task_history", Reason: "cleanup", Status: "pending", CreatedAt: "2026-04-18T10:01:05Z", UpdatedAt: "2026-04-18T10:01:05Z"}); err != nil {
				t.Fatalf("WriteApprovalRequest returned error: %v", err)
			}
			if err := service.AuthorizationRecordStore().WriteAuthorizationRecord(ctx, AuthorizationRecordRecord{AuthorizationRecordID: "auth_001", TaskID: "task_001", ApprovalID: "appr_001", Decision: "allow_once", Operator: "user", CreatedAt: "2026-04-18T10:01:06Z"}); err != nil {
				t.Fatalf("WriteAuthorizationRecord returned error: %v", err)
			}
			if err := service.MirrorConversationStore().SaveMirrorConversation(ctx, MirrorConversationRecord{RecordID: "mirror_001", TraceID: "trace_001", CreatedAt: "2026-04-18T10:01:07Z", UpdatedAt: "2026-04-18T10:01:08Z", Source: "dashboard", Trigger: "voice_commit", InputMode: "voice", TaskID: "task_001", UserText: "history", Status: "responded"}); err != nil {
				t.Fatalf("SaveMirrorConversation returned error: %v", err)
			}

			if err := service.DeleteAllTaskHistory(ctx); err != nil {
				t.Fatalf("DeleteAllTaskHistory returned error: %v", err)
			}

			if items, total, err := service.SessionStore().ListSessions(ctx, 20, 0); err != nil || total != 0 || len(items) != 0 {
				t.Fatalf("expected cleared sessions, got total=%d items=%+v err=%v", total, items, err)
			}
			if items, total, err := service.TaskStore().ListTasks(ctx, 20, 0); err != nil || total != 0 || len(items) != 0 {
				t.Fatalf("expected cleared tasks, got total=%d items=%+v err=%v", total, items, err)
			}
			if items, total, err := service.TaskStepStore().ListTaskSteps(ctx, "task_001", 20, 0); err != nil || total != 0 || len(items) != 0 {
				t.Fatalf("expected cleared task steps, got total=%d items=%+v err=%v", total, items, err)
			}
			if _, err := service.LoopRuntimeStore().GetRun(ctx, "run_001"); err == nil {
				t.Fatal("expected cleared run record")
			}
			if items, total, err := service.LoopRuntimeStore().ListEvents(ctx, "task_001", "", "", "", "", 20, 0); err != nil || total != 0 || len(items) != 0 {
				t.Fatalf("expected cleared events, got total=%d items=%+v err=%v", total, items, err)
			}
			if items, total, err := service.LoopRuntimeStore().ListDeliveryResults(ctx, "task_001", 20, 0); err != nil || total != 0 || len(items) != 0 {
				t.Fatalf("expected cleared delivery results, got total=%d items=%+v err=%v", total, items, err)
			}
			if items, err := service.LoopRuntimeStore().ListTaskCitations(ctx, "task_001"); err != nil || len(items) != 0 {
				t.Fatalf("expected cleared citations, got %+v err=%v", items, err)
			}
			if items, total, err := service.ToolCallStore().ListToolCalls(ctx, "task_001", "", 20, 0); err != nil || total != 0 || len(items) != 0 {
				t.Fatalf("expected cleared tool calls, got total=%d items=%+v err=%v", total, items, err)
			}
			if items, total, err := service.ArtifactStore().ListArtifacts(ctx, "task_001", 20, 0); err != nil || total != 0 || len(items) != 0 {
				t.Fatalf("expected cleared artifacts, got total=%d items=%+v err=%v", total, items, err)
			}
			if items, total, err := service.AuditStore().ListAuditRecords(ctx, "task_001", 20, 0); err != nil || total != 0 || len(items) != 0 {
				t.Fatalf("expected cleared audit records, got total=%d items=%+v err=%v", total, items, err)
			}
			if items, total, err := service.RecoveryPointStore().ListRecoveryPoints(ctx, "task_001", 20, 0); err != nil || total != 0 || len(items) != 0 {
				t.Fatalf("expected cleared recovery points, got total=%d items=%+v err=%v", total, items, err)
			}
			if items, total, err := service.ApprovalRequestStore().ListApprovalRequests(ctx, "task_001", 20, 0); err != nil || total != 0 || len(items) != 0 {
				t.Fatalf("expected cleared approval requests, got total=%d items=%+v err=%v", total, items, err)
			}
			if items, total, err := service.AuthorizationRecordStore().ListAuthorizationRecords(ctx, "task_001", 20, 0); err != nil || total != 0 || len(items) != 0 {
				t.Fatalf("expected cleared authorization records, got total=%d items=%+v err=%v", total, items, err)
			}
			if items, total, err := service.MirrorConversationStore().ListMirrorConversations(ctx, "task_001", "", "", 20, 0); err != nil || total != 0 || len(items) != 0 {
				t.Fatalf("expected cleared mirror conversations, got total=%d items=%+v err=%v", total, items, err)
			}
		})
	}
}
