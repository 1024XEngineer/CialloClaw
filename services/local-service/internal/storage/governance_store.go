package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
)

var ErrApprovalRequestNotFound = errors.New("approval request not found")

type inMemoryGovernanceState struct {
	mu                   sync.Mutex
	approvalRequests     []ApprovalRequestRecord
	authorizationRecords []AuthorizationRecordRecord
}

type inMemoryApprovalRequestStore struct {
	state *inMemoryGovernanceState
}

func newInMemoryApprovalRequestStore() *inMemoryApprovalRequestStore {
	return newInMemoryApprovalRequestStoreWithState(&inMemoryGovernanceState{
		approvalRequests:     make([]ApprovalRequestRecord, 0),
		authorizationRecords: make([]AuthorizationRecordRecord, 0),
	})
}

func newInMemoryApprovalRequestStoreWithState(state *inMemoryGovernanceState) *inMemoryApprovalRequestStore {
	return &inMemoryApprovalRequestStore{state: state}
}

func (s *inMemoryApprovalRequestStore) WriteApprovalRequest(_ context.Context, record ApprovalRequestRecord) error {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	for i := range s.state.approvalRequests {
		if s.state.approvalRequests[i].ApprovalID == record.ApprovalID {
			s.state.approvalRequests[i] = record
			return nil
		}
	}
	s.state.approvalRequests = append(s.state.approvalRequests, record)
	return nil
}

func (s *inMemoryApprovalRequestStore) UpdateApprovalRequestStatus(_ context.Context, approvalID string, status string, updatedAt string) error {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	return updateInMemoryApprovalRequestStatusLocked(s.state.approvalRequests, approvalID, status, updatedAt)
}

func updateInMemoryApprovalRequestStatusLocked(records []ApprovalRequestRecord, approvalID string, status string, updatedAt string) error {
	for i := range records {
		if records[i].ApprovalID == approvalID {
			records[i].Status = status
			if updatedAt != "" {
				records[i].UpdatedAt = updatedAt
			}
			return nil
		}
	}
	return ErrApprovalRequestNotFound
}

func (s *inMemoryApprovalRequestStore) ListApprovalRequests(_ context.Context, taskID string, limit, offset int) ([]ApprovalRequestRecord, int, error) {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	items := filterApprovalRequests(s.state.approvalRequests, taskID, "")
	return pageApprovalRequests(items, limit, offset), len(items), nil
}

func (s *inMemoryApprovalRequestStore) ListPendingApprovalRequests(_ context.Context, limit, offset int) ([]ApprovalRequestRecord, int, error) {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	items := filterApprovalRequests(s.state.approvalRequests, "", "pending")
	items = latestApprovalRequestPerTask(items)
	return pageApprovalRequests(items, limit, offset), len(items), nil
}

type inMemoryAuthorizationRecordStore struct {
	state *inMemoryGovernanceState
}

func newInMemoryAuthorizationRecordStore() *inMemoryAuthorizationRecordStore {
	return newInMemoryAuthorizationRecordStoreWithState(&inMemoryGovernanceState{
		approvalRequests:     make([]ApprovalRequestRecord, 0),
		authorizationRecords: make([]AuthorizationRecordRecord, 0),
	})
}

func newInMemoryAuthorizationRecordStoreWithState(state *inMemoryGovernanceState) *inMemoryAuthorizationRecordStore {
	return &inMemoryAuthorizationRecordStore{state: state}
}

func (s *inMemoryAuthorizationRecordStore) WriteAuthorizationRecord(_ context.Context, record AuthorizationRecordRecord) error {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	s.state.authorizationRecords = append(s.state.authorizationRecords, record)
	return nil
}

func (s *inMemoryAuthorizationRecordStore) WriteAuthorizationDecision(_ context.Context, record AuthorizationRecordRecord, approvalStatus string, updatedAt string) error {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	if err := updateInMemoryApprovalRequestStatusLocked(s.state.approvalRequests, record.ApprovalID, approvalStatus, updatedAt); err != nil {
		return err
	}
	s.state.authorizationRecords = append(s.state.authorizationRecords, record)
	return nil
}

