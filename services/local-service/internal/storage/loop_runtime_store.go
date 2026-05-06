package storage

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// RunRecord persists one normalized run snapshot alongside the task snapshot.
type RunRecord struct {
	RunID      string
	TaskID     string
	SessionID  string
	SourceType string
	Status     string
	IntentName string
	StartedAt  string
	UpdatedAt  string
	FinishedAt string
	StopReason string
}

// StepRecord persists one normalized loop step snapshot.
type StepRecord struct {
	StepID        string
	RunID         string
	TaskID        string
	OrderIndex    int
	AttemptIndex  int
	SegmentKind   string
	LoopRound     int
	Name          string
	Status        string
	InputSummary  string
	OutputSummary string
	StopReason    string
	StartedAt     string
	CompletedAt   string
	PlannerInput  string
	PlannerOutput string
	Observation   string
	ToolName      string
	ToolCallID    string
}

// EventRecord persists one normalized compatibility event.
type EventRecord struct {
	EventID     string
	RunID       string
	TaskID      string
	StepID      string
	Type        string
	Level       string
	PayloadJSON string
	CreatedAt   string
}

// DeliveryResultRecord persists one delivery_result snapshot outside task_runs.
type DeliveryResultRecord struct {
	DeliveryResultID string
	TaskID           string
	RunID            string
	Type             string
	Title            string
	PayloadJSON      string
	PreviewText      string
	CreatedAt        string
}

// CitationRecord persists one formal citation snapshot outside task_runs.
type CitationRecord struct {
	CitationID      string
	TaskID          string
	RunID           string
	SourceType      string
	SourceRef       string
	Label           string
	ArtifactID      string
	ArtifactType    string
	EvidenceRole    string
	ExcerptText     string
	ScreenSessionID string
	OrderIndex      int
}

type inMemoryLoopRuntimeStore struct {
	mu              sync.Mutex
	runs            map[string]RunRecord
	steps           map[string]StepRecord
	events          []EventRecord
	deliveryResults map[string]DeliveryResultRecord
	citations       map[string][]CitationRecord
}

func newInMemoryLoopRuntimeStore() *inMemoryLoopRuntimeStore {
	return &inMemoryLoopRuntimeStore{
		runs:            map[string]RunRecord{},
		steps:           map[string]StepRecord{},
		events:          []EventRecord{},
		deliveryResults: map[string]DeliveryResultRecord{},
		citations:       map[string][]CitationRecord{},
	}
}

func (s *inMemoryLoopRuntimeStore) SaveRun(_ context.Context, record RunRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[record.RunID] = record
	return nil
}

func (s *inMemoryLoopRuntimeStore) SaveSteps(_ context.Context, records []StepRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, record := range records {
		s.steps[record.StepID] = record
	}
	return nil
}

func (s *inMemoryLoopRuntimeStore) SaveEvents(_ context.Context, records []EventRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, records...)
	return nil
}

func (s *inMemoryLoopRuntimeStore) SaveDeliveryResult(_ context.Context, record DeliveryResultRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliveryResults[record.DeliveryResultID] = record
	return nil
}

func (s *inMemoryLoopRuntimeStore) GetRun(_ context.Context, runID string) (RunRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.runs[runID]
	if !ok {
		return RunRecord{}, sql.ErrNoRows
	}
	return record, nil
}

func (s *inMemoryLoopRuntimeStore) ListDeliveryResults(_ context.Context, taskID, runID string, limit, offset int) ([]DeliveryResultRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]DeliveryResultRecord, 0, len(s.deliveryResults))
	for _, record := range s.deliveryResults {
		if taskID != "" && record.TaskID != taskID {
			continue
		}
		if runID != "" && record.RunID != runID {
			continue
		}
		items = append(items, record)
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt > items[j].CreatedAt
	})
	total := len(items)
	if offset >= total {
		return []DeliveryResultRecord{}, total, nil
	}
	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}
	return append([]DeliveryResultRecord(nil), items[offset:end]...), total, nil
}

func (s *inMemoryLoopRuntimeStore) ReplaceTaskCitations(_ context.Context, taskID string, records []CitationRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cloned := make([]CitationRecord, 0, len(records))
	for _, record := range records {
		cloned = append(cloned, record)
	}
	s.citations[taskID] = cloned
	return nil
}

