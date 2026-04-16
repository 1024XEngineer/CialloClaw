package runengine

import (
	"strings"
	"time"
)

const (
	notepadBucketClosed        = "closed"
	notepadBucketLater         = "later"
	notepadBucketRecurringRule = "recurring_rule"
	notepadBucketUpcoming      = "upcoming"
)

// protocolNotepadItemMap projects the richer internal notepad model back to the
// frozen TodoItem RPC shape so owner-5 foundation work does not leak undeclared
// fields across the current protocol boundary.
func protocolNotepadItemMap(item map[string]any, now time.Time) map[string]any {
	normalized := normalizeNotepadItem(item, now)
	if len(normalized) == 0 {
		return nil
	}

	result := map[string]any{
		"item_id":          stringValue(normalized, "item_id", ""),
		"title":            stringValue(normalized, "title", ""),
		"bucket":           stringValue(normalized, "bucket", ""),
		"status":           stringValue(normalized, "status", "normal"),
		"type":             stringValue(normalized, "type", ""),
		"agent_suggestion": normalized["agent_suggestion"],
		"due_at":           normalized["due_at"],
	}
	return result
}

// normalizeNotepadItem enriches the internal note foundation fields that owner
// 5 can prepare ahead of protocol freeze, while keeping the existing TodoItem
// contract derivable through protocolNotepadItemMap.
func normalizeNotepadItem(item map[string]any, now time.Time) map[string]any {
	normalized := cloneMap(item)
	if len(normalized) == 0 {
		return nil
	}

	normalized["status"] = deriveNotepadStatus(normalized, now)
	if plannedAt := deriveNotepadPlannedAt(normalized); plannedAt != "" {
		normalized["planned_at"] = plannedAt
	}
	normalized["note_text"] = deriveNotepadNoteText(normalized)
	normalized["prerequisite"] = deriveNotepadPrerequisite(normalized)
	normalized["related_resources"] = deriveNotepadRelatedResources(normalized)
	normalized["ended_at"] = deriveNotepadEndedAt(normalized)
	if stringValue(normalized, "bucket", "") == notepadBucketRecurringRule {
		normalized["recurring_enabled"] = notepadBoolValue(normalized, "recurring_enabled", true)
		normalized["repeat_rule_text"] = deriveRecurringRuleText(normalized)
		normalized["next_occurrence_at"] = deriveRecurringNextOccurrence(normalized)
		normalized["recent_instance_status"] = deriveRecurringRecentStatus(normalized)
		normalized["effective_scope"] = deriveRecurringEffectiveScope(normalized)
	}
	return normalized
}

// CancelNotepadItem closes a note without deleting its foundation data so later
// restore/detail flows can still reference the original schedule and metadata.
func (e *Engine) CancelNotepadItem(itemID string) (map[string]any, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	updated, index, ok := e.updatedNotepadItem(itemID)
	if !ok {
		return nil, false
	}

	closeNotepadItem(updated, "cancelled", e.now())
	e.notepadItems[index] = updated
	return normalizeNotepadItem(updated, e.now()), true
}

// RestoreNotepadItem reopens a closed note using its preserved open-bucket and
// planned timing metadata.
func (e *Engine) RestoreNotepadItem(itemID string) (map[string]any, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	updated, index, ok := e.updatedNotepadItem(itemID)
	if !ok {
		return nil, false
	}

	restoreNotepadItem(updated)
	e.notepadItems[index] = updated
	return normalizeNotepadItem(updated, e.now()), true
}

// DeleteNotepadItem removes a note from the in-memory foundation store.
func (e *Engine) DeleteNotepadItem(itemID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	_, index, ok := e.findNotepadItem(itemID)
	if !ok {
		return false
	}
	e.notepadItems = append(e.notepadItems[:index], e.notepadItems[index+1:]...)
	return true
}

// SetNotepadRecurringEnabled toggles whether a recurring note should continue
// producing future occurrences.
func (e *Engine) SetNotepadRecurringEnabled(itemID string, enabled bool) (map[string]any, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	updated, index, ok := e.updatedNotepadItem(itemID)
	if !ok {
		return nil, false
	}
	if stringValue(updated, "bucket", "") != notepadBucketRecurringRule {
		return nil, false
	}

	updated["recurring_enabled"] = enabled
	if enabled {
		if nextOccurrence := deriveRecurringNextOccurrence(updated); nextOccurrence != "" {
			updated["due_at"] = nextOccurrence
		}
	} else {
		updated["recent_instance_status"] = "paused"
		updated["due_at"] = nil
	}
	e.notepadItems[index] = updated
	return normalizeNotepadItem(updated, e.now()), true
}

