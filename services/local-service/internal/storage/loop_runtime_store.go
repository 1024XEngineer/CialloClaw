package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// RunRecord persists one normalized run snapshot alongside the task snapshot.
type RunRecord struct {
	RunID      string
	TaskID     string
	SessionID  string
	Status     string
	IntentName string
	StartedAt  string
	UpdatedAt  string
	FinishedAt string
	StopReason string
}

// StepRecord persists one normalized loop step snapshot.
type StepRecord struct {
	StepID        string
	RunID         string
	TaskID        string
	OrderIndex    int
	AttemptIndex  int
	SegmentKind   string
	LoopRound     int
	Name          string
	Status        string
	InputSummary  string
	OutputSummary string
	StopReason    string
	StartedAt     string
	CompletedAt   string
	PlannerInput  string
	PlannerOutput string
	Observation   string
	ToolName      string
	ToolCallID    string
}

// EventRecord persists one normalized compatibility event.
type EventRecord struct {
	EventID     string
	RunID       string
	TaskID      string
	StepID      string
	Type        string
	Level       string
	PayloadJSON string
	CreatedAt   string
}

// DeliveryResultRecord persists one delivery_result snapshot outside task_runs.
type DeliveryResultRecord struct {
	DeliveryResultID string
	TaskID           string
	Type             string
	Title            string
	PayloadJSON      string
	PreviewText      string
	CreatedAt        string
}

type inMemoryLoopRuntimeStore struct {
	mu              sync.Mutex
	runs            map[string]RunRecord
	steps           map[string]StepRecord
	events          []EventRecord
	deliveryResults map[string]DeliveryResultRecord
}

func newInMemoryLoopRuntimeStore() *inMemoryLoopRuntimeStore {
	return &inMemoryLoopRuntimeStore{
		runs:            map[string]RunRecord{},
		steps:           map[string]StepRecord{},
		events:          []EventRecord{},
		deliveryResults: map[string]DeliveryResultRecord{},
	}
}

func (s *inMemoryLoopRuntimeStore) SaveRun(_ context.Context, record RunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[record.RunID] = record
	return nil
}

func (s *inMemoryLoopRuntimeStore) SaveSteps(_ context.Context, records []StepRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, record := range records {
		s.steps[record.StepID] = record
	}
	return nil
}

func (s *inMemoryLoopRuntimeStore) SaveEvents(_ context.Context, records []EventRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, records...)
	return nil
}

func (s *inMemoryLoopRuntimeStore) SaveDeliveryResult(_ context.Context, record DeliveryResultRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliveryResults[record.DeliveryResultID] = record
	return nil
}

func (s *inMemoryLoopRuntimeStore) ListEvents(_ context.Context, taskID, runID, eventType, createdAtFrom, createdAtTo string, limit, offset int) ([]EventRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fromTime := parseGovernanceTime(createdAtFrom)
	toTime := parseGovernanceTime(createdAtTo)
	filtered := make([]EventRecord, 0, len(s.events))
	for _, record := range s.events {
		if taskID != "" && record.TaskID != taskID {
			continue
		}
		if runID != "" && record.RunID != runID {
			continue
		}
		if eventType != "" && record.Type != eventType {
			continue
		}
		recordTime := parseGovernanceTime(record.CreatedAt)
		if !fromTime.IsZero() && recordTime.Before(fromTime) {
			continue
		}
		if !toTime.IsZero() && recordTime.After(toTime) {
			continue
		}
		filtered = append(filtered, record)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return parseGovernanceTime(filtered[i].CreatedAt).After(parseGovernanceTime(filtered[j].CreatedAt))
	})
	if offset >= len(filtered) {
		return []EventRecord{}, len(filtered), nil
	}
	end := offset + limit
	if limit <= 0 || end > len(filtered) {
		end = len(filtered)
	}
	return append([]EventRecord(nil), filtered[offset:end]...), len(filtered), nil
}

type SQLiteLoopRuntimeStore struct {
	db *sql.DB
}

func NewSQLiteLoopRuntimeStore(databasePath string) (*SQLiteLoopRuntimeStore, error) {
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteLoopRuntimeStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteLoopRuntimeStore) SaveRun(ctx context.Context, record RunRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO runs (run_id, task_id, session_id, status, intent_name, started_at, updated_at, finished_at, stop_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, record.RunID, record.TaskID, record.SessionID, record.Status, record.IntentName, record.StartedAt, record.UpdatedAt, nullableRuntimeString(record.FinishedAt), nullableRuntimeString(record.StopReason))
	if err != nil {
		return fmt.Errorf("write run record: %w", err)
	}
	return nil
}

