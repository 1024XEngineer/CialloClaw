package runengine

import (
	"context"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

func TestEngineNotepadListProjectsExpandedProtocolShape(t *testing.T) {
	engine := NewEngine()
	now := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }
	engine.ReplaceNotepadItems([]map[string]any{{
		"item_id":          "todo_protocol_only",
		"title":            "整理模板草稿",
		"bucket":           notepadBucketUpcoming,
		"status":           "normal",
		"type":             "one_time",
		"due_at":           now.Add(3 * time.Hour).Format(time.RFC3339),
		"agent_suggestion": "生成摘要",
		"note_text":        "这是内部详情正文，不应直接泄漏到 TodoItem 列表。",
		"related_resources": []map[string]any{{
			"id":          "res_protocol",
			"label":       "模板目录",
			"path":        "workspace/templates",
			"type":        "directory",
			"target_kind": "folder",
		}},
	}})

	items, total := engine.NotepadItems(notepadBucketUpcoming, 10, 0)
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected one projected notepad item, total=%d len=%d", total, len(items))
	}
	if items[0]["status"] != "due_today" {
		t.Fatalf("expected projected item to keep normalized status, got %+v", items[0])
	}
	if items[0]["note_text"] != "这是内部详情正文，不应直接泄漏到 TodoItem 列表。" {
		t.Fatalf("expected list projection to expose protocol note_text, got %+v", items[0])
	}
	resources, ok := items[0]["related_resources"].([]map[string]any)
	if !ok || len(resources) != 1 {
		t.Fatalf("expected list projection to expose related_resources, got %+v", items[0]["related_resources"])
	}
	if resources[0]["resource_id"] != "res_protocol" || resources[0]["open_action"] != "reveal_in_folder" {
		t.Fatalf("expected projected resource shape to match protocol contract, got %+v", resources[0])
	}

	detail, ok := engine.NotepadItem("todo_protocol_only")
	if !ok {
		t.Fatal("expected internal notepad detail to exist")
	}
	if detail["note_text"] == nil || detail["planned_at"] == nil {
		t.Fatalf("expected internal detail fields to be preserved, got %+v", detail)
	}
	resources, ok = detail["related_resources"].([]map[string]any)
	if !ok || len(resources) != 1 {
		t.Fatalf("expected internal related resources to stay available, got %+v", detail["related_resources"])
	}
}

func TestEngineRecurringNotepadFoundationFieldsAreDerived(t *testing.T) {
	engine := NewEngine()
	now := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }
	engine.ReplaceNotepadItems([]map[string]any{{
		"item_id":          "todo_recurring_detail",
		"title":            "每周模板复盘",
		"bucket":           notepadBucketRecurringRule,
		"status":           "normal",
		"type":             "recurring",
		"due_at":           now.Add(7 * 24 * time.Hour).Format(time.RFC3339),
		"agent_suggestion": "沿用模板",
	}})

	detail, ok := engine.NotepadItem("todo_recurring_detail")
	if !ok {
		t.Fatal("expected recurring item to exist")
	}
	if detail["recurring_enabled"] != true {
		t.Fatalf("expected recurring item to default to enabled, got %+v", detail)
	}
	if detail["repeat_rule_text"] != "每周重复一次" {
		t.Fatalf("expected recurring rule fallback text, got %+v", detail["repeat_rule_text"])
	}
	if detail["next_occurrence_at"] != now.Add(14*24*time.Hour).Format(time.RFC3339) {
		t.Fatalf("expected next occurrence to follow due_at, got %+v", detail["next_occurrence_at"])
	}
	resources, ok := detail["related_resources"].([]map[string]any)
	if !ok || len(resources) == 0 {
		t.Fatalf("expected recurring item to derive related resources, got %+v", detail["related_resources"])
	}
	if resources[0]["path"] != defaultTaskSourcePath {
		t.Fatalf("expected recurring item to point to task source directory, got %+v", resources[0])
	}
}

