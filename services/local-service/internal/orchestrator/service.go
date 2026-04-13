// 该文件负责 4 号主链路的任务编排与对外语义收口。
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/execution"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/memory"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/recommendation"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/risk"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskinspector"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

// ErrTaskNotFound 定义当前模块的基础变量。

// ErrTaskNotFound 表示调用方给出的 task_id 在当前运行态中不存在。
var (
	ErrTaskNotFound          = errors.New("task not found")
	ErrTaskStatusInvalid     = errors.New("task status invalid")
	ErrTaskAlreadyFinished   = errors.New("task already finished")
	ErrStorageQueryFailed    = errors.New("storage query failed")
	ErrRecoveryPointNotFound = errors.New("recovery point not found")
)

// Service 提供当前模块的服务能力。

// Service 是 4 号后端 Harness 主链路的统一编排入口。
// 所有稳定的 task-centric RPC 方法都会在这里汇合，并继续拆分到 context、intent、runengine、delivery 等子模块。
type Service struct {
	context        *contextsvc.Service
	intent         *intent.Service
	runEngine      *runengine.Engine
	delivery       *delivery.Service
	memory         *memory.Service
	risk           *risk.Service
	model          *model.Service
	tools          *tools.Registry
	plugin         *plugin.Service
	audit          *audit.Service
	recommendation *recommendation.Service
	executor       *execution.Service
	inspector      *taskinspector.Service
	storage        *storage.Service
}

// NewService 创建并返回Service。

// NewService 组装主链路编排服务依赖。
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
		context:        context,
		intent:         intent,
		runEngine:      runEngine,
		delivery:       delivery,
		memory:         memory,
		risk:           risk,
		model:          model,
		tools:          tools,
		plugin:         plugin,
		audit:          audit.NewService(),
		recommendation: recommendation.NewService(),
		inspector:      taskinspector.NewService(nil),
	}
}

// WithAudit 挂接共享审计服务，避免运行态出现独立计数器。
func (s *Service) WithAudit(auditService *audit.Service) *Service {
	if auditService != nil {
		s.audit = auditService
	}
	return s
}

// WithExecutor 把真实执行服务挂入 orchestrator。
func (s *Service) WithExecutor(executorService *execution.Service) *Service {
	s.executor = executorService
	return s
}

// WithTaskInspector 挂接任务巡检运行态服务。
func (s *Service) WithTaskInspector(inspectorService *taskinspector.Service) *Service {
	if inspectorService != nil {
		s.inspector = inspectorService
	}
	return s
}

// WithStorage 挂接共享 storage 服务，用于治理数据读侧回填。
func (s *Service) WithStorage(storageService *storage.Service) *Service {
	if storageService != nil {
		s.storage = storageService
	}
	return s
}

// Snapshot 处理当前模块的相关逻辑。

// Snapshot 返回 orchestrator 当前用于调试和健康检查的最小概览。
func (s *Service) Snapshot() map[string]any {
	pendingApprovals, pendingTotal := s.runEngine.PendingApprovalRequests(100, 0)
	return map[string]any{
		"context_source":          s.context.Snapshot()["source"],
		"intent_state":            s.intent.Analyze("bootstrap"),
		"task_status":             s.runEngine.CurrentTaskStatus(),
		"run_state":               s.runEngine.CurrentState(),
		"delivery_type":           s.delivery.DefaultResultType(),
		"memory_backend":          s.memory.RetrievalBackend(),
		"risk_level":              s.risk.DefaultLevel(),
		"model":                   s.model.Descriptor(),
		"tool_count":              len(s.tools.Names()),
		"primary_worker":          s.plugin.Workers()[0],
		"pending_approvals":       pendingTotal,
		"latest_approval_request": firstMapOrNil(pendingApprovals),
	}
}

// SubmitInput 处理当前模块的相关逻辑。

// SubmitInput 处理 agent.input.submit。
// 这条路径负责承接悬浮球文本输入，根据上下文生成意图建议，并决定进入确认态、等待输入态还是直接执行。
func (s *Service) SubmitInput(params map[string]any) (map[string]any, error) {
	snapshot := s.context.Capture(params)
	options := mapValue(params, "options")
	confirmRequired := boolValue(options, "confirm_required", true)
	preferredDelivery, fallbackDelivery := deliveryPreferenceFromSubmit(params)
	suggestion := s.intent.Suggest(snapshot, nil, confirmRequired)
	if s.intent.AnalyzeSnapshot(snapshot) == "waiting_input" {
		task := s.runEngine.CreateTask(runengine.CreateTaskInput{
			SessionID:         stringValue(params, "session_id", ""),
			Title:             "等待补充输入",
			SourceType:        suggestion.TaskSourceType,
			Status:            "waiting_input",
			Intent:            nil,
			PreferredDelivery: preferredDelivery,
			FallbackDelivery:  fallbackDelivery,
			CurrentStep:       "collect_input",
			RiskLevel:         s.risk.DefaultLevel(),
			Timeline:          initialTimeline("waiting_input", "collect_input"),
		})

		bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "请先告诉我你希望我处理什么内容。", task.StartedAt.Format(dateTimeLayout))
		if _, ok := s.runEngine.SetPresentation(task.TaskID, bubble, nil, nil); ok {
			task, _ = s.runEngine.GetTask(task.TaskID)
		}

		return map[string]any{
			"task":           taskMap(task),
			"bubble_message": bubble,
		}, nil
	}

	task := s.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:         stringValue(params, "session_id", ""),
		Title:             suggestion.TaskTitle,
		SourceType:        suggestion.TaskSourceType,
		Status:            taskStatusForSuggestion(suggestion.RequiresConfirm),
		Intent:            suggestion.Intent,
		PreferredDelivery: preferredDelivery,
		FallbackDelivery:  fallbackDelivery,
		CurrentStep:       currentStepForSuggestion(suggestion.RequiresConfirm),
		RiskLevel:         s.risk.DefaultLevel(),
		Timeline:          initialTimeline(taskStatusForSuggestion(suggestion.RequiresConfirm), currentStepForSuggestion(suggestion.RequiresConfirm)),
	})
	s.attachMemoryReadPlans(task.TaskID, task.RunID, snapshot, suggestion.Intent)

	bubble := s.delivery.BuildBubbleMessage(task.TaskID, bubbleTypeForSuggestion(suggestion.RequiresConfirm), bubbleTextForInput(suggestion), task.StartedAt.Format(dateTimeLayout))
	deliveryResult := map[string]any(nil)
	if !suggestion.RequiresConfirm {
		if requiresAuthorization(suggestion.Intent) {
			pendingExecution := s.buildPendingExecution(task, suggestion.Intent)
			approvalRequest := buildApprovalRequest(task.TaskID, suggestion.Intent, "red")
			bubble = s.delivery.BuildBubbleMessage(task.TaskID, "status", "检测到待授权操作，请先确认。", task.StartedAt.Format(dateTimeLayout))
			if _, ok := s.runEngine.MarkWaitingApprovalWithPlan(task.TaskID, approvalRequest, pendingExecution, bubble); ok {
				task, _ = s.runEngine.GetTask(task.TaskID)
			}
			return map[string]any{
				"task":           taskMap(task),
				"bubble_message": bubble,
			}, nil
		}
		var err error
		task, bubble, deliveryResult, _, err = s.executeTask(task, snapshot, suggestion.Intent)
		if err != nil {
			return nil, err
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

// StartTask 启动Task。

// StartTask 处理 agent.task.start。
// 它会基于显式 intent 或默认建议创建 task/run 映射，并决定是否需要确认或授权。
func (s *Service) StartTask(params map[string]any) (map[string]any, error) {
	snapshot := s.context.Capture(params)
	explicitIntent := mapValue(params, "intent")
	preferredDelivery, fallbackDelivery := deliveryPreferenceFromStart(params)
	suggestion := s.intent.Suggest(snapshot, explicitIntent, len(explicitIntent) == 0)

	task := s.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:         stringValue(params, "session_id", ""),
		Title:             suggestion.TaskTitle,
		SourceType:        suggestion.TaskSourceType,
		Status:            taskStatusForSuggestion(suggestion.RequiresConfirm),
		Intent:            suggestion.Intent,
		PreferredDelivery: preferredDelivery,
		FallbackDelivery:  fallbackDelivery,
		CurrentStep:       currentStepForSuggestion(suggestion.RequiresConfirm),
		RiskLevel:         s.risk.DefaultLevel(),
		Timeline:          initialTimeline(taskStatusForSuggestion(suggestion.RequiresConfirm), currentStepForSuggestion(suggestion.RequiresConfirm)),
	})
	s.attachMemoryReadPlans(task.TaskID, task.RunID, snapshot, suggestion.Intent)

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

	if requiresAuthorization(suggestion.Intent) {
		pendingExecution := s.buildPendingExecution(task, suggestion.Intent)
		approvalRequest := buildApprovalRequest(task.TaskID, suggestion.Intent, "red")
		bubble = s.delivery.BuildBubbleMessage(task.TaskID, "status", "检测到待授权操作，请先确认。", task.StartedAt.Format(dateTimeLayout))
		if _, ok := s.runEngine.MarkWaitingApprovalWithPlan(task.TaskID, approvalRequest, pendingExecution, bubble); ok {
			task, _ = s.runEngine.GetTask(task.TaskID)
			response["task"] = taskMap(task)
			response["bubble_message"] = bubble
		}
		return response, nil
	}

	var err error
	task, bubble, deliveryResult, _, err := s.executeTask(task, snapshot, suggestion.Intent)
	if err != nil {
		return nil, err
	}
	response["task"] = taskMap(task)
	response["bubble_message"] = bubble
	response["delivery_result"] = deliveryResult
	return response, nil
}

// ConfirmTask 确认Task。

