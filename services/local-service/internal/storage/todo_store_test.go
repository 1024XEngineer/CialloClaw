package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestInMemoryTodoStoreReplacesAndLoadsState(t *testing.T) {
	store := NewInMemoryTodoStore()
	err := store.ReplaceTodoState(context.Background(), []TodoItemRecord{{
		ItemID:         "todo_001",
		Title:          "review notes",
		Bucket:         "upcoming",
		Status:         "normal",
		SourcePath:     "workspace/todos/inbox.md",
		SourceBucket:   "later",
		PreviousBucket: "later",
		PreviousDueAt:  "2026-04-21T10:00:00Z",
		PreviousStatus: "normal",
		CreatedAt:      "2026-04-20T10:00:00Z",
		UpdatedAt:      "2026-04-20T10:00:00Z",
	}}, []RecurringRuleRecord{{
		RuleID:           "rule_001",
		ItemID:           "todo_001",
		RuleType:         "interval",
		IntervalValue:    1,
		IntervalUnit:     "week",
		ReminderStrategy: "due_at",
		Enabled:          true,
		CreatedAt:        "2026-04-20T10:00:00Z",
		UpdatedAt:        "2026-04-20T10:00:00Z",
	}})
	if err != nil {
		t.Fatalf("replace todo state failed: %v", err)
	}

	items, rules, err := store.LoadTodoState(context.Background())
	if err != nil {
		t.Fatalf("load todo state failed: %v", err)
	}
	if len(items) != 1 || items[0].ItemID != "todo_001" {
		t.Fatalf("expected one persisted todo item, got %+v", items)
	}
	if items[0].PreviousBucket != "later" || items[0].PreviousDueAt != "2026-04-21T10:00:00Z" || items[0].PreviousStatus != "normal" {
		t.Fatalf("expected previous close metadata to round-trip in memory store, got %+v", items[0])
	}
	if len(rules) != 1 || rules[0].RuleID != "rule_001" {
		t.Fatalf("expected one persisted recurring rule, got %+v", rules)
	}

	err = store.ReplaceTodoState(context.Background(), []TodoItemRecord{{
		ItemID:    "todo_002",
		Title:     "rewrite packet",
		Bucket:    "later",
		Status:    "normal",
		CreatedAt: "2026-04-20T11:00:00Z",
		UpdatedAt: "2026-04-20T11:00:00Z",
	}}, nil)
	if err != nil {
		t.Fatalf("replace todo state second time failed: %v", err)
	}
	items, rules, err = store.LoadTodoState(context.Background())
	if err != nil {
		t.Fatalf("load todo state after replace failed: %v", err)
	}
	if len(items) != 1 || items[0].ItemID != "todo_002" || len(rules) != 0 {
		t.Fatalf("expected replace semantics, got items=%+v rules=%+v", items, rules)
	}
}