func TestEngineNotepadActionMutationsPreserveFoundationState(t *testing.T) {
	engine := NewEngine()
	now := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }
	plannedAt := now.Add(4 * time.Hour).Format(time.RFC3339)
	nextOccurrence := now.Add(14 * 24 * time.Hour).Format(time.RFC3339)
	engine.ReplaceNotepadItems([]map[string]any{
		{
			"item_id":          "todo_restore",
			"title":            "待恢复事项",
			"bucket":           notepadBucketLater,
			"status":           "normal",
			"type":             "one_time",
			"due_at":           plannedAt,
			"agent_suggestion": "整理上下文",
		},
		{
			"item_id":            "todo_rule",
			"title":              "每周项目复盘",
			"bucket":             notepadBucketRecurringRule,
			"status":             "normal",
			"type":               "recurring",
			"due_at":             plannedAt,
			"next_occurrence_at": plannedAt,
			"recurring_enabled":  true,
		},
	})

	completed, ok := engine.CompleteNotepadItem("todo_restore")
	if !ok {
		t.Fatal("expected completion to succeed")
	}
	if completed["bucket"] != notepadBucketClosed || completed["ended_at"] == nil || completed["planned_at"] != plannedAt {
		t.Fatalf("expected completion to preserve foundation state, got %+v", completed)
	}

	restored, ok := engine.RestoreNotepadItem("todo_restore")
	if !ok {
		t.Fatal("expected restore to succeed")
	}
	if restored["bucket"] != notepadBucketLater || restored["due_at"] != plannedAt || restored["ended_at"] != nil {
		t.Fatalf("expected restore to recover original bucket and schedule, got %+v", restored)
	}

	cancelled, ok := engine.CancelNotepadItem("todo_restore")
	if !ok || cancelled["status"] != "cancelled" {
		t.Fatalf("expected cancel to close item, got %+v ok=%v", cancelled, ok)
	}

	paused, ok := engine.SetNotepadRecurringEnabled("todo_rule", false)
	if !ok || paused["recent_instance_status"] != "paused" || paused["due_at"] != nil {
		t.Fatalf("expected recurring pause to clear due_at and mark paused, got %+v ok=%v", paused, ok)
	}

	resumed, ok := engine.SetNotepadRecurringEnabled("todo_rule", true)
	if !ok || resumed["due_at"] != plannedAt {
		t.Fatalf("expected recurring resume to restore due_at, got %+v ok=%v", resumed, ok)
	}

	updatedRule, ok := engine.UpdateNotepadRecurringRule("todo_rule", "每两周一次", nextOccurrence, "仅项目 A")
	if !ok {
		t.Fatal("expected recurring rule update to succeed")
	}
	if updatedRule["repeat_rule_text"] != "每两周一次" || updatedRule["next_occurrence_at"] != nextOccurrence || updatedRule["effective_scope"] != "仅项目 A" {
		t.Fatalf("expected recurring rule fields to update, got %+v", updatedRule)
	}

	if !engine.DeleteNotepadItem("todo_rule") {
		t.Fatal("expected delete to remove recurring item")
	}
	if _, ok := engine.NotepadItem("todo_rule"); ok {
		t.Fatal("expected deleted recurring item to disappear")
	}
}

