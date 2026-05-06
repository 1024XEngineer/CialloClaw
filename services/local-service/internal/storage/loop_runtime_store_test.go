package storage

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestInMemoryLoopRuntimeStoreSupportsStructuredQueries(t *testing.T) {
	store := newInMemoryLoopRuntimeStore()
	if err := store.SaveRun(context.Background(), RunRecord{RunID: "run_mem_001", TaskID: "task_mem_001", SessionID: "sess_mem_001", SourceType: "hover_input", Status: "running", IntentName: "summarize", StartedAt: "2026-04-21T10:00:00Z", UpdatedAt: "2026-04-21T10:00:01Z"}); err != nil {
		t.Fatalf("SaveRun returned error: %v", err)
	}
	if err := store.SaveSteps(context.Background(), []StepRecord{{StepID: "step_mem_001", RunID: "run_mem_001", TaskID: "task_mem_001", Name: "plan", Status: "completed", OrderIndex: 1}}); err != nil {
		t.Fatalf("SaveSteps returned error: %v", err)
	}
	if err := store.SaveEvents(context.Background(), []EventRecord{{EventID: "evt_mem_001", RunID: "run_mem_001", TaskID: "task_mem_001", Type: "loop.completed", CreatedAt: "2026-04-21T10:00:03Z"}}); err != nil {
		t.Fatalf("SaveEvents returned error: %v", err)
	}
	if err := store.SaveDeliveryResult(context.Background(), DeliveryResultRecord{DeliveryResultID: "delivery_mem_001", TaskID: "task_mem_001", RunID: "run_mem_001", Type: "bubble", Title: "result", CreatedAt: "2026-04-21T10:00:05Z"}); err != nil {
		t.Fatalf("SaveDeliveryResult returned error: %v", err)
	}
	if err := store.SaveDeliveryResult(context.Background(), DeliveryResultRecord{DeliveryResultID: "delivery_mem_002", TaskID: "task_mem_001", RunID: "run_mem_002", Type: "bubble", Title: "result-2", CreatedAt: "2026-04-21T10:00:06Z"}); err != nil {
		t.Fatalf("SaveDeliveryResult second run returned error: %v", err)
	}
	if err := store.ReplaceTaskCitations(context.Background(), "task_mem_001", []CitationRecord{{CitationID: "cit_mem_002", TaskID: "task_mem_001", RunID: "run_mem_002", OrderIndex: 2}, {CitationID: "cit_mem_001", TaskID: "task_mem_001", RunID: "run_mem_001", OrderIndex: 1}}); err != nil {
		t.Fatalf("ReplaceTaskCitations returned error: %v", err)
	}

	runRecord, err := store.GetRun(context.Background(), "run_mem_001")
	if err != nil || runRecord.TaskID != "task_mem_001" || runRecord.SourceType != "hover_input" {
		t.Fatalf("GetRun returned record=%+v err=%v", runRecord, err)
	}
	deliveryResults, total, err := store.ListDeliveryResults(context.Background(), "task_mem_001", "", 10, 0)
	if err != nil || total != 2 || len(deliveryResults) != 2 {
		t.Fatalf("ListDeliveryResults returned total=%d items=%+v err=%v", total, deliveryResults, err)
	}
	filteredDelivery, filteredTotal, err := store.ListDeliveryResults(context.Background(), "task_mem_001", "run_mem_001", 10, 0)
	if err != nil || filteredTotal != 1 || len(filteredDelivery) != 1 || filteredDelivery[0].DeliveryResultID != "delivery_mem_001" {
		t.Fatalf("expected run-scoped in-memory delivery results, total=%d items=%+v err=%v", filteredTotal, filteredDelivery, err)
	}
	latestDelivery, ok, err := store.GetLatestDeliveryResult(context.Background(), "task_mem_001", "")
	if err != nil || !ok || latestDelivery.DeliveryResultID != "delivery_mem_002" || latestDelivery.RunID != "run_mem_002" {
		t.Fatalf("GetLatestDeliveryResult returned record=%+v ok=%v err=%v", latestDelivery, ok, err)
	}
	latestDelivery, ok, err = store.GetLatestDeliveryResult(context.Background(), "task_mem_001", "run_mem_002")
	if err != nil || !ok || latestDelivery.DeliveryResultID != "delivery_mem_002" {
		t.Fatalf("expected run-scoped latest in-memory delivery result, record=%+v ok=%v err=%v", latestDelivery, ok, err)
	}
	citations, err := store.ListTaskCitations(context.Background(), "task_mem_001", "")
	if err != nil || len(citations) != 2 || citations[0].CitationID != "cit_mem_001" {
		t.Fatalf("ListTaskCitations returned %+v err=%v", citations, err)
	}
	citations, err = store.ListTaskCitations(context.Background(), "task_mem_001", "run_mem_001")
	if err != nil || len(citations) != 1 || citations[0].CitationID != "cit_mem_001" {
		t.Fatalf("expected run-scoped in-memory citations, citations=%+v err=%v", citations, err)
	}
	events, total, err := store.ListEvents(context.Background(), "task_mem_001", "run_mem_001", "loop.completed", "", "", 10, 0)
	if err != nil || total != 1 || len(events) != 1 {
		t.Fatalf("ListEvents returned total=%d items=%+v err=%v", total, events, err)
	}
	if _, err := store.GetRun(context.Background(), "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected missing run lookup to return sql.ErrNoRows, got %v", err)
	}
}