// ConfirmTask 处理 agent.task.confirm。
// 这条路径会把确认后的意图写回运行态，并继续推进执行、授权挂起或正式交付。
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
	if requiresAuthorization(intentValue) {
		updatedTask, ok := s.runEngine.UpdateIntent(task.TaskID, intentValue)
		if !ok {
			return nil, ErrTaskNotFound
		}
		s.attachMemoryReadPlans(updatedTask.TaskID, updatedTask.RunID, snapshotFromTask(updatedTask), intentValue)

		pendingExecution := s.buildPendingExecution(updatedTask, intentValue)
		approvalRequest := buildApprovalRequest(task.TaskID, intentValue, "red")
		bubble = s.delivery.BuildBubbleMessage(task.TaskID, "status", "检测到待授权操作，请先确认。", updatedTask.UpdatedAt.Format(dateTimeLayout))
		updatedTask, ok = s.runEngine.MarkWaitingApprovalWithPlan(task.TaskID, approvalRequest, pendingExecution, bubble)
		if !ok {
			return nil, ErrTaskNotFound
		}
		return map[string]any{
			"task":            taskMap(updatedTask),
			"bubble_message":  bubble,
			"delivery_result": nil,
		}, nil
	}

	updatedTask, ok := s.runEngine.ConfirmTask(task.TaskID, intentValue, bubble)
	if !ok {
		return nil, ErrTaskNotFound
	}
	snapshot := snapshotFromTask(updatedTask)
	s.attachMemoryReadPlans(updatedTask.TaskID, updatedTask.RunID, snapshot, intentValue)

	updatedTask, resultBubble, deliveryResult, _, err := s.executeTask(updatedTask, snapshot, intentValue)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"task":            taskMap(updatedTask),
		"bubble_message":  resultBubble,
		"delivery_result": deliveryResult,
	}, nil
}

// RecommendationGet 处理当前模块的相关逻辑。

// RecommendationGet 处理 agent.recommendation.get，返回轻量推荐动作。
func (s *Service) RecommendationGet(params map[string]any) (map[string]any, error) {
	contextValue := mapValue(params, "context")
	unfinishedTasks, _ := s.runEngine.ListTasks("unfinished", "updated_at", "desc", 20, 0)
	finishedTasks, _ := s.runEngine.ListTasks("finished", "finished_at", "desc", 20, 0)
	notepadItems, _ := s.runEngine.NotepadItems("", 20, 0)
	result := s.recommendation.Get(recommendation.GenerateInput{
		Source:          stringValue(params, "source", "floating_ball"),
		Scene:           stringValue(params, "scene", "hover"),
		PageTitle:       stringValue(contextValue, "page_title", ""),
		AppName:         stringValue(contextValue, "app_name", ""),
		SelectionText:   stringValue(contextValue, "selection_text", ""),
		UnfinishedTasks: unfinishedTasks,
		FinishedTasks:   finishedTasks,
		NotepadItems:    notepadItems,
	})
	return map[string]any{
		"cooldown_hit": result.CooldownHit,
		"items":        result.Items,
	}, nil
}

// RecommendationFeedbackSubmit 处理当前模块的相关逻辑。

// RecommendationFeedbackSubmit 处理 agent.recommendation.feedback.submit。
func (s *Service) RecommendationFeedbackSubmit(params map[string]any) (map[string]any, error) {
	return map[string]any{
		"applied": s.recommendation.SubmitFeedback(
			stringValue(params, "recommendation_id", ""),
			stringValue(params, "feedback", ""),
		),
	}, nil
}

// TaskList 处理当前模块的相关逻辑。

// TaskList 处理 agent.task.list，返回符合排序规则的任务列表。
func (s *Service) TaskList(params map[string]any) (map[string]any, error) {
	group := stringValue(params, "group", "unfinished")
	limit := intValue(params, "limit", 20)
	offset := intValue(params, "offset", 0)
	sortBy := stringValue(params, "sort_by", "updated_at")
	sortOrder := stringValue(params, "sort_order", "desc")
	tasks, total := s.runEngine.ListTasks(group, sortBy, sortOrder, limit, offset)

	items := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, taskMap(task))
	}

	return map[string]any{
		"items": items,
		"page":  pageMap(limit, offset, total),
	}, nil
}

// TaskDetailGet 处理当前模块的相关逻辑。

// TaskDetailGet 处理 agent.task.detail.get，返回任务详情视图需要的完整数据。
func (s *Service) TaskDetailGet(params map[string]any) (map[string]any, error) {
	taskID := stringValue(params, "task_id", "")
	task, ok := s.runEngine.TaskDetail(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}

	securitySummary := cloneMap(task.SecuritySummary)
	if securitySummary == nil {
		securitySummary = map[string]any{}
	}
	if latestRestorePointFromSummary(securitySummary) == nil {
		if restorePoint := s.latestRestorePointFromStorage(task.TaskID); restorePoint != nil {
			securitySummary["latest_restore_point"] = restorePoint
		}
	}

	return map[string]any{
		"task":              taskMap(task),
		"timeline":          timelineMap(task.Timeline),
		"artifacts":         cloneMapSlice(task.Artifacts),
		"mirror_references": cloneMapSlice(task.MirrorReferences),
		"security_summary":  securitySummary,
	}, nil
}

// TaskControl 处理当前模块的相关逻辑。

// TaskControl 处理 agent.task.control，把用户控制动作转换成状态机操作。
func (s *Service) TaskControl(params map[string]any) (map[string]any, error) {
	taskID := stringValue(params, "task_id", "")
	action := stringValue(params, "action", "pause")
	if !isSupportedTaskControlAction(action) {
		return nil, fmt.Errorf("unsupported task control action: %s", action)
	}
	bubble := s.delivery.BuildBubbleMessage(taskID, "status", controlBubbleText(action), currentTimeFromTask(s.runEngine, taskID))
	updatedTask, err := s.runEngine.ControlTask(taskID, action, bubble)
	if err != nil {
		switch {
		case errors.Is(err, runengine.ErrTaskNotFound):
			return nil, ErrTaskNotFound
		case errors.Is(err, runengine.ErrTaskStatusInvalid):
			return nil, ErrTaskStatusInvalid
		case errors.Is(err, runengine.ErrTaskAlreadyFinished):
			return nil, ErrTaskAlreadyFinished
		default:
			return nil, err
		}
	}

	return map[string]any{
		"task":           taskMap(updatedTask),
		"bubble_message": bubble,
	}, nil
}

// TaskInspectorConfigGet 处理当前模块的相关逻辑。

// TaskInspectorConfigGet 处理 agent.task_inspector.config.get。
func (s *Service) TaskInspectorConfigGet() (map[string]any, error) {
	return s.runEngine.InspectorConfig(), nil
}

// TaskInspectorConfigUpdate 处理当前模块的相关逻辑。

// TaskInspectorConfigUpdate 处理 agent.task_inspector.config.update。
func (s *Service) TaskInspectorConfigUpdate(params map[string]any) (map[string]any, error) {
	effective := s.runEngine.UpdateInspectorConfig(params)
	return map[string]any{
		"updated":          true,
		"effective_config": effective,
	}, nil
}

// TaskInspectorRun 处理当前模块的相关逻辑。

// TaskInspectorRun 处理 agent.task_inspector.run，返回巡检摘要和建议。
func (s *Service) TaskInspectorRun(params map[string]any) (map[string]any, error) {
	config := s.runEngine.InspectorConfig()
	targetSources := stringSliceValue(params["target_sources"])
	notepadItems, _ := s.runEngine.NotepadItems("", 0, 0)
	unfinishedTasks, _ := s.runEngine.ListTasks("unfinished", "updated_at", "desc", 0, 0)
	finishedTasks, _ := s.runEngine.ListTasks("finished", "finished_at", "desc", 0, 0)

	result := s.inspector.Run(taskinspector.RunInput{
		Reason:          stringValue(params, "reason", ""),
		TargetSources:   targetSources,
		Config:          config,
		UnfinishedTasks: unfinishedTasks,
		FinishedTasks:   finishedTasks,
		NotepadItems:    notepadItems,
	})

	return map[string]any{
		"inspection_id": result.InspectionID,
		"summary":       result.Summary,
		"suggestions":   append([]string(nil), result.Suggestions...),
	}, nil
}

// NotepadList 处理当前模块的相关逻辑。

// NotepadList 处理 agent.notepad.list。
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

// NotepadConvertToTask 处理当前模块的相关逻辑。

// NotepadConvertToTask 处理 agent.notepad.convert_to_task。
func (s *Service) NotepadConvertToTask(params map[string]any) (map[string]any, error) {
	itemID := stringValue(params, "item_id", "")
	if itemID == "" {
		return nil, fmt.Errorf("item_id is required")
	}

	item, ok := s.runEngine.NotepadItem(itemID)
	if !ok {
		return nil, fmt.Errorf("notepad item not found: %s", itemID)
	}

	if status := stringValue(item, "status", "normal"); status == "completed" || status == "cancelled" {
		return nil, fmt.Errorf("notepad item is already closed: %s", itemID)
	}

	itemTitle := stringValue(item, "title", "待办事项")
	taskIntent := notepadIntent(item)
	task := s.runEngine.CreateTask(runengine.CreateTaskInput{
		Title:       itemTitle,
		SourceType:  "todo",
		Status:      "confirming_intent",
		Intent:      taskIntent,
		CurrentStep: "intent_confirmation",
		RiskLevel:   s.risk.DefaultLevel(),
		Timeline:    initialTimeline("confirming_intent", "intent_confirmation"),
	})
	s.attachMemoryReadPlans(task.TaskID, task.RunID, notepadSnapshot(item), taskIntent)

	return map[string]any{
		"task": taskMap(task),
	}, nil
}

// DashboardOverviewGet 处理当前模块的相关逻辑。

