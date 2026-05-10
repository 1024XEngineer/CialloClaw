// Package presentation owns user-facing copy used by backend workflow outputs.
package presentation

import (
	"fmt"
	"strings"
)

const defaultLocale = "zh-CN"

// MessageKey is the stable semantic identifier for backend-owned presentation
// copy. Workflow packages should select keys and params instead of embedding
// display text in control flow.
type MessageKey string

// Message carries one renderable presentation message.
type Message struct {
	Key    MessageKey
	Params map[string]string
}

// ResultSpec groups the messages used when an intent resolves to a delivery.
type ResultSpec struct {
	Title      string
	Preview    string
	BubbleText string
}

// ResultSpecMessages keeps semantic keys available for tests and workflow
// builders before the final locale-specific strings are rendered.
type ResultSpecMessages struct {
	Title      Message
	Preview    Message
	BubbleText Message
}

// TaskTitleOptions describes input facts that affect title copy.
type TaskTitleOptions struct {
	Subject    string
	HasError   bool
	IsFile     bool
	IsScreen   bool
	ScreenMode string
}

const (
	MessageTaskTitleConfirm         MessageKey = "task.title.confirm"
	MessageTaskTitleRewrite         MessageKey = "task.title.rewrite"
	MessageTaskTitleTranslate       MessageKey = "task.title.translate"
	MessageTaskTitleExplain         MessageKey = "task.title.explain"
	MessageTaskTitleExplainError    MessageKey = "task.title.explain_error"
	MessageTaskTitleSummarize       MessageKey = "task.title.summarize"
	MessageTaskTitleSummarizeFile   MessageKey = "task.title.summarize_file"
	MessageTaskTitleGeneric         MessageKey = "task.title.generic"
	MessageTaskTitleScreenError     MessageKey = "task.title.screen_error"
	MessageTaskTitleScreenCurrent   MessageKey = "task.title.screen_current"
	MessageTaskTitleScreenFallback  MessageKey = "task.title.screen_fallback"
	MessageTaskTitleWaitingInput    MessageKey = "task.title.waiting_input"
	MessageTaskTitleNotepadItem     MessageKey = "task.title.notepad_item"
	MessageTaskTitleCurrentTask     MessageKey = "task.title.current_task"
	MessageTaskSubjectCurrentScreen MessageKey = "task.subject.current_screen"

	MessageResultTitlePending       MessageKey = "result.title.pending"
	MessageResultTitleGeneric       MessageKey = "result.title.generic"
	MessageResultTitleRewrite       MessageKey = "result.title.rewrite"
	MessageResultTitleTranslate     MessageKey = "result.title.translate"
	MessageResultTitleExplain       MessageKey = "result.title.explain"
	MessageResultTitleSummarize     MessageKey = "result.title.summarize"
	MessageResultTitlePageRead      MessageKey = "result.title.page_read"
	MessageResultTitlePageSearch    MessageKey = "result.title.page_search"
	MessageResultTitleBrowserAttach MessageKey = "result.title.browser_attach"
	MessageResultTitleBrowserSnap   MessageKey = "result.title.browser_snapshot"
	MessageResultTitleBrowserTabs   MessageKey = "result.title.browser_tabs"
	MessageResultTitleBrowserNav    MessageKey = "result.title.browser_navigate"
	MessageResultTitleBrowserFocus  MessageKey = "result.title.browser_focus"
	MessageResultTitleBrowserAct    MessageKey = "result.title.browser_interact"
	MessageResultTitleWriteFile     MessageKey = "result.title.write_file"
	MessageResultTitleScreen        MessageKey = "result.title.screen_analysis"
	MessageResultTitleTaskDelivery  MessageKey = "result.title.task_delivery"
	MessageResultTitleTaskResult    MessageKey = "result.title.task_result"

	MessagePreviewBubble       MessageKey = "result.preview.bubble"
	MessagePreviewWorkspaceDoc MessageKey = "result.preview.workspace_document"
	MessagePreviewScreenShot   MessageKey = "result.preview.screen_screenshot"
	MessagePreviewScreenClip   MessageKey = "result.preview.screen_clip"

	MessageBubbleResultGeneric         MessageKey = "bubble.result.generic"
	MessageBubbleResultRewrite         MessageKey = "bubble.result.rewrite"
	MessageBubbleResultTranslate       MessageKey = "bubble.result.translate"
	MessageBubbleResultExplain         MessageKey = "bubble.result.explain"
	MessageBubbleResultSummarize       MessageKey = "bubble.result.summarize"
	MessageBubbleResultPageRead        MessageKey = "bubble.result.page_read"
	MessageBubbleResultPageSearch      MessageKey = "bubble.result.page_search"
	MessageBubbleResultBrowserAttach   MessageKey = "bubble.result.browser_attach"
	MessageBubbleResultBrowserSnapshot MessageKey = "bubble.result.browser_snapshot"
	MessageBubbleResultBrowserTabs     MessageKey = "bubble.result.browser_tabs"
	MessageBubbleResultBrowserNavigate MessageKey = "bubble.result.browser_navigate"
	MessageBubbleResultBrowserFocus    MessageKey = "bubble.result.browser_focus"
	MessageBubbleResultBrowserInteract MessageKey = "bubble.result.browser_interact"
	MessageBubbleResultWriteFile       MessageKey = "bubble.result.write_file"
	MessageBubbleScreenDowngrade       MessageKey = "bubble.screen.downgraded"
	MessageBubbleScreenApproval        MessageKey = "bubble.screen.approval_required"
	MessageBubbleScreenReady           MessageKey = "bubble.screen.ready"
	MessageBubbleInputNeedGoal         MessageKey = "bubble.input.need_goal"
	MessageBubbleInputConfirmUnknown   MessageKey = "bubble.input.confirm_unknown"
	MessageBubbleStartConfirmUnknown   MessageKey = "bubble.start.confirm_unknown"
	MessageBubbleConfirmTranslate      MessageKey = "bubble.confirm.translate"
	MessageBubbleConfirmRewrite        MessageKey = "bubble.confirm.rewrite"
	MessageBubbleConfirmExplain        MessageKey = "bubble.confirm.explain"
	MessageBubbleConfirmSummarize      MessageKey = "bubble.confirm.summarize"
	MessageBubbleConfirmWriteFile      MessageKey = "bubble.confirm.write_file"
	MessageBubbleConfirmDefault        MessageKey = "bubble.confirm.default"
	MessageBubbleConfirmRejected       MessageKey = "bubble.confirm.rejected"
	MessageBubbleConfirmMissingIntent  MessageKey = "bubble.confirm.missing_intent"
	MessageBubbleConfirmStarted        MessageKey = "bubble.confirm.started"
	MessageBubbleGovernancePending     MessageKey = "bubble.governance.pending"
	MessageBubbleReviewReplan          MessageKey = "bubble.review.replan"
	MessageBubbleReviewContinue        MessageKey = "bubble.review.continue"
	MessageBubbleContinuationNeedMore  MessageKey = "bubble.continuation.need_more"
	MessageBubbleContinuationDefault   MessageKey = "bubble.continuation.default"
	MessageBubbleContinuationFiles     MessageKey = "bubble.continuation.files"
	MessageBubbleContinuationSelection MessageKey = "bubble.continuation.selection"
	MessageBubbleContinuationError     MessageKey = "bubble.continuation.error"
	MessageBubbleContinuationText      MessageKey = "bubble.continuation.text"
	MessageBubbleSteeringRecorded      MessageKey = "bubble.steering.recorded"
	MessageBubbleTaskPaused            MessageKey = "bubble.task.paused"
	MessageBubbleTaskResumed           MessageKey = "bubble.task.resumed"
	MessageBubbleTaskCancelled         MessageKey = "bubble.task.cancelled"
	MessageBubbleTaskRestarted         MessageKey = "bubble.task.restarted"
	MessageBubbleTaskUpdated           MessageKey = "bubble.task.updated"
	MessageBubbleQueueWait             MessageKey = "bubble.queue.wait"
	MessageBubbleQueueResume           MessageKey = "bubble.queue.resume"
	MessageBubbleAuthorizationDenied   MessageKey = "bubble.authorization.denied"
	MessageBubbleAuthorizationAllowed  MessageKey = "bubble.authorization.allowed"

	MessageTimelineWaiting      MessageKey = "timeline.waiting"
	MessageTimelineWaitingInput MessageKey = "timeline.waiting_input"
	MessageTimelineInputSeen    MessageKey = "timeline.input_seen"
	MessageTimelineStartOutput  MessageKey = "timeline.start_output"

	MessageExecutionFailureCheckpoint MessageKey = "execution.failure.checkpoint"
	MessageExecutionFailureBoundary   MessageKey = "execution.failure.boundary"
	MessageExecutionFailureCommand    MessageKey = "execution.failure.command"
	MessageExecutionFailureTimeout    MessageKey = "execution.failure.timeout"
	MessageExecutionFailureCanceled   MessageKey = "execution.failure.canceled"
	MessageExecutionFailurePlatform   MessageKey = "execution.failure.platform"
	MessageExecutionFailureTool       MessageKey = "execution.failure.tool"
	MessageExecutionFailureGeneric    MessageKey = "execution.failure.generic"
	MessageExecutionFailureModelSetup MessageKey = "execution.failure.model_setup"
	MessageExecutionFailureToolCall   MessageKey = "execution.failure.model_tool_call"
	MessageExecutionFailureInvalid    MessageKey = "execution.failure.model_invalid"
	MessageExecutionFailureModelTime  MessageKey = "execution.failure.model_timeout"
	MessageExecutionFailureRequest    MessageKey = "execution.failure.model_request"
	MessageExecutionFailureRejected   MessageKey = "execution.failure.model_rejected"
	MessageExecutionFailureAuth       MessageKey = "execution.failure.model_auth"
	MessageExecutionFailureEndpoint   MessageKey = "execution.failure.model_endpoint"
	MessageExecutionFailureRate       MessageKey = "execution.failure.model_rate"
	MessageExecutionFailureUpstream   MessageKey = "execution.failure.model_upstream"
	MessageExecutionFailureModel      MessageKey = "execution.failure.model_generic"

	MessageBudgetDowngradeProvider MessageKey = "budget.downgrade.provider_unavailable"
	MessageBudgetDowngradeFailure  MessageKey = "budget.downgrade.provider_failure"
	MessageBudgetDowngradePressure MessageKey = "budget.downgrade.resource_pressure"

	MessageFallbackNoInput        MessageKey = "execution.fallback.no_input"
	MessageFallbackClarify        MessageKey = "execution.fallback.clarify"
	MessageFallbackRewriteHeader  MessageKey = "execution.fallback.rewrite_header"
	MessageFallbackTranslate      MessageKey = "execution.fallback.translate"
	MessageFallbackExplainHeader  MessageKey = "execution.fallback.explain_header"
	MessageFallbackSummarizeTitle MessageKey = "execution.fallback.summarize_title"
	MessageFallbackSummarizeEmpty MessageKey = "execution.fallback.summarize_empty"

	MessageDocumentEmpty        MessageKey = "document.empty"
	MessageDocumentDefaultTitle MessageKey = "document.default_title"

	MessagePreviewGenerated          MessageKey = "preview.generated"
	MessagePreviewWorkspaceGenerated MessageKey = "preview.workspace_generated"
	MessageBubbleGenerated           MessageKey = "bubble.result.generated"

	MessageBubbleWriteFileReady        MessageKey = "bubble.write_file.ready"
	MessageBubbleScreenOCRUnavailable  MessageKey = "bubble.screen.ocr_unavailable"
	MessageBubbleScreenAnalyzed        MessageKey = "bubble.screen.analyzed"
	MessageToolBubbleGeneric           MessageKey = "tool.bubble.generic"
	MessageToolBubbleSearchMatches     MessageKey = "tool.bubble.search_matches"
	MessageToolBubbleBrowserAttach     MessageKey = "tool.bubble.browser_attach"
	MessageToolBubbleBrowserAttachHere MessageKey = "tool.bubble.browser_attach_current"
	MessageToolBubbleBrowserFocus      MessageKey = "tool.bubble.browser_focus"
	MessageToolBubbleBrowserFocused    MessageKey = "tool.bubble.browser_focused"
	MessageToolBubbleBrowserTabsCount  MessageKey = "tool.bubble.browser_tabs_count"
	MessageToolBubbleBrowserTabsReady  MessageKey = "tool.bubble.browser_tabs_ready"
	MessageToolBubbleDirectoryEntries  MessageKey = "tool.bubble.directory_entries"
)

