package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"sync"
)

type inMemoryTaskStore struct {
	mu      sync.Mutex
	records map[string]TaskRecord
}

func newInMemoryTaskStore() *inMemoryTaskStore {
	return &inMemoryTaskStore{records: make(map[string]TaskRecord)}
}

func (s *inMemoryTaskStore) WriteTask(_ context.Context, record TaskRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[record.TaskID] = record
	return nil
}

func (s *inMemoryTaskStore) DeleteTask(_ context.Context, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, taskID)
	return nil
}

func (s *inMemoryTaskStore) GetTask(_ context.Context, taskID string) (TaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[taskID]
	if !ok {
		return TaskRecord{}, sql.ErrNoRows
	}
	return record, nil
}

func (s *inMemoryTaskStore) ListTasks(_ context.Context, limit, offset int) ([]TaskRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]TaskRecord, 0, len(s.records))
	for _, record := range s.records {
		items = append(items, record)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return parseGovernanceTime(items[i].StartedAt).After(parseGovernanceTime(items[j].StartedAt))
	})
	return pageTasks(items, limit, offset), len(items), nil
}

type inMemoryTaskStepStore struct {
	mu      sync.Mutex
	records map[string][]TaskStepRecord
}

func newInMemoryTaskStepStore() *inMemoryTaskStepStore {
	return &inMemoryTaskStepStore{records: make(map[string][]TaskStepRecord)}
}

func (s *inMemoryTaskStepStore) ReplaceTaskSteps(_ context.Context, taskID string, records []TaskStepRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := append([]TaskStepRecord(nil), records...)
	s.records[taskID] = cloned
	return nil
}

func (s *inMemoryTaskStepStore) ListTaskSteps(_ context.Context, taskID string, limit, offset int) ([]TaskStepRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := append([]TaskStepRecord(nil), s.records[taskID]...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].OrderIndex == items[j].OrderIndex {
			return items[i].StepID < items[j].StepID
		}
		return items[i].OrderIndex < items[j].OrderIndex
	})
	return pageTaskSteps(items, limit, offset), len(items), nil
}

type SQLiteTaskStore struct {
	db *sql.DB
}

func NewSQLiteTaskStore(databasePath string) (*SQLiteTaskStore, error) {
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteTaskStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteTaskStore) WriteTask(ctx context.Context, record TaskRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO tasks (
			task_id, session_id, run_id, title, source_type, status, intent_name, intent_arguments_json,
			preferred_delivery, fallback_delivery, current_step, current_step_status, risk_level,
			started_at, updated_at, finished_at, snapshot_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, record.TaskID, record.SessionID, record.RunID, record.Title, record.SourceType, record.Status, record.IntentName, record.IntentArgumentsJSON, record.PreferredDelivery, record.FallbackDelivery, record.CurrentStep, record.CurrentStepStatus, record.RiskLevel, record.StartedAt, record.UpdatedAt, nullableText(record.FinishedAt), record.SnapshotJSON)
	if err != nil {
		return fmt.Errorf("write task: %w", err)
	}
	return nil
}

