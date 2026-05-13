package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
)

const orphanedEvalSnapshotsTable = "eval_snapshots_orphaned"

type inMemoryTraceStore struct {
	mu      sync.Mutex
	records []TraceRecord
}

func newInMemoryTraceStore() *inMemoryTraceStore {
	return &inMemoryTraceStore{records: make([]TraceRecord, 0)}
}

func (s *inMemoryTraceStore) WriteTraceRecord(_ context.Context, record TraceRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, record)
	return nil
}

func (s *inMemoryTraceStore) DeleteTraceRecord(_ context.Context, traceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := s.records[:0]
	for _, record := range s.records {
		if record.TraceID == traceID {
			continue
		}
		filtered = append(filtered, record)
	}
	s.records = filtered
	return nil
}

func (s *inMemoryTraceStore) ListTraceRecords(_ context.Context, taskID string, limit, offset int) ([]TraceRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]TraceRecord, 0)
	for _, record := range s.records {
		if taskID == "" || record.TaskID == taskID {
			items = append(items, record)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return parseGovernanceTime(items[i].CreatedAt).After(parseGovernanceTime(items[j].CreatedAt))
	})
	return pageTraceRecords(items, limit, offset), len(items), nil
}

type inMemoryEvalStore struct {
	mu      sync.Mutex
	records []EvalSnapshotRecord
}

func newInMemoryEvalStore() *inMemoryEvalStore {
	return &inMemoryEvalStore{records: make([]EvalSnapshotRecord, 0)}
}

func (s *inMemoryEvalStore) WriteEvalSnapshot(_ context.Context, record EvalSnapshotRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, record)
	return nil
}

func (s *inMemoryEvalStore) ListEvalSnapshots(_ context.Context, taskID string, limit, offset int) ([]EvalSnapshotRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]EvalSnapshotRecord, 0)
	for _, record := range s.records {
		if taskID == "" || record.TaskID == taskID {
			items = append(items, record)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return parseGovernanceTime(items[i].CreatedAt).After(parseGovernanceTime(items[j].CreatedAt))
	})
	return pageEvalSnapshots(items, limit, offset), len(items), nil
}

// SQLiteTraceStore persists trace records in SQLite.
type SQLiteTraceStore struct {
	db *sql.DB
}

// NewSQLiteTraceStore creates and returns a SQLiteTraceStore.
func NewSQLiteTraceStore(databasePath string) (*SQLiteTraceStore, error) {
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteTraceStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteTraceStore) WriteTraceRecord(ctx context.Context, record TraceRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO trace_records (
			trace_id, task_id, run_id, loop_round, llm_input_summary, llm_output_summary,
			latency_ms, cost, asset_refs_json, rule_hits_json, review_result, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, record.TraceID, record.TaskID, nullableString(record.RunID), record.LoopRound, record.LLMInputSummary, record.LLMOutputSummary, record.LatencyMS, record.Cost, nullableString(record.AssetRefsJSON), nullableString(record.RuleHitsJSON), nullableString(record.ReviewResult), record.CreatedAt)
	if err != nil {
		return fmt.Errorf("write trace record: %w", err)
	}
	return nil
}

func (s *SQLiteTraceStore) DeleteTraceRecord(ctx context.Context, traceID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete trace record transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM eval_snapshots WHERE trace_id = ?`, traceID); err != nil {
		return fmt.Errorf("delete eval snapshots for trace rollback: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM trace_records WHERE trace_id = ?`, traceID); err != nil {
		return fmt.Errorf("delete trace record: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete trace record transaction: %w", err)
	}
	return nil
}

func (s *SQLiteTraceStore) ListTraceRecords(ctx context.Context, taskID string, limit, offset int) ([]TraceRecord, int, error) {
	countQuery := `SELECT COUNT(1) FROM trace_records`
	query := `SELECT trace_id, task_id, run_id, loop_round, llm_input_summary, llm_output_summary, latency_ms, cost, asset_refs_json, rule_hits_json, review_result, created_at FROM trace_records`
	args := []any{}
	if taskID != "" {
		countQuery += ` WHERE task_id = ?`
		query += ` WHERE task_id = ?`
		args = append(args, taskID)
	}
	query += ` ORDER BY created_at DESC, trace_id DESC`
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, firstArg(taskID)...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count trace records: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list trace records: %w", err)
	}
	defer rows.Close()

	items := make([]TraceRecord, 0)
	for rows.Next() {
		var record TraceRecord
		var runID sql.NullString
		var assetRefs sql.NullString
		var ruleHits sql.NullString
		var reviewResult sql.NullString
		if err := rows.Scan(&record.TraceID, &record.TaskID, &runID, &record.LoopRound, &record.LLMInputSummary, &record.LLMOutputSummary, &record.LatencyMS, &record.Cost, &assetRefs, &ruleHits, &reviewResult, &record.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan trace record: %w", err)
		}
		record.RunID = runID.String
		record.AssetRefsJSON = assetRefs.String
		record.RuleHitsJSON = ruleHits.String
		record.ReviewResult = reviewResult.String
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate trace records: %w", err)
	}
	return items, total, nil
}