var zhCNMessages = map[MessageKey]string{
	MessageTaskTitleConfirm:         "确认处理方式：{subject}",
	MessageTaskTitleRewrite:         "改写：{subject}",
	MessageTaskTitleTranslate:       "翻译：{subject}",
	MessageTaskTitleExplain:         "解释：{subject}",
	MessageTaskTitleExplainError:    "解释错误：{subject}",
	MessageTaskTitleSummarize:       "总结：{subject}",
	MessageTaskTitleSummarizeFile:   "总结文件：{subject}",
	MessageTaskTitleGeneric:         "处理：{subject}",
	MessageTaskTitleScreenError:     "查看屏幕报错：{subject}",
	MessageTaskTitleScreenCurrent:   "查看当前屏幕：{subject}",
	MessageTaskTitleScreenFallback:  "处理：{subject}",
	MessageTaskTitleWaitingInput:    "等待补充输入",
	MessageTaskTitleNotepadItem:     "待办事项",
	MessageTaskTitleCurrentTask:     "当前任务",
	MessageTaskSubjectCurrentScreen: "当前屏幕",

	MessageResultTitlePending:       "待确认处理方式",
	MessageResultTitleGeneric:       "处理结果",
	MessageResultTitleRewrite:       "改写结果",
	MessageResultTitleTranslate:     "翻译结果",
	MessageResultTitleExplain:       "解释结果",
	MessageResultTitleSummarize:     "总结结果",
	MessageResultTitlePageRead:      "网页读取结果",
	MessageResultTitlePageSearch:    "网页搜索结果",
	MessageResultTitleBrowserAttach: "浏览器附着结果",
	MessageResultTitleBrowserSnap:   "浏览器快照结果",
	MessageResultTitleBrowserTabs:   "浏览器标签页结果",
	MessageResultTitleBrowserNav:    "浏览器导航结果",
	MessageResultTitleBrowserFocus:  "浏览器切页结果",
	MessageResultTitleBrowserAct:    "浏览器交互结果",
	MessageResultTitleWriteFile:     "文件写入结果",
	MessageResultTitleScreen:        "屏幕分析结果",
	MessageResultTitleTaskDelivery:  "任务交付结果",
	MessageResultTitleTaskResult:    "任务结果",

	MessagePreviewBubble:       "结果已通过气泡返回",
	MessagePreviewWorkspaceDoc: "已为你写入文档并打开",
	MessagePreviewScreenShot:   "已准备分析屏幕截图",
	MessagePreviewScreenClip:   "已准备分析屏幕录屏片段",

	MessageBubbleResultGeneric:         "结果已经生成，可直接查看。",
	MessageBubbleResultRewrite:         "内容已经按要求改写完成，可直接查看。",
	MessageBubbleResultTranslate:       "翻译结果已经生成，可直接查看。",
	MessageBubbleResultExplain:         "这段内容的意思已经整理好了。",
	MessageBubbleResultSummarize:       "总结结果已经生成，可直接查看。",
	MessageBubbleResultPageRead:        "网页主要内容已经整理完成，可直接查看。",
	MessageBubbleResultPageSearch:      "网页搜索结果已经返回，可直接查看。",
	MessageBubbleResultBrowserAttach:   "当前浏览器页已经附着成功，可继续操作。",
	MessageBubbleResultBrowserSnapshot: "当前浏览器页的关键信息已经整理完成，可直接查看。",
	MessageBubbleResultBrowserTabs:     "当前浏览器标签页列表已经返回，可直接查看。",
	MessageBubbleResultBrowserNavigate: "当前浏览器页已经导航完成，可继续查看。",
	MessageBubbleResultBrowserFocus:    "目标浏览器标签页已经切换完成，可继续查看。",
	MessageBubbleResultBrowserInteract: "当前浏览器页交互已经完成，可继续查看。",
	MessageBubbleResultWriteFile:       "文件已经生成，可直接查看。",
	MessageBubbleScreenDowngrade:       "当前环境暂不支持受控屏幕查看，已改为按现有文本和页面上下文继续处理。",
	MessageBubbleScreenApproval:        "屏幕截图分析属于敏感能力，请先确认授权。",
	MessageBubbleScreenReady:           "已准备查看当前屏幕，等待授权后继续分析。",
	MessageBubbleInputNeedGoal:         "请先告诉我你希望我处理什么内容。",
	MessageBubbleInputConfirmUnknown:   "我还不确定你想如何处理这段内容，请确认目标。",
	MessageBubbleStartConfirmUnknown:   "我还不确定你想如何处理当前对象，请先确认。",
	MessageBubbleConfirmTranslate:      "你是想翻译这段内容吗？",
	MessageBubbleConfirmRewrite:        "你是想改写这段内容吗？",
	MessageBubbleConfirmExplain:        "你是想解释这段内容吗？",
	MessageBubbleConfirmSummarize:      "你是想总结这段内容吗？",
	MessageBubbleConfirmWriteFile:      "你是想把结果整理成文档吗？",
	MessageBubbleConfirmDefault:        "请确认你希望我如何处理当前内容。",
	MessageBubbleConfirmRejected:       "这不是我该做的处理方式。请重新说明你的目标，或给我一个更准确的处理意图。",
	MessageBubbleConfirmMissingIntent:  "请先明确告诉我你希望执行的处理方式。",
	MessageBubbleConfirmStarted:        "已按新的要求开始处理",
	MessageBubbleGovernancePending:     "检测到待授权操作，请先确认。",
	MessageBubbleReviewReplan:          "人工复核要求重新规划，请确认新的处理意图。",
	MessageBubbleReviewContinue:        "人工复核完成，任务继续执行。",
	MessageBubbleContinuationNeedMore:  "已把补充内容挂回当前任务，请继续补充剩余信息。",
	MessageBubbleContinuationDefault:   "已把补充内容挂回当前任务。",
	MessageBubbleContinuationFiles:     "已把 {count} 个补充文件挂回当前任务。",
	MessageBubbleContinuationSelection: "已把补充选中文本挂回当前任务。",
	MessageBubbleContinuationError:     "已把补充报错信息挂回当前任务。",
	MessageBubbleContinuationText:      "已把补充说明挂回当前任务。",
	MessageBubbleSteeringRecorded:      "已记录新的补充要求，后续执行会纳入该指令。",
	MessageBubbleTaskPaused:            "任务已暂停",
	MessageBubbleTaskResumed:           "任务已继续执行",
	MessageBubbleTaskCancelled:         "任务已取消",
	MessageBubbleTaskRestarted:         "任务已重新开始",
	MessageBubbleTaskUpdated:           "任务状态已更新",
	MessageBubbleQueueWait:             "当前会话已有任务 {task_title} 正在执行，本任务已排队等待。",
	MessageBubbleQueueResume:           "前序任务已完成，当前会话中的下一个任务开始执行。",
	MessageBubbleAuthorizationDenied:   "已拒绝本次操作，任务已取消。",
	MessageBubbleAuthorizationAllowed:  "已允许本次操作，任务继续执行。",

	MessageTimelineWaiting:      "等待继续处理",
	MessageTimelineWaitingInput: "等待用户补充输入",
	MessageTimelineInputSeen:    "已识别到当前任务对象",
	MessageTimelineStartOutput:  "开始生成正式结果",

	MessageExecutionFailureCheckpoint: "执行失败：执行前恢复点创建失败，请稍后重试。",
	MessageExecutionFailureBoundary:   "执行失败：目标超出工作区边界，已阻止本次操作。",
	MessageExecutionFailureCommand:    "执行失败：命令存在高危风险，已被策略拦截。",
	MessageExecutionFailureTimeout:    "执行失败：本地任务执行超时，请重试。",
	MessageExecutionFailureCanceled:   "执行失败：本地任务已取消。",
	MessageExecutionFailurePlatform:   "执行失败：当前平台能力不可用，请检查环境后重试。",
	MessageExecutionFailureTool:       "执行失败：工具运行失败，请检查环境后重试。",
	MessageExecutionFailureGeneric:    "执行失败：请稍后重试。",
	MessageExecutionFailureModelSetup: "执行失败：当前模型未完成配置，请检查 Provider、Base URL、Model 和 API Key。",
	MessageExecutionFailureToolCall:   "执行失败：当前模型接口不支持工具调用，请切换到兼容工具调用的模型或关闭相关工具路径。",
	MessageExecutionFailureInvalid:    "执行失败：模型返回内容无法解析，请检查上游接口兼容性。",
	MessageExecutionFailureModelTime:  "执行失败：模型请求超时，请稍后重试。",
	MessageExecutionFailureRequest:    "执行失败：模型请求发送失败，请检查网络连接或上游地址。",
	MessageExecutionFailureRejected:   "执行失败：模型请求被上游拒绝{detail}，请检查输入内容、模型能力和接口兼容性。",
	MessageExecutionFailureAuth:       "执行失败：模型鉴权失败{detail}，请检查 API Key 或访问权限。",
	MessageExecutionFailureEndpoint:   "执行失败：模型接口不存在{detail}，请检查 Base URL 或接口兼容性。",
	MessageExecutionFailureRate:       "执行失败：模型请求过于频繁{detail}，请稍后重试。",
	MessageExecutionFailureUpstream:   "执行失败：模型服务暂时不可用{detail}，请稍后重试。",
	MessageExecutionFailureModel:      "执行失败：模型调用失败{detail}。",

	MessageBudgetDowngradeProvider: "预算降级已生效：当前模型提供方不可用，任务改走轻量交付路径。",
	MessageBudgetDowngradeFailure:  "预算降级已生效：最近出现模型/提供方失败，任务改走轻量保守执行路径。",
	MessageBudgetDowngradePressure: "预算降级已生效：当前任务命中 token/成本压力，改为轻量交付并压缩上下文。",

	MessageFallbackNoInput:        "无可用输入",
	MessageFallbackClarify:        "我还不确定你希望我怎么处理这段内容，请补充你的目标，例如解释、翻译、改写或总结。",
	MessageFallbackRewriteHeader:  "改写结果：",
	MessageFallbackTranslate:      "翻译结果（回退模式，目标语言：{target_language}）：",
	MessageFallbackExplainHeader:  "解释结果：",
	MessageFallbackSummarizeTitle: "总结结果：",
	MessageFallbackSummarizeEmpty: "- 暂无可总结内容",

	MessageDocumentEmpty:        "暂无内容",
	MessageDocumentDefaultTitle: "处理结果",

	MessagePreviewGenerated:          "结果已生成",
	MessagePreviewWorkspaceGenerated: "已生成正式文档：{preview}",
	MessageBubbleGenerated:           "结果已生成。",

	MessageBubbleWriteFileReady:        "结果已写入 {path}，可直接查看。",
	MessageBubbleScreenOCRUnavailable:  "未识别到可用屏幕文本。",
	MessageBubbleScreenAnalyzed:        "已分析屏幕内容：{summary}",
	MessageToolBubbleGeneric:           "{tool_name} 执行完成。",
	MessageToolBubbleSearchMatches:     "页面搜索完成，关键词 {query} 共匹配 {count} 处。",
	MessageToolBubbleBrowserAttach:     "已定位浏览器标签页：{title}。",
	MessageToolBubbleBrowserAttachHere: "已定位当前浏览器标签页。",
	MessageToolBubbleBrowserFocus:      "已切换到浏览器标签页：{title}。",
	MessageToolBubbleBrowserFocused:    "目标浏览器标签页已经切换完成。",
	MessageToolBubbleBrowserTabsCount:  "当前浏览器共有 {count} 个标签页可用。",
	MessageToolBubbleBrowserTabsReady:  "当前浏览器标签页列表已经返回。",
	MessageToolBubbleDirectoryEntries:  "{tool_name} 执行完成，当前目录条目数：{count}。",
}

