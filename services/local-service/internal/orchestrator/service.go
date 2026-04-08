package orchestrator

import (
	"errors"
	"fmt"
	"time"

	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/memory"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/risk"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

var ErrTaskNotFound = errors.New("task not found")

type Service struct {
	context   *contextsvc.Service
	intent    *intent.Service
	runEngine *runengine.Engine
	delivery  *delivery.Service
	memory    *memory.Service
	risk      *risk.Service
	model     *model.Service
	tools     *tools.Registry
	plugin    *plugin.Service
}

func NewService(
	context *contextsvc.Service,
	intent *intent.Service,
	runEngine *runengine.Engine,
	delivery *delivery.Service,
	memory *memory.Service,
	risk *risk.Service,
	model *model.Service,
	tools *tools.Registry,
	plugin *plugin.Service,
) *Service {
	return &Service{
		context:   context,
		intent:    intent,
		runEngine: runEngine,
		delivery:  delivery,
		memory:    memory,
		risk:      risk,
		model:     model,
		tools:     tools,
		plugin:    plugin,
	}
}

func (s *Service) Snapshot() map[string]any {
	return map[string]any{
		"context_source": s.context.Snapshot()["source"],
		"intent_state":   s.intent.Analyze("bootstrap"),
		"task_status":    s.runEngine.CurrentTaskStatus(),
		"run_state":      s.runEngine.CurrentState(),
		"delivery_type":  s.delivery.DefaultResultType(),
		"memory_backend": s.memory.RetrievalBackend(),
		"risk_level":     s.risk.DefaultLevel(),
		"model":          s.model.Descriptor(),
		"tool_count":     len(s.tools.Names()),
		"primary_worker": s.plugin.Workers()[0],
	}
}

func (s *Service) SubmitInput(params map[string]any) (map[string]any, error) {
	snapshot := s.context.Capture(params)
	options := mapValue(params, "options")
	confirmRequired := boolValue(options, "confirm_required", true)
	suggestion := s.intent.Suggest(snapshot, nil, confirmRequired)

	task := s.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   stringValue(params, "session_id", ""),
		Title:       suggestion.TaskTitle,
		SourceType:  suggestion.TaskSourceType,
		Status:      taskStatusForSuggestion(suggestion.RequiresConfirm),
		Intent:      suggestion.Intent,
		CurrentStep: currentStepForSuggestion(suggestion.RequiresConfirm),
		RiskLevel:   s.risk.DefaultLevel(),
		Timeline:    initialTimeline(taskStatusForSuggestion(suggestion.RequiresConfirm), currentStepForSuggestion(suggestion.RequiresConfirm)),
		Finished:    !suggestion.RequiresConfirm,
	})

	bubble := s.delivery.BuildBubbleMessage(task.TaskID, bubbleTypeForSuggestion(suggestion.RequiresConfirm), bubbleTextForInput(suggestion), task.StartedAt.Format(dateTimeLayout))
	deliveryResult := map[string]any(nil)
	artifacts := []map[string]any(nil)
	if !suggestion.RequiresConfirm {
		deliveryResult = s.delivery.BuildDeliveryResult(task.TaskID, suggestion.DirectDeliveryType, suggestion.ResultTitle, suggestion.ResultPreview)
		artifacts = s.delivery.BuildArtifact(task.TaskID, suggestion.ResultTitle, deliveryResult)
		if _, ok := s.runEngine.CompleteTask(task.TaskID, deliveryResult, bubble, artifacts); ok {
			task, _ = s.runEngine.GetTask(task.TaskID)
		}
	} else {
		if _, ok := s.runEngine.SetPresentation(task.TaskID, bubble, nil, nil); ok {
			task, _ = s.runEngine.GetTask(task.TaskID)
		}
	}

	response := map[string]any{
		"task":           taskMap(task),
		"bubble_message": bubble,
	}
	if deliveryResult != nil {
		response["delivery_result"] = deliveryResult
	}

	return response, nil
}