func (s *SQLiteLoopRuntimeStore) SaveSteps(ctx context.Context, records []StepRecord) error {
	for _, record := range records {
		_, err := s.db.ExecContext(ctx, `
			INSERT OR REPLACE INTO steps (step_id, run_id, task_id, order_index, attempt_index, segment_kind, loop_round, name, status, input_summary, output_summary, stop_reason, started_at, completed_at, planner_input, planner_output, observation, tool_name, tool_call_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, record.StepID, record.RunID, record.TaskID, record.OrderIndex, record.AttemptIndex, nullableRuntimeString(record.SegmentKind), record.LoopRound, record.Name, record.Status, record.InputSummary, record.OutputSummary, nullableRuntimeString(record.StopReason), record.StartedAt, nullableRuntimeString(record.CompletedAt), record.PlannerInput, record.PlannerOutput, record.Observation, nullableRuntimeString(record.ToolName), nullableRuntimeString(record.ToolCallID))
		if err != nil {
			return fmt.Errorf("write step record %s: %w", record.StepID, err)
		}
	}
	return nil
}

func (s *SQLiteLoopRuntimeStore) SaveEvents(ctx context.Context, records []EventRecord) error {
	for _, record := range records {
		_, err := s.db.ExecContext(ctx, `
			INSERT OR REPLACE INTO events (event_id, run_id, task_id, step_id, type, level, payload_json, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, record.EventID, record.RunID, record.TaskID, nullableRuntimeString(record.StepID), record.Type, record.Level, record.PayloadJSON, record.CreatedAt)
		if err != nil {
			return fmt.Errorf("write event record %s: %w", record.EventID, err)
		}
	}
	return nil
}

func (s *SQLiteLoopRuntimeStore) SaveDeliveryResult(ctx context.Context, record DeliveryResultRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO delivery_results (delivery_result_id, task_id, type, title, payload_json, preview_text, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, record.DeliveryResultID, record.TaskID, record.Type, record.Title, record.PayloadJSON, nullableRuntimeString(record.PreviewText), record.CreatedAt)
	if err != nil {
		return fmt.Errorf("write delivery_result record: %w", err)
	}
	return nil
}

func (s *SQLiteLoopRuntimeStore) ListEvents(ctx context.Context, taskID, runID, eventType, createdAtFrom, createdAtTo string, limit, offset int) ([]EventRecord, int, error) {
	filters := make([]string, 0, 5)
	filterArgs := make([]any, 0, 5)
	if strings.TrimSpace(taskID) != "" {
		filters = append(filters, `task_id = ?`)
		filterArgs = append(filterArgs, taskID)
	}
	if strings.TrimSpace(runID) != "" {
		filters = append(filters, `run_id = ?`)
		filterArgs = append(filterArgs, runID)
	}
	if strings.TrimSpace(eventType) != "" {
		filters = append(filters, `type = ?`)
		filterArgs = append(filterArgs, eventType)
	}
	if strings.TrimSpace(createdAtFrom) != "" {
		filters = append(filters, `created_at >= ?`)
		filterArgs = append(filterArgs, createdAtFrom)
	}
	if strings.TrimSpace(createdAtTo) != "" {
		filters = append(filters, `created_at <= ?`)
		filterArgs = append(filterArgs, createdAtTo)
	}
	countQuery := `SELECT COUNT(1) FROM events`
	query := `SELECT event_id, run_id, task_id, step_id, type, level, payload_json, created_at FROM events`
	if len(filters) > 0 {
		whereClause := ` WHERE ` + strings.Join(filters, ` AND `)
		countQuery += whereClause
		query += whereClause
	}
	query += ` ORDER BY created_at DESC, event_id DESC`
	args := append([]any(nil), filterArgs...)
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, filterArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count events: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()
	items := make([]EventRecord, 0)
	for rows.Next() {
		var record EventRecord
		var stepID sql.NullString
		if err := rows.Scan(&record.EventID, &record.RunID, &record.TaskID, &stepID, &record.Type, &record.Level, &record.PayloadJSON, &record.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan event record: %w", err)
		}
		record.StepID = stepID.String
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate events: %w", err)
	}
	return items, total, nil
}

func (s *SQLiteLoopRuntimeStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=5000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS runs (run_id TEXT PRIMARY KEY, task_id TEXT NOT NULL, session_id TEXT NOT NULL, status TEXT NOT NULL, intent_name TEXT NOT NULL, started_at TEXT NOT NULL, updated_at TEXT NOT NULL, finished_at TEXT, stop_reason TEXT);`,
		`CREATE INDEX IF NOT EXISTS idx_runs_task_time ON runs(task_id, started_at DESC);`,
		`CREATE TABLE IF NOT EXISTS steps (step_id TEXT PRIMARY KEY, run_id TEXT NOT NULL, task_id TEXT NOT NULL, order_index INTEGER NOT NULL, attempt_index INTEGER NOT NULL DEFAULT 1, segment_kind TEXT NOT NULL DEFAULT 'initial', loop_round INTEGER NOT NULL DEFAULT 0, name TEXT NOT NULL, status TEXT NOT NULL, input_summary TEXT, output_summary TEXT, stop_reason TEXT, started_at TEXT NOT NULL, completed_at TEXT, planner_input TEXT, planner_output TEXT, observation TEXT, tool_name TEXT, tool_call_id TEXT);`,
		`ALTER TABLE steps ADD COLUMN attempt_index INTEGER NOT NULL DEFAULT 1;`,
		`ALTER TABLE steps ADD COLUMN segment_kind TEXT NOT NULL DEFAULT 'initial';`,
		`CREATE INDEX IF NOT EXISTS idx_steps_run_order ON steps(run_id, order_index);`,
		`CREATE TABLE IF NOT EXISTS events (event_id TEXT PRIMARY KEY, run_id TEXT NOT NULL, task_id TEXT NOT NULL, step_id TEXT, type TEXT NOT NULL, level TEXT NOT NULL, payload_json TEXT NOT NULL, created_at TEXT NOT NULL);`,
		`CREATE INDEX IF NOT EXISTS idx_events_task_time ON events(task_id, created_at DESC);`,
		`CREATE TABLE IF NOT EXISTS delivery_results (delivery_result_id TEXT PRIMARY KEY, task_id TEXT NOT NULL, type TEXT NOT NULL, title TEXT NOT NULL, payload_json TEXT NOT NULL, preview_text TEXT, created_at TEXT NOT NULL);`,
		`CREATE INDEX IF NOT EXISTS idx_delivery_results_task_time ON delivery_results(task_id, created_at DESC);`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			if isSQLiteDuplicateColumnError(err) {
				continue
			}
			return fmt.Errorf("initialize loop runtime store: %w", err)
		}
	}
	return nil
}

func isSQLiteDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate column name")
}

func (s *SQLiteLoopRuntimeStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func nullableRuntimeString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
