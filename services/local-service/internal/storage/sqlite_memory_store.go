// Package storage provides the SQLite-backed memory store implementation.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// sqliteDriverName identifies the SQLite driver shared across storage stores.
const sqliteDriverName = "sqlite"

// sqliteMemorySource labels retrieval hits produced by this SQLite store.
const sqliteMemorySource = "storage_sqlite"

const sqliteFTSTableName = "memory_summaries_fts"
const sqliteVectorStubTableName = "memory_summary_vectors"

// ErrMemorySummaryIDRequired reports a missing memory_summary_id value.
var ErrMemorySummaryIDRequired = errors.New("storage memory_summary_id is required")

// ErrMemoryTaskIDRequired reports a missing task_id value for memory rows.
var ErrMemoryTaskIDRequired = errors.New("storage memory task_id is required")

// ErrMemoryRunIDRequired reports a missing run_id value for memory rows.
var ErrMemoryRunIDRequired = errors.New("storage memory run_id is required")

// ErrMemorySummaryRequired reports a missing summary payload.
var ErrMemorySummaryRequired = errors.New("storage memory summary is required")

// ErrMemoryCreatedAtRequired reports a missing created_at timestamp.
var ErrMemoryCreatedAtRequired = errors.New("storage memory created_at is required")

// ErrMemoryCreatedAtInvalid reports an invalid created_at timestamp.
var ErrMemoryCreatedAtInvalid = errors.New("storage memory created_at must be rfc3339")

// ErrRetrievalHitIDRequired reports a missing retrieval_hit_id value.
var ErrRetrievalHitIDRequired = errors.New("storage retrieval_hit_id is required")

// ErrRetrievalHitTaskIDRequired reports a missing task_id value for hits.
var ErrRetrievalHitTaskIDRequired = errors.New("storage retrieval_hit task_id is required")

// ErrRetrievalHitRunIDRequired reports a missing run_id value for hits.
var ErrRetrievalHitRunIDRequired = errors.New("storage retrieval_hit run_id is required")

// ErrRetrievalHitMemoryIDRequired reports a missing memory_id value for hits.
var ErrRetrievalHitMemoryIDRequired = errors.New("storage retrieval_hit memory_id is required")

// ErrRetrievalHitSourceRequired reports a missing source value for hits.
var ErrRetrievalHitSourceRequired = errors.New("storage retrieval_hit source is required")

// ErrRetrievalHitCreatedAtRequired reports a missing created_at timestamp.
var ErrRetrievalHitCreatedAtRequired = errors.New("storage retrieval_hit created_at is required")

// ErrRetrievalHitCreatedAtInvalid reports an invalid created_at timestamp.
var ErrRetrievalHitCreatedAtInvalid = errors.New("storage retrieval_hit created_at must be rfc3339")

// SQLiteMemoryStore persists memory summaries and retrieval hits in SQLite.
type SQLiteMemoryStore struct {
	db *sql.DB
}

