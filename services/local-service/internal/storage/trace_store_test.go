package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestInMemoryTraceAndEvalStoresPersistAndList(t *testing.T) {
	traceStore := newInMemoryTraceStore()
	evalStore := newInMemoryEvalStore()
	if err := traceStore.WriteTraceRecord(context.Background(), TraceRecord{
		TraceID:          "trace_001",
		TaskID:           "task_001",
		RunID:            "run_001",
		LoopRound:        2,
		LLMInputSummary:  "input",
		LLMOutputSummary: "output",
		CreatedAt:        "2026-04-17T10:00:00Z",
	}); err != nil {
		t.Fatalf("write trace record failed: %v", err)
	}
	if err := evalStore.WriteEvalSnapshot(context.Background(), EvalSnapshotRecord{
		EvalSnapshotID: "eval_001",
		TraceID:        "trace_001",
		TaskID:         "task_001",
		Status:         "passed",
		MetricsJSON:    `{"latency_ms":321}`,
		CreatedAt:      "2026-04-17T10:00:00Z",
	}); err != nil {
		t.Fatalf("write eval snapshot failed: %v", err)
	}
	traces, total, err := traceStore.ListTraceRecords(context.Background(), "task_001", 10, 0)
	if err != nil || total != 1 || len(traces) != 1 {
		t.Fatalf("expected one trace record, total=%d len=%d err=%v", total, len(traces), err)
	}
	evals, total, err := evalStore.ListEvalSnapshots(context.Background(), "task_001", 10, 0)
	if err != nil || total != 1 || len(evals) != 1 {
		t.Fatalf("expected one eval snapshot, total=%d len=%d err=%v", total, len(evals), err)
	}
	if err := traceStore.DeleteTraceRecord(context.Background(), "trace_001"); err != nil {
		t.Fatalf("delete trace record failed: %v", err)
	}
	traces, total, err = traceStore.ListTraceRecords(context.Background(), "task_001", 10, 0)
	if err != nil || total != 0 || len(traces) != 0 {
		t.Fatalf("expected deleted in-memory trace record to disappear, total=%d len=%d err=%v", total, len(traces), err)
	}
}

func TestSQLiteTraceAndEvalStoresPersistAndList(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "trace-eval.db")
	traceStore, err := NewSQLiteTraceStore(databasePath)
	if err != nil {
		t.Fatalf("new sqlite trace store failed: %v", err)
	}
	defer func() { _ = traceStore.Close() }()
	evalStore, err := NewSQLiteEvalStore(databasePath)
	if err != nil {
		t.Fatalf("new sqlite eval store failed: %v", err)
	}
	defer func() { _ = evalStore.Close() }()
	if err := traceStore.WriteTraceRecord(context.Background(), TraceRecord{
		TraceID:          "trace_sql_001",
		TaskID:           "task_sql_001",
		RunID:            "run_sql_001",
		LoopRound:        3,
		LLMInputSummary:  "input summary",
		LLMOutputSummary: "output summary",
		LatencyMS:        321,
		Cost:             0.012,
		RuleHitsJSON:     `{"doom_loop":"triggered"}`,
		ReviewResult:     "human_review_required",
		CreatedAt:        "2026-04-17T10:00:00Z",
	}); err != nil {
		t.Fatalf("write sqlite trace failed: %v", err)
	}
	if err := evalStore.WriteEvalSnapshot(context.Background(), EvalSnapshotRecord{
		EvalSnapshotID: "eval_sql_001",
		TraceID:        "trace_sql_001",
		TaskID:         "task_sql_001",
		Status:         "human_review_required",
		MetricsJSON:    `{"doom_loop_triggered":true}`,
		CreatedAt:      "2026-04-17T10:00:00Z",
	}); err != nil {
		t.Fatalf("write sqlite eval failed: %v", err)
	}
	traces, total, err := traceStore.ListTraceRecords(context.Background(), "task_sql_001", 10, 0)
	if err != nil || total != 1 || len(traces) != 1 {
		t.Fatalf("expected one sqlite trace record, total=%d len=%d err=%v", total, len(traces), err)
	}
	if traces[0].ReviewResult != "human_review_required" {
		t.Fatalf("expected review result to round-trip, got %+v", traces[0])
	}
	if err := traceStore.DeleteTraceRecord(context.Background(), "trace_sql_001"); err != nil {
		t.Fatalf("delete sqlite trace failed: %v", err)
	}
	traces, total, err = traceStore.ListTraceRecords(context.Background(), "task_sql_001", 10, 0)
	if err != nil || total != 0 || len(traces) != 0 {
		t.Fatalf("expected deleted sqlite trace record to disappear, total=%d len=%d err=%v", total, len(traces), err)
	}
	evals, total, err := evalStore.ListEvalSnapshots(context.Background(), "task_sql_001", 10, 0)
	if err != nil || total != 0 || len(evals) != 0 {
		t.Fatalf("expected linked eval snapshots to be deleted with trace rollback, total=%d len=%d err=%v", total, len(evals), err)
	}
}

