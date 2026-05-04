package perception

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/textutil"
)

const (
	copyIntentThreshold         = 1
	activeDwellThresholdMillis  = 12000
	switchBurstThreshold        = 3
	opportunityPreviewMaxLength = 24
)

// SignalSnapshot captures normalized perception inputs behind the current
// public RPC boundary.
type SignalSnapshot struct {
	Source            string
	Scene             string
	PageTitle         string
	PageURL           string
	AppName           string
	WindowTitle       string
	VisibleText       string
	ScreenSummary     string
	SelectionText     string
	ClipboardText     string
	ClipboardMimeType string
	HoverTarget       string
	LastAction        string
	ErrorText         string
	DwellMillis       int
	WindowSwitchCount int
	PageSwitchCount   int
	CopyCount         int
}

// Opportunity describes one ranked collaboration opportunity inferred from the
// current perception snapshot.
type Opportunity struct {
	IntentName string
	Text       string
	Reason     string
	Priority   int
}

// CaptureContextSignals normalizes page, screen, clipboard, dwell, and switch
// signals from a best-effort context payload.
func CaptureContextSignals(source, scene string, context map[string]any) SignalSnapshot {
	page := mapValue(context, "page")
	if len(page) == 0 {
		page = mapValue(context, "page_context")
	}
	clipboard := mapValue(context, "clipboard")
	screen := mapValue(context, "screen")
	behavior := mapValue(context, "behavior")
	errorValue := mapValue(context, "error")

	return SignalSnapshot{
		Source:            firstNonEmpty(strings.TrimSpace(source), stringValue(context, "source")),
		Scene:             firstNonEmpty(strings.TrimSpace(scene), stringValue(context, "scene")),
		PageTitle:         firstNonEmpty(stringValue(context, "page_title"), stringValue(page, "title")),
		PageURL:           firstNonEmpty(stringValue(context, "page_url"), stringValue(page, "url")),
		AppName:           firstNonEmpty(stringValue(context, "app_name"), stringValue(page, "app_name")),
		WindowTitle:       firstNonEmpty(stringValue(context, "window_title"), stringValue(page, "window_title"), stringValue(screen, "window_title")),
		VisibleText:       firstNonEmpty(stringValue(context, "visible_text"), stringValue(page, "visible_text"), stringValue(screen, "visible_text")),
		ScreenSummary:     firstNonEmpty(stringValue(context, "screen_summary"), stringValue(screen, "summary"), stringValue(screen, "screen_summary")),
		SelectionText:     firstNonEmpty(stringValue(context, "selection_text"), stringValue(mapValue(context, "selection"), "text")),
		ClipboardText:     firstNonEmpty(stringValue(context, "clipboard_text"), stringValue(clipboard, "text")),
		ClipboardMimeType: firstNonEmpty(stringValue(context, "clipboard_mime_type"), stringValue(clipboard, "mime_type")),
		HoverTarget:       firstNonEmpty(stringValue(context, "hover_target"), stringValue(page, "hover_target"), stringValue(screen, "hover_target")),
		LastAction:        firstNonEmpty(stringValue(context, "last_action"), stringValue(behavior, "last_action")),
		ErrorText:         firstNonEmpty(stringValue(context, "error_text"), stringValue(errorValue, "message")),
		DwellMillis:       intValue(context, "dwell_millis", intValue(behavior, "dwell_millis", 0)),
		WindowSwitchCount: intValue(context, "window_switch_count", intValue(behavior, "window_switch_count", 0)),
		PageSwitchCount:   intValue(context, "page_switch_count", intValue(behavior, "page_switch_count", 0)),
		CopyCount:         intValue(context, "copy_count", intValue(behavior, "copy_count", 0)),
	}
}