func (s *inMemoryLoopRuntimeStore) GetLatestDeliveryResult(_ context.Context, taskID, runID string) (DeliveryResultRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	latest := DeliveryResultRecord{}
	found := false
	for _, record := range s.deliveryResults {
		if taskID != "" && record.TaskID != taskID {
			continue
		}
		if runID != "" && record.RunID != runID {
			continue
		}
		if !found || parseGovernanceTime(record.CreatedAt).After(parseGovernanceTime(latest.CreatedAt)) {
			latest = record
			found = true
		}
	}
	return latest, found, nil
}

func (s *inMemoryLoopRuntimeStore) ListTaskCitations(_ context.Context, taskID, runID string) ([]CitationRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	source := s.citations[taskID]
	records := make([]CitationRecord, 0, len(source))
	for _, record := range source {
		if runID != "" && record.RunID != runID {
			continue
		}
		records = append(records, record)
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].OrderIndex == records[j].OrderIndex {
			return records[i].CitationID < records[j].CitationID
		}
		return records[i].OrderIndex < records[j].OrderIndex
	})
	return records, nil
}

func (s *inMemoryLoopRuntimeStore) ListEvents(_ context.Context, taskID, runID, eventType, createdAtFrom, createdAtTo string, limit, offset int) ([]EventRecord, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fromTime := parseGovernanceTime(createdAtFrom)
	toTime := parseGovernanceTime(createdAtTo)
	filtered := make([]EventRecord, 0, len(s.events))
	for _, record := range s.events {
		if taskID != "" && record.TaskID != taskID {
			continue
		}
		if runID != "" && record.RunID != runID {
			continue
		}
		if eventType != "" && record.Type != eventType {
			continue
		}
		recordTime := parseGovernanceTime(record.CreatedAt)
		if !fromTime.IsZero() && recordTime.Before(fromTime) {
			continue
		}
		if !toTime.IsZero() && recordTime.After(toTime) {
			continue
		}
		filtered = append(filtered, record)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		return parseGovernanceTime(filtered[i].CreatedAt).After(parseGovernanceTime(filtered[j].CreatedAt))
	})
	if offset >= len(filtered) {
		return []EventRecord{}, len(filtered), nil
	}
	end := offset + limit
	if limit <= 0 || end > len(filtered) {
		end = len(filtered)
	}
	return append([]EventRecord(nil), filtered[offset:end]...), len(filtered), nil
}

type SQLiteLoopRuntimeStore struct {
	db *sql.DB
}

func NewSQLiteLoopRuntimeStore(databasePath string) (*SQLiteLoopRuntimeStore, error) {
	db, err := openSQLiteDatabase(databasePath)
	if err != nil {
		return nil, err
	}
	store := &SQLiteLoopRuntimeStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *SQLiteLoopRuntimeStore) SaveRun(ctx context.Context, record RunRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO runs (run_id, task_id, session_id, source_type, status, intent_name, started_at, updated_at, finished_at, stop_reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, record.RunID, record.TaskID, record.SessionID, strings.TrimSpace(record.SourceType), record.Status, record.IntentName, record.StartedAt, record.UpdatedAt, nullableRuntimeString(record.FinishedAt), nullableRuntimeString(record.StopReason))
	if err != nil {
		return fmt.Errorf("write run record: %w", err)
	}
	return nil
}