func TestEngineTodoStorePersistsAndReloadsNotepadState(t *testing.T) {
	todoStore := storage.NewInMemoryTodoStore()
	engine, err := NewEngineWithStore(storage.NewInMemoryTaskRunStore())
	if err != nil {
		t.Fatalf("new engine with store failed: %v", err)
	}
	now := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }
	if err := engine.WithTodoStore(todoStore); err != nil {
		t.Fatalf("attach todo store failed: %v", err)
	}
	if err := engine.SyncNotepadItems([]map[string]any{{
		"item_id":            "todo_persisted_rule",
		"title":              "每周项目复盘",
		"bucket":             notepadBucketRecurringRule,
		"status":             "normal",
		"type":               "recurring",
		"source_path":        "workspace/todos/weekly.md",
		"source_line":        2,
		"note_text":          "从真实任务源解析出来的说明。",
		"note_text_origin":   "user_provided",
		"repeat_rule_text":   "每两周一次",
		"next_occurrence_at": now.Add(14 * 24 * time.Hour).Format(time.RFC3339),
		"related_resources": []map[string]any{{
			"id":          "res_persisted",
			"label":       "模板",
			"path":        "workspace/templates/retro.md",
			"type":        "file",
			"target_kind": "file",
		}},
	}}); err != nil {
		t.Fatalf("sync notepad items failed: %v", err)
	}

	reloaded, err := NewEngineWithStore(storage.NewInMemoryTaskRunStore())
	if err != nil {
		t.Fatalf("new engine reload failed: %v", err)
	}
	reloaded.now = func() time.Time { return now }
	if err := reloaded.WithTodoStore(todoStore); err != nil {
		t.Fatalf("attach todo store on reload failed: %v", err)
	}

	detail, ok := reloaded.NotepadItem("todo_persisted_rule")
	if !ok {
		t.Fatal("expected persisted note to reload from todo store")
	}
	if detail["repeat_rule_text"] != "每两周一次" || detail["source_path"] != "workspace/todos/weekly.md" {
		t.Fatalf("expected recurring note metadata to reload, got %+v", detail)
	}
	if detail["note_text_origin"] != "user_provided" {
		t.Fatalf("expected note_text_origin to persist across reload, got %+v", detail)
	}
	resources, ok := detail["related_resources"].([]map[string]any)
	if !ok || len(resources) != 1 {
		t.Fatalf("expected related resources to reload, got %+v", detail["related_resources"])
	}
}

func TestEngineTodoStoreReloadKeepsAmbiguousLegacyNoteTextAndMarksGeneratedDerivedResources(t *testing.T) {
	todoStore := storage.NewInMemoryTodoStore()
	now := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	legacySynthetic := deriveSyntheticNotepadNoteText("周报模板", "")

	if err := todoStore.ReplaceTodoState(context.Background(), []storage.TodoItemRecord{{
		ItemID:               "todo_legacy_defaults",
		Title:                "周报模板",
		Bucket:               notepadBucketUpcoming,
		Status:               "normal",
		NoteText:             legacySynthetic,
		RelatedResourcesJSON: `[{"id":"todo_legacy_defaults_drafts","label":"草稿目录","path":"workspace/drafts","type":"directory","target_kind":"folder"},{"id":"todo_legacy_defaults_workspace","label":"默认工作区","path":"workspace","type":"directory","target_kind":"folder"}]`,
		CreatedAt:            now.Format(time.RFC3339),
		UpdatedAt:            now.Format(time.RFC3339),
	}}, nil); err != nil {
		t.Fatalf("seed legacy todo store state failed: %v", err)
	}

	reloaded, err := NewEngineWithStore(storage.NewInMemoryTaskRunStore())
	if err != nil {
		t.Fatalf("new reload engine failed: %v", err)
	}
	reloaded.now = func() time.Time { return now }
	if err := reloaded.WithTodoStore(todoStore); err != nil {
		t.Fatalf("attach todo store on reload failed: %v", err)
	}

	detail, ok := reloaded.NotepadItem("todo_legacy_defaults")
	if !ok {
		t.Fatal("expected legacy note to reload from todo store")
	}
	if detail["note_text_origin"] != "user_provided" {
		t.Fatalf("expected ambiguous legacy note_text to stay on the conservative user_provided side, got %+v", detail)
	}
	resources, ok := detail["related_resources"].([]map[string]any)
	if !ok || len(resources) != 2 {
		t.Fatalf("expected legacy related resources to reload, got %+v", detail["related_resources"])
	}
	for _, resource := range resources {
		if resource["resource_origin"] != "derived_default" {
			t.Fatalf("expected legacy fallback resource to be marked derived_default, got %+v", resources)
		}
	}
}

