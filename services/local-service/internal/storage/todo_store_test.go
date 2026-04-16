package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestInMemoryTodoStoreReplacesAndLoadsState(t *testing.T) {
	store := NewInMemoryTodoStore()
	err := store.ReplaceTodoState(context.Background(), []TodoItemRecord{{
		ItemID:     "todo_001",
		Title:      "review notes",
		Bucket:     "upcoming",
		Status:     "normal",
		SourcePath: "workspace/todos/inbox.md",
		CreatedAt:  "2026-04-20T10:00:00Z",
		UpdatedAt:  "2026-04-20T10:00:00Z",
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
			DueAt:                "2026-04-25T10:00:00Z",
			NoteText:             "review blockers",
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
	if len(rules) != 1 || rules[0].RuleID != "rule_sql_001" || rules[0].NextOccurrenceAt != "2026-05-09T10:00:00Z" {
		t.Fatalf("expected sqlite recurring rule payload to persist, got %+v", rules)
	}
}
