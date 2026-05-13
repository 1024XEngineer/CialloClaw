package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type inMemoryArtifactStore struct {
	mu      sync.Mutex
	records []ArtifactRecord
}

func newInMemoryArtifactStore() *inMemoryArtifactStore {
	return &inMemoryArtifactStore{records: make([]ArtifactRecord, 0)}
}

func (s *inMemoryArtifactStore) SaveArtifacts(_ context.Context, records []ArtifactRecord) error {
	if len(records) == 0 {
		return nil
	}
	for _, record := range records {
		if err := validateArtifactRecord(record); err != nil {
			return err
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, record := range records {
		replaced := false
		for index, existing := range s.records {
			if existing.ArtifactID != record.ArtifactID {
				continue
			}
			s.records[index] = record
			replaced = true
			break
		}
		if !replaced {
			s.records = append(s.records, record)
		}
	}
	return nil
}

func (s *inMemoryArtifactStore) ListArtifacts(_ context.Context, taskID, runID string, limit, offset int) ([]ArtifactRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]ArtifactRecord, 0, len(s.records))
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
		left := parseGovernanceTime(items[i].CreatedAt)
		right := parseGovernanceTime(items[j].CreatedAt)
		if left.Equal(right) {
			return items[i].ArtifactID > items[j].ArtifactID
		}
		return left.After(right)
	})
	return pageArtifactRecords(items, limit, offset), len(items), nil
}

// SQLiteArtifactStore persists artifacts in SQLite WAL storage.
type SQLiteArtifactStore struct {
	db *sql.DB
}

// NewSQLiteArtifactStore creates a SQLite-backed artifact store.
func NewSQLiteArtifactStore(databasePath string) (*SQLiteArtifactStore, error) {
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteArtifactStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// SaveArtifacts persists artifact snapshots.
func (s *SQLiteArtifactStore) SaveArtifacts(ctx context.Context, records []ArtifactRecord) error {
	if len(records) == 0 {
		return nil
	}
	for _, record := range records {
		if err := validateArtifactRecord(record); err != nil {
			return err
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin artifact transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, record := range records {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT OR REPLACE INTO artifacts (artifact_id, task_id, run_id, artifact_type, title, path, mime_type, delivery_type, delivery_payload_json, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			record.ArtifactID,
			record.TaskID,
			nullableRuntimeString(record.RunID),
			record.ArtifactType,
			record.Title,
			record.Path,
			record.MimeType,
			record.DeliveryType,
			record.DeliveryPayloadJSON,
			record.CreatedAt,
		); err != nil {
			return fmt.Errorf("save artifact %s: %w", record.ArtifactID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit artifact transaction: %w", err)
	}
	return nil
}

// ListArtifacts returns persisted artifacts for one task.
func (s *SQLiteArtifactStore) ListArtifacts(ctx context.Context, taskID, runID string, limit, offset int) ([]ArtifactRecord, int, error) {
	countQuery := `SELECT COUNT(1) FROM artifacts`
	query := `SELECT artifact_id, task_id, COALESCE(run_id, ''), artifact_type, title, path, mime_type, delivery_type, delivery_payload_json, created_at FROM artifacts`
	filters := make([]string, 0, 2)
	filterArgs := make([]any, 0, 2)
	if taskID != "" {
		filters = append(filters, `task_id = ?`)
		filterArgs = append(filterArgs, taskID)
	}
	if runID != "" {
		filters = append(filters, `run_id = ?`)
		filterArgs = append(filterArgs, runID)
	}
	if len(filters) > 0 {
		whereClause := ` WHERE ` + strings.Join(filters, ` AND `)
		countQuery += whereClause
		query += whereClause
	}
	query += ` ORDER BY created_at DESC, artifact_id DESC`
	args := append([]any(nil), filterArgs...)
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, filterArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count artifacts: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list artifacts: %w", err)
	}
	defer rows.Close()
	items := make([]ArtifactRecord, 0)
	for rows.Next() {
		var record ArtifactRecord
		if err := rows.Scan(&record.ArtifactID, &record.TaskID, &record.RunID, &record.ArtifactType, &record.Title, &record.Path, &record.MimeType, &record.DeliveryType, &record.DeliveryPayloadJSON, &record.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan artifact record: %w", err)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate artifact records: %w", err)
	}
	return items, total, nil
}

// Close closes the artifact database handle.
func (s *SQLiteArtifactStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteArtifactStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=300000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS artifacts (
			artifact_id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			run_id TEXT,
			artifact_type TEXT NOT NULL,
			title TEXT NOT NULL,
			path TEXT NOT NULL,
			mime_type TEXT NOT NULL,
			delivery_type TEXT NOT NULL,
			delivery_payload_json TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create artifacts table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE artifacts ADD COLUMN run_id TEXT;`); err != nil && !isSQLiteDuplicateColumnError(err) {
		return fmt.Errorf("add artifacts run_id column: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_artifacts_task_id ON artifacts(task_id, created_at DESC, artifact_id DESC);`); err != nil {
		return fmt.Errorf("create artifacts task index: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_artifacts_task_run_time ON artifacts(task_id, run_id, created_at DESC, artifact_id DESC);`); err != nil {
		return fmt.Errorf("create artifacts task run index: %w", err)
	}
	return nil
}

func validateArtifactRecord(record ArtifactRecord) error {
	if record.ArtifactID == "" {
		return fmt.Errorf("storage artifact artifact_id is required")
	}
	if record.TaskID == "" {
		return fmt.Errorf("storage artifact task_id is required")
	}
	if record.ArtifactType == "" {
		return fmt.Errorf("storage artifact artifact_type is required")
	}
	if record.Title == "" {
		return fmt.Errorf("storage artifact title is required")
	}
	if record.Path == "" {
		return fmt.Errorf("storage artifact path is required")
	}
	if record.MimeType == "" {
		return fmt.Errorf("storage artifact mime_type is required")
	}
	if record.DeliveryType == "" {
		return fmt.Errorf("storage artifact delivery_type is required")
	}
	if record.DeliveryPayloadJSON == "" {
		return fmt.Errorf("storage artifact delivery_payload_json is required")
	}
	if record.CreatedAt == "" {
		return fmt.Errorf("storage artifact created_at is required")
	}
	if _, err := time.Parse(time.RFC3339, record.CreatedAt); err != nil {
		if _, nanoErr := time.Parse(time.RFC3339Nano, record.CreatedAt); nanoErr != nil {
			return fmt.Errorf("storage artifact created_at must be rfc3339")
		}
	}
	if !json.Valid([]byte(record.DeliveryPayloadJSON)) {
		return fmt.Errorf("storage artifact delivery_payload_json must be valid json")
	}
	return nil
}

func pageArtifactRecords(items []ArtifactRecord, limit, offset int) []ArtifactRecord {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return nil
	}
	if limit <= 0 {
		limit = 20
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return append([]ArtifactRecord(nil), items[offset:end]...)
}