func TestEngineTodoStoreReloadKeepsUserAttachedFallbackPathResourcesExplicit(t *testing.T) {
	todoStore := storage.NewInMemoryTodoStore()
	now := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)

	if err := todoStore.ReplaceTodoState(context.Background(), []storage.TodoItemRecord{{
		ItemID:               "todo_user_templates",
		Title:                "模板评审",
		Bucket:               notepadBucketUpcoming,
		Status:               "normal",
		NoteText:             "请重点检查 workspace/templates 中这套模板。",
		RelatedResourcesJSON: `[{"id":"user_templates_attachment","label":"关联模板","path":"workspace/templates","type":"directory","target_kind":"folder"}]`,
		CreatedAt:            now.Format(time.RFC3339),
		UpdatedAt:            now.Format(time.RFC3339),
	}}, nil); err != nil {
		t.Fatalf("seed user-attached todo store state failed: %v", err)
	}

	reloaded, err := NewEngineWithStore(storage.NewInMemoryTaskRunStore())
	if err != nil {
		t.Fatalf("new reload engine failed: %v", err)
	}
	reloaded.now = func() time.Time { return now }
	if err := reloaded.WithTodoStore(todoStore); err != nil {
		t.Fatalf("attach todo store on reload failed: %v", err)
	}

	detail, ok := reloaded.NotepadItem("todo_user_templates")
	if !ok {
		t.Fatal("expected explicit template note to reload from todo store")
	}
	resources, ok := detail["related_resources"].([]map[string]any)
	if !ok || len(resources) != 1 {
		t.Fatalf("expected explicit template resource to reload, got %+v", detail["related_resources"])
	}
	if _, marked := resources[0]["resource_origin"]; marked {
		t.Fatalf("expected explicit template attachment to stay user-authored, got %+v", resources[0])
	}
}

func TestEngineSyncNotepadItemsRecomputesDerivedOriginWhenUserClearsNoteText(t *testing.T) {
	engine := NewEngine()
	now := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }

	if err := engine.SyncNotepadItems([]map[string]any{{
		"item_id":          "todo_clear_note_text",
		"title":            "整理评审要点",
		"bucket":           notepadBucketUpcoming,
		"status":           "normal",
		"note_text":        "这是用户之前写下的正文。",
		"note_text_origin": "user_provided",
	}}); err != nil {
		t.Fatalf("seed notepad items failed: %v", err)
	}

	if err := engine.SyncNotepadItems([]map[string]any{{
		"item_id":          "todo_clear_note_text",
		"title":            "整理评审要点",
		"bucket":           notepadBucketUpcoming,
		"status":           "normal",
		"note_text":        "",
		"note_text_origin": "user_provided",
	}}); err != nil {
		t.Fatalf("update notepad items failed: %v", err)
	}

	detail, ok := engine.NotepadItem("todo_clear_note_text")
	if !ok {
		t.Fatal("expected cleared note to remain available")
	}
	if detail["note_text_origin"] != "derived_default" {
		t.Fatalf("expected cleared note_text to recompute derived_default origin, got %+v", detail)
	}
	expectedNoteText := deriveSyntheticNotepadNoteText("整理评审要点", "")
	if detail["note_text"] != expectedNoteText {
		t.Fatalf("expected cleared note to synthesize display-only note_text, got %+v", detail["note_text"])
	}
}

func TestEngineWithTodoStoreStartsEmptyWhenStoreHasNoNotes(t *testing.T) {
	engine := NewEngine()
	items, total := engine.NotepadItems("", 20, 0)
	if total == 0 || len(items) == 0 {
		t.Fatal("expected default engine to start with demo notes before todo store attaches")
	}
	if err := engine.WithTodoStore(storage.NewInMemoryTodoStore()); err != nil {
		t.Fatalf("attach empty todo store failed: %v", err)
	}
	items, total = engine.NotepadItems("", 20, 0)
	if total != 0 || len(items) != 0 {
		t.Fatalf("expected empty todo store to override demo seed data, total=%d len=%d items=%+v", total, len(items), items)
	}
}