// UpdateNotepadRecurringRule refreshes the core rule fields that future detail
// read models or action RPCs can expose once owner-4 freezes the protocol.
func (e *Engine) UpdateNotepadRecurringRule(itemID, repeatRuleText, nextOccurrenceAt, effectiveScope string) (map[string]any, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	updated, index, ok := e.updatedNotepadItem(itemID)
	if !ok {
		return nil, false
	}
	if stringValue(updated, "bucket", "") != notepadBucketRecurringRule {
		return nil, false
	}

	if strings.TrimSpace(repeatRuleText) != "" {
		updated["repeat_rule_text"] = strings.TrimSpace(repeatRuleText)
	}
	if strings.TrimSpace(nextOccurrenceAt) != "" {
		updated["next_occurrence_at"] = strings.TrimSpace(nextOccurrenceAt)
		updated["planned_at"] = strings.TrimSpace(nextOccurrenceAt)
		if notepadBoolValue(updated, "recurring_enabled", true) {
			updated["due_at"] = strings.TrimSpace(nextOccurrenceAt)
		}
	}
	if strings.TrimSpace(effectiveScope) != "" {
		updated["effective_scope"] = strings.TrimSpace(effectiveScope)
	}
	e.notepadItems[index] = updated
	return normalizeNotepadItem(updated, e.now()), true
}

func (e *Engine) updatedNotepadItem(itemID string) (map[string]any, int, bool) {
	item, index, ok := e.findNotepadItem(itemID)
	if !ok {
		return nil, -1, false
	}
	return cloneMap(item), index, true
}

func closeNotepadItem(item map[string]any, status string, now time.Time) {
	if openBucket := stringValue(item, "bucket", ""); openBucket != "" && openBucket != notepadBucketClosed {
		item["source_bucket"] = openBucket
	}
	if plannedAt := deriveNotepadPlannedAt(item); plannedAt != "" {
		item["planned_at"] = plannedAt
	}
	item["bucket"] = notepadBucketClosed
	item["status"] = status
	item["ended_at"] = now.UTC().Format(time.RFC3339)
	item["due_at"] = nil
	if status == "cancelled" {
		item["recent_instance_status"] = "cancelled"
	}
}

func restoreNotepadItem(item map[string]any) {
	bucket := stringValue(item, "source_bucket", "")
	if bucket == "" {
		if stringValue(item, "type", "") == "recurring" || notepadBoolValue(item, "recurring_enabled", false) {
			bucket = notepadBucketRecurringRule
		} else {
			bucket = notepadBucketUpcoming
		}
	}
	item["bucket"] = bucket
	item["status"] = "normal"
	item["ended_at"] = nil
	if bucket == notepadBucketRecurringRule {
		if nextOccurrence := deriveRecurringNextOccurrence(item); nextOccurrence != "" && notepadBoolValue(item, "recurring_enabled", true) {
			item["due_at"] = nextOccurrence
		}
		return
	}
	if plannedAt := deriveNotepadPlannedAt(item); plannedAt != "" {
		item["due_at"] = plannedAt
	}
}

func deriveNotepadPlannedAt(item map[string]any) string {
	if plannedAt := stringValue(item, "planned_at", ""); plannedAt != "" {
		return plannedAt
	}
	if dueAt := stringValue(item, "due_at", ""); dueAt != "" {
		return dueAt
	}
	if nextOccurrence := stringValue(item, "next_occurrence_at", ""); nextOccurrence != "" {
		return nextOccurrence
	}
	return ""
}

func deriveNotepadNoteText(item map[string]any) string {
	if noteText := strings.TrimSpace(stringValue(item, "note_text", "")); noteText != "" {
		return noteText
	}
	title := strings.TrimSpace(stringValue(item, "title", "待办事项"))
	suggestion := strings.TrimSpace(stringValue(item, "agent_suggestion", ""))
	if suggestion != "" {
		return title + "。当前建议：" + suggestion + "。"
	}
	return title + "。当前处于便签巡检域，等待进入正式执行。"
}

func deriveNotepadPrerequisite(item map[string]any) string {
	if prerequisite := strings.TrimSpace(stringValue(item, "prerequisite", "")); prerequisite != "" {
		return prerequisite
	}
	switch stringValue(item, "bucket", "") {
	case notepadBucketLater:
		return "等进入处理窗口后再推进。"
	case notepadBucketRecurringRule:
		return "确认这条规则仍需持续生效，并保留对应资料入口。"
	default:
		return ""
	}
}