func TestSQLiteEvalStoreEnforcesTraceForeignKey(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "trace-eval-fk.db")
	traceStore, err := NewSQLiteTraceStore(databasePath)
	if err != nil {
		t.Fatalf("new sqlite trace store failed: %v", err)
	}
	defer func() { _ = traceStore.Close() }()
	evalStore, err := NewSQLiteEvalStore(databasePath)
	if err != nil {
		t.Fatalf("new sqlite eval store failed: %v", err)
	}
	defer func() { _ = evalStore.Close() }()

	err = evalStore.WriteEvalSnapshot(context.Background(), EvalSnapshotRecord{
		EvalSnapshotID: "eval_orphan_001",
		TraceID:        "trace_missing",
		TaskID:         "task_sql_002",
		Status:         "passed",
		MetricsJSON:    `{"latency_ms":100}`,
		CreatedAt:      "2026-04-17T11:00:00Z",
	})
	if err == nil {
		t.Fatal("expected foreign key error when writing eval snapshot without trace record")
	}

	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open sqlite db failed: %v", err)
	}
	defer db.Close()
	rows, err := db.Query(`PRAGMA foreign_key_list(eval_snapshots);`)
	if err != nil {
		t.Fatalf("query foreign key list failed: %v", err)
	}
	defer rows.Close()

	hasTraceForeignKey := false
	for rows.Next() {
		var id, seq int
		var table, from, to, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			t.Fatalf("scan foreign key row failed: %v", err)
		}
		if table == "trace_records" && from == "trace_id" && to == "trace_id" {
			hasTraceForeignKey = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate foreign key rows failed: %v", err)
	}
	if !hasTraceForeignKey {
		t.Fatal("expected eval_snapshots to keep foreign key back to trace_records")
	}
}

func TestSQLiteEvalStoreMigratesLegacyRowsAndQuarantinesOrphans(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "trace-eval-migration.db")
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open sqlite db failed: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`PRAGMA foreign_keys=OFF;`); err != nil {
		t.Fatalf("disable foreign keys failed: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE trace_records (
			trace_id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			run_id TEXT,
			loop_round INTEGER NOT NULL DEFAULT 0,
			llm_input_summary TEXT NOT NULL,
			llm_output_summary TEXT NOT NULL,
			latency_ms INTEGER NOT NULL DEFAULT 0,
			cost REAL NOT NULL DEFAULT 0,
			rule_hits_json TEXT,
			review_result TEXT,
			created_at TEXT NOT NULL
		);
	`); err != nil {
		t.Fatalf("create legacy trace_records failed: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE eval_snapshots (
			eval_snapshot_id TEXT PRIMARY KEY,
			trace_id TEXT NOT NULL,
			task_id TEXT NOT NULL,
			status TEXT NOT NULL,
			metrics_json TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
	`); err != nil {
		t.Fatalf("create legacy eval_snapshots failed: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO trace_records (trace_id, task_id, run_id, loop_round, llm_input_summary, llm_output_summary, latency_ms, cost, rule_hits_json, review_result, created_at)
		VALUES ('trace_live', 'task_live', 'run_live', 1, 'input', 'output', 10, 0, '[]', 'passed', '2026-04-17T10:00:00Z');
	`); err != nil {
		t.Fatalf("seed trace_records failed: %v", err)
	}
	if _, err := db.Exec(`
		INSERT INTO eval_snapshots (eval_snapshot_id, trace_id, task_id, status, metrics_json, created_at)
		VALUES
			('eval_live', 'trace_live', 'task_live', 'passed', '{"latency_ms":10}', '2026-04-17T10:00:00Z'),
			('eval_orphan', 'trace_missing', 'task_orphan', 'needs_attention', '{"latency_ms":20}', '2026-04-17T10:01:00Z');
	`); err != nil {
		t.Fatalf("seed eval_snapshots failed: %v", err)
	}

	store, err := NewSQLiteEvalStore(databasePath)
	if err != nil {
		t.Fatalf("new sqlite eval store failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	var keptCount int
	if err := db.QueryRow(`SELECT COUNT(1) FROM eval_snapshots`).Scan(&keptCount); err != nil {
		t.Fatalf("count migrated eval snapshots failed: %v", err)
	}
	if keptCount != 1 {
		t.Fatalf("expected only linked eval snapshots to survive migration, got %d", keptCount)
	}
	var orphanCount int
	if err := db.QueryRow(`SELECT COUNT(1) FROM eval_snapshots_orphaned`).Scan(&orphanCount); err != nil {
		t.Fatalf("count quarantined eval snapshots failed: %v", err)
	}
	if orphanCount != 1 {
		t.Fatalf("expected one orphaned eval snapshot to be quarantined, got %d", orphanCount)
	}
	var orphanID, orphanReason string
	if err := db.QueryRow(`SELECT eval_snapshot_id, reason FROM eval_snapshots_orphaned`).Scan(&orphanID, &orphanReason); err != nil {
		t.Fatalf("query quarantined eval snapshot failed: %v", err)
	}
	if orphanID != "eval_orphan" || orphanReason == "" {
		t.Fatalf("expected orphan metadata to be preserved, got id=%q reason=%q", orphanID, orphanReason)
	}
}

func TestTraceAndEvalPagingHelpersHandleOffsetsAndUnlimitedPages(t *testing.T) {
	traceItems := []TraceRecord{{TraceID: "trace_1"}, {TraceID: "trace_2"}, {TraceID: "trace_3"}}
	if got := pageTraceRecords(traceItems, 0, 1); len(got) != 2 || got[0].TraceID != "trace_2" {
		t.Fatalf("expected unlimited trace page from offset, got %+v", got)
	}
	if got := pageTraceRecords(traceItems, 2, 5); len(got) != 0 {
		t.Fatalf("expected empty trace page beyond range, got %+v", got)
	}

	evalItems := []EvalSnapshotRecord{{EvalSnapshotID: "eval_1"}, {EvalSnapshotID: "eval_2"}, {EvalSnapshotID: "eval_3"}}
	if got := pageEvalSnapshots(evalItems, 2, 0); len(got) != 2 || got[1].EvalSnapshotID != "eval_2" {
		t.Fatalf("expected bounded eval page, got %+v", got)
	}
	if got := pageEvalSnapshots(evalItems, 0, 2); len(got) != 1 || got[0].EvalSnapshotID != "eval_3" {
		t.Fatalf("expected unlimited eval page from offset, got %+v", got)
	}
}