func TestEngineCompleteNotepadItemPersistsThroughTodoStore(t *testing.T) {
	todoStore := storage.NewInMemoryTodoStore()
	engine, err := NewEngineWithStore(storage.NewInMemoryTaskRunStore())
	if err != nil {
		t.Fatalf("new engine with store failed: %v", err)
	}
	now := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }
	if err := engine.WithTodoStore(todoStore); err != nil {
		t.Fatalf("attach todo store failed: %v", err)
	}
	if err := engine.SyncNotepadItems([]map[string]any{{
		"item_id":     "todo_complete_persist",
		"title":       "persist completion",
		"bucket":      notepadBucketUpcoming,
		"status":      "normal",
		"type":        "one_time",
		"planned_at":  now.Add(4 * time.Hour).Format(time.RFC3339),
		"due_at":      now.Add(4 * time.Hour).Format(time.RFC3339),
		"created_at":  now.Format(time.RFC3339),
		"updated_at":  now.Format(time.RFC3339),
		"note_text":   "persist me",
		"source_path": "workspace/todos/inbox.md",
	}}); err != nil {
		t.Fatalf("sync notepad items failed: %v", err)
	}
	completed, ok := engine.CompleteNotepadItem("todo_complete_persist")
	if !ok || completed["bucket"] != notepadBucketClosed {
		t.Fatalf("expected completion to succeed and close note, got %+v ok=%v", completed, ok)
	}
	reloaded, err := NewEngineWithStore(storage.NewInMemoryTaskRunStore())
	if err != nil {
		t.Fatalf("new reload engine failed: %v", err)
	}
	reloaded.now = func() time.Time { return now }
	if err := reloaded.WithTodoStore(todoStore); err != nil {
		t.Fatalf("attach todo store on reload failed: %v", err)
	}
	detail, ok := reloaded.NotepadItem("todo_complete_persist")
	if !ok || detail["bucket"] != notepadBucketClosed || detail["status"] != "completed" {
		t.Fatalf("expected completed note to persist across reload, got %+v ok=%v", detail, ok)
	}
}

