package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrStrongholdAccessFailed reports formal Stronghold lifecycle access failure.
var ErrStrongholdAccessFailed = ErrSecretStoreAccessFailed

type inMemorySecretStore struct {
	mu      sync.Mutex
	records map[string]SecretRecord
}

func newInMemorySecretStore() *inMemorySecretStore {
	return &inMemorySecretStore{records: make(map[string]SecretRecord)}
}

func (s *inMemorySecretStore) PutSecret(_ context.Context, record SecretRecord) error {
	if err := validateSecretRecord(record); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[secretStoreKey(record.Namespace, record.Key)] = record
	return nil
}

func (s *inMemorySecretStore) GetSecret(_ context.Context, namespace, key string) (SecretRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[secretStoreKey(namespace, key)]
	if !ok {
		return SecretRecord{}, ErrSecretNotFound
	}
	return record, nil
}

func (s *inMemorySecretStore) DeleteSecret(_ context.Context, namespace, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, secretStoreKey(namespace, key))
	return nil
}

// SQLiteSecretStore persists secrets in a dedicated SQLite fallback database.
// It remains the development fallback once the formal Stronghold lifecycle is
// introduced through StrongholdProvider.
type SQLiteSecretStore struct {
	db *sql.DB
}

// NewSQLiteSecretStore creates a SQLite-backed secret store.
func NewSQLiteSecretStore(databasePath string) (*SQLiteSecretStore, error) {
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSecretStoreAccessFailed, err)
	}
	store := &SQLiteSecretStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("%w: %v", ErrSecretStoreAccessFailed, err)
	}
	return store, nil
}

// PutSecret writes or replaces one secret record.
func (s *SQLiteSecretStore) PutSecret(ctx context.Context, record SecretRecord) error {
	if err := validateSecretRecord(record); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO stronghold_secrets (namespace, key_name, value, updated_at) VALUES (?, ?, ?, ?)`,
		record.Namespace,
		record.Key,
		record.Value,
		record.UpdatedAt,
	); err != nil {
		return fmt.Errorf("%w: %v", ErrSecretStoreAccessFailed, err)
	}
	return nil
}

// GetSecret returns one secret record by namespace and key.
func (s *SQLiteSecretStore) GetSecret(ctx context.Context, namespace, key string) (SecretRecord, error) {
	row := s.db.QueryRowContext(ctx, `SELECT namespace, key_name, value, updated_at FROM stronghold_secrets WHERE namespace = ? AND key_name = ?`, namespace, key)
	var record SecretRecord
	if err := row.Scan(&record.Namespace, &record.Key, &record.Value, &record.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return SecretRecord{}, ErrSecretNotFound
		}
		return SecretRecord{}, fmt.Errorf("%w: %v", ErrSecretStoreAccessFailed, err)
	}
	return record, nil
}

// DeleteSecret removes one secret record.
func (s *SQLiteSecretStore) DeleteSecret(ctx context.Context, namespace, key string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM stronghold_secrets WHERE namespace = ? AND key_name = ?`, namespace, key); err != nil {
		return fmt.Errorf("%w: %v", ErrSecretStoreAccessFailed, err)
	}
	return nil
}

// UnavailableSecretStore is used when Stronghold lifecycle exists but cannot be
// opened. This keeps secret APIs explicit about unavailability instead of
// silently pretending the fallback is the formal source of truth.
type UnavailableSecretStore struct{}

func (UnavailableSecretStore) PutSecret(context.Context, SecretRecord) error {
	return ErrStrongholdUnavailable
}

func (UnavailableSecretStore) GetSecret(context.Context, string, string) (SecretRecord, error) {
	return SecretRecord{}, ErrStrongholdUnavailable
}

func (UnavailableSecretStore) DeleteSecret(context.Context, string, string) error {
	return ErrStrongholdUnavailable
}

func NormalizeSecretStoreError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrSecretNotFound) {
		return ErrSecretNotFound
	}
	if errors.Is(err, ErrStrongholdUnavailable) {
		return ErrStrongholdAccessFailed
	}
	if errors.Is(err, ErrSecretStoreAccessFailed) {
		return ErrStrongholdAccessFailed
	}
	return err
}

// Close closes the secret store handle.
func (s *SQLiteSecretStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteSecretStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=300000;`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS stronghold_secrets (
			namespace TEXT NOT NULL,
			key_name TEXT NOT NULL,
			value TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			PRIMARY KEY(namespace, key_name)
		);
	`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_stronghold_updated_at ON stronghold_secrets(updated_at DESC);`); err != nil {
		return err
	}
	return nil
}

// EnsureStrongholdMetadata stores the active Stronghold backend descriptor in a
// dedicated metadata table so diagnostics can distinguish formal and fallback
// lifecycle modes without exposing any secret values.
func (s *SQLiteSecretStore) EnsureStrongholdMetadata(ctx context.Context, backend string) error {
	if s == nil || s.db == nil {
		return ErrSecretStoreAccessFailed
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS stronghold_metadata (metadata_key TEXT PRIMARY KEY, backend TEXT NOT NULL, updated_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("%w: %v", ErrSecretStoreAccessFailed, err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO stronghold_metadata (metadata_key, backend, updated_at) VALUES (?, ?, ?)`, "active_backend", backend, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("%w: %v", ErrSecretStoreAccessFailed, err)
	}
	return nil
}

func validateSecretRecord(record SecretRecord) error {
	if record.Namespace == "" {
		return fmt.Errorf("secret namespace is required")
	}
	if record.Key == "" {
		return fmt.Errorf("secret key is required")
	}
	if record.Value == "" {
		return fmt.Errorf("secret value is required")
	}
	if record.UpdatedAt == "" {
		return fmt.Errorf("secret updated_at is required")
	}
	if _, err := time.Parse(time.RFC3339, record.UpdatedAt); err != nil {
		if _, nanoErr := time.Parse(time.RFC3339Nano, record.UpdatedAt); nanoErr != nil {
			return fmt.Errorf("secret updated_at must be rfc3339")
		}
	}
	return nil
}

func secretRecordValue(record SecretRecord) string {
	return record.Value
}

func secretStoreKey(namespace, key string) string {
	return namespace + "::" + key
}