func (s *Service) StartTask(params map[string]any) (map[string]any, error) {
	snapshot := s.context.Capture(params)
	explicitIntent := mapValue(params, "intent")
	suggestion := s.intent.Suggest(snapshot, explicitIntent, len(explicitIntent) == 0)

	task := s.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   stringValue(params, "session_id", ""),
		Title:       suggestion.TaskTitle,
		SourceType:  suggestion.TaskSourceType,
		Status:      taskStatusForSuggestion(suggestion.RequiresConfirm),
		Intent:      suggestion.Intent,
		CurrentStep: currentStepForSuggestion(suggestion.RequiresConfirm),
		RiskLevel:   s.risk.DefaultLevel(),
		Timeline:    initialTimeline(taskStatusForSuggestion(suggestion.RequiresConfirm), currentStepForSuggestion(suggestion.RequiresConfirm)),
		Finished:    !suggestion.RequiresConfirm,
	})

	bubble := s.delivery.BuildBubbleMessage(task.TaskID, bubbleTypeForSuggestion(suggestion.RequiresConfirm), bubbleTextForStart(suggestion), task.StartedAt.Format(dateTimeLayout))
	response := map[string]any{
		"task":            taskMap(task),
		"bubble_message":  bubble,
		"delivery_result": nil,
	}

	if suggestion.RequiresConfirm {
		if _, ok := s.runEngine.SetPresentation(task.TaskID, bubble, nil, nil); ok {
			task, _ = s.runEngine.GetTask(task.TaskID)
			response["task"] = taskMap(task)
		}
		return response, nil
	}

	deliveryResult := s.delivery.BuildDeliveryResult(task.TaskID, suggestion.DirectDeliveryType, suggestion.ResultTitle, suggestion.ResultPreview)
	artifacts := s.delivery.BuildArtifact(task.TaskID, suggestion.ResultTitle, deliveryResult)
	if _, ok := s.runEngine.CompleteTask(task.TaskID, deliveryResult, bubble, artifacts); ok {
		task, _ = s.runEngine.GetTask(task.TaskID)
		response["task"] = taskMap(task)
	}
	response["delivery_result"] = deliveryResult
	return response, nil
}

func (s *Service) ConfirmTask(params map[string]any) (map[string]any, error) {
	taskID := stringValue(params, "task_id", "")
	task, ok := s.runEngine.GetTask(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}

	intentValue := mapValue(params, "corrected_intent")
	if len(intentValue) == 0 {
		intentValue = cloneMap(task.Intent)
	}

	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "已按新的要求开始处理", task.UpdatedAt.Format(dateTimeLayout))
	updatedTask, ok := s.runEngine.ConfirmTask(task.TaskID, intentValue, bubble)
	if !ok {
		return nil, ErrTaskNotFound
	}

	return map[string]any{
		"task":            taskMap(updatedTask),
		"bubble_message":  bubble,
		"delivery_result": nil,
	}, nil
}

func (s *Service) RecommendationGet(params map[string]any) (map[string]any, error) {
	selectionText := stringValue(mapValue(params, "context"), "selection_text", "当前内容")
	return map[string]any{
		"cooldown_hit": false,
		"items": []map[string]any{
			{
				"recommendation_id": "rec_001",
				"text":              fmt.Sprintf("要不要我帮你总结这段内容：%s", truncateText(selectionText, 16)),
				"intent":            defaultIntentMap("summarize"),
			},
			{
				"recommendation_id": "rec_002",
				"text":              "也可以直接改写成更正式的版本。",
				"intent":            defaultIntentMap("rewrite"),
			},
		},
	}, nil
}

func (s *Service) RecommendationFeedbackSubmit(params map[string]any) (map[string]any, error) {
	_ = params
	return map[string]any{"applied": true}, nil
}

func (s *Service) TaskList(params map[string]any) (map[string]any, error) {
	group := stringValue(params, "group", "unfinished")
	limit := intValue(params, "limit", 20)
	offset := intValue(params, "offset", 0)
	tasks, total := s.runEngine.ListTasks(group, limit, offset)

	items := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, taskMap(task))
	}

	return map[string]any{
		"items": items,
		"page":  pageMap(limit, offset, total),
	}, nil
}

func (s *Service) TaskDetailGet(params map[string]any) (map[string]any, error) {
	taskID := stringValue(params, "task_id", "")
	task, ok := s.runEngine.TaskDetail(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}

	return map[string]any{
		"task":              taskMap(task),
		"timeline":          timelineMap(task.Timeline),
		"artifacts":         cloneMapSlice(task.Artifacts),
		"mirror_references": cloneMapSlice(task.MirrorReferences),
		"security_summary":  cloneMap(task.SecuritySummary),
	}, nil
}