func (s *inMemoryAuthorizationRecordStore) ListAuthorizationRecords(_ context.Context, taskID, runID string, limit, offset int) ([]AuthorizationRecordRecord, int, error) {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	items := make([]AuthorizationRecordRecord, 0)
	for _, record := range s.state.authorizationRecords {
		if taskID != "" && record.TaskID != taskID {
			continue
		}
		if runID != "" && record.RunID != runID {
			continue
		}
		items = append(items, record)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return parseGovernanceTime(items[i].CreatedAt).After(parseGovernanceTime(items[j].CreatedAt))
	})
	return pageAuthorizationRecords(items, limit, offset), len(items), nil
}

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

func (s *inMemoryAuditStore) ListAuditRecords(_ context.Context, taskID, runID string, limit, offset int) ([]audit.Record, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]audit.Record, 0)
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
		return parseGovernanceTime(items[i].CreatedAt).After(parseGovernanceTime(items[j].CreatedAt))
	})
	return pageAuditRecords(items, limit, offset), len(items), nil
}

type inMemoryRecoveryPointStore struct {
	mu     sync.Mutex
	points []checkpoint.RecoveryPoint
}

var ErrRecoveryPointNotFound = errors.New("recovery point not found")

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

func (s *inMemoryRecoveryPointStore) GetRecoveryPoint(_ context.Context, recoveryPointID string) (checkpoint.RecoveryPoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, point := range s.points {
		if point.RecoveryPointID == recoveryPointID {
			return point, nil
		}
	}
	return checkpoint.RecoveryPoint{}, ErrRecoveryPointNotFound
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
		`INSERT OR REPLACE INTO audit_records (audit_id, task_id, run_id, type, action, summary, target, result, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.AuditID,
		record.TaskID,
		nullableRuntimeString(record.RunID),
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

func (s *SQLiteAuditStore) ListAuditRecords(ctx context.Context, taskID, runID string, limit, offset int) ([]audit.Record, int, error) {
	countQuery := `SELECT COUNT(1) FROM audit_records`
	query := `SELECT audit_id, task_id, COALESCE(run_id, ''), type, action, summary, target, result, created_at FROM audit_records`
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
	query += ` ORDER BY created_at DESC, audit_id DESC`
	args := append([]any(nil), filterArgs...)
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}

	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, filterArgs...).Scan(&total); err != nil {
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
		if err := rows.Scan(&record.AuditID, &record.TaskID, &record.RunID, &record.Type, &record.Action, &record.Summary, &record.Target, &record.Result, &record.CreatedAt); err != nil {
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
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=300000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS audit_records (
			audit_id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			run_id TEXT,
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
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE audit_records ADD COLUMN run_id TEXT;`); err != nil && !isSQLiteDuplicateColumnError(err) {
		return fmt.Errorf("add audit_records run_id column: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_audit_records_task_run_time ON audit_records(task_id, run_id, created_at DESC);`); err != nil {
		return fmt.Errorf("create audit task run index: %w", err)
	}
	return nil
}

type SQLiteRecoveryPointStore struct {
	db *sql.DB
}

type SQLiteApprovalRequestStore struct {
	db *sql.DB
}

func NewSQLiteApprovalRequestStore(databasePath string) (*SQLiteApprovalRequestStore, error) {
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteApprovalRequestStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteApprovalRequestStore) WriteApprovalRequest(ctx context.Context, record ApprovalRequestRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO approval_requests (
			approval_id, task_id, operation_name, risk_level, target_object, reason, status, impact_scope_json, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, record.ApprovalID, record.TaskID, record.OperationName, record.RiskLevel, record.TargetObject, record.Reason, record.Status, record.ImpactScopeJSON, record.CreatedAt, record.UpdatedAt)
	if err != nil {
		return fmt.Errorf("write approval request: %w", err)
	}
	return nil
}

