// Package intent derives lightweight task suggestions from normalized context.
package intent

import (
	"path/filepath"
	"strings"
	"unicode/utf8"

	taskcontext "github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/textutil"
)

const (
	defaultAgentLoopIntent  = "agent_loop"
	subjectPreviewMaxLength = 24
)

// Suggestion is the minimum intent output required to create or continue a
// task in the main pipeline.
type Suggestion struct {
	Intent             map[string]any
	IntentConfirmed    bool
	TaskTitle          string
	TaskSourceType     string
	RequiresConfirm    bool
	DirectDeliveryType string
	ResultPreview      string
	ResultTitle        string
	ResultBubbleText   string
}

// Service maps context snapshots to lightweight intent suggestions.
// It stays read-only and leaves task mutation to the orchestrator/runengine.
type Service struct{}

// NewService constructs an intent suggestion service.
func NewService() *Service {
	return &Service{}
}

// Analyze performs the coarsest possible input gate for the main flow.
// The current pipeline only distinguishes missing input from actionable input.
func (s *Service) Analyze(input string) string {
	if strings.TrimSpace(input) == "" {
		return "waiting_input"
	}

	return "confirming_intent"
}

func (s *Service) AnalyzeSnapshot(snapshot taskcontext.TaskContextSnapshot) string {
	if strings.TrimSpace(snapshot.Text) == "" &&
		strings.TrimSpace(snapshot.SelectionText) == "" &&
		strings.TrimSpace(snapshot.ErrorText) == "" &&
		len(snapshot.Files) == 0 {
		return "waiting_input"
	}

	return "confirming_intent"
}

// Suggest derives a task suggestion from normalized context and an optional
// explicit intent payload.
func (s *Service) Suggest(snapshot taskcontext.TaskContextSnapshot, explicitIntent map[string]any, confirmRequired bool) Suggestion {
	intent := explicitIntent
	if len(intent) == 0 {
		intent = s.defaultIntent(snapshot)
	}

	intentName := stringValue(intent, "name")
	intentConfirmed := intentName != ""
	sourceType := sourceTypeFromSnapshot(snapshot)
	requiresConfirm := confirmRequired
	if !intentConfirmed {
		requiresConfirm = true
	}
	if intentName == "screen_analyze" {
		requiresConfirm = false
	}

	directDeliveryType := directDeliveryTypeForSnapshot(snapshot, intentName)
	resultPreview := previewForDeliveryType(directDeliveryType)

	return Suggestion{
		Intent:             intent,
		IntentConfirmed:    intentConfirmed,
		TaskTitle:          s.buildTaskTitle(snapshot, intentName),
		TaskSourceType:     sourceType,
		RequiresConfirm:    requiresConfirm,
		DirectDeliveryType: directDeliveryType,
		ResultPreview:      resultPreview,
		ResultTitle:        s.buildResultTitle(intentName),
		ResultBubbleText:   s.buildResultBubbleText(intentName),
	}
}

// defaultIntent chooses the minimum default route when the client does not provide
// an explicit intent payload. Free-form requests always fall back to the
// generic agent loop path so clarification stays in the main execution flow
// instead of growing an intent-side phrase router.
func (s *Service) defaultIntent(snapshot taskcontext.TaskContextSnapshot) map[string]any {
	if screenIntent, ok := screenAnalyzeIntent(snapshot); ok {
		return screenIntent
	}

	return intentPayload(defaultAgentLoopIntent)
}

// buildTaskTitle creates the user-facing task title that appears in task lists,
// dashboard modules, and later memory summaries.
func (s *Service) buildTaskTitle(snapshot taskcontext.TaskContextSnapshot, intentName string) string {
	subject := subjectText(snapshot)
	switch intentName {
	case "":
		return "确认处理方式：" + subject
	case defaultAgentLoopIntent:
		return "处理：" + subject
	case "screen_analyze":
		return "查看屏幕：" + screenSubjectText(snapshot)
	case "rewrite":
		return "改写：" + subject
	case "translate":
		return "翻译：" + subject
	case "explain":
		if snapshot.ErrorText != "" || snapshot.InputType == "error" {
			return "解释错误：" + subject
		}
		return "解释：" + subject
	case "summarize":
		if len(snapshot.Files) > 0 || snapshot.InputType == "file" {
			return "总结文件：" + subject
		}
		return "总结：" + subject
	default:
		return "处理：" + subject
	}
}

