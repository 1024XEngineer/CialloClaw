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

type inMemorySkillManifestStore struct {
	mu      sync.Mutex
	records map[string]SkillManifestRecord
	order   []string
}

func newInMemorySkillManifestStore() *inMemorySkillManifestStore {
	return &inMemorySkillManifestStore{records: make(map[string]SkillManifestRecord), order: make([]string, 0)}
}

func (s *inMemorySkillManifestStore) WriteSkillManifest(_ context.Context, record SkillManifestRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.records[record.SkillManifestID]; !exists {
		s.order = append(s.order, record.SkillManifestID)
	}
	s.records[record.SkillManifestID] = record
	return nil
}

func (s *inMemorySkillManifestStore) GetSkillManifest(_ context.Context, skillManifestID string) (SkillManifestRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[skillManifestID]
	if !ok {
		return SkillManifestRecord{}, sql.ErrNoRows
	}
	return record, nil
}

func (s *inMemorySkillManifestStore) ListSkillManifests(_ context.Context, limit, offset int) ([]SkillManifestRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]SkillManifestRecord, 0, len(s.order))
	for _, id := range s.order {
		items = append(items, s.records[id])
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].UpdatedAt > items[j].UpdatedAt
	})
	return pageSkillManifests(items, limit, offset), len(items), nil
}

type inMemoryBlueprintDefinitionStore struct {
	mu      sync.Mutex
	records map[string]BlueprintDefinitionRecord
	order   []string
}

func newInMemoryBlueprintDefinitionStore() *inMemoryBlueprintDefinitionStore {
	return &inMemoryBlueprintDefinitionStore{records: make(map[string]BlueprintDefinitionRecord), order: make([]string, 0)}
}

func (s *inMemoryBlueprintDefinitionStore) WriteBlueprintDefinition(_ context.Context, record BlueprintDefinitionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.records[record.BlueprintDefinitionID]; !exists {
		s.order = append(s.order, record.BlueprintDefinitionID)
	}
	s.records[record.BlueprintDefinitionID] = record
	return nil
}

func (s *inMemoryBlueprintDefinitionStore) GetBlueprintDefinition(_ context.Context, blueprintDefinitionID string) (BlueprintDefinitionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[blueprintDefinitionID]
	if !ok {
		return BlueprintDefinitionRecord{}, sql.ErrNoRows
	}
	return record, nil
}

func (s *inMemoryBlueprintDefinitionStore) ListBlueprintDefinitions(_ context.Context, limit, offset int) ([]BlueprintDefinitionRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]BlueprintDefinitionRecord, 0, len(s.order))
	for _, id := range s.order {
		items = append(items, s.records[id])
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].UpdatedAt > items[j].UpdatedAt
	})
	return pageBlueprintDefinitions(items, limit, offset), len(items), nil
}

type inMemoryPromptTemplateVersionStore struct {
	mu      sync.Mutex
	records map[string]PromptTemplateVersionRecord
	order   []string
}

func newInMemoryPromptTemplateVersionStore() *inMemoryPromptTemplateVersionStore {
	return &inMemoryPromptTemplateVersionStore{records: make(map[string]PromptTemplateVersionRecord), order: make([]string, 0)}
}

func (s *inMemoryPromptTemplateVersionStore) WritePromptTemplateVersion(_ context.Context, record PromptTemplateVersionRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.records[record.PromptTemplateVersionID]; !exists {
		s.order = append(s.order, record.PromptTemplateVersionID)
	}
	s.records[record.PromptTemplateVersionID] = record
	return nil
}

func (s *inMemoryPromptTemplateVersionStore) GetPromptTemplateVersion(_ context.Context, promptTemplateVersionID string) (PromptTemplateVersionRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[promptTemplateVersionID]
	if !ok {
		return PromptTemplateVersionRecord{}, sql.ErrNoRows
	}
	return record, nil
}

func (s *inMemoryPromptTemplateVersionStore) ListPromptTemplateVersions(_ context.Context, limit, offset int) ([]PromptTemplateVersionRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]PromptTemplateVersionRecord, 0, len(s.order))
	for _, id := range s.order {
		items = append(items, s.records[id])
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].UpdatedAt > items[j].UpdatedAt
	})
	return pagePromptTemplateVersions(items, limit, offset), len(items), nil
}

type SQLiteSkillManifestStore struct{ db *sql.DB }

func NewSQLiteSkillManifestStore(databasePath string) (*SQLiteSkillManifestStore, error) {
	if strings.TrimSpace(databasePath) == "" {
		return nil, errors.New("sqlite config asset database path is required")
	}
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteSkillManifestStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteSkillManifestStore) WriteSkillManifest(ctx context.Context, record SkillManifestRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO skill_manifests (skill_manifest_id, name, version, source, summary, manifest_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, record.SkillManifestID, record.Name, record.Version, record.Source, record.Summary, record.ManifestJSON, record.CreatedAt, record.UpdatedAt)
	if err != nil {
		return fmt.Errorf("write skill manifest: %w", err)
	}
	return nil
}