// SignalFingerprint returns a stable string representation for perception
// cooldown and dedupe logic.
func SignalFingerprint(snapshot SignalSnapshot) string {
	textHash := sha256.Sum256([]byte(strings.Join([]string{
		strings.TrimSpace(snapshot.PageTitle),
		strings.TrimSpace(snapshot.PageURL),
		strings.TrimSpace(snapshot.WindowTitle),
		strings.TrimSpace(snapshot.SelectionText),
		strings.TrimSpace(snapshot.ClipboardText),
		strings.TrimSpace(snapshot.VisibleText),
		strings.TrimSpace(snapshot.ScreenSummary),
		strings.TrimSpace(snapshot.HoverTarget),
		strings.TrimSpace(snapshot.ErrorText),
	}, "|")))
	parts := []string{
		strings.TrimSpace(snapshot.Source),
		strings.TrimSpace(snapshot.Scene),
		strings.TrimSpace(snapshot.AppName),
		strings.TrimSpace(snapshot.LastAction),
		fmt.Sprintf("text=%x", textHash[:8]),
		"dwell=" + dwellBucket(snapshot.DwellMillis),
		"copy=" + activityBucket(snapshot.CopyCount),
		"window=" + switchBucket(snapshot.WindowSwitchCount),
		"page=" + switchBucket(snapshot.PageSwitchCount),
	}
	return strings.ToLower(strings.Join(parts, "|"))
}

// BehaviorSignals returns concise signal summaries that can feed dashboard or
// recommendation telemetry.
func BehaviorSignals(snapshot SignalSnapshot) []string {
	signals := make([]string, 0, 7)
	if hasCopyBehavior(snapshot) {
		signals = append(signals, "检测到最近的复制行为，可能存在整理或改写机会。")
	}
	if hasRichPageContext(snapshot) {
		signals = append(signals, "当前页面与屏幕上下文已具备可解析信号。")
	}
	if hasActiveDwell(snapshot) {
		signals = append(signals, "检测到用户在当前页面停留，适合提供轻量帮助。")
	}
	if hasSwitchBurst(snapshot) {
		signals = append(signals, "检测到频繁切换页面或窗口，可能需要整理上下文。")
	}
	if hasErrorOpportunity(snapshot) {
		signals = append(signals, "当前页面或屏幕内容中出现异常/错误信号。")
	}
	if strings.TrimSpace(snapshot.HoverTarget) != "" {
		signals = append(signals, "当前悬停对象已可作为下一步协助候选。")
	}
	return dedupeStrings(signals)
}

// IdentifyOpportunities converts the current perception snapshot into ranked
// collaboration opportunities.
func IdentifyOpportunities(snapshot SignalSnapshot, unfinishedTasks []runengine.TaskRecord, notepadItems []map[string]any) []Opportunity {
	opportunities := make([]Opportunity, 0, 6)
	if hasCopyBehavior(snapshot) {
		intentName, promptText := copyOpportunity(snapshot)
		opportunities = append(opportunities, Opportunity{
			IntentName: intentName,
			Text:       promptText,
			Reason:     "copy_behavior",
			Priority:   100,
		})
	}
	if hasErrorOpportunity(snapshot) {
		opportunities = append(opportunities, Opportunity{
			IntentName: "explain",
			Text:       "检测到当前页面可能存在错误，要不要我先解释原因和处理方向？",
			Reason:     "error_visible",
			Priority:   95,
		})
	}
	if hasActiveDwell(snapshot) {
		target := firstNonEmpty(snapshot.PageTitle, snapshot.WindowTitle, snapshot.HoverTarget, "当前内容")
		opportunities = append(opportunities, Opportunity{
			IntentName: inferredPageIntent(snapshot),
			Text:       fmt.Sprintf("你在这里停留了一段时间，要不要我先整理一下：%s", truncateText(target, opportunityPreviewMaxLength)),
			Reason:     "active_dwell",
			Priority:   80,
		})
	}
	if hasSwitchBurst(snapshot) {
		opportunities = append(opportunities, Opportunity{
			IntentName: "summarize",
			Text:       "最近页面或窗口切换较频繁，要不要我帮你汇总当前线索并给出下一步建议？",
			Reason:     "switch_burst",
			Priority:   75,
		})
	}
	if hasRichPageContext(snapshot) && len(unfinishedTasks) == 0 && len(notepadItems) == 0 && snapshot.Scene != "selected_text" {
		target := firstNonEmpty(snapshot.PageTitle, snapshot.WindowTitle, "当前页面")
		opportunities = append(opportunities, Opportunity{
			IntentName: inferredPageIntent(snapshot),
			Text:       fmt.Sprintf("要不要我基于当前页面做一个轻量整理：%s", truncateText(target, opportunityPreviewMaxLength)),
			Reason:     "page_context",
			Priority:   60,
		})
	}
	return rankOpportunities(opportunities)
}

func dwellBucket(dwellMillis int) string {
	switch {
	case dwellMillis >= activeDwellThresholdMillis:
		return "engaged"
	case dwellMillis > 0:
		return "brief"
	default:
		return "idle"
	}
}