// DashboardOverviewGet 处理 agent.dashboard.overview.get。
func (s *Service) DashboardOverviewGet(params map[string]any) (map[string]any, error) {
	_ = params
	unfinishedTasks, _ := s.runEngine.ListTasks("unfinished", "updated_at", "desc", 0, 0)
	finishedTasks, _ := s.runEngine.ListTasks("finished", "finished_at", "desc", 0, 0)
	pendingApprovals, pendingTotal := s.runEngine.PendingApprovalRequests(20, 0)

	focusTask, hasFocusTask := focusTaskForOverview(unfinishedTasks, finishedTasks)
	var focusSummary map[string]any
	if hasFocusTask {
		focusSummary = map[string]any{
			"task_id":      focusTask.TaskID,
			"title":        focusTask.Title,
			"status":       focusTask.Status,
			"current_step": focusTask.CurrentStep,
			"next_action":  nextActionForTask(focusTask),
			"updated_at":   focusTask.UpdatedAt.Format(dateTimeLayout),
		}
	}

	allTasks := append(append([]runengine.TaskRecord{}, unfinishedTasks...), finishedTasks...)
	hasRestorePoint := latestRestorePointFromTasks(allTasks) != nil
	if !hasRestorePoint {
		hasRestorePoint = s.latestRestorePointFromStorage("") != nil
	}
	latestAudit := latestAuditRecordFromTasks(allTasks)
	if latestAudit == nil {
		latestAudit = s.latestAuditRecordFromStorage("")
	}

	return map[string]any{
		"overview": map[string]any{
			"focus_summary": focusSummary,
			"trust_summary": map[string]any{
				"risk_level":             aggregateRiskLevel(allTasks, pendingApprovals, s.risk.DefaultLevel()),
				"pending_authorizations": pendingTotal,
				"has_restore_point":      hasRestorePoint,
				"workspace_path":         workspacePathFromSettings(s.runEngine.Settings()),
			},
			"quick_actions":     buildDashboardQuickActions(hasFocusTask, pendingTotal, len(finishedTasks)),
			"global_state":      s.Snapshot(),
			"high_value_signal": buildDashboardSignalsWithAudit(unfinishedTasks, finishedTasks, pendingApprovals, latestAudit),
		},
	}, nil
}

// DashboardModuleGet 处理当前模块的相关逻辑。

// DashboardModuleGet 处理 agent.dashboard.module.get。
func (s *Service) DashboardModuleGet(params map[string]any) (map[string]any, error) {
	module := stringValue(params, "module", "mirror")
	tab := stringValue(params, "tab", "daily_summary")
	finishedTasks, _ := s.runEngine.ListTasks("finished", "finished_at", "desc", 0, 0)
	unfinishedTasks, _ := s.runEngine.ListTasks("unfinished", "updated_at", "desc", 0, 0)
	_, pendingTotal := s.runEngine.PendingApprovalRequests(20, 0)
	latestAudit := latestAuditRecordFromTasks(append(append([]runengine.TaskRecord{}, unfinishedTasks...), finishedTasks...))
	if latestAudit == nil {
		latestAudit = s.latestAuditRecordFromStorage("")
	}
	return map[string]any{
		"module": module,
		"tab":    tab,
		"summary": map[string]any{
			"completed_tasks":     len(finishedTasks),
			"generated_outputs":   countGeneratedOutputs(finishedTasks),
			"authorizations_used": countAuthorizedTasks(unfinishedTasks, finishedTasks),
			"exceptions":          countExceptionTasks(unfinishedTasks, finishedTasks),
		},
		"highlights": buildDashboardModuleHighlightsWithAudit(unfinishedTasks, finishedTasks, pendingTotal, latestAudit),
	}, nil
}

// MirrorOverviewGet 处理当前模块的相关逻辑。

// MirrorOverviewGet 处理 agent.mirror.overview.get。
func (s *Service) MirrorOverviewGet(params map[string]any) (map[string]any, error) {
	_ = params
	finishedTasks, _ := s.runEngine.ListTasks("finished", "finished_at", "desc", 0, 0)
	memoryReferences := collectMirrorReferences(finishedTasks)
	return map[string]any{
		"history_summary": buildMirrorHistorySummary(finishedTasks, memoryReferences),
		"daily_summary": map[string]any{
			"date":              time.Now().Format("2006-01-02"),
			"completed_tasks":   len(finishedTasks),
			"generated_outputs": countGeneratedOutputs(finishedTasks),
		},
		"profile":           buildMirrorProfile(finishedTasks),
		"memory_references": memoryReferences,
	}, nil
}

// SecuritySummaryGet 处理当前模块的相关逻辑。

// SecuritySummaryGet 处理 agent.security.summary.get。
func (s *Service) SecuritySummaryGet() (map[string]any, error) {
	_, pendingTotal := s.runEngine.PendingApprovalRequests(20, 0)
	unfinishedTasks, _ := s.runEngine.ListTasks("unfinished", "updated_at", "desc", 0, 0)
	finishedTasks, _ := s.runEngine.ListTasks("finished", "finished_at", "desc", 0, 0)
	allTasks := append(append([]runengine.TaskRecord{}, unfinishedTasks...), finishedTasks...)
	dataLogSettings := mapValue(s.runEngine.Settings(), "data_log")
	latestRestorePoint := latestRestorePointFromTasks(allTasks)
	if latestRestorePoint == nil {
		latestRestorePoint = s.latestRestorePointFromStorage("")
	}
	return map[string]any{
		"summary": map[string]any{
			"security_status":        aggregateSecurityStatus(allTasks, pendingTotal),
			"pending_authorizations": pendingTotal,
			"latest_restore_point":   latestRestorePoint,
			"token_cost_summary":     aggregateTokenCostSummary(unfinishedTasks, finishedTasks, boolValue(dataLogSettings, "budget_auto_downgrade", true)),
		},
	}, nil
}

// SecurityPendingList 处理当前模块的相关逻辑。

// SecurityPendingList 处理 agent.security.pending.list。
func (s *Service) SecurityPendingList(params map[string]any) (map[string]any, error) {
	limit := intValue(params, "limit", 20)
	offset := intValue(params, "offset", 0)
	items, total := s.runEngine.PendingApprovalRequests(limit, offset)
	return map[string]any{
		"items": items,
		"page":  pageMap(limit, offset, total),
	}, nil
}

// SecurityAuditList 处理 agent.security.audit.list。
func (s *Service) SecurityAuditList(params map[string]any) (map[string]any, error) {
	limit := clampListLimit(intValue(params, "limit", 20))
	offset := clampListOffset(intValue(params, "offset", 0))
	taskID := stringValue(params, "task_id", "")
	if strings.TrimSpace(taskID) == "" {
		return nil, errors.New("task_id is required")
	}
	if s.storage == nil {
		return map[string]any{"items": []map[string]any{}, "page": pageMap(limit, offset, 0)}, nil
	}
	records, total, err := s.storage.AuditStore().ListAuditRecords(context.Background(), taskID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
	}
	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		items = append(items, record.Map())
	}
	return map[string]any{
		"items": items,
		"page":  pageMap(limit, offset, total),
	}, nil
}

// SecurityRestorePointsList 处理 agent.security.restore_points.list。
func (s *Service) SecurityRestorePointsList(params map[string]any) (map[string]any, error) {
	limit := clampListLimit(intValue(params, "limit", 20))
	offset := clampListOffset(intValue(params, "offset", 0))
	taskID := stringValue(params, "task_id", "")
	if s.storage == nil {
		return map[string]any{"items": []map[string]any{}, "page": pageMap(limit, offset, 0)}, nil
	}
	points, total, err := s.storage.RecoveryPointStore().ListRecoveryPoints(context.Background(), taskID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
	}
	items := make([]map[string]any, 0, len(points))
	for _, point := range points {
		items = append(items, map[string]any{
			"recovery_point_id": point.RecoveryPointID,
			"task_id":           point.TaskID,
			"summary":           point.Summary,
			"created_at":        point.CreatedAt,
			"objects":           append([]string(nil), point.Objects...),
		})
	}
	return map[string]any{
		"items": items,
		"page":  pageMap(limit, offset, total),
	}, nil
}

// SecurityRestoreApply 处理 agent.security.restore.apply。
func (s *Service) SecurityRestoreApply(params map[string]any) (map[string]any, error) {
	recoveryPointID := stringValue(params, "recovery_point_id", "")
	if strings.TrimSpace(recoveryPointID) == "" {
		return nil, errors.New("recovery_point_id is required")
	}
	taskID := stringValue(params, "task_id", "")
	point, err := s.findRecoveryPointFromStorage(taskID, recoveryPointID)
	if err != nil {
		return nil, err
	}
	resolvedTaskID := firstNonEmptyString(strings.TrimSpace(taskID), point.TaskID)
	task, ok := s.runEngine.GetTask(resolvedTaskID)
	if !ok {
		if s.storage == nil {
			return nil, ErrTaskNotFound
		}
		persisted, loadErr := s.storage.TaskRunStore().LoadTaskRuns(context.Background())
		if loadErr != nil {
			return nil, fmt.Errorf("%w: %v", ErrStorageQueryFailed, loadErr)
		}
		loadedTask, found := findTaskRecordFromStorage(persisted, resolvedTaskID)
		if !found {
			return nil, ErrTaskNotFound
		}
		task = s.runEngine.HydrateTaskFromStorage(loadedTask)
	}

	recoveryPoint := recoveryPointMap(point)
	applied := false
	securityStatus := "recovered"
	bubbleText := fmt.Sprintf("已根据恢复点 %s 恢复 %d 个对象。", point.RecoveryPointID, len(point.Objects))
	if s.executor == nil {
		securityStatus = "execution_error"
		bubbleText = "恢复失败：执行后端不可用。"
	} else if applyResult, err := s.executor.ApplyRecoveryPoint(context.Background(), point); err != nil {
		securityStatus = "execution_error"
		bubbleText = fmt.Sprintf("恢复失败：%s", err.Error())
	} else {
		applied = true
		if len(applyResult.RestoredObjects) > 0 {
			bubbleText = fmt.Sprintf("已根据恢复点 %s 恢复 %d 个对象。", point.RecoveryPointID, len(applyResult.RestoredObjects))
		}
	}

	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", bubbleText, time.Now().Format(dateTimeLayout))
	updatedTask, ok := s.runEngine.ApplyRecoveryOutcome(task.TaskID, securityStatus, recoveryPoint, bubble)
	if !ok {
		return nil, ErrTaskNotFound
	}
	auditRecord := s.writeRestoreAuditRecord(updatedTask.TaskID, point, applied, bubbleText)
	updatedTask = s.appendAuditData(updatedTask, compactAuditRecords(auditRecord), nil)

	return map[string]any{
		"applied":        applied,
		"task":           taskMap(updatedTask),
		"recovery_point": recoveryPoint,
		"audit_record":   auditRecord,
		"bubble_message": bubble,
	}, nil
}

func clampListLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func clampListOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	return offset
}

// PendingNotifications 返回待处理的Notifications。

