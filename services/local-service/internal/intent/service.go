// 该文件负责任务意图的识别与建议生成。
package intent

import (
	"strings"

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
	if !requiresConfirm {
		requiresConfirm = intentName == "summarize" && (snapshot.InputType == "file" || snapshot.InputType == "text_selection")
	}

	directDeliveryType := "bubble"
	resultPreview := "结果已通过气泡返回"
	if intentName == "summarize" || intentName == "rewrite" {
		directDeliveryType = "workspace_document"
		resultPreview = "已为你写入文档并打开"
	}

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
		return map[string]any{
			"name":      "explain",
			"arguments": map[string]any{},
		}
	}

	if len(snapshot.Files) > 0 || snapshot.InputType == "file" {
		return map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		}
	}

	if snapshot.SelectionText != "" || snapshot.InputType == "text_selection" {
		return map[string]any{
			"name":      "explain",
			"arguments": map[string]any{},
		}
	}

	return map[string]any{
		"name": "summarize",
		"arguments": map[string]any{
			"style": "key_points",
		},
	}
}

// buildTaskTitle 处理当前模块的相关逻辑。

// buildTaskTitle 生成面向 task 视角的标题文本。
// 标题会直接进入 task 列表、dashboard 和后续 memory 摘要，因此这里尽量保持面向用户可读。
func (s *Service) buildTaskTitle(snapshot contextsvc.TaskContextSnapshot, intentName string) string {
	switch intentName {
	case "rewrite":
		return "改写当前内容"
	case "translate":
		return "翻译当前内容"
	case "explain":
		if snapshot.ErrorText != "" || snapshot.InputType == "error" {
			return "解释当前错误信息"
		}
		return "解释当前选中内容"
	case "summarize":
		if len(snapshot.Files) > 0 || snapshot.InputType == "file" {
			return "整理并总结拖入文件"
		}
		return "总结当前内容"
	default:
		return "处理当前任务对象"
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
		return "hover_input"
	}
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
