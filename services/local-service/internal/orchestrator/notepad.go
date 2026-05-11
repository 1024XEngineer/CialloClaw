package orchestrator

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

// NotepadList returns lightweight notepad items from storage without promoting
// them to formal tasks.
func (s *Service) NotepadList(params map[string]any) (map[string]any, error) {
	group := stringValue(params, "group", "upcoming")
	limit := intValue(params, "limit", 20)
	offset := intValue(params, "offset", 0)
	items, total := s.runEngine.NotepadItems(group, limit, offset)
	return map[string]any{
		"items": items,
		"page":  pageMap(limit, offset, total),
	}, nil
}

// NotepadUpdate persists one notepad item update while leaving task creation to
// NotepadConvertToTask.
func (s *Service) NotepadUpdate(params map[string]any) (map[string]any, error) {
	itemID := stringValue(params, "item_id", "")
	if itemID == "" {
		return nil, fmt.Errorf("item_id is required")
	}

	action := stringValue(params, "action", "")
	if action == "" {
		return nil, fmt.Errorf("action is required")
	}

	updatedItem, refreshGroups, deletedItemID, handled, err := s.runEngine.UpdateNotepadItem(itemID, action)
	if err != nil {
		return nil, err
	}
	if !handled {
		return nil, fmt.Errorf("notepad item not found: %s", itemID)
	}

	response := map[string]any{
		"notepad_item":    any(nil),
		"refresh_groups":  refreshGroups,
		"deleted_item_id": nil,
	}
	if updatedItem != nil {
		response["notepad_item"] = updatedItem
	}
	if deletedItemID != "" {
		response["deleted_item_id"] = deletedItemID
	}
	return response, nil
}

// NotepadConvertToTask promotes one notepad item into the formal task/run
// workflow and returns the same task payload shape as normal task creation.
func (s *Service) NotepadConvertToTask(params map[string]any) (map[string]any, error) {
	itemID := stringValue(params, "item_id", "")
	if itemID == "" {
		return nil, fmt.Errorf("item_id is required")
	}
	if !boolValue(params, "confirmed", false) {
		return nil, fmt.Errorf("confirmed must be true to convert notepad item")
	}

	item, handled, claimErr := s.runEngine.ClaimNotepadItemTask(itemID)
	if claimErr != nil {
		return nil, claimErr
	}
	if !handled {
		return nil, fmt.Errorf("notepad item not found: %s", itemID)
	}
	claimed := true
	defer func() {
		if claimed {
			s.runEngine.ReleaseNotepadItemClaim(itemID)
		}
	}()

	itemTitle := stringValue(item, "title", "待办事项")
	taskIntent := notepadIntent(item)
	task := s.runEngine.CreateTask(runengine.CreateTaskInput{
		RequestSource: "dashboard",
		Title:         itemTitle,
		SourceType:    "todo",
		Status:        "confirming_intent",
		Intent:        taskIntent,
		CurrentStep:   "intent_confirmation",
		RiskLevel:     s.risk.DefaultLevel(),
		Timeline:      initialTimeline("confirming_intent", "intent_confirmation"),
	})
	s.attachMemoryReadPlans(task.TaskID, task.RunID, notepadSnapshot(item), taskIntent)
	updatedItem, ok := s.runEngine.LinkNotepadItemTask(itemID, task.TaskID)
	if !ok {
		linkErr := fmt.Errorf("failed to link notepad item to task: %s", itemID)
		if rollbackErr := s.runEngine.DeleteTask(task.TaskID); rollbackErr != nil {
			return nil, errors.Join(linkErr, fmt.Errorf("rollback task %s: %w", task.TaskID, rollbackErr))
		}
		return nil, linkErr
	}
	claimed = false

	return map[string]any{
		"task":           taskMap(task),
		"notepad_item":   updatedItem,
		"refresh_groups": []string{stringValue(updatedItem, "bucket", "upcoming")},
	}, nil
}

// defaultIntentMap creates a minimal default intent payload for notepad
// conversions.
func defaultIntentMap(name string) map[string]any {
	arguments := map[string]any{}
	if name == "summarize" {
		arguments["style"] = "key_points"
	}
	if name == "rewrite" {
		arguments["tone"] = "professional"
	}
	return map[string]any{
		"name":      name,
		"arguments": arguments,
	}
}

func notepadIntent(item map[string]any) map[string]any {
	title := strings.ToLower(stringValue(item, "title", ""))
	suggestion := strings.ToLower(stringValue(item, "agent_suggestion", ""))
	combined := title + " " + suggestion

	switch {
	case strings.Contains(combined, "翻译") || strings.Contains(combined, "translate"):
		return defaultIntentMap("translate")
	case strings.Contains(combined, "改写") || strings.Contains(combined, "rewrite"):
		return defaultIntentMap("rewrite")
	case strings.Contains(combined, "解释") || strings.Contains(combined, "explain"):
		return defaultIntentMap("explain")
	default:
		return defaultIntentMap("summarize")
	}
}