func TestEngineRestoreNotepadItemPersistsLatestClosedStateAcrossReloads(t *testing.T) {
	todoStore := storage.NewInMemoryTodoStore()
	now := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	plannedAt := now.Add(48 * time.Hour).Format(time.RFC3339)

	engine, err := NewEngineWithStore(storage.NewInMemoryTaskRunStore())
	if err != nil {
		t.Fatalf("new engine with store failed: %v", err)
	}
	engine.now = func() time.Time { return now }
	if err := engine.WithTodoStore(todoStore); err != nil {
		t.Fatalf("attach todo store failed: %v", err)
	}
	if err := engine.SyncNotepadItems([]map[string]any{{
		"item_id":    "todo_restore_reload",
		"title":      "persist restore metadata",
		"bucket":     notepadBucketLater,
		"status":     "normal",
		"type":       "one_time",
		"planned_at": plannedAt,
		"due_at":     plannedAt,
		"created_at": now.Format(time.RFC3339),
		"updated_at": now.Format(time.RFC3339),
	}}); err != nil {
		t.Fatalf("sync notepad items failed: %v", err)
	}
	if completed, ok := engine.CompleteNotepadItem("todo_restore_reload"); !ok || completed["previous_bucket"] != notepadBucketLater {
		t.Fatalf("expected initial close to preserve previous bucket, got %+v ok=%v", completed, ok)
	}

	reloaded, err := NewEngineWithStore(storage.NewInMemoryTaskRunStore())
	if err != nil {
		t.Fatalf("new reload engine failed: %v", err)
	}
	reloaded.now = func() time.Time { return now }
	if err := reloaded.WithTodoStore(todoStore); err != nil {
		t.Fatalf("attach todo store on reload failed: %v", err)
	}
	restored, ok := reloaded.RestoreNotepadItem("todo_restore_reload")
	if !ok || restored["bucket"] != notepadBucketLater || restored["status"] != "normal" || restored["due_at"] != plannedAt {
		t.Fatalf("expected restore after reload to recover latest closed state, got %+v ok=%v", restored, ok)
	}
	moved, refreshGroups, deletedItemID, handled, err := reloaded.UpdateNotepadItem("todo_restore_reload", "move_upcoming")
	if err != nil || !handled || deletedItemID != "" {
		t.Fatalf("move_upcoming after restore failed, handled=%v deleted=%q err=%v", handled, deletedItemID, err)
	}
	if moved["bucket"] != notepadBucketUpcoming || len(refreshGroups) != 2 {
		t.Fatalf("expected move_upcoming to change bucket before re-close, got %+v refresh=%+v", moved, refreshGroups)
	}
	if cancelled, ok := reloaded.CancelNotepadItem("todo_restore_reload"); !ok || cancelled["previous_bucket"] != notepadBucketUpcoming {
		t.Fatalf("expected re-close to overwrite previous bucket before reload, got %+v ok=%v", cancelled, ok)
	}

	reloadedAgain, err := NewEngineWithStore(storage.NewInMemoryTaskRunStore())
	if err != nil {
		t.Fatalf("new second reload engine failed: %v", err)
	}
	reloadedAgain.now = func() time.Time { return now }
	if err := reloadedAgain.WithTodoStore(todoStore); err != nil {
		t.Fatalf("attach todo store on second reload failed: %v", err)
	}
	restoredAgain, ok := reloadedAgain.RestoreNotepadItem("todo_restore_reload")
	if !ok || restoredAgain["bucket"] != notepadBucketUpcoming || restoredAgain["status"] != "normal" || restoredAgain["due_at"] != plannedAt {
		t.Fatalf("expected second restore after reload to use latest pre-close state, got %+v ok=%v", restoredAgain, ok)
	}
}

func TestEngineLinkNotepadItemTaskPersistsThroughTodoStore(t *testing.T) {
	todoStore := storage.NewInMemoryTodoStore()
	engine, err := NewEngineWithStore(storage.NewInMemoryTaskRunStore())
	if err != nil {
		t.Fatalf("new engine with store failed: %v", err)
	}
	now := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }
	if err := engine.WithTodoStore(todoStore); err != nil {
		t.Fatalf("attach todo store failed: %v", err)
	}
	if err := engine.SyncNotepadItems([]map[string]any{{
		"item_id":     "todo_link_persist",
		"title":       "persist linked task",
		"bucket":      notepadBucketUpcoming,
		"status":      "normal",
		"type":        "one_time",
		"planned_at":  now.Add(4 * time.Hour).Format(time.RFC3339),
		"due_at":      now.Add(4 * time.Hour).Format(time.RFC3339),
		"created_at":  now.Format(time.RFC3339),
		"updated_at":  now.Format(time.RFC3339),
		"note_text":   "persist linked task",
		"source_path": "workspace/todos/inbox.md",
	}}); err != nil {
		t.Fatalf("sync notepad items failed: %v", err)
	}
	if _, handled, err := engine.ClaimNotepadItemTask("todo_link_persist"); err != nil || !handled {
		t.Fatalf("claim before link failed, handled=%v err=%v", handled, err)
	}
	linked, ok := engine.LinkNotepadItemTask("todo_link_persist", "task_999")
	if !ok || linked["linked_task_id"] != "task_999" {
		t.Fatalf("expected linked task id to be written, got %+v ok=%v", linked, ok)
	}

	reloaded, err := NewEngineWithStore(storage.NewInMemoryTaskRunStore())
	if err != nil {
		t.Fatalf("new reload engine failed: %v", err)
	}
	reloaded.now = func() time.Time { return now }
	if err := reloaded.WithTodoStore(todoStore); err != nil {
		t.Fatalf("attach todo store on reload failed: %v", err)
	}
	detail, ok := reloaded.NotepadItem("todo_link_persist")
	if !ok || detail["linked_task_id"] != "task_999" {
		t.Fatalf("expected linked_task_id to persist across reload, got %+v ok=%v", detail, ok)
	}
}