func (s *SQLiteLoopRuntimeStore) SaveSteps(ctx context.Context, records []StepRecord) error {
	for _, record := range records {
		_, err := s.db.ExecContext(ctx, `
			INSERT OR REPLACE INTO steps (step_id, run_id, task_id, order_index, attempt_index, segment_kind, loop_round, name, status, input_summary, output_summary, stop_reason, started_at, completed_at, planner_input, planner_output, observation, tool_name, tool_call_id)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, record.StepID, record.RunID, record.TaskID, record.OrderIndex, record.AttemptIndex, nullableRuntimeString(record.SegmentKind), record.LoopRound, record.Name, record.Status, record.InputSummary, record.OutputSummary, nullableRuntimeString(record.StopReason), record.StartedAt, nullableRuntimeString(record.CompletedAt), record.PlannerInput, record.PlannerOutput, record.Observation, nullableRuntimeString(record.ToolName), nullableRuntimeString(record.ToolCallID))
		if err != nil {
			return fmt.Errorf("write step record %s: %w", record.StepID, err)
		}
	}
	return nil
}

func (s *SQLiteLoopRuntimeStore) SaveEvents(ctx context.Context, records []EventRecord) error {
	for _, record := range records {
		_, err := s.db.ExecContext(ctx, `
			INSERT OR REPLACE INTO events (event_id, run_id, task_id, step_id, type, level, payload_json, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, record.EventID, record.RunID, record.TaskID, nullableRuntimeString(record.StepID), record.Type, record.Level, record.PayloadJSON, record.CreatedAt)
		if err != nil {
			return fmt.Errorf("write event record %s: %w", record.EventID, err)
		}
	}
	return nil
}

func (s *SQLiteLoopRuntimeStore) SaveDeliveryResult(ctx context.Context, record DeliveryResultRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO delivery_results (delivery_result_id, task_id, run_id, type, title, payload_json, preview_text, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, record.DeliveryResultID, record.TaskID, nullableRuntimeString(record.RunID), record.Type, record.Title, record.PayloadJSON, nullableRuntimeString(record.PreviewText), record.CreatedAt)
	if err != nil {
		return fmt.Errorf("write delivery_result record: %w", err)
	}
	return nil
}

func (s *SQLiteLoopRuntimeStore) GetRun(ctx context.Context, runID string) (RunRecord, error) {
	var record RunRecord
	err := s.db.QueryRowContext(ctx, `SELECT run_id, task_id, session_id, COALESCE(source_type, ''), status, intent_name, started_at, updated_at, COALESCE(finished_at, ''), COALESCE(stop_reason, '') FROM runs WHERE run_id = ?`, runID).Scan(&record.RunID, &record.TaskID, &record.SessionID, &record.SourceType, &record.Status, &record.IntentName, &record.StartedAt, &record.UpdatedAt, &record.FinishedAt, &record.StopReason)
	if err != nil {
		return RunRecord{}, err
	}
	return record, nil
}

func (s *SQLiteLoopRuntimeStore) ListDeliveryResults(ctx context.Context, taskID, runID string, limit, offset int) ([]DeliveryResultRecord, int, error) {
	filters := make([]string, 0, 2)
	filterArgs := make([]any, 0, 2)
	if strings.TrimSpace(taskID) != "" {
		filters = append(filters, `task_id = ?`)
		filterArgs = append(filterArgs, taskID)
	}
	if strings.TrimSpace(runID) != "" {
		filters = append(filters, `run_id = ?`)
		filterArgs = append(filterArgs, runID)
	}
	countQuery := `SELECT COUNT(1) FROM delivery_results`
	query := `SELECT delivery_result_id, task_id, COALESCE(run_id, ''), type, title, payload_json, COALESCE(preview_text, ''), created_at FROM delivery_results`
	if len(filters) > 0 {
		whereClause := ` WHERE ` + strings.Join(filters, ` AND `)
		countQuery += whereClause
		query += whereClause
	}
	query += ` ORDER BY created_at DESC, delivery_result_id DESC`
	args := append([]any(nil), filterArgs...)
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, filterArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count delivery results: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list delivery results: %w", err)
	}
	defer rows.Close()
	items := make([]DeliveryResultRecord, 0)
	for rows.Next() {
		var record DeliveryResultRecord
		if err := rows.Scan(&record.DeliveryResultID, &record.TaskID, &record.RunID, &record.Type, &record.Title, &record.PayloadJSON, &record.PreviewText, &record.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan delivery result: %w", err)
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate delivery results: %w", err)
	}
	return items, total, nil
}

func (s *SQLiteLoopRuntimeStore) ReplaceTaskCitations(ctx context.Context, taskID string, records []CitationRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin task citation replace: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM task_citations WHERE task_id = ?`, taskID); err != nil {
		return fmt.Errorf("delete task citations: %w", err)
	}
	for _, record := range records {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO task_citations (
				citation_id, task_id, run_id, source_type, source_ref, label,
				artifact_id, artifact_type, evidence_role, excerpt_text, screen_session_id, order_index
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, record.CitationID, record.TaskID, nullableRuntimeString(record.RunID), record.SourceType, record.SourceRef, record.Label,
			nullableRuntimeString(record.ArtifactID), nullableRuntimeString(record.ArtifactType), nullableRuntimeString(record.EvidenceRole),
			nullableRuntimeString(record.ExcerptText), nullableRuntimeString(record.ScreenSessionID), record.OrderIndex); err != nil {
			return fmt.Errorf("insert task citation %s: %w", record.CitationID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit task citation replace: %w", err)
	}
	return nil
}

func (s *SQLiteLoopRuntimeStore) GetLatestDeliveryResult(ctx context.Context, taskID, runID string) (DeliveryResultRecord, bool, error) {
	filters := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if strings.TrimSpace(taskID) != "" {
		filters = append(filters, `task_id = ?`)
		args = append(args, taskID)
	}
	if strings.TrimSpace(runID) != "" {
		filters = append(filters, `run_id = ?`)
		args = append(args, runID)
	}
	query := `
		SELECT delivery_result_id, task_id, COALESCE(run_id, ''), type, title, payload_json, preview_text, created_at
		FROM delivery_results
	`
	if len(filters) > 0 {
		query += ` WHERE ` + strings.Join(filters, ` AND `)
	}
	query += ` ORDER BY created_at DESC, delivery_result_id DESC LIMIT 1`
	row := s.db.QueryRowContext(ctx, query, args...)
	var record DeliveryResultRecord
	var previewText sql.NullString
	if err := row.Scan(&record.DeliveryResultID, &record.TaskID, &record.RunID, &record.Type, &record.Title, &record.PayloadJSON, &previewText, &record.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return DeliveryResultRecord{}, false, nil
		}
		return DeliveryResultRecord{}, false, fmt.Errorf("get latest delivery_result: %w", err)
	}
	record.PreviewText = previewText.String
	return record, true, nil
}

func (s *SQLiteLoopRuntimeStore) ListTaskCitations(ctx context.Context, taskID, runID string) ([]CitationRecord, error) {
	filters := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if strings.TrimSpace(taskID) != "" {
		filters = append(filters, `task_id = ?`)
		args = append(args, taskID)
	}
	if strings.TrimSpace(runID) != "" {
		filters = append(filters, `run_id = ?`)
		args = append(args, runID)
	}
	query := `
		SELECT citation_id, task_id, run_id, source_type, source_ref, label,
		       artifact_id, artifact_type, evidence_role, excerpt_text, screen_session_id, order_index
		FROM task_citations
	`
	if len(filters) > 0 {
		query += ` WHERE ` + strings.Join(filters, ` AND `)
	}
	query += ` ORDER BY order_index ASC, citation_id ASC`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list task citations: %w", err)
	}
	defer rows.Close()
	items := make([]CitationRecord, 0)
	for rows.Next() {
		var record CitationRecord
		var runID, artifactID, artifactType, evidenceRole, excerptText, screenSessionID sql.NullString
		if err := rows.Scan(
			&record.CitationID,
			&record.TaskID,
			&runID,
			&record.SourceType,
			&record.SourceRef,
			&record.Label,
			&artifactID,
			&artifactType,
			&evidenceRole,
			&excerptText,
			&screenSessionID,
			&record.OrderIndex,
		); err != nil {
			return nil, fmt.Errorf("scan task citation: %w", err)
		}
		record.RunID = runID.String
		record.ArtifactID = artifactID.String
		record.ArtifactType = artifactType.String
		record.EvidenceRole = evidenceRole.String
		record.ExcerptText = excerptText.String
		record.ScreenSessionID = screenSessionID.String
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task citations: %w", err)
	}
	return items, nil
}

func (s *SQLiteLoopRuntimeStore) ListEvents(ctx context.Context, taskID, runID, eventType, createdAtFrom, createdAtTo string, limit, offset int) ([]EventRecord, int, error) {
	filters := make([]string, 0, 5)
	filterArgs := make([]any, 0, 5)
	if strings.TrimSpace(taskID) != "" {
		filters = append(filters, `task_id = ?`)
		filterArgs = append(filterArgs, taskID)
	}
	if strings.TrimSpace(runID) != "" {
		filters = append(filters, `run_id = ?`)
		filterArgs = append(filterArgs, runID)
	}
	if strings.TrimSpace(eventType) != "" {
		filters = append(filters, `type = ?`)
		filterArgs = append(filterArgs, eventType)
	}
	if strings.TrimSpace(createdAtFrom) != "" {
		filters = append(filters, `created_at >= ?`)
		filterArgs = append(filterArgs, createdAtFrom)
	}
	if strings.TrimSpace(createdAtTo) != "" {
		filters = append(filters, `created_at <= ?`)
		filterArgs = append(filterArgs, createdAtTo)
	}
	countQuery := `SELECT COUNT(1) FROM events`
	query := `SELECT event_id, run_id, task_id, step_id, type, level, payload_json, created_at FROM events`
	if len(filters) > 0 {
		whereClause := ` WHERE ` + strings.Join(filters, ` AND `)
		countQuery += whereClause
		query += whereClause
	}
	query += ` ORDER BY created_at DESC, event_id DESC`
	args := append([]any(nil), filterArgs...)
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}
	var total int
	if err := s.db.QueryRowContext(ctx, countQuery, filterArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count events: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()
	items := make([]EventRecord, 0)
	for rows.Next() {
		var record EventRecord
		var stepID sql.NullString
		if err := rows.Scan(&record.EventID, &record.RunID, &record.TaskID, &stepID, &record.Type, &record.Level, &record.PayloadJSON, &record.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan event record: %w", err)
		}
		record.StepID = stepID.String
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate events: %w", err)
	}
	return items, total, nil
}

func (s *SQLiteLoopRuntimeStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=5000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS runs (run_id TEXT PRIMARY KEY, task_id TEXT NOT NULL, session_id TEXT NOT NULL, source_type TEXT NOT NULL DEFAULT '', status TEXT NOT NULL, intent_name TEXT NOT NULL, started_at TEXT NOT NULL, updated_at TEXT NOT NULL, finished_at TEXT, stop_reason TEXT);`,
		`ALTER TABLE runs ADD COLUMN source_type TEXT NOT NULL DEFAULT '';`,
		`CREATE INDEX IF NOT EXISTS idx_runs_task_time ON runs(task_id, started_at DESC);`,
		`CREATE TABLE IF NOT EXISTS steps (step_id TEXT PRIMARY KEY, run_id TEXT NOT NULL, task_id TEXT NOT NULL, order_index INTEGER NOT NULL, attempt_index INTEGER NOT NULL DEFAULT 1, segment_kind TEXT NOT NULL DEFAULT 'initial', loop_round INTEGER NOT NULL DEFAULT 0, name TEXT NOT NULL, status TEXT NOT NULL, input_summary TEXT, output_summary TEXT, stop_reason TEXT, started_at TEXT NOT NULL, completed_at TEXT, planner_input TEXT, planner_output TEXT, observation TEXT, tool_name TEXT, tool_call_id TEXT);`,
		`ALTER TABLE steps ADD COLUMN attempt_index INTEGER NOT NULL DEFAULT 1;`,
		`ALTER TABLE steps ADD COLUMN segment_kind TEXT NOT NULL DEFAULT 'initial';`,
		`CREATE INDEX IF NOT EXISTS idx_steps_run_order ON steps(run_id, order_index);`,
		`CREATE TABLE IF NOT EXISTS events (event_id TEXT PRIMARY KEY, run_id TEXT NOT NULL, task_id TEXT NOT NULL, step_id TEXT, type TEXT NOT NULL, level TEXT NOT NULL, payload_json TEXT NOT NULL, created_at TEXT NOT NULL);`,
		`CREATE INDEX IF NOT EXISTS idx_events_task_time ON events(task_id, created_at DESC);`,
		`CREATE TABLE IF NOT EXISTS delivery_results (delivery_result_id TEXT PRIMARY KEY, task_id TEXT NOT NULL, run_id TEXT, type TEXT NOT NULL, title TEXT NOT NULL, payload_json TEXT NOT NULL, preview_text TEXT, created_at TEXT NOT NULL);`,
		`ALTER TABLE delivery_results ADD COLUMN run_id TEXT;`,
		`CREATE INDEX IF NOT EXISTS idx_delivery_results_task_time ON delivery_results(task_id, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_delivery_results_task_run_time ON delivery_results(task_id, run_id, created_at DESC);`,
		`CREATE TABLE IF NOT EXISTS task_citations (citation_id TEXT PRIMARY KEY, task_id TEXT NOT NULL, run_id TEXT, source_type TEXT NOT NULL, source_ref TEXT NOT NULL, label TEXT NOT NULL, artifact_id TEXT, artifact_type TEXT, evidence_role TEXT, excerpt_text TEXT, screen_session_id TEXT, order_index INTEGER NOT NULL DEFAULT 0);`,
		`CREATE INDEX IF NOT EXISTS idx_task_citations_task_order ON task_citations(task_id, order_index ASC, citation_id ASC);`,
	}
	for _, statement := range statements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			if isSQLiteDuplicateColumnError(err) {
				continue
			}
			return fmt.Errorf("initialize loop runtime store: %w", err)
		}
	}
	return nil
}

func isSQLiteDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate column name")
}

func (s *SQLiteLoopRuntimeStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func nullableRuntimeString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
