// 该文件负责任务意图的识别与建议生成。
package intent

import (
	"path/filepath"
	"strings"
	"unicode/utf8"

	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
)

// Suggestion 定义当前模块的数据结构。

// Suggestion 描述 intent 模块对一次输入给出的最小建议结果。
// 它既包含建议采用的 intent，也包含主链路创建 task 时需要的标题、来源类型和默认交付信息。
type Suggestion struct {
	Intent             map[string]any
	TaskTitle          string
	TaskSourceType     string
	RequiresConfirm    bool
	DirectDeliveryType string
	ResultPreview      string
	ResultTitle        string
	ResultBubbleText   string
}

// Service 提供当前模块的服务能力。

// Service 负责把上下文快照映射成主链路可执行的意图建议。
// 这一层只做轻量规则判断，不直接修改任务状态，也不处理执行细节。
type Service struct{}

// NewService 创建并返回Service。

// NewService 创建意图识别服务。
func NewService() *Service {
	return &Service{}
}

// Analyze 处理当前模块的相关逻辑。

// Analyze 对输入内容做最粗粒度的入口判断。
// 当前主链路只区分“缺少输入，需要补充”和“已有输入，可以继续确认/执行”两种状态。
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

// Suggest 处理当前模块的相关逻辑。

// Suggest 根据上下文快照和可选的显式 intent 生成任务建议。
// 这里会统一决定任务标题、来源类型、是否需要确认，以及默认交付类型。
func (s *Service) Suggest(snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any, confirmRequired bool) Suggestion {
	intent := explicitIntent
	if len(intent) == 0 {
		intent = s.defaultIntent(snapshot)
	}

	intentName := stringValue(intent, "name")
	sourceType := sourceTypeFromSnapshot(snapshot)
	requiresConfirm := confirmRequired
	if !requiresConfirm && len(explicitIntent) == 0 {
		requiresConfirm = requiresConfirmation(snapshot, intentName)
	}

	directDeliveryType := directDeliveryTypeForSnapshot(snapshot, intentName)
	resultPreview := previewForDeliveryType(directDeliveryType)

	return Suggestion{
		Intent:             intent,
		TaskTitle:          s.buildTaskTitle(snapshot, intentName),
		TaskSourceType:     sourceType,
		RequiresConfirm:    requiresConfirm,
		DirectDeliveryType: directDeliveryType,
		ResultPreview:      resultPreview,
		ResultTitle:        s.buildResultTitle(intentName),
		ResultBubbleText:   s.buildResultBubbleText(intentName),
	}
}

// defaultIntent 处理当前模块的相关逻辑。

// defaultIntent 在前端没有显式传入 intent 时，根据输入对象推断一个默认意图。
// 这条默认路径服务于 P0 主链路的“先承接，再确认或执行”流程。
func (s *Service) defaultIntent(snapshot contextsvc.TaskContextSnapshot) map[string]any {
	if snapshot.ErrorText != "" || snapshot.InputType == "error" {
		return intentPayload("explain")
	}

	if detected := detectIntentFromText(snapshot.Text); detected != "" {
		return intentPayload(detected)
	}

	if len(snapshot.Files) > 0 || snapshot.InputType == "file" {
		return intentPayload("summarize")
	}

	if snapshot.SelectionText != "" || snapshot.InputType == "text_selection" {
		if isLongContent(snapshot.SelectionText) {
			return intentPayload("summarize")
		}
		return intentPayload("explain")
	}

	if isQuestionText(snapshot.Text) {
		return intentPayload("explain")
	}

	return intentPayload("summarize")
}

// buildTaskTitle 处理当前模块的相关逻辑。

// buildTaskTitle 生成面向 task 视角的标题文本。
// 标题会直接进入 task 列表、dashboard 和后续 memory 摘要，因此这里尽量保持面向用户可读。
func (s *Service) buildTaskTitle(snapshot contextsvc.TaskContextSnapshot, intentName string) string {
	subject := subjectText(snapshot)
	switch intentName {
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

// buildResultTitle 处理当前模块的相关逻辑。

// buildResultTitle 生成交付结果标题，用于 delivery_result 和 artifact 展示。
func (s *Service) buildResultTitle(intentName string) string {
	switch intentName {
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

// buildResultBubbleText 处理当前模块的相关逻辑。

// buildResultBubbleText 生成完成后的结果气泡文案。
func (s *Service) buildResultBubbleText(intentName string) string {
	switch intentName {
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

// sourceTypeFromSnapshot 处理当前模块的相关逻辑。

// sourceTypeFromSnapshot 把触发器枚举映射到 task_source_type。
// 这样 runengine 里记录的任务来源就能和协议里的统一枚举保持一致。
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

func directDeliveryTypeForSnapshot(snapshot contextsvc.TaskContextSnapshot, intentName string) string {
	switch intentName {
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

func detectIntentFromText(text string) string {
	value := strings.ToLower(strings.TrimSpace(text))
	switch {
	case value == "":
		return ""
	case strings.Contains(value, "翻译") || strings.HasPrefix(value, "translate") || strings.HasPrefix(value, "翻成"):
		return "translate"
	case strings.Contains(value, "改写") || strings.HasPrefix(value, "rewrite") || strings.Contains(value, "润色"):
		return "rewrite"
	case strings.Contains(value, "解释") || strings.HasPrefix(value, "explain") || isQuestionText(value):
		return "explain"
	default:
		return ""
	}
}

func intentPayload(name string) map[string]any {
	switch name {
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

// stringValue 处理当前模块的相关逻辑。

// stringValue 安全读取意图对象中的字符串字段。
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