func (s *SQLiteApprovalRequestStore) UpdateApprovalRequestStatus(ctx context.Context, approvalID string, status string, updatedAt string) error {
	if approvalID == "" {
		return nil
	}
	if updatedAt == "" {
		updatedAt = time.Now().Format(time.RFC3339Nano)
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE approval_requests
		SET status = ?, updated_at = ?
		WHERE approval_id = ?
	`, status, updatedAt, approvalID)
	if err != nil {
		return fmt.Errorf("update approval request status: %w", err)
	}
	return nil
}

func (s *SQLiteApprovalRequestStore) ListApprovalRequests(ctx context.Context, taskID string, limit, offset int) ([]ApprovalRequestRecord, int, error) {
	items, total, err := s.listApprovalRequests(ctx, taskID, "", limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *SQLiteApprovalRequestStore) ListPendingApprovalRequests(ctx context.Context, limit, offset int) ([]ApprovalRequestRecord, int, error) {
	items, total, err := s.listLatestPendingApprovalRequests(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *SQLiteApprovalRequestStore) listApprovalRequests(ctx context.Context, taskID string, status string, limit, offset int) ([]ApprovalRequestRecord, int, error) {
	countQuery := `SELECT COUNT(1) FROM approval_requests`
	query := `SELECT approval_id, task_id, operation_name, risk_level, target_object, reason, status, impact_scope_json, created_at, updated_at FROM approval_requests`
	where := make([]string, 0, 2)
	countArgs := make([]any, 0, 2)
	queryArgs := make([]any, 0, 4)
	if taskID != "" {
		where = append(where, `task_id = ?`)
		countArgs = append(countArgs, taskID)
		queryArgs = append(queryArgs, taskID)
	}
	if status != "" {
		where = append(where, `status = ?`)
		countArgs = append(countArgs, status)
		queryArgs = append(queryArgs, status)
	}
	if len(where) > 0 {
		clause := " WHERE " + strings.Join(where, " AND ")
		countQuery += clause
		query += clause
	}
	query += ` ORDER BY created_at DESC, approval_id DESC`
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		queryArgs = append(queryArgs, limit, offset)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count approval requests: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list approval requests: %w", err)
	}
	defer rows.Close()
	items := make([]ApprovalRequestRecord, 0)
	for rows.Next() {
		var record ApprovalRequestRecord
		if err := rows.Scan(&record.ApprovalID, &record.TaskID, &record.OperationName, &record.RiskLevel, &record.TargetObject, &record.Reason, &record.Status, &record.ImpactScopeJSON, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan approval request: %w", err)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate approval requests: %w", err)
	}
	return items, total, nil
}

func (s *SQLiteApprovalRequestStore) listLatestPendingApprovalRequests(ctx context.Context, limit, offset int) ([]ApprovalRequestRecord, int, error) {
	countQuery := `
		SELECT COUNT(1)
		FROM approval_requests current
		WHERE current.status = 'pending'
			AND NOT EXISTS (
				SELECT 1
				FROM approval_requests newer
				WHERE newer.status = 'pending'
					AND newer.task_id = current.task_id
					AND (
						newer.created_at > current.created_at
						OR (newer.created_at = current.created_at AND newer.approval_id > current.approval_id)
					)
			)
	`
	query := `
		SELECT current.approval_id, current.task_id, current.operation_name, current.risk_level, current.target_object, current.reason, current.status, current.impact_scope_json, current.created_at, current.updated_at
		FROM approval_requests current
		WHERE current.status = 'pending'
			AND NOT EXISTS (
				SELECT 1
				FROM approval_requests newer
				WHERE newer.status = 'pending'
					AND newer.task_id = current.task_id
					AND (
						newer.created_at > current.created_at
						OR (newer.created_at = current.created_at AND newer.approval_id > current.approval_id)
					)
			)
		ORDER BY current.created_at DESC, current.approval_id DESC
	`
	queryArgs := make([]any, 0, 2)
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		queryArgs = append(queryArgs, limit, offset)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count latest pending approval requests: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list latest pending approval requests: %w", err)
	}
	defer rows.Close()
	items := make([]ApprovalRequestRecord, 0)
	for rows.Next() {
		var record ApprovalRequestRecord
		if err := rows.Scan(&record.ApprovalID, &record.TaskID, &record.OperationName, &record.RiskLevel, &record.TargetObject, &record.Reason, &record.Status, &record.ImpactScopeJSON, &record.CreatedAt, &record.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan latest pending approval request: %w", err)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate latest pending approval requests: %w", err)
	}
	return items, total, nil
}

func (s *SQLiteApprovalRequestStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteApprovalRequestStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=300000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS approval_requests (
			approval_id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			operation_name TEXT NOT NULL,
			risk_level TEXT NOT NULL,
			target_object TEXT NOT NULL,
			reason TEXT NOT NULL,
			status TEXT NOT NULL,
			impact_scope_json TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create approval_requests table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_approval_requests_task_status ON approval_requests(task_id, status);`); err != nil {
		return fmt.Errorf("create approval_requests index: %w", err)
	}
	return nil
}