func TestEngineUpdateNotepadItemPersistsThroughTodoStore(t *testing.T) {
	todoStore := storage.NewInMemoryTodoStore()
	engine, err := NewEngineWithStore(storage.NewInMemoryTaskRunStore())
	if err != nil {
		t.Fatalf("new engine with store failed: %v", err)
	}
	now := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }
	if err := engine.WithTodoStore(todoStore); err != nil {
		t.Fatalf("attach todo store failed: %v", err)
	}
	if err := engine.SyncNotepadItems([]map[string]any{{
		"item_id":            "todo_update_persist",
		"title":              "persist recurring rule",
		"bucket":             notepadBucketRecurringRule,
		"status":             "normal",
		"type":               "recurring",
		"due_at":             now.Add(24 * time.Hour).Format(time.RFC3339),
		"next_occurrence_at": now.Add(24 * time.Hour).Format(time.RFC3339),
		"recurring_enabled":  true,
		"created_at":         now.Format(time.RFC3339),
		"updated_at":         now.Format(time.RFC3339),
		"note_text":          "persist recurring update",
	}}); err != nil {
		t.Fatalf("sync notepad items failed: %v", err)
	}
	updated, refreshGroups, deletedItemID, handled, err := engine.UpdateNotepadItem("todo_update_persist", "toggle_recurring")
	if err != nil || !handled {
		t.Fatalf("toggle recurring failed, handled=%v err=%v", handled, err)
	}
	if deletedItemID != "" || len(refreshGroups) != 1 || refreshGroups[0] != notepadBucketRecurringRule {
		t.Fatalf("unexpected update result, deleted=%q refresh=%+v", deletedItemID, refreshGroups)
	}
	if updated["recurring_enabled"] != false {
		t.Fatalf("expected recurring_enabled=false after toggle, got %+v", updated)
	}

	reloaded, err := NewEngineWithStore(storage.NewInMemoryTaskRunStore())
	if err != nil {
		t.Fatalf("new reload engine failed: %v", err)
	}
	reloaded.now = func() time.Time { return now }
	if err := reloaded.WithTodoStore(todoStore); err != nil {
		t.Fatalf("attach todo store on reload failed: %v", err)
	}
	detail, ok := reloaded.NotepadItem("todo_update_persist")
	if !ok || detail["recurring_enabled"] != false || detail["status"] != "cancelled" {
		t.Fatalf("expected recurring update to persist across reload, got %+v ok=%v", detail, ok)
	}
}

