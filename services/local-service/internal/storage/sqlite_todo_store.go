package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

const sqliteTodoTableName = "todo_items"
const sqliteRecurringRuleTableName = "recurring_rules"

// SQLiteTodoStore persists todo items and recurring rules in SQLite.
type SQLiteTodoStore struct {
	db *sql.DB
}

// NewSQLiteTodoStore creates and returns a SQLiteTodoStore.
func NewSQLiteTodoStore(databasePath string) (*SQLiteTodoStore, error) {
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

	store := &SQLiteTodoStore{db: db}
	if err := store.initialize(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// ReplaceTodoState atomically replaces the persisted todo and recurring state.
func (s *SQLiteTodoStore) ReplaceTodoState(ctx context.Context, items []TodoItemRecord, rules []RecurringRuleRecord) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin todo replace transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM recurring_rules`); err != nil {
		return fmt.Errorf("clear recurring rules: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM todo_items`); err != nil {
		return fmt.Errorf("clear todo items: %w", err)
	}

	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO todo_items (
				item_id, title, bucket, status, source_path, source_line, source_bucket, due_at, tags_json,
				agent_suggestion, note_text, prerequisite, planned_at, previous_bucket, previous_due_at, previous_status, ended_at,
				related_resources_json, linked_task_id, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, item.ItemID, item.Title, item.Bucket, item.Status, item.SourcePath, item.SourceLine, nullableString(item.SourceBucket), nullableString(item.DueAt), nullableString(item.TagsJSON), nullableString(item.AgentSuggestion), nullableString(item.NoteText), nullableString(item.Prerequisite), nullableString(item.PlannedAt), nullableString(item.PreviousBucket), nullableString(item.PreviousDueAt), nullableString(item.PreviousStatus), nullableString(item.EndedAt), nullableString(item.RelatedResourcesJSON), nullableString(item.LinkedTaskID), item.CreatedAt, item.UpdatedAt); err != nil {
			return fmt.Errorf("insert todo item %s: %w", item.ItemID, err)
		}
	}

	for _, rule := range rules {
		enabledValue := 0
		if rule.Enabled {
			enabledValue = 1
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO recurring_rules (
				rule_id, item_id, rule_type, cron_expr, interval_value, interval_unit,
				reminder_strategy, enabled, repeat_rule_text, next_occurrence_at,
				recent_instance_status, effective_scope, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, rule.RuleID, rule.ItemID, rule.RuleType, nullableString(rule.CronExpr), nullableInt(rule.IntervalValue), nullableString(rule.IntervalUnit), rule.ReminderStrategy, enabledValue, nullableString(rule.RepeatRuleText), nullableString(rule.NextOccurrenceAt), nullableString(rule.RecentInstanceStatus), nullableString(rule.EffectiveScope), rule.CreatedAt, rule.UpdatedAt); err != nil {
			return fmt.Errorf("insert recurring rule %s: %w", rule.RuleID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit todo replace transaction: %w", err)
	}
	return nil
}

// LoadTodoState returns the persisted todo and recurring snapshots.
func (s *SQLiteTodoStore) LoadTodoState(ctx context.Context) ([]TodoItemRecord, []RecurringRuleRecord, error) {
	itemRows, err := s.db.QueryContext(ctx, `
		SELECT item_id, title, bucket, status, source_path, source_line, source_bucket, due_at, tags_json,
		       agent_suggestion, note_text, prerequisite, planned_at, previous_bucket, previous_due_at, previous_status, ended_at,
		       related_resources_json, linked_task_id, created_at, updated_at
		FROM todo_items ORDER BY updated_at DESC, item_id DESC
	`)
	if err != nil {
		return nil, nil, fmt.Errorf("load todo items: %w", err)
	}
	defer itemRows.Close()

	items := make([]TodoItemRecord, 0)
	for itemRows.Next() {
		var item TodoItemRecord
		var sourcePath, sourceBucket, dueAt, tagsJSON, agentSuggestion, noteText, prerequisite, plannedAt, previousBucket, previousDueAt, previousStatus, endedAt, relatedResourcesJSON, linkedTaskID sql.NullString
		var sourceLine sql.NullInt64
		if err := itemRows.Scan(&item.ItemID, &item.Title, &item.Bucket, &item.Status, &sourcePath, &sourceLine, &sourceBucket, &dueAt, &tagsJSON, &agentSuggestion, &noteText, &prerequisite, &plannedAt, &previousBucket, &previousDueAt, &previousStatus, &endedAt, &relatedResourcesJSON, &linkedTaskID, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, nil, fmt.Errorf("scan todo item row: %w", err)
		}
		item.SourcePath = sourcePath.String
		if sourceLine.Valid {
			item.SourceLine = int(sourceLine.Int64)
		}
		item.SourceBucket = sourceBucket.String
		item.DueAt = dueAt.String
		item.TagsJSON = tagsJSON.String
		item.AgentSuggestion = agentSuggestion.String
		item.NoteText = noteText.String
		item.Prerequisite = prerequisite.String
		item.PlannedAt = plannedAt.String
		item.PreviousBucket = previousBucket.String
		item.PreviousDueAt = previousDueAt.String
		item.PreviousStatus = previousStatus.String
		item.EndedAt = endedAt.String
		item.RelatedResourcesJSON = relatedResourcesJSON.String
		item.LinkedTaskID = linkedTaskID.String
		items = append(items, item)
	}
	if err := itemRows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate todo item rows: %w", err)
	}

	ruleRows, err := s.db.QueryContext(ctx, `
		SELECT rule_id, item_id, rule_type, cron_expr, interval_value, interval_unit,
		       reminder_strategy, enabled, repeat_rule_text, next_occurrence_at,
		       recent_instance_status, effective_scope, created_at, updated_at
		FROM recurring_rules ORDER BY updated_at DESC, rule_id DESC
	`)
	if err != nil {
		return nil, nil, fmt.Errorf("load recurring rules: %w", err)
	}
	defer ruleRows.Close()

	rules := make([]RecurringRuleRecord, 0)
	for ruleRows.Next() {
		var rule RecurringRuleRecord
		var cronExpr, intervalUnit, repeatRuleText, nextOccurrenceAt, recentInstanceStatus, effectiveScope sql.NullString
		var intervalValue sql.NullInt64
		var enabledValue int
		if err := ruleRows.Scan(&rule.RuleID, &rule.ItemID, &rule.RuleType, &cronExpr, &intervalValue, &intervalUnit, &rule.ReminderStrategy, &enabledValue, &repeatRuleText, &nextOccurrenceAt, &recentInstanceStatus, &effectiveScope, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, nil, fmt.Errorf("scan recurring rule row: %w", err)
		}
		rule.CronExpr = cronExpr.String
		if intervalValue.Valid {
			rule.IntervalValue = int(intervalValue.Int64)
		}
		rule.IntervalUnit = intervalUnit.String
		rule.Enabled = enabledValue == 1
		rule.RepeatRuleText = repeatRuleText.String
		rule.NextOccurrenceAt = nextOccurrenceAt.String
		rule.RecentInstanceStatus = recentInstanceStatus.String
		rule.EffectiveScope = effectiveScope.String
		rules = append(rules, rule)
	}
	if err := ruleRows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate recurring rule rows: %w", err)
	}

	return items, rules, nil
}