// PendingNotifications 读取某个任务当前尚未消费的通知列表。
func (s *Service) PendingNotifications(taskID string) ([]map[string]any, error) {
	notifications, ok := s.runEngine.PendingNotifications(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}

	items := make([]map[string]any, 0, len(notifications))
	for _, notification := range notifications {
		items = append(items, map[string]any{
			"method":     notification.Method,
			"params":     cloneMap(notification.Params),
			"created_at": notification.CreatedAt.Format(dateTimeLayout),
		})
	}

	return items, nil
}

// DrainNotifications 取出并清空Notifications。

// DrainNotifications 取出并清空某个任务的通知列表。
func (s *Service) DrainNotifications(taskID string) ([]map[string]any, error) {
	notifications, ok := s.runEngine.DrainNotifications(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}

	items := make([]map[string]any, 0, len(notifications))
	for _, notification := range notifications {
		items = append(items, map[string]any{
			"method":     notification.Method,
			"params":     cloneMap(notification.Params),
			"created_at": notification.CreatedAt.Format(dateTimeLayout),
		})
	}

	return items, nil
}

// SecurityRespond 处理当前模块的相关逻辑。

// SecurityRespond 处理 agent.security.respond。
// 它是风险挂起链路的恢复入口，负责把“允许/拒绝”转换成任务状态推进、交付恢复和审计结果。
func (s *Service) SecurityRespond(params map[string]any) (map[string]any, error) {
	taskID := stringValue(params, "task_id", "")
	task, ok := s.runEngine.GetTask(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}

	decision := stringValue(params, "decision", "allow_once")
	rememberRule := boolValue(params, "remember_rule", false)
	authorizationRecord := map[string]any{
		"authorization_record_id": fmt.Sprintf("auth_%s", task.TaskID),
		"task_id":                 task.TaskID,
		"approval_id":             stringValue(params, "approval_id", "appr_001"),
		"decision":                decision,
		"remember_rule":           rememberRule,
		"operator":                "user",
		"created_at":              time.Now().Format(dateTimeLayout),
	}
	pendingExecution, ok := s.runEngine.PendingExecutionPlan(task.TaskID)
	if !ok {
		pendingExecution = s.buildPendingExecution(task, task.Intent)
	}
	pendingExecution = s.applyResolvedDeliveryToPlan(task, pendingExecution, task.Intent)
	impactScope := s.buildImpactScope(task, pendingExecution)
	if decision == "deny_once" {
		bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "已拒绝本次操作，任务已取消。", task.UpdatedAt.Format(dateTimeLayout))
		updatedTask, ok := s.runEngine.DenyAfterApproval(task.TaskID, authorizationRecord, impactScope, bubble)
		if !ok {
			return nil, ErrTaskNotFound
		}
		updatedTask = s.appendAuditData(updatedTask, compactAuditRecords(s.audit.BuildAuthorizationAudit(updatedTask.TaskID, updatedTask.RunID, decision, impactScope)), nil)
		return map[string]any{
			"authorization_record": authorizationRecord,
			"task":                 taskMap(updatedTask),
			"bubble_message":       bubble,
			"impact_scope":         impactScope,
		}, nil
	}

	resumeBubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "已允许本次操作，任务继续执行。", task.UpdatedAt.Format(dateTimeLayout))
	processingTask, ok := s.runEngine.ResumeAfterApproval(task.TaskID, authorizationRecord, impactScope, resumeBubble)
	if !ok {
		return nil, ErrTaskNotFound
	}
	processingTask = s.appendAuditData(processingTask, compactAuditRecords(s.audit.BuildAuthorizationAudit(processingTask.TaskID, processingTask.RunID, decision, impactScope)), nil)

	resultTitle := stringValue(pendingExecution, "result_title", "处理结果")
	resultPreview := stringValue(pendingExecution, "preview_text", "已为你写入文档并打开")
	resultBubbleText := stringValue(pendingExecution, "result_bubble_text", "结果已经生成，可直接查看。")
	deliveryType := stringValue(pendingExecution, "delivery_type", deliveryTypeFromIntent(task.Intent))
	deliveryType = resolveTaskDeliveryType(task, task.Intent)
	resultPreview = previewTextForDeliveryType(deliveryType)
	_, _, _, _ = resultTitle, resultPreview, resultBubbleText, deliveryType
	updatedTask, resultBubble, deliveryResult, _, err := s.executeTask(processingTask, snapshotFromTask(processingTask), processingTask.Intent)
	if err != nil {
		return nil, err
	}
	updatedTask, _ = s.runEngine.ResolveAuthorization(task.TaskID, authorizationRecord, impactScope)

	return map[string]any{
		"authorization_record": authorizationRecord,
		"task":                 taskMap(updatedTask),
		"bubble_message":       resultBubble,
		"delivery_result":      deliveryResult,
		"impact_scope":         impactScope,
	}, nil
}

// SettingsGet 设置tingsGet。

// SettingsGet 处理 agent.settings.get。
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

// SettingsUpdate 设置tingsUpdate。

// SettingsUpdate 处理 agent.settings.update，并返回生效设置和应用模式。
func (s *Service) SettingsUpdate(params map[string]any) (map[string]any, error) {
	effectiveSettings, updatedKeys, applyMode, needRestart := s.runEngine.UpdateSettings(params)
	return map[string]any{
		"updated_keys":       updatedKeys,
		"effective_settings": effectiveSettings,
		"apply_mode":         applyMode,
		"need_restart":       needRestart,
	}, nil
}

// taskMap 处理当前模块的相关逻辑。

// taskMap 把 runengine 内部任务记录映射成对外统一的 task 结构。
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

// timelineMap 处理当前模块的相关逻辑。

// timelineMap 把内部时间线记录映射成对外返回值。
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

// pageMap 处理当前模块的相关逻辑。

// pageMap 统一列表接口返回的分页信息。
func pageMap(limit, offset, total int) map[string]any {
	return map[string]any{
		"limit":    limit,
		"offset":   offset,
		"total":    total,
		"has_more": offset+limit < total,
	}
}

// taskStatusForSuggestion 处理当前模块的相关逻辑。
func taskStatusForSuggestion(requiresConfirm bool) string {
	if requiresConfirm {
		return "confirming_intent"
	}
	return "processing"
}

// currentStepForSuggestion 处理当前模块的相关逻辑。
func currentStepForSuggestion(requiresConfirm bool) string {
	if requiresConfirm {
		return "intent_confirmation"
	}
	return "generate_output"
}

// bubbleTypeForSuggestion 处理当前模块的相关逻辑。
func bubbleTypeForSuggestion(requiresConfirm bool) string {
	if requiresConfirm {
		return "intent_confirm"
	}
	return "result"
}

// bubbleTextForInput 处理当前模块的相关逻辑。
func bubbleTextForInput(suggestion intent.Suggestion) string {
	if suggestion.RequiresConfirm {
		return "你是想总结这段内容吗？"
	}
	return suggestion.ResultBubbleText
}

// bubbleTextForStart 处理当前模块的相关逻辑。
func bubbleTextForStart(suggestion intent.Suggestion) string {
	if suggestion.RequiresConfirm {
		return "你是想让我按当前对象继续处理吗？"
	}
	return suggestion.ResultBubbleText
}

// initialTimeline 处理当前模块的相关逻辑。

// initialTimeline 生成任务创建时的第一条时间线步骤。
// 它会根据当前 task_status 决定步骤初始状态是 pending 还是 running。
func initialTimeline(status, currentStep string) []runengine.TaskStepRecord {
	stepStatus := "running"
	if status == "confirming_intent" || status == "waiting_input" {
		stepStatus = "pending"
	}

	outputSummary := "等待继续处理"
	if status == "waiting_input" {
		outputSummary = "等待用户补充输入"
	}

	return []runengine.TaskStepRecord{
		{
			StepID:        fmt.Sprintf("step_%s", currentStep),
			Name:          currentStep,
			Status:        stepStatus,
			OrderIndex:    1,
			InputSummary:  "已识别到当前任务对象",
			OutputSummary: outputSummary,
		},
	}
}

// controlBubbleText 处理当前模块的相关逻辑。

// controlBubbleText 根据 task_control_action 生成对应的状态气泡文案。
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

func isSupportedTaskControlAction(action string) bool {
	switch action {
	case "pause", "resume", "cancel", "restart":
		return true
	default:
		return false
	}
}

// currentTimeFromTask 处理当前模块的相关逻辑。
func currentTimeFromTask(engine *runengine.Engine, taskID string) string {
	task, ok := engine.GetTask(taskID)
	if !ok {
		return ""
	}
	return task.UpdatedAt.Format(dateTimeLayout)
}

// workspacePathFromSettings 处理当前模块的相关逻辑。

// workspacePathFromSettings 从设置快照里提取当前 workspace 路径。
func workspacePathFromSettings(settings map[string]any) string {
	general, ok := settings["general"].(map[string]any)
	if !ok {
		return "workspace"
	}
	download, ok := general["download"].(map[string]any)
	if !ok {
		return "workspace"
	}
	return stringValue(download, "workspace_path", "workspace")
}

// defaultIntentMap 处理当前模块的相关逻辑。
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

func notepadIntent(item map[string]any) map[string]any {
	title := strings.ToLower(stringValue(item, "title", ""))
	suggestion := strings.ToLower(stringValue(item, "agent_suggestion", ""))
	combined := title + " " + suggestion

	switch {
	case strings.Contains(combined, "翻译") || strings.Contains(combined, "translate"):
		return defaultIntentMap("translate")
	case strings.Contains(combined, "改写") || strings.Contains(combined, "rewrite"):
		return defaultIntentMap("rewrite")
	case strings.Contains(combined, "解释") || strings.Contains(combined, "explain"):
		return defaultIntentMap("explain")
	default:
		return defaultIntentMap("summarize")
	}
}

func notepadSnapshot(item map[string]any) contextsvc.TaskContextSnapshot {
	return contextsvc.TaskContextSnapshot{
		Source:    "dashboard",
		InputType: "text",
		Text:      stringValue(item, "title", ""),
		PageTitle: "notepad",
		AppName:   "dashboard",
	}
}

// defaultMirrorReference 处理当前模块的相关逻辑。

