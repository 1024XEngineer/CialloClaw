package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	taskcontext "github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
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
	loadedRecord, err := store.GetTaskRun(context.Background(), record.TaskID)
	if err != nil || loadedRecord.RequestSource != "floating_ball" || loadedRecord.RequestTrigger != "hover_text_input" {
		t.Fatalf("expected GetTaskRun to preserve request metadata, record=%+v err=%v", loadedRecord, err)
	}
	taskItems, taskTotal, err := store.taskStore.ListTasks(context.Background(), 10, 0)
	if err != nil || taskTotal != 1 || len(taskItems) != 1 {
		t.Fatalf("expected one first-class task record, got total=%d items=%+v err=%v", taskTotal, taskItems, err)
	}
	if taskItems[0].TaskID != record.TaskID || taskItems[0].IntentName != "summarize" || taskItems[0].PrimaryRunID != record.RunID || taskItems[0].RequestSource != "floating_ball" || taskItems[0].RequestTrigger != "hover_text_input" {
		t.Fatalf("unexpected first-class task record: %+v", taskItems[0])
	}
	stepItems, stepTotal, err := store.stepStore.ListTaskSteps(context.Background(), record.TaskID, 10, 0)
	if err != nil || stepTotal != 1 || len(stepItems) != 1 {
		t.Fatalf("expected one first-class task_step record, got total=%d items=%+v err=%v", stepTotal, stepItems, err)
	}
	if stepItems[0].StepID != record.Timeline[0].StepID || stepItems[0].OrderIndex != 1 {
		t.Fatalf("unexpected first-class task_step record: %+v", stepItems[0])
	}

	if err := store.DeleteTaskRun(context.Background(), record.TaskID); err != nil {
		t.Fatalf("DeleteTaskRun returned error: %v", err)
	}

	records, err = store.LoadTaskRuns(context.Background())
	if err != nil {
		t.Fatalf("LoadTaskRuns after delete returned error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected task run record to be deleted, got %d records", len(records))
	}
	taskItems, taskTotal, err = store.taskStore.ListTasks(context.Background(), 10, 0)
	if err != nil || taskTotal != 0 || len(taskItems) != 0 {
		t.Fatalf("expected task record to be deleted too, got total=%d items=%+v err=%v", taskTotal, taskItems, err)
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
	loadedRecord, err := store.GetTaskRun(context.Background(), taskID)
	if err != nil || loadedRecord.RequestSource != "floating_ball" || loadedRecord.RequestTrigger != "hover_text_input" {
		t.Fatalf("expected sqlite GetTaskRun to preserve request metadata, record=%+v err=%v", loadedRecord, err)
	}
	if records[0].ExecutionAttempt != 1 {
		t.Fatalf("expected execution attempt to default to 1, got %d", records[0].ExecutionAttempt)
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
	taskItems, taskTotal, err := store.taskStore.ListTasks(context.Background(), 10, 0)
	if err != nil || taskTotal != 1 || len(taskItems) != 1 {
		t.Fatalf("expected one structured task record, got total=%d items=%+v err=%v", taskTotal, taskItems, err)
	}
	if taskItems[0].TaskID != taskID || taskItems[0].RunID != runID || taskItems[0].CurrentStepStatus != "completed" {
		t.Fatalf("unexpected structured task record: %+v", taskItems[0])
	}
	stepItems, stepTotal, err := store.stepStore.ListTaskSteps(context.Background(), taskID, 10, 0)
	if err != nil || stepTotal != 1 || len(stepItems) != 1 {
		t.Fatalf("expected one structured task_step record, got total=%d items=%+v err=%v", stepTotal, stepItems, err)
	}
	if stepItems[0].TaskID != taskID || stepItems[0].Name != "return_result" {
		t.Fatalf("unexpected structured task_step record: %+v", stepItems[0])
	}
	updatedRecord := sampleTaskRunRecord()
	updatedRecord.TaskID = taskID
	updatedRecord.RunID = runID
	updatedRecord.Status = "processing"
	updatedRecord.CurrentStep = "draft_response"
	updatedRecord.CurrentStepStatus = "processing"
	updatedRecord.Timeline = []TaskStepSnapshot{{
		StepID:        "step_002",
		TaskID:        taskID,
		Name:          "draft_response",
		Status:        "processing",
		OrderIndex:    1,
		InputSummary:  "updated input",
		OutputSummary: "",
	}}
	if err := store.SaveTaskRun(context.Background(), updatedRecord); err != nil {
		t.Fatalf("SaveTaskRun update returned error: %v", err)
	}
	updatedStepItems, updatedStepTotal, err := store.stepStore.ListTaskSteps(context.Background(), taskID, 10, 0)
	if err != nil || updatedStepTotal != 1 || len(updatedStepItems) != 1 {
		t.Fatalf("expected replaced structured task_step records, got total=%d items=%+v err=%v", updatedStepTotal, updatedStepItems, err)
	}
	if updatedStepItems[0].StepID != "step_002" || updatedStepItems[0].Status != "processing" {
		t.Fatalf("expected task_steps to be replaced on update, got %+v", updatedStepItems[0])
	}

	if err := store.DeleteTaskRun(context.Background(), taskID); err != nil {
		t.Fatalf("DeleteTaskRun returned error: %v", err)
	}

	records, err = store.LoadTaskRuns(context.Background())
	if err != nil {
		t.Fatalf("LoadTaskRuns after delete returned error: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected sqlite task run record to be deleted, got %d records", len(records))
	}
	taskItems, taskTotal, err = store.taskStore.ListTasks(context.Background(), 10, 0)
	if err != nil || taskTotal != 0 || len(taskItems) != 0 {
		t.Fatalf("expected sqlite structured task record to be deleted, got total=%d items=%+v err=%v", taskTotal, taskItems, err)
	}
}

func TestTaskRunStoresLoadLegacyTaskRunsOnlyForMissingStructuredRows(t *testing.T) {
	inMemory := NewInMemoryTaskRunStore()
	first := sampleTaskRunRecord()
	second := sampleTaskRunRecord()
	second.TaskID = "task_legacy_only"
	second.RunID = "run_legacy_only"
	second.Timeline[0].StepID = "step_legacy_only"
	second.Timeline[0].TaskID = second.TaskID
	second.StartedAt = second.StartedAt.Add(5 * time.Minute)
	second.UpdatedAt = second.UpdatedAt.Add(5 * time.Minute)
	if err := inMemory.SaveTaskRun(context.Background(), first); err != nil {
		t.Fatalf("in-memory SaveTaskRun first returned error: %v", err)
	}
	if err := inMemory.SaveTaskRun(context.Background(), second); err != nil {
		t.Fatalf("in-memory SaveTaskRun second returned error: %v", err)
	}
	if err := inMemory.taskStore.DeleteTask(context.Background(), second.TaskID); err != nil {
		t.Fatalf("in-memory DeleteTask returned error: %v", err)
	}
	legacyRecords, err := inMemory.LoadLegacyTaskRuns(context.Background(), []string{first.TaskID})
	if err != nil || len(legacyRecords) != 1 || legacyRecords[0].TaskID != second.TaskID {
		t.Fatalf("expected in-memory legacy task run filter to keep only missing structured rows, records=%+v err=%v", legacyRecords, err)
	}

	path := filepath.Join(t.TempDir(), "task-runs-legacy.db")
	sqliteStore, err := NewSQLiteTaskRunStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteTaskRunStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()
	if err := sqliteStore.SaveTaskRun(context.Background(), first); err != nil {
		t.Fatalf("sqlite SaveTaskRun first returned error: %v", err)
	}
	if err := sqliteStore.SaveTaskRun(context.Background(), second); err != nil {
		t.Fatalf("sqlite SaveTaskRun second returned error: %v", err)
	}
	if err := sqliteStore.taskStore.DeleteTask(context.Background(), second.TaskID); err != nil {
		t.Fatalf("sqlite DeleteTask returned error: %v", err)
	}
	legacyRecords, err = sqliteStore.LoadLegacyTaskRuns(context.Background(), []string{first.TaskID})
	if err != nil || len(legacyRecords) != 1 || legacyRecords[0].TaskID != second.TaskID {
		t.Fatalf("expected sqlite legacy task run filter to keep only missing structured rows, records=%+v err=%v", legacyRecords, err)
	}
}

func TestTaskRunStoresCoverGetTaskRunMissesAndSnapshotFallbacks(t *testing.T) {
	inMemory := NewInMemoryTaskRunStore()
	if _, err := inMemory.GetTaskRun(context.Background(), "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected missing in-memory task run to return sql.ErrNoRows, got %v", err)
	}
	record := sampleTaskRunRecord()
	record.RequestSource = ""
	record.RequestTrigger = ""
	record.Snapshot.Source = "selection_toolbar"
	record.Snapshot.Trigger = "selection_click"
	if err := inMemory.SaveTaskRun(context.Background(), record); err != nil {
		t.Fatalf("in-memory SaveTaskRun returned error: %v", err)
	}
	taskRecord, err := inMemory.taskStore.GetTask(context.Background(), record.TaskID)
	if err != nil || taskRecord.RequestSource != "selection_toolbar" || taskRecord.RequestTrigger != "selection_click" {
		t.Fatalf("expected task snapshot projection to fall back to snapshot request metadata, record=%+v err=%v", taskRecord, err)
	}
	second := sampleTaskRunRecord()
	second.TaskID = "task_same_started_b"
	second.RunID = "run_same_started_b"
	second.StartedAt = record.StartedAt
	second.UpdatedAt = record.UpdatedAt
	if err := inMemory.SaveTaskRun(context.Background(), second); err != nil {
		t.Fatalf("in-memory SaveTaskRun second returned error: %v", err)
	}
	legacyRecords, err := inMemory.LoadLegacyTaskRuns(context.Background(), []string{"", "   "})
	if err != nil || len(legacyRecords) != 2 || legacyRecords[0].TaskID != second.TaskID {
		t.Fatalf("expected blank structured ids to be ignored and equal timestamps to sort by task id, records=%+v err=%v", legacyRecords, err)
	}

	path := filepath.Join(t.TempDir(), "task-runs-missing.db")
	sqliteStore, err := NewSQLiteTaskRunStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteTaskRunStore returned error: %v", err)
	}
	defer func() { _ = sqliteStore.Close() }()
	if _, err := sqliteStore.GetTaskRun(context.Background(), "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected missing sqlite task run to return sql.ErrNoRows, got %v", err)
	}
}

