package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	_ "modernc.org/sqlite"
)

const sqliteTaskRunTableName = "task_runs"
const sqliteEngineSequenceTableName = "engine_sequences"

var (
	// ErrTaskRunTaskIDRequired reports that task_id is missing.
	ErrTaskRunTaskIDRequired = errors.New("storage task_run task_id is required")
	// ErrTaskRunSessionIDRequired reports that session_id is missing.
	ErrTaskRunSessionIDRequired = errors.New("storage task_run session_id is required")
	// ErrTaskRunRunIDRequired reports that run_id is missing.
	ErrTaskRunRunIDRequired = errors.New("storage task_run run_id is required")
	// ErrTaskRunStatusRequired reports that status is missing.
	ErrTaskRunStatusRequired = errors.New("storage task_run status is required")
	// ErrTaskRunStartedAtRequired reports that started_at is missing.
	ErrTaskRunStartedAtRequired = errors.New("storage task_run started_at is required")
	// ErrTaskRunUpdatedAtRequired reports that updated_at is missing.
	ErrTaskRunUpdatedAtRequired = errors.New("storage task_run updated_at is required")
	// ErrTaskRunIdentifierPrefixRequired reports that AllocateIdentifier received an empty prefix.
	ErrTaskRunIdentifierPrefixRequired = errors.New("storage task_run identifier prefix is required")
)

// SQLiteTaskRunStore persists task/run snapshots in SQLite.
type SQLiteTaskRunStore struct {
	db        *sql.DB
	taskStore TaskStore
	stepStore TaskStepStore
}

// NewSQLiteTaskRunStore opens and initializes the SQLite task/run store.
func NewSQLiteTaskRunStore(databasePath string) (*SQLiteTaskRunStore, error) {
	databasePath = strings.TrimSpace(databasePath)
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

	store := &SQLiteTaskRunStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	taskStore, err := NewSQLiteTaskStore(databasePath)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	stepStore, err := NewSQLiteTaskStepStore(databasePath)
	if err != nil {
		_ = taskStore.Close()
		_ = store.Close()
		return nil, err
	}
	store.taskStore = taskStore
	store.stepStore = stepStore

	return store, nil
}

// AllocateIdentifier reserves the next stable identifier for the given prefix.
func (s *SQLiteTaskRunStore) AllocateIdentifier(ctx context.Context, prefix string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "", ErrTaskRunIdentifierPrefixRequired
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin identifier allocation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO engine_sequences (prefix, current_value) VALUES (?, 0)`,
		prefix,
	); err != nil {
		return "", fmt.Errorf("insert engine sequence seed: %w", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE engine_sequences SET current_value = current_value + 1 WHERE prefix = ?`,
		prefix,
	); err != nil {
		return "", fmt.Errorf("update engine sequence: %w", err)
	}

	var currentValue uint64
	if err := tx.QueryRowContext(
		ctx,
		`SELECT current_value FROM engine_sequences WHERE prefix = ?`,
		prefix,
	).Scan(&currentValue); err != nil {
		return "", fmt.Errorf("query engine sequence: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("commit identifier allocation transaction: %w", err)
	}

	return fmt.Sprintf("%s_%03d", prefix, currentValue), nil
}

