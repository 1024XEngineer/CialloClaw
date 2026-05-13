package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

const structuredTaskSelectColumns = `
	task_id,
	session_id,
	run_id,
	COALESCE(primary_run_id, run_id, ''),
	title,
	source_type,
	status,
	intent_name,
	intent_arguments_json,
	preferred_delivery,
	fallback_delivery,
	current_step,
	current_step_status,
	risk_level,
	COALESCE(request_source, ''),
	COALESCE(request_trigger, ''),
	started_at,
	updated_at,
	COALESCE(finished_at, ''),
	snapshot_json`

func normalizedPrimaryRunID(record TaskRecord) string {
	if strings.TrimSpace(record.PrimaryRunID) != "" {
		return record.PrimaryRunID
	}
	return strings.TrimSpace(record.RunID)
}

func scanStructuredTask(scanner interface {
	Scan(dest ...any) error
}, record *TaskRecord) error {
	if record == nil {
		return fmt.Errorf("scan structured task: nil record")
	}
	return scanner.Scan(
		&record.TaskID,
		&record.SessionID,
		&record.RunID,
		&record.PrimaryRunID,
		&record.Title,
		&record.SourceType,
		&record.Status,
		&record.IntentName,
		&record.IntentArgumentsJSON,
		&record.PreferredDelivery,
		&record.FallbackDelivery,
		&record.CurrentStep,
		&record.CurrentStepStatus,
		&record.RiskLevel,
		&record.RequestSource,
		&record.RequestTrigger,
		&record.StartedAt,
		&record.UpdatedAt,
		&record.FinishedAt,
		&record.SnapshotJSON,
	)
}

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
	record.PrimaryRunID = normalizedPrimaryRunID(record)
	if strings.TrimSpace(record.RunID) == "" {
		record.RunID = record.PrimaryRunID
	}
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

func (s *inMemoryTaskStore) ListTasksByIDs(_ context.Context, taskIDs []string) ([]TaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]TaskRecord, 0, len(taskIDs))
	seen := make(map[string]struct{}, len(taskIDs))
	for _, taskID := range taskIDs {
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			continue
		}
		if _, duplicate := seen[taskID]; duplicate {
			continue
		}
		record, ok := s.records[taskID]
		if !ok {
			continue
		}
		items = append(items, record)
		seen[taskID] = struct{}{}
	}
	return items, nil
}

func (s *inMemoryTaskStore) ListTasksForTaskList(_ context.Context, statusGroup, sortBy, sortOrder string, limit, offset int) ([]TaskRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]TaskRecord, 0, len(s.records))
	for _, record := range s.records {
		if !taskRecordMatchesStatusGroup(record.Status, statusGroup) {
			continue
		}
		items = append(items, record)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return compareTaskRecordsForList(items[i], items[j], sortBy, sortOrder)
	})
	return pageTasks(items, limit, offset), len(items), nil
}

