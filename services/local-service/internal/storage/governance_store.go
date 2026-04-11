package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
)

type inMemoryAuditStore struct {
	mu      sync.Mutex
	records []audit.Record
}

func newInMemoryAuditStore() *inMemoryAuditStore {
	return &inMemoryAuditStore{records: make([]audit.Record, 0)}
}

func (s *inMemoryAuditStore) WriteAuditRecord(_ context.Context, record audit.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, record)
	return nil
}

func (s *inMemoryAuditStore) ListAuditRecords(_ context.Context, taskID string, limit, offset int) ([]audit.Record, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]audit.Record, 0)
	for _, record := range s.records {
		if taskID == "" || record.TaskID == taskID {
			items = append(items, record)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return parseGovernanceTime(items[i].CreatedAt).After(parseGovernanceTime(items[j].CreatedAt))
	})
	return pageAuditRecords(items, limit, offset), len(items), nil
}

type inMemoryRecoveryPointStore struct {
	mu     sync.Mutex
	points []checkpoint.RecoveryPoint
}

func newInMemoryRecoveryPointStore() *inMemoryRecoveryPointStore {
	return &inMemoryRecoveryPointStore{points: make([]checkpoint.RecoveryPoint, 0)}
}

func (s *inMemoryRecoveryPointStore) WriteRecoveryPoint(_ context.Context, point checkpoint.RecoveryPoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.points = append(s.points, point)
	return nil
}

func (s *inMemoryRecoveryPointStore) ListRecoveryPoints(_ context.Context, taskID string, limit, offset int) ([]checkpoint.RecoveryPoint, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]checkpoint.RecoveryPoint, 0)
	for _, point := range s.points {
		if taskID == "" || point.TaskID == taskID {
			items = append(items, point)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		return parseGovernanceTime(items[i].CreatedAt).After(parseGovernanceTime(items[j].CreatedAt))
	})
	return pageRecoveryPoints(items, limit, offset), len(items), nil
}

type SQLiteAuditStore struct {
	db *sql.DB
}

func NewSQLiteAuditStore(databasePath string) (*SQLiteAuditStore, error) {
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteAuditStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteAuditStore) WriteAuditRecord(ctx context.Context, record audit.Record) error {
	_, err := s.db.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO audit_records (audit_id, task_id, type, action, summary, target, result, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		record.AuditID,
		record.TaskID,
		record.Type,
		record.Action,
		record.Summary,
		record.Target,
		record.Result,
		record.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("write audit record: %w", err)
	}
	return nil
}

func (s *SQLiteAuditStore) ListAuditRecords(ctx context.Context, taskID string, limit, offset int) ([]audit.Record, int, error) {
	countQuery := `SELECT COUNT(1) FROM audit_records`
	query := `SELECT audit_id, task_id, type, action, summary, target, result, created_at FROM audit_records`
	args := []any{}
	if taskID != "" {
		countQuery += ` WHERE task_id = ?`
		query += ` WHERE task_id = ?`
		args = append(args, taskID)
	}
	query += ` ORDER BY created_at DESC, audit_id DESC`
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, firstArg(taskID)...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit records: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit records: %w", err)
	}
	defer rows.Close()
	items := make([]audit.Record, 0)
	for rows.Next() {
		var record audit.Record
		if err := rows.Scan(&record.AuditID, &record.TaskID, &record.Type, &record.Action, &record.Summary, &record.Target, &record.Result, &record.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan audit record: %w", err)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate audit records: %w", err)
	}
	return items, total, nil
}

func (s *SQLiteAuditStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteAuditStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=5000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS audit_records (
			audit_id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			type TEXT NOT NULL,
			action TEXT NOT NULL,
			summary TEXT NOT NULL,
			target TEXT NOT NULL,
			result TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create audit_records table: %w", err)
	}
	return nil
}

type SQLiteRecoveryPointStore struct {
	db *sql.DB
}

func NewSQLiteRecoveryPointStore(databasePath string) (*SQLiteRecoveryPointStore, error) {
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteRecoveryPointStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteRecoveryPointStore) WriteRecoveryPoint(ctx context.Context, point checkpoint.RecoveryPoint) error {
	objectsJSON, err := json.Marshal(point.Objects)
	if err != nil {
		return fmt.Errorf("marshal recovery point objects: %w", err)
	}
	_, err = s.db.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO recovery_points (recovery_point_id, task_id, summary, created_at, objects_json)
		 VALUES (?, ?, ?, ?, ?)`,
		point.RecoveryPointID,
		point.TaskID,
		point.Summary,
		point.CreatedAt,
		string(objectsJSON),
	)
	if err != nil {
		return fmt.Errorf("write recovery point: %w", err)
	}
	return nil
}

func (s *SQLiteRecoveryPointStore) ListRecoveryPoints(ctx context.Context, taskID string, limit, offset int) ([]checkpoint.RecoveryPoint, int, error) {
	countQuery := `SELECT COUNT(1) FROM recovery_points`
	query := `SELECT recovery_point_id, task_id, summary, created_at, objects_json FROM recovery_points`
	args := []any{}
	if taskID != "" {
		countQuery += ` WHERE task_id = ?`
		query += ` WHERE task_id = ?`
		args = append(args, taskID)
	}
	query += ` ORDER BY created_at DESC, recovery_point_id DESC`
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, firstArg(taskID)...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count recovery points: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list recovery points: %w", err)
	}
	defer rows.Close()
	items := make([]checkpoint.RecoveryPoint, 0)
	for rows.Next() {
		var point checkpoint.RecoveryPoint
		var objectsJSON string
		if err := rows.Scan(&point.RecoveryPointID, &point.TaskID, &point.Summary, &point.CreatedAt, &objectsJSON); err != nil {
			return nil, 0, fmt.Errorf("scan recovery point: %w", err)
		}
		if err := json.Unmarshal([]byte(objectsJSON), &point.Objects); err != nil {
			return nil, 0, fmt.Errorf("unmarshal recovery point objects: %w", err)
		}
		items = append(items, point)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate recovery points: %w", err)
	}
	return items, total, nil
}

func (s *SQLiteRecoveryPointStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteRecoveryPointStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=5000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS recovery_points (
			recovery_point_id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			summary TEXT NOT NULL,
			created_at TEXT NOT NULL,
			objects_json TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create recovery_points table: %w", err)
	}
	return nil
}

func openSQLiteDatabase(databasePath string) (*sql.DB, error) {
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
	return db, nil
}

func pageAuditRecords(items []audit.Record, limit, offset int) []audit.Record {
	if offset >= len(items) {
		return nil
	}
	if limit <= 0 {
		limit = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return append([]audit.Record(nil), items[offset:end]...)
}

func pageRecoveryPoints(items []checkpoint.RecoveryPoint, limit, offset int) []checkpoint.RecoveryPoint {
	if offset >= len(items) {
		return nil
	}
	if limit <= 0 {
		limit = len(items)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return append([]checkpoint.RecoveryPoint(nil), items[offset:end]...)
}

func firstArg(taskID string) []any {
	if taskID == "" {
		return nil
	}
	return []any{taskID}
}

func parseGovernanceTime(value string) time.Time {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed
	}
	return time.Time{}
}
