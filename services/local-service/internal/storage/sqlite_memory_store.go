// 该文件负责存储层的数据接口或落盘实现。
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const sqliteDriverName = "sqlite"
const sqliteMemorySource = "storage_sqlite"

var ErrMemorySummaryIDRequired = errors.New("storage memory_summary_id is required")
var ErrMemoryTaskIDRequired = errors.New("storage memory task_id is required")
var ErrMemoryRunIDRequired = errors.New("storage memory run_id is required")
var ErrMemorySummaryRequired = errors.New("storage memory summary is required")
var ErrMemoryCreatedAtRequired = errors.New("storage memory created_at is required")
var ErrMemoryCreatedAtInvalid = errors.New("storage memory created_at must be rfc3339")

type SQLiteMemoryStore struct {
	db *sql.DB
}

func NewSQLiteMemoryStore(databasePath string) (*SQLiteMemoryStore, error) {
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

	store := &SQLiteMemoryStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *SQLiteMemoryStore) SaveSummary(ctx context.Context, summary MemorySummaryRecord) error {
	if err := validateMemorySummaryRecord(summary); err != nil {
		return err
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO memory_summaries (memory_summary_id, task_id, run_id, summary, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		summary.MemorySummaryID,
		summary.TaskID,
		summary.RunID,
		summary.Summary,
		summary.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("save memory summary: %w", err)
	}

	return nil
}

func (s *SQLiteMemoryStore) SearchSummaries(ctx context.Context, taskID, runID, query string, limit int) ([]MemoryRetrievalRecord, error) {
	limit = normalizeMemoryLimit(limit)
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return []MemoryRetrievalRecord{}, nil
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT memory_summary_id, summary
		 FROM memory_summaries
		 WHERE NOT (task_id = ? AND run_id = ?)`,
		taskID,
		runID,
	)
	if err != nil {
		return nil, fmt.Errorf("search memory summaries: %w", err)
	}
	defer rows.Close()

	hits := make([]MemoryRetrievalRecord, 0)
	for rows.Next() {
		var memoryID string
		var summary string
		if err := rows.Scan(&memoryID, &summary); err != nil {
			return nil, fmt.Errorf("scan memory summary search row: %w", err)
		}

		score := matchMemorySummary(summary, query)
		if score <= 0 {
			continue
		}

		hits = append(hits, MemoryRetrievalRecord{
			RetrievalHitID: memoryID,
			TaskID:         taskID,
			RunID:          runID,
			MemoryID:       memoryID,
			Score:          score,
			Source:         sqliteMemorySource,
			Summary:        summary,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memory summary search rows: %w", err)
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].RetrievalHitID < hits[j].RetrievalHitID
		}

		return hits[i].Score > hits[j].Score
	})

	if len(hits) > limit {
		return hits[:limit], nil
	}

	return hits, nil
}

func (s *SQLiteMemoryStore) ListRecentSummaries(ctx context.Context, limit int) ([]MemorySummaryRecord, error) {
	limit = normalizeMemoryLimit(limit)

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT memory_summary_id, task_id, run_id, summary, created_at
		 FROM memory_summaries
		 ORDER BY created_at DESC, memory_summary_id DESC
		 LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list recent memory summaries: %w", err)
	}
	defer rows.Close()

	summaries := make([]MemorySummaryRecord, 0, limit)
	for rows.Next() {
		var summary MemorySummaryRecord
		if err := rows.Scan(&summary.MemorySummaryID, &summary.TaskID, &summary.RunID, &summary.Summary, &summary.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan recent memory summary row: %w", err)
		}
		summaries = append(summaries, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recent memory summaries: %w", err)
	}

	return summaries, nil
}

func (s *SQLiteMemoryStore) Close() error {
	if s.db == nil {
		return nil
	}

	return s.db.Close()
}

func (s *SQLiteMemoryStore) journalMode(ctx context.Context) (string, error) {
	var mode string
	if err := s.db.QueryRowContext(ctx, `PRAGMA journal_mode;`).Scan(&mode); err != nil {
		return "", fmt.Errorf("query sqlite journal mode: %w", err)
	}

	return strings.ToLower(strings.TrimSpace(mode)), nil
}

func (s *SQLiteMemoryStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=5000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS memory_summaries (
			memory_summary_id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			summary TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create memory summaries table: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_memory_summaries_created_at ON memory_summaries(created_at DESC);`); err != nil {
		return fmt.Errorf("create memory summaries created_at index: %w", err)
	}

	return nil
}

func validateMemorySummaryRecord(summary MemorySummaryRecord) error {
	if strings.TrimSpace(summary.MemorySummaryID) == "" {
		return ErrMemorySummaryIDRequired
	}
	if strings.TrimSpace(summary.TaskID) == "" {
		return ErrMemoryTaskIDRequired
	}
	if strings.TrimSpace(summary.RunID) == "" {
		return ErrMemoryRunIDRequired
	}
	if strings.TrimSpace(summary.Summary) == "" {
		return ErrMemorySummaryRequired
	}
	if strings.TrimSpace(summary.CreatedAt) == "" {
		return ErrMemoryCreatedAtRequired
	}
	if _, err := time.Parse(time.RFC3339, summary.CreatedAt); err != nil {
		return ErrMemoryCreatedAtInvalid
	}

	return nil
}