func (s *inMemoryTaskStore) ListTasksBySession(_ context.Context, sessionID string, limit, offset int) ([]TaskRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]TaskRecord, 0)
	for _, record := range s.records {
		if record.SessionID == sessionID {
			items = append(items, record)
		}
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
	primaryRunID := normalizedPrimaryRunID(record)
	runID := strings.TrimSpace(record.RunID)
	if runID == "" {
		runID = primaryRunID
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO tasks (
			task_id, session_id, run_id, primary_run_id, title, source_type, status, intent_name, intent_arguments_json,
			preferred_delivery, fallback_delivery, current_step, current_step_status, risk_level, request_source, request_trigger,
			started_at, updated_at, finished_at, snapshot_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, record.TaskID, record.SessionID, runID, primaryRunID, record.Title, record.SourceType, record.Status, record.IntentName, record.IntentArgumentsJSON, record.PreferredDelivery, record.FallbackDelivery, record.CurrentStep, record.CurrentStepStatus, record.RiskLevel, nullableText(record.RequestSource), nullableText(record.RequestTrigger), record.StartedAt, record.UpdatedAt, nullableText(record.FinishedAt), record.SnapshotJSON)
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
	err := scanStructuredTask(s.db.QueryRowContext(ctx, `
		SELECT `+structuredTaskSelectColumns+`
		FROM tasks WHERE task_id = ?
	`, taskID), &record)
	if err != nil {
		return TaskRecord{}, err
	}
	return record, nil
}

func (s *SQLiteTaskStore) ListTasks(ctx context.Context, limit, offset int) ([]TaskRecord, int, error) {
	query := `SELECT ` + structuredTaskSelectColumns + ` FROM tasks ORDER BY started_at DESC, task_id DESC`
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
		if err := scanStructuredTask(rows, &record); err != nil {
			return nil, 0, fmt.Errorf("scan task: %w", err)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate tasks: %w", err)
	}
	return items, total, nil
}

func (s *SQLiteTaskStore) ListTasksByIDs(ctx context.Context, taskIDs []string) ([]TaskRecord, error) {
	filteredTaskIDs := uniqueNonEmptyTaskIDs(taskIDs)
	if len(filteredTaskIDs) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(filteredTaskIDs))
	for _, taskID := range filteredTaskIDs {
		args = append(args, taskID)
	}
	query := `SELECT ` + structuredTaskSelectColumns + ` FROM tasks WHERE task_id IN (` + sqlitePlaceholders(len(filteredTaskIDs)) + `)`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks by ids: %w", err)
	}
	defer rows.Close()
	items := make([]TaskRecord, 0, len(filteredTaskIDs))
	for rows.Next() {
		var record TaskRecord
		if err := scanStructuredTask(rows, &record); err != nil {
			return nil, fmt.Errorf("scan task by ids: %w", err)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks by ids: %w", err)
	}
	return items, nil
}

func (s *SQLiteTaskStore) ListTasksForTaskList(ctx context.Context, statusGroup, sortBy, sortOrder string, limit, offset int) ([]TaskRecord, int, error) {
	whereClause, whereArgs := sqliteTaskListStatusClause(statusGroup)
	orderClause := sqliteTaskListOrderClause(sortBy, sortOrder)
	query := `SELECT ` + structuredTaskSelectColumns + ` FROM tasks` + whereClause + ` ORDER BY ` + orderClause
	countQuery := `SELECT COUNT(1) FROM tasks` + whereClause
	args := append([]any{}, whereArgs...)
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count task-list rows: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list task-list rows: %w", err)
	}
	defer rows.Close()
	items := make([]TaskRecord, 0)
	for rows.Next() {
		var record TaskRecord
		if err := scanStructuredTask(rows, &record); err != nil {
			return nil, 0, fmt.Errorf("scan task-list row: %w", err)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate task-list rows: %w", err)
	}
	return items, total, nil
}