func (s *Service) TaskControl(params map[string]any) (map[string]any, error) {
	taskID := stringValue(params, "task_id", "")
	action := stringValue(params, "action", "pause")
	bubble := s.delivery.BuildBubbleMessage(taskID, "status", controlBubbleText(action), currentTimeFromTask(s.runEngine, taskID))
	updatedTask, ok := s.runEngine.ControlTask(taskID, action, bubble)
	if !ok {
		return nil, ErrTaskNotFound
	}

	return map[string]any{
		"task":           taskMap(updatedTask),
		"bubble_message": bubble,
	}, nil
}

func (s *Service) TaskInspectorConfigGet() (map[string]any, error) {
	return s.runEngine.InspectorConfig(), nil
}

func (s *Service) TaskInspectorConfigUpdate(params map[string]any) (map[string]any, error) {
	effective := s.runEngine.UpdateInspectorConfig(params)
	return map[string]any{
		"updated":          true,
		"effective_config": effective,
	}, nil
}

func (s *Service) TaskInspectorRun(params map[string]any) (map[string]any, error) {
	_ = params
	return map[string]any{
		"inspection_id": "insp_001",
		"summary": map[string]any{
			"parsed_files":     3,
			"identified_items": 12,
			"due_today":        2,
			"overdue":          1,
			"stale":            3,
		},
		"suggestions": []string{"优先处理今天到期的复盘邮件", "下周评审材料建议先生成草稿"},
	}, nil
}

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

func (s *Service) NotepadConvertToTask(params map[string]any) (map[string]any, error) {
	_ = params
	task := s.runEngine.CreateTask(runengine.CreateTaskInput{
		Title:       "整理 Q3 复盘要点",
		SourceType:  "todo",
		Status:      "confirming_intent",
		Intent:      defaultIntentMap("summarize"),
		CurrentStep: "intent_confirmation",
		RiskLevel:   s.risk.DefaultLevel(),
		Timeline:    initialTimeline("confirming_intent", "intent_confirmation"),
	})

	return map[string]any{
		"task": taskMap(task),
	}, nil
}

func (s *Service) DashboardOverviewGet(params map[string]any) (map[string]any, error) {
	_ = params
	tasks, _ := s.runEngine.ListTasks("unfinished", 1, 0)
	var focusSummary map[string]any
	if len(tasks) > 0 {
		focusSummary = map[string]any{
			"task_id":      tasks[0].TaskID,
			"title":        tasks[0].Title,
			"status":       tasks[0].Status,
			"current_step": tasks[0].CurrentStep,
			"next_action":  "等待用户查看结果",
			"updated_at":   tasks[0].UpdatedAt.Format(dateTimeLayout),
		}
	}

	return map[string]any{
		"overview": map[string]any{
			"focus_summary": focusSummary,
			"trust_summary": map[string]any{
				"risk_level":             s.risk.DefaultLevel(),
				"pending_authorizations": 0,
				"has_restore_point":      len(tasks) > 0 && tasks[0].SecuritySummary["latest_restore_point"] != nil,
				"workspace_path":         workspacePathFromSettings(s.runEngine.Settings()),
			},
			"quick_actions":     []string{"打开任务详情", "查看最近结果"},
			"global_state":      s.Snapshot(),
			"high_value_signal": []string{"主链路 task/run 映射已进入内存态运行。"},
		},
	}, nil
}

func (s *Service) DashboardModuleGet(params map[string]any) (map[string]any, error) {
	module := stringValue(params, "module", "mirror")
	tab := stringValue(params, "tab", "daily_summary")
	tasks, _ := s.runEngine.ListTasks("finished", 20, 0)
	return map[string]any{
		"module": module,
		"tab":    tab,
		"summary": map[string]any{
			"completed_tasks":     len(tasks),
			"generated_outputs":   len(tasks),
			"authorizations_used": 0,
			"exceptions":          0,
		},
		"highlights": []string{"主链路核心接口已通过同一 orchestrator 收口。"},
	}, nil
}

