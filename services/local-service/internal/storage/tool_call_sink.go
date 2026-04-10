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
	s.records = append(s.records, record)
	return nil
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