func TestSQLiteLoopRuntimeStoreStructuredQueries(t *testing.T) {
	store, err := NewSQLiteLoopRuntimeStore(filepath.Join(t.TempDir(), "loop-runtime-queries.db"))
	if err != nil {
		t.Fatalf("NewSQLiteLoopRuntimeStore returned error: %v", err)
	}
	defer func() { _ = store.Close() }()
	if err := store.SaveRun(context.Background(), RunRecord{RunID: "run_sql_001", TaskID: "task_sql_001", SessionID: "sess_sql_001", SourceType: "hover_input", Status: "completed", IntentName: "summarize", StartedAt: "2026-04-21T10:00:00Z", UpdatedAt: "2026-04-21T10:00:01Z", FinishedAt: "2026-04-21T10:00:02Z", StopReason: "completed"}); err != nil {
		t.Fatalf("SaveRun returned error: %v", err)
	}
	if err := store.SaveDeliveryResult(context.Background(), DeliveryResultRecord{DeliveryResultID: "delivery_sql_001", TaskID: "task_sql_001", RunID: "run_sql_001", Type: "workspace_document", Title: "result", PayloadJSON: `{"task_id":"task_sql_001"}`, PreviewText: "preview", CreatedAt: "2026-04-21T10:00:03Z"}); err != nil {
		t.Fatalf("SaveDeliveryResult returned error: %v", err)
	}
	if err := store.SaveDeliveryResult(context.Background(), DeliveryResultRecord{DeliveryResultID: "delivery_sql_002", TaskID: "task_sql_001", RunID: "run_sql_002", Type: "workspace_document", Title: "result-2", PayloadJSON: `{"task_id":"task_sql_001"}`, PreviewText: "preview-2", CreatedAt: "2026-04-21T10:00:04Z"}); err != nil {
		t.Fatalf("SaveDeliveryResult second run returned error: %v", err)
	}
	if err := store.ReplaceTaskCitations(context.Background(), "task_sql_001", []CitationRecord{{CitationID: "cit_sql_001", TaskID: "task_sql_001", RunID: "run_sql_001", OrderIndex: 0}, {CitationID: "cit_sql_002", TaskID: "task_sql_001", RunID: "run_sql_002", OrderIndex: 1}}); err != nil {
		t.Fatalf("ReplaceTaskCitations returned error: %v", err)
	}
	if err := store.SaveEvents(context.Background(), []EventRecord{{EventID: "evt_sql_001", RunID: "run_sql_001", TaskID: "task_sql_001", Type: "loop.completed", CreatedAt: "2026-04-21T10:00:04Z"}, {EventID: "evt_sql_002", RunID: "run_sql_001", TaskID: "task_sql_001", Type: "loop.started", CreatedAt: "2026-04-21T10:00:02Z"}}); err != nil {
		t.Fatalf("SaveEvents returned error: %v", err)
	}

	runRecord, err := store.GetRun(context.Background(), "run_sql_001")
	if err != nil || runRecord.StopReason != "completed" || runRecord.SourceType != "hover_input" {
		t.Fatalf("GetRun returned record=%+v err=%v", runRecord, err)
	}
	deliveryResults, total, err := store.ListDeliveryResults(context.Background(), "task_sql_001", "", 10, 0)
	if err != nil || total != 2 || len(deliveryResults) != 2 || deliveryResults[0].PreviewText != "preview-2" {
		t.Fatalf("ListDeliveryResults returned total=%d items=%+v err=%v", total, deliveryResults, err)
	}
	if filteredDelivery, total, err := store.ListDeliveryResults(context.Background(), "task_sql_001", "run_sql_001", 10, 0); err != nil || total != 1 || len(filteredDelivery) != 1 || filteredDelivery[0].DeliveryResultID != "delivery_sql_001" {
		t.Fatalf("expected run-scoped sqlite delivery results, total=%d items=%+v err=%v", total, filteredDelivery, err)
	}
	if emptyPage, total, err := store.ListDeliveryResults(context.Background(), "task_sql_001", "run_sql_001", 1, 9); err != nil || total != 1 || len(emptyPage) != 0 {
		t.Fatalf("expected empty paged delivery result slice, total=%d items=%+v err=%v", total, emptyPage, err)
	}
	latestDelivery, ok, err := store.GetLatestDeliveryResult(context.Background(), "task_sql_001", "")
	if err != nil || !ok || latestDelivery.DeliveryResultID != "delivery_sql_002" || latestDelivery.RunID != "run_sql_002" {
		t.Fatalf("GetLatestDeliveryResult returned record=%+v ok=%v err=%v", latestDelivery, ok, err)
	}
	latestDelivery, ok, err = store.GetLatestDeliveryResult(context.Background(), "task_sql_001", "run_sql_001")
	if err != nil || !ok || latestDelivery.DeliveryResultID != "delivery_sql_001" {
		t.Fatalf("expected run-scoped sqlite latest delivery result, record=%+v ok=%v err=%v", latestDelivery, ok, err)
	}
	if latestDelivery, ok, err := store.GetLatestDeliveryResult(context.Background(), "missing_task", ""); err != nil || ok || latestDelivery.DeliveryResultID != "" {
		t.Fatalf("expected missing task latest delivery query to return no record, record=%+v ok=%v err=%v", latestDelivery, ok, err)
	}
	citations, err := store.ListTaskCitations(context.Background(), "task_sql_001", "")
	if err != nil || len(citations) != 2 || citations[0].CitationID != "cit_sql_001" {
		t.Fatalf("ListTaskCitations returned %+v err=%v", citations, err)
	}
	if citations, err = store.ListTaskCitations(context.Background(), "task_sql_001", "run_sql_002"); err != nil || len(citations) != 1 || citations[0].CitationID != "cit_sql_002" {
		t.Fatalf("expected run-scoped sqlite citations, citations=%+v err=%v", citations, err)
	}
	if err := store.ReplaceTaskCitations(context.Background(), "task_sql_001", []CitationRecord{{CitationID: "cit_sql_003", TaskID: "task_sql_001", RunID: "run_sql_002", OrderIndex: 1}, {CitationID: "cit_sql_004", TaskID: "task_sql_001", RunID: "run_sql_001", OrderIndex: 0}}); err != nil {
		t.Fatalf("ReplaceTaskCitations second pass returned error: %v", err)
	}
	citations, err = store.ListTaskCitations(context.Background(), "task_sql_001", "")
	if err != nil || len(citations) != 2 || citations[0].CitationID != "cit_sql_004" || citations[1].CitationID != "cit_sql_003" {
		t.Fatalf("expected replaced citations to be sorted and old rows removed, got %+v err=%v", citations, err)
	}
	events, total, err := store.ListEvents(context.Background(), "task_sql_001", "run_sql_001", "loop.completed", "", "", 10, 0)
	if err != nil || total != 1 || len(events) != 1 || events[0].EventID != "evt_sql_001" {
		t.Fatalf("ListEvents returned total=%d items=%+v err=%v", total, events, err)
	}
	if emptyEvents, total, err := store.ListEvents(context.Background(), "task_sql_001", "run_sql_001", "loop.completed", "2026-04-21T10:00:05Z", "", 10, 0); err != nil || total != 0 || len(emptyEvents) != 0 {
		t.Fatalf("expected filtered ListEvents to return empty slice, total=%d items=%+v err=%v", total, emptyEvents, err)
	}
	if emptyCitations, err := store.ListTaskCitations(context.Background(), "missing_task", ""); err != nil || len(emptyCitations) != 0 {
		t.Fatalf("expected missing task citations to return empty slice, citations=%+v err=%v", emptyCitations, err)
	}
	if err := store.initialize(context.Background()); err != nil {
		t.Fatalf("expected repeated initialize to tolerate duplicate columns, got %v", err)
	}
	if !isSQLiteDuplicateColumnError(errors.New("duplicate column name: attempt_index")) || isSQLiteDuplicateColumnError(nil) || isSQLiteDuplicateColumnError(errors.New("other failure")) {
		t.Fatal("unexpected duplicate-column detection result")
	}
	if nullableRuntimeString("") != nil || nullableRuntimeString("  ") != nil || nullableRuntimeString("value") != "value" {
		t.Fatal("expected nullableRuntimeString to drop blank values and preserve non-blank strings")
	}
}