// defaultMirrorReference 构造镜像模块返回的示例记忆引用。
func defaultMirrorReference() map[string]any {
	return map[string]any{
		"memory_id": "pref_001",
		"reason":    "当前任务命中了用户的输出偏好",
		"summary":   "偏好简洁三点式摘要",
	}
}

func focusTaskForOverview(unfinishedTasks, finishedTasks []runengine.TaskRecord) (runengine.TaskRecord, bool) {
	if len(unfinishedTasks) > 0 {
		return unfinishedTasks[0], true
	}
	if len(finishedTasks) > 0 {
		return finishedTasks[0], true
	}
	return runengine.TaskRecord{}, false
}

func nextActionForTask(task runengine.TaskRecord) string {
	switch task.Status {
	case "confirming_intent":
		return "确认当前意图"
	case "waiting_auth":
		return "处理待授权操作"
	case "waiting_input":
		return "补充输入内容"
	case "processing":
		return "等待处理完成"
	case "completed":
		return "查看交付结果"
	default:
		return "打开任务详情"
	}
}

func buildDashboardQuickActions(hasFocusTask bool, pendingTotal, finishedCount int) []string {
	actions := make([]string, 0, 3)
	if pendingTotal > 0 {
		actions = append(actions, "处理待授权操作")
	}
	if hasFocusTask {
		actions = append(actions, "打开任务详情")
	}
	if finishedCount > 0 {
		actions = append(actions, "查看最近结果")
	}
	if len(actions) == 0 {
		actions = append(actions, "等待新任务")
	}
	return actions
}

func buildDashboardSignals(unfinishedTasks, finishedTasks []runengine.TaskRecord, pendingApprovals []map[string]any) []string {
	signals := make([]string, 0, 3)
	if len(unfinishedTasks) > 0 {
		signals = append(signals, fmt.Sprintf("当前有 %d 个未完成任务处于 runtime 管控中。", len(unfinishedTasks)))
	}
	if len(pendingApprovals) > 0 {
		signals = append(signals, fmt.Sprintf("当前有 %d 个待授权操作等待用户确认。", len(pendingApprovals)))
	}
	if latestRestorePointFromTasks(finishedTasks) != nil {
		signals = append(signals, "最近一次正式交付已经生成可回放的恢复点。")
	}
	if len(signals) == 0 {
		signals = append(signals, "主链路当前暂无活跃任务。")
	}
	return signals
}

func buildDashboardModuleHighlights(unfinishedTasks, finishedTasks []runengine.TaskRecord, pendingTotal int) []string {
	highlights := make([]string, 0, 4)
	if latestOutputPath := latestOutputPathFromTasks(finishedTasks); latestOutputPath != "" {
		highlights = append(highlights, fmt.Sprintf("最近正式交付已落到 %s。", latestOutputPath))
	}
	if pendingTotal > 0 {
		highlights = append(highlights, fmt.Sprintf("当前仍有 %d 个待授权任务等待处理。", pendingTotal))
	}
	if restorePoint := latestRestorePointFromTasks(finishedTasks); restorePoint != nil {
		highlights = append(highlights, fmt.Sprintf("最近恢复点 %s 已可用于安全回显。", stringValue(restorePoint, "recovery_point_id", "latest")))
	}
	if len(unfinishedTasks) > 0 {
		highlights = append(highlights, fmt.Sprintf("最近活跃任务状态为 %s。", unfinishedTasks[0].Status))
	}
	if len(highlights) == 0 {
		highlights = append(highlights, "当前模块视图已切换为 runtime 聚合结果。")
	}
	return highlights
}

func countGeneratedOutputs(tasks []runengine.TaskRecord) int {
	total := 0
	for _, task := range tasks {
		if len(task.DeliveryResult) > 0 || len(task.Artifacts) > 0 {
			total++
		}
	}
	return total
}

func buildDashboardSignalsWithAudit(unfinishedTasks, finishedTasks []runengine.TaskRecord, pendingApprovals []map[string]any, latestAudit map[string]any) []string {
	signals := buildDashboardSignals(unfinishedTasks, finishedTasks, pendingApprovals)
	if latestAudit != nil {
		signals = append(signals, fmt.Sprintf("最近审计摘要：%s。", truncateText(stringValue(latestAudit, "summary", "runtime audit recorded"), 48)))
	}
	return signals
}

func buildDashboardModuleHighlightsWithAudit(unfinishedTasks, finishedTasks []runengine.TaskRecord, pendingTotal int, latestAudit map[string]any) []string {
	highlights := buildDashboardModuleHighlights(unfinishedTasks, finishedTasks, pendingTotal)
	if latestAudit != nil {
		highlights = append(highlights, fmt.Sprintf("最近审计动作：%s -> %s。", truncateText(stringValue(latestAudit, "action", "audit"), 24), truncateText(stringValue(latestAudit, "target", "main_flow"), 36)))
	}
	return highlights
}

func countAuthorizedTasks(taskGroups ...[]runengine.TaskRecord) int {
	total := 0
	for _, tasks := range taskGroups {
		for _, task := range tasks {
			if len(task.Authorization) > 0 {
				total++
			}
		}
	}
	return total
}

func countExceptionTasks(taskGroups ...[]runengine.TaskRecord) int {
	total := 0
	for _, tasks := range taskGroups {
		for _, task := range tasks {
			switch task.Status {
			case "failed", "cancelled", "blocked", "ended_unfinished":
				total++
			}
		}
	}
	return total
}

func collectMirrorReferences(tasks []runengine.TaskRecord) []map[string]any {
	references := make([]map[string]any, 0)
	seen := map[string]struct{}{}
	for _, task := range tasks {
		for _, reference := range task.MirrorReferences {
			memoryID := stringValue(reference, "memory_id", "")
			if memoryID == "" {
				continue
			}
			if _, ok := seen[memoryID]; ok {
				continue
			}
			seen[memoryID] = struct{}{}
			references = append(references, cloneMap(reference))
		}
	}
	return references
}

func buildMirrorHistorySummary(tasks []runengine.TaskRecord, memoryReferences []map[string]any) []string {
	if len(tasks) == 0 {
		return []string{"当前还没有完成任务，镜像概览会在首个正式交付后生成。"}
	}

	summaries := []string{
		fmt.Sprintf("最近已完成 %d 个任务，其中 %d 个产出了正式交付。", len(tasks), countGeneratedOutputs(tasks)),
	}
	if len(memoryReferences) > 0 {
		summaries = append(summaries, fmt.Sprintf("当前累计挂接了 %d 条记忆引用，可供 task detail 与 mirror 回显复用。", len(memoryReferences)))
	}
	if latestOutputPath := latestOutputPathFromTasks(tasks); latestOutputPath != "" {
		summaries = append(summaries, fmt.Sprintf("最近一次落盘结果位于 %s。", latestOutputPath))
	}
	return summaries
}

func buildMirrorProfile(tasks []runengine.TaskRecord) map[string]any {
	if len(tasks) == 0 {
		return nil
	}

	documentCount := 0
	bubbleCount := 0
	earliestHour := 24
	latestHour := -1
	for _, task := range tasks {
		switch stringValue(task.DeliveryResult, "type", "") {
		case "workspace_document":
			documentCount++
		case "bubble":
			bubbleCount++
		}
		hour := task.StartedAt.Hour()
		if hour < earliestHour {
			earliestHour = hour
		}
		if hour > latestHour {
			latestHour = hour
		}
	}

	workStyle := "偏好即时结果回显"
	preferredOutput := "bubble"
	if documentCount >= bubbleCount {
		workStyle = "偏好结构化落盘输出"
		preferredOutput = "workspace_document"
	}
	if earliestHour == 24 || latestHour == -1 {
		earliestHour = 0
		latestHour = 0
	}

	return map[string]any{
		"work_style":       workStyle,
		"preferred_output": preferredOutput,
		"active_hours":     fmt.Sprintf("%02d-%02dh", earliestHour, latestHour+1),
	}
}

func aggregateRiskLevel(tasks []runengine.TaskRecord, pendingApprovals []map[string]any, fallback string) string {
	if len(pendingApprovals) > 0 {
		return "red"
	}
	result := fallback
	for _, task := range tasks {
		switch task.RiskLevel {
		case "red":
			return "red"
		case "yellow":
			result = "yellow"
		case "green":
			if result == "" {
				result = "green"
			}
		}
	}
	if result == "" {
		return "green"
	}
	return result
}

func aggregateSecurityStatus(tasks []runengine.TaskRecord, pendingTotal int) string {
	if pendingTotal > 0 {
		return "pending_confirmation"
	}
	for _, task := range tasks {
		status := stringValue(task.SecuritySummary, "security_status", "")
		if status != "" && status != "normal" {
			return status
		}
	}
	return "normal"
}

func latestAuditRecordFromTasks(tasks []runengine.TaskRecord) map[string]any {
	var latestAudit map[string]any
	var latestAt time.Time
	for _, task := range tasks {
		for _, auditRecord := range task.AuditRecords {
			auditAt := parseAuditTime(auditRecord)
			if latestAudit == nil || auditAt.After(latestAt) {
				latestAudit = cloneMap(auditRecord)
				latestAt = auditAt
			}
		}
	}
	return latestAudit
}

func (s *Service) latestAuditRecordFromStorage(taskID string) map[string]any {
	if s.storage == nil {
		return nil
	}
	items, _, err := s.storage.AuditStore().ListAuditRecords(context.Background(), taskID, 1, 0)
	if err != nil || len(items) == 0 {
		return nil
	}
	return items[0].Map()
}

func aggregateTokenCostSummary(unfinishedTasks, finishedTasks []runengine.TaskRecord, budgetAutoDowngrade bool) map[string]any {
	currentTaskTokens := 0
	currentTaskCost := 0.0
	if currentTask, ok := latestTokenUsageTask(unfinishedTasks, finishedTasks); ok {
		currentTaskTokens = intValueFromAny(currentTask.TokenUsage["total_tokens"])
		currentTaskCost = floatValueFromAny(currentTask.TokenUsage["estimated_cost"])
	}

	todayTokens := 0
	todayCost := 0.0
	now := time.Now()
	for _, task := range append(append([]runengine.TaskRecord{}, unfinishedTasks...), finishedTasks...) {
		if !sameDay(task.StartedAt, now) {
			continue
		}
		todayTokens += intValueFromAny(task.TokenUsage["total_tokens"])
		todayCost += floatValueFromAny(task.TokenUsage["estimated_cost"])
	}

	return map[string]any{
		"current_task_tokens":   currentTaskTokens,
		"current_task_cost":     currentTaskCost,
		"today_tokens":          todayTokens,
		"today_cost":            todayCost,
		"single_task_limit":     0.0,
		"daily_limit":           0.0,
		"budget_auto_downgrade": budgetAutoDowngrade,
	}
}