func activityBucket(count int) string {
	if count > 0 {
		return "active"
	}
	return "idle"
}

func switchBucket(count int) string {
	if count >= switchBurstThreshold {
		return "burst"
	}
	if count > 0 {
		return "some"
	}
	return "none"
}

func rankOpportunities(opportunities []Opportunity) []Opportunity {
	if len(opportunities) == 0 {
		return nil
	}
	filtered := make([]Opportunity, 0, len(opportunities))
	seen := map[string]struct{}{}
	for _, opportunity := range opportunities {
		key := strings.TrimSpace(opportunity.IntentName) + "|" + strings.TrimSpace(opportunity.Reason)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		filtered = append(filtered, opportunity)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Priority != filtered[j].Priority {
			return filtered[i].Priority > filtered[j].Priority
		}
		return filtered[i].IntentName < filtered[j].IntentName
	})
	return filtered
}

func copyOpportunity(snapshot SignalSnapshot) (string, string) {
	text := firstNonEmpty(snapshot.SelectionText, snapshot.ClipboardText, snapshot.VisibleText)
	if shouldTranslate(text, snapshot.PageTitle, snapshot.ScreenSummary) {
		return "translate", fmt.Sprintf("你刚复制了一段内容，要不要我先帮你翻译：%s", truncateText(text, opportunityPreviewMaxLength))
	}
	if utf8.RuneCountInString(text) >= 96 || strings.Contains(text, "\n") {
		return "summarize", fmt.Sprintf("你刚复制了一段较长内容，要不要我先帮你提炼重点：%s", truncateText(text, opportunityPreviewMaxLength))
	}
	return "rewrite", fmt.Sprintf("你刚复制了一段内容，要不要我先帮你改写整理：%s", truncateText(text, opportunityPreviewMaxLength))
}

func inferredPageIntent(snapshot SignalSnapshot) string {
	if hasErrorOpportunity(snapshot) {
		return "explain"
	}
	if strings.TrimSpace(snapshot.VisibleText) != "" && utf8.RuneCountInString(snapshot.VisibleText) >= 80 {
		return "summarize"
	}
	return "explain"
}

func hasCopyBehavior(snapshot SignalSnapshot) bool {
	return snapshot.CopyCount >= copyIntentThreshold || strings.EqualFold(snapshot.LastAction, "copy")
}

func hasRichPageContext(snapshot SignalSnapshot) bool {
	return strings.TrimSpace(snapshot.PageTitle) != "" || strings.TrimSpace(snapshot.PageURL) != "" || strings.TrimSpace(snapshot.VisibleText) != "" || strings.TrimSpace(snapshot.ScreenSummary) != "" || strings.TrimSpace(snapshot.WindowTitle) != ""
}

func hasActiveDwell(snapshot SignalSnapshot) bool {
	return snapshot.DwellMillis >= activeDwellThresholdMillis && (hasRichPageContext(snapshot) || strings.TrimSpace(snapshot.HoverTarget) != "")
}

func hasSwitchBurst(snapshot SignalSnapshot) bool {
	return snapshot.WindowSwitchCount >= switchBurstThreshold || snapshot.PageSwitchCount >= switchBurstThreshold
}

func hasErrorOpportunity(snapshot SignalSnapshot) bool {
	combined := strings.ToLower(strings.Join([]string{snapshot.ErrorText, snapshot.VisibleText, snapshot.ScreenSummary, snapshot.PageTitle, snapshot.WindowTitle}, " "))
	for _, token := range []string{"error", "failed", "warning", "异常", "报错", "失败"} {
		if strings.Contains(combined, token) {
			return true
		}
	}
	return false
}

func shouldTranslate(values ...string) bool {
	text := strings.Join(values, " ")
	return containsCJK(text) && containsLatin(text)
}

func containsCJK(value string) bool {
	for _, r := range value {
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
	}
	return false
}

func containsLatin(value string) bool {
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

func mapValue(values map[string]any, key string) map[string]any {
	rawValue, ok := values[key]
	if !ok {
		return map[string]any{}
	}
	value, ok := rawValue.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return value
}

func stringValue(values map[string]any, key string) string {
	rawValue, ok := values[key]
	if !ok {
		return ""
	}
	value, ok := rawValue.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func intValue(values map[string]any, key string, fallback int) int {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func truncateText(value string, maxLength int) string {
	return textutil.TruncateGraphemes(value, maxLength)
}