func deriveNotepadEndedAt(item map[string]any) any {
	if endedAt := stringValue(item, "ended_at", ""); endedAt != "" {
		return endedAt
	}
	if stringValue(item, "status", "") != "completed" && stringValue(item, "status", "") != "cancelled" {
		return nil
	}
	if bucket := stringValue(item, "bucket", ""); bucket != notepadBucketClosed {
		return nil
	}
	if dueAt := stringValue(item, "due_at", ""); dueAt != "" {
		return dueAt
	}
	return nil
}

func deriveRecurringRuleText(item map[string]any) string {
	if ruleText := strings.TrimSpace(stringValue(item, "repeat_rule_text", "")); ruleText != "" {
		return ruleText
	}
	return "每周重复一次"
}

func deriveRecurringNextOccurrence(item map[string]any) string {
	if nextOccurrence := strings.TrimSpace(stringValue(item, "next_occurrence_at", "")); nextOccurrence != "" {
		return nextOccurrence
	}
	if dueAt := strings.TrimSpace(stringValue(item, "due_at", "")); dueAt != "" {
		return dueAt
	}
	return strings.TrimSpace(stringValue(item, "planned_at", ""))
}

func deriveRecurringRecentStatus(item map[string]any) string {
	if recentStatus := strings.TrimSpace(stringValue(item, "recent_instance_status", "")); recentStatus != "" {
		return recentStatus
	}
	if !notepadBoolValue(item, "recurring_enabled", true) {
		return "paused"
	}
	return "completed"
}

func deriveRecurringEffectiveScope(item map[string]any) string {
	if effectiveScope := strings.TrimSpace(stringValue(item, "effective_scope", "")); effectiveScope != "" {
		return effectiveScope
	}
	if !notepadBoolValue(item, "recurring_enabled", true) {
		return "规则已暂停，不会生成新的巡检实例。"
	}
	return "在默认工作区巡检范围内持续生效。"
}

func deriveNotepadRelatedResources(item map[string]any) []map[string]any {
	if resources := cloneResourceList(item["related_resources"]); len(resources) > 0 {
		return resources
	}

	resources := make([]map[string]any, 0, 2)
	title := strings.ToLower(strings.TrimSpace(stringValue(item, "title", "")))
	switch stringValue(item, "bucket", "") {
	case notepadBucketRecurringRule:
		resources = append(resources, map[string]any{
			"id":          stringValue(item, "item_id", "") + "_rule_source",
			"label":       "任务源目录",
			"path":        defaultTaskSourcePath,
			"type":        "directory",
			"target_kind": "folder",
		})
	case notepadBucketClosed:
		resources = append(resources, map[string]any{
			"id":          stringValue(item, "item_id", "") + "_archive",
			"label":       "归档目录",
			"path":        "workspace/archive",
			"type":        "directory",
			"target_kind": "folder",
		})
	}
	if strings.Contains(title, "模板") {
		resources = append(resources, map[string]any{
			"id":          stringValue(item, "item_id", "") + "_template",
			"label":       "关联模板",
			"path":        "workspace/templates",
			"type":        "directory",
			"target_kind": "folder",
		})
	}
	if strings.Contains(title, "周报") || strings.Contains(title, "报告") || strings.Contains(title, "评审") {
		resources = append(resources, map[string]any{
			"id":          stringValue(item, "item_id", "") + "_drafts",
			"label":       "草稿目录",
			"path":        "workspace/drafts",
			"type":        "directory",
			"target_kind": "folder",
		})
	}
	if len(resources) == 0 {
		resources = append(resources, map[string]any{
			"id":          stringValue(item, "item_id", "") + "_workspace",
			"label":       "默认工作区",
			"path":        defaultWorkspaceRoot,
			"type":        "directory",
			"target_kind": "folder",
		})
	}
	return resources
}

func cloneResourceList(rawValue any) []map[string]any {
	resources, ok := rawValue.([]map[string]any)
	if ok {
		return cloneMapSlice(resources)
	}
	anyResources, ok := rawValue.([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]any, 0, len(anyResources))
	for _, rawResource := range anyResources {
		resource, ok := rawResource.(map[string]any)
		if ok {
			result = append(result, cloneMap(resource))
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func notepadBoolValue(values map[string]any, key string, fallback bool) bool {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}
	value, ok := rawValue.(bool)
	if !ok {
		return fallback
	}
	return value
}