func (s *SQLiteSkillManifestStore) GetSkillManifest(ctx context.Context, skillManifestID string) (SkillManifestRecord, error) {
	var record SkillManifestRecord
	err := s.db.QueryRowContext(ctx, `SELECT skill_manifest_id, name, version, source, summary, manifest_json, created_at, updated_at FROM skill_manifests WHERE skill_manifest_id = ?`, skillManifestID).Scan(&record.SkillManifestID, &record.Name, &record.Version, &record.Source, &record.Summary, &record.ManifestJSON, &record.CreatedAt, &record.UpdatedAt)
	if err != nil {
		return SkillManifestRecord{}, err
	}
	return record, nil
}

func (s *SQLiteSkillManifestStore) ListSkillManifests(ctx context.Context, limit, offset int) ([]SkillManifestRecord, int, error) {
	return listSQLiteSkillManifests(ctx, s.db, limit, offset)
}

func (s *SQLiteSkillManifestStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteSkillManifestStore) initialize(ctx context.Context) error {
	if err := configureConfigAssetSQLiteDatabase(ctx, s.db); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS skill_manifests (skill_manifest_id TEXT PRIMARY KEY, name TEXT NOT NULL, version TEXT NOT NULL, source TEXT NOT NULL, summary TEXT NOT NULL, manifest_json TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create skill_manifests table: %w", err)
	}
	return nil
}

type SQLiteBlueprintDefinitionStore struct{ db *sql.DB }

func NewSQLiteBlueprintDefinitionStore(databasePath string) (*SQLiteBlueprintDefinitionStore, error) {
	if strings.TrimSpace(databasePath) == "" {
		return nil, errors.New("sqlite config asset database path is required")
	}
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteBlueprintDefinitionStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteBlueprintDefinitionStore) WriteBlueprintDefinition(ctx context.Context, record BlueprintDefinitionRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO blueprint_definitions (blueprint_definition_id, name, version, source, summary, definition_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, record.BlueprintDefinitionID, record.Name, record.Version, record.Source, record.Summary, record.DefinitionJSON, record.CreatedAt, record.UpdatedAt)
	if err != nil {
		return fmt.Errorf("write blueprint definition: %w", err)
	}
	return nil
}

func (s *SQLiteBlueprintDefinitionStore) GetBlueprintDefinition(ctx context.Context, blueprintDefinitionID string) (BlueprintDefinitionRecord, error) {
	var record BlueprintDefinitionRecord
	err := s.db.QueryRowContext(ctx, `SELECT blueprint_definition_id, name, version, source, summary, definition_json, created_at, updated_at FROM blueprint_definitions WHERE blueprint_definition_id = ?`, blueprintDefinitionID).Scan(&record.BlueprintDefinitionID, &record.Name, &record.Version, &record.Source, &record.Summary, &record.DefinitionJSON, &record.CreatedAt, &record.UpdatedAt)
	if err != nil {
		return BlueprintDefinitionRecord{}, err
	}
	return record, nil
}

func (s *SQLiteBlueprintDefinitionStore) ListBlueprintDefinitions(ctx context.Context, limit, offset int) ([]BlueprintDefinitionRecord, int, error) {
	return listSQLiteBlueprintDefinitions(ctx, s.db, limit, offset)
}

func (s *SQLiteBlueprintDefinitionStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteBlueprintDefinitionStore) initialize(ctx context.Context) error {
	if err := configureConfigAssetSQLiteDatabase(ctx, s.db); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS blueprint_definitions (blueprint_definition_id TEXT PRIMARY KEY, name TEXT NOT NULL, version TEXT NOT NULL, source TEXT NOT NULL, summary TEXT NOT NULL, definition_json TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create blueprint_definitions table: %w", err)
	}
	return nil
}

type SQLitePromptTemplateVersionStore struct{ db *sql.DB }

func NewSQLitePromptTemplateVersionStore(databasePath string) (*SQLitePromptTemplateVersionStore, error) {
	if strings.TrimSpace(databasePath) == "" {
		return nil, errors.New("sqlite config asset database path is required")
	}
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	store := &SQLitePromptTemplateVersionStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLitePromptTemplateVersionStore) WritePromptTemplateVersion(ctx context.Context, record PromptTemplateVersionRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO prompt_template_versions (prompt_template_version_id, template_name, version, source, summary, template_body, variables_json, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, record.PromptTemplateVersionID, record.TemplateName, record.Version, record.Source, record.Summary, record.TemplateBody, record.VariablesJSON, record.CreatedAt, record.UpdatedAt)
	if err != nil {
		return fmt.Errorf("write prompt template version: %w", err)
	}
	return nil
}

func (s *SQLitePromptTemplateVersionStore) GetPromptTemplateVersion(ctx context.Context, promptTemplateVersionID string) (PromptTemplateVersionRecord, error) {
	var record PromptTemplateVersionRecord
	err := s.db.QueryRowContext(ctx, `SELECT prompt_template_version_id, template_name, version, source, summary, template_body, variables_json, created_at, updated_at FROM prompt_template_versions WHERE prompt_template_version_id = ?`, promptTemplateVersionID).Scan(&record.PromptTemplateVersionID, &record.TemplateName, &record.Version, &record.Source, &record.Summary, &record.TemplateBody, &record.VariablesJSON, &record.CreatedAt, &record.UpdatedAt)
	if err != nil {
		return PromptTemplateVersionRecord{}, err
	}
	return record, nil
}