func TestLoopRuntimeStoresCoverAdditionalPagingAndErrorBranches(t *testing.T) {
	if _, err := NewSQLiteLoopRuntimeStore(""); err == nil {
		t.Fatal("expected sqlite loop runtime constructor to reject empty path")
	}

	inMemory := newInMemoryLoopRuntimeStore()
	if err := inMemory.SaveDeliveryResult(context.Background(), DeliveryResultRecord{DeliveryResultID: "delivery_mem_a", TaskID: "task_mem_a", Type: "bubble", Title: "A", CreatedAt: "2026-04-21T10:00:05Z"}); err != nil {
		t.Fatalf("SaveDeliveryResult returned error: %v", err)
	}
	if err := inMemory.SaveDeliveryResult(context.Background(), DeliveryResultRecord{DeliveryResultID: "delivery_mem_b", TaskID: "task_mem_b", Type: "bubble", Title: "B", CreatedAt: "2026-04-21T10:00:06Z"}); err != nil {
		t.Fatalf("SaveDeliveryResult returned error: %v", err)
	}
	results, total, err := inMemory.ListDeliveryResults(context.Background(), "", "", 0, 1)
	if err != nil || total != 2 || len(results) != 1 || results[0].DeliveryResultID != "delivery_mem_a" {
		t.Fatalf("expected unlimited in-memory delivery paging to return later page, total=%d items=%+v err=%v", total, results, err)
	}
	latest, ok, err := inMemory.GetLatestDeliveryResult(context.Background(), "missing_task", "")
	if err != nil || ok || latest.DeliveryResultID != "" {
		t.Fatalf("expected missing in-memory latest delivery query to return no record, record=%+v ok=%v err=%v", latest, ok, err)
	}
	if err := inMemory.SaveEvents(context.Background(), []EventRecord{
		{EventID: "evt_mem_a", RunID: "run_mem_a", TaskID: "task_mem_a", Type: "loop.started", CreatedAt: "2026-04-21T10:00:01Z"},
		{EventID: "evt_mem_b", RunID: "run_mem_a", TaskID: "task_mem_a", Type: "loop.completed", CreatedAt: "2026-04-21T10:00:03Z"},
		{EventID: "evt_mem_c", RunID: "run_mem_b", TaskID: "task_mem_b", Type: "loop.completed", CreatedAt: "2026-04-21T10:00:02Z"},
	}); err != nil {
		t.Fatalf("SaveEvents returned error: %v", err)
	}
	events, total, err := inMemory.ListEvents(context.Background(), "", "", "", "2026-04-21T10:00:02Z", "2026-04-21T10:00:03Z", 0, 0)
	if err != nil || total != 2 || len(events) != 2 || events[0].EventID != "evt_mem_b" {
		t.Fatalf("expected in-memory ListEvents to honor time range and sort order, total=%d items=%+v err=%v", total, events, err)
	}
	if emptyEvents, total, err := inMemory.ListEvents(context.Background(), "", "", "", "", "", 1, 9); err != nil || total != 3 || len(emptyEvents) != 0 {
		t.Fatalf("expected in-memory ListEvents overflow page to be empty, total=%d items=%+v err=%v", total, emptyEvents, err)
	}
	if err := inMemory.ReplaceTaskCitations(context.Background(), "task_mem_a", nil); err != nil {
		t.Fatalf("ReplaceTaskCitations returned error: %v", err)
	}
	if citations, err := inMemory.ListTaskCitations(context.Background(), "task_mem_a", ""); err != nil || len(citations) != 0 {
		t.Fatalf("expected empty citation replacement to clear in-memory citations, citations=%+v err=%v", citations, err)
	}

	sqliteStore, err := NewSQLiteLoopRuntimeStore(filepath.Join(t.TempDir(), "loop-runtime-errors.db"))
	if err != nil {
		t.Fatalf("NewSQLiteLoopRuntimeStore returned error: %v", err)
	}
	if err := sqliteStore.SaveRun(context.Background(), RunRecord{RunID: "run_sql_extra", TaskID: "task_sql_extra", SessionID: "sess_sql_extra", SourceType: "hover_input", Status: "completed", IntentName: "summarize", StartedAt: "2026-04-21T10:00:00Z", UpdatedAt: "2026-04-21T10:00:01Z"}); err != nil {
		t.Fatalf("SaveRun returned error: %v", err)
	}
	if err := sqliteStore.SaveEvents(context.Background(), []EventRecord{{EventID: "evt_sql_extra", RunID: "run_sql_extra", TaskID: "task_sql_extra", Type: "loop.started", CreatedAt: "2026-04-21T10:00:01Z"}}); err != nil {
		t.Fatalf("SaveEvents returned error: %v", err)
	}
	if err := sqliteStore.SaveDeliveryResult(context.Background(), DeliveryResultRecord{DeliveryResultID: "delivery_sql_extra", TaskID: "task_sql_extra", Type: "bubble", Title: "extra", CreatedAt: "2026-04-21T10:00:02Z"}); err != nil {
		t.Fatalf("SaveDeliveryResult returned error: %v", err)
	}
	if _, err := sqliteStore.GetRun(context.Background(), "missing_run"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected missing sqlite run to return sql.ErrNoRows, got %v", err)
	}
	if results, total, err := sqliteStore.ListDeliveryResults(context.Background(), "task_sql_extra", "", 0, 0); err != nil || total != 1 || len(results) != 1 {
		t.Fatalf("expected sqlite delivery query without limit to return all rows, total=%d items=%+v err=%v", total, results, err)
	}
	if events, total, err := sqliteStore.ListEvents(context.Background(), "", "", "", "", "", 0, 0); err != nil || total != 1 || len(events) != 1 {
		t.Fatalf("expected sqlite ListEvents without filters to return all rows, total=%d items=%+v err=%v", total, events, err)
	}
	if err := sqliteStore.ReplaceTaskCitations(context.Background(), "task_sql_extra", []CitationRecord{{CitationID: "cit_sql_extra", TaskID: "task_sql_extra", OrderIndex: 0}}); err != nil {
		t.Fatalf("ReplaceTaskCitations returned error: %v", err)
	}
	if err := sqliteStore.ReplaceTaskCitations(context.Background(), "task_sql_extra", nil); err != nil {
		t.Fatalf("expected empty sqlite citation replacement to succeed, got %v", err)
	}
	if citations, err := sqliteStore.ListTaskCitations(context.Background(), "task_sql_extra", ""); err != nil || len(citations) != 0 {
		t.Fatalf("expected empty sqlite citation replacement to clear rows, citations=%+v err=%v", citations, err)
	}
	if err := sqliteStore.initialize(context.Background()); err != nil {
		t.Fatalf("expected repeated sqlite loop runtime initialize to succeed, got %v", err)
	}
	if err := sqliteStore.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if err := sqliteStore.SaveRun(context.Background(), RunRecord{RunID: "run_closed", TaskID: "task_closed", SessionID: "sess_closed", SourceType: "hover_input", Status: "failed", IntentName: "summarize", StartedAt: "2026-04-21T10:10:00Z", UpdatedAt: "2026-04-21T10:10:01Z"}); err == nil {
		t.Fatal("expected SaveRun on closed sqlite loop runtime store to fail")
	}
	if err := sqliteStore.SaveSteps(context.Background(), []StepRecord{{StepID: "step_closed", RunID: "run_closed", TaskID: "task_closed", Name: "plan", Status: "failed", OrderIndex: 1}}); err == nil {
		t.Fatal("expected SaveSteps on closed sqlite loop runtime store to fail")
	}
	if err := sqliteStore.SaveEvents(context.Background(), []EventRecord{{EventID: "evt_closed", RunID: "run_closed", TaskID: "task_closed", Type: "loop.failed", CreatedAt: "2026-04-21T10:10:02Z"}}); err == nil {
		t.Fatal("expected SaveEvents on closed sqlite loop runtime store to fail")
	}
	if err := sqliteStore.SaveDeliveryResult(context.Background(), DeliveryResultRecord{DeliveryResultID: "delivery_closed", TaskID: "task_closed", Type: "bubble", Title: "closed", CreatedAt: "2026-04-21T10:10:03Z"}); err == nil {
		t.Fatal("expected SaveDeliveryResult on closed sqlite loop runtime store to fail")
	}
	if _, err := sqliteStore.GetRun(context.Background(), "run_sql_extra"); err == nil {
		t.Fatal("expected GetRun on closed sqlite loop runtime store to fail")
	}
	if _, _, err := sqliteStore.ListDeliveryResults(context.Background(), "task_sql_extra", "", 10, 0); err == nil {
		t.Fatal("expected ListDeliveryResults on closed sqlite loop runtime store to fail")
	}
	if err := sqliteStore.ReplaceTaskCitations(context.Background(), "task_sql_extra", nil); err == nil {
		t.Fatal("expected ReplaceTaskCitations on closed sqlite loop runtime store to fail")
	}
	if _, _, err := sqliteStore.GetLatestDeliveryResult(context.Background(), "task_sql_extra", ""); err == nil {
		t.Fatal("expected GetLatestDeliveryResult on closed sqlite loop runtime store to fail")
	}
	if _, err := sqliteStore.ListTaskCitations(context.Background(), "task_sql_extra", ""); err == nil {
		t.Fatal("expected ListTaskCitations on closed sqlite loop runtime store to fail")
	}
	if _, _, err := sqliteStore.ListEvents(context.Background(), "", "", "", "", "", 10, 0); err == nil {
		t.Fatal("expected ListEvents on closed sqlite loop runtime store to fail")
	}
	if err := sqliteStore.initialize(context.Background()); err == nil {
		t.Fatal("expected initialize on closed sqlite loop runtime store to fail")
	}

	var nilStore SQLiteLoopRuntimeStore
	if err := nilStore.Close(); err != nil {
		t.Fatalf("expected nil sqlite loop runtime close to succeed, got %v", err)
	}
}