func (s *SQLiteTaskStore) DeleteTask(ctx context.Context, taskID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM tasks WHERE task_id = ?`, taskID)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}

func (s *SQLiteTaskStore) GetTask(ctx context.Context, taskID string) (TaskRecord, error) {
	var record TaskRecord
	err := s.db.QueryRowContext(ctx, `
		SELECT task_id, session_id, run_id, title, source_type, status, intent_name, intent_arguments_json,
		       preferred_delivery, fallback_delivery, current_step, current_step_status, risk_level,
		       started_at, updated_at, COALESCE(finished_at, ''), snapshot_json
		FROM tasks WHERE task_id = ?
	`, taskID).Scan(&record.TaskID, &record.SessionID, &record.RunID, &record.Title, &record.SourceType, &record.Status, &record.IntentName, &record.IntentArgumentsJSON, &record.PreferredDelivery, &record.FallbackDelivery, &record.CurrentStep, &record.CurrentStepStatus, &record.RiskLevel, &record.StartedAt, &record.UpdatedAt, &record.FinishedAt, &record.SnapshotJSON)
	if err != nil {
		return TaskRecord{}, err
	}
	return record, nil
}

func (s *SQLiteTaskStore) ListTasks(ctx context.Context, limit, offset int) ([]TaskRecord, int, error) {
	query := `SELECT task_id, session_id, run_id, title, source_type, status, intent_name, intent_arguments_json, preferred_delivery, fallback_delivery, current_step, current_step_status, risk_level, started_at, updated_at, COALESCE(finished_at, ''), snapshot_json FROM tasks ORDER BY started_at DESC, task_id DESC`
	countQuery := `SELECT COUNT(1) FROM tasks`
	args := make([]any, 0, 2)
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()
	items := make([]TaskRecord, 0)
	for rows.Next() {
		var record TaskRecord
		if err := rows.Scan(&record.TaskID, &record.SessionID, &record.RunID, &record.Title, &record.SourceType, &record.Status, &record.IntentName, &record.IntentArgumentsJSON, &record.PreferredDelivery, &record.FallbackDelivery, &record.CurrentStep, &record.CurrentStepStatus, &record.RiskLevel, &record.StartedAt, &record.UpdatedAt, &record.FinishedAt, &record.SnapshotJSON); err != nil {
			return nil, 0, fmt.Errorf("scan task: %w", err)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate tasks: %w", err)
	}
	return items, total, nil
}

func (s *SQLiteTaskStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteTaskStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=5000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS tasks (
			task_id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			run_id TEXT NOT NULL UNIQUE,
			title TEXT NOT NULL,
			source_type TEXT NOT NULL,
			status TEXT NOT NULL,
			intent_name TEXT NOT NULL,
			intent_arguments_json TEXT NOT NULL,
			preferred_delivery TEXT NOT NULL,
			fallback_delivery TEXT NOT NULL,
			current_step TEXT NOT NULL,
			current_step_status TEXT NOT NULL,
			risk_level TEXT NOT NULL,
			started_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			finished_at TEXT,
			snapshot_json TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create tasks table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_tasks_started_at ON tasks(started_at DESC, task_id DESC);`); err != nil {
		return fmt.Errorf("create tasks started_at index: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_tasks_session_id ON tasks(session_id);`); err != nil {
		return fmt.Errorf("create tasks session index: %w", err)
	}
	return nil
}

type SQLiteTaskStepStore struct {
	db *sql.DB
}

func NewSQLiteTaskStepStore(databasePath string) (*SQLiteTaskStepStore, error) {
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteTaskStepStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteTaskStepStore) ReplaceTaskSteps(ctx context.Context, taskID string, records []TaskStepRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin task step replace transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM task_steps WHERE task_id = ?`, taskID); err != nil {
		return fmt.Errorf("delete task steps: %w", err)
	}
	for _, record := range records {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO task_steps (step_id, task_id, name, status, order_index, input_summary, output_summary, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, record.StepID, record.TaskID, record.Name, record.Status, record.OrderIndex, record.InputSummary, record.OutputSummary, record.CreatedAt, record.UpdatedAt); err != nil {
			return fmt.Errorf("insert task step: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit task step replace transaction: %w", err)
	}
	return nil
}

func (s *SQLiteTaskStepStore) ListTaskSteps(ctx context.Context, taskID string, limit, offset int) ([]TaskStepRecord, int, error) {
	query := `SELECT step_id, task_id, name, status, order_index, input_summary, output_summary, created_at, updated_at FROM task_steps WHERE task_id = ? ORDER BY order_index ASC, step_id ASC`
	countQuery := `SELECT COUNT(1) FROM task_steps WHERE task_id = ?`
	args := []any{taskID}
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, taskID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count task steps: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list task steps: %w", err)
	}
	defer rows.Close()
	items := make([]TaskStepRecord, 0)
	for rows.Next() {
		var record TaskStepRecord
		if err := rows.Scan(&record.StepID, &record.TaskID, &record.Name, &record.Status, &record.OrderIndex, &record.InputSummary, &record.OutputSummary, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan task step: %w", err)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate task steps: %w", err)
	}
	return items, total, nil
}

func (s *SQLiteTaskStepStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteTaskStepStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=5000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS task_steps (
			step_id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			name TEXT NOT NULL,
			status TEXT NOT NULL,
			order_index INTEGER NOT NULL,
			input_summary TEXT NOT NULL,
			output_summary TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create task_steps table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_task_steps_task_order ON task_steps(task_id, order_index ASC);`); err != nil {
		return fmt.Errorf("create task_steps task_order index: %w", err)
	}
	return nil
}

func pageTasks(items []TaskRecord, limit, offset int) []TaskRecord {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return nil
	}
	if limit <= 0 {
		return append([]TaskRecord(nil), items[offset:]...)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return append([]TaskRecord(nil), items[offset:end]...)
}

func pageTaskSteps(items []TaskStepRecord, limit, offset int) []TaskStepRecord {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return nil
	}
	if limit <= 0 {
		return append([]TaskStepRecord(nil), items[offset:]...)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return append([]TaskStepRecord(nil), items[offset:end]...)
}

func nullableText(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func IsTaskRecordNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
