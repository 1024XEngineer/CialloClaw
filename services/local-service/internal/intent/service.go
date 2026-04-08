// 该文件负责任务意图的识别与建议生成。
package intent

import (
	"strings"

	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
)

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

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Analyze(input string) string {
	if strings.TrimSpace(input) == "" {
		return "waiting_input"
	}

	return "confirming_intent"
}

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