func TestSQLiteTaskRunStoreLoadLegacyTaskRunsHandlesErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-runs-errors.db")
	store, err := NewSQLiteTaskRunStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteTaskRunStore returned error: %v", err)
	}
	if _, err := store.db.Exec(`INSERT INTO task_runs (task_id, run_id, session_id, status, started_at, updated_at, record_json) VALUES (?, ?, ?, ?, ?, ?, ?)`, "task_bad_json", "run_bad_json", "sess_bad_json", "completed", "2026-04-22T10:00:00Z", "2026-04-22T10:00:00Z", `{bad-json}`); err != nil {
		t.Fatalf("insert malformed task run row failed: %v", err)
	}
	if _, err := store.LoadLegacyTaskRuns(context.Background(), []string{"", "   "}); err == nil {
		t.Fatal("expected malformed task run row to surface an error")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if _, err := store.LoadLegacyTaskRuns(context.Background(), nil); err == nil {
		t.Fatal("expected closed sqlite task run store to fail legacy load")
	}
}
func TestSQLiteTaskRunStorePersistsExecutionAttempt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-run-attempt.db")
	store, err := NewSQLiteTaskRunStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteTaskRunStore returned error: %v", err)
	}
	defer func() { _ = store.Close() }()

	record := sampleTaskRunRecord()
	record.ExecutionAttempt = 3
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
	if records[0].ExecutionAttempt != 3 {
		t.Fatalf("expected execution attempt 3 to round-trip, got %d", records[0].ExecutionAttempt)
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

func TestSQLiteTaskRunStoreValidatesConstructorAndIdentifiers(t *testing.T) {
	if _, err := NewSQLiteTaskRunStore("   "); err != ErrDatabasePathRequired {
		t.Fatalf("expected ErrDatabasePathRequired, got %v", err)
	}

	path := filepath.Join(t.TempDir(), "task-run-validation.db")
	store, err := NewSQLiteTaskRunStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteTaskRunStore returned error: %v", err)
	}
	defer func() { _ = store.Close() }()

	if _, err := store.AllocateIdentifier(context.Background(), "   "); err != ErrTaskRunIdentifierPrefixRequired {
		t.Fatalf("expected ErrTaskRunIdentifierPrefixRequired, got %v", err)
	}
	if err := store.DeleteTaskRun(context.Background(), "   "); err != ErrTaskRunTaskIDRequired {
		t.Fatalf("expected ErrTaskRunTaskIDRequired, got %v", err)
	}

	var nilStore SQLiteTaskRunStore
	if err := nilStore.Close(); err != nil {
		t.Fatalf("expected nil sqlite task run store close to succeed, got %v", err)
	}
}