// buildResultTitle creates the formal delivery title used by delivery_result
// and artifact views.
func (s *Service) buildResultTitle(intentName string) string {
	switch intentName {
	case "":
		return "待确认处理方式"
	case defaultAgentLoopIntent:
		return "处理结果"
	case "screen_analyze":
		return "屏幕分析结果"
	case "rewrite":
		return "改写结果"
	case "translate":
		return "翻译结果"
	case "explain":
		return "解释结果"
	default:
		return "处理结果"
	}
}

// buildResultBubbleText generates the completion bubble text shown after
// delivery is ready.
func (s *Service) buildResultBubbleText(intentName string) string {
	switch intentName {
	case "":
		return "请先告诉我希望如何处理这段内容。"
	case defaultAgentLoopIntent:
		return "结果已经生成，可直接查看。"
	case "screen_analyze":
		return "已准备查看当前屏幕，等待授权后继续分析。"
	case "rewrite":
		return "内容已经按要求改写完成，可直接查看。"
	case "translate":
		return "翻译结果已经生成，可直接查看。"
	case "explain":
		return "这段内容的意思已经整理好了。"
	default:
		return "结果已经生成，可直接查看。"
	}
}

// sourceTypeFromSnapshot maps trigger-level input semantics into the stable
// task_source_type enum recorded by runengine.
func sourceTypeFromSnapshot(snapshot taskcontext.TaskContextSnapshot) string {
	switch snapshot.Trigger {
	case "voice_commit":
		return "voice"
	case "hover_text_input":
		return "hover_input"
	case "text_selected_click":
		return "selected_text"
	case "file_drop":
		return "dragged_file"
	case "error_detected":
		return "error_signal"
	case "recommendation_click":
		return "hover_input"
	default:
		if len(snapshot.Files) > 0 || snapshot.InputType == "file" {
			return "dragged_file"
		}
		if snapshot.ErrorText != "" || snapshot.InputType == "error" {
			return "error_signal"
		}
		if snapshot.SelectionText != "" || snapshot.InputType == "text_selection" {
			return "selected_text"
		}
		return "hover_input"
	}
}

func directDeliveryTypeForSnapshot(snapshot taskcontext.TaskContextSnapshot, intentName string) string {
	switch intentName {
	case defaultAgentLoopIntent:
		if len(snapshot.Files) > 0 || isLongContent(snapshot.SelectionText) || isLongContent(snapshot.Text) {
			return "workspace_document"
		}
	case "rewrite":
		return "workspace_document"
	case "summarize":
		if len(snapshot.Files) > 0 || isLongContent(snapshot.SelectionText) || isLongContent(snapshot.Text) {
			return "workspace_document"
		}
	case "translate":
		if len(snapshot.Files) > 0 {
			return "workspace_document"
		}
	case "screen_analyze":
		return "bubble"
	}
	return "bubble"
}

func previewForDeliveryType(deliveryType string) string {
	if deliveryType == "workspace_document" {
		return "已为你写入文档并打开"
	}
	return "结果已通过气泡返回"
}

func intentPayload(name string) map[string]any {
	switch name {
	case defaultAgentLoopIntent:
		return map[string]any{
			"name":      defaultAgentLoopIntent,
			"arguments": map[string]any{},
		}
	case "rewrite":
		return map[string]any{
			"name": "rewrite",
			"arguments": map[string]any{
				"tone": "professional",
			},
		}
	case "translate":
		return map[string]any{
			"name": "translate",
			"arguments": map[string]any{
				"target_language": "en",
			},
		}
	case "explain":
		return map[string]any{
			"name":      "explain",
			"arguments": map[string]any{},
		}
	case "screen_analyze":
		return map[string]any{
			"name": "screen_analyze",
			"arguments": map[string]any{
				"language":      "eng",
				"evidence_role": "error_evidence",
			},
		}
	default:
		return map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		}
	}
}

