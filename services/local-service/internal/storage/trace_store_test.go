package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

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
		AssetRefsJSON:    `[{"asset_kind":"skill_manifest","asset_id":"skill_001"}]`,
		CreatedAt:        "2026-04-17T10:00:00Z",
	}); err != nil {
		t.Fatalf("write trace record failed: %v", err)
	}
	if err := evalStore.WriteEvalSnapshot(context.Background(), EvalSnapshotRecord{
		EvalSnapshotID: "eval_001",
		TraceID:        "trace_001",
		TaskID:         "task_001",
		Status:         "passed",
		AssetRefsJSON:  `[{"asset_kind":"skill_manifest","asset_id":"skill_001"}]`,
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
		AssetRefsJSON:    `[{"asset_kind":"prompt_template_version","asset_id":"prompt_001"}]`,
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
		AssetRefsJSON:  `[{"asset_kind":"prompt_template_version","asset_id":"prompt_001"}]`,
		MetricsJSON:    `{"doom_loop_triggered":true}`,
		CreatedAt:      "2026-04-17T10:00:00Z",
	}); err != nil {
		t.Fatalf("write sqlite eval failed: %v", err)
	}
	traces, total, err := traceStore.ListTraceRecords(context.Background(), "task_sql_001", 10, 0)
	if err != nil || total != 1 || len(traces) != 1 {
		t.Fatalf("expected one sqlite trace record, total=%d len=%d err=%v", total, len(traces), err)
	}
	allTraces, total, err := traceStore.ListTraceRecords(context.Background(), "", 10, 0)
	if err != nil || total != 1 || len(allTraces) != 1 {
		t.Fatalf("expected sqlite trace list without task filter to return records, total=%d items=%+v err=%v", total, allTraces, err)
	}
	if traces[0].ReviewResult != "human_review_required" {
		t.Fatalf("expected review result to round-trip, got %+v", traces[0])
	}
	if traces[0].AssetRefsJSON == "" {
		t.Fatalf("expected asset refs to round-trip, got %+v", traces[0])
	}
	evals, total, err := evalStore.ListEvalSnapshots(context.Background(), "", 10, 0)
	if err != nil || total != 1 || len(evals) != 1 {
		t.Fatalf("expected sqlite eval list without task filter to return records, total=%d items=%+v err=%v", total, evals, err)
	}
	if err := traceStore.DeleteTraceRecord(context.Background(), "trace_sql_001"); err != nil {
		t.Fatalf("delete sqlite trace failed: %v", err)
	}
	if err := traceStore.DeleteTraceRecord(context.Background(), "trace_sql_missing"); err != nil {
		t.Fatalf("delete sqlite missing trace failed: %v", err)
	}
	traces, total, err = traceStore.ListTraceRecords(context.Background(), "task_sql_001", 10, 0)
	if err != nil || total != 0 || len(traces) != 0 {
		t.Fatalf("expected deleted sqlite trace record to disappear, total=%d len=%d err=%v", total, len(traces), err)
	}
	evals, total, err = evalStore.ListEvalSnapshots(context.Background(), "task_sql_001", 10, 0)
	if err != nil || total != 0 || len(evals) != 0 {
		t.Fatalf("expected linked eval snapshots to be deleted with trace rollback, total=%d len=%d err=%v", total, len(evals), err)
	}
	if traces, total, err := traceStore.ListTraceRecords(context.Background(), "", 1, 5); err != nil || total != 0 || len(traces) != 0 {
		t.Fatalf("expected empty overflow trace page, total=%d traces=%+v err=%v", total, traces, err)
	}
	if evals, total, err := evalStore.ListEvalSnapshots(context.Background(), "", 1, 5); err != nil || total != 0 || len(evals) != 0 {
		t.Fatalf("expected empty overflow eval page, total=%d evals=%+v err=%v", total, evals, err)
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

func TestServiceInspectorTitleGenerationUsageFallsBackToFilteredInMemoryScan(t *testing.T) {
	traceStore := newInMemoryTraceStore()
	evalStore := newInMemoryEvalStore()
	service := &Service{
		traceStore: traceStore,
		evalStore:  evalStore,
	}

	records := []TraceRecord{
		{
			TraceID:          "trace_manual_today",
			TaskID:           "insp_001",
			LLMInputSummary:  inspectorTitleGenerationIntent,
			LLMOutputSummary: "generated note title",
			Cost:             0.12,
			CreatedAt:        "2026-05-11T08:00:00Z",
		},
		{
			TraceID:          "trace_manual_other_intent",
			TaskID:           "insp_002",
			LLMInputSummary:  "task_inspector.other",
			LLMOutputSummary: "ignored",
			Cost:             9.99,
			CreatedAt:        "2026-05-11T09:00:00Z",
		},
		{
			TraceID:          "trace_other_owner",
			TaskID:           "task_001",
			LLMInputSummary:  inspectorTitleGenerationIntent,
			LLMOutputSummary: "ignored",
			Cost:             8.88,
			CreatedAt:        "2026-05-11T10:00:00Z",
		},
		{
			TraceID:          "trace_manual_yesterday",
			TaskID:           "insp_003",
			LLMInputSummary:  inspectorTitleGenerationIntent,
			LLMOutputSummary: "ignored",
			Cost:             7.77,
			CreatedAt:        "2026-05-10T23:59:00Z",
		},
	}
	for _, record := range records {
		if err := traceStore.WriteTraceRecord(context.Background(), record); err != nil {
			t.Fatalf("write trace record failed: %v", err)
		}
	}

	evals := []EvalSnapshotRecord{
		{
			EvalSnapshotID: "eval_manual_today",
			TraceID:        "trace_manual_today",
			TaskID:         "insp_001",
			Status:         "passed",
			MetricsJSON:    `{"total_tokens":22}`,
			CreatedAt:      "2026-05-11T08:00:00Z",
		},
		{
			EvalSnapshotID: "eval_manual_other_intent",
			TraceID:        "trace_manual_other_intent",
			TaskID:         "insp_002",
			Status:         "passed",
			MetricsJSON:    `{"total_tokens":40}`,
			CreatedAt:      "2026-05-11T09:00:00Z",
		},
		{
			EvalSnapshotID: "eval_other_owner",
			TraceID:        "trace_other_owner",
			TaskID:         "task_001",
			Status:         "passed",
			MetricsJSON:    `{"total_tokens":50}`,
			CreatedAt:      "2026-05-11T10:00:00Z",
		},
	}
	for _, record := range evals {
		if err := evalStore.WriteEvalSnapshot(context.Background(), record); err != nil {
			t.Fatalf("write eval snapshot failed: %v", err)
		}
	}

	summary, err := service.InspectorTitleGenerationUsage(context.Background(), time.Date(2026, 5, 11, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("InspectorTitleGenerationUsage returned error: %v", err)
	}
	if summary.TotalTokens != 22 || summary.TotalCost != 0.12 {
		t.Fatalf("expected only today's inspector title usage to be counted, got %+v", summary)
	}
}

func TestServiceInspectorTitleGenerationUsageUsesSQLiteAggregate(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "inspector-title-usage.db")
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

	service := &Service{
		traceStore: traceStore,
		evalStore:  evalStore,
	}

	records := []TraceRecord{
		{
			TraceID:          "trace_sql_manual_today",
			TaskID:           "insp_100",
			LLMInputSummary:  inspectorTitleGenerationIntent,
			LLMOutputSummary: "generated note title",
			Cost:             0.34,
			CreatedAt:        "2026-05-11T11:00:00Z",
		},
		{
			TraceID:          "trace_sql_manual_today_no_eval",
			TaskID:           "insp_101",
			LLMInputSummary:  inspectorTitleGenerationIntent,
			LLMOutputSummary: "fallback note title",
			Cost:             0.11,
			CreatedAt:        "2026-05-11T12:00:00Z",
		},
		{
			TraceID:          "trace_sql_other_task",
			TaskID:           "task_100",
			LLMInputSummary:  inspectorTitleGenerationIntent,
			LLMOutputSummary: "ignored",
			Cost:             4.56,
			CreatedAt:        "2026-05-11T13:00:00Z",
		},
		{
			TraceID:          "trace_sql_yesterday",
			TaskID:           "insp_102",
			LLMInputSummary:  inspectorTitleGenerationIntent,
			LLMOutputSummary: "ignored",
			Cost:             5.67,
			CreatedAt:        "2026-05-10T18:00:00Z",
		},
	}
	for _, record := range records {
		if err := traceStore.WriteTraceRecord(context.Background(), record); err != nil {
			t.Fatalf("write sqlite trace failed: %v", err)
		}
	}

	for _, record := range []EvalSnapshotRecord{
		{
			EvalSnapshotID: "eval_sql_manual_today",
			TraceID:        "trace_sql_manual_today",
			TaskID:         "insp_100",
			Status:         "passed",
			MetricsJSON:    `{"total_tokens":31}`,
			CreatedAt:      "2026-05-11T11:00:00Z",
		},
		{
			EvalSnapshotID: "eval_sql_other_task",
			TraceID:        "trace_sql_other_task",
			TaskID:         "task_100",
			Status:         "passed",
			MetricsJSON:    `{"total_tokens":88}`,
			CreatedAt:      "2026-05-11T13:00:00Z",
		},
	} {
		if err := evalStore.WriteEvalSnapshot(context.Background(), record); err != nil {
			t.Fatalf("write sqlite eval failed: %v", err)
		}
	}

	summary, err := service.InspectorTitleGenerationUsage(context.Background(), time.Date(2026, 5, 11, 16, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("InspectorTitleGenerationUsage returned error: %v", err)
	}
	if summary.TotalTokens != 31 || summary.TotalCost != 0.45 {
		t.Fatalf("expected sqlite aggregate to count only today's inspector title usage, got %+v", summary)
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

func TestSQLiteEvalStoreMigratesLegacyRowsWithoutOrphans(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "trace-eval-migration-clean.db")
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
		VALUES ('eval_live', 'trace_live', 'task_live', 'passed', '{"latency_ms":10}', '2026-04-17T10:00:00Z');
	`); err != nil {
		t.Fatalf("seed eval_snapshots failed: %v", err)
	}

	store, err := NewSQLiteEvalStore(databasePath)
	if err != nil {
		t.Fatalf("new sqlite eval store failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	evals, total, err := store.ListEvalSnapshots(context.Background(), "task_live", 10, 0)
	if err != nil || total != 1 || len(evals) != 1 {
		t.Fatalf("expected migrated eval snapshot to remain available, total=%d items=%+v err=%v", total, evals, err)
	}
	var orphanCount int
	if err := db.QueryRow(`SELECT COUNT(1) FROM eval_snapshots_orphaned`).Scan(&orphanCount); err != nil {
		t.Fatalf("count orphaned eval snapshots failed: %v", err)
	}
	if orphanCount != 0 {
		t.Fatalf("expected no orphaned eval snapshots for clean migration, got %d", orphanCount)
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
	if got := pageEvalSnapshots(evalItems, 2, 5); len(got) != 0 {
		t.Fatalf("expected empty eval page beyond range, got %+v", got)
	}
}

func TestSQLiteTraceAndEvalStoreConstructorsAndHelpers(t *testing.T) {
	if _, err := NewSQLiteTraceStore(""); err == nil {
		t.Fatal("expected sqlite trace constructor to reject empty path")
	}
	if _, err := NewSQLiteEvalStore(""); err == nil {
		t.Fatal("expected sqlite eval constructor to reject empty path")
	}
	var nilTraceStore SQLiteTraceStore
	if err := nilTraceStore.Close(); err != nil {
		t.Fatalf("expected nil sqlite trace close to succeed, got %v", err)
	}
	var nilEvalStore SQLiteEvalStore
	if err := nilEvalStore.Close(); err != nil {
		t.Fatalf("expected nil sqlite eval close to succeed, got %v", err)
	}

	databasePath := filepath.Join(t.TempDir(), "trace-helper.db")
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open sqlite db failed: %v", err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE trace_records (trace_id TEXT PRIMARY KEY, task_id TEXT NOT NULL, run_id TEXT, loop_round INTEGER NOT NULL DEFAULT 0, llm_input_summary TEXT NOT NULL, llm_output_summary TEXT NOT NULL, latency_ms INTEGER NOT NULL DEFAULT 0, cost REAL NOT NULL DEFAULT 0, rule_hits_json TEXT, review_result TEXT, created_at TEXT NOT NULL);`); err != nil {
		t.Fatalf("create helper trace_records failed: %v", err)
	}
	if err := ensureTraceEvalColumns(context.Background(), db, "trace_records", map[string]string{"asset_refs_json": "TEXT"}); err != nil {
		t.Fatalf("ensureTraceEvalColumns returned error: %v", err)
	}
	if err := ensureTraceEvalColumns(context.Background(), db, "trace_records", map[string]string{"asset_refs_json": "TEXT"}); err != nil {
		t.Fatalf("ensureTraceEvalColumns idempotency returned error: %v", err)
	}
	columns, err := traceEvalTableColumns(context.Background(), db, "trace_records")
	if err != nil {
		t.Fatalf("traceEvalTableColumns returned error: %v", err)
	}
	if _, ok := columns["asset_refs_json"]; !ok {
		t.Fatalf("expected asset_refs_json column to be added, got %+v", columns)
	}
	if _, err := db.Exec(`CREATE TABLE eval_snapshots (eval_snapshot_id TEXT PRIMARY KEY, trace_id TEXT NOT NULL, task_id TEXT NOT NULL, status TEXT NOT NULL, asset_refs_json TEXT, metrics_json TEXT NOT NULL, created_at TEXT NOT NULL, FOREIGN KEY(trace_id) REFERENCES trace_records(trace_id));`); err != nil {
		t.Fatalf("create helper eval_snapshots failed: %v", err)
	}
	if err := ensureEvalSnapshotForeignKey(context.Background(), db); err != nil {
		t.Fatalf("expected helper foreign-key check to pass without migration, got %v", err)
	}
	traceStore, err := NewSQLiteTraceStore(databasePath)
	if err != nil {
		t.Fatalf("NewSQLiteTraceStore returned error: %v", err)
	}
	defer func() { _ = traceStore.Close() }()
	evalStore, err := NewSQLiteEvalStore(databasePath)
	if err != nil {
		t.Fatalf("NewSQLiteEvalStore returned error: %v", err)
	}
	defer func() { _ = evalStore.Close() }()
	if err := traceStore.initialize(context.Background()); err != nil {
		t.Fatalf("expected repeated trace initialize to succeed, got %v", err)
	}
	if err := evalStore.initialize(context.Background()); err != nil {
		t.Fatalf("expected repeated eval initialize to succeed, got %v", err)
	}
}

func TestTraceAndEvalStoresCoverAdditionalPagingAndErrorBranches(t *testing.T) {
	traceStore := newInMemoryTraceStore()
	evalStore := newInMemoryEvalStore()
	if err := traceStore.WriteTraceRecord(context.Background(), TraceRecord{TraceID: "trace_a", TaskID: "task_a", CreatedAt: "2026-04-17T10:00:00Z"}); err != nil {
		t.Fatalf("write in-memory trace record failed: %v", err)
	}
	if err := traceStore.WriteTraceRecord(context.Background(), TraceRecord{TraceID: "trace_b", TaskID: "task_b", CreatedAt: "2026-04-17T10:01:00Z"}); err != nil {
		t.Fatalf("write in-memory trace record failed: %v", err)
	}
	if err := evalStore.WriteEvalSnapshot(context.Background(), EvalSnapshotRecord{EvalSnapshotID: "eval_a", TraceID: "trace_a", TaskID: "task_a", Status: "passed", MetricsJSON: `{"latency_ms":1}`, CreatedAt: "2026-04-17T10:00:00Z"}); err != nil {
		t.Fatalf("write in-memory eval snapshot failed: %v", err)
	}
	if err := evalStore.WriteEvalSnapshot(context.Background(), EvalSnapshotRecord{EvalSnapshotID: "eval_b", TraceID: "trace_b", TaskID: "task_b", Status: "passed", MetricsJSON: `{"latency_ms":2}`, CreatedAt: "2026-04-17T10:01:00Z"}); err != nil {
		t.Fatalf("write in-memory eval snapshot failed: %v", err)
	}
	if err := traceStore.DeleteTraceRecord(context.Background(), "trace_missing"); err != nil {
		t.Fatalf("delete missing in-memory trace record failed: %v", err)
	}
	traces, total, err := traceStore.ListTraceRecords(context.Background(), "", 0, 1)
	if err != nil || total != 2 || len(traces) != 1 || traces[0].TraceID != "trace_a" {
		t.Fatalf("expected unlimited in-memory trace paging from offset, total=%d traces=%+v err=%v", total, traces, err)
	}
	evals, total, err := evalStore.ListEvalSnapshots(context.Background(), "", 0, 0)
	if err != nil || total != 2 || len(evals) != 2 || evals[0].EvalSnapshotID != "eval_b" {
		t.Fatalf("expected unlimited in-memory eval paging to preserve sort order, total=%d evals=%+v err=%v", total, evals, err)
	}

	databasePath := filepath.Join(t.TempDir(), "trace-eval-errors.db")
	sqliteTraceStore, err := NewSQLiteTraceStore(databasePath)
	if err != nil {
		t.Fatalf("NewSQLiteTraceStore returned error: %v", err)
	}
	sqliteEvalStore, err := NewSQLiteEvalStore(databasePath)
	if err != nil {
		t.Fatalf("NewSQLiteEvalStore returned error: %v", err)
	}
	if traces, total, err := sqliteTraceStore.ListTraceRecords(context.Background(), "", 0, 0); err != nil || total != 0 || len(traces) != 0 {
		t.Fatalf("expected empty sqlite trace list without limit to succeed, total=%d traces=%+v err=%v", total, traces, err)
	}
	if evals, total, err := sqliteEvalStore.ListEvalSnapshots(context.Background(), "", 0, 0); err != nil || total != 0 || len(evals) != 0 {
		t.Fatalf("expected empty sqlite eval list without limit to succeed, total=%d evals=%+v err=%v", total, evals, err)
	}
	if err := sqliteTraceStore.Close(); err != nil {
		t.Fatalf("Close trace store returned error: %v", err)
	}
	if err := sqliteEvalStore.Close(); err != nil {
		t.Fatalf("Close eval store returned error: %v", err)
	}
	if err := sqliteTraceStore.WriteTraceRecord(context.Background(), TraceRecord{TraceID: "trace_closed", TaskID: "task_closed", CreatedAt: "2026-04-17T11:00:00Z"}); err == nil {
		t.Fatal("expected WriteTraceRecord on closed sqlite trace store to fail")
	}
	if err := sqliteTraceStore.DeleteTraceRecord(context.Background(), "trace_closed"); err == nil {
		t.Fatal("expected DeleteTraceRecord on closed sqlite trace store to fail")
	}
	if _, _, err := sqliteTraceStore.ListTraceRecords(context.Background(), "", 10, 0); err == nil {
		t.Fatal("expected ListTraceRecords on closed sqlite trace store to fail")
	}
	if err := sqliteTraceStore.initialize(context.Background()); err == nil {
		t.Fatal("expected initialize on closed sqlite trace store to fail")
	}
	if err := sqliteEvalStore.WriteEvalSnapshot(context.Background(), EvalSnapshotRecord{EvalSnapshotID: "eval_closed", TraceID: "trace_closed", TaskID: "task_closed", Status: "failed", MetricsJSON: `{"latency_ms":3}`, CreatedAt: "2026-04-17T11:00:01Z"}); err == nil {
		t.Fatal("expected WriteEvalSnapshot on closed sqlite eval store to fail")
	}
	if _, _, err := sqliteEvalStore.ListEvalSnapshots(context.Background(), "", 10, 0); err == nil {
		t.Fatal("expected ListEvalSnapshots on closed sqlite eval store to fail")
	}
	if err := sqliteEvalStore.initialize(context.Background()); err == nil {
		t.Fatal("expected initialize on closed sqlite eval store to fail")
	}

	helperDB, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "trace-helper-errors.db"))
	if err != nil {
		t.Fatalf("open sqlite helper db failed: %v", err)
	}
	if err := helperDB.Close(); err != nil {
		t.Fatalf("close sqlite helper db failed: %v", err)
	}
	if err := ensureEvalSnapshotForeignKey(context.Background(), helperDB); err == nil {
		t.Fatal("expected ensureEvalSnapshotForeignKey on closed db to fail")
	}
	if err := ensureTraceEvalColumns(context.Background(), helperDB, "trace_records", map[string]string{"asset_refs_json": "TEXT"}); err == nil {
		t.Fatal("expected ensureTraceEvalColumns on closed db to fail")
	}
	if _, err := traceEvalTableColumns(context.Background(), helperDB, "trace_records"); err == nil {
		t.Fatal("expected traceEvalTableColumns on closed db to fail")
	}
}