func TestSQLiteTodoStorePersistsAndLoadsState(t *testing.T) {
	store, err := NewSQLiteTodoStore(filepath.Join(t.TempDir(), "todos.db"))
	if err != nil {
		t.Fatalf("new sqlite todo store failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	err = store.ReplaceTodoState(context.Background(), []TodoItemRecord{
		{
			ItemID:               "todo_sql_001",
			Title:                "weekly retro",
			Bucket:               "recurring_rule",
			Status:               "normal",
			SourcePath:           "workspace/todos/weekly.md",
			SourceLine:           1,
			SourceBucket:         "upcoming",
			DueAt:                "2026-04-25T10:00:00Z",
			NoteText:             "review blockers",
			PreviousBucket:       "upcoming",
			PreviousDueAt:        "2026-04-25T10:00:00Z",
			PreviousStatus:       "normal",
			NoteTextOrigin:       "user_provided",
			RelatedResourcesJSON: `[{"id":"res_001","path":"workspace/templates/retro.md"}]`,
			CreatedAt:            "2026-04-20T10:00:00Z",
			UpdatedAt:            "2026-04-20T10:00:00Z",
		},
	}, []RecurringRuleRecord{{
		RuleID:               "rule_sql_001",
		ItemID:               "todo_sql_001",
		RuleType:             "interval",
		IntervalValue:        2,
		IntervalUnit:         "week",
		ReminderStrategy:     "due_at",
		Enabled:              true,
		RepeatRuleText:       "每两周一次",
		NextOccurrenceAt:     "2026-05-09T10:00:00Z",
		RecentInstanceStatus: "completed",
		EffectiveScope:       "Project A",
		CreatedAt:            "2026-04-20T10:00:00Z",
		UpdatedAt:            "2026-04-20T10:00:00Z",
	}})
	if err != nil {
		t.Fatalf("replace sqlite todo state failed: %v", err)
	}

	items, rules, err := store.LoadTodoState(context.Background())
	if err != nil {
		t.Fatalf("load sqlite todo state failed: %v", err)
	}
	if len(items) != 1 || items[0].ItemID != "todo_sql_001" || items[0].RelatedResourcesJSON == "" {
		t.Fatalf("expected sqlite todo item payload to persist, got %+v", items)
	}
	if items[0].NoteTextOrigin != "user_provided" {
		t.Fatalf("expected sqlite todo item origin marker to persist, got %+v", items[0])
	}
	if items[0].SourceBucket != "upcoming" || items[0].PreviousBucket != "upcoming" || items[0].PreviousDueAt != "2026-04-25T10:00:00Z" || items[0].PreviousStatus != "normal" {
		t.Fatalf("expected sqlite todo item close metadata to persist, got %+v", items[0])
	}
	if len(rules) != 1 || rules[0].RuleID != "rule_sql_001" || rules[0].NextOccurrenceAt != "2026-05-09T10:00:00Z" {
		t.Fatalf("expected sqlite recurring rule payload to persist, got %+v", rules)
	}
}

func TestSQLiteTodoStoreLoadsEmptyStateAndBlankPathFails(t *testing.T) {
	if _, err := NewSQLiteTodoStore("   "); err == nil {
		t.Fatal("expected blank database path to be rejected")
	}
	store, err := NewSQLiteTodoStore(filepath.Join(t.TempDir(), "empty.db"))
	if err != nil {
		t.Fatalf("new sqlite todo store failed: %v", err)
	}
	defer func() { _ = store.Close() }()
	items, rules, err := store.LoadTodoState(context.Background())
	if err != nil {
		t.Fatalf("load empty sqlite todo state failed: %v", err)
	}
	if len(items) != 0 || len(rules) != 0 {
		t.Fatalf("expected empty sqlite todo state, got items=%+v rules=%+v", items, rules)
	}
}