func (s *SQLitePromptTemplateVersionStore) ListPromptTemplateVersions(ctx context.Context, limit, offset int) ([]PromptTemplateVersionRecord, int, error) {
	return listSQLitePromptTemplateVersions(ctx, s.db, limit, offset)
}

func (s *SQLitePromptTemplateVersionStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLitePromptTemplateVersionStore) initialize(ctx context.Context) error {
	if err := configureConfigAssetSQLiteDatabase(ctx, s.db); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS prompt_template_versions (prompt_template_version_id TEXT PRIMARY KEY, template_name TEXT NOT NULL, version TEXT NOT NULL, source TEXT NOT NULL, summary TEXT NOT NULL, template_body TEXT NOT NULL, variables_json TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create prompt_template_versions table: %w", err)
	}
	return nil
}

// configureConfigAssetSQLiteDatabase aligns config-asset stores with the same
// busy-timeout and WAL behavior used by the rest of the SQLite storage layer so
// future concurrent reads/writes do not fail immediately with locked database
// errors.
func configureConfigAssetSQLiteDatabase(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA busy_timeout=5000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	return nil
}

func listSQLiteSkillManifests(ctx context.Context, db *sql.DB, limit, offset int) ([]SkillManifestRecord, int, error) {
	return listSQLiteConfigAssets(ctx, db, `SELECT skill_manifest_id, name, version, source, summary, manifest_json, created_at, updated_at FROM skill_manifests ORDER BY updated_at DESC, skill_manifest_id DESC`, `SELECT COUNT(1) FROM skill_manifests`, limit, offset, scanSkillManifest)
}

func listSQLiteBlueprintDefinitions(ctx context.Context, db *sql.DB, limit, offset int) ([]BlueprintDefinitionRecord, int, error) {
	return listSQLiteConfigAssets(ctx, db, `SELECT blueprint_definition_id, name, version, source, summary, definition_json, created_at, updated_at FROM blueprint_definitions ORDER BY updated_at DESC, blueprint_definition_id DESC`, `SELECT COUNT(1) FROM blueprint_definitions`, limit, offset, scanBlueprintDefinition)
}

func listSQLitePromptTemplateVersions(ctx context.Context, db *sql.DB, limit, offset int) ([]PromptTemplateVersionRecord, int, error) {
	return listSQLiteConfigAssets(ctx, db, `SELECT prompt_template_version_id, template_name, version, source, summary, template_body, variables_json, created_at, updated_at FROM prompt_template_versions ORDER BY updated_at DESC, prompt_template_version_id DESC`, `SELECT COUNT(1) FROM prompt_template_versions`, limit, offset, scanPromptTemplateVersion)
}

func listSQLiteConfigAssets[T any](ctx context.Context, db *sql.DB, query, countQuery string, limit, offset int, scan func(*sql.Rows) (T, error)) ([]T, int, error) {
	args := make([]any, 0, 2)
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}
	var total int
	if err := db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]T, 0)
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func scanSkillManifest(rows *sql.Rows) (SkillManifestRecord, error) {
	var record SkillManifestRecord
	err := rows.Scan(&record.SkillManifestID, &record.Name, &record.Version, &record.Source, &record.Summary, &record.ManifestJSON, &record.CreatedAt, &record.UpdatedAt)
	return record, err
}

func scanBlueprintDefinition(rows *sql.Rows) (BlueprintDefinitionRecord, error) {
	var record BlueprintDefinitionRecord
	err := rows.Scan(&record.BlueprintDefinitionID, &record.Name, &record.Version, &record.Source, &record.Summary, &record.DefinitionJSON, &record.CreatedAt, &record.UpdatedAt)
	return record, err
}

func scanPromptTemplateVersion(rows *sql.Rows) (PromptTemplateVersionRecord, error) {
	var record PromptTemplateVersionRecord
	err := rows.Scan(&record.PromptTemplateVersionID, &record.TemplateName, &record.Version, &record.Source, &record.Summary, &record.TemplateBody, &record.VariablesJSON, &record.CreatedAt, &record.UpdatedAt)
	return record, err
}

func pageSkillManifests(items []SkillManifestRecord, limit, offset int) []SkillManifestRecord {
	return pageConfigAssets(items, limit, offset)
}

func pageBlueprintDefinitions(items []BlueprintDefinitionRecord, limit, offset int) []BlueprintDefinitionRecord {
	return pageConfigAssets(items, limit, offset)
}

func pagePromptTemplateVersions(items []PromptTemplateVersionRecord, limit, offset int) []PromptTemplateVersionRecord {
	return pageConfigAssets(items, limit, offset)
}

func pageConfigAssets[T any](items []T, limit, offset int) []T {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) {
		return nil
	}
	if limit <= 0 {
		return append([]T(nil), items[offset:]...)
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return append([]T(nil), items[offset:end]...)
}
