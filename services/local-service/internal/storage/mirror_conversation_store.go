package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"sync"
	"time"
)

type inMemoryMirrorConversationStore struct {
	mu      sync.Mutex
	records []MirrorConversationRecord
}

func newInMemoryMirrorConversationStore() *inMemoryMirrorConversationStore {
	return &inMemoryMirrorConversationStore{records: make([]MirrorConversationRecord, 0)}
}

func (s *inMemoryMirrorConversationStore) SaveMirrorConversation(_ context.Context, record MirrorConversationRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	record = normalizeMirrorConversationRecord(record)
	for index, existing := range s.records {
		if existing.RecordID != record.RecordID {
			continue
		}
		s.records[index] = record
		return nil
	}
	s.records = append(s.records, record)
	return nil
}

func (s *inMemoryMirrorConversationStore) ListMirrorConversations(_ context.Context, taskID, source, status string, limit, offset int) ([]MirrorConversationRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := filterMirrorConversationRecords(s.records, taskID, source, status)
	total := len(items)
	if offset >= total {
		return []MirrorConversationRecord{}, total, nil
	}
	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}
	return cloneMirrorConversationRecords(items[offset:end]), total, nil
}

type SQLiteMirrorConversationStore struct {
	db *sql.DB
}

func NewSQLiteMirrorConversationStore(databasePath string) (*SQLiteMirrorConversationStore, error) {
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteMirrorConversationStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteMirrorConversationStore) SaveMirrorConversation(ctx context.Context, record MirrorConversationRecord) error {
	record = normalizeMirrorConversationRecord(record)
	_, err := s.db.ExecContext(
		ctx,
		`INSERT OR REPLACE INTO mirror_conversations (
			record_id, trace_id, created_at, updated_at, source, trigger, input_mode,
			session_id, task_id, user_text, agent_text, agent_bubble_type, status, error_message
		 ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.RecordID,
		record.TraceID,
		record.CreatedAt,
		record.UpdatedAt,
		record.Source,
		record.Trigger,
		record.InputMode,
		nullableText(record.SessionID),
		nullableText(record.TaskID),
		record.UserText,
		nullableText(record.AgentText),
		nullableText(record.AgentBubbleType),
		record.Status,
		nullableText(record.ErrorMessage),
	)
	if err != nil {
		return fmt.Errorf("save mirror conversation: %w", err)
	}
	return nil
}

func (s *SQLiteMirrorConversationStore) ListMirrorConversations(ctx context.Context, taskID, source, status string, limit, offset int) ([]MirrorConversationRecord, int, error) {
	countQuery := `SELECT COUNT(1) FROM mirror_conversations WHERE 1 = 1`
	query := `SELECT record_id, trace_id, created_at, updated_at, source, trigger, input_mode, session_id, task_id, user_text, agent_text, agent_bubble_type, status, error_message FROM mirror_conversations WHERE 1 = 1`
	args := make([]any, 0, 6)
	countArgs := make([]any, 0, 3)
	if taskID != "" {
		countQuery += ` AND task_id = ?`
		query += ` AND task_id = ?`
		args = append(args, taskID)
		countArgs = append(countArgs, taskID)
	}
	if source != "" {
		countQuery += ` AND source = ?`
		query += ` AND source = ?`
		args = append(args, source)
		countArgs = append(countArgs, source)
	}
	if status != "" {
		countQuery += ` AND status = ?`
		query += ` AND status = ?`
		args = append(args, status)
		countArgs = append(countArgs, status)
	}
	query += ` ORDER BY updated_at DESC, created_at DESC, record_id DESC`
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count mirror conversations: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list mirror conversations: %w", err)
	}
	defer rows.Close()
	items := make([]MirrorConversationRecord, 0)
	for rows.Next() {
		record, err := scanMirrorConversationRecord(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate mirror conversations: %w", err)
	}
	return items, total, nil
}

func (s *SQLiteMirrorConversationStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteMirrorConversationStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=5000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS mirror_conversations (
			record_id TEXT PRIMARY KEY,
			trace_id TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			source TEXT NOT NULL,
			trigger TEXT NOT NULL,
			input_mode TEXT NOT NULL,
			session_id TEXT,
			task_id TEXT,
			user_text TEXT NOT NULL,
			agent_text TEXT,
			agent_bubble_type TEXT,
			status TEXT NOT NULL,
			error_message TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_mirror_conversations_updated ON mirror_conversations(updated_at DESC, created_at DESC, record_id DESC);
		CREATE INDEX IF NOT EXISTS idx_mirror_conversations_task ON mirror_conversations(task_id, updated_at DESC);
	`); err != nil {
		return fmt.Errorf("create mirror_conversations table: %w", err)
	}
	return nil
}

func filterMirrorConversationRecords(records []MirrorConversationRecord, taskID, source, status string) []MirrorConversationRecord {
	items := make([]MirrorConversationRecord, 0, len(records))
	for _, record := range records {
		if taskID != "" && record.TaskID != taskID {
			continue
		}
		if source != "" && record.Source != source {
			continue
		}
		if status != "" && record.Status != status {
			continue
		}
		items = append(items, record)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].UpdatedAt != items[j].UpdatedAt {
			return items[i].UpdatedAt > items[j].UpdatedAt
		}
		if items[i].CreatedAt != items[j].CreatedAt {
			return items[i].CreatedAt > items[j].CreatedAt
		}
		return items[i].RecordID > items[j].RecordID
	})
	return items
}

func cloneMirrorConversationRecords(records []MirrorConversationRecord) []MirrorConversationRecord {
	cloned := make([]MirrorConversationRecord, 0, len(records))
	for _, record := range records {
		cloned = append(cloned, record)
	}
	return cloned
}

func normalizeMirrorConversationRecord(record MirrorConversationRecord) MirrorConversationRecord {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if record.CreatedAt == "" {
		record.CreatedAt = now
	}
	if record.UpdatedAt == "" {
		record.UpdatedAt = record.CreatedAt
	}
	return record
}

func scanMirrorConversationRecord(scanner interface{ Scan(dest ...any) error }) (MirrorConversationRecord, error) {
	record := MirrorConversationRecord{}
	var sessionID sql.NullString
	var taskID sql.NullString
	var agentText sql.NullString
	var agentBubbleType sql.NullString
	var errorMessage sql.NullString
	if err := scanner.Scan(
		&record.RecordID,
		&record.TraceID,
		&record.CreatedAt,
		&record.UpdatedAt,
		&record.Source,
		&record.Trigger,
		&record.InputMode,
		&sessionID,
		&taskID,
		&record.UserText,
		&agentText,
		&agentBubbleType,
		&record.Status,
		&errorMessage,
	); err != nil {
		return MirrorConversationRecord{}, fmt.Errorf("scan mirror conversation: %w", err)
	}
	record.SessionID = nullableStringValue(sessionID)
	record.TaskID = nullableStringValue(taskID)
	record.AgentText = nullableStringValue(agentText)
	record.AgentBubbleType = nullableStringValue(agentBubbleType)
	record.ErrorMessage = nullableStringValue(errorMessage)
	return record, nil
}

func nullableStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}