func TestSQLiteTodoStoreMigratesExistingTablesBeforeLoadingAndReplacing(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "todos-legacy.db")
	db, err := sql.Open(sqliteDriverName, databasePath)
	if err != nil {
		t.Fatalf("open legacy sqlite db failed: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`
		CREATE TABLE todo_items (
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
			linked_task_id TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE TABLE recurring_rules (
			rule_id TEXT PRIMARY KEY,
			item_id TEXT NOT NULL,
			rule_type TEXT NOT NULL,
			cron_expr TEXT,
			interval_value INTEGER,
			interval_unit TEXT,
			reminder_strategy TEXT NOT NULL,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		INSERT INTO todo_items (item_id, title, bucket, status, created_at, updated_at) VALUES ('todo_legacy', 'legacy todo', 'upcoming', 'normal', '2026-04-20T10:00:00Z', '2026-04-20T10:00:00Z');
		INSERT INTO recurring_rules (rule_id, item_id, rule_type, reminder_strategy, enabled, created_at, updated_at) VALUES ('rule_legacy', 'todo_legacy', 'interval', 'due_at', 1, '2026-04-20T10:00:00Z', '2026-04-20T10:00:00Z');
	`); err != nil {
		t.Fatalf("seed legacy schema failed: %v", err)
	}
	_ = db.Close()

	store, err := NewSQLiteTodoStore(databasePath)
	if err != nil {
		t.Fatalf("open migrated sqlite todo store failed: %v", err)
	}
	defer func() { _ = store.Close() }()

	items, rules, err := store.LoadTodoState(context.Background())
	if err != nil {
		t.Fatalf("load migrated todo state failed: %v", err)
	}
	if len(items) != 1 || items[0].ItemID != "todo_legacy" {
		t.Fatalf("expected legacy todo row to survive migration, got %+v", items)
	}
	if len(rules) != 1 || rules[0].RuleID != "rule_legacy" {
		t.Fatalf("expected legacy recurring row to survive migration, got %+v", rules)
	}

	err = store.ReplaceTodoState(context.Background(), []TodoItemRecord{{
		ItemID:               "todo_legacy",
		Title:                "legacy todo updated",
		Bucket:               "later",
		Status:               "normal",
		SourceBucket:         "upcoming",
		NoteText:             "now with richer fields",
		NoteTextOrigin:       "user_provided",
		PreviousBucket:       "upcoming",
		PreviousDueAt:        "2026-04-21T10:00:00Z",
		PreviousStatus:       "normal",
		RelatedResourcesJSON: `[{"id":"res_legacy"}]`,
		CreatedAt:            "2026-04-20T10:00:00Z",
		UpdatedAt:            "2026-04-20T12:00:00Z",
	}}, []RecurringRuleRecord{{
		RuleID:               "rule_legacy",
		ItemID:               "todo_legacy",
		RuleType:             "interval",
		IntervalValue:        1,
		IntervalUnit:         "week",
		ReminderStrategy:     "due_at",
		Enabled:              true,
		RepeatRuleText:       "每周一次",
		NextOccurrenceAt:     "2026-04-27T10:00:00Z",
		RecentInstanceStatus: "completed",
		EffectiveScope:       "legacy",
		CreatedAt:            "2026-04-20T10:00:00Z",
		UpdatedAt:            "2026-04-20T12:00:00Z",
	}})
	if err != nil {
		t.Fatalf("replace migrated todo state failed: %v", err)
	}
	items, rules, err = store.LoadTodoState(context.Background())
	if err != nil {
		t.Fatalf("reload migrated todo state failed: %v", err)
	}
	if items[0].NoteText != "now with richer fields" || items[0].NoteTextOrigin != "user_provided" || items[0].SourceBucket != "upcoming" || items[0].PreviousBucket != "upcoming" || items[0].PreviousDueAt != "2026-04-21T10:00:00Z" || items[0].PreviousStatus != "normal" || rules[0].RepeatRuleText != "每周一次" {
		t.Fatalf("expected migrated schema to support new columns, got items=%+v rules=%+v", items, rules)
	}
}

func TestServiceTodoStoreAccessorUsesConfiguredStore(t *testing.T) {
	service := NewService(nil)
	if service.TodoStore() == nil {
		t.Fatal("expected todo store accessor to return fallback store")
	}
	configured := NewService(stubAdapter{databasePath: filepath.Join(t.TempDir(), "service.db")})
	defer func() { _ = configured.Close() }()
	if configured.TodoStore() == nil {
		t.Fatal("expected configured service to expose todo store")
	}
	if err := configured.TodoStore().ReplaceTodoState(context.Background(), []TodoItemRecord{{
		ItemID:    "todo_service",
		Title:     "service note",
		Bucket:    "upcoming",
		Status:    "normal",
		CreatedAt: "2026-04-20T10:00:00Z",
		UpdatedAt: "2026-04-20T10:00:00Z",
	}}, nil); err != nil {
		t.Fatalf("expected service todo store to persist state, got %v", err)
	}
}