// DeleteTaskRun removes one persisted task/run snapshot from SQLite storage.
func (s *SQLiteTaskRunStore) DeleteTaskRun(ctx context.Context, taskID string) error {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return ErrTaskRunTaskIDRequired
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin task run delete transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := deleteStructuredTaskStateWithSQLiteTx(ctx, tx, taskID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM task_runs WHERE task_id = ?`, taskID); err != nil {
		return fmt.Errorf("delete task run %s: %w", taskID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit task run delete transaction: %w", err)
	}

	return nil
}

// SaveTaskRun saves or overwrites one task/run snapshot.
func (s *SQLiteTaskRunStore) SaveTaskRun(ctx context.Context, record TaskRunRecord) error {
	if err := validateTaskRunRecord(record); err != nil {
		return err
	}

	recordJSON, err := marshalTaskRunRecord(record)
	if err != nil {
		return err
	}

	var finishedAt any
	if record.FinishedAt != nil {
		finishedAt = record.FinishedAt.Format(time.RFC3339Nano)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin task run save transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO task_runs (
			task_id,
			run_id,
			session_id,
			status,
			started_at,
			updated_at,
			finished_at,
			record_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		record.TaskID,
		record.RunID,
		record.SessionID,
		record.Status,
		record.StartedAt.Format(time.RFC3339Nano),
		record.UpdatedAt.Format(time.RFC3339Nano),
		finishedAt,
		recordJSON,
	); err != nil {
		return fmt.Errorf("save task run %s: %w", record.TaskID, err)
	}
	if err := writeStructuredTaskStateWithSQLiteTx(ctx, tx, record); err != nil {
		return fmt.Errorf("save structured task state %s: %w", record.TaskID, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit task run save transaction: %w", err)
	}

	return nil
}

// LoadTaskRuns loads all task/run snapshots from SQLite storage.
func (s *SQLiteTaskRunStore) LoadTaskRuns(ctx context.Context) ([]TaskRunRecord, error) {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT record_json
		 FROM task_runs
		 ORDER BY started_at DESC, task_id DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("load task runs: %w", err)
	}
	defer rows.Close()

	records := make([]TaskRunRecord, 0)
	for rows.Next() {
		var recordJSON string
		if err := rows.Scan(&recordJSON); err != nil {
			return nil, fmt.Errorf("scan task run row: %w", err)
		}

		record, err := unmarshalTaskRunRecord(recordJSON)
		if err != nil {
			return nil, err
		}

		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task run rows: %w", err)
	}

	return records, nil
}

// GetTaskRun loads one task_run compatibility snapshot by task_id.
func (s *SQLiteTaskRunStore) GetTaskRun(ctx context.Context, taskID string) (TaskRunRecord, error) {
	var recordJSON string
	err := s.db.QueryRowContext(ctx, `SELECT record_json FROM task_runs WHERE task_id = ?`, taskID).Scan(&recordJSON)
	if err != nil {
		return TaskRunRecord{}, err
	}
	return unmarshalTaskRunRecord(recordJSON)
}

// LoadLegacyTaskRuns narrows compatibility fallback to task_runs rows that do
// not yet have a first-class task row counterpart.
func (s *SQLiteTaskRunStore) LoadLegacyTaskRuns(ctx context.Context, structuredTaskIDs []string) ([]TaskRunRecord, error) {
	query := `SELECT record_json FROM task_runs`
	args := make([]any, 0, len(structuredTaskIDs))
	filteredTaskIDs := make([]string, 0, len(structuredTaskIDs))
	for _, taskID := range structuredTaskIDs {
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			continue
		}
		filteredTaskIDs = append(filteredTaskIDs, taskID)
		args = append(args, taskID)
	}
	if len(filteredTaskIDs) > 0 {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(filteredTaskIDs)), ",")
		query += ` WHERE task_id NOT IN (` + placeholders + `)`
	}
	query += ` ORDER BY started_at DESC, task_id DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load legacy task runs: %w", err)
	}
	defer rows.Close()

	records := make([]TaskRunRecord, 0)
	for rows.Next() {
		var recordJSON string
		if err := rows.Scan(&recordJSON); err != nil {
			return nil, fmt.Errorf("scan legacy task run row: %w", err)
		}
		record, err := unmarshalTaskRunRecord(recordJSON)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate legacy task run rows: %w", err)
	}
	return records, nil
}

// Close closes the underlying SQLite connection.
func (s *SQLiteTaskRunStore) Close() error {
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	if closer, ok := s.taskStore.(interface{ Close() error }); ok {
		err = errors.Join(err, closer.Close())
	}
	if closer, ok := s.stepStore.(interface{ Close() error }); ok {
		err = errors.Join(err, closer.Close())
	}
	return err
}

func (s *SQLiteTaskRunStore) journalMode(ctx context.Context) (string, error) {
	var mode string
	if err := s.db.QueryRowContext(ctx, `PRAGMA journal_mode;`).Scan(&mode); err != nil {
		return "", fmt.Errorf("query sqlite journal mode: %w", err)
	}

	return strings.ToLower(strings.TrimSpace(mode)), nil
}