func (s *Service) MirrorOverviewGet(params map[string]any) (map[string]any, error) {
	_ = params
	tasks, _ := s.runEngine.ListTasks("finished", 20, 0)
	completedCount := len(tasks)
	if completedCount == 0 {
		completedCount = 1
	}
	return map[string]any{
		"history_summary": []string{"最近任务以文档总结与解释类需求为主。", "系统已经开始围绕 task 主对象组织返回。"},
		"daily_summary": map[string]any{
			"date":              time.Now().Format("2006-01-02"),
			"completed_tasks":   completedCount,
			"generated_outputs": completedCount,
		},
		"profile": map[string]any{
			"work_style":       "偏好结构化输出",
			"preferred_output": "3点摘要",
			"active_hours":     "10-12h",
		},
		"memory_references": []map[string]any{defaultMirrorReference()},
	}, nil
}

func (s *Service) SecuritySummaryGet() (map[string]any, error) {
	return map[string]any{
		"summary": map[string]any{
			"security_status":        "normal",
			"pending_authorizations": 0,
			"latest_restore_point":   nil,
			"token_cost_summary": map[string]any{
				"current_task_tokens":   2847,
				"current_task_cost":     0.12,
				"today_tokens":          9321,
				"today_cost":            0.46,
				"single_task_limit":     10.0,
				"daily_limit":           50.0,
				"budget_auto_downgrade": true,
			},
		},
	}, nil
}

func (s *Service) SecurityPendingList(params map[string]any) (map[string]any, error) {
	limit := intValue(params, "limit", 20)
	offset := intValue(params, "offset", 0)
	return map[string]any{
		"items": []map[string]any{},
		"page":  pageMap(limit, offset, 0),
	}, nil
}

func (s *Service) SecurityRespond(params map[string]any) (map[string]any, error) {
	taskID := stringValue(params, "task_id", "")
	task, ok := s.runEngine.GetTask(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}

	decision := stringValue(params, "decision", "allow_once")
	rememberRule := boolValue(params, "remember_rule", false)
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "已允许本次操作，任务继续执行", task.UpdatedAt.Format(dateTimeLayout))
	updatedTask, ok := s.runEngine.ControlTask(task.TaskID, "resume", bubble)
	if !ok {
		return nil, ErrTaskNotFound
	}

	return map[string]any{
		"authorization_record": map[string]any{
			"authorization_record_id": fmt.Sprintf("auth_%s", updatedTask.TaskID),
			"task_id":                 updatedTask.TaskID,
			"approval_id":             stringValue(params, "approval_id", "appr_001"),
			"decision":                decision,
			"remember_rule":           rememberRule,
			"operator":                "user",
			"created_at":              updatedTask.UpdatedAt.Format(dateTimeLayout),
		},
		"task":           taskMap(updatedTask),
		"bubble_message": bubble,
		"impact_scope": map[string]any{
			"files":                    []string{"D:/CialloClawWorkspace/report.md"},
			"webpages":                 []string{},
			"apps":                     []string{},
			"out_of_workspace":         false,
			"overwrite_or_delete_risk": false,
		},
	}, nil
}

func (s *Service) SettingsGet(params map[string]any) (map[string]any, error) {
	settings := s.runEngine.Settings()
	scope := stringValue(params, "scope", "all")
	if scope == "all" {
		return map[string]any{"settings": settings}, nil
	}

	section, ok := settings[scope].(map[string]any)
	if !ok {
		return map[string]any{"settings": map[string]any{}}, nil
	}

	return map[string]any{"settings": map[string]any{scope: cloneMap(section)}}, nil
}

func (s *Service) SettingsUpdate(params map[string]any) (map[string]any, error) {
	effectiveSettings, updatedKeys, applyMode, needRestart := s.runEngine.UpdateSettings(params)
	return map[string]any{
		"updated_keys":       updatedKeys,
		"effective_settings": effectiveSettings,
		"apply_mode":         applyMode,
		"need_restart":       needRestart,
	}, nil
}

func taskMap(record runengine.TaskRecord) map[string]any {
	result := map[string]any{
		"task_id":      record.TaskID,
		"title":        record.Title,
		"source_type":  record.SourceType,
		"status":       record.Status,
		"intent":       cloneMap(record.Intent),
		"current_step": record.CurrentStep,
		"risk_level":   record.RiskLevel,
		"started_at":   record.StartedAt.Format(dateTimeLayout),
		"updated_at":   record.UpdatedAt.Format(dateTimeLayout),
		"finished_at":  nil,
	}
	if record.FinishedAt != nil {
		result["finished_at"] = record.FinishedAt.Format(dateTimeLayout)
	}
	return result
}

