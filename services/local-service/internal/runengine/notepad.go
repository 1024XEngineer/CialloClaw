package runengine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
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
		"item_id":                stringValue(normalized, "item_id", ""),
		"title":                  stringValue(normalized, "title", ""),
		"bucket":                 stringValue(normalized, "bucket", ""),
		"status":                 stringValue(normalized, "status", "normal"),
		"type":                   stringValue(normalized, "type", ""),
		"agent_suggestion":       normalized["agent_suggestion"],
		"due_at":                 normalized["due_at"],
		"recurring_enabled":      normalized["recurring_enabled"],
		"note_text":              normalized["note_text"],
		"prerequisite":           normalized["prerequisite"],
		"repeat_rule":            normalized["repeat_rule_text"],
		"next_occurrence_at":     normalized["next_occurrence_at"],
		"recent_instance_status": normalized["recent_instance_status"],
		"effective_scope":        normalized["effective_scope"],
		"ended_at":               normalized["ended_at"],
		"related_resources":      protocolNotepadResourceList(normalized["related_resources"]),
		"linked_task_id":         normalized["linked_task_id"],
	}
	return result
}

func protocolNotepadResourceList(rawValue any) []map[string]any {
	resources := cloneResourceList(rawValue)
	if len(resources) == 0 {
		return nil
	}

	result := make([]map[string]any, 0, len(resources))
	for _, resource := range resources {
		projected := map[string]any{
			"resource_id":   firstNonEmpty(stringValue(resource, "resource_id", ""), stringValue(resource, "id", "")),
			"label":         stringValue(resource, "label", ""),
			"path":          stringValue(resource, "path", ""),
			"resource_type": firstNonEmpty(stringValue(resource, "resource_type", ""), stringValue(resource, "type", "")),
		}

		if openAction := firstNonEmpty(stringValue(resource, "open_action", ""), resourceTargetKindToOpenAction(stringValue(resource, "target_kind", ""))); openAction != "" {
			projected["open_action"] = openAction
		}
		if payload := protocolNotepadResourcePayload(projected["open_action"], projected["path"].(string)); len(payload) > 0 {
			projected["open_payload"] = payload
		}
		result = append(result, projected)
	}
	return result
}

func resourceTargetKindToOpenAction(targetKind string) string {
	switch strings.TrimSpace(targetKind) {
	case "file":
		return "open_file"
	case "folder":
		return "reveal_in_folder"
	default:
		return ""
	}
}

func protocolNotepadResourcePayload(action any, path string) map[string]any {
	actionValue, _ := action.(string)
	if strings.TrimSpace(path) == "" || strings.TrimSpace(actionValue) == "" {
		return nil
	}
	return map[string]any{
		"path":    path,
		"task_id": nil,
		"url":     nil,
	}
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

// SyncNotepadItems replaces the current notepad foundation state and persists it
// when a todo store is configured.
func (e *Engine) SyncNotepadItems(items []map[string]any) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.replaceNotepadItemsLocked(items)
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
	items := cloneMapSlice(e.notepadItems)
	items[index] = updated
	if err := e.replaceNotepadItemsLocked(items); err != nil {
		return nil, false
	}
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

	restoreNotepadItem(updated, e.now())
	items := cloneMapSlice(e.notepadItems)
	items[index] = updated
	if err := e.replaceNotepadItemsLocked(items); err != nil {
		return nil, false
	}
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
	items := cloneMapSlice(e.notepadItems)
	items = append(items[:index], items[index+1:]...)
	return e.replaceNotepadItemsLocked(items) == nil
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
	updated["updated_at"] = e.now().UTC().Format(time.RFC3339)
	if enabled {
		if nextOccurrence := deriveRecurringNextOccurrence(updated); nextOccurrence != "" {
			updated["due_at"] = nextOccurrence
		}
	} else {
		updated["recent_instance_status"] = "paused"
		updated["due_at"] = nil
	}
	items := cloneMapSlice(e.notepadItems)
	items[index] = updated
	if err := e.replaceNotepadItemsLocked(items); err != nil {
		return nil, false
	}
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
	updateRecurringRuleMetadata(updated)
	updated["updated_at"] = e.now().UTC().Format(time.RFC3339)
	items := cloneMapSlice(e.notepadItems)
	items[index] = updated
	if err := e.replaceNotepadItemsLocked(items); err != nil {
		return nil, false
	}
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
		item["previous_bucket"] = openBucket
	}
	if plannedAt := deriveNotepadPlannedAt(item); plannedAt != "" {
		item["planned_at"] = plannedAt
		item["previous_due_at"] = plannedAt
	}
	item["previous_status"] = stringValue(item, "status", "normal")
	item["bucket"] = notepadBucketClosed
	item["status"] = status
	item["ended_at"] = now.UTC().Format(time.RFC3339)
	item["due_at"] = nil
	item["updated_at"] = now.UTC().Format(time.RFC3339)
	if status == "cancelled" {
		item["recent_instance_status"] = "cancelled"
	}
}