// Render returns the locale-specific copy for a semantic message key.
func Render(locale string, message Message) string {
	template := messageTemplate(locale, message.Key)
	if template == "" {
		return string(message.Key)
	}
	for key, value := range message.Params {
		template = strings.ReplaceAll(template, "{"+key+"}", value)
	}
	return template
}

// Text renders a message using the default backend locale.
func Text(key MessageKey, params map[string]string) string {
	return Render(defaultLocale, Message{Key: key, Params: params})
}

// TaskTitle renders a task title from intent and context facts.
func TaskTitle(intentName string, options TaskTitleOptions) string {
	return Render(defaultLocale, TaskTitleMessage(intentName, options))
}

// TaskTitleMessage returns the semantic key for a task title.
func TaskTitleMessage(intentName string, options TaskTitleOptions) Message {
	subject := strings.TrimSpace(options.Subject)
	if subject == "" {
		subject = Text(MessageTaskTitleCurrentTask, nil)
	}
	params := map[string]string{"subject": subject}
	switch intentName {
	case "":
		return Message{Key: MessageTaskTitleConfirm, Params: params}
	case "rewrite":
		return Message{Key: MessageTaskTitleRewrite, Params: params}
	case "translate":
		return Message{Key: MessageTaskTitleTranslate, Params: params}
	case "explain":
		if options.HasError {
			return Message{Key: MessageTaskTitleExplainError, Params: params}
		}
		return Message{Key: MessageTaskTitleExplain, Params: params}
	case "summarize":
		if options.IsFile {
			return Message{Key: MessageTaskTitleSummarizeFile, Params: params}
		}
		return Message{Key: MessageTaskTitleSummarize, Params: params}
	case "screen_analyze":
		if options.HasError {
			return Message{Key: MessageTaskTitleScreenError, Params: params}
		}
		return Message{Key: MessageTaskTitleScreenCurrent, Params: params}
	default:
		return Message{Key: MessageTaskTitleGeneric, Params: params}
	}
}

