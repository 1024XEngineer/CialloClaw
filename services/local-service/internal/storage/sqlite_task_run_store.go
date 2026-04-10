// 该文件负责 task/run 主状态在 SQLite 中的持久化实现。
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

	_ "modernc.org/sqlite"
)

const sqliteTaskRunTableName = "task_runs"
const sqliteEngineSequenceTableName = "engine_sequences"

var (
	// ErrTaskRunTaskIDRequired 表示缺少 task_id。
	ErrTaskRunTaskIDRequired = errors.New("storage task_run task_id is required")
	// ErrTaskRunSessionIDRequired 表示缺少 session_id。
	ErrTaskRunSessionIDRequired = errors.New("storage task_run session_id is required")
	// ErrTaskRunRunIDRequired 表示缺少 run_id。
	ErrTaskRunRunIDRequired = errors.New("storage task_run run_id is required")
	// ErrTaskRunStatusRequired 表示缺少 status。
	ErrTaskRunStatusRequired = errors.New("storage task_run status is required")
	// ErrTaskRunStartedAtRequired 表示缺少 started_at。
	ErrTaskRunStartedAtRequired = errors.New("storage task_run started_at is required")
	// ErrTaskRunUpdatedAtRequired 表示缺少 updated_at。
	ErrTaskRunUpdatedAtRequired = errors.New("storage task_run updated_at is required")
	// ErrTaskRunIdentifierPrefixRequired 表示缺少分配标识符所需的 prefix。
	ErrTaskRunIdentifierPrefixRequired = errors.New("storage task_run identifier prefix is required")
)

// SQLiteTaskRunStore 提供 task/run 主状态的 SQLite 持久化能力。
type SQLiteTaskRunStore struct {
	db *sql.DB
}

// NewSQLiteTaskRunStore 创建并返回 SQLiteTaskRunStore。
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

	return store, nil
}

// AllocateIdentifier 为给定前缀分配一个稳定递增的标识符。
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

// SaveTaskRun 保存或覆盖一条 task/run 主状态快照。
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

	if _, err := s.db.ExecContext(
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

	return nil
}

// LoadTaskRuns 加载所有 task/run 主状态快照。
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

// Close 关闭底层 SQLite 连接。
func (s *SQLiteTaskRunStore) Close() error {
	if s.db == nil {
		return nil
	}

	return s.db.Close()
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
	clone.MirrorReferences = cloneMapSlice(record.MirrorReferences)
	clone.SecuritySummary = cloneMap(record.SecuritySummary)
	clone.ApprovalRequest = cloneMap(record.ApprovalRequest)
	clone.PendingExecution = cloneMap(record.PendingExecution)
	clone.Authorization = cloneMap(record.Authorization)
	clone.ImpactScope = cloneMap(record.ImpactScope)
	clone.MemoryReadPlans = cloneMapSlice(record.MemoryReadPlans)
	clone.MemoryWritePlans = cloneMapSlice(record.MemoryWritePlans)
	clone.StorageWritePlan = cloneMap(record.StorageWritePlan)
	clone.ArtifactPlans = cloneMapSlice(record.ArtifactPlans)
	clone.Notifications = cloneNotificationSnapshots(record.Notifications)
	clone.LatestEvent = cloneMap(record.LatestEvent)
	clone.LatestToolCall = cloneMap(record.LatestToolCall)
	if record.FinishedAt != nil {
		finishedAt := *record.FinishedAt
		clone.FinishedAt = &finishedAt
	}
	return clone
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