func (s *SQLiteTraceStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteTraceStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys=ON;`); err != nil {
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=300000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS trace_records (
			trace_id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			run_id TEXT,
			loop_round INTEGER NOT NULL DEFAULT 0,
			llm_input_summary TEXT NOT NULL,
			llm_output_summary TEXT NOT NULL,
			latency_ms INTEGER NOT NULL DEFAULT 0,
			cost REAL NOT NULL DEFAULT 0,
			asset_refs_json TEXT,
			rule_hits_json TEXT,
			review_result TEXT,
			created_at TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create trace_records table: %w", err)
	}
	if err := ensureTraceEvalColumns(ctx, s.db, "trace_records", map[string]string{"asset_refs_json": "TEXT"}); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_trace_records_task_time ON trace_records(task_id, created_at DESC);`); err != nil {
		return fmt.Errorf("create trace_records index: %w", err)
	}
	return nil
}

// SQLiteEvalStore persists eval snapshots in SQLite.
type SQLiteEvalStore struct {
	db *sql.DB
}

// NewSQLiteEvalStore creates and returns a SQLiteEvalStore.
func NewSQLiteEvalStore(databasePath string) (*SQLiteEvalStore, error) {
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteEvalStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteEvalStore) WriteEvalSnapshot(ctx context.Context, record EvalSnapshotRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO eval_snapshots (
			eval_snapshot_id, trace_id, task_id, status, asset_refs_json, metrics_json, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`, record.EvalSnapshotID, record.TraceID, record.TaskID, record.Status, nullableString(record.AssetRefsJSON), record.MetricsJSON, record.CreatedAt)
	if err != nil {
		return fmt.Errorf("write eval snapshot: %w", err)
	}
	return nil
}

func (s *SQLiteEvalStore) ListEvalSnapshots(ctx context.Context, taskID string, limit, offset int) ([]EvalSnapshotRecord, int, error) {
	countQuery := `SELECT COUNT(1) FROM eval_snapshots`
	query := `SELECT eval_snapshot_id, trace_id, task_id, status, asset_refs_json, metrics_json, created_at FROM eval_snapshots`
	args := []any{}
	if taskID != "" {
		countQuery += ` WHERE task_id = ?`
		query += ` WHERE task_id = ?`
		args = append(args, taskID)
	}
	query += ` ORDER BY created_at DESC, eval_snapshot_id DESC`
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, firstArg(taskID)...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count eval snapshots: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list eval snapshots: %w", err)
	}
	defer rows.Close()

	items := make([]EvalSnapshotRecord, 0)
	for rows.Next() {
		var record EvalSnapshotRecord
		var assetRefs sql.NullString
		if err := rows.Scan(&record.EvalSnapshotID, &record.TraceID, &record.TaskID, &record.Status, &assetRefs, &record.MetricsJSON, &record.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan eval snapshot: %w", err)
		}
		record.AssetRefsJSON = assetRefs.String
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate eval snapshots: %w", err)
	}
	return items, total, nil
}

func (s *SQLiteEvalStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteEvalStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA foreign_keys=ON;`); err != nil {
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=300000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS eval_snapshots (
			eval_snapshot_id TEXT PRIMARY KEY,
			trace_id TEXT NOT NULL,
			task_id TEXT NOT NULL,
			status TEXT NOT NULL,
			asset_refs_json TEXT,
			metrics_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(trace_id) REFERENCES trace_records(trace_id)
		);
	`); err != nil {
		return fmt.Errorf("create eval_snapshots table: %w", err)
	}
	if err := ensureTraceEvalColumns(ctx, s.db, "eval_snapshots", map[string]string{"asset_refs_json": "TEXT"}); err != nil {
		return err
	}
	if err := ensureEvalSnapshotForeignKey(ctx, s.db); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_eval_snapshots_task_time ON eval_snapshots(task_id, created_at DESC);`); err != nil {
		return fmt.Errorf("create eval_snapshots index: %w", err)
	}
	return nil
}