func restoreNotepadItem(item map[string]any, now time.Time) {
	bucket := firstNonEmpty(stringValue(item, "previous_bucket", ""), stringValue(item, "source_bucket", ""))
	if bucket == "" {
		if stringValue(item, "type", "") == "recurring" || notepadBoolValue(item, "recurring_enabled", false) {
			bucket = notepadBucketRecurringRule
		} else {
			bucket = notepadBucketUpcoming
		}
	}
	item["bucket"] = bucket
	item["status"] = firstNonEmpty(stringValue(item, "previous_status", ""), "normal")
	item["ended_at"] = nil
	item["updated_at"] = now.UTC().Format(time.RFC3339)
	if bucket == notepadBucketRecurringRule {
		if nextOccurrence := deriveRecurringNextOccurrence(item); nextOccurrence != "" && notepadBoolValue(item, "recurring_enabled", true) {
			item["due_at"] = nextOccurrence
		}
		return
	}
	if plannedAt := firstNonEmpty(stringValue(item, "previous_due_at", ""), deriveNotepadPlannedAt(item)); plannedAt != "" {
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
	updateRecurringRuleMetadata(item)
	if ruleText := strings.TrimSpace(stringValue(item, "repeat_rule_text", "")); ruleText != "" {
		return ruleText
	}
	return "每周重复一次"
}

func deriveRecurringNextOccurrence(item map[string]any) string {
	if nextOccurrence := strings.TrimSpace(stringValue(item, "next_occurrence_at", "")); nextOccurrence != "" {
		return nextOccurrence
	}
	updateRecurringRuleMetadata(item)
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
	updateRecurringRuleMetadata(item)
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
	updateRecurringRuleMetadata(item)
	if effectiveScope := strings.TrimSpace(stringValue(item, "effective_scope", "")); effectiveScope != "" {
		return effectiveScope
	}
	if !notepadBoolValue(item, "recurring_enabled", true) {
		return "规则已暂停，不会生成新的巡检实例。"
	}
	return "在默认工作区巡检范围内持续生效。"
}

func updateRecurringRuleMetadata(item map[string]any) {
	ruleText := strings.TrimSpace(stringValue(item, "repeat_rule_text", ""))
	cronExpr := strings.TrimSpace(stringValue(item, "cron_expr", ""))
	intervalValue := itemIntValue(item, "interval_value", 0)
	intervalUnit := strings.TrimSpace(stringValue(item, "interval_unit", ""))
	if ruleText == "" && cronExpr == "" && intervalValue <= 0 && intervalUnit == "" {
		ruleText = "每周重复一次"
	}

	spec := parseRecurringRuleSpec(ruleText, cronExpr, intervalValue, intervalUnit)
	if spec.ruleType != "" {
		item["rule_type"] = spec.ruleType
	}
	if spec.cronExpr != "" {
		item["cron_expr"] = spec.cronExpr
	}
	if spec.intervalValue > 0 {
		item["interval_value"] = spec.intervalValue
	}
	if spec.intervalUnit != "" {
		item["interval_unit"] = spec.intervalUnit
	}
	if ruleText == "" {
		ruleText = recurringRuleTextFromSpec(spec)
	}
	if ruleText != "" {
		item["repeat_rule_text"] = ruleText
	}
	if stringValue(item, "reminder_strategy", "") == "" {
		item["reminder_strategy"] = "due_at"
	}
	if stringValue(item, "next_occurrence_at", "") == "" {
		if nextOccurrence := recurringNextOccurrenceFromSpec(item, spec); nextOccurrence != "" {
			item["next_occurrence_at"] = nextOccurrence
		}
	}
	if stringValue(item, "recent_instance_status", "") == "" {
		if !notepadBoolValue(item, "recurring_enabled", true) {
			item["recent_instance_status"] = "paused"
		} else if status := stringValue(item, "status", ""); status == "completed" || status == "cancelled" {
			item["recent_instance_status"] = status
		} else {
			item["recent_instance_status"] = "completed"
		}
	}
}

type recurringRuleSpec struct {
	ruleType      string
	cronExpr      string
	intervalValue int
	intervalUnit  string
}

func parseRecurringRuleSpec(ruleText, cronExpr string, intervalValue int, intervalUnit string) recurringRuleSpec {
	if strings.TrimSpace(cronExpr) != "" {
		return recurringRuleSpec{ruleType: "cron", cronExpr: strings.TrimSpace(cronExpr)}
	}
	if intervalValue > 0 && strings.TrimSpace(intervalUnit) != "" {
		return recurringRuleSpec{ruleType: "interval", intervalValue: intervalValue, intervalUnit: strings.TrimSpace(intervalUnit)}
	}
	normalized := strings.ToLower(strings.TrimSpace(ruleText))
	switch {
	case normalized == "", strings.Contains(normalized, "每周"), strings.Contains(normalized, "weekly"):
		value := 1
		if strings.Contains(normalized, "两") || strings.Contains(normalized, "biweekly") {
			value = 2
		}
		return recurringRuleSpec{ruleType: "interval", intervalValue: value, intervalUnit: "week"}
	case strings.Contains(normalized, "每天"), strings.Contains(normalized, "daily"):
		return recurringRuleSpec{ruleType: "interval", intervalValue: 1, intervalUnit: "day"}
	case strings.Contains(normalized, "每月"), strings.Contains(normalized, "monthly"):
		return recurringRuleSpec{ruleType: "interval", intervalValue: 1, intervalUnit: "month"}
	case strings.Contains(normalized, "每两周"):
		return recurringRuleSpec{ruleType: "interval", intervalValue: 2, intervalUnit: "week"}
	case strings.HasPrefix(normalized, "cron:"):
		return recurringRuleSpec{ruleType: "cron", cronExpr: strings.TrimSpace(strings.TrimPrefix(ruleText, "cron:"))}
	default:
		return recurringRuleSpec{ruleType: "interval", intervalValue: 1, intervalUnit: "week"}
	}
}

func recurringRuleTextFromSpec(spec recurringRuleSpec) string {
	if spec.ruleType == "cron" && spec.cronExpr != "" {
		return "cron: " + spec.cronExpr
	}
	if spec.intervalValue <= 1 {
		switch spec.intervalUnit {
		case "day":
			return "每天一次"
		case "month":
			return "每月一次"
		default:
			return "每周重复一次"
		}
	}
	return fmt.Sprintf("每%d%s一次", spec.intervalValue, recurringIntervalText(spec.intervalUnit))
}

func recurringIntervalText(unit string) string {
	switch unit {
	case "day":
		return "天"
	case "month":
		return "月"
	default:
		return "周"
	}
}

func recurringNextOccurrenceFromSpec(item map[string]any, spec recurringRuleSpec) string {
	base := strings.TrimSpace(stringValue(item, "planned_at", ""))
	if base == "" {
		base = strings.TrimSpace(stringValue(item, "due_at", ""))
	}
	if base == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, base)
	if err != nil {
		return base
	}
	if spec.ruleType == "interval" {
		switch spec.intervalUnit {
		case "day":
			parsed = parsed.Add(time.Duration(spec.intervalValue) * 24 * time.Hour)
		case "month":
			parsed = parsed.AddDate(0, spec.intervalValue, 0)
		default:
			parsed = parsed.AddDate(0, 0, 7*maxRecurringInterval(spec.intervalValue))
		}
	}
	return parsed.Format(time.RFC3339)
}