// TaskTitlePrefixes returns rendered prefixes used by legacy title parsing.
func TaskTitlePrefixes() []string {
	keys := []MessageKey{
		MessageTaskTitleConfirm,
		MessageTaskTitleRewrite,
		MessageTaskTitleTranslate,
		MessageTaskTitleExplainError,
		MessageTaskTitleExplain,
		MessageTaskTitleSummarizeFile,
		MessageTaskTitleSummarize,
		MessageTaskTitleGeneric,
		MessageTaskTitleScreenError,
		MessageTaskTitleScreenCurrent,
	}
	prefixes := make([]string, 0, len(keys))
	for _, key := range keys {
		template := messageTemplate(defaultLocale, key)
		if prefix, _, ok := strings.Cut(template, "{subject}"); ok {
			prefixes = append(prefixes, prefix)
		}
	}
	return prefixes
}

// RenderResultSpec renders all user-facing delivery strings for an intent.
func RenderResultSpec(intentName string) ResultSpec {
	messages := ResultSpecMessagesForIntent(intentName)
	return ResultSpec{
		Title:      Render(defaultLocale, messages.Title),
		Preview:    Render(defaultLocale, messages.Preview),
		BubbleText: Render(defaultLocale, messages.BubbleText),
	}
}

// RenderApprovalResultSpec renders delivery copy for authorization resumes.
func RenderApprovalResultSpec(intentName string) ResultSpec {
	if intentName == "summarize" {
		messages := resultSpecMessages(MessageResultTitleSummarize, MessagePreviewWorkspaceDoc, MessageBubbleResultSummarize)
		return ResultSpec{
			Title:      Render(defaultLocale, messages.Title),
			Preview:    Render(defaultLocale, messages.Preview),
			BubbleText: Render(defaultLocale, messages.BubbleText),
		}
	}
	return RenderResultSpec(intentName)
}