func ensureEvalSnapshotForeignKey(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `PRAGMA foreign_key_list(eval_snapshots);`)
	if err != nil {
		return fmt.Errorf("inspect eval_snapshots foreign keys: %w", err)
	}
	defer rows.Close()

	hasTraceForeignKey := false
	for rows.Next() {
		var (
			id       int
			seq      int
			table    string
			from     string
			to       string
			onUpdate string
			onDelete string
			match    string
		)
		if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return fmt.Errorf("scan eval_snapshots foreign keys: %w", err)
		}
		if table == "trace_records" && from == "trace_id" && to == "trace_id" {
			hasTraceForeignKey = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate eval_snapshots foreign keys: %w", err)
	}
	if hasTraceForeignKey {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin eval_snapshots foreign key migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `ALTER TABLE eval_snapshots RENAME TO eval_snapshots_legacy;`); err != nil {
		return fmt.Errorf("rename legacy eval_snapshots table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE eval_snapshots (
			eval_snapshot_id TEXT PRIMARY KEY,
			trace_id TEXT NOT NULL,
			task_id TEXT NOT NULL,
			status TEXT NOT NULL,
			asset_refs_json TEXT,
			metrics_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(trace_id) REFERENCES trace_records(trace_id)
		);
	`); err != nil {
		return fmt.Errorf("recreate eval_snapshots table with foreign key: %w", err)
	}
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			eval_snapshot_id TEXT PRIMARY KEY,
			trace_id TEXT NOT NULL,
			task_id TEXT NOT NULL,
			status TEXT NOT NULL,
			asset_refs_json TEXT,
			metrics_json TEXT NOT NULL,
			created_at TEXT NOT NULL,
			quarantined_at TEXT NOT NULL,
			reason TEXT NOT NULL
		);
	`, orphanedEvalSnapshotsTable)); err != nil {
		return fmt.Errorf("create orphaned eval_snapshots quarantine table: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO eval_snapshots (eval_snapshot_id, trace_id, task_id, status, asset_refs_json, metrics_json, created_at)
		SELECT eval_snapshot_id, trace_id, task_id, status, NULL, metrics_json, created_at
		FROM eval_snapshots_legacy;
	`); err != nil {
		if _, quarantineErr := tx.ExecContext(ctx, fmt.Sprintf(`
			INSERT OR REPLACE INTO %s (
				eval_snapshot_id, trace_id, task_id, status, asset_refs_json, metrics_json, created_at, quarantined_at, reason
			)
			SELECT legacy.eval_snapshot_id, legacy.trace_id, legacy.task_id, legacy.status, NULL, legacy.metrics_json, legacy.created_at, CURRENT_TIMESTAMP, ?
			FROM eval_snapshots_legacy legacy
			LEFT JOIN trace_records traces ON traces.trace_id = legacy.trace_id
			WHERE traces.trace_id IS NULL;
		`, orphanedEvalSnapshotsTable), "missing trace_records parent during foreign key migration"); quarantineErr != nil {
			return fmt.Errorf("quarantine orphaned eval_snapshots rows: %w", quarantineErr)
		}
		if _, copyErr := tx.ExecContext(ctx, `
			INSERT INTO eval_snapshots (eval_snapshot_id, trace_id, task_id, status, asset_refs_json, metrics_json, created_at)
			SELECT legacy.eval_snapshot_id, legacy.trace_id, legacy.task_id, legacy.status, NULL, legacy.metrics_json, legacy.created_at
			FROM eval_snapshots_legacy legacy
			INNER JOIN trace_records traces ON traces.trace_id = legacy.trace_id;
		`); copyErr != nil {
			return fmt.Errorf("copy compatible eval_snapshots rows into migrated table: %w", copyErr)
		}
	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE eval_snapshots_legacy;`); err != nil {
		return fmt.Errorf("drop legacy eval_snapshots table: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit eval_snapshots foreign key migration: %w", err)
	}
	return nil
}

func ensureTraceEvalColumns(ctx context.Context, db *sql.DB, tableName string, requiredColumns map[string]string) error {
	existingColumns, err := traceEvalTableColumns(ctx, db, tableName)
	if err != nil {
		return err
	}
	for name, definition := range requiredColumns {
		if _, ok := existingColumns[name]; ok {
			continue
		}
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, tableName, name, definition)); err != nil {
			return fmt.Errorf("migrate %s add column %s: %w", tableName, name, err)
		}
	}
	return nil
}

func traceEvalTableColumns(ctx context.Context, db *sql.DB, tableName string) (map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s);`, tableName))
	if err != nil {
		return nil, fmt.Errorf("inspect %s schema: %w", tableName, err)
	}
	defer rows.Close()
	columns := make(map[string]struct{})
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return nil, fmt.Errorf("scan %s schema: %w", tableName, err)
		}
		columns[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s schema: %w", tableName, err)
	}
	return columns, nil
}

func pageTraceRecords(items []TraceRecord, limit, offset int) []TraceRecord {
	if offset >= len(items) {
		return []TraceRecord{}
	}
	if limit <= 0 {
		return append([]TraceRecord(nil), items[offset:]...)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return append([]TraceRecord(nil), items[offset:end]...)
}

func pageEvalSnapshots(items []EvalSnapshotRecord, limit, offset int) []EvalSnapshotRecord {
	if offset >= len(items) {
		return []EvalSnapshotRecord{}
	}
	if limit <= 0 {
		return append([]EvalSnapshotRecord(nil), items[offset:]...)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return append([]EvalSnapshotRecord(nil), items[offset:end]...)
}
