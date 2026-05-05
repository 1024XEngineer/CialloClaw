package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

type inMemoryToolCallStore struct {
	mu      sync.Mutex
	records []tools.ToolCallRecord
}

func newInMemoryToolCallStore() *inMemoryToolCallStore {
	return &inMemoryToolCallStore{records: make([]tools.ToolCallRecord, 0)}
}

func (s *inMemoryToolCallStore) SaveToolCall(_ context.Context, record tools.ToolCallRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record = normalizeStoredToolCallRecord(record)
	for index, existing := range s.records {
		if existing.ToolCallID != record.ToolCallID {
			continue
		}
		s.records = append(append(append([]tools.ToolCallRecord{}, s.records[:index]...), s.records[index+1:]...), record)
		return nil
	}
	s.records = append(s.records, record)
	return nil
}

func normalizeStoredToolCallRecord(record tools.ToolCallRecord) tools.ToolCallRecord {
	// Keep the in-memory fallback backend aligned with SQLite by round-tripping the
	// same normalize/denormalize helpers that the persistent backend uses.
	record.Status = denormalizeToolCallStatus(normalizeToolCallStatus(record.Status))
	if record.CreatedAt == "" {
		record.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	return record
}

func (s *inMemoryToolCallStore) ListToolCalls(_ context.Context, taskID, runID string, limit, offset int) ([]tools.ToolCallRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]tools.ToolCallRecord, 0, len(s.records))
	for _, record := range s.records {
		if taskID != "" && record.TaskID != taskID {
			continue
		}
		if runID != "" && record.RunID != runID {
			continue
		}
		items = append(items, record)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt == items[j].CreatedAt {
			return items[i].ToolCallID > items[j].ToolCallID
		}
		return items[i].CreatedAt > items[j].CreatedAt
	})
	total := len(items)
	if offset >= total {
		return []tools.ToolCallRecord{}, total, nil
	}
	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}
	return append([]tools.ToolCallRecord(nil), items[offset:end]...), total, nil
}

// ListErrorToolCalls exposes only failed/error-bearing tool calls so higher
// level log queries can page the failure subset at the storage boundary.
func (s *inMemoryToolCallStore) ListErrorToolCalls(_ context.Context, taskID, runID string, limit, offset int) ([]tools.ToolCallRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]tools.ToolCallRecord, 0, len(s.records))
	for _, record := range s.records {
		if taskID != "" && record.TaskID != taskID {
			continue
		}
		if runID != "" && record.RunID != runID {
			continue
		}
		if !toolCallRecordMatchesErrorLog(record) {
			continue
		}
		items = append(items, record)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt == items[j].CreatedAt {
			return items[i].ToolCallID > items[j].ToolCallID
		}
		return items[i].CreatedAt > items[j].CreatedAt
	})
	total := len(items)
	if offset >= total {
		return []tools.ToolCallRecord{}, total, nil
	}
	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}
	return append([]tools.ToolCallRecord(nil), items[offset:end]...), total, nil
}

type SQLiteToolCallStore struct {
	db *sql.DB
}