// Close closes the underlying SQLite connection.
func (s *SQLiteTodoStore) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLiteTodoStore) initialize(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `PRAGMA journal_mode=WAL;`); err != nil {
		return fmt.Errorf("enable sqlite wal mode: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `PRAGMA busy_timeout=5000;`); err != nil {
		return fmt.Errorf("set sqlite busy timeout: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS todo_items (
			item_id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			bucket TEXT NOT NULL,
			status TEXT NOT NULL,
			source_path TEXT,
			source_line INTEGER,
			source_bucket TEXT,
			due_at TEXT,
			tags_json TEXT,
			agent_suggestion TEXT,
			note_text TEXT,
			prerequisite TEXT,
			planned_at TEXT,
			previous_bucket TEXT,
			previous_due_at TEXT,
			previous_status TEXT,
			ended_at TEXT,
			related_resources_json TEXT,
			linked_task_id TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
	`); err != nil {
		return fmt.Errorf("create todo_items table: %w", err)
	}
	if err := s.ensureTodoItemColumns(ctx); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_todo_items_bucket_due_at ON todo_items(bucket, due_at);`); err != nil {
		return fmt.Errorf("create todo_items bucket index: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_todo_items_linked_task_id ON todo_items(linked_task_id);`); err != nil {
		return fmt.Errorf("create todo_items linked task index: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS recurring_rules (
			rule_id TEXT PRIMARY KEY,
			item_id TEXT NOT NULL,
			rule_type TEXT NOT NULL,
			cron_expr TEXT,
			interval_value INTEGER,
			interval_unit TEXT,
			reminder_strategy TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			repeat_rule_text TEXT,
			next_occurrence_at TEXT,
			recent_instance_status TEXT,
			effective_scope TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			FOREIGN KEY(item_id) REFERENCES todo_items(item_id)
		);
	`); err != nil {
		return fmt.Errorf("create recurring_rules table: %w", err)
	}
	if err := s.ensureRecurringRuleColumns(ctx); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_recurring_rules_item_id ON recurring_rules(item_id);`); err != nil {
		return fmt.Errorf("create recurring_rules item index: %w", err)
	}
	return nil
}

func (s *SQLiteTodoStore) ensureTodoItemColumns(ctx context.Context) error {
	requiredColumns := map[string]string{
		"previous_bucket":        "TEXT",
		"previous_due_at":        "TEXT",
		"previous_status":        "TEXT",
		"note_text":              "TEXT",
		"prerequisite":           "TEXT",
		"planned_at":             "TEXT",
		"ended_at":               "TEXT",
		"related_resources_json": "TEXT",
		"source_bucket":          "TEXT",
	}
	return s.ensureColumns(ctx, sqliteTodoTableName, requiredColumns)
}

func (s *SQLiteTodoStore) ensureRecurringRuleColumns(ctx context.Context) error {
	requiredColumns := map[string]string{
		"repeat_rule_text":       "TEXT",
		"next_occurrence_at":     "TEXT",
		"recent_instance_status": "TEXT",
		"effective_scope":        "TEXT",
	}
	return s.ensureColumns(ctx, sqliteRecurringRuleTableName, requiredColumns)
}

func (s *SQLiteTodoStore) ensureColumns(ctx context.Context, tableName string, requiredColumns map[string]string) error {
	existingColumns, err := s.tableColumns(ctx, tableName)
	if err != nil {
		return err
	}
	columnNames := make([]string, 0, len(requiredColumns))
	for name := range requiredColumns {
		columnNames = append(columnNames, name)
	}
	sort.Strings(columnNames)
	for _, name := range columnNames {
		if _, ok := existingColumns[name]; ok {
			continue
		}
		if _, err := s.db.ExecContext(ctx, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, tableName, name, requiredColumns[name])); err != nil {
			return fmt.Errorf("migrate %s add column %s: %w", tableName, name, err)
		}
	}
	return nil
}

func (s *SQLiteTodoStore) tableColumns(ctx context.Context, tableName string) (map[string]struct{}, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info(%s);`, tableName))
	if err != nil {
		return nil, fmt.Errorf("inspect %s schema: %w", tableName, err)
	}
	defer rows.Close()

	columns := make(map[string]struct{})
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return nil, fmt.Errorf("scan %s schema: %w", tableName, err)
		}
		columns[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s schema: %w", tableName, err)
	}
	return columns, nil
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func nullableInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}
