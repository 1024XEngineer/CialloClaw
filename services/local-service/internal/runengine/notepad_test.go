package runengine

import (
	"testing"
	"time"
)

func TestEngineNotepadListProjectsProtocolShapeOnly(t *testing.T) {
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
	if _, ok := items[0]["note_text"]; ok {
		t.Fatalf("expected list projection to omit internal note_text, got %+v", items[0])
	}
	if _, ok := items[0]["related_resources"]; ok {
		t.Fatalf("expected list projection to omit internal related_resources, got %+v", items[0])
	}

	detail, ok := engine.NotepadItem("todo_protocol_only")
	if !ok {
		t.Fatal("expected internal notepad detail to exist")
	}
	if detail["note_text"] == nil || detail["planned_at"] == nil {
		t.Fatalf("expected internal detail fields to be preserved, got %+v", detail)
	}
	resources, ok := detail["related_resources"].([]map[string]any)
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
	if detail["next_occurrence_at"] != now.Add(7*24*time.Hour).Format(time.RFC3339) {
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