func TestSQLiteTaskRunStoreSaveTaskRunRollsBackStructuredTaskStateOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-runs-rollback.db")
	store, err := NewSQLiteTaskRunStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteTaskRunStore returned error: %v", err)
	}
	defer func() { _ = store.Close() }()

	if _, err := store.db.Exec(`DROP TABLE task_steps;`); err != nil {
		t.Fatalf("drop task_steps table: %v", err)
	}
	record := sampleTaskRunRecord()
	err = store.SaveTaskRun(context.Background(), record)
	if err == nil {
		t.Fatal("expected SaveTaskRun to fail when structured step table is missing")
	}
	records, loadErr := store.LoadTaskRuns(context.Background())
	if loadErr != nil {
		t.Fatalf("LoadTaskRuns returned error: %v", loadErr)
	}
	if len(records) != 0 {
		t.Fatalf("expected task_runs write to rollback on structured write failure, got %+v", records)
	}
	taskItems, taskTotal, err := store.taskStore.ListTasks(context.Background(), 10, 0)
	if err == nil && (taskTotal != 0 || len(taskItems) != 0) {
		t.Fatalf("expected structured task rows to rollback too, got total=%d items=%+v", taskTotal, taskItems)
	}
}

func TestTaskStoresSupportUnlimitedPaginationAndDirectLookup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-store-pagination.db")
	taskStore, err := NewSQLiteTaskStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteTaskStore returned error: %v", err)
	}
	defer func() { _ = taskStore.Close() }()
	stepStore, err := NewSQLiteTaskStepStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteTaskStepStore returned error: %v", err)
	}
	defer func() { _ = stepStore.Close() }()

	for index := 0; index < 25; index++ {
		record := sampleTaskRunRecord()
		record.TaskID = fmt.Sprintf("task_%03d", index)
		record.RunID = fmt.Sprintf("run_%03d", index)
		record.Timeline = []TaskStepSnapshot{{
			StepID:        fmt.Sprintf("step_%03d", index),
			TaskID:        record.TaskID,
			Name:          "return_result",
			Status:        "completed",
			OrderIndex:    1,
			InputSummary:  "task input",
			OutputSummary: "task output",
		}}
		record.Title = fmt.Sprintf("task title %03d", index)
		record.StartedAt = time.Date(2026, 4, 10, 9, 0, index, 0, time.UTC)
		record.UpdatedAt = time.Date(2026, 4, 10, 9, 5, index, 0, time.UTC)
		taskRecord, err := taskRecordFromSnapshot(record)
		if err != nil {
			t.Fatalf("taskRecordFromSnapshot returned error: %v", err)
		}
		if err := taskStore.WriteTask(context.Background(), taskRecord); err != nil {
			t.Fatalf("WriteTask returned error: %v", err)
		}
		if err := stepStore.ReplaceTaskSteps(context.Background(), record.TaskID, taskStepRecordsFromSnapshot(record)); err != nil {
			t.Fatalf("ReplaceTaskSteps returned error: %v", err)
		}
	}

	items, total, err := taskStore.ListTasks(context.Background(), 0, 0)
	if err != nil || total != 25 || len(items) != 25 {
		t.Fatalf("expected unlimited ListTasks to return all rows, got total=%d len=%d err=%v", total, len(items), err)
	}
	record, err := taskStore.GetTask(context.Background(), "task_005")
	if err != nil {
		t.Fatalf("GetTask returned error: %v", err)
	}
	if record.TaskID != "task_005" || record.RunID != "run_005" {
		t.Fatalf("unexpected task lookup result: %+v", record)
	}
	stepItems, stepTotal, err := stepStore.ListTaskSteps(context.Background(), "task_005", 0, 0)
	if err != nil || stepTotal != 1 || len(stepItems) != 1 {
		t.Fatalf("expected unlimited ListTaskSteps to return full timeline, got total=%d len=%d err=%v", stepTotal, len(stepItems), err)
	}
	inMemoryTaskStore := newInMemoryTaskStore()
	for index := 0; index < 25; index++ {
		if err := inMemoryTaskStore.WriteTask(context.Background(), TaskRecord{TaskID: fmt.Sprintf("mem_task_%03d", index), StartedAt: time.Date(2026, 4, 10, 9, 0, index, 0, time.UTC).Format(time.RFC3339Nano)}); err != nil {
			t.Fatalf("in-memory WriteTask returned error: %v", err)
		}
	}
	inMemoryItems, inMemoryTotal, err := inMemoryTaskStore.ListTasks(context.Background(), 0, 0)
	if err != nil || inMemoryTotal != 25 || len(inMemoryItems) != 25 {
		t.Fatalf("expected in-memory unlimited ListTasks to return all rows, got total=%d len=%d err=%v", inMemoryTotal, len(inMemoryItems), err)
	}
	inMemoryStepStore := newInMemoryTaskStepStore()
	stepRecords := make([]TaskStepRecord, 0, 25)
	for index := 0; index < 25; index++ {
		stepRecords = append(stepRecords, TaskStepRecord{StepID: fmt.Sprintf("step_%03d", index), TaskID: "mem_task", OrderIndex: index})
	}
	if err := inMemoryStepStore.ReplaceTaskSteps(context.Background(), "mem_task", stepRecords); err != nil {
		t.Fatalf("in-memory ReplaceTaskSteps returned error: %v", err)
	}
	inMemoryStepItems, inMemoryStepTotal, err := inMemoryStepStore.ListTaskSteps(context.Background(), "mem_task", 0, 0)
	if err != nil || inMemoryStepTotal != 25 || len(inMemoryStepItems) != 25 {
		t.Fatalf("expected in-memory unlimited ListTaskSteps to return all rows, got total=%d len=%d err=%v", inMemoryStepTotal, len(inMemoryStepItems), err)
	}
}

