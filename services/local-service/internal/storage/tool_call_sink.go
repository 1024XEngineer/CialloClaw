package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

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
	return record
}

func (s *inMemoryToolCallStore) ListToolCalls(_ context.Context, taskID, runID string, limit, offset int) ([]tools.ToolCallRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]tools.ToolCallRecord, 0, len(s.records))
	for index := len(s.records) - 1; index >= 0; index-- {
		record := s.records[index]
		if taskID != "" && record.TaskID != taskID {
			continue
		}
		if runID != "" && record.RunID != runID {
			continue
		}
		items = append(items, record)
	}
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
	databasePath = filepath.Clean(databasePath)
	if databasePath == "" {
		return nil, ErrDatabasePathRequired
	}
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
		return nil, fmt.Errorf("prepare sqlite directory: %w", err)
	}
	db, err := sql.Open(sqliteDriverName, databasePath)
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
		`INSERT OR REPLACE INTO tool_calls (tool_call_id, run_id, task_id, step_id, tool_name, status, input_json, output_json, error_code, duration_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
	)
	if err != nil {
		return fmt.Errorf("save tool call: %w", err)
	}
	return nil
}

func (s *SQLiteToolCallStore) ListToolCalls(ctx context.Context, taskID, runID string, limit, offset int) ([]tools.ToolCallRecord, int, error) {
	countQuery := `SELECT COUNT(1) FROM tool_calls WHERE 1 = 1`
	query := `SELECT tool_call_id, run_id, task_id, step_id, tool_name, status, input_json, output_json, error_code, duration_ms FROM tool_calls WHERE 1 = 1`
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
	query += ` ORDER BY rowid DESC`
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
			duration_ms INTEGER NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create tool_calls table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_tool_calls_task_run ON tool_calls(task_id, run_id);`); err != nil {
		return fmt.Errorf("create tool_calls index: %w", err)
	}
	return nil
}

func scanToolCallRecord(rows *sql.Rows) (tools.ToolCallRecord, error) {
	var (
		record     tools.ToolCallRecord
		inputJSON  string
		outputJSON string
		errorCode  sql.NullInt64
		status     string
		stepID     string
	)
	if err := rows.Scan(&record.ToolCallID, &record.RunID, &record.TaskID, &stepID, &record.ToolName, &status, &inputJSON, &outputJSON, &errorCode, &record.DurationMS); err != nil {
		return tools.ToolCallRecord{}, fmt.Errorf("scan tool call: %w", err)
	}
	record.StepID = stepID
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