func TestNotepadHelperFunctionsCoverRecurringAndEncodingPaths(t *testing.T) {
	now := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	if recurringRuleTextFromSpec(recurringRuleSpec{ruleType: "cron", cronExpr: "0 9 * * 1"}) != "cron: 0 9 * * 1" {
		t.Fatal("expected cron spec to render explicit rule text")
	}
	if recurringRuleTextFromSpec(recurringRuleSpec{ruleType: "interval", intervalValue: 1, intervalUnit: "day"}) != "每天一次" {
		t.Fatal("expected daily interval rule text")
	}
	if recurringRuleTextFromSpec(recurringRuleSpec{ruleType: "interval", intervalValue: 2, intervalUnit: "week"}) != "每2周一次" {
		t.Fatal("expected biweekly interval rule text")
	}
	if recurringIntervalText("month") != "月" {
		t.Fatal("expected month interval text")
	}
	item := map[string]any{"planned_at": now.Format(time.RFC3339)}
	if recurringNextOccurrenceFromSpec(item, recurringRuleSpec{ruleType: "interval", intervalValue: 1, intervalUnit: "month"}) != now.AddDate(0, 1, 0).Format(time.RFC3339) {
		t.Fatal("expected monthly next occurrence")
	}
	if recurringNextOccurrenceFromSpec(item, recurringRuleSpec{ruleType: "interval", intervalValue: 0, intervalUnit: "week"}) != now.AddDate(0, 0, 7).Format(time.RFC3339) {
		t.Fatal("expected zero interval to normalize to one week")
	}
	if maxRecurringInterval(0) != 1 {
		t.Fatal("expected maxRecurringInterval to normalize zero")
	}
	if deriveNotepadEndedAt(map[string]any{"status": "completed", "bucket": notepadBucketClosed, "due_at": now.Format(time.RFC3339)}) != now.Format(time.RFC3339) {
		t.Fatal("expected ended_at fallback to use due_at for closed completed notes")
	}
	resources := cloneResourceList([]any{map[string]any{"id": "res_001", "path": "workspace/todos/inbox.md"}})
	if len(resources) != 1 || resources[0]["id"] != "res_001" {
		t.Fatalf("expected cloneResourceList to accept []any values, got %+v", resources)
	}
	tagsJSON, err := marshalStringSlice([]any{"weekly", "notes"})
	if err != nil || tagsJSON == "" {
		t.Fatalf("expected marshalStringSlice to encode []any tags, got %q err=%v", tagsJSON, err)
	}
	if decoded := decodeTags(tagsJSON); len(decoded) != 2 || decoded[1] != "notes" {
		t.Fatalf("expected decodeTags to restore encoded tags, got %+v", decoded)
	}
	if _, err := marshalStringSlice([]any{"weekly", 123}); err != nil {
		t.Fatalf("expected non-string tag entries to be ignored rather than failing, got %v", err)
	}
	if itemIntValue(map[string]any{"count": int64(2)}, "count", 0) != 2 {
		t.Fatal("expected itemIntValue to read int64 values")
	}
	if firstNonEmptyString("", "  value  ") != "value" {
		t.Fatal("expected firstNonEmptyString to trim and return first value")
	}
	if spec := parseRecurringRuleSpec("daily", "", 0, ""); spec.intervalUnit != "day" || spec.intervalValue != 1 {
		t.Fatalf("expected daily rule text to parse as one-day interval, got %+v", spec)
	}
	if spec := parseRecurringRuleSpec("cron: 0 9 * * 1", "", 0, ""); spec.ruleType != "cron" || spec.cronExpr != "0 9 * * 1" {
		t.Fatalf("expected cron rule text to parse as cron, got %+v", spec)
	}
	restoredRecurring := map[string]any{"source_bucket": notepadBucketRecurringRule, "recurring_enabled": true, "next_occurrence_at": now.Add(24 * time.Hour).Format(time.RFC3339)}
	restoreNotepadItem(restoredRecurring, now)
	if restoredRecurring["due_at"] != now.Add(24*time.Hour).Format(time.RFC3339) {
		t.Fatalf("expected recurring restore to restore due_at from next occurrence, got %+v", restoredRecurring)
	}
	if deriveRecurringRecentStatus(map[string]any{"status": "cancelled"}) != "cancelled" {
		t.Fatal("expected recurring recent status to follow explicit cancelled status")
	}
	engine, err := NewEngineWithStore(storage.NewInMemoryTaskRunStore())
	if err != nil {
		t.Fatalf("new engine with store failed: %v", err)
	}
	if engine.CurrentState() != "processing" || engine.CurrentTaskStatus() != "confirming_intent" {
		t.Fatalf("expected empty engine defaults for run/task state, got run=%s task=%s", engine.CurrentState(), engine.CurrentTaskStatus())
	}
}