func TestSQLiteTaskRunStoreValidationAndMarshalHelpers(t *testing.T) {
	validRecord := sampleTaskRunRecord()
	if err := validateTaskRunRecord(validRecord); err != nil {
		t.Fatalf("expected valid task run record, got %v", err)
	}

	tests := []struct {
		name   string
		record TaskRunRecord
		want   error
	}{
		{name: "missing task id", record: func() TaskRunRecord { r := sampleTaskRunRecord(); r.TaskID = ""; return r }(), want: ErrTaskRunTaskIDRequired},
		{name: "missing session id", record: func() TaskRunRecord { r := sampleTaskRunRecord(); r.SessionID = ""; return r }(), want: ErrTaskRunSessionIDRequired},
		{name: "missing run id", record: func() TaskRunRecord { r := sampleTaskRunRecord(); r.RunID = ""; return r }(), want: ErrTaskRunRunIDRequired},
		{name: "missing status", record: func() TaskRunRecord { r := sampleTaskRunRecord(); r.Status = ""; return r }(), want: ErrTaskRunStatusRequired},
		{name: "missing started at", record: func() TaskRunRecord { r := sampleTaskRunRecord(); r.StartedAt = time.Time{}; return r }(), want: ErrTaskRunStartedAtRequired},
		{name: "missing updated at", record: func() TaskRunRecord { r := sampleTaskRunRecord(); r.UpdatedAt = time.Time{}; return r }(), want: ErrTaskRunUpdatedAtRequired},
	}
	for _, test := range tests {
		if err := validateTaskRunRecord(test.record); err != test.want {
			t.Fatalf("%s: expected %v, got %v", test.name, test.want, err)
		}
	}

	badRecord := sampleTaskRunRecord()
	badRecord.Intent = map[string]any{"unsupported": func() {}}
	if _, err := marshalTaskRunRecord(badRecord); err == nil {
		t.Fatal("expected marshalTaskRunRecord to fail for unsupported payload")
	}
	if _, err := unmarshalTaskRunRecord("{bad json}"); err == nil {
		t.Fatal("expected unmarshalTaskRunRecord to fail for invalid json")
	}
}

