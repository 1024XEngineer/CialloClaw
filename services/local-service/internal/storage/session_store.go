package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
)

type inMemorySessionStore struct {
	mu      sync.Mutex
	records map[string]SessionRecord
}

func newInMemorySessionStore() *inMemorySessionStore {
	return &inMemorySessionStore{records: make(map[string]SessionRecord)}
}

func (s *inMemorySessionStore) WriteSession(_ context.Context, record SessionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[record.SessionID] = record
	return nil
}

func (s *inMemorySessionStore) DeleteSession(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, sessionID)
	return nil
}

func (s *inMemorySessionStore) GetSession(_ context.Context, sessionID string) (SessionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[sessionID]
	if !ok {
		return SessionRecord{}, sql.ErrNoRows
	}
	return record, nil
}

func (s *inMemorySessionStore) ListSessions(_ context.Context, limit, offset int) ([]SessionRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]SessionRecord, 0, len(s.records))
	for _, record := range s.records {
		items = append(items, record)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return parseGovernanceTime(items[i].UpdatedAt).After(parseGovernanceTime(items[j].UpdatedAt))
	})
	return pageSessions(items, limit, offset), len(items), nil
}

// SQLiteSessionStore persists first-class sessions rows in SQLite.
type SQLiteSessionStore struct {
	db *sql.DB
}

func NewSQLiteSessionStore(databasePath string) (*SQLiteSessionStore, error) {
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteSessionStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteSessionStore) WriteSession(ctx context.Context, record SessionRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO sessions (session_id, title, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, record.SessionID, record.Title, record.Status, record.CreatedAt, record.UpdatedAt)
	if err != nil {
		return fmt.Errorf("write session: %w", err)
	}
	return nil
}

func (s *SQLiteSessionStore) DeleteSession(ctx context.Context, sessionID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE session_id = ?`, sessionID)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *SQLiteSessionStore) GetSession(ctx context.Context, sessionID string) (SessionRecord, error) {
	var record SessionRecord
	err := s.db.QueryRowContext(ctx, `SELECT session_id, title, status, created_at, updated_at FROM sessions WHERE session_id = ?`, sessionID).Scan(&record.SessionID, &record.Title, &record.Status, &record.CreatedAt, &record.UpdatedAt)
	if err != nil {
		return SessionRecord{}, err
	}
	return record, nil
}

func (s *SQLiteSessionStore) ListSessions(ctx context.Context, limit, offset int) ([]SessionRecord, int, error) {
	query := `SELECT session_id, title, status, created_at, updated_at FROM sessions ORDER BY updated_at DESC, session_id DESC`
	countQuery := `SELECT COUNT(1) FROM sessions`
	args := make([]any, 0, 2)
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count sessions: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()
	items := make([]SessionRecord, 0)
	for rows.Next() {
		var record SessionRecord
		if err := rows.Scan(&record.SessionID, &record.Title, &record.Status, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan session: %w", err)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate sessions: %w", err)
	}
	return items, total, nil
}

func (s *SQLiteSessionStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteSessionStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=300000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS sessions (session_id TEXT PRIMARY KEY, title TEXT NOT NULL, status TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create sessions table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_sessions_updated_at ON sessions(updated_at DESC);`); err != nil {
		return fmt.Errorf("create sessions updated_at index: %w", err)
	}
	return nil
}

func pageSessions(items []SessionRecord, limit, offset int) []SessionRecord {
	if offset >= len(items) {
		return []SessionRecord{}
	}
	if limit <= 0 {
		return append([]SessionRecord(nil), items[offset:]...)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return append([]SessionRecord(nil), items[offset:end]...)
}
