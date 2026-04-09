// 该文件负责 4 号主链路的任务编排与对外语义收口。
package orchestrator

import (
	"errors"
	"fmt"
	"path"
	"strings"
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

// ErrTaskNotFound 定义当前模块的基础变量。
var ErrTaskNotFound = errors.New("task not found")

// Service 提供当前模块的服务能力。
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

// NewService 创建并返回Service。
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

// Snapshot 处理当前模块的相关逻辑。
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
func (s *Service) SubmitInput(params map[string]any) (map[string]any, error) {
	snapshot := s.context.Capture(params)
	options := mapValue(params, "options")
	confirmRequired := boolValue(options, "confirm_required", true)
	preferredDelivery, fallbackDelivery := deliveryPreferenceFromSubmit(params)
	suggestion := s.intent.Suggest(snapshot, nil, confirmRequired)
	if s.intent.Analyze(snapshot.Text) == "waiting_input" {
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
	artifacts := []map[string]any(nil)
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
		deliveryType := resolveTaskDeliveryType(task, suggestion.Intent)
		deliveryResult = s.delivery.BuildDeliveryResult(task.TaskID, deliveryType, suggestion.ResultTitle, previewTextForDeliveryType(deliveryType))
		artifacts = s.delivery.BuildArtifact(task.TaskID, suggestion.ResultTitle, deliveryResult)
		if _, ok := s.runEngine.CompleteTask(task.TaskID, deliveryResult, bubble, artifacts); ok {
			task, _ = s.runEngine.GetTask(task.TaskID)
		}
		s.attachPostDeliveryHandoffs(task.TaskID, task.RunID, snapshot, suggestion.Intent, deliveryResult, artifacts)
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

	deliveryType := resolveTaskDeliveryType(task, suggestion.Intent)
	deliveryResult := s.delivery.BuildDeliveryResult(task.TaskID, deliveryType, suggestion.ResultTitle, previewTextForDeliveryType(deliveryType))
	artifacts := s.delivery.BuildArtifact(task.TaskID, suggestion.ResultTitle, deliveryResult)
	if _, ok := s.runEngine.CompleteTask(task.TaskID, deliveryResult, bubble, artifacts); ok {
		task, _ = s.runEngine.GetTask(task.TaskID)
		response["task"] = taskMap(task)
	}
	s.attachPostDeliveryHandoffs(task.TaskID, task.RunID, snapshot, suggestion.Intent, deliveryResult, artifacts)
	response["delivery_result"] = deliveryResult
	return response, nil
}

// ConfirmTask 确认Task。
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

	resultTitle, resultPreview, resultBubbleText := resultSpecFromIntent(intentValue)
	deliveryType := resolveTaskDeliveryType(updatedTask, intentValue)
	resultPreview = previewTextForDeliveryType(deliveryType)
	deliveryResult := s.delivery.BuildDeliveryResult(updatedTask.TaskID, deliveryType, resultTitle, resultPreview)
	artifacts := s.delivery.BuildArtifact(updatedTask.TaskID, resultTitle, deliveryResult)
	resultBubble := s.delivery.BuildBubbleMessage(updatedTask.TaskID, "result", resultBubbleText, updatedTask.UpdatedAt.Format(dateTimeLayout))
	updatedTask, ok = s.runEngine.CompleteTask(updatedTask.TaskID, deliveryResult, resultBubble, artifacts)
	if !ok {
		return nil, ErrTaskNotFound
	}
	s.attachPostDeliveryHandoffs(updatedTask.TaskID, updatedTask.RunID, snapshot, intentValue, deliveryResult, artifacts)

	return map[string]any{
		"task":            taskMap(updatedTask),
		"bubble_message":  resultBubble,
		"delivery_result": deliveryResult,
	}, nil
}

// RecommendationGet 处理当前模块的相关逻辑。
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

// RecommendationFeedbackSubmit 处理当前模块的相关逻辑。
func (s *Service) RecommendationFeedbackSubmit(params map[string]any) (map[string]any, error) {
	_ = params
	return map[string]any{"applied": true}, nil
}

// TaskList 处理当前模块的相关逻辑。
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

// TaskControl 处理当前模块的相关逻辑。
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

// TaskInspectorConfigGet 处理当前模块的相关逻辑。
func (s *Service) TaskInspectorConfigGet() (map[string]any, error) {
	return s.runEngine.InspectorConfig(), nil
}

// TaskInspectorConfigUpdate 处理当前模块的相关逻辑。
func (s *Service) TaskInspectorConfigUpdate(params map[string]any) (map[string]any, error) {
	effective := s.runEngine.UpdateInspectorConfig(params)
	return map[string]any{
		"updated":          true,
		"effective_config": effective,
	}, nil
}

// TaskInspectorRun 处理当前模块的相关逻辑。
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

// NotepadList 处理当前模块的相关逻辑。
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

// DashboardOverviewGet 处理当前模块的相关逻辑。
func (s *Service) DashboardOverviewGet(params map[string]any) (map[string]any, error) {
	_ = params
	tasks, _ := s.runEngine.ListTasks("unfinished", "updated_at", "desc", 1, 0)
	_, pendingTotal := s.runEngine.PendingApprovalRequests(20, 0)
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
				"pending_authorizations": pendingTotal,
				"has_restore_point":      len(tasks) > 0 && tasks[0].SecuritySummary["latest_restore_point"] != nil,
				"workspace_path":         workspacePathFromSettings(s.runEngine.Settings()),
			},
			"quick_actions":     []string{"打开任务详情", "查看最近结果"},
			"global_state":      s.Snapshot(),
			"high_value_signal": []string{"主链路 task/run 映射已进入内存态运行。"},
		},
	}, nil
}

