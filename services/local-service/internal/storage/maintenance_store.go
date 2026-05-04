package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

var memoryCleanupSQLiteTables = []string{"retrieval_hits", "memory_summaries_fts", "memory_summaries"}

var taskHistoryCleanupSQLiteTables = []string{
	"mirror_conversations",
	"authorization_records",
	"approval_requests",
	"recovery_points",
	"audit_records",
	"artifacts",
	"tool_calls",
	"task_citations",
	"delivery_results",
	"events",
	"steps",
	"runs",
	"task_steps",
	"tasks",
	"task_runs",
	"sessions",
}

// DeleteAllMemory clears persisted mirror/memory summaries and retrieval hits.
func (s *Service) DeleteAllMemory(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if sqliteMemoryStore, ok := s.memoryStore.(*SQLiteMemoryStore); ok && sqliteMemoryStore != nil {
		if err := deleteSQLiteTables(ctx, s.DatabasePath(), memoryCleanupSQLiteTables...); err != nil {
			return err
		}
	}
	if inMemoryStore, ok := s.memoryStore.(*InMemoryMemoryStore); ok && inMemoryStore != nil {
		clearInMemoryMemoryStore(inMemoryStore)
	}
	return nil
}

// DeleteAllTaskHistory clears task-facing runtime history and its persisted read
// models without touching settings, secrets, workspace files, or todo state.
func (s *Service) DeleteAllTaskHistory(ctx context.Context) error {
	if s == nil {
		return nil
	}
	if hasSQLiteTaskHistoryStore(s) {
		if err := deleteSQLiteTables(ctx, s.DatabasePath(), taskHistoryCleanupSQLiteTables...); err != nil {
			return err
		}
	}
	clearInMemoryTaskHistoryStores(s)
	return nil
}

func hasSQLiteTaskHistoryStore(s *Service) bool {
	if s == nil {
		return false
	}
	for _, store := range []any{
		s.taskRunStore,
		s.sessionStore,
		s.taskStore,
		s.taskStepStore,
		s.loopRuntimeStore,
		s.toolCallStore,
		s.artifactStore,
		s.auditStore,
		s.recoveryPointStore,
		s.approvalRequestStore,
		s.authorizationRecordStore,
		s.mirrorConversationStore,
	} {
		switch store.(type) {
		case *SQLiteTaskRunStore,
			*SQLiteSessionStore,
			*SQLiteTaskStore,
			*SQLiteTaskStepStore,
			*SQLiteLoopRuntimeStore,
			*SQLiteToolCallStore,
			*SQLiteArtifactStore,
			*SQLiteAuditStore,
			*SQLiteRecoveryPointStore,
			*SQLiteApprovalRequestStore,
			*SQLiteAuthorizationRecordStore,
			*SQLiteMirrorConversationStore:
			return true
		}
	}
	return false
}

func clearInMemoryTaskHistoryStores(s *Service) {
	if s == nil {
		return
	}
	if store, ok := s.taskRunStore.(*InMemoryTaskRunStore); ok && store != nil {
		store.mu.Lock()
		store.records = make(map[string]TaskRunRecord)
		store.mu.Unlock()
	}
	if store, ok := s.sessionStore.(*inMemorySessionStore); ok && store != nil {
		store.mu.Lock()
		store.records = make(map[string]SessionRecord)
		store.mu.Unlock()
	}
	if store, ok := s.taskStore.(*inMemoryTaskStore); ok && store != nil {
		store.mu.Lock()
		store.records = make(map[string]TaskRecord)
		store.mu.Unlock()
	}
	if store, ok := s.taskStepStore.(*inMemoryTaskStepStore); ok && store != nil {
		store.mu.Lock()
		store.records = make(map[string][]TaskStepRecord)
		store.mu.Unlock()
	}
	if store, ok := s.loopRuntimeStore.(*inMemoryLoopRuntimeStore); ok && store != nil {
		store.mu.Lock()
		store.runs = map[string]RunRecord{}
		store.steps = map[string]StepRecord{}
		store.events = []EventRecord{}
		store.deliveryResults = map[string]DeliveryResultRecord{}
		store.citations = map[string][]CitationRecord{}
		store.mu.Unlock()
	}
	if store, ok := s.toolCallStore.(*inMemoryToolCallStore); ok && store != nil {
		store.mu.Lock()
		store.records = []tools.ToolCallRecord{}
		store.mu.Unlock()
	}
	if store, ok := s.artifactStore.(*inMemoryArtifactStore); ok && store != nil {
		store.mu.Lock()
		store.records = []ArtifactRecord{}
		store.mu.Unlock()
	}
	if store, ok := s.auditStore.(*inMemoryAuditStore); ok && store != nil {
		store.mu.Lock()
		store.records = []audit.Record{}
		store.mu.Unlock()
	}
	if store, ok := s.recoveryPointStore.(*inMemoryRecoveryPointStore); ok && store != nil {
		store.mu.Lock()
		store.points = []checkpoint.RecoveryPoint{}
		store.mu.Unlock()
	}
	if store, ok := s.approvalRequestStore.(*inMemoryApprovalRequestStore); ok && store != nil && store.state != nil {
		store.state.mu.Lock()
		store.state.approvalRequests = []ApprovalRequestRecord{}
		store.state.mu.Unlock()
	}
	if store, ok := s.authorizationRecordStore.(*inMemoryAuthorizationRecordStore); ok && store != nil && store.state != nil {
		store.state.mu.Lock()
		store.state.authorizationRecords = []AuthorizationRecordRecord{}
		store.state.mu.Unlock()
	}
	if store, ok := s.mirrorConversationStore.(*inMemoryMirrorConversationStore); ok && store != nil {
		store.mu.Lock()
		store.records = []MirrorConversationRecord{}
		store.mu.Unlock()
	}
}

func clearInMemoryMemoryStore(store *InMemoryMemoryStore) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.summaries = []MemorySummaryRecord{}
	store.retrievalHits = []MemoryRetrievalRecord{}
}

func deleteSQLiteTables(ctx context.Context, databasePath string, tableNames ...string) error {
	databasePath = strings.TrimSpace(databasePath)
	if databasePath == "" {
		return ErrDatabasePathRequired
	}
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin cleanup transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	for _, tableName := range tableNames {
		exists, err := sqliteTableExists(ctx, tx, tableName)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		statement := fmt.Sprintf("DELETE FROM %s", tableName)
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("delete rows from %s: %w", tableName, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit cleanup transaction: %w", err)
	}
	committed = true
	return nil
}

func sqliteTableExists(ctx context.Context, tx *sql.Tx, tableName string) (bool, error) {
	if tx == nil {
		return false, errors.New("sqlite transaction is required")
	}
	var name string
	err := tx.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, tableName).Scan(&name)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, fmt.Errorf("lookup sqlite table %s: %w", tableName, err)
}
