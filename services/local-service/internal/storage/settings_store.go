package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

const settingsSnapshotKey = "runtime"

type inMemorySettingsStore struct {
	mu       sync.Mutex
	snapshot map[string]any
}

func newInMemorySettingsStore() *inMemorySettingsStore {
	return &inMemorySettingsStore{snapshot: nil}
}

func (s *inMemorySettingsStore) SaveSettingsSnapshot(_ context.Context, snapshot map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot = cloneSettingsMap(snapshot)
	return nil
}

func (s *inMemorySettingsStore) LoadSettingsSnapshot(_ context.Context) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneSettingsMap(s.snapshot), nil
}

// SQLiteSettingsStore persists one ordinary settings snapshot in SQLite.
type SQLiteSettingsStore struct {
	db *sql.DB
}

func NewSQLiteSettingsStore(databasePath string) (*SQLiteSettingsStore, error) {
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteSettingsStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteSettingsStore) SaveSettingsSnapshot(ctx context.Context, snapshot map[string]any) error {
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal settings snapshot: %w", err)
	}
	_, err = s.db.ExecContext(ctx, `INSERT OR REPLACE INTO settings_snapshots (snapshot_key, snapshot_json, updated_at) VALUES (?, ?, ?)`, settingsSnapshotKey, string(encoded), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("%w: write settings snapshot: %v", ErrStructuredStoreUnavailable, err)
	}
	return nil
}

func (s *SQLiteSettingsStore) LoadSettingsSnapshot(ctx context.Context) (map[string]any, error) {
	var rawSnapshot string
	err := s.db.QueryRowContext(ctx, `SELECT snapshot_json FROM settings_snapshots WHERE snapshot_key = ?`, settingsSnapshotKey).Scan(&rawSnapshot)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("%w: load settings snapshot: %v", ErrStructuredStoreUnavailable, err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal([]byte(rawSnapshot), &snapshot); err != nil {
		return nil, fmt.Errorf("decode settings snapshot: %w", err)
	}
	return snapshot, nil
}

func (s *SQLiteSettingsStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteSettingsStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("%w: enable sqlite wal mode: %v", ErrStructuredStoreUnavailable, err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=300000;`); err != nil {
		return fmt.Errorf("%w: set sqlite busy timeout: %v", ErrStructuredStoreUnavailable, err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS settings_snapshots (snapshot_key TEXT PRIMARY KEY, snapshot_json TEXT NOT NULL, updated_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("%w: create settings_snapshots table: %v", ErrStructuredStoreUnavailable, err)
	}
	return nil
}

func cloneSettingsMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			result[key] = cloneSettingsMap(typed)
		case []map[string]any:
			cloned := make([]map[string]any, 0, len(typed))
			for _, item := range typed {
				cloned = append(cloned, cloneSettingsMap(item))
			}
			result[key] = cloned
		case []string:
			result[key] = append([]string(nil), typed...)
		case []any:
			result[key] = append([]any(nil), typed...)
		default:
			result[key] = value
		}
	}
	return result
}
