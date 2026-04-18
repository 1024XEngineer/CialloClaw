// Package intent derives lightweight task suggestions from normalized context.
package intent

import (
	"path/filepath"
	"strings"
	"unicode/utf8"

	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
)

const defaultAgentLoopIntent = "agent_loop"

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

func (s *Service) AnalyzeSnapshot(snapshot contextsvc.TaskContextSnapshot) string {
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
func (s *Service) Suggest(snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, confirmRequired bool) Suggestion {
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
	if !requiresConfirm && len(explicitIntent) == 0 {
		requiresConfirm = requiresConfirmation(snapshot, intentName)
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
// an explicit intent payload. The current correction path no longer classifies
// free-form requests into summarize / translate / explain via keyword matching.
// Instead, non-trivial inputs fall back to the generic agent loop path.
func (s *Service) defaultIntent(snapshot contextsvc.TaskContextSnapshot) map[string]any {
	if shouldConfirmTextGoal(snapshot) {
		return map[string]any{}
	}

	return intentPayload(defaultAgentLoopIntent)
}

// buildTaskTitle creates the user-facing task title that appears in task lists,
// dashboard modules, and later memory summaries.
func (s *Service) buildTaskTitle(snapshot contextsvc.TaskContextSnapshot, intentName string) string {
	subject := subjectText(snapshot)
	switch intentName {
	case "":
		return "确认处理方式：" + subject
	case defaultAgentLoopIntent:
		return "处理：" + subject
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
func sourceTypeFromSnapshot(snapshot contextsvc.TaskContextSnapshot) string {
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

func requiresConfirmation(snapshot contextsvc.TaskContextSnapshot, intentName string) bool {
	switch {
	case intentName == "":
		return true
	case intentName == defaultAgentLoopIntent:
		return false
	case snapshot.InputType == "file":
		return true
	case snapshot.InputType == "text_selection":
		return intentName != "translate"
	case isLongContent(snapshot.Text):
		return intentName == "summarize" || intentName == "rewrite"
	default:
		return false
	}
}

func shouldConfirmTextGoal(snapshot contextsvc.TaskContextSnapshot) bool {
	if snapshot.InputType != "text" {
		return false
	}
	trimmed := strings.TrimSpace(snapshot.Text)
	if trimmed == "" {
		return false
	}
	if isLongContent(trimmed) || isQuestionText(trimmed) {
		return false
	}
	return utf8.RuneCountInString(trimmed) <= 4
}

func directDeliveryTypeForSnapshot(snapshot contextsvc.TaskContextSnapshot, intentName string) string {
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
	default:
		return map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		}
	}
}

func subjectText(snapshot contextsvc.TaskContextSnapshot) string {
	switch {
	case len(snapshot.Files) > 0:
		return filepath.Base(snapshot.Files[0])
	case strings.TrimSpace(snapshot.SelectionText) != "":
		return truncateText(snapshot.SelectionText, 18)
	case strings.TrimSpace(snapshot.Text) != "":
		return truncateText(snapshot.Text, 18)
	case strings.TrimSpace(snapshot.ErrorText) != "":
		return truncateText(snapshot.ErrorText, 18)
	case strings.TrimSpace(snapshot.PageTitle) != "":
		return truncateText(snapshot.PageTitle, 18)
	default:
		return "当前内容"
	}
}

func isQuestionText(text string) bool {
	value := strings.TrimSpace(strings.ToLower(text))
	switch {
	case strings.Contains(value, "?"), strings.Contains(value, "？"),
		strings.Contains(value, "why"), strings.Contains(value, "how"),
		strings.Contains(value, "什么"), strings.Contains(value, "为什么"), strings.Contains(value, "怎么"):
		return true
	default:
		return false
	}
}

func isLongContent(text string) bool {
	trimmed := strings.TrimSpace(text)
	return strings.Contains(trimmed, "\n") || utf8.RuneCountInString(trimmed) >= 80
}

func truncateText(value string, maxLength int) string {
	if utf8.RuneCountInString(value) <= maxLength {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxLength]) + "..."
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