func maxRecurringInterval(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}

func deriveNotepadRelatedResources(item map[string]any) []map[string]any {
	if resources := cloneResourceList(item["related_resources"]); len(resources) > 0 {
		return resources
	}

	// These fallback resources exist for notepad browsing only. Downstream task
	// snapshots must still distinguish them from user-supplied execution inputs.
	resources := make([]map[string]any, 0, 2)
	title := strings.ToLower(strings.TrimSpace(stringValue(item, "title", "")))
	switch stringValue(item, "bucket", "") {
	case notepadBucketRecurringRule:
		resources = append(resources, map[string]any{
			"id":              stringValue(item, "item_id", "") + "_rule_source",
			"label":           "任务源目录",
			"path":            defaultTaskSourcePath,
			"type":            "directory",
			"target_kind":     "folder",
			"resource_origin": "derived_default",
		})
	case notepadBucketClosed:
		resources = append(resources, map[string]any{
			"id":              stringValue(item, "item_id", "") + "_archive",
			"label":           "归档目录",
			"path":            "workspace/archive",
			"type":            "directory",
			"target_kind":     "folder",
			"resource_origin": "derived_default",
		})
	}
	if strings.Contains(title, "模板") {
		resources = append(resources, map[string]any{
			"id":              stringValue(item, "item_id", "") + "_template",
			"label":           "关联模板",
			"path":            "workspace/templates",
			"type":            "directory",
			"target_kind":     "folder",
			"resource_origin": "derived_default",
		})
	}
	if strings.Contains(title, "周报") || strings.Contains(title, "报告") || strings.Contains(title, "评审") {
		resources = append(resources, map[string]any{
			"id":              stringValue(item, "item_id", "") + "_drafts",
			"label":           "草稿目录",
			"path":            "workspace/drafts",
			"type":            "directory",
			"target_kind":     "folder",
			"resource_origin": "derived_default",
		})
	}
	if len(resources) == 0 {
		resources = append(resources, map[string]any{
			"id":              stringValue(item, "item_id", "") + "_workspace",
			"label":           "默认工作区",
			"path":            defaultWorkspaceRoot,
			"type":            "directory",
			"target_kind":     "folder",
			"resource_origin": "derived_default",
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

func (e *Engine) replaceNotepadItemsLocked(items []map[string]any) error {
	items = cloneMapSlice(items)
	if err := e.persistNotepadItemsLocked(items); err != nil {
		return err
	}
	e.notepadItems = items
	return nil
}

func (e *Engine) persistNotepadItemsLocked(items []map[string]any) error {
	if e.todoStore == nil {
		return nil
	}
	records, rules, err := notepadItemsToStoreState(items, e.now())
	if err != nil {
		return err
	}
	if err := e.todoStore.ReplaceTodoState(context.Background(), records, rules); err != nil {
		return fmt.Errorf("replace todo state: %w", err)
	}
	return nil
}

func notepadItemsToStoreState(items []map[string]any, now time.Time) ([]storage.TodoItemRecord, []storage.RecurringRuleRecord, error) {
	itemRecords := make([]storage.TodoItemRecord, 0, len(items))
	ruleRecords := make([]storage.RecurringRuleRecord, 0)
	for _, item := range items {
		normalized := normalizeNotepadItem(item, now)
		if len(normalized) == 0 {
			continue
		}
		itemRecord, err := todoItemRecordFromMap(normalized, now)
		if err != nil {
			return nil, nil, err
		}
		itemRecords = append(itemRecords, itemRecord)
		if ruleRecord, ok, err := recurringRuleRecordFromMap(normalized, now); err != nil {
			return nil, nil, err
		} else if ok {
			ruleRecords = append(ruleRecords, ruleRecord)
		}
	}
	return itemRecords, ruleRecords, nil
}

func restoreNotepadItemsFromStore(items []storage.TodoItemRecord, rules []storage.RecurringRuleRecord) []map[string]any {
	if len(items) == 0 {
		return nil
	}
	rulesByItemID := make(map[string]storage.RecurringRuleRecord, len(rules))
	for _, rule := range rules {
		rulesByItemID[rule.ItemID] = rule
	}
	result := make([]map[string]any, 0, len(items))
	for _, record := range items {
		item := map[string]any{
			"item_id":          record.ItemID,
			"title":            record.Title,
			"bucket":           record.Bucket,
			"status":           record.Status,
			"type":             todoItemType(record.Bucket),
			"source_path":      record.SourcePath,
			"source_line":      record.SourceLine,
			"due_at":           nullableMapString(record.DueAt),
			"agent_suggestion": nullableMapString(record.AgentSuggestion),
			"note_text":        nullableMapString(record.NoteText),
			"prerequisite":     nullableMapString(record.Prerequisite),
			"planned_at":       nullableMapString(record.PlannedAt),
			"ended_at":         nullableMapString(record.EndedAt),
			"created_at":       record.CreatedAt,
			"updated_at":       record.UpdatedAt,
		}
		if record.SourceBucket != "" {
			item["source_bucket"] = record.SourceBucket
		}
		if record.PreviousBucket != "" {
			item["previous_bucket"] = record.PreviousBucket
		}
		if record.PreviousDueAt != "" {
			item["previous_due_at"] = record.PreviousDueAt
		}
		if record.PreviousStatus != "" {
			item["previous_status"] = record.PreviousStatus
		}
		if record.LinkedTaskID != "" {
			item["linked_task_id"] = record.LinkedTaskID
		}
		if resources := decodeRelatedResources(record.RelatedResourcesJSON); len(resources) > 0 {
			item["related_resources"] = resources
		}
		if tags := decodeTags(record.TagsJSON); len(tags) > 0 {
			item["tags"] = tags
		}
		if rule, ok := rulesByItemID[record.ItemID]; ok {
			item["rule_id"] = rule.RuleID
			item["rule_type"] = rule.RuleType
			item["cron_expr"] = nullableMapString(rule.CronExpr)
			item["interval_value"] = rule.IntervalValue
			item["interval_unit"] = nullableMapString(rule.IntervalUnit)
			item["reminder_strategy"] = rule.ReminderStrategy
			item["recurring_enabled"] = rule.Enabled
			item["repeat_rule_text"] = nullableMapString(rule.RepeatRuleText)
			item["next_occurrence_at"] = nullableMapString(rule.NextOccurrenceAt)
			item["recent_instance_status"] = nullableMapString(rule.RecentInstanceStatus)
			item["effective_scope"] = nullableMapString(rule.EffectiveScope)
		}
		result = append(result, item)
	}
	return result
}

func todoItemRecordFromMap(item map[string]any, now time.Time) (storage.TodoItemRecord, error) {
	relatedResourcesJSON, err := marshalRelatedResources(item["related_resources"])
	if err != nil {
		return storage.TodoItemRecord{}, err
	}
	tagsJSON, err := marshalStringSlice(item["tags"])
	if err != nil {
		return storage.TodoItemRecord{}, err
	}
	createdAt := stringValue(item, "created_at", "")
	if createdAt == "" {
		createdAt = now.UTC().Format(time.RFC3339)
	}
	updatedAt := stringValue(item, "updated_at", "")
	if updatedAt == "" {
		updatedAt = now.UTC().Format(time.RFC3339)
	}
	return storage.TodoItemRecord{
		ItemID:               stringValue(item, "item_id", ""),
		Title:                stringValue(item, "title", ""),
		Bucket:               stringValue(item, "bucket", ""),
		Status:               stringValue(item, "status", ""),
		SourcePath:           stringValue(item, "source_path", ""),
		SourceLine:           itemIntValue(item, "source_line", 0),
		SourceBucket:         stringValue(item, "source_bucket", ""),
		DueAt:                stringValue(item, "due_at", ""),
		TagsJSON:             tagsJSON,
		AgentSuggestion:      stringValue(item, "agent_suggestion", ""),
		NoteText:             stringValue(item, "note_text", ""),
		Prerequisite:         stringValue(item, "prerequisite", ""),
		PlannedAt:            stringValue(item, "planned_at", ""),
		PreviousBucket:       stringValue(item, "previous_bucket", ""),
		PreviousDueAt:        stringValue(item, "previous_due_at", ""),
		PreviousStatus:       stringValue(item, "previous_status", ""),
		EndedAt:              stringValue(item, "ended_at", ""),
		RelatedResourcesJSON: relatedResourcesJSON,
		LinkedTaskID:         stringValue(item, "linked_task_id", ""),
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
	}, nil
}

func recurringRuleRecordFromMap(item map[string]any, now time.Time) (storage.RecurringRuleRecord, bool, error) {
	if stringValue(item, "bucket", "") != notepadBucketRecurringRule && stringValue(item, "type", "") != "recurring" {
		return storage.RecurringRuleRecord{}, false, nil
	}
	updateRecurringRuleMetadata(item)
	createdAt := stringValue(item, "created_at", "")
	if createdAt == "" {
		createdAt = now.UTC().Format(time.RFC3339)
	}
	updatedAt := stringValue(item, "updated_at", "")
	if updatedAt == "" {
		updatedAt = now.UTC().Format(time.RFC3339)
	}
	ruleID := stringValue(item, "rule_id", "")
	if ruleID == "" {
		ruleID = fmt.Sprintf("rule_%s", stringValue(item, "item_id", ""))
	}
	return storage.RecurringRuleRecord{
		RuleID:               ruleID,
		ItemID:               stringValue(item, "item_id", ""),
		RuleType:             stringValue(item, "rule_type", "interval"),
		CronExpr:             stringValue(item, "cron_expr", ""),
		IntervalValue:        itemIntValue(item, "interval_value", 0),
		IntervalUnit:         stringValue(item, "interval_unit", ""),
		ReminderStrategy:     firstNonEmptyString(stringValue(item, "reminder_strategy", ""), "due_at"),
		Enabled:              notepadBoolValue(item, "recurring_enabled", true),
		RepeatRuleText:       stringValue(item, "repeat_rule_text", ""),
		NextOccurrenceAt:     stringValue(item, "next_occurrence_at", ""),
		RecentInstanceStatus: stringValue(item, "recent_instance_status", ""),
		EffectiveScope:       stringValue(item, "effective_scope", ""),
		CreatedAt:            createdAt,
		UpdatedAt:            updatedAt,
	}, true, nil
}

func marshalRelatedResources(rawValue any) (string, error) {
	resources := cloneResourceList(rawValue)
	if len(resources) == 0 {
		return "", nil
	}
	payload, err := json.Marshal(resources)
	if err != nil {
		return "", fmt.Errorf("marshal related resources: %w", err)
	}
	return string(payload), nil
}

func marshalStringSlice(rawValue any) (string, error) {
	values, ok := rawValue.([]string)
	if ok {
		if len(values) == 0 {
			return "", nil
		}
		payload, err := json.Marshal(values)
		if err != nil {
			return "", fmt.Errorf("marshal tags: %w", err)
		}
		return string(payload), nil
	}
	anyValues, ok := rawValue.([]any)
	if !ok {
		return "", nil
	}
	result := make([]string, 0, len(anyValues))
	for _, rawItem := range anyValues {
		item, ok := rawItem.(string)
		if ok && strings.TrimSpace(item) != "" {
			result = append(result, strings.TrimSpace(item))
		}
	}
	if len(result) == 0 {
		return "", nil
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("marshal tags: %w", err)
	}
	return string(payload), nil
}

func decodeRelatedResources(payload string) []map[string]any {
	if strings.TrimSpace(payload) == "" {
		return nil
	}
	var resources []map[string]any
	if err := json.Unmarshal([]byte(payload), &resources); err != nil {
		return nil
	}
	return cloneMapSlice(resources)
}

func decodeTags(payload string) []string {
	if strings.TrimSpace(payload) == "" {
		return nil
	}
	var tags []string
	if err := json.Unmarshal([]byte(payload), &tags); err != nil {
		return nil
	}
	return append([]string(nil), tags...)
}

func nullableMapString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func itemIntValue(values map[string]any, key string, fallback int) int {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}
	switch typed := rawValue.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return fallback
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func todoItemType(bucket string) string {
	if bucket == notepadBucketRecurringRule {
		return "recurring"
	}
	return "one_time"
}