func latestTokenUsageTask(unfinishedTasks, finishedTasks []runengine.TaskRecord) (runengine.TaskRecord, bool) {
	for _, task := range unfinishedTasks {
		if len(task.TokenUsage) > 0 {
			return task, true
		}
	}
	for _, task := range finishedTasks {
		if len(task.TokenUsage) > 0 {
			return task, true
		}
	}
	return runengine.TaskRecord{}, false
}

func parseAuditTime(auditRecord map[string]any) time.Time {
	createdAt := stringValue(auditRecord, "created_at", "")
	if createdAt == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func latestRestorePointFromTasks(tasks []runengine.TaskRecord) map[string]any {
	for _, task := range tasks {
		restorePoint, ok := task.SecuritySummary["latest_restore_point"].(map[string]any)
		if ok && len(restorePoint) > 0 {
			return cloneMap(restorePoint)
		}
	}
	return nil
}

func latestRestorePointFromSummary(summary map[string]any) map[string]any {
	if summary == nil {
		return nil
	}
	latestRestorePoint, ok := summary["latest_restore_point"].(map[string]any)
	if !ok {
		return nil
	}
	return cloneMap(latestRestorePoint)
}

func (s *Service) latestRestorePointFromStorage(taskID string) map[string]any {
	if s.storage == nil {
		return nil
	}
	items, _, err := s.storage.RecoveryPointStore().ListRecoveryPoints(context.Background(), taskID, 1, 0)
	if err != nil || len(items) == 0 {
		return nil
	}
	item := items[0]
	return map[string]any{
		"recovery_point_id": item.RecoveryPointID,
		"task_id":           item.TaskID,
		"summary":           item.Summary,
		"created_at":        item.CreatedAt,
		"objects":           append([]string(nil), item.Objects...),
	}
}

func (s *Service) findRecoveryPointFromStorage(taskID, recoveryPointID string) (checkpoint.RecoveryPoint, error) {
	if s.storage == nil {
		return checkpoint.RecoveryPoint{}, fmt.Errorf("%w: recovery point store unavailable", ErrStorageQueryFailed)
	}
	item, err := s.storage.RecoveryPointStore().GetRecoveryPoint(context.Background(), recoveryPointID)
	if err != nil {
		if errors.Is(err, storage.ErrRecoveryPointNotFound) {
			return checkpoint.RecoveryPoint{}, ErrRecoveryPointNotFound
		}
		return checkpoint.RecoveryPoint{}, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
	}
	if taskID != "" && item.TaskID != taskID {
		return checkpoint.RecoveryPoint{}, ErrRecoveryPointNotFound
	}
	return item, nil
}

func recoveryPointMap(point checkpoint.RecoveryPoint) map[string]any {
	return map[string]any{
		"recovery_point_id": point.RecoveryPointID,
		"task_id":           point.TaskID,
		"summary":           point.Summary,
		"created_at":        point.CreatedAt,
		"objects":           append([]string(nil), point.Objects...),
	}
}

func (s *Service) writeRestoreAuditRecord(taskID string, point checkpoint.RecoveryPoint, applied bool, summary string) map[string]any {
	if s.audit == nil {
		return nil
	}
	input := audit.RecordInput{
		TaskID:  taskID,
		Type:    "recovery",
		Action:  "restore_apply",
		Summary: firstNonEmptyString(strings.TrimSpace(summary), "restore apply completed"),
		Target:  firstNonEmptyString(strings.Join(point.Objects, ", "), "recovery_scope"),
		Result:  map[bool]string{true: "success", false: "failed"}[applied],
	}
	if record, err := s.audit.Write(context.Background(), input); err == nil {
		return record.Map()
	}
	if record, err := s.audit.BuildRecord(input); err == nil {
		return record.Map()
	}
	return nil
}

func findTaskRecordFromStorage(records []storage.TaskRunRecord, taskID string) (runengine.TaskRecord, bool) {
	for _, record := range records {
		if record.TaskID == taskID {
			return runengine.TaskRecord{
				TaskID:            record.TaskID,
				SessionID:         record.SessionID,
				RunID:             record.RunID,
				Title:             record.Title,
				SourceType:        record.SourceType,
				Status:            record.Status,
				Intent:            cloneMap(record.Intent),
				PreferredDelivery: record.PreferredDelivery,
				FallbackDelivery:  record.FallbackDelivery,
				CurrentStep:       record.CurrentStep,
				RiskLevel:         record.RiskLevel,
				StartedAt:         record.StartedAt,
				UpdatedAt:         record.UpdatedAt,
				FinishedAt:        cloneStorageTimePointer(record.FinishedAt),
				Timeline:          taskTimelineFromStorage(record.Timeline),
				BubbleMessage:     cloneMap(record.BubbleMessage),
				DeliveryResult:    cloneMap(record.DeliveryResult),
				Artifacts:         cloneMapSlice(record.Artifacts),
				AuditRecords:      cloneMapSlice(record.AuditRecords),
				MirrorReferences:  cloneMapSlice(record.MirrorReferences),
				SecuritySummary:   cloneMap(record.SecuritySummary),
				ApprovalRequest:   cloneMap(record.ApprovalRequest),
				PendingExecution:  cloneMap(record.PendingExecution),
				Authorization:     cloneMap(record.Authorization),
				ImpactScope:       cloneMap(record.ImpactScope),
				TokenUsage:        cloneMap(record.TokenUsage),
				MemoryReadPlans:   cloneMapSlice(record.MemoryReadPlans),
				MemoryWritePlans:  cloneMapSlice(record.MemoryWritePlans),
				StorageWritePlan:  cloneMap(record.StorageWritePlan),
				ArtifactPlans:     cloneMapSlice(record.ArtifactPlans),
				Notifications:     taskNotificationsFromStorage(record.Notifications),
				LatestEvent:       cloneMap(record.LatestEvent),
				LatestToolCall:    cloneMap(record.LatestToolCall),
				CurrentStepStatus: record.CurrentStepStatus,
			}, true
		}
	}
	return runengine.TaskRecord{}, false
}

func taskTimelineFromStorage(timeline []storage.TaskStepSnapshot) []runengine.TaskStepRecord {
	if len(timeline) == 0 {
		return nil
	}
	result := make([]runengine.TaskStepRecord, len(timeline))
	for index, step := range timeline {
		result[index] = runengine.TaskStepRecord{
			StepID:        step.StepID,
			TaskID:        step.TaskID,
			Name:          step.Name,
			Status:        step.Status,
			OrderIndex:    step.OrderIndex,
			InputSummary:  step.InputSummary,
			OutputSummary: step.OutputSummary,
		}
	}
	return result
}

func taskNotificationsFromStorage(values []storage.NotificationSnapshot) []runengine.NotificationRecord {
	if len(values) == 0 {
		return nil
	}
	result := make([]runengine.NotificationRecord, len(values))
	for index, value := range values {
		result[index] = runengine.NotificationRecord{
			Method:    value.Method,
			Params:    cloneMap(value.Params),
			CreatedAt: value.CreatedAt,
		}
	}
	return result
}

func cloneStorageTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func latestOutputPathFromTasks(tasks []runengine.TaskRecord) string {
	for _, task := range tasks {
		if outputPath := pathFromDeliveryResult(task.DeliveryResult); outputPath != "" {
			return outputPath
		}
		if outputPath := stringValue(task.StorageWritePlan, "target_path", ""); outputPath != "" {
			return outputPath
		}
	}
	return ""
}

func (s *Service) refreshMirrorReferences(taskID string) {
	task, ok := s.runEngine.GetTask(taskID)
	if !ok {
		return
	}
	_, _ = s.runEngine.SetMirrorReferences(taskID, buildTaskMirrorReferences(task))
}

func buildTaskMirrorReferences(task runengine.TaskRecord) []map[string]any {
	references := make([]map[string]any, 0, len(task.MemoryReadPlans)+len(task.MemoryWritePlans))
	for index, plan := range task.MemoryReadPlans {
		query := firstNonEmptyString(
			stringValue(plan, "query", ""),
			stringValue(plan, "selection_text", ""),
		)
		query = firstNonEmptyString(query, stringValue(plan, "input_text", ""))
		query = firstNonEmptyString(query, task.Title)
		references = append(references, map[string]any{
			"memory_id": fmt.Sprintf("mem_read_%s_%d", task.TaskID, index+1),
			"reason":    firstNonEmptyString(stringValue(plan, "reason", ""), "任务开始前准备记忆召回"),
			"summary":   fmt.Sprintf("召回查询：%s", truncateText(query, 48)),
		})
	}
	for index, plan := range task.MemoryWritePlans {
		summary := firstNonEmptyString(stringValue(plan, "summary", ""), task.Title)
		references = append(references, map[string]any{
			"memory_id": fmt.Sprintf("mem_write_%s_%d", task.TaskID, index+1),
			"reason":    firstNonEmptyString(stringValue(plan, "reason", ""), "任务完成后准备写入记忆摘要"),
			"summary":   truncateText(summary, 64),
		})
	}
	return references
}

func deriveImpactScopeFiles(task runengine.TaskRecord, pendingExecution map[string]any, deliveryService *delivery.Service) []string {
	files := make([]string, 0, 4)
	files = appendImpactScopePath(files, stringValue(task.StorageWritePlan, "target_path", ""))
	for _, artifactPlan := range task.ArtifactPlans {
		files = appendImpactScopePath(files, stringValue(artifactPlan, "path", ""))
	}
	files = appendImpactScopePath(files, pathFromDeliveryResult(task.DeliveryResult))
	files = appendImpactScopePath(files, pathFromPendingExecution(task.TaskID, pendingExecution, deliveryService))
	files = appendImpactScopePath(files, targetPathFromIntent(task.Intent))
	return files
}

func appendImpactScopePath(files []string, candidate string) []string {
	candidate = strings.TrimSpace(strings.ReplaceAll(candidate, "\\", "/"))
	if candidate == "" {
		return files
	}
	candidate = path.Clean(candidate)
	if candidate == "." {
		return files
	}
	for _, existing := range files {
		if existing == candidate {
			return files
		}
	}
	return append(files, candidate)
}

func pathFromPendingExecution(taskID string, pendingExecution map[string]any, deliveryService *delivery.Service) string {
	if len(pendingExecution) == 0 {
		return ""
	}
	deliveryType := stringValue(pendingExecution, "delivery_type", "")
	if deliveryType != "workspace_document" {
		return ""
	}
	resultTitle := stringValue(pendingExecution, "result_title", "处理结果")
	previewText := stringValue(pendingExecution, "preview_text", "")
	deliveryResult := deliveryService.BuildDeliveryResult(taskID, deliveryType, resultTitle, previewText)
	return pathFromDeliveryResult(deliveryResult)
}

func pathFromDeliveryResult(deliveryResult map[string]any) string {
	payload, ok := deliveryResult["payload"].(map[string]any)
	if !ok {
		return ""
	}
	return stringValue(payload, "path", "")
}

func targetPathFromIntent(taskIntent map[string]any) string {
	targetPath := stringValue(mapValue(taskIntent, "arguments"), "target_path", "")
	switch targetPath {
	case "", "workspace_document", "bubble", "result_page", "task_detail", "open_file", "reveal_in_folder":
		return ""
	default:
		return targetPath
	}
}

func isWorkspaceRelativePath(filePath, workspaceRoot string) bool {
	normalizedRoot := strings.Trim(strings.ReplaceAll(workspaceRoot, "\\", "/"), "/")
	normalizedPath := strings.Trim(strings.ReplaceAll(filePath, "\\", "/"), "/")
	if normalizedRoot == "" {
		normalizedRoot = "workspace"
	}
	return normalizedPath == normalizedRoot || strings.HasPrefix(normalizedPath, normalizedRoot+"/")
}

func hasOverwriteOrDeleteRisk(taskIntent map[string]any) bool {
	if stringValue(taskIntent, "name", "") == "write_file" {
		return true
	}
	arguments := mapValue(taskIntent, "arguments")
	return boolValue(arguments, "overwrite", false) || boolValue(arguments, "delete", false)
}

// attachMemoryReadPlans 处理当前模块的相关逻辑。

// attachMemoryReadPlans 在任务启动或确认后挂接 memory 读取计划。
func (s *Service) attachMemoryReadPlans(taskID, runID string, snapshot contextsvc.TaskContextSnapshot, taskIntent map[string]any) {
	readPlans := []map[string]any{
		{
			"kind":           "retrieval",
			"backend":        s.memory.RetrievalBackend(),
			"task_id":        taskID,
			"run_id":         runID,
			"query":          memoryQueryFromSnapshot(snapshot),
			"reason":         "任务开始前准备记忆召回",
			"intent_name":    stringValue(taskIntent, "name", "summarize"),
			"selection_text": snapshot.SelectionText,
			"input_text":     snapshot.Text,
			"source_type":    snapshot.Trigger,
		},
	}

	_, _ = s.runEngine.SetMemoryPlans(taskID, readPlans, nil)
	s.refreshMirrorReferences(taskID)
}

// attachPostDeliveryHandoffs 处理当前模块的相关逻辑。

// attachPostDeliveryHandoffs 在任务完成后挂接 memory 写入和交付落盘计划。
func (s *Service) attachPostDeliveryHandoffs(taskID, runID string, snapshot contextsvc.TaskContextSnapshot, taskIntent map[string]any, deliveryResult map[string]any, artifacts []map[string]any) {
	writePlans := []map[string]any{
		{
			"kind":        "summary_write",
			"backend":     s.memory.RetrievalBackend(),
			"task_id":     taskID,
			"run_id":      runID,
			"summary":     buildMemorySummary(taskIntent, deliveryResult),
			"reason":      "任务完成后准备写入阶段摘要",
			"source_type": snapshot.Trigger,
		},
	}
	_, _ = s.runEngine.SetMemoryPlans(taskID, nil, writePlans)
	s.refreshMirrorReferences(taskID)

	storageWritePlan := s.delivery.BuildStorageWritePlan(taskID, deliveryResult)
	artifactPlans := s.delivery.BuildArtifactPersistPlans(taskID, artifacts)
	_, _ = s.runEngine.SetDeliveryPlans(taskID, storageWritePlan, artifactPlans)
}

// requiresAuthorization 处理当前模块的相关逻辑。

// requiresAuthorization 判断当前意图是否必须进入等待授权链路。
func requiresAuthorization(taskIntent map[string]any) bool {
	if stringValue(taskIntent, "name", "") == "write_file" {
		return true
	}

	arguments := mapValue(taskIntent, "arguments")
	if requireAuthorization, ok := arguments["require_authorization"].(bool); ok {
		return requireAuthorization
	}

	return false
}

// buildApprovalRequest 处理当前模块的相关逻辑。

// buildApprovalRequest 构造统一的 approval_request 结构。
func buildApprovalRequest(taskID string, taskIntent map[string]any, riskLevel string) map[string]any {
	arguments := mapValue(taskIntent, "arguments")
	targetObject := stringValue(arguments, "target_path", "workspace_document")
	if targetObject == "" {
		targetObject = "workspace_document"
	}

	return map[string]any{
		"approval_id":    fmt.Sprintf("appr_%s", taskID),
		"task_id":        taskID,
		"operation_name": firstNonEmptyString(stringValue(taskIntent, "name", ""), "write_file"),
		"risk_level":     firstNonEmptyString(riskLevel, "red"),
		"target_object":  targetObject,
		"reason":         "policy_requires_authorization",
		"status":         "pending",
		"created_at":     time.Now().Format(dateTimeLayout),
	}
}

// buildImpactScope 处理当前模块的相关逻辑。

// buildImpactScope 构造最小影响范围摘要，用于授权结果回传和安全面板展示。
func (s *Service) buildImpactScope(task runengine.TaskRecord, pendingExecution map[string]any) map[string]any {
	files := deriveImpactScopeFiles(task, pendingExecution, s.delivery)
	workspacePath := workspacePathFromSettings(s.runEngine.Settings())
	outOfWorkspace := false
	for _, filePath := range files {
		if !isWorkspaceRelativePath(filePath, workspacePath) {
			outOfWorkspace = true
			break
		}
	}

	return map[string]any{
		"files":                    files,
		"webpages":                 []string{},
		"apps":                     []string{},
		"out_of_workspace":         outOfWorkspace,
		"overwrite_or_delete_risk": hasOverwriteOrDeleteRisk(task.Intent),
	}
}

// snapshotFromTask 处理当前模块的相关逻辑。

// snapshotFromTask 从任务记录反推一个最小上下文快照，用于授权恢复等场景。
func snapshotFromTask(task runengine.TaskRecord) contextsvc.TaskContextSnapshot {
	return contextsvc.TaskContextSnapshot{
		Trigger:   task.SourceType,
		InputType: "text",
		Text:      task.Title,
	}
}

// memoryQueryFromSnapshot 处理当前模块的相关逻辑。

// memoryQueryFromSnapshot 从当前上下文挑选最适合作为检索 query 的内容。
func memoryQueryFromSnapshot(snapshot contextsvc.TaskContextSnapshot) string {
	for _, value := range []string{snapshot.SelectionText, snapshot.Text, snapshot.ErrorText, snapshot.PageTitle} {
		if value != "" {
			return truncateText(value, 64)
		}
	}

	if len(snapshot.Files) > 0 {
		return snapshot.Files[0]
	}

	return "task_context"
}

// buildMemorySummary 处理当前模块的相关逻辑。

// buildMemorySummary 构造任务完成后写入 memory 的简要摘要。
func buildMemorySummary(taskIntent map[string]any, deliveryResult map[string]any) string {
	intentName := stringValue(taskIntent, "name", "summarize")
	title := stringValue(deliveryResult, "title", "任务结果")
	return fmt.Sprintf("任务完成，意图=%s，交付=%s", intentName, title)
}

// resultSpecFromIntent 处理当前模块的相关逻辑。

// resultSpecFromIntent 根据 intent 返回默认结果标题、预览文案和结果气泡文案。
func resultSpecFromIntent(taskIntent map[string]any) (string, string, string) {
	switch stringValue(taskIntent, "name", "summarize") {
	case "rewrite":
		return "改写结果", "已为你写入文档并打开", "内容已经按要求改写完成，可直接查看。"
	case "translate":
		return "翻译结果", "结果已通过气泡返回", "翻译结果已经生成，可直接查看。"
	case "explain":
		return "解释结果", "结果已通过气泡返回", "这段内容的意思已经整理好了。"
	case "write_file":
		return "文件写入结果", "已为你写入文档并打开", "文件已经生成，可直接查看。"
	default:
		return "处理结果", "已为你写入文档并打开", "结果已经生成，可直接查看。"
	}
}

// deliveryTypeFromIntent 处理当前模块的相关逻辑。

// deliveryTypeFromIntent 根据意图类型返回默认交付方式。
func deliveryTypeFromIntent(taskIntent map[string]any) string {
	switch stringValue(taskIntent, "name", "summarize") {
	case "translate", "explain":
		return "bubble"
	default:
		return "workspace_document"
	}
}

// firstNonEmptyString 处理当前模块的相关逻辑。
func deliveryPreferenceFromSubmit(params map[string]any) (string, string) {
	options := mapValue(params, "options")
	return stringValue(options, "preferred_delivery", ""), ""
}

func deliveryPreferenceFromStart(params map[string]any) (string, string) {
	deliveryOptions := mapValue(params, "delivery")
	return stringValue(deliveryOptions, "preferred", ""), stringValue(deliveryOptions, "fallback", "")
}

// buildPendingExecution 生成等待授权任务在恢复执行时需要的交付计划。
func (s *Service) buildPendingExecution(task runengine.TaskRecord, taskIntent map[string]any) map[string]any {
	plan := s.delivery.BuildApprovalExecutionPlan(task.TaskID, taskIntent)
	return s.applyResolvedDeliveryToPlan(task, plan, taskIntent)
}

// applyResolvedDeliveryToPlan 把任务级交付偏好解析结果回填到恢复执行计划中。
func (s *Service) applyResolvedDeliveryToPlan(task runengine.TaskRecord, plan map[string]any, taskIntent map[string]any) map[string]any {
	if len(plan) == 0 {
		return nil
	}

	updatedPlan := cloneMap(plan)
	deliveryType := resolveTaskDeliveryType(task, taskIntent)
	updatedPlan["delivery_type"] = deliveryType
	updatedPlan["preview_text"] = previewTextForDeliveryType(deliveryType)
	return updatedPlan
}

// resolveTaskDeliveryType 统一计算某个任务当前应采用的交付类型。
func resolveTaskDeliveryType(task runengine.TaskRecord, taskIntent map[string]any) string {
	return resolveDeliveryType(task.PreferredDelivery, task.FallbackDelivery, deliveryTypeFromIntent(taskIntent))
}

// resolveDeliveryType 按“任务偏好 -> fallback -> 默认值”的顺序解析最终交付类型。
func resolveDeliveryType(preferred, fallback, defaultType string) string {
	if normalized := normalizeDeliveryType(preferred); normalized != "" {
		return normalized
	}
	if strings.TrimSpace(preferred) != "" {
		if normalized := normalizeDeliveryType(fallback); normalized != "" {
			return normalized
		}
	}
	if normalized := normalizeDeliveryType(defaultType); normalized != "" {
		return normalized
	}
	if normalized := normalizeDeliveryType(fallback); normalized != "" {
		return normalized
	}
	return "workspace_document"
}

func normalizeDeliveryType(deliveryType string) string {
	switch deliveryType {
	case "bubble", "workspace_document":
		return deliveryType
	default:
		return ""
	}
}

// previewTextForDeliveryType 返回不同交付类型对应的预览文案。
func previewTextForDeliveryType(deliveryType string) string {
	if deliveryType == "bubble" {
		return "\u7ed3\u679c\u5df2\u901a\u8fc7\u6c14\u6ce1\u8fd4\u56de"
	}
	return "\u5df2\u4e3a\u4f60\u5199\u5165\u6587\u6863\u5e76\u6253\u5f00"
}

func firstNonEmptyString(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

func compactAuditRecords(records ...map[string]any) []map[string]any {
	if len(records) == 0 {
		return nil
	}

	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		if len(record) == 0 {
			continue
		}
		items = append(items, cloneMap(record))
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func sameDay(left, right time.Time) bool {
	left = left.In(right.Location())
	return left.Year() == right.Year() && left.YearDay() == right.YearDay()
}

func intValueFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func floatValueFromAny(value any) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0.0
	}
}

// firstMapOrNil 处理当前模块的相关逻辑。

// firstMapOrNil 返回列表中的第一项拷贝；如果为空则返回 nil。
func firstMapOrNil(items []map[string]any) map[string]any {
	if len(items) == 0 {
		return nil
	}
	return cloneMap(items[0])
}

// latestRestorePointFromApprovals 处理当前模块的相关逻辑。
func latestRestorePointFromApprovals(items []map[string]any) any {
	if len(items) == 0 {
		return nil
	}
	return map[string]any{
		"recovery_point_id": fmt.Sprintf("rp_%s", stringValue(items[0], "task_id", "latest")),
		"created_at":        time.Now().Format(dateTimeLayout),
	}
}

// cloneMap 处理当前模块的相关逻辑。

// cloneMap 对 map[string]any 做递归复制，避免不同层之间共享引用。
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

// cloneMapSlice 处理当前模块的相关逻辑。

// cloneMapSlice 对 []map[string]any 做逐项复制。
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

// mapValue 处理当前模块的相关逻辑。

// mapValue 安全读取嵌套对象字段。
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

// stringValue 处理当前模块的相关逻辑。

// stringValue 安全读取字符串字段，并在空值时回退到默认值。
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

// boolValue 处理当前模块的相关逻辑。

// boolValue 安全读取布尔字段。
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

// intValue 处理当前模块的相关逻辑。

// intValue 安全读取经过 JSON 解码后的数值字段。
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

// truncateText 处理当前模块的相关逻辑。

// truncateText 以固定长度截断文本，用于推荐文案和 memory 查询。
func truncateText(value string, maxLength int) string {
	if len(value) <= maxLength {
		return value
	}
	return value[:maxLength] + "..."
}

// dateTimeLayout 定义当前模块的基础变量。
func (s *Service) executeTask(task runengine.TaskRecord, snapshot contextsvc.TaskContextSnapshot, taskIntent map[string]any) (runengine.TaskRecord, map[string]any, map[string]any, []map[string]any, error) {
	processingTask, ok := s.runEngine.BeginExecution(task.TaskID, "generate_output", "开始生成正式结果")
	if !ok {
		return runengine.TaskRecord{}, nil, nil, nil, ErrTaskNotFound
	}

	resultTitle, _, resultBubbleText := resultSpecFromIntent(taskIntent)
	deliveryType := resolveTaskDeliveryType(processingTask, taskIntent)

	if s.executor == nil {
		deliveryResult := s.delivery.BuildDeliveryResultWithTargetPath(
			processingTask.TaskID,
			deliveryType,
			resultTitle,
			previewTextForDeliveryType(deliveryType),
			targetPathFromIntent(taskIntent),
		)
		artifacts := s.delivery.BuildArtifact(processingTask.TaskID, resultTitle, deliveryResult)
		resultBubble := s.delivery.BuildBubbleMessage(processingTask.TaskID, "result", resultBubbleText, processingTask.UpdatedAt.Format(dateTimeLayout))
		processingTask = s.appendAuditData(processingTask, compactAuditRecords(s.audit.BuildDeliveryAudit(processingTask.TaskID, processingTask.RunID, deliveryResult)), nil)
		updatedTask, ok := s.runEngine.CompleteTask(processingTask.TaskID, deliveryResult, resultBubble, artifacts)
		if !ok {
			return runengine.TaskRecord{}, nil, nil, nil, ErrTaskNotFound
		}
		s.attachPostDeliveryHandoffs(updatedTask.TaskID, updatedTask.RunID, snapshot, taskIntent, deliveryResult, artifacts)
		return updatedTask, resultBubble, deliveryResult, artifacts, nil
	}

	executionResult, err := s.executor.Execute(context.Background(), execution.Request{
		TaskID:       processingTask.TaskID,
		RunID:        processingTask.RunID,
		Title:        processingTask.Title,
		Intent:       taskIntent,
		Snapshot:     snapshot,
		DeliveryType: deliveryType,
		ResultTitle:  resultTitle,
	})
	if err != nil {
		return runengine.TaskRecord{}, nil, nil, nil, fmt.Errorf("execute task %s: %w", processingTask.TaskID, err)
	}

	for _, toolCall := range executionResult.ToolCalls {
		if toolCall.ToolName == "" {
			continue
		}
		if recordedTask, ok := s.runEngine.RecordToolCall(
			processingTask.TaskID,
			toolCall.ToolName,
			toolCall.Input,
			toolCall.Output,
			toolCall.DurationMS,
		); ok {
			processingTask = recordedTask
		}
	}
	executionAuditRecords, executionTokenUsage := s.buildExecutionAudit(processingTask, executionResult.ToolCalls, executionResult.DeliveryResult)
	processingTask = s.appendAuditData(processingTask, executionAuditRecords, executionTokenUsage)

	resultBubble := s.delivery.BuildBubbleMessage(
		processingTask.TaskID,
		"result",
		firstNonEmptyString(executionResult.BubbleText, resultBubbleText),
		processingTask.UpdatedAt.Format(dateTimeLayout),
	)
	updatedTask, ok := s.runEngine.CompleteTask(processingTask.TaskID, executionResult.DeliveryResult, resultBubble, executionResult.Artifacts, executionResult.RecoveryPoint)
	if !ok {
		return runengine.TaskRecord{}, nil, nil, nil, ErrTaskNotFound
	}
	s.attachPostDeliveryHandoffs(updatedTask.TaskID, updatedTask.RunID, snapshot, taskIntent, executionResult.DeliveryResult, executionResult.Artifacts)
	return updatedTask, resultBubble, executionResult.DeliveryResult, executionResult.Artifacts, nil
}

func (s *Service) buildExecutionAudit(task runengine.TaskRecord, toolCalls []tools.ToolCallRecord, deliveryResult map[string]any) ([]map[string]any, map[string]any) {
	if s.audit == nil {
		return nil, nil
	}

	auditRecords := make([]map[string]any, 0, len(toolCalls)+1)
	var tokenUsage map[string]any
	for _, toolCall := range toolCalls {
		auditRecord, usage, ok := s.audit.BuildToolAudit(task.TaskID, task.RunID, toolCall)
		if ok {
			auditRecords = append(auditRecords, auditRecord)
		}
		if len(usage) > 0 {
			tokenUsage = cloneMap(usage)
		}
	}
	if deliveryAudit := s.audit.BuildDeliveryAudit(task.TaskID, task.RunID, deliveryResult); len(deliveryAudit) > 0 {
		auditRecords = append(auditRecords, deliveryAudit)
	}

	return auditRecords, tokenUsage
}

func (s *Service) appendAuditData(task runengine.TaskRecord, auditRecords []map[string]any, tokenUsage map[string]any) runengine.TaskRecord {
	if len(auditRecords) == 0 && len(tokenUsage) == 0 {
		return task
	}
	updatedTask, ok := s.runEngine.AppendAuditData(task.TaskID, auditRecords, tokenUsage)
	if !ok {
		return task
	}
	return updatedTask
}

const dateTimeLayout = time.RFC3339

func stringSliceValue(rawValue any) []string {
	values, ok := rawValue.([]string)
	if ok {
		return append([]string(nil), values...)
	}

	anyValues, ok := rawValue.([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(anyValues))
	for _, rawItem := range anyValues {
		item, ok := rawItem.(string)
		if ok && strings.TrimSpace(item) != "" {
			result = append(result, item)
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
}
