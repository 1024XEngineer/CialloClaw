package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestInMemoryTaskRunStoreSaveLoadAndAllocate(t *testing.T) {
	store := NewInMemoryTaskRunStore()

	firstID, err := store.AllocateIdentifier(context.Background(), "task")
	if err != nil {
		t.Fatalf("AllocateIdentifier returned error: %v", err)
	}
	secondID, err := store.AllocateIdentifier(context.Background(), "task")
	if err != nil {
		t.Fatalf("AllocateIdentifier returned error: %v", err)
	}
	if firstID != "task_001" || secondID != "task_002" {
		t.Fatalf("expected sequential in-memory identifiers, got %q and %q", firstID, secondID)
	}

	record := sampleTaskRunRecord()
	if err := store.SaveTaskRun(context.Background(), record); err != nil {
		t.Fatalf("SaveTaskRun returned error: %v", err)
	}

	records, err := store.LoadTaskRuns(context.Background())
	if err != nil {
		t.Fatalf("LoadTaskRuns returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one task run record, got %d", len(records))
	}
	if records[0].TaskID != record.TaskID || records[0].RunID != record.RunID {
		t.Fatalf("unexpected task run record: %+v", records[0])
	}
}

func TestNewSQLiteTaskRunStoreInitializesWALMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-runs.db")
	store, err := NewSQLiteTaskRunStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteTaskRunStore returned error: %v", err)
	}
	defer func() { _ = store.Close() }()

	mode, err := store.journalMode(context.Background())
	if err != nil {
		t.Fatalf("journalMode returned error: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("expected wal journal mode, got %q", mode)
	}

	assertTableExists(t, store.db, sqliteTaskRunTableName)
	assertTableExists(t, store.db, sqliteEngineSequenceTableName)
}

func TestSQLiteTaskRunStoreSaveLoadAndAllocate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-runs.db")
	store, err := NewSQLiteTaskRunStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteTaskRunStore returned error: %v", err)
	}
	defer func() { _ = store.Close() }()

	taskID, err := store.AllocateIdentifier(context.Background(), "task")
	if err != nil {
		t.Fatalf("AllocateIdentifier returned error: %v", err)
	}
	runID, err := store.AllocateIdentifier(context.Background(), "run")
	if err != nil {
		t.Fatalf("AllocateIdentifier returned error: %v", err)
	}
	if taskID != "task_001" || runID != "run_001" {
		t.Fatalf("expected sequential sqlite identifiers, got %q and %q", taskID, runID)
	}

	record := sampleTaskRunRecord()
	record.TaskID = taskID
	record.RunID = runID
	if err := store.SaveTaskRun(context.Background(), record); err != nil {
		t.Fatalf("SaveTaskRun returned error: %v", err)
	}

	records, err := store.LoadTaskRuns(context.Background())
	if err != nil {
		t.Fatalf("LoadTaskRuns returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one task run record, got %d", len(records))
	}
	if records[0].TaskID != taskID || records[0].RunID != runID {
		t.Fatalf("unexpected loaded record: %+v", records[0])
	}
	if records[0].DeliveryResult["type"] != "workspace_document" {
		t.Fatalf("expected delivery result to round-trip, got %+v", records[0].DeliveryResult)
	}
	if len(records[0].Artifacts) != 1 || records[0].Artifacts[0]["artifact_id"] != "art_001" {
		t.Fatalf("expected artifacts to round-trip, got %+v", records[0].Artifacts)
	}
	if len(records[0].Notifications) != 1 || records[0].Notifications[0].Method != "task.updated" {
		t.Fatalf("expected notifications to round-trip, got %+v", records[0].Notifications)
	}
}

func TestSQLiteTaskRunStoreRejectsInvalidRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-runs-invalid.db")
	store, err := NewSQLiteTaskRunStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteTaskRunStore returned error: %v", err)
	}
	defer func() { _ = store.Close() }()

	record := sampleTaskRunRecord()
	record.RunID = ""
	if err := store.SaveTaskRun(context.Background(), record); err != ErrTaskRunRunIDRequired {
		t.Fatalf("expected ErrTaskRunRunIDRequired, got %v", err)
	}
}

func sampleTaskRunRecord() TaskRunRecord {
	startedAt := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	updatedAt := startedAt.Add(2 * time.Minute)
	finishedAt := updatedAt.Add(3 * time.Minute)

	return TaskRunRecord{
		TaskID:            "task_001",
		SessionID:         "sess_001",
		RunID:             "run_001",
		Title:             "sqlite task record",
		SourceType:        "hover_input",
		Status:            "completed",
		Intent:            map[string]any{"name": "summarize", "arguments": map[string]any{"style": "key_points"}},
		PreferredDelivery: "workspace_document",
		FallbackDelivery:  "bubble",
		CurrentStep:       "return_result",
		RiskLevel:         "yellow",
		StartedAt:         startedAt,
		UpdatedAt:         updatedAt,
		FinishedAt:        &finishedAt,
		Timeline: []TaskStepSnapshot{{
			StepID:        "step_001",
			TaskID:        "task_001",
			Name:          "return_result",
			Status:        "completed",
			OrderIndex:    1,
			InputSummary:  "task input",
			OutputSummary: "task output",
		}},
		BubbleMessage:  map[string]any{"task_id": "task_001", "type": "result", "text": "completed"},
		DeliveryResult: map[string]any{"type": "workspace_document", "payload": map[string]any{"path": "workspace/result.md"}},
		Artifacts:      []map[string]any{{"artifact_id": "art_001", "task_id": "task_001"}},
		MirrorReferences: []map[string]any{{
			"memory_id": "mem_001",
		}},
		SecuritySummary:  map[string]any{"security_status": "recoverable", "risk_level": "yellow"},
		Authorization:    map[string]any{"decision": "allow_once"},
		ImpactScope:      map[string]any{"files": []string{"workspace/result.md"}},
		MemoryReadPlans:  []map[string]any{{"kind": "retrieval"}},
		MemoryWritePlans: []map[string]any{{"kind": "summary_write"}},
		StorageWritePlan: map[string]any{"target_path": "workspace/result.md"},
		ArtifactPlans:    []map[string]any{{"artifact_id": "art_001"}},
		Notifications: []NotificationSnapshot{{
			Method:    "task.updated",
			Params:    map[string]any{"task_id": "task_001", "status": "completed"},
			CreatedAt: updatedAt,
		}},
		LatestEvent:       map[string]any{"type": "delivery.ready"},
		LatestToolCall:    map[string]any{"tool_name": "write_file"},
		CurrentStepStatus: "completed",
	}
}