type SQLiteAuthorizationRecordStore struct {
	db *sql.DB
}

func NewSQLiteAuthorizationRecordStore(databasePath string) (*SQLiteAuthorizationRecordStore, error) {
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteAuthorizationRecordStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteAuthorizationRecordStore) WriteAuthorizationRecord(ctx context.Context, record AuthorizationRecordRecord) error {
	rememberRule := 0
	if record.RememberRule {
		rememberRule = 1
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO authorization_records (
			authorization_record_id, task_id, run_id, approval_id, decision, operator, remember_rule, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, record.AuthorizationRecordID, record.TaskID, nullableRuntimeString(record.RunID), record.ApprovalID, record.Decision, record.Operator, rememberRule, record.CreatedAt)
	if err != nil {
		return fmt.Errorf("write authorization record: %w", err)
	}
	return nil
}

func (s *SQLiteAuthorizationRecordStore) WriteAuthorizationDecision(ctx context.Context, record AuthorizationRecordRecord, approvalStatus string, updatedAt string) error {
	if strings.TrimSpace(record.ApprovalID) == "" {
		return ErrApprovalRequestNotFound
	}
	if updatedAt == "" {
		updatedAt = record.CreatedAt
	}
	rememberRule := 0
	if record.RememberRule {
		rememberRule = 1
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin authorization decision transaction: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	result, err := tx.ExecContext(ctx, `
		UPDATE approval_requests
		SET status = ?, updated_at = ?
		WHERE approval_id = ?
	`, approvalStatus, updatedAt, record.ApprovalID)
	if err != nil {
		return fmt.Errorf("update approval request status: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read approval request update result: %w", err)
	}
	if rowsAffected == 0 {
		return ErrApprovalRequestNotFound
	}
	if _, err := tx.ExecContext(ctx, `
			INSERT OR REPLACE INTO authorization_records (
				authorization_record_id, task_id, run_id, approval_id, decision, operator, remember_rule, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, record.AuthorizationRecordID, record.TaskID, nullableRuntimeString(record.RunID), record.ApprovalID, record.Decision, record.Operator, rememberRule, record.CreatedAt); err != nil {
		return fmt.Errorf("write authorization record: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit authorization decision transaction: %w", err)
	}
	committed = true
	return nil
}

func (s *SQLiteAuthorizationRecordStore) ListAuthorizationRecords(ctx context.Context, taskID, runID string, limit, offset int) ([]AuthorizationRecordRecord, int, error) {
	countQuery := `SELECT COUNT(1) FROM authorization_records`
	query := `SELECT authorization_record_id, task_id, COALESCE(run_id, ''), approval_id, decision, operator, remember_rule, created_at FROM authorization_records`
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
	query += ` ORDER BY created_at DESC, authorization_record_id DESC`
	args := append([]any(nil), filterArgs...)
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, filterArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count authorization records: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list authorization records: %w", err)
	}
	defer rows.Close()
	items := make([]AuthorizationRecordRecord, 0)
	for rows.Next() {
		var record AuthorizationRecordRecord
		var rememberRule int
		if err := rows.Scan(&record.AuthorizationRecordID, &record.TaskID, &record.RunID, &record.ApprovalID, &record.Decision, &record.Operator, &rememberRule, &record.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan authorization record: %w", err)
		}
		record.RememberRule = rememberRule != 0
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate authorization records: %w", err)
	}
	return items, total, nil
}

func (s *SQLiteAuthorizationRecordStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteAuthorizationRecordStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=300000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS authorization_records (
			authorization_record_id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			run_id TEXT,
			approval_id TEXT NOT NULL,
			decision TEXT NOT NULL,
			operator TEXT NOT NULL,
			remember_rule INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create authorization_records table: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE authorization_records ADD COLUMN run_id TEXT;`); err != nil && !isSQLiteDuplicateColumnError(err) {
		return fmt.Errorf("add authorization_records run_id column: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_authorization_records_task_time ON authorization_records(task_id, created_at DESC);`); err != nil {
		return fmt.Errorf("create authorization_records index: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_authorization_records_task_run_time ON authorization_records(task_id, run_id, created_at DESC);`); err != nil {
		return fmt.Errorf("create authorization_records task run index: %w", err)
	}
	return nil
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

func pageApprovalRequests(items []ApprovalRequestRecord, limit, offset int) []ApprovalRequestRecord {
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
	return append([]ApprovalRequestRecord(nil), items[offset:end]...)
}

func pageAuthorizationRecords(items []AuthorizationRecordRecord, limit, offset int) []AuthorizationRecordRecord {
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
	return append([]AuthorizationRecordRecord(nil), items[offset:end]...)
}

func filterApprovalRequests(records []ApprovalRequestRecord, taskID string, status string) []ApprovalRequestRecord {
	items := make([]ApprovalRequestRecord, 0)
	for _, record := range records {
		if taskID != "" && record.TaskID != taskID {
			continue
		}
		if status != "" && record.Status != status {
			continue
		}
		items = append(items, record)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return parseGovernanceTime(items[i].CreatedAt).After(parseGovernanceTime(items[j].CreatedAt))
	})
	return items
}

func latestApprovalRequestPerTask(records []ApprovalRequestRecord) []ApprovalRequestRecord {
	items := make([]ApprovalRequestRecord, 0, len(records))
	seenTasks := make(map[string]struct{}, len(records))
	for _, record := range records {
		key := strings.TrimSpace(record.TaskID)
		if key == "" {
			key = strings.TrimSpace(record.ApprovalID)
		}
		if _, ok := seenTasks[key]; ok {
			continue
		}
		seenTasks[key] = struct{}{}
		items = append(items, record)
	}
	return items
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

func (s *SQLiteRecoveryPointStore) GetRecoveryPoint(ctx context.Context, recoveryPointID string) (checkpoint.RecoveryPoint, error) {
	row := s.db.QueryRowContext(ctx, `SELECT recovery_point_id, task_id, summary, created_at, objects_json FROM recovery_points WHERE recovery_point_id = ?`, recoveryPointID)
	var point checkpoint.RecoveryPoint
	var objectsJSON string
	if err := row.Scan(&point.RecoveryPointID, &point.TaskID, &point.Summary, &point.CreatedAt, &objectsJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return checkpoint.RecoveryPoint{}, ErrRecoveryPointNotFound
		}
		return checkpoint.RecoveryPoint{}, fmt.Errorf("get recovery point: %w", err)
	}
	if err := json.Unmarshal([]byte(objectsJSON), &point.Objects); err != nil {
		return checkpoint.RecoveryPoint{}, fmt.Errorf("unmarshal recovery point objects: %w", err)
	}
	return point, nil
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
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=300000;`); err != nil {
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
	return db, nil
}

func pageAuditRecords(items []audit.Record, limit, offset int) []audit.Record {
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
	return append([]audit.Record(nil), items[offset:end]...)
}

func pageRecoveryPoints(items []checkpoint.RecoveryPoint, limit, offset int) []checkpoint.RecoveryPoint {
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