func timelineMap(timeline []runengine.TaskStepRecord) []map[string]any {
	result := make([]map[string]any, 0, len(timeline))
	for _, step := range timeline {
		result = append(result, map[string]any{
			"step_id":        step.StepID,
			"task_id":        step.TaskID,
			"name":           step.Name,
			"status":         step.Status,
			"order_index":    step.OrderIndex,
			"input_summary":  step.InputSummary,
			"output_summary": step.OutputSummary,
		})
	}
	return result
}

func pageMap(limit, offset, total int) map[string]any {
	return map[string]any{
		"limit":    limit,
		"offset":   offset,
		"total":    total,
		"has_more": offset+limit < total,
	}
}

func taskStatusForSuggestion(requiresConfirm bool) string {
	if requiresConfirm {
		return "confirming_intent"
	}
	return "processing"
}

func currentStepForSuggestion(requiresConfirm bool) string {
	if requiresConfirm {
		return "intent_confirmation"
	}
	return "return_result"
}

func bubbleTypeForSuggestion(requiresConfirm bool) string {
	if requiresConfirm {
		return "intent_confirm"
	}
	return "result"
}

func bubbleTextForInput(suggestion intent.Suggestion) string {
	if suggestion.RequiresConfirm {
		return "你是想总结这段内容吗？"
	}
	return suggestion.ResultBubbleText
}

func bubbleTextForStart(suggestion intent.Suggestion) string {
	if suggestion.RequiresConfirm {
		return "你是想让我按当前对象继续处理吗？"
	}
	return suggestion.ResultBubbleText
}

func initialTimeline(status, currentStep string) []runengine.TaskStepRecord {
	stepStatus := "running"
	if status == "confirming_intent" {
		stepStatus = "pending"
	}
	return []runengine.TaskStepRecord{
		{
			StepID:        fmt.Sprintf("step_%s", currentStep),
			Name:          currentStep,
			Status:        stepStatus,
			OrderIndex:    1,
			InputSummary:  "已识别到当前任务对象",
			OutputSummary: "等待继续处理",
		},
	}
}

func controlBubbleText(action string) string {
	switch action {
	case "pause":
		return "任务已暂停"
	case "resume":
		return "任务已继续执行"
	case "cancel":
		return "任务已取消"
	case "restart":
		return "任务已重新开始"
	default:
		return "任务状态已更新"
	}
}

func currentTimeFromTask(engine *runengine.Engine, taskID string) string {
	task, ok := engine.GetTask(taskID)
	if !ok {
		return ""
	}
	return task.UpdatedAt.Format(dateTimeLayout)
}

func workspacePathFromSettings(settings map[string]any) string {
	general, ok := settings["general"].(map[string]any)
	if !ok {
		return "D:/CialloClawWorkspace"
	}
	download, ok := general["download"].(map[string]any)
	if !ok {
		return "D:/CialloClawWorkspace"
	}
	return stringValue(download, "workspace_path", "D:/CialloClawWorkspace")
}

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

func defaultMirrorReference() map[string]any {
	return map[string]any{
		"memory_id": "pref_001",
		"reason":    "当前任务命中了用户的输出偏好",
		"summary":   "偏好简洁三点式摘要",
	}
}

func cloneMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			result[key] = cloneMap(typed)
		case []map[string]any:
			result[key] = cloneMapSlice(typed)
		case []string:
			result[key] = append([]string(nil), typed...)
		default:
			result[key] = value
		}
	}
	return result
}

func cloneMapSlice(values []map[string]any) []map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		result = append(result, cloneMap(value))
	}
	return result
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

func stringValue(values map[string]any, key, fallback string) string {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}
	value, ok := rawValue.(string)
	if !ok || value == "" {
		return fallback
	}
	return value
}

func boolValue(values map[string]any, key string, fallback bool) bool {
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

func intValue(values map[string]any, key string, fallback int) int {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}
	value, ok := rawValue.(float64)
	if !ok {
		return fallback
	}
	return int(value)
}

func truncateText(value string, maxLength int) string {
	if len(value) <= maxLength {
		return value
	}
	return value[:maxLength] + "..."
}

const dateTimeLayout = time.RFC3339