func (s *SQLiteTaskStore) ListTasksBySession(ctx context.Context, sessionID string, limit, offset int) ([]TaskRecord, int, error) {
	query := `SELECT ` + structuredTaskSelectColumns + ` FROM tasks WHERE session_id = ? ORDER BY started_at DESC, task_id DESC`
	countQuery := `SELECT COUNT(1) FROM tasks WHERE session_id = ?`
	args := []any{sessionID}
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, sessionID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tasks by session: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tasks by session: %w", err)
	}
	defer rows.Close()
	items := make([]TaskRecord, 0)
	for rows.Next() {
		var record TaskRecord
		if err := scanStructuredTask(rows, &record); err != nil {
			return nil, 0, fmt.Errorf("scan task by session: %w", err)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate tasks by session: %w", err)
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
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=300000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS tasks (
			task_id TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			run_id TEXT NOT NULL UNIQUE,
			primary_run_id TEXT,
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
			request_source TEXT,
			request_trigger TEXT,
			started_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			finished_at TEXT,
			snapshot_json TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create tasks table: %w", err)
	}
	statements := []string{
		`ALTER TABLE tasks ADD COLUMN primary_run_id TEXT;`,
		`ALTER TABLE tasks ADD COLUMN request_source TEXT;`,
		`ALTER TABLE tasks ADD COLUMN request_trigger TEXT;`,
		`UPDATE tasks SET primary_run_id = run_id WHERE COALESCE(primary_run_id, '') = '';`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			if isSQLiteDuplicateColumnError(err) {
				continue
			}
			return fmt.Errorf("initialize tasks table: %w", err)
		}
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_tasks_started_at ON tasks(started_at DESC, task_id DESC);`); err != nil {
		return fmt.Errorf("create tasks started_at index: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_tasks_session_id ON tasks(session_id);`); err != nil {
		return fmt.Errorf("create tasks session index: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_tasks_primary_run_id ON tasks(primary_run_id);`); err != nil {
		return fmt.Errorf("create tasks primary run index: %w", err)
	}
	return nil
}

func sqliteTaskListStatusClause(statusGroup string) (string, []any) {
	finishedStatuses := []any{"completed", "cancelled", "ended_unfinished", "failed"}
	switch strings.TrimSpace(statusGroup) {
	case "finished":
		return ` WHERE status IN (?, ?, ?, ?)`, finishedStatuses
	default:
		return ` WHERE status NOT IN (?, ?, ?, ?)`, finishedStatuses
	}
}

func sqliteTaskListOrderClause(sortBy, sortOrder string) string {
	column := "updated_at"
	switch strings.TrimSpace(sortBy) {
	case "started_at":
		column = "started_at"
	case "finished_at":
		column = "finished_at"
	}
	direction := "DESC"
	if strings.TrimSpace(sortOrder) == "asc" {
		direction = "ASC"
	}
	return column + ` ` + direction + `, updated_at ` + direction + `, task_id ` + direction
}

func taskRecordMatchesStatusGroup(status, statusGroup string) bool {
	switch strings.TrimSpace(statusGroup) {
	case "finished":
		return status == "completed" || status == "cancelled" || status == "ended_unfinished" || status == "failed"
	default:
		return !(status == "completed" || status == "cancelled" || status == "ended_unfinished" || status == "failed")
	}
}

func compareTaskRecordsForList(left, right TaskRecord, sortBy, sortOrder string) bool {
	leftTime := parseGovernanceTime(taskRecordListSortTime(left, sortBy))
	rightTime := parseGovernanceTime(taskRecordListSortTime(right, sortBy))
	ascending := strings.TrimSpace(sortOrder) == "asc"
	if leftTime.Equal(rightTime) {
		leftUpdated := parseGovernanceTime(left.UpdatedAt)
		rightUpdated := parseGovernanceTime(right.UpdatedAt)
		if leftUpdated.Equal(rightUpdated) {
			if ascending {
				return left.TaskID < right.TaskID
			}
			return left.TaskID > right.TaskID
		}
		if ascending {
			return leftUpdated.Before(rightUpdated)
		}
		return leftUpdated.After(rightUpdated)
	}
	if ascending {
		return leftTime.Before(rightTime)
	}
	return leftTime.After(rightTime)
}

func taskRecordListSortTime(record TaskRecord, sortBy string) string {
	switch strings.TrimSpace(sortBy) {
	case "started_at":
		return record.StartedAt
	case "finished_at":
		return record.FinishedAt
	default:
		return record.UpdatedAt
	}
}

func uniqueNonEmptyTaskIDs(taskIDs []string) []string {
	filtered := make([]string, 0, len(taskIDs))
	seen := make(map[string]struct{}, len(taskIDs))
	for _, taskID := range taskIDs {
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			continue
		}
		if _, duplicate := seen[taskID]; duplicate {
			continue
		}
		seen[taskID] = struct{}{}
		filtered = append(filtered, taskID)
	}
	return filtered
}

func sqlitePlaceholders(count int) string {
	if count <= 0 {
		return ""
	}
	return strings.TrimRight(strings.Repeat("?,", count), ",")
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
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=300000;`); err != nil {
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