func subjectText(snapshot taskcontext.TaskContextSnapshot) string {
	switch {
	case len(snapshot.Files) > 0:
		return filepath.Base(snapshot.Files[0])
	case strings.TrimSpace(snapshot.SelectionText) != "":
		return truncateText(snapshot.SelectionText, subjectPreviewMaxLength)
	case strings.TrimSpace(snapshot.Text) != "":
		return truncateText(snapshot.Text, subjectPreviewMaxLength)
	case strings.TrimSpace(snapshot.ErrorText) != "":
		return truncateText(snapshot.ErrorText, subjectPreviewMaxLength)
	case strings.TrimSpace(snapshot.PageTitle) != "":
		return truncateText(snapshot.PageTitle, subjectPreviewMaxLength)
	case strings.TrimSpace(snapshot.WindowTitle) != "":
		return truncateText(snapshot.WindowTitle, subjectPreviewMaxLength)
	default:
		return "当前内容"
	}
}

func screenSubjectText(snapshot taskcontext.TaskContextSnapshot) string {
	switch {
	case strings.TrimSpace(snapshot.PageTitle) != "":
		return truncateText(snapshot.PageTitle, subjectPreviewMaxLength)
	case strings.TrimSpace(snapshot.WindowTitle) != "":
		return truncateText(snapshot.WindowTitle, subjectPreviewMaxLength)
	default:
		return subjectText(snapshot)
	}
}

func screenAnalyzeIntent(snapshot taskcontext.TaskContextSnapshot) (map[string]any, bool) {
	if !shouldUseScreenAnalyze(snapshot) {
		return nil, false
	}
	intent := intentPayload("screen_analyze")
	arguments := map[string]any{
		"language":      "eng",
		"evidence_role": screenEvidenceRole(snapshot),
	}
	if strings.TrimSpace(snapshot.PageTitle) != "" {
		arguments["page_title"] = snapshot.PageTitle
	}
	if strings.TrimSpace(snapshot.WindowTitle) != "" {
		arguments["window_title"] = snapshot.WindowTitle
	}
	if strings.TrimSpace(snapshot.VisibleText) != "" {
		arguments["visible_text"] = snapshot.VisibleText
	}
	if strings.TrimSpace(snapshot.ScreenSummary) != "" {
		arguments["screen_summary"] = snapshot.ScreenSummary
	}
	intent["arguments"] = arguments
	return intent, true
}

func shouldUseScreenAnalyze(snapshot taskcontext.TaskContextSnapshot) bool {
	if snapshot.InputType != "text" {
		return false
	}
	text := strings.TrimSpace(strings.ToLower(snapshot.Text))
	if text == "" {
		return false
	}
	hasVisualTarget := strings.TrimSpace(snapshot.PageTitle) != "" || strings.TrimSpace(snapshot.WindowTitle) != "" || strings.TrimSpace(snapshot.VisibleText) != "" || strings.TrimSpace(snapshot.ScreenSummary) != ""
	if !hasVisualTarget {
		return false
	}
	visualIntentMarkers := []string{"screen", "page", "window", "ui", "screenshot", "页面", "屏幕", "界面", "窗口", "报错", "错误"}
	analysisMarkers := []string{"look", "see", "check", "analyze", "inspect", "review", "看看", "查看", "分析", "检查", "识别", "定位", "解释"}
	return containsAny(text, visualIntentMarkers...) && containsAny(text, analysisMarkers...)
}

func screenEvidenceRole(snapshot taskcontext.TaskContextSnapshot) string {
	combined := strings.ToLower(strings.Join([]string{snapshot.Text, snapshot.ErrorText, snapshot.VisibleText, snapshot.ScreenSummary}, " "))
	if containsAny(combined, "error", "warning", "exception", "报错", "错误", "异常", "warning") {
		return "error_evidence"
	}
	return "page_context"
}

func containsAny(text string, markers ...string) bool {
	for _, marker := range markers {
		if marker != "" && strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func isLongContent(text string) bool {
	trimmed := strings.TrimSpace(text)
	return strings.Contains(trimmed, "\n") || utf8.RuneCountInString(trimmed) >= 80
}

func truncateText(value string, maxLength int) string {
	return textutil.TruncateGraphemes(value, maxLength)
}

// stringValue safely reads a string field from an intent payload.
func stringValue(values map[string]any, key string) string {
	rawValue, ok := values[key]
	if !ok {
		return ""
	}

	value, ok := rawValue.(string)
	if !ok {
		return ""
	}

	return value
}