// ResultSpecMessagesForIntent returns the semantic keys for delivery copy.
func ResultSpecMessagesForIntent(intentName string) ResultSpecMessages {
	switch intentName {
	case "":
		return resultSpecMessages(MessageResultTitlePending, MessagePreviewWorkspaceDoc, MessageBubbleInputNeedGoal)
	case "agent_loop":
		return resultSpecMessages(MessageResultTitleGeneric, MessagePreviewBubble, MessageBubbleResultGeneric)
	case "rewrite":
		return resultSpecMessages(MessageResultTitleRewrite, MessagePreviewWorkspaceDoc, MessageBubbleResultRewrite)
	case "translate":
		return resultSpecMessages(MessageResultTitleTranslate, MessagePreviewBubble, MessageBubbleResultTranslate)
	case "explain":
		return resultSpecMessages(MessageResultTitleExplain, MessagePreviewBubble, MessageBubbleResultExplain)
	case "page_read":
		return resultSpecMessages(MessageResultTitlePageRead, MessagePreviewBubble, MessageBubbleResultPageRead)
	case "page_search":
		return resultSpecMessages(MessageResultTitlePageSearch, MessagePreviewBubble, MessageBubbleResultPageSearch)
	case "browser_attach_current":
		return resultSpecMessages(MessageResultTitleBrowserAttach, MessagePreviewBubble, MessageBubbleResultBrowserAttach)
	case "browser_snapshot":
		return resultSpecMessages(MessageResultTitleBrowserSnap, MessagePreviewBubble, MessageBubbleResultBrowserSnapshot)
	case "browser_tabs_list":
		return resultSpecMessages(MessageResultTitleBrowserTabs, MessagePreviewBubble, MessageBubbleResultBrowserTabs)
	case "browser_navigate":
		return resultSpecMessages(MessageResultTitleBrowserNav, MessagePreviewBubble, MessageBubbleResultBrowserNavigate)
	case "browser_tab_focus":
		return resultSpecMessages(MessageResultTitleBrowserFocus, MessagePreviewBubble, MessageBubbleResultBrowserFocus)
	case "browser_interact":
		return resultSpecMessages(MessageResultTitleBrowserAct, MessagePreviewBubble, MessageBubbleResultBrowserInteract)
	case "screen_analyze":
		return resultSpecMessages(MessageResultTitleScreen, MessagePreviewBubble, MessageBubbleScreenReady)
	case "write_file":
		return resultSpecMessages(MessageResultTitleWriteFile, MessagePreviewWorkspaceDoc, MessageBubbleResultWriteFile)
	default:
		return resultSpecMessages(MessageResultTitleGeneric, MessagePreviewWorkspaceDoc, MessageBubbleResultGeneric)
	}
}