func NewSQLiteToolCallStore(databasePath string) (*SQLiteToolCallStore, error) {
	cleanedPath, err := prepareSQLiteDatabasePath(databasePath)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(sqliteDriverName, cleanedPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}
	store := &SQLiteToolCallStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteToolCallStore) SaveToolCall(ctx context.Context, record tools.ToolCallRecord) error {
	if record.CreatedAt == "" {
		record.CreatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	inputJSON, err := json.Marshal(record.Input)
	if err != nil {
		return fmt.Errorf("marshal tool call input: %w", err)
	}
	outputJSON, err := json.Marshal(record.Output)
	if err != nil {
		return fmt.Errorf("marshal tool call output: %w", err)
	}
	_, err = s.db.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO tool_calls (tool_call_id, run_id, task_id, step_id, tool_name, status, input_json, output_json, error_code, duration_ms, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ToolCallID,
		record.RunID,
		record.TaskID,
		record.StepID,
		record.ToolName,
		normalizeToolCallStatus(record.Status),
		string(inputJSON),
		string(outputJSON),
		record.ErrorCode,
		record.DurationMS,
		record.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save tool call: %w", err)
	}
	return nil
}

func (s *SQLiteToolCallStore) ListToolCalls(ctx context.Context, taskID, runID string, limit, offset int) ([]tools.ToolCallRecord, int, error) {
	countQuery := `SELECT COUNT(1) FROM tool_calls WHERE 1 = 1`
	query := `SELECT tool_call_id, run_id, task_id, step_id, tool_name, status, input_json, output_json, error_code, duration_ms, created_at FROM tool_calls WHERE 1 = 1`
	args := make([]any, 0, 4)
	countArgs := make([]any, 0, 2)
	if taskID != "" {
		countQuery += ` AND task_id = ?`
		query += ` AND task_id = ?`
		args = append(args, taskID)
		countArgs = append(countArgs, taskID)
	}
	if runID != "" {
		countQuery += ` AND run_id = ?`
		query += ` AND run_id = ?`
		args = append(args, runID)
		countArgs = append(countArgs, runID)
	}
	query += ` ORDER BY created_at DESC, tool_call_id DESC`
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tool calls: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tool calls: %w", err)
	}
	defer rows.Close()
	items := make([]tools.ToolCallRecord, 0)
	for rows.Next() {
		record, err := scanToolCallRecord(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate tool calls: %w", err)
	}
	return items, total, nil
}

// ListErrorToolCalls exposes only failed/error-bearing tool-call rows so merged
// execution logs do not need to scan every stored tool call before filtering.
func (s *SQLiteToolCallStore) ListErrorToolCalls(ctx context.Context, taskID, runID string, limit, offset int) ([]tools.ToolCallRecord, int, error) {
	countQuery := `SELECT COUNT(1) FROM tool_calls WHERE 1 = 1`
	query := `SELECT tool_call_id, run_id, task_id, step_id, tool_name, status, input_json, output_json, error_code, duration_ms, created_at FROM tool_calls WHERE 1 = 1`
	args := make([]any, 0, 4)
	countArgs := make([]any, 0, 2)
	if taskID != "" {
		countQuery += ` AND task_id = ?`
		query += ` AND task_id = ?`
		args = append(args, taskID)
		countArgs = append(countArgs, taskID)
	}
	if runID != "" {
		countQuery += ` AND run_id = ?`
		query += ` AND run_id = ?`
		args = append(args, runID)
		countArgs = append(countArgs, runID)
	}
	countQuery += ` AND (status = 'failed' OR error_code IS NOT NULL)`
	query += ` AND (status = 'failed' OR error_code IS NOT NULL)`
	query += ` ORDER BY created_at DESC, tool_call_id DESC`
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count error tool calls: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list error tool calls: %w", err)
	}
	defer rows.Close()
	items := make([]tools.ToolCallRecord, 0)
	for rows.Next() {
		record, err := scanToolCallRecord(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate error tool calls: %w", err)
	}
	return items, total, nil
}

func toolCallRecordMatchesErrorLog(record tools.ToolCallRecord) bool {
	return record.ErrorCode != nil ||
		record.Status == tools.ToolCallStatusFailed ||
		record.Status == tools.ToolCallStatusTimeout ||
		strings.EqualFold(strings.TrimSpace(string(record.Status)), string(tools.ToolCallStatusFailed))
}

func normalizeToolCallStatus(status tools.ToolCallStatus) string {
	switch status {
	case tools.ToolCallStatusStarted:
		return "running"
	case tools.ToolCallStatusSucceeded:
		return "succeeded"
	case tools.ToolCallStatusFailed, tools.ToolCallStatusTimeout:
		return "failed"
	default:
		return "pending"
	}
}

func (s *SQLiteToolCallStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteToolCallStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=5000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS tool_calls (
			tool_call_id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL,
			task_id TEXT NOT NULL,
			step_id TEXT NOT NULL,
			tool_name TEXT NOT NULL,
			status TEXT NOT NULL,
			input_json TEXT NOT NULL,
			output_json TEXT NOT NULL,
			error_code INTEGER,
			duration_ms INTEGER NOT NULL,
			created_at TEXT NOT NULL DEFAULT ''
		);
	`); err != nil {
		return fmt.Errorf("create tool_calls table: %w", err)
	}
	if err := ensureToolCallColumns(ctx, s.db); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_tool_calls_task_run ON tool_calls(task_id, run_id);`); err != nil {
		return fmt.Errorf("create tool_calls index: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_tool_calls_task_time ON tool_calls(task_id, created_at DESC, tool_call_id DESC);`); err != nil {
		return fmt.Errorf("create tool_calls time index: %w", err)
	}
	return nil
}

func ensureToolCallColumns(ctx context.Context, db *sql.DB) error {
	columns, err := toolCallTableColumns(ctx, db)
	if err != nil {
		return err
	}
	if _, ok := columns["created_at"]; !ok {
		if _, err := db.ExecContext(ctx, `ALTER TABLE tool_calls ADD COLUMN created_at TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migrate tool_calls add created_at: %w", err)
		}
	}
	if err := backfillToolCallCreatedAt(ctx, db); err != nil {
		return err
	}
	return nil
}

func backfillToolCallCreatedAt(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `SELECT rowid FROM tool_calls WHERE created_at = '' ORDER BY rowid ASC`)
	if err != nil {
		return fmt.Errorf("load tool_calls rowids for created_at backfill: %w", err)
	}
	defer rows.Close()
	rowIDs := make([]int64, 0)
	for rows.Next() {
		var rowID int64
		if err := rows.Scan(&rowID); err != nil {
			return fmt.Errorf("scan tool_calls rowid for created_at backfill: %w", err)
		}
		rowIDs = append(rowIDs, rowID)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tool_calls rowids for created_at backfill: %w", err)
	}
	if len(rowIDs) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tool_calls created_at backfill: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()
	const layout = time.RFC3339Nano
	base := time.Unix(0, 0).UTC()
	for index, rowID := range rowIDs {
		if index > math.MaxInt32 {
			return fmt.Errorf("backfill tool_calls created_at exceeded supported row count")
		}
		createdAt := base.Add(time.Duration(index) * time.Second).Format(layout)
		if _, err := tx.ExecContext(ctx, `UPDATE tool_calls SET created_at = ? WHERE rowid = ? AND created_at = ''`, createdAt, rowID); err != nil {
			return fmt.Errorf("backfill tool_calls created_at row %d: %w", rowID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tool_calls created_at backfill: %w", err)
	}
	tx = nil
	return nil
}

func toolCallTableColumns(ctx context.Context, db *sql.DB) (map[string]struct{}, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(tool_calls);`)
	if err != nil {
		return nil, fmt.Errorf("inspect tool_calls schema: %w", err)
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
			return nil, fmt.Errorf("scan tool_calls schema: %w", err)
		}
		columns[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tool_calls schema: %w", err)
	}
	return columns, nil
}

func scanToolCallRecord(rows *sql.Rows) (tools.ToolCallRecord, error) {
	var (
		record     tools.ToolCallRecord
		inputJSON  string
		outputJSON string
		errorCode  sql.NullInt64
		status     string
		stepID     string
		createdAt  string
	)
	if err := rows.Scan(&record.ToolCallID, &record.RunID, &record.TaskID, &stepID, &record.ToolName, &status, &inputJSON, &outputJSON, &errorCode, &record.DurationMS, &createdAt); err != nil {
		return tools.ToolCallRecord{}, fmt.Errorf("scan tool call: %w", err)
	}
	record.StepID = stepID
	record.CreatedAt = createdAt
	record.Status = denormalizeToolCallStatus(status)
	if errorCode.Valid {
		converted := int(errorCode.Int64)
		record.ErrorCode = &converted
	}
	if inputJSON != "" {
		if err := json.Unmarshal([]byte(inputJSON), &record.Input); err != nil {
			return tools.ToolCallRecord{}, fmt.Errorf("decode tool call input: %w", err)
		}
	}
	if outputJSON != "" {
		if err := json.Unmarshal([]byte(outputJSON), &record.Output); err != nil {
			return tools.ToolCallRecord{}, fmt.Errorf("decode tool call output: %w", err)
		}
	}
	return record, nil
}

func denormalizeToolCallStatus(status string) tools.ToolCallStatus {
	switch status {
	case "running":
		return tools.ToolCallStatusStarted
	case "succeeded":
		return tools.ToolCallStatusSucceeded
	case "failed":
		return tools.ToolCallStatusFailed
	default:
		return tools.ToolCallStatusStarted
	}
}