// DashboardModuleGet 处理当前模块的相关逻辑。
func (s *Service) DashboardModuleGet(params map[string]any) (map[string]any, error) {
	module := stringValue(params, "module", "mirror")
	tab := stringValue(params, "tab", "daily_summary")
	tasks, _ := s.runEngine.ListTasks("finished", "finished_at", "desc", 20, 0)
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

// MirrorOverviewGet 处理当前模块的相关逻辑。
func (s *Service) MirrorOverviewGet(params map[string]any) (map[string]any, error) {
	_ = params
	tasks, _ := s.runEngine.ListTasks("finished", "finished_at", "desc", 20, 0)
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

// SecuritySummaryGet 处理当前模块的相关逻辑。
func (s *Service) SecuritySummaryGet() (map[string]any, error) {
	pendingApprovals, pendingTotal := s.runEngine.PendingApprovalRequests(20, 0)
	securityStatus := "normal"
	if pendingTotal > 0 {
		securityStatus = "pending_confirmation"
	}
	return map[string]any{
		"summary": map[string]any{
			"security_status":        securityStatus,
			"pending_authorizations": pendingTotal,
			"latest_restore_point":   latestRestorePointFromApprovals(pendingApprovals),
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

// SecurityPendingList 处理当前模块的相关逻辑。
func (s *Service) SecurityPendingList(params map[string]any) (map[string]any, error) {
	limit := intValue(params, "limit", 20)
	offset := intValue(params, "offset", 0)
	items, total := s.runEngine.PendingApprovalRequests(limit, offset)
	return map[string]any{
		"items": items,
		"page":  pageMap(limit, offset, total),
	}, nil
}

// PendingNotifications 返回待处理的Notifications。
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
	impactScope := buildImpactScope()
	if decision == "deny_once" {
		bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "已拒绝本次操作，任务已取消。", task.UpdatedAt.Format(dateTimeLayout))
		updatedTask, ok := s.runEngine.DenyAfterApproval(task.TaskID, authorizationRecord, impactScope, bubble)
		if !ok {
			return nil, ErrTaskNotFound
		}
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

	pendingExecution, ok := s.runEngine.PendingExecutionPlan(task.TaskID)
	if !ok {
		pendingExecution = s.buildPendingExecution(task, task.Intent)
	}
	pendingExecution = s.applyResolvedDeliveryToPlan(task, pendingExecution, task.Intent)

	resultTitle := stringValue(pendingExecution, "result_title", "处理结果")
	resultPreview := stringValue(pendingExecution, "preview_text", "已为你写入文档并打开")
	resultBubbleText := stringValue(pendingExecution, "result_bubble_text", "结果已经生成，可直接查看。")
	deliveryType := stringValue(pendingExecution, "delivery_type", deliveryTypeFromIntent(task.Intent))
	deliveryType = resolveTaskDeliveryType(task, task.Intent)
	resultPreview = previewTextForDeliveryType(deliveryType)
	deliveryResult := s.delivery.BuildDeliveryResult(task.TaskID, deliveryType, resultTitle, resultPreview)
	artifacts := s.delivery.BuildArtifact(task.TaskID, resultTitle, deliveryResult)
	resultBubble := s.delivery.BuildBubbleMessage(task.TaskID, "result", resultBubbleText, processingTask.UpdatedAt.Format(dateTimeLayout))
	updatedTask, ok := s.runEngine.CompleteTask(task.TaskID, deliveryResult, resultBubble, artifacts)
	if !ok {
		return nil, ErrTaskNotFound
	}
	updatedTask, _ = s.runEngine.ResolveAuthorization(task.TaskID, authorizationRecord, impactScope)
	s.attachPostDeliveryHandoffs(updatedTask.TaskID, updatedTask.RunID, snapshotFromTask(updatedTask), updatedTask.Intent, deliveryResult, artifacts)

	return map[string]any{
		"authorization_record": authorizationRecord,
		"task":                 taskMap(updatedTask),
		"bubble_message":       resultBubble,
		"impact_scope":         impactScope,
	}, nil
}

// SettingsGet 设置tingsGet。
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
	return "return_result"
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

// currentTimeFromTask 处理当前模块的相关逻辑。
func currentTimeFromTask(engine *runengine.Engine, taskID string) string {
	task, ok := engine.GetTask(taskID)
	if !ok {
		return ""
	}
	return task.UpdatedAt.Format(dateTimeLayout)
}

// workspacePathFromSettings 处理当前模块的相关逻辑。
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

// defaultMirrorReference 处理当前模块的相关逻辑。
func defaultMirrorReference() map[string]any {
	return map[string]any{
		"memory_id": "pref_001",
		"reason":    "当前任务命中了用户的输出偏好",
		"summary":   "偏好简洁三点式摘要",
	}
}

// attachMemoryReadPlans 处理当前模块的相关逻辑。
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
}

// attachPostDeliveryHandoffs 处理当前模块的相关逻辑。
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

	storageWritePlan := s.delivery.BuildStorageWritePlan(taskID, deliveryResult)
	artifactPlans := s.delivery.BuildArtifactPersistPlans(taskID, artifacts)
	_, _ = s.runEngine.SetDeliveryPlans(taskID, storageWritePlan, artifactPlans)
}

// requiresAuthorization 处理当前模块的相关逻辑。
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
func buildImpactScope() map[string]any {
	return map[string]any{
		"files":                    []string{path.Join("workspace", "report.md")},
		"webpages":                 []string{},
		"apps":                     []string{},
		"out_of_workspace":         false,
		"overwrite_or_delete_risk": false,
	}
}

// snapshotFromTask 处理当前模块的相关逻辑。
func snapshotFromTask(task runengine.TaskRecord) contextsvc.TaskContextSnapshot {
	return contextsvc.TaskContextSnapshot{
		Trigger:   task.SourceType,
		InputType: "text",
		Text:      task.Title,
	}
}

// memoryQueryFromSnapshot 处理当前模块的相关逻辑。
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
func buildMemorySummary(taskIntent map[string]any, deliveryResult map[string]any) string {
	intentName := stringValue(taskIntent, "name", "summarize")
	title := stringValue(deliveryResult, "title", "任务结果")
	return fmt.Sprintf("任务完成，意图=%s，交付=%s", intentName, title)
}

// resultSpecFromIntent 处理当前模块的相关逻辑。
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

func (s *Service) buildPendingExecution(task runengine.TaskRecord, taskIntent map[string]any) map[string]any {
	plan := s.delivery.BuildApprovalExecutionPlan(task.TaskID, taskIntent)
	return s.applyResolvedDeliveryToPlan(task, plan, taskIntent)
}

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

func resolveTaskDeliveryType(task runengine.TaskRecord, taskIntent map[string]any) string {
	return resolveDeliveryType(task.PreferredDelivery, task.FallbackDelivery, deliveryTypeFromIntent(taskIntent))
}

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

// firstMapOrNil 处理当前模块的相关逻辑。
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
func truncateText(value string, maxLength int) string {
	if len(value) <= maxLength {
		return value
	}
	return value[:maxLength] + "..."
}

// dateTimeLayout 定义当前模块的基础变量。
const dateTimeLayout = time.RFC3339