// DeliveryPreviewText renders the default preview for the delivery channel.
func DeliveryPreviewText(deliveryType string) string {
	if deliveryType == "bubble" {
		return Text(MessagePreviewBubble, nil)
	}
	return Text(MessagePreviewWorkspaceDoc, nil)
}

// ScreenPreviewText renders the authorization preview for a screen mode.
func ScreenPreviewText(captureMode string) string {
	if strings.TrimSpace(captureMode) == "clip" {
		return Text(MessagePreviewScreenClip, nil)
	}
	return Text(MessagePreviewScreenShot, nil)
}

// CountParam formats count values for message interpolation.
func CountParam(count int) map[string]string {
	return map[string]string{"count": fmt.Sprintf("%d", count)}
}

// DetailParam formats optional provider details for presentation templates.
func DetailParam(detail string) map[string]string {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return map[string]string{"detail": ""}
	}
	return map[string]string{"detail": "（" + detail + "）"}
}

func resultSpecMessages(title, preview, bubble MessageKey) ResultSpecMessages {
	return ResultSpecMessages{
		Title:      Message{Key: title},
		Preview:    Message{Key: preview},
		BubbleText: Message{Key: bubble},
	}
}

func messageTemplate(locale string, key MessageKey) string {
	switch locale {
	case "", defaultLocale:
		return zhCNMessages[key]
	default:
		return zhCNMessages[key]
	}
}