// NewSQLiteMemoryStore opens and initializes the SQLite memory store.
func NewSQLiteMemoryStore(databasePath string) (*SQLiteMemoryStore, error) {
	cleanedPath, err := prepareSQLiteDatabasePath(databasePath)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open(sqliteDriverName, cleanedPath)
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

// SaveSummary persists one memory summary and refreshes its FTS row.
func (s *SQLiteMemoryStore) SaveSummary(ctx context.Context, summary MemorySummaryRecord) error {
	if err := validateMemorySummaryRecord(summary); err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin memory summary transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO memory_summaries (memory_summary_id, task_id, run_id, summary, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		summary.MemorySummaryID,
		summary.TaskID,
		summary.RunID,
		summary.Summary,
		summary.CreatedAt,
	); err != nil {
		return fmt.Errorf("save memory summary: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM memory_summaries_fts WHERE memory_summary_id = ?`, summary.MemorySummaryID); err != nil {
		return fmt.Errorf("delete stale fts summary row: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO memory_summaries_fts (memory_summary_id, summary) VALUES (?, ?)`,
		summary.MemorySummaryID,
		summary.Summary,
	); err != nil {
		return fmt.Errorf("insert fts summary row: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit memory summary transaction: %w", err)
	}

	return nil
}

// SaveRetrievalHits persists one batch of retrieval-hit records.
func (s *SQLiteMemoryStore) SaveRetrievalHits(ctx context.Context, hits []MemoryRetrievalRecord) error {
	if len(hits) == 0 {
		return nil
	}

	for _, hit := range hits {
		if err := validateMemoryRetrievalRecord(hit); err != nil {
			return err
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin retrieval hit transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, hit := range hits {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT OR REPLACE INTO retrieval_hits (retrieval_hit_id, task_id, run_id, memory_id, score, source, summary, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			hit.RetrievalHitID,
			hit.TaskID,
			hit.RunID,
			hit.MemoryID,
			hit.Score,
			hit.Source,
			hit.Summary,
			hit.CreatedAt,
		); err != nil {
			return fmt.Errorf("save retrieval hit %s: %w", hit.RetrievalHitID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit retrieval hit transaction: %w", err)
	}

	return nil
}

// SearchSummaries returns the best matching summaries outside the current run.
func (s *SQLiteMemoryStore) SearchSummaries(ctx context.Context, taskID, runID, query string, limit int) ([]MemoryRetrievalRecord, error) {
	limit = normalizeMemoryLimit(limit)
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return []MemoryRetrievalRecord{}, nil
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT s.memory_summary_id, s.summary
		 FROM memory_summaries_fts
		 JOIN memory_summaries s ON s.memory_summary_id = memory_summaries_fts.memory_summary_id
		 WHERE memory_summaries_fts MATCH ?
		   AND NOT (s.task_id = ? AND s.run_id = ?)`,
		buildFTS5Query(query),
		taskID,
		runID,
	)
	if err != nil {
		rows, err = s.db.QueryContext(
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

// ListRecentSummaries returns the most recent persisted memory summaries.
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

// Close releases the underlying SQLite connection.
func (s *SQLiteMemoryStore) Close() error {
	if s.db == nil {
		return nil
	}

	return s.db.Close()
}

// journalMode reports the active SQLite journal mode for this store.
func (s *SQLiteMemoryStore) journalMode(ctx context.Context) (string, error) {
	var mode string
	if err := s.db.QueryRowContext(ctx, `PRAGMA journal_mode;`).Scan(&mode); err != nil {
		return "", fmt.Errorf("query sqlite journal mode: %w", err)
	}

	return strings.ToLower(strings.TrimSpace(mode)), nil
}

// initialize creates the SQLite tables and indexes required by the store.
func (s *SQLiteMemoryStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=300000;`); err != nil {
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

	if _, err := s.db.ExecContext(ctx, `
		CREATE VIRTUAL TABLE IF NOT EXISTS memory_summaries_fts USING fts5(
			memory_summary_id UNINDEXED,
			summary
		);
	`); err != nil {
		return fmt.Errorf("create memory summaries fts table: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS retrieval_hits (
			retrieval_hit_id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			run_id TEXT NOT NULL,
			memory_id TEXT NOT NULL,
			score REAL NOT NULL,
			source TEXT NOT NULL,
			summary TEXT NOT NULL,
			created_at TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create retrieval hits table: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS memory_summary_vectors (
			memory_summary_id TEXT PRIMARY KEY,
			embedding_blob BLOB,
			provider TEXT NOT NULL DEFAULT 'sqlite_vec_skeleton'
		);
	`); err != nil {
		return fmt.Errorf("create sqlite vec skeleton table: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_memory_summaries_created_at ON memory_summaries(created_at DESC);`); err != nil {
		return fmt.Errorf("create memory summaries created_at index: %w", err)
	}

	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_retrieval_hits_task_run_created_at ON retrieval_hits(task_id, run_id, created_at DESC);`); err != nil {
		return fmt.Errorf("create retrieval hits index: %w", err)
	}

	return nil
}

func buildFTS5Query(query string) string {
	terms := strings.Fields(strings.TrimSpace(query))
	if len(terms) == 0 {
		return `""`
	}

	quotedTerms := make([]string, 0, len(terms))
	for _, term := range terms {
		normalized := strings.TrimSpace(term)
		if normalized == "" {
			continue
		}
		quotedTerms = append(quotedTerms, fmt.Sprintf("\"%s\"", strings.ReplaceAll(normalized, `"`, `""`)))
	}
	if len(quotedTerms) == 0 {
		return `""`
	}

	return strings.Join(quotedTerms, " OR ")
}

// validateMemorySummaryRecord enforces the minimal summary persistence shape.
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

func validateMemoryRetrievalRecord(hit MemoryRetrievalRecord) error {
	if strings.TrimSpace(hit.RetrievalHitID) == "" {
		return ErrRetrievalHitIDRequired
	}
	if strings.TrimSpace(hit.TaskID) == "" {
		return ErrRetrievalHitTaskIDRequired
	}
	if strings.TrimSpace(hit.RunID) == "" {
		return ErrRetrievalHitRunIDRequired
	}
	if strings.TrimSpace(hit.MemoryID) == "" {
		return ErrRetrievalHitMemoryIDRequired
	}
	if strings.TrimSpace(hit.Source) == "" {
		return ErrRetrievalHitSourceRequired
	}
	if strings.TrimSpace(hit.CreatedAt) == "" {
		return ErrRetrievalHitCreatedAtRequired
	}
	if _, err := time.Parse(time.RFC3339, hit.CreatedAt); err != nil {
		return ErrRetrievalHitCreatedAtInvalid
	}

	return nil
}