func TestSQLiteTaskRunStoreCloneHelpersPreserveIsolation(t *testing.T) {
	record := sampleTaskRunRecord()
	clone := cloneTaskRunRecord(record)
	clone.Intent["name"] = "rewrite"
	clone.Timeline[0].Name = "changed_step"
	clone.Artifacts[0]["artifact_id"] = "art_clone"
	clone.Snapshot.Text = "changed snapshot"
	clone.Notifications[0].Method = "task.changed"
	clone.LatestEvent["type"] = "event.changed"
	clone.SteeringMessages = []string{"changed"}
	if record.Intent["name"] != "summarize" || record.Timeline[0].Name != "return_result" || record.Artifacts[0]["artifact_id"] != "art_001" || record.Snapshot.Text != "sample input" || record.Notifications[0].Method != "task.updated" || record.LatestEvent["type"] != "delivery.ready" || len(record.SteeringMessages) != 0 {
		t.Fatalf("expected cloneTaskRunRecord to isolate mutable nested fields, got original %+v", record)
	}

	originalMap := map[string]any{
		"nested": map[string]any{"value": "a"},
		"slice":  []map[string]any{{"id": "one"}},
		"texts":  []string{"x"},
	}
	clonedMap := cloneMap(originalMap)
	clonedMap["nested"].(map[string]any)["value"] = "b"
	clonedMap["slice"].([]map[string]any)[0]["id"] = "two"
	clonedMap["texts"].([]string)[0] = "y"
	if originalMap["nested"].(map[string]any)["value"] != "a" || originalMap["slice"].([]map[string]any)[0]["id"] != "one" || originalMap["texts"].([]string)[0] != "x" {
		t.Fatalf("expected cloneMap to preserve original data, got %+v", originalMap)
	}

	originalSlice := []map[string]any{{"id": "first"}}
	clonedSlice := cloneMapSlice(originalSlice)
	clonedSlice[0]["id"] = "second"
	if originalSlice[0]["id"] != "first" {
		t.Fatalf("expected cloneMapSlice to preserve original slice, got %+v", originalSlice)
	}
	if cloneMap(nil) != nil || cloneMapSlice(nil) != nil || cloneTaskStepSnapshots(nil) != nil || cloneNotificationSnapshots(nil) != nil {
		t.Fatal("expected clone helpers to preserve nil inputs")
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
		RequestSource:     "floating_ball",
		RequestTrigger:    "hover_text_input",
		ExecutionAttempt:  1,
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
		Citations:      []map[string]any{{"citation_id": "cit_001", "task_id": "task_001", "run_id": "run_001", "source_type": "file", "source_ref": "art_001", "label": "workspace evidence"}},
		Snapshot: taskcontext.TaskContextSnapshot{
			Source:        "floating_ball",
			Trigger:       "hover_text_input",
			InputType:     "text",
			InputMode:     "text",
			Text:          "sample input",
			SelectionText: "selected text",
			Files:         []string{"workspace/input.md"},
			PageTitle:     "Sample Page",
			PageURL:       "https://example.com",
			AppName:       "browser",
		},
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