func (s *SQLiteTaskRunStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=5000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS task_runs (
			task_id TEXT PRIMARY KEY,
			run_id TEXT NOT NULL UNIQUE,
			session_id TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			finished_at TEXT,
			record_json TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create task_runs table: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS engine_sequences (
			prefix TEXT PRIMARY KEY,
			current_value INTEGER NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create engine_sequences table: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_task_runs_started_at ON task_runs(started_at DESC, task_id DESC);`); err != nil {
		return fmt.Errorf("create task_runs started_at index: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_task_runs_updated_at ON task_runs(updated_at DESC, task_id DESC);`); err != nil {
		return fmt.Errorf("create task_runs updated_at index: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_task_runs_session_id ON task_runs(session_id);`); err != nil {
		return fmt.Errorf("create task_runs session_id index: %w", err)
	}

	return nil
}

func marshalTaskRunRecord(record TaskRunRecord) (string, error) {
	payload, err := json.Marshal(cloneTaskRunRecord(record))
	if err != nil {
		return "", fmt.Errorf("marshal task run record %s: %w", record.TaskID, err)
	}

	return string(payload), nil
}

func unmarshalTaskRunRecord(payload string) (TaskRunRecord, error) {
	var record TaskRunRecord
	if err := json.Unmarshal([]byte(payload), &record); err != nil {
		return TaskRunRecord{}, fmt.Errorf("unmarshal task run record: %w", err)
	}
	if err := validateTaskRunRecord(record); err != nil {
		return TaskRunRecord{}, err
	}

	return record, nil
}

func validateTaskRunRecord(record TaskRunRecord) error {
	switch {
	case strings.TrimSpace(record.TaskID) == "":
		return ErrTaskRunTaskIDRequired
	case strings.TrimSpace(record.SessionID) == "":
		return ErrTaskRunSessionIDRequired
	case strings.TrimSpace(record.RunID) == "":
		return ErrTaskRunRunIDRequired
	case strings.TrimSpace(record.Status) == "":
		return ErrTaskRunStatusRequired
	case record.StartedAt.IsZero():
		return ErrTaskRunStartedAtRequired
	case record.UpdatedAt.IsZero():
		return ErrTaskRunUpdatedAtRequired
	default:
		return nil
	}
}

func cloneTaskRunRecord(record TaskRunRecord) TaskRunRecord {
	clone := record
	clone.Intent = cloneMap(record.Intent)
	clone.Timeline = cloneTaskStepSnapshots(record.Timeline)
	clone.BubbleMessage = cloneMap(record.BubbleMessage)
	clone.DeliveryResult = cloneMap(record.DeliveryResult)
	clone.Artifacts = cloneMapSlice(record.Artifacts)
	clone.Citations = cloneMapSlice(record.Citations)
	clone.AuditRecords = cloneMapSlice(record.AuditRecords)
	clone.MirrorReferences = cloneMapSlice(record.MirrorReferences)
	clone.Snapshot = cloneContextSnapshot(record.Snapshot)
	clone.SecuritySummary = cloneMap(record.SecuritySummary)
	clone.ApprovalRequest = cloneMap(record.ApprovalRequest)
	clone.PendingExecution = cloneMap(record.PendingExecution)
	clone.Authorization = cloneMap(record.Authorization)
	clone.ImpactScope = cloneMap(record.ImpactScope)
	clone.TokenUsage = cloneMap(record.TokenUsage)
	clone.MemoryReadPlans = cloneMapSlice(record.MemoryReadPlans)
	clone.MemoryWritePlans = cloneMapSlice(record.MemoryWritePlans)
	clone.StorageWritePlan = cloneMap(record.StorageWritePlan)
	clone.ArtifactPlans = cloneMapSlice(record.ArtifactPlans)
	clone.Notifications = cloneNotificationSnapshots(record.Notifications)
	clone.LatestEvent = cloneMap(record.LatestEvent)
	clone.LatestToolCall = cloneMap(record.LatestToolCall)
	clone.LoopStopReason = record.LoopStopReason
	clone.SteeringMessages = append([]string(nil), record.SteeringMessages...)
	if record.FinishedAt != nil {
		finishedAt := *record.FinishedAt
		clone.FinishedAt = &finishedAt
	}
	return clone
}

func writeStructuredTaskStateWithSQLiteTx(ctx context.Context, tx *sql.Tx, record TaskRunRecord) error {
	taskRecord, err := taskRecordFromSnapshot(record)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO tasks (
			task_id, session_id, run_id, primary_run_id, title, source_type, status, intent_name, intent_arguments_json,
			preferred_delivery, fallback_delivery, current_step, current_step_status, risk_level, request_source, request_trigger,
			started_at, updated_at, finished_at, snapshot_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, taskRecord.TaskID, taskRecord.SessionID, taskRecord.RunID, taskRecord.PrimaryRunID, taskRecord.Title, taskRecord.SourceType, taskRecord.Status, taskRecord.IntentName, taskRecord.IntentArgumentsJSON, taskRecord.PreferredDelivery, taskRecord.FallbackDelivery, taskRecord.CurrentStep, taskRecord.CurrentStepStatus, taskRecord.RiskLevel, nullableText(taskRecord.RequestSource), nullableText(taskRecord.RequestTrigger), taskRecord.StartedAt, taskRecord.UpdatedAt, nullableText(taskRecord.FinishedAt), taskRecord.SnapshotJSON); err != nil {
		return fmt.Errorf("write task: %w", err)
	}
	if err := deleteStructuredTaskStepsWithSQLiteTx(ctx, tx, record.TaskID); err != nil {
		return err
	}
	for _, step := range taskStepRecordsFromSnapshot(record) {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO task_steps (step_id, task_id, name, status, order_index, input_summary, output_summary, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, step.StepID, step.TaskID, step.Name, step.Status, step.OrderIndex, step.InputSummary, step.OutputSummary, step.CreatedAt, step.UpdatedAt); err != nil {
			return fmt.Errorf("insert task step: %w", err)
		}
	}
	return nil
}

func deleteStructuredTaskStateWithSQLiteTx(ctx context.Context, tx *sql.Tx, taskID string) error {
	if err := deleteStructuredTaskStepsWithSQLiteTx(ctx, tx, taskID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM tasks WHERE task_id = ?`, taskID); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}

func deleteStructuredTaskStepsWithSQLiteTx(ctx context.Context, tx *sql.Tx, taskID string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM task_steps WHERE task_id = ?`, taskID); err != nil {
		return fmt.Errorf("delete task steps: %w", err)
	}
	return nil
}

func cloneTaskStepSnapshots(values []TaskStepSnapshot) []TaskStepSnapshot {
	if len(values) == 0 {
		return nil
	}

	result := make([]TaskStepSnapshot, len(values))
	copy(result, values)
	return result
}

func cloneNotificationSnapshots(values []NotificationSnapshot) []NotificationSnapshot {
	if len(values) == 0 {
		return nil
	}

	result := make([]NotificationSnapshot, len(values))
	for index, value := range values {
		result[index] = NotificationSnapshot{
			Method:    value.Method,
			Params:    cloneMap(value.Params),
			CreatedAt: value.CreatedAt,
		}
	}

	return result
}

func cloneContextSnapshot(snapshot contextsvc.TaskContextSnapshot) contextsvc.TaskContextSnapshot {
	cloned := snapshot
	if len(snapshot.Files) > 0 {
		cloned.Files = append([]string(nil), snapshot.Files...)
	}
	return cloned
}

func cloneMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}

	result := make(map[string]any, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			result[key] = cloneMap(typed)
		case []map[string]any:
			result[key] = cloneMapSlice(typed)
		case []string:
			result[key] = append([]string(nil), typed...)
		default:
			result[key] = value
		}
	}
	return result
}

func cloneMapSlice(values []map[string]any) []map[string]any {
	if len(values) == 0 {
		return nil
	}

	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		result = append(result, cloneMap(value))
	}
	return result
}
