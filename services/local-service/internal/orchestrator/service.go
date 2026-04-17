// 该文件负责 4 号主链路的任务编排与对外语义收口。
package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
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
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/perception"
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
	ErrTaskNotFound           = errors.New("task not found")
	ErrArtifactNotFound       = errors.New("artifact not found")
	ErrTaskStatusInvalid      = errors.New("task status invalid")
	ErrTaskAlreadyFinished    = errors.New("task already finished")
	ErrStorageQueryFailed     = errors.New("storage query failed")
	ErrStrongholdAccessFailed = errors.New("stronghold access failed")
	ErrRecoveryPointNotFound  = errors.New("recovery point not found")
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
	confirmRequired := boolValue(options, "confirm_required", false)
	suggestion := s.intent.Suggest(snapshot, nil, confirmRequired)
	preferredDelivery, fallbackDelivery := deliveryPreferenceFromSubmit(params)
	if !suggestion.RequiresConfirm {
		preferredDelivery, fallbackDelivery = mergeSuggestedDeliveryPreference(preferredDelivery, fallbackDelivery, suggestion.DirectDeliveryType)
	}
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
			Snapshot:          snapshot,
		})

		bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "请先告诉我你希望我处理什么内容。", task.StartedAt.Format(dateTimeLayout))
		if _, ok := s.runEngine.SetPresentation(task.TaskID, bubble, nil, nil); ok {
			task, _ = s.runEngine.GetTask(task.TaskID)
		}

		return map[string]any{
			"task":            taskMap(task),
			"bubble_message":  bubble,
			"delivery_result": nil,
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
		CurrentStep:       currentStepForSuggestion(suggestion.RequiresConfirm, suggestion.Intent),
		RiskLevel:         s.risk.DefaultLevel(),
		Timeline:          initialTimeline(taskStatusForSuggestion(suggestion.RequiresConfirm), currentStepForSuggestion(suggestion.RequiresConfirm, suggestion.Intent)),
		Snapshot:          snapshot,
	})
	s.attachMemoryReadPlans(task.TaskID, task.RunID, snapshot, suggestion.Intent)

	bubble := s.delivery.BuildBubbleMessage(task.TaskID, bubbleTypeForSuggestion(suggestion.RequiresConfirm), bubbleTextForInput(suggestion), task.StartedAt.Format(dateTimeLayout))
	deliveryResult := map[string]any(nil)
	if !suggestion.RequiresConfirm {
		if queuedTask, queueBubble, queued, queueErr := s.queueTaskIfSessionBusy(task); queueErr != nil {
			return nil, queueErr
		} else if queued {
			task = queuedTask
			bubble = queueBubble
		} else {
			governedTask, governedResponse, handled, governanceErr := s.handleTaskGovernanceDecision(task, suggestion.Intent)
			if governanceErr != nil {
				return nil, governanceErr
			}
			if handled {
				return governedResponse, nil
			}
			task = governedTask
			var execErr error
			task, bubble, deliveryResult, _, execErr = s.executeTask(task, snapshot, suggestion.Intent)
			if execErr != nil {
				return nil, execErr
			}
		}
	} else {
		if _, ok := s.runEngine.SetPresentation(task.TaskID, bubble, nil, nil); ok {
			task, _ = s.runEngine.GetTask(task.TaskID)
		}
	}

	response := map[string]any{
		"task":            taskMap(task),
		"bubble_message":  bubble,
		"delivery_result": nil,
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
	suggestion := s.intent.Suggest(snapshot, explicitIntent, len(explicitIntent) == 0)
	preferredDelivery, fallbackDelivery := deliveryPreferenceFromStart(params)
	if len(explicitIntent) == 0 && !suggestion.RequiresConfirm {
		preferredDelivery, fallbackDelivery = mergeSuggestedDeliveryPreference(preferredDelivery, fallbackDelivery, suggestion.DirectDeliveryType)
	}

	task := s.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:         stringValue(params, "session_id", ""),
		Title:             suggestion.TaskTitle,
		SourceType:        suggestion.TaskSourceType,
		Status:            taskStatusForSuggestion(suggestion.RequiresConfirm),
		Intent:            suggestion.Intent,
		PreferredDelivery: preferredDelivery,
		FallbackDelivery:  fallbackDelivery,
		CurrentStep:       currentStepForSuggestion(suggestion.RequiresConfirm, suggestion.Intent),
		RiskLevel:         s.risk.DefaultLevel(),
		Timeline:          initialTimeline(taskStatusForSuggestion(suggestion.RequiresConfirm), currentStepForSuggestion(suggestion.RequiresConfirm, suggestion.Intent)),
		Snapshot:          snapshot,
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

	if queuedTask, queueBubble, queued, queueErr := s.queueTaskIfSessionBusy(task); queueErr != nil {
		return nil, queueErr
	} else if queued {
		response["task"] = taskMap(queuedTask)
		response["bubble_message"] = queueBubble
		return response, nil
	}

	governedTask, governedResponse, handled, governanceErr := s.handleTaskGovernanceDecision(task, suggestion.Intent)
	if governanceErr != nil {
		return nil, governanceErr
	}
	if handled {
		return governedResponse, nil
	}
	task = governedTask

	deliveryResult := map[string]any(nil)
	var execErr error
	task, bubble, deliveryResult, _, execErr = s.executeTask(task, snapshot, suggestion.Intent)
	if execErr != nil {
		return nil, execErr
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
	if !boolValue(params, "confirmed", false) {
		bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "已取消本次处理，请重新告诉我你的目标。", task.UpdatedAt.Format(dateTimeLayout))
		updatedTask, err := s.runEngine.ControlTask(task.TaskID, "cancel", bubble)
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
			"task":            taskMap(updatedTask),
			"bubble_message":  bubble,
			"delivery_result": nil,
		}, nil
	}

	intentValue := mapValue(params, "corrected_intent")
	if len(intentValue) == 0 {
		intentValue = cloneMap(task.Intent)
	}
	if strings.TrimSpace(stringValue(intentValue, "name", "")) == "" {
		bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "请先明确告诉我你希望执行的处理方式。", task.UpdatedAt.Format(dateTimeLayout))
		if updatedTask, ok := s.runEngine.SetPresentation(task.TaskID, bubble, nil, nil); ok {
			return map[string]any{
				"task":            taskMap(updatedTask),
				"bubble_message":  bubble,
				"delivery_result": nil,
			}, nil
		}
		return nil, ErrTaskNotFound
	}
	updatedTitle := s.intent.Suggest(snapshotFromTask(task), intentValue, false).TaskTitle

	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "已按新的要求开始处理", task.UpdatedAt.Format(dateTimeLayout))
	updatedTask, ok := s.runEngine.UpdateIntent(task.TaskID, updatedTitle, intentValue)
	if !ok {
		return nil, ErrTaskNotFound
	}
	s.attachMemoryReadPlans(updatedTask.TaskID, updatedTask.RunID, snapshotFromTask(updatedTask), intentValue)
	if queuedTask, queueBubble, queued, queueErr := s.queueTaskIfSessionBusy(updatedTask); queueErr != nil {
		return nil, queueErr
	} else if queued {
		return map[string]any{
			"task":            taskMap(queuedTask),
			"bubble_message":  queueBubble,
			"delivery_result": nil,
		}, nil
	}
	governedTask, governedResponse, handled, governanceErr := s.handleTaskGovernanceDecision(updatedTask, intentValue)
	if governanceErr != nil {
		return nil, governanceErr
	}
	if handled {
		return governedResponse, nil
	}
	updatedTask = governedTask

	updatedTask, ok = s.runEngine.ConfirmTask(task.TaskID, updatedTitle, intentValue, bubble)
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
	signals := perception.CaptureContextSignals(stringValue(params, "source", "floating_ball"), stringValue(params, "scene", "hover"), contextValue)
	unfinishedTasks, _ := s.runEngine.ListTasks("unfinished", "updated_at", "desc", 20, 0)
	finishedTasks, _ := s.runEngine.ListTasks("finished", "finished_at", "desc", 20, 0)
	notepadItems, _ := s.runEngine.NotepadItems("", 20, 0)
	result := s.recommendation.Get(recommendation.GenerateInput{
		Source:          stringValue(params, "source", "floating_ball"),
		Scene:           stringValue(params, "scene", "hover"),
		PageTitle:       signals.PageTitle,
		PageURL:         signals.PageURL,
		AppName:         signals.AppName,
		WindowTitle:     signals.WindowTitle,
		VisibleText:     signals.VisibleText,
		ScreenSummary:   signals.ScreenSummary,
		SelectionText:   signals.SelectionText,
		ClipboardText:   signals.ClipboardText,
		ClipboardMime:   signals.ClipboardMimeType,
		HoverTarget:     signals.HoverTarget,
		LastAction:      signals.LastAction,
		ErrorText:       signals.ErrorText,
		DwellMillis:     signals.DwellMillis,
		WindowSwitches:  signals.WindowSwitchCount,
		PageSwitches:    signals.PageSwitchCount,
		CopyCount:       signals.CopyCount,
		Signals:         signals,
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
	if total == 0 {
		if persistedTasks, persistedTotal, ok := s.listTasksFromStorage(group, sortBy, sortOrder, limit, offset); ok {
			tasks = persistedTasks
			total = persistedTotal
		}
	}

	items := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, taskMap(task))
	}

	return map[string]any{
		"items": items,
		"page":  pageMap(limit, offset, total),
	}, nil
}

// TaskDetailGet returns the task detail payload for `agent.task.detail.get`.
// It normalizes collection fields and protocol-facing objects before they cross
// the JSON-RPC boundary.
func (s *Service) TaskDetailGet(params map[string]any) (map[string]any, error) {
	taskID := stringValue(params, "task_id", "")
	task, ok := s.runEngine.TaskDetail(taskID)
	if !ok {
		task, ok = s.taskDetailFromStorage(taskID)
	}
	if !ok {
		return nil, ErrTaskNotFound
	}

	securitySummary := cloneMap(task.SecuritySummary)
	if securitySummary == nil {
		securitySummary = map[string]any{}
	}
	approvalRequest := activeTaskDetailApprovalRequest(task)
	approvalRequestValue := any(nil)
	if approvalRequest != nil {
		approvalRequestValue = approvalRequest
	}
	securitySummary["pending_authorizations"] = 0
	if approvalRequest != nil {
		securitySummary["pending_authorizations"] = 1
	}
	latestRestorePoint := s.normalizeTaskDetailRestorePoint(task.TaskID, securitySummary)
	if latestRestorePoint == nil {
		securitySummary["latest_restore_point"] = nil
	} else {
		securitySummary["latest_restore_point"] = latestRestorePoint
	}

	return map[string]any{
		"task":              taskMap(task),
		"timeline":          protocolTaskStepList(timelineMap(task.Timeline)),
		"artifacts":         protocolArtifactList(s.artifactsForTask(task.TaskID, task.Artifacts)),
		"mirror_references": protocolMirrorReferenceList(task.MirrorReferences),
		"approval_request":  approvalRequestValue,
		"security_summary":  securitySummary,
	}, nil
}

// TaskArtifactList handles `agent.task.artifact.list` and returns protocol-ready
// artifact items.
func (s *Service) TaskArtifactList(params map[string]any) (map[string]any, error) {
	limit := clampListLimit(intValue(params, "limit", 20))
	offset := clampListOffset(intValue(params, "offset", 0))
	taskID := stringValue(params, "task_id", "")
	if strings.TrimSpace(taskID) == "" {
		return nil, errors.New("task_id is required")
	}
	items, total, err := s.listArtifactsPage(taskID, limit, offset)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"items": protocolArtifactList(items),
		"page":  pageMap(limit, offset, total),
	}, nil
}

// TaskArtifactOpen handles `agent.task.artifact.open` and keeps the open
// resolution metadata while exposing a formal Artifact payload.
func (s *Service) TaskArtifactOpen(params map[string]any) (map[string]any, error) {
	taskID := stringValue(params, "task_id", "")
	artifactID := stringValue(params, "artifact_id", "")
	if strings.TrimSpace(taskID) == "" {
		return nil, errors.New("task_id is required")
	}
	if strings.TrimSpace(artifactID) == "" {
		return nil, errors.New("artifact_id is required")
	}
	artifact, err := s.findArtifactForTask(taskID, artifactID)
	if err != nil {
		return nil, err
	}
	openResult := buildDeliveryOpenResult(cloneMap(artifact), nil, taskID)
	openResult["artifact"] = protocolArtifactMap(artifact)
	return openResult, nil
}

// DeliveryOpen handles `agent.delivery.open` and resolves the final open action.
func (s *Service) DeliveryOpen(params map[string]any) (map[string]any, error) {
	taskID := stringValue(params, "task_id", "")
	if strings.TrimSpace(taskID) == "" {
		return nil, errors.New("task_id is required")
	}
	artifactID := stringValue(params, "artifact_id", "")
	if strings.TrimSpace(artifactID) != "" {
		artifact, err := s.findArtifactForTask(taskID, artifactID)
		if err != nil {
			return nil, err
		}
		result := buildDeliveryOpenResult(cloneMap(artifact), nil, taskID)
		result["artifact"] = protocolArtifactMap(artifact)
		return result, nil
	}
	task, ok := s.runEngine.GetTask(taskID)
	if !ok {
		task, ok = s.taskDetailFromStorage(taskID)
	}
	if !ok {
		return nil, ErrTaskNotFound
	}
	return buildDeliveryOpenResult(nil, cloneMap(task.DeliveryResult), taskID), nil
}

func inferArtifactDeliveryType(artifact map[string]any) string {
	if deliveryType := stringValue(artifact, "delivery_type", ""); deliveryType != "" {
		return deliveryType
	}
	if path := stringValue(artifact, "path", ""); path != "" {
		return "open_file"
	}
	return "task_detail"
}

// protocolTaskStepList guarantees that task detail timeline stays an array.
func protocolTaskStepList(steps []map[string]any) []map[string]any {
	if len(steps) == 0 {
		return []map[string]any{}
	}
	return cloneMapSlice(steps)
}

// protocolArtifactList trims artifact items to the declared protocol fields and
// keeps the collection non-null for RPC consumers.
func protocolArtifactList(artifacts []map[string]any) []map[string]any {
	if len(artifacts) == 0 {
		return []map[string]any{}
	}
	result := make([]map[string]any, 0, len(artifacts))
	for _, artifact := range artifacts {
		normalized := protocolArtifactMap(artifact)
		if normalized == nil {
			continue
		}
		result = append(result, normalized)
	}
	if len(result) == 0 {
		return []map[string]any{}
	}
	return result
}

// protocolArtifactMap trims one artifact to the formal Artifact contract.
func protocolArtifactMap(artifact map[string]any) map[string]any {
	if len(artifact) == 0 {
		return nil
	}
	return map[string]any{
		"artifact_id":   stringValue(artifact, "artifact_id", ""),
		"task_id":       stringValue(artifact, "task_id", ""),
		"artifact_type": stringValue(artifact, "artifact_type", ""),
		"title":         stringValue(artifact, "title", ""),
		"path":          stringValue(artifact, "path", ""),
		"mime_type":     stringValue(artifact, "mime_type", ""),
	}
}

// protocolMirrorReferenceList trims mirror references to the declared protocol
// fields and keeps the collection non-null for RPC consumers.
func protocolMirrorReferenceList(references []map[string]any) []map[string]any {
	if len(references) == 0 {
		return []map[string]any{}
	}
	result := make([]map[string]any, 0, len(references))
	for _, reference := range references {
		if len(reference) == 0 {
			continue
		}
		result = append(result, map[string]any{
			"memory_id": stringValue(reference, "memory_id", ""),
			"reason":    stringValue(reference, "reason", ""),
			"summary":   stringValue(reference, "summary", ""),
		})
	}
	if len(result) == 0 {
		return []map[string]any{}
	}
	return result
}

func buildDeliveryOpenResult(artifact map[string]any, deliveryResult map[string]any, taskID string) map[string]any {
	resolvedDelivery := normalizeDeliveryOpenResult(artifact, deliveryResult, taskID)
	return map[string]any{
		"delivery_result":  resolvedDelivery,
		"open_action":      stringValue(resolvedDelivery, "type", "task_detail"),
		"resolved_payload": cloneMap(mapValue(resolvedDelivery, "payload")),
	}
}

func normalizeDeliveryOpenResult(artifact map[string]any, deliveryResult map[string]any, taskID string) map[string]any {
	if len(deliveryResult) == 0 {
		payload := cloneMap(mapValue(artifact, "delivery_payload"))
		if payload == nil {
			payload = map[string]any{}
		}
		pathValue := firstNonEmptyString(stringValue(artifact, "path", ""), stringValue(payload, "path", ""))
		if pathValue != "" {
			payload["path"] = pathValue
		}
		if payload["task_id"] == nil {
			payload["task_id"] = taskID
		}
		return map[string]any{
			"type":         firstNonEmptyString(stringValue(artifact, "delivery_type", ""), inferArtifactDeliveryType(artifact)),
			"title":        stringValue(artifact, "title", ""),
			"payload":      payload,
			"preview_text": stringValue(artifact, "title", ""),
		}
	}
	resolved := cloneMap(deliveryResult)
	payload := cloneMap(mapValue(resolved, "payload"))
	if payload == nil {
		payload = map[string]any{}
	}
	if payload["task_id"] == nil {
		payload["task_id"] = taskID
	}
	resolved["payload"] = payload
	if stringValue(resolved, "type", "") == "" {
		resolved["type"] = "task_detail"
	}
	if stringValue(resolved, "title", "") == "" {
		resolved["title"] = "任务交付结果"
	}
	if stringValue(resolved, "preview_text", "") == "" {
		resolved["preview_text"] = stringValue(resolved, "title", "")
	}
	return resolved
}

// TaskControl 处理当前模块的相关逻辑。

// TaskControl 处理 agent.task.control，把用户控制动作转换成状态机操作。
func (s *Service) TaskControl(params map[string]any) (map[string]any, error) {
	taskID := stringValue(params, "task_id", "")
	if strings.TrimSpace(taskID) == "" {
		return nil, errors.New("task_id is required")
	}
	action := stringValue(params, "action", "")
	if strings.TrimSpace(action) == "" {
		return nil, errors.New("action is required")
	}
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
	if taskIsTerminal(updatedTask.Status) {
		if queueErr := s.drainSessionQueue(updatedTask.SessionID); queueErr != nil {
			return nil, queueErr
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
	if result.SourceSynced {
		if err := s.runEngine.SyncNotepadItems(result.NotepadItems); err != nil {
			return nil, err
		}
	}

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
	if !boolValue(params, "confirmed", false) {
		return nil, fmt.Errorf("confirmed must be true to convert notepad item")
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
	runtimeUnfinishedTasks, _ := s.runEngine.ListTasks("unfinished", "updated_at", "desc", 0, 0)
	runtimeFinishedTasks, _ := s.runEngine.ListTasks("finished", "finished_at", "desc", 0, 0)
	pendingApprovals, pendingTotal := s.runEngine.PendingApprovalRequests(20, 0)

	// Always merge storage data with runtime data for complete dashboard view
	allPersistedTasks := s.loadAllTasksFromStorage()
	unfinishedTasks := mergeTaskLists(runtimeUnfinishedTasks, filterAndSortTasks(allPersistedTasks, "unfinished", "updated_at", "desc"))
	finishedTasks := mergeTaskLists(runtimeFinishedTasks, filterAndSortTasks(allPersistedTasks, "finished", "finished_at", "desc"))
	needStorageFallback := len(runtimeUnfinishedTasks) == 0 && len(runtimeFinishedTasks) == 0

	if pendingTotal == 0 {
		pendingTotal = countPendingApprovalTasks(unfinishedTasks)
	}
	if len(pendingApprovals) == 0 && pendingTotal > 0 {
		pendingApprovals = pendingApprovalsFromTasks(unfinishedTasks)
	}
	focusMode := boolValue(params, "focus_mode", false)
	requestedIncludes := stringSliceValue(params["include"])
	includeAll := len(requestedIncludes) == 0
	includeSet := make(map[string]struct{}, len(requestedIncludes))
	for _, value := range requestedIncludes {
		includeSet[value] = struct{}{}
	}

	focusTask, hasFocusTask := focusTaskForOverview(unfinishedTasks, finishedTasks)
	var focusSummary map[string]any
	if hasFocusTask && shouldIncludeOverviewField(includeAll, includeSet, "focus_summary") {
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
	quickActions := []string(nil)
	if shouldIncludeOverviewField(includeAll, includeSet, "quick_actions") {
		quickActions = buildDashboardQuickActions(hasFocusTask, pendingTotal, len(finishedTasks))
		if focusMode {
			quickActions = filterDashboardQuickActionsForFocus(quickActions)
		}
	}
	var globalState map[string]any
	if shouldIncludeOverviewField(includeAll, includeSet, "global_state") {
		// Only include global_state when runtime engine has active state
		// to avoid contradictory data in cold-start fallback scenarios
		if !needStorageFallback {
			globalState = s.Snapshot()
		}
	}
	highValueSignal := []string(nil)
	if shouldIncludeOverviewField(includeAll, includeSet, "high_value_signal") {
		highValueSignal = buildDashboardSignalsWithAudit(unfinishedTasks, finishedTasks, pendingApprovals, latestAudit)
		if contextValue := mapValue(params, "context"); len(contextValue) > 0 {
			highValueSignal = append(highValueSignal, perception.BehaviorSignals(perception.CaptureContextSignals("dashboard", "hover", contextValue))...)
			highValueSignal = dedupeStringSlice(highValueSignal)
		}
		if focusMode {
			highValueSignal = filterDashboardSignalsForFocus(highValueSignal)
		}
	}
	var trustSummary map[string]any
	if shouldIncludeOverviewField(includeAll, includeSet, "trust_summary") {
		trustSummary = map[string]any{
			"risk_level":             aggregateRiskLevel(allTasks, pendingApprovals, s.risk.DefaultLevel()),
			"pending_authorizations": pendingTotal,
			"has_restore_point":      hasRestorePoint,
			"workspace_path":         workspacePathFromSettings(s.runEngine.Settings()),
		}
	}

	overview := map[string]any{}
	if shouldIncludeOverviewField(includeAll, includeSet, "focus_summary") {
		overview["focus_summary"] = focusSummary
	} else {
		overview["focus_summary"] = nil
	}
	if shouldIncludeOverviewField(includeAll, includeSet, "trust_summary") {
		overview["trust_summary"] = trustSummary
	} else {
		overview["trust_summary"] = nil
	}
	if shouldIncludeOverviewField(includeAll, includeSet, "quick_actions") {
		overview["quick_actions"] = quickActions
	} else {
		overview["quick_actions"] = []string{}
	}
	if shouldIncludeOverviewField(includeAll, includeSet, "global_state") {
		overview["global_state"] = globalState
	} else {
		overview["global_state"] = map[string]any{}
	}
	if shouldIncludeOverviewField(includeAll, includeSet, "high_value_signal") {
		overview["high_value_signal"] = highValueSignal
	} else {
		overview["high_value_signal"] = []string{}
	}

	return map[string]any{"overview": overview}, nil
}

func pendingApprovalsFromTasks(tasks []runengine.TaskRecord) []map[string]any {
	items := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		if task.Status != "waiting_auth" || len(task.ApprovalRequest) == 0 {
			continue
		}
		item := cloneMap(task.ApprovalRequest)
		if stringValue(item, "task_id", "") == "" {
			item["task_id"] = task.TaskID
		}
		if stringValue(item, "risk_level", "") == "" {
			item["risk_level"] = task.RiskLevel
		}
		items = append(items, item)
	}
	return items
}

// DashboardModuleGet 处理当前模块的相关逻辑。

// DashboardModuleGet 处理 agent.dashboard.module.get。
func (s *Service) DashboardModuleGet(params map[string]any) (map[string]any, error) {
	module := stringValue(params, "module", "mirror")
	tab := stringValue(params, "tab", "daily_summary")
	finishedTasks, _ := s.runEngine.ListTasks("finished", "finished_at", "desc", 0, 0)
	unfinishedTasks, _ := s.runEngine.ListTasks("unfinished", "updated_at", "desc", 0, 0)
	if len(finishedTasks) == 0 {
		if persistedTasks, _, ok := s.listTasksFromStorage("finished", "finished_at", "desc", 0, 0); ok {
			finishedTasks = persistedTasks
		}
	}
	if len(unfinishedTasks) == 0 {
		if persistedTasks, _, ok := s.listTasksFromStorage("unfinished", "updated_at", "desc", 0, 0); ok {
			unfinishedTasks = persistedTasks
		}
	}
	_, pendingTotal := s.runEngine.PendingApprovalRequests(20, 0)
	if pendingTotal == 0 {
		pendingTotal = countPendingApprovalTasks(unfinishedTasks)
	}
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
	if len(finishedTasks) == 0 {
		if persistedTasks, _, ok := s.listTasksFromStorage("finished", "finished_at", "desc", 0, 0); ok {
			finishedTasks = persistedTasks
		}
	}
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
	if len(unfinishedTasks) == 0 {
		if persistedTasks, _, ok := s.listTasksFromStorage("unfinished", "updated_at", "desc", 0, 0); ok {
			unfinishedTasks = persistedTasks
		}
	}
	if len(finishedTasks) == 0 {
		if persistedTasks, _, ok := s.listTasksFromStorage("finished", "finished_at", "desc", 0, 0); ok {
			finishedTasks = persistedTasks
		}
	}
	if pendingTotal == 0 {
		pendingTotal = countPendingApprovalTasks(unfinishedTasks)
	}
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

	// Fallback to storage if runtime has no pending approvals
	if total == 0 {
		unfinishedTasks, totalTasks, ok := s.listTasksFromStorage("unfinished", "updated_at", "desc", 0, 0)
		if ok && totalTasks > 0 {
			allPendingApprovals := pendingApprovalsFromTasks(unfinishedTasks)
			total = len(allPendingApprovals)
			if total > 0 {
				start := offset
				if start >= total {
					start = total
				}
				end := start + limit
				if end > total {
					end = total
				}
				items = allPendingApprovals[start:end]
			}
		}
	}

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
	assessment := restoreApplyAssessment(point)
	pendingExecution := buildRestoreApplyPendingExecution(point, assessment)
	approvalRequest := buildApprovalRequest(task.TaskID, task.Intent, assessment)
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "恢复点回滚属于高风险操作，请先确认授权。", time.Now().Format(dateTimeLayout))
	updatedTask, ok := s.runEngine.MarkWaitingApprovalWithPlan(task.TaskID, approvalRequest, pendingExecution, bubble)
	if !ok {
		return nil, ErrTaskNotFound
	}
	return map[string]any{
		"applied":        false,
		"task":           taskMap(updatedTask),
		"recovery_point": recoveryPoint,
		"audit_record":   nil,
		"bubble_message": bubble,
	}, nil
}

func (s *Service) applyRestoreAfterApproval(task runengine.TaskRecord, point checkpoint.RecoveryPoint) (runengine.TaskRecord, map[string]any, map[string]any, error) {
	recoveryPoint := recoveryPointMap(point)
	applied := false
	securityStatus := "recovered"
	finalStatus := "completed"
	bubbleText := fmt.Sprintf("已根据恢复点 %s 恢复 %d 个对象。", point.RecoveryPointID, len(point.Objects))
	if s.executor == nil {
		securityStatus = "execution_error"
		finalStatus = "failed"
		bubbleText = "恢复失败：执行后端不可用。"
	} else if applyResult, err := s.executor.ApplyRecoveryPoint(context.Background(), point); err != nil {
		securityStatus = "execution_error"
		finalStatus = "failed"
		bubbleText = "恢复失败：恢复点内容不可用或恢复执行失败。"
	} else {
		applied = true
		if len(applyResult.RestoredObjects) > 0 {
			bubbleText = fmt.Sprintf("已根据恢复点 %s 恢复 %d 个对象。", point.RecoveryPointID, len(applyResult.RestoredObjects))
		}
	}

	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", bubbleText, time.Now().Format(dateTimeLayout))
	updatedTask, ok := s.runEngine.ApplyRecoveryOutcome(task.TaskID, finalStatus, securityStatus, recoveryPoint, bubble)
	if !ok {
		return runengine.TaskRecord{}, nil, nil, ErrTaskNotFound
	}
	auditRecord := s.writeRestoreAuditRecord(updatedTask.TaskID, point, applied, bubbleText)
	updatedTask = s.appendAuditData(updatedTask, compactAuditRecords(auditRecord), nil)
	return updatedTask, bubble, map[string]any{
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
	operationName := stringValue(pendingExecution, "operation_name", "")
	if decision == "deny_once" {
		bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "已拒绝本次操作，任务已取消。", task.UpdatedAt.Format(dateTimeLayout))
		updatedTask, ok := s.runEngine.DenyAfterApproval(task.TaskID, authorizationRecord, impactScope, bubble)
		if !ok {
			return nil, ErrTaskNotFound
		}
		updatedTask = s.appendAuditData(updatedTask, compactAuditRecords(s.audit.BuildAuthorizationAudit(updatedTask.TaskID, updatedTask.RunID, decision, impactScope)), nil)
		if queueErr := s.drainSessionQueue(updatedTask.SessionID); queueErr != nil {
			return nil, queueErr
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
	processingTask = s.appendAuditData(processingTask, compactAuditRecords(s.audit.BuildAuthorizationAudit(processingTask.TaskID, processingTask.RunID, decision, impactScope)), nil)
	if operationName == "restore_apply" {
		recoveryPointID := stringValue(pendingExecution, "recovery_point_id", "")
		point, err := s.findRecoveryPointFromStorage(task.TaskID, recoveryPointID)
		if err != nil {
			return nil, err
		}
		updatedTask, _, response, err := s.applyRestoreAfterApproval(processingTask, point)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"authorization_record": authorizationRecord,
			"task":                 taskMap(updatedTask),
			"bubble_message":       response["bubble_message"],
			"impact_scope":         impactScope,
			"delivery_result":      nil,
			"recovery_point":       response["recovery_point"],
			"audit_record":         response["audit_record"],
			"applied":              response["applied"],
		}, nil
	}

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
	if updatedTask.Status == "completed" {
		updatedTask, _ = s.runEngine.ResolveAuthorization(task.TaskID, authorizationRecord, impactScope)
	}
	if updatedTask.Status == "failed" {
		deliveryResult = nil
	}
	if taskIsTerminal(updatedTask.Status) {
		if queueErr := s.drainSessionQueue(updatedTask.SessionID); queueErr != nil {
			return nil, queueErr
		}
	}

	response := map[string]any{
		"authorization_record": authorizationRecord,
		"task":                 taskMap(updatedTask),
		"bubble_message":       resultBubble,
		"impact_scope":         impactScope,
	}
	if len(deliveryResult) > 0 {
		response["delivery_result"] = deliveryResult
	} else {
		response["delivery_result"] = nil
	}
	return response, nil
}

// SettingsGet 设置tingsGet。

// SettingsGet 处理 agent.settings.get。
func (s *Service) SettingsGet(params map[string]any) (map[string]any, error) {
	settings := s.runEngine.Settings()
	settingsWithSecrets, err := s.attachSensitiveSettingAvailability(settings)
	if err != nil {
		return nil, err
	}
	settings = settingsWithSecrets
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
	if dataLog := mapValue(params, "data_log"); len(dataLog) > 0 {
		if apiKey := stringValue(dataLog, "api_key", ""); apiKey != "" {
			provider := s.providerForSettingsUpdate(dataLog)
			if err := s.persistModelSecret(provider, apiKey); err != nil {
				return nil, err
			}
			delete(dataLog, "api_key")
			params["data_log"] = dataLog
		}
	}
	effectiveSettings, updatedKeys, applyMode, needRestart := s.runEngine.UpdateSettings(params)
	effectiveSettingsWithSecrets, err := s.attachSensitiveSettingAvailability(effectiveSettings)
	if err != nil {
		return nil, err
	}
	effectiveSettings = effectiveSettingsWithSecrets
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

func (s *Service) queueTaskIfSessionBusy(task runengine.TaskRecord) (runengine.TaskRecord, map[string]any, bool, error) {
	activeTask, ok := s.runEngine.ActiveSessionTask(task.SessionID, task.TaskID)
	if !ok {
		return runengine.TaskRecord{}, nil, false, nil
	}

	bubble := s.delivery.BuildBubbleMessage(
		task.TaskID,
		"status",
		fmt.Sprintf("当前会话已有任务 %s 正在执行，本任务已排队等待。", truncateText(activeTask.Title, 24)),
		task.UpdatedAt.Format(dateTimeLayout),
	)
	queuedTask, changed := s.runEngine.QueueTaskForSession(task.TaskID, activeTask.TaskID, bubble)
	if !changed {
		return runengine.TaskRecord{}, nil, false, ErrTaskNotFound
	}
	return queuedTask, bubble, true, nil
}

func (s *Service) drainSessionQueue(sessionID string) error {
	for {
		nextTask, ok := s.runEngine.NextQueuedTaskForSession(sessionID)
		if !ok {
			return nil
		}
		if activeTask, busy := s.runEngine.ActiveSessionTask(sessionID, nextTask.TaskID); busy && activeTask.TaskID != "" {
			return nil
		}

		bubble := s.delivery.BuildBubbleMessage(
			nextTask.TaskID,
			"status",
			"前序任务已完成，当前会话中的下一个任务开始执行。",
			nextTask.UpdatedAt.Format(dateTimeLayout),
		)
		resumedTask, changed := s.runEngine.ResumeQueuedTask(nextTask.TaskID, executionStepName(nextTask.Intent), bubble)
		if !changed {
			return ErrTaskNotFound
		}

		governedTask, _, handled, governanceErr := s.handleTaskGovernanceDecision(resumedTask, resumedTask.Intent)
		if governanceErr != nil {
			return governanceErr
		}
		if handled {
			if taskIsTerminal(governedTask.Status) {
				continue
			}
			return nil
		}

		updatedTask, _, _, _, err := s.executeTask(governedTask, snapshotFromTask(governedTask), governedTask.Intent)
		if err != nil {
			return err
		}
		if !taskIsTerminal(updatedTask.Status) {
			return nil
		}
	}
}

func taskIsTerminal(status string) bool {
	switch status {
	case "completed", "cancelled", "ended_unfinished", "failed":
		return true
	default:
		return false
	}
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

func (s *Service) listTasksFromStorage(group, sortBy, sortOrder string, limit, offset int) ([]runengine.TaskRecord, int, bool) {
	if s.storage == nil {
		return nil, 0, false
	}
	records, err := s.storage.TaskRunStore().LoadTaskRuns(context.Background())
	if err != nil || len(records) == 0 {
		return nil, 0, false
	}
	tasks := make([]runengine.TaskRecord, 0, len(records))
	for _, record := range records {
		task := taskRecordFromStorage(record)
		if !matchesTaskGroup(task, group) {
			continue
		}
		tasks = append(tasks, task)
	}
	runengineSortTaskRecords(tasks, sortBy, sortOrder)
	total := len(tasks)
	if offset >= total {
		return []runengine.TaskRecord{}, total, true
	}
	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}
	return tasks[offset:end], total, true
}

func (s *Service) loadAllTasksFromStorage() []runengine.TaskRecord {
	if s.storage == nil {
		return nil
	}
	records, err := s.storage.TaskRunStore().LoadTaskRuns(context.Background())
	if err != nil || len(records) == 0 {
		return nil
	}
	tasks := make([]runengine.TaskRecord, 0, len(records))
	for _, record := range records {
		tasks = append(tasks, taskRecordFromStorage(record))
	}
	return tasks
}

func filterAndSortTasks(tasks []runengine.TaskRecord, group, sortBy, sortOrder string) []runengine.TaskRecord {
	if len(tasks) == 0 {
		return nil
	}
	filtered := make([]runengine.TaskRecord, 0, len(tasks))
	for _, task := range tasks {
		if matchesTaskGroup(task, group) {
			filtered = append(filtered, task)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	runengineSortTaskRecords(filtered, sortBy, sortOrder)
	return filtered
}

func mergeTaskLists(runtimeTasks, storageTasks []runengine.TaskRecord) []runengine.TaskRecord {
	if len(runtimeTasks) == 0 {
		return storageTasks
	}
	if len(storageTasks) == 0 {
		return runtimeTasks
	}
	// Build map of runtime task IDs for deduplication
	runtimeIDs := make(map[string]struct{}, len(runtimeTasks))
	for _, task := range runtimeTasks {
		runtimeIDs[task.TaskID] = struct{}{}
	}
	// Merge: runtime tasks take precedence, add storage tasks not in runtime
	merged := make([]runengine.TaskRecord, 0, len(runtimeTasks)+len(storageTasks))
	merged = append(merged, runtimeTasks...)
	for _, task := range storageTasks {
		if _, exists := runtimeIDs[task.TaskID]; !exists {
			merged = append(merged, task)
		}
	}
	return merged
}

func (s *Service) taskDetailFromStorage(taskID string) (runengine.TaskRecord, bool) {
	if s.storage == nil || strings.TrimSpace(taskID) == "" {
		return runengine.TaskRecord{}, false
	}
	records, err := s.storage.TaskRunStore().LoadTaskRuns(context.Background())
	if err != nil {
		return runengine.TaskRecord{}, false
	}
	for _, record := range records {
		if record.TaskID == taskID {
			return taskRecordFromStorage(record), true
		}
	}
	return runengine.TaskRecord{}, false
}

func (s *Service) attachSensitiveSettingAvailability(settings map[string]any) (map[string]any, error) {
	cloned := cloneMap(settings)
	if cloned == nil {
		cloned = map[string]any{}
	}
	dataLog := cloneMap(mapValue(cloned, "data_log"))
	if dataLog == nil {
		dataLog = map[string]any{}
	}
	provider, configured, err := s.modelSecretConfigured(providerFromSettings(dataLog, s.defaultSettingsProvider()))
	if err != nil {
		return nil, err
	}
	if stringValue(dataLog, "provider", "") == "" && provider != "" {
		dataLog["provider"] = provider
	}
	dataLog["provider_api_key_configured"] = configured
	cloned["data_log"] = dataLog
	return cloned, nil
}

func (s *Service) modelSecretConfigured(provider string) (string, bool, error) {
	resolvedProvider := firstNonEmptyString(strings.TrimSpace(provider), s.defaultSettingsProvider())
	if s.storage == nil || s.storage.SecretStore() == nil || resolvedProvider == "" {
		return resolvedProvider, false, nil
	}
	_, err := s.storage.SecretStore().GetSecret(context.Background(), "model", resolvedProvider+"_api_key")
	if err == nil {
		return resolvedProvider, true, nil
	}
	if errors.Is(err, storage.ErrSecretNotFound) {
		return resolvedProvider, false, nil
	}
	if errors.Is(err, storage.ErrSecretStoreAccessFailed) {
		return resolvedProvider, false, ErrStrongholdAccessFailed
	}
	return resolvedProvider, false, err
}

func (s *Service) persistModelSecret(provider, apiKey string) error {
	resolvedProvider := firstNonEmptyString(strings.TrimSpace(provider), s.defaultSettingsProvider())
	if s.storage == nil || s.storage.SecretStore() == nil || resolvedProvider == "" {
		return ErrStrongholdAccessFailed
	}
	if err := s.storage.SecretStore().PutSecret(context.Background(), storage.SecretRecord{
		Namespace: "model",
		Key:       resolvedProvider + "_api_key",
		Value:     strings.TrimSpace(apiKey),
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		if errors.Is(err, storage.ErrSecretStoreAccessFailed) {
			return ErrStrongholdAccessFailed
		}
		return err
	}
	return nil
}

func (s *Service) providerForSettingsUpdate(dataLog map[string]any) string {
	return providerFromSettings(dataLog, s.defaultSettingsProvider())
}

func (s *Service) defaultSettingsProvider() string {
	if s.model == nil {
		return ""
	}
	return strings.TrimSpace(s.model.Provider())
}

func providerFromSettings(dataLog map[string]any, fallback string) string {
	provider := firstNonEmptyString(stringValue(dataLog, "provider", ""), fallback)
	if provider == "openai" {
		return model.OpenAIResponsesProvider
	}
	return provider
}

func matchesTaskGroup(task runengine.TaskRecord, group string) bool {
	switch group {
	case "finished":
		return isFinishedTaskStatus(task.Status)
	default:
		return !isFinishedTaskStatus(task.Status)
	}
}

func isFinishedTaskStatus(status string) bool {
	switch status {
	case "completed", "cancelled", "ended_unfinished", "failed":
		return true
	default:
		return false
	}
}

func runengineSortTaskRecords(tasks []runengine.TaskRecord, sortBy, sortOrder string) {
	switch sortBy {
	case "started_at", "finished_at", "updated_at":
	default:
		sortBy = "updated_at"
	}
	if sortOrder != "asc" {
		sortOrder = "desc"
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		left := taskSortTime(tasks[i], sortBy)
		right := taskSortTime(tasks[j], sortBy)
		if left.Equal(right) {
			leftUpdated := tasks[i].UpdatedAt
			rightUpdated := tasks[j].UpdatedAt
			if leftUpdated.Equal(rightUpdated) {
				if sortOrder == "asc" {
					return tasks[i].TaskID < tasks[j].TaskID
				}
				return tasks[i].TaskID > tasks[j].TaskID
			}
			if sortOrder == "asc" {
				return leftUpdated.Before(rightUpdated)
			}
			return leftUpdated.After(rightUpdated)
		}
		if sortOrder == "asc" {
			return left.Before(right)
		}
		return left.After(right)
	})
}

func countPendingApprovalTasks(tasks []runengine.TaskRecord) int {
	count := 0
	for _, task := range tasks {
		if task.Status == "waiting_auth" && len(task.ApprovalRequest) != 0 {
			count++
		}
	}
	return count
}

func taskSortTime(task runengine.TaskRecord, sortBy string) time.Time {
	switch sortBy {
	case "started_at":
		return task.StartedAt
	case "finished_at":
		if task.FinishedAt != nil {
			return *task.FinishedAt
		}
		return time.Time{}
	default:
		return task.UpdatedAt
	}
}

func taskRecordFromStorage(record storage.TaskRunRecord) runengine.TaskRecord {
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
		FinishedAt:        cloneTimePointer(record.FinishedAt),
		Timeline:          timelineFromStorage(record.Timeline),
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
		LatestEvent:       cloneMap(record.LatestEvent),
		LatestToolCall:    cloneMap(record.LatestToolCall),
		CurrentStepStatus: record.CurrentStepStatus,
	}
}

func timelineFromStorage(timeline []storage.TaskStepSnapshot) []runengine.TaskStepRecord {
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

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

// taskStatusForSuggestion 处理当前模块的相关逻辑。
func taskStatusForSuggestion(requiresConfirm bool) string {
	if requiresConfirm {
		return "confirming_intent"
	}
	return "processing"
}

// currentStepForSuggestion 处理当前模块的相关逻辑。
func currentStepForSuggestion(requiresConfirm bool, taskIntent map[string]any) string {
	if requiresConfirm {
		return "intent_confirmation"
	}
	if stringValue(taskIntent, "name", "") == "agent_loop" {
		return "agent_loop"
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
		if !suggestion.IntentConfirmed {
			return "我还不确定你想如何处理这段内容，请确认目标。"
		}
		return confirmIntentText(suggestion.Intent)
	}
	return suggestion.ResultBubbleText
}

// bubbleTextForStart 处理当前模块的相关逻辑。
func bubbleTextForStart(suggestion intent.Suggestion) string {
	if suggestion.RequiresConfirm {
		if !suggestion.IntentConfirmed {
			return "我还不确定你想如何处理当前对象，请先确认。"
		}
		return confirmIntentText(suggestion.Intent)
	}
	return suggestion.ResultBubbleText
}

func confirmIntentText(taskIntent map[string]any) string {
	switch stringValue(taskIntent, "name", "") {
	case "translate":
		return "你是想翻译这段内容吗？"
	case "rewrite":
		return "你是想改写这段内容吗？"
	case "explain":
		return "你是想解释这段内容吗？"
	case "summarize":
		return "你是想总结这段内容吗？"
	case "write_file":
		return "你是想把结果整理成文档吗？"
	default:
		return "请确认你希望我如何处理当前内容。"
	}
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

func shouldIncludeOverviewField(includeAll bool, includeSet map[string]struct{}, field string) bool {
	if includeAll {
		return true
	}
	_, ok := includeSet[field]
	return ok
}

func filterDashboardQuickActionsForFocus(actions []string) []string {
	filtered := make([]string, 0, len(actions))
	for _, action := range actions {
		if action == "查看最近结果" {
			continue
		}
		filtered = append(filtered, action)
	}
	if len(filtered) == 0 {
		return []string{"打开任务详情"}
	}
	return filtered
}

func filterDashboardSignalsForFocus(signals []string) []string {
	if len(signals) <= 2 {
		return signals
	}
	return append([]string(nil), signals[:2]...)
}

func dedupeStringSlice(values []string) []string {
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

func activeTaskDetailApprovalRequest(task runengine.TaskRecord) map[string]any {
	if task.Status != "waiting_auth" || len(task.ApprovalRequest) == 0 {
		return nil
	}
	return normalizeTaskDetailApprovalRequest(task.TaskID, task.RiskLevel, task.ApprovalRequest)
}

func (s *Service) normalizeTaskDetailRestorePoint(taskID string, securitySummary map[string]any) map[string]any {
	if latestRestorePoint := normalizeTaskDetailRecoveryPoint(taskID, latestRestorePointFromSummary(securitySummary)); latestRestorePoint != nil {
		return latestRestorePoint
	}
	if restorePoint := s.latestRestorePointFromStorage(taskID); restorePoint != nil {
		return restorePoint
	}
	return nil
}

func normalizeTaskDetailApprovalRequest(taskID, fallbackRiskLevel string, approvalRequest map[string]any) map[string]any {
	if len(approvalRequest) == 0 {
		return nil
	}

	approvalID := strings.TrimSpace(stringValue(approvalRequest, "approval_id", ""))
	approvalTaskID := strings.TrimSpace(stringValue(approvalRequest, "task_id", ""))
	operationName := strings.TrimSpace(stringValue(approvalRequest, "operation_name", ""))
	targetObject := strings.TrimSpace(stringValue(approvalRequest, "target_object", ""))
	reason := strings.TrimSpace(stringValue(approvalRequest, "reason", ""))
	status := strings.TrimSpace(stringValue(approvalRequest, "status", ""))
	createdAt := strings.TrimSpace(stringValue(approvalRequest, "created_at", ""))
	riskLevel := strings.TrimSpace(stringValue(approvalRequest, "risk_level", ""))
	if riskLevel == "" {
		riskLevel = strings.TrimSpace(fallbackRiskLevel)
	}

	if approvalID == "" || approvalTaskID != taskID || operationName == "" || targetObject == "" || reason == "" || createdAt == "" {
		return nil
	}
	if status != "pending" || !isSupportedRiskLevel(riskLevel) {
		return nil
	}

	return map[string]any{
		"approval_id":    approvalID,
		"task_id":        approvalTaskID,
		"operation_name": operationName,
		"risk_level":     riskLevel,
		"target_object":  targetObject,
		"reason":         reason,
		"status":         status,
		"created_at":     createdAt,
	}
}

func normalizeTaskDetailRecoveryPoint(taskID string, recoveryPoint map[string]any) map[string]any {
	if len(recoveryPoint) == 0 {
		return nil
	}

	recoveryPointID := strings.TrimSpace(stringValue(recoveryPoint, "recovery_point_id", ""))
	recoveryTaskID := strings.TrimSpace(stringValue(recoveryPoint, "task_id", ""))
	summary := strings.TrimSpace(stringValue(recoveryPoint, "summary", ""))
	createdAt := strings.TrimSpace(stringValue(recoveryPoint, "created_at", ""))
	objects, ok := normalizeStringSlice(recoveryPoint["objects"])
	if !ok {
		return nil
	}

	if recoveryPointID == "" || recoveryTaskID != taskID || summary == "" || createdAt == "" {
		return nil
	}

	return map[string]any{
		"recovery_point_id": recoveryPointID,
		"task_id":           recoveryTaskID,
		"summary":           summary,
		"created_at":        createdAt,
		"objects":           objects,
	}
}

func isSupportedRiskLevel(riskLevel string) bool {
	switch riskLevel {
	case "green", "yellow", "red":
		return true
	default:
		return false
	}
}

func normalizeStringSlice(value any) ([]string, bool) {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...), true
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok {
				return nil, false
			}
			items = append(items, text)
		}
		return items, true
	default:
		return nil, false
	}
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

func restoreApplyAssessment(point checkpoint.RecoveryPoint) execution.GovernanceAssessment {
	impactScope := restoreImpactScope(point)
	return execution.GovernanceAssessment{
		OperationName:      "restore_apply",
		TargetObject:       firstNonEmptyString(firstImpactFile(impactScope), firstNonEmptyString(strings.Join(point.Objects, ", "), "workspace")),
		RiskLevel:          "red",
		ApprovalRequired:   true,
		CheckpointRequired: false,
		Reason:             "policy_requires_authorization",
		ImpactScope:        impactScope,
	}
}

func buildRestoreApplyPendingExecution(point checkpoint.RecoveryPoint, assessment execution.GovernanceAssessment) map[string]any {
	return map[string]any{
		"operation_name":      assessment.OperationName,
		"target_object":       assessment.TargetObject,
		"risk_level":          assessment.RiskLevel,
		"risk_reason":         assessment.Reason,
		"impact_scope":        cloneMap(assessment.ImpactScope),
		"recovery_point_id":   point.RecoveryPointID,
		"checkpoint_required": assessment.CheckpointRequired,
	}
}

func restoreImpactScope(point checkpoint.RecoveryPoint) map[string]any {
	files := append([]string(nil), point.Objects...)
	outOfWorkspace := false
	for _, filePath := range files {
		normalized := strings.TrimSpace(strings.ReplaceAll(filePath, "\\", "/"))
		if normalized == "" {
			continue
		}
		if !strings.HasPrefix(normalized, "workspace/") && normalized != "workspace" {
			outOfWorkspace = true
			break
		}
	}
	return map[string]any{
		"files":                    files,
		"webpages":                 []string{},
		"apps":                     []string{},
		"out_of_workspace":         outOfWorkspace,
		"overwrite_or_delete_risk": true,
	}
}

func firstImpactFile(impactScope map[string]any) string {
	if len(impactScope) == 0 {
		return ""
	}
	files, ok := impactScope["files"].([]string)
	if !ok || len(files) == 0 {
		return ""
	}
	return files[0]
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
		for _, artifact := range task.Artifacts {
			if outputPath := stringValue(artifact, "path", ""); outputPath != "" {
				return outputPath
			}
		}
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

func (s *Service) syncTaskReadMirrorReferences(taskID string, references []map[string]any, err error) {
	if err == nil {
		_, _ = s.runEngine.SetMirrorReferences(taskID, cloneMapSlice(references))
		return
	}
	if errors.Is(err, memory.ErrStoreNotConfigured) {
		s.refreshMirrorReferences(taskID)
	}
}

func (s *Service) syncTaskWriteMirrorReferences(taskID string, references []map[string]any, err error) {
	if err == nil {
		_, _ = s.runEngine.SetMirrorReferences(taskID, mergeMirrorReferences(currentTaskMirrorReferences(s.runEngine, taskID), references))
		return
	}
	if errors.Is(err, memory.ErrStoreNotConfigured) {
		s.refreshMirrorReferences(taskID)
	}
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

func currentTaskMirrorReferences(engine *runengine.Engine, taskID string) []map[string]any {
	if engine == nil {
		return nil
	}
	task, ok := engine.GetTask(taskID)
	if !ok {
		return nil
	}
	return cloneMapSlice(task.MirrorReferences)
}

func mergeMirrorReferences(referenceGroups ...[]map[string]any) []map[string]any {
	merged := make([]map[string]any, 0)
	seen := make(map[string]struct{})
	for _, references := range referenceGroups {
		for _, reference := range references {
			memoryID := stringValue(reference, "memory_id", "")
			if memoryID == "" {
				continue
			}
			if _, ok := seen[memoryID]; ok {
				continue
			}
			seen[memoryID] = struct{}{}
			merged = append(merged, cloneMap(reference))
		}
	}
	return merged
}

func (s *Service) materializeMemoryReadReferences(taskID, runID string, snapshot contextsvc.TaskContextSnapshot) ([]map[string]any, error) {
	if s.memory == nil {
		return nil, memory.ErrStoreNotConfigured
	}
	hits, err := s.memory.Search(context.Background(), memory.RetrievalQuery{
		TaskID: taskID,
		RunID:  runID,
		Query:  memoryQueryFromSnapshot(snapshot),
		Limit:  memory.DefaultSearchLimit,
	})
	if err != nil {
		return nil, err
	}
	persistedHits := cloneRetrievalHitsForTask(taskID, runID, hits)
	if err := s.memory.WriteRetrievalHits(context.Background(), persistedHits); err != nil {
		return nil, err
	}
	return mirrorReferencesFromRetrievalHits(persistedHits), nil
}

func (s *Service) materializeMemoryWriteReferences(taskID, runID string, snapshot contextsvc.TaskContextSnapshot, taskIntent map[string]any, deliveryResult map[string]any) ([]map[string]any, error) {
	if s.memory == nil {
		return nil, memory.ErrStoreNotConfigured
	}
	summary := memory.MemorySummary{
		MemorySummaryID: fmt.Sprintf("memsum_%s_%s", taskID, runID),
		TaskID:          taskID,
		RunID:           runID,
		Summary:         buildMemorySummary(snapshot, taskIntent, deliveryResult),
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
	}
	if err := s.memory.WriteSummary(context.Background(), summary); err != nil {
		return nil, err
	}
	return []map[string]any{mirrorReferenceFromSummary(summary)}, nil
}

func mirrorReferencesFromRetrievalHits(hits []memory.RetrievalHit) []map[string]any {
	if len(hits) == 0 {
		return nil
	}
	references := make([]map[string]any, 0, len(hits))
	for _, hit := range hits {
		reason := "当前任务命中了历史记忆"
		if strings.TrimSpace(hit.Source) != "" {
			reason = fmt.Sprintf("当前任务命中了来源为 %s 的历史记忆", hit.Source)
		}
		references = append(references, map[string]any{
			"memory_id": hit.MemoryID,
			"reason":    reason,
			"summary":   truncateText(hit.Summary, 64),
		})
	}
	return references
}

func cloneRetrievalHitsForTask(taskID, runID string, hits []memory.RetrievalHit) []memory.RetrievalHit {
	if len(hits) == 0 {
		return nil
	}
	cloned := make([]memory.RetrievalHit, 0, len(hits))
	for _, hit := range hits {
		hit.TaskID = taskID
		hit.RunID = runID
		hit.RetrievalHitID = ""
		cloned = append(cloned, hit)
	}
	return cloned
}

func mirrorReferenceFromSummary(summary memory.MemorySummary) map[string]any {
	return map[string]any{
		"memory_id": summary.MemorySummaryID,
		"reason":    "任务完成后写入真实记忆摘要",
		"summary":   truncateText(summary.Summary, 64),
	}
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
	references, err := s.materializeMemoryReadReferences(taskID, runID, snapshot)
	s.syncTaskReadMirrorReferences(taskID, references, err)
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
			"summary":     buildMemorySummary(snapshot, taskIntent, deliveryResult),
			"reason":      "任务完成后准备写入阶段摘要",
			"source_type": snapshot.Trigger,
		},
	}
	_, _ = s.runEngine.SetMemoryPlans(taskID, nil, writePlans)
	references, err := s.materializeMemoryWriteReferences(taskID, runID, snapshot, taskIntent, deliveryResult)
	s.syncTaskWriteMirrorReferences(taskID, references, err)

	storageWritePlan := s.delivery.BuildStorageWritePlan(taskID, deliveryResult)
	artifacts = delivery.EnsureArtifactIdentifiers(taskID, attachDeliveryResultToArtifacts(deliveryResult, artifacts))
	artifactPlans := s.delivery.BuildArtifactPersistPlans(taskID, artifacts)
	_, _ = s.runEngine.SetDeliveryPlans(taskID, storageWritePlan, artifactPlans)
	s.persistArtifacts(taskID, artifactPlans)
}

// buildApprovalRequest 处理当前模块的相关逻辑。

// buildApprovalRequest 构造统一的 approval_request 结构。
func buildApprovalRequest(taskID string, taskIntent map[string]any, assessment execution.GovernanceAssessment) map[string]any {
	arguments := mapValue(taskIntent, "arguments")
	targetObject := firstNonEmptyString(assessment.TargetObject, stringValue(arguments, "target_path", "workspace_document"))
	if targetObject == "" {
		targetObject = "workspace_document"
	}

	return map[string]any{
		"approval_id":    fmt.Sprintf("appr_%s", taskID),
		"task_id":        taskID,
		"operation_name": firstNonEmptyString(assessment.OperationName, firstNonEmptyString(stringValue(taskIntent, "name", ""), "write_file")),
		"risk_level":     firstNonEmptyString(assessment.RiskLevel, "red"),
		"target_object":  targetObject,
		"reason":         firstNonEmptyString(assessment.Reason, "policy_requires_authorization"),
		"status":         "pending",
		"created_at":     time.Now().Format(dateTimeLayout),
	}
}

// buildImpactScope 处理当前模块的相关逻辑。

// buildImpactScope 构造最小影响范围摘要，用于授权结果回传和安全面板展示。
func (s *Service) buildImpactScope(task runengine.TaskRecord, pendingExecution map[string]any) map[string]any {
	if impactScope, ok := pendingExecution["impact_scope"].(map[string]any); ok && len(impactScope) > 0 {
		return cloneMap(impactScope)
	}
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
	if !isEmptySnapshot(task.Snapshot) {
		return cloneTaskSnapshot(task.Snapshot)
	}
	return contextsvc.TaskContextSnapshot{
		Trigger:   task.SourceType,
		InputType: "text",
		Text:      originalTextFromTaskTitle(task.Title),
	}
}

func cloneTaskSnapshot(snapshot contextsvc.TaskContextSnapshot) contextsvc.TaskContextSnapshot {
	cloned := snapshot
	if len(snapshot.Files) > 0 {
		cloned.Files = append([]string(nil), snapshot.Files...)
	}
	return cloned
}

func isEmptySnapshot(snapshot contextsvc.TaskContextSnapshot) bool {
	return strings.TrimSpace(snapshot.Source) == "" &&
		strings.TrimSpace(snapshot.Trigger) == "" &&
		strings.TrimSpace(snapshot.InputType) == "" &&
		strings.TrimSpace(snapshot.InputMode) == "" &&
		strings.TrimSpace(snapshot.Text) == "" &&
		strings.TrimSpace(snapshot.SelectionText) == "" &&
		strings.TrimSpace(snapshot.ErrorText) == "" &&
		len(snapshot.Files) == 0 &&
		strings.TrimSpace(snapshot.PageTitle) == "" &&
		strings.TrimSpace(snapshot.PageURL) == "" &&
		strings.TrimSpace(snapshot.AppName) == "" &&
		strings.TrimSpace(snapshot.WindowTitle) == "" &&
		strings.TrimSpace(snapshot.VisibleText) == "" &&
		strings.TrimSpace(snapshot.ScreenSummary) == "" &&
		strings.TrimSpace(snapshot.ClipboardText) == "" &&
		strings.TrimSpace(snapshot.HoverTarget) == "" &&
		strings.TrimSpace(snapshot.LastAction) == "" &&
		snapshot.DwellMillis == 0 &&
		snapshot.CopyCount == 0 &&
		snapshot.WindowSwitches == 0 &&
		snapshot.PageSwitches == 0
}

func originalTextFromTaskTitle(title string) string {
	trimmed := strings.TrimSpace(title)
	for _, prefix := range []string{"确认处理方式：", "改写：", "翻译：", "解释错误：", "解释：", "总结文件：", "总结：", "处理："} {
		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		}
	}
	return trimmed
}

// memoryQueryFromSnapshot 处理当前模块的相关逻辑。

// memoryQueryFromSnapshot 从当前上下文挑选最适合作为检索 query 的内容。
func memoryQueryFromSnapshot(snapshot contextsvc.TaskContextSnapshot) string {
	for _, value := range []string{snapshot.SelectionText, snapshot.ClipboardText, snapshot.Text, snapshot.ErrorText, snapshot.VisibleText, snapshot.ScreenSummary, snapshot.PageTitle, snapshot.WindowTitle} {
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
func buildMemorySummary(snapshot contextsvc.TaskContextSnapshot, taskIntent map[string]any, deliveryResult map[string]any) string {
	intentName := stringValue(taskIntent, "name", "summarize")
	title := stringValue(deliveryResult, "title", "任务结果")
	query := memoryQueryFromSnapshot(snapshot)
	preview := stringValue(deliveryResult, "preview_text", "")
	if preview == "" {
		preview = title
	}
	perceptionSummary := []string{}
	if snapshot.CopyCount > 0 || strings.EqualFold(snapshot.LastAction, "copy") {
		perceptionSummary = append(perceptionSummary, "copy")
	}
	if snapshot.DwellMillis > 0 {
		perceptionSummary = append(perceptionSummary, fmt.Sprintf("dwell=%dms", snapshot.DwellMillis))
	}
	if snapshot.WindowSwitches > 0 || snapshot.PageSwitches > 0 {
		perceptionSummary = append(perceptionSummary, fmt.Sprintf("switch=%d/%d", snapshot.WindowSwitches, snapshot.PageSwitches))
	}
	if snapshot.PageTitle != "" {
		perceptionSummary = append(perceptionSummary, "page="+truncateText(snapshot.PageTitle, 24))
	}
	if len(perceptionSummary) == 0 {
		return fmt.Sprintf("任务完成，意图=%s，输入=%s，交付=%s，结果摘要=%s", intentName, truncateText(query, 48), title, truncateText(preview, 96))
	}
	return fmt.Sprintf("任务完成，意图=%s，输入=%s，感知=%s，交付=%s，结果摘要=%s", intentName, truncateText(query, 48), strings.Join(perceptionSummary, ", "), title, truncateText(preview, 96))
}

// resultSpecFromIntent 处理当前模块的相关逻辑。

// resultSpecFromIntent 根据 intent 返回默认结果标题、预览文案和结果气泡文案。
func resultSpecFromIntent(taskIntent map[string]any) (string, string, string) {
	switch stringValue(taskIntent, "name", "summarize") {
	case "agent_loop":
		return "处理结果", "结果已通过气泡返回", "结果已经生成，可直接查看。"
	case "rewrite":
		return "改写结果", "已为你写入文档并打开", "内容已经按要求改写完成，可直接查看。"
	case "translate":
		return "翻译结果", "结果已通过气泡返回", "翻译结果已经生成，可直接查看。"
	case "explain":
		return "解释结果", "结果已通过气泡返回", "这段内容的意思已经整理好了。"
	case "page_read":
		return "网页读取结果", "结果已通过气泡返回", "网页主要内容已经整理完成，可直接查看。"
	case "page_search":
		return "网页搜索结果", "结果已通过气泡返回", "网页搜索结果已经返回，可直接查看。"
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
	case "agent_loop", "translate", "explain", "page_read", "page_search":
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

// mergeSuggestedDeliveryPreference preserves explicit caller preferences and only
// falls back to the intent layer's suggested delivery when the caller left the
// preferred delivery unset.
func mergeSuggestedDeliveryPreference(preferredDelivery, fallbackDelivery, suggestedDelivery string) (string, string) {
	if strings.TrimSpace(preferredDelivery) == "" && strings.TrimSpace(suggestedDelivery) != "" {
		preferredDelivery = suggestedDelivery
	}
	return preferredDelivery, fallbackDelivery
}

// buildPendingExecution 生成等待授权任务在恢复执行时需要的交付计划。
func (s *Service) buildPendingExecution(task runengine.TaskRecord, taskIntent map[string]any) map[string]any {
	plan := s.delivery.BuildApprovalExecutionPlan(task.TaskID, taskIntent)
	return s.applyResolvedDeliveryToPlan(task, plan, taskIntent)
}

func (s *Service) applyGovernanceAssessment(plan map[string]any, assessment execution.GovernanceAssessment) map[string]any {
	updatedPlan := cloneMap(plan)
	if updatedPlan == nil {
		updatedPlan = map[string]any{}
	}
	if len(assessment.ImpactScope) > 0 {
		updatedPlan["impact_scope"] = cloneMap(assessment.ImpactScope)
	}
	if assessment.OperationName != "" {
		updatedPlan["operation_name"] = assessment.OperationName
	}
	if assessment.TargetObject != "" {
		updatedPlan["target_object"] = assessment.TargetObject
	}
	if assessment.RiskLevel != "" {
		updatedPlan["risk_level"] = assessment.RiskLevel
	}
	if assessment.Reason != "" {
		updatedPlan["risk_reason"] = assessment.Reason
	}
	updatedPlan["checkpoint_required"] = assessment.CheckpointRequired
	return updatedPlan
}

func (s *Service) assessTaskGovernance(task runengine.TaskRecord, taskIntent map[string]any) (execution.GovernanceAssessment, bool, error) {
	if s.executor == nil {
		return execution.GovernanceAssessment{}, false, nil
	}
	resultTitle, _, _ := resultSpecFromIntent(taskIntent)
	return s.executor.AssessGovernance(context.Background(), execution.Request{
		TaskID:       task.TaskID,
		RunID:        task.RunID,
		Title:        task.Title,
		Intent:       taskIntent,
		Snapshot:     snapshotFromTask(task),
		DeliveryType: resolveTaskDeliveryType(task, taskIntent),
		ResultTitle:  resultTitle,
	})
}

func (s *Service) handleTaskGovernanceDecision(task runengine.TaskRecord, taskIntent map[string]any) (runengine.TaskRecord, map[string]any, bool, error) {
	assessment, ok, err := s.assessTaskGovernance(task, taskIntent)
	if err != nil {
		return task, nil, false, err
	}
	if !ok {
		assessment, ok = s.fallbackGovernanceAssessment(task, taskIntent)
		if !ok {
			return task, nil, false, nil
		}
	}
	if assessment.Deny {
		response, blockedTask, blockErr := s.blockTaskByAssessment(task, assessment)
		return blockedTask, response, true, blockErr
	}
	if !assessment.ApprovalRequired {
		return task, nil, false, nil
	}
	pendingExecution := s.applyGovernanceAssessment(s.buildPendingExecution(task, taskIntent), assessment)
	approvalRequest := buildApprovalRequest(task.TaskID, taskIntent, assessment)
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "检测到待授权操作，请先确认。", task.UpdatedAt.Format(dateTimeLayout))
	updatedTask, changed := s.runEngine.MarkWaitingApprovalWithPlan(task.TaskID, approvalRequest, pendingExecution, bubble)
	if !changed {
		return task, nil, false, ErrTaskNotFound
	}
	return updatedTask, map[string]any{
		"task":            taskMap(updatedTask),
		"bubble_message":  bubble,
		"delivery_result": nil,
	}, true, nil
}

func (s *Service) fallbackGovernanceAssessment(task runengine.TaskRecord, taskIntent map[string]any) (execution.GovernanceAssessment, bool) {
	if stringValue(taskIntent, "name", "") != "write_file" && !boolValue(mapValue(taskIntent, "arguments"), "require_authorization", false) {
		return execution.GovernanceAssessment{}, false
	}
	plan := s.buildPendingExecution(task, taskIntent)
	impactScope := s.buildImpactScope(task, plan)
	return execution.GovernanceAssessment{
		OperationName:    firstNonEmptyString(stringValue(taskIntent, "name", ""), "write_file"),
		TargetObject:     impactScopeTarget(impactScope, targetPathFromIntent(taskIntent)),
		RiskLevel:        "red",
		ApprovalRequired: true,
		Reason:           "policy_requires_authorization",
		ImpactScope:      impactScope,
	}, true
}

func (s *Service) blockTaskByAssessment(task runengine.TaskRecord, assessment execution.GovernanceAssessment) (map[string]any, runengine.TaskRecord, error) {
	bubbleText := governanceInterceptionBubble(assessment)
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", bubbleText, task.UpdatedAt.Format(dateTimeLayout))
	updatedTask, ok := s.runEngine.BlockTaskByPolicy(task.TaskID, assessment.RiskLevel, bubbleText, assessment.ImpactScope, bubble)
	if !ok {
		return nil, task, ErrTaskNotFound
	}
	auditRecord := s.writeGovernanceAuditRecord(updatedTask.TaskID, updatedTask.RunID, "risk", "intercept_operation", bubbleText, impactScopeTarget(assessment.ImpactScope, assessment.TargetObject), "denied")
	updatedTask = s.appendAuditData(updatedTask, compactAuditRecords(auditRecord), nil)
	return map[string]any{
		"task":            taskMap(updatedTask),
		"bubble_message":  bubble,
		"delivery_result": nil,
		"impact_scope":    cloneMap(assessment.ImpactScope),
	}, updatedTask, nil
}

func (s *Service) writeGovernanceAuditRecord(taskID, runID, auditType, action, summary, target, result string) map[string]any {
	if s.audit == nil {
		return nil
	}
	if record, err := s.audit.Write(context.Background(), audit.RecordInput{
		TaskID:  taskID,
		Type:    auditType,
		Action:  action,
		Summary: summary,
		Target:  target,
		Result:  result,
	}); err == nil {
		return record.Map()
	}
	if record, err := s.audit.BuildRecord(audit.RecordInput{
		TaskID:  taskID,
		Type:    auditType,
		Action:  action,
		Summary: summary,
		Target:  target,
		Result:  result,
	}); err == nil {
		return record.Map()
	}
	return nil
}

func attachDeliveryResultToArtifacts(deliveryResult map[string]any, artifacts []map[string]any) []map[string]any {
	if len(artifacts) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(artifacts))
	for _, artifact := range artifacts {
		cloned := cloneMap(artifact)
		if cloned == nil {
			continue
		}
		cloned["delivery_type"] = stringValue(deliveryResult, "type", "")
		cloned["delivery_payload"] = cloneMap(mapValue(deliveryResult, "payload"))
		if stringValue(cloned, "created_at", "") == "" {
			cloned["created_at"] = time.Now().UTC().Format(time.RFC3339)
		}
		result = append(result, cloned)
	}
	return result
}

func (s *Service) persistArtifacts(taskID string, artifactPlans []map[string]any) {
	if s.storage == nil || s.storage.ArtifactStore() == nil || len(artifactPlans) == 0 {
		return
	}
	records := make([]storage.ArtifactRecord, 0, len(artifactPlans))
	for _, plan := range artifactPlans {
		records = append(records, storage.ArtifactRecord{
			ArtifactID:          stringValue(plan, "artifact_id", ""),
			TaskID:              firstNonEmptyString(stringValue(plan, "task_id", ""), taskID),
			ArtifactType:        stringValue(plan, "artifact_type", ""),
			Title:               stringValue(plan, "title", ""),
			Path:                stringValue(plan, "path", ""),
			MimeType:            stringValue(plan, "mime_type", ""),
			DeliveryType:        stringValue(plan, "delivery_type", ""),
			DeliveryPayloadJSON: stringValue(plan, "delivery_payload_json", "{}"),
			CreatedAt:           firstNonEmptyString(stringValue(plan, "created_at", ""), time.Now().UTC().Format(time.RFC3339)),
		})
	}
	_ = s.storage.ArtifactStore().SaveArtifacts(context.Background(), records)
	if task, ok := s.runEngine.GetTask(taskID); ok {
		merged := mergeArtifactsWithStored(task.Artifacts, s.loadArtifactsFromStorage(taskID, 0, 0))
		_, _ = s.runEngine.SetPresentation(taskID, task.BubbleMessage, task.DeliveryResult, merged)
	}
}

func (s *Service) artifactsForTask(taskID string, runtimeArtifacts []map[string]any) []map[string]any {
	return mergeArtifactsWithStored(delivery.EnsureArtifactIdentifiers(taskID, runtimeArtifacts), s.loadArtifactsFromStorage(taskID, 0, 0))
}

func (s *Service) loadArtifactsFromStorage(taskID string, limit, offset int) []map[string]any {
	if s.storage == nil || s.storage.ArtifactStore() == nil || strings.TrimSpace(taskID) == "" {
		return nil
	}
	records, _, err := s.storage.ArtifactStore().ListArtifacts(context.Background(), taskID, limit, offset)
	if err != nil {
		return nil
	}
	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		items = append(items, artifactMapFromStorage(record))
	}
	return items
}

func (s *Service) listArtifactsPage(taskID string, limit, offset int) ([]map[string]any, int, error) {
	if s.storage != nil && s.storage.ArtifactStore() != nil {
		records, total, err := s.storage.ArtifactStore().ListArtifacts(context.Background(), taskID, limit, offset)
		if err != nil {
			return nil, 0, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
		}
		if total > 0 {
			items := make([]map[string]any, 0, len(records))
			for _, record := range records {
				items = append(items, artifactMapFromStorage(record))
			}
			return items, total, nil
		}
	}
	items := s.artifactsForTask(taskID, currentTaskArtifacts(s.runEngine, taskID))
	total := len(items)
	if offset >= total {
		return []map[string]any{}, total, nil
	}
	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}
	return cloneMapSlice(items[offset:end]), total, nil
}

func currentTaskArtifacts(engine *runengine.Engine, taskID string) []map[string]any {
	if engine == nil || strings.TrimSpace(taskID) == "" {
		return nil
	}
	task, ok := engine.GetTask(taskID)
	if !ok {
		return nil
	}
	return cloneMapSlice(task.Artifacts)
}

func (s *Service) findArtifactForTask(taskID, artifactID string) (map[string]any, error) {
	if strings.TrimSpace(taskID) == "" {
		return nil, ErrTaskNotFound
	}
	exists := false
	if task, ok := s.runEngine.GetTask(taskID); ok {
		exists = true
		for _, artifact := range delivery.EnsureArtifactIdentifiers(taskID, task.Artifacts) {
			if stringValue(artifact, "artifact_id", "") == artifactID {
				return cloneMap(artifact), nil
			}
		}
	}
	if !exists {
		if _, ok := s.taskDetailFromStorage(taskID); ok {
			exists = true
		}
	}
	if s.storage != nil && s.storage.ArtifactStore() != nil {
		records, _, err := s.storage.ArtifactStore().ListArtifacts(context.Background(), taskID, 0, 0)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
		}
		if len(records) > 0 {
			exists = true
		}
		for _, record := range records {
			if record.ArtifactID == artifactID {
				return artifactMapFromStorage(record), nil
			}
		}
	}
	if !exists {
		return nil, ErrTaskNotFound
	}
	return nil, ErrArtifactNotFound
}

func mergeArtifactsWithStored(runtimeArtifacts, storedArtifacts []map[string]any) []map[string]any {
	if len(runtimeArtifacts) == 0 && len(storedArtifacts) == 0 {
		return nil
	}
	merged := make([]map[string]any, 0, len(runtimeArtifacts)+len(storedArtifacts))
	seen := make(map[string]struct{})
	for _, group := range [][]map[string]any{storedArtifacts, runtimeArtifacts} {
		for _, artifact := range group {
			artifactID := stringValue(artifact, "artifact_id", "")
			if artifactID == "" {
				continue
			}
			if _, ok := seen[artifactID]; ok {
				continue
			}
			seen[artifactID] = struct{}{}
			merged = append(merged, cloneMap(artifact))
		}
	}
	return merged
}

func artifactMapFromStorage(record storage.ArtifactRecord) map[string]any {
	payload := map[string]any{}
	if strings.TrimSpace(record.DeliveryPayloadJSON) != "" {
		_ = json.Unmarshal([]byte(record.DeliveryPayloadJSON), &payload)
	}
	return map[string]any{
		"artifact_id":      record.ArtifactID,
		"task_id":          record.TaskID,
		"artifact_type":    record.ArtifactType,
		"title":            record.Title,
		"path":             record.Path,
		"mime_type":        record.MimeType,
		"delivery_type":    record.DeliveryType,
		"delivery_payload": payload,
		"created_at":       record.CreatedAt,
	}
}

func governanceInterceptionBubble(assessment execution.GovernanceAssessment) string {
	switch assessment.Reason {
	case risk.ReasonOutOfWorkspace:
		return "目标超出工作区边界，已阻止本次操作。"
	case risk.ReasonCommandNotAllowed:
		return "命令存在高危风险，已被策略拦截。"
	case risk.ReasonCapabilityDenied:
		return "当前平台能力不可用，已阻止本次操作。"
	default:
		return "高风险操作已被策略拦截，未进入执行。"
	}
}

func impactScopeTarget(impactScope map[string]any, fallback string) string {
	if files := stringSliceValue(impactScope["files"]); len(files) > 0 {
		return files[0]
	}
	return firstNonEmptyString(strings.TrimSpace(fallback), "main_flow")
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
	processingTask, ok := s.runEngine.BeginExecution(task.TaskID, executionStepName(taskIntent), "开始生成正式结果")
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
		artifacts := delivery.EnsureArtifactIdentifiers(processingTask.TaskID, s.delivery.BuildArtifact(processingTask.TaskID, resultTitle, deliveryResult))
		resultBubble := s.delivery.BuildBubbleMessage(processingTask.TaskID, "result", resultBubbleText, processingTask.UpdatedAt.Format(dateTimeLayout))
		processingTask = s.appendAuditData(processingTask, compactAuditRecords(s.audit.BuildDeliveryAudit(processingTask.TaskID, processingTask.RunID, deliveryResult)), nil)
		updatedTask, ok := s.runEngine.CompleteTask(processingTask.TaskID, deliveryResult, resultBubble, artifacts)
		if !ok {
			return runengine.TaskRecord{}, nil, nil, nil, ErrTaskNotFound
		}
		s.attachPostDeliveryHandoffs(updatedTask.TaskID, updatedTask.RunID, snapshot, taskIntent, deliveryResult, artifacts)
		return updatedTask, resultBubble, deliveryResult, artifacts, nil
	}

	approvedOperation, approvedTargetObject := approvedExecutionFromTask(processingTask)
	executionResult, err := s.executor.Execute(context.Background(), execution.Request{
		TaskID:               processingTask.TaskID,
		RunID:                processingTask.RunID,
		Title:                processingTask.Title,
		Intent:               taskIntent,
		Snapshot:             snapshot,
		DeliveryType:         deliveryType,
		ResultTitle:          resultTitle,
		ApprovalGranted:      processingTask.Authorization != nil,
		ApprovedOperation:    approvedOperation,
		ApprovedTargetObject: approvedTargetObject,
	})
	processingTask = s.recordExecutionToolCalls(processingTask, executionResult.ToolCalls)
	auditDeliveryResult := executionResult.DeliveryResult
	if err != nil {
		auditDeliveryResult = nil
	}
	executionAuditRecords, executionTokenUsage := s.buildExecutionAudit(processingTask, executionResult.ToolCalls, auditDeliveryResult)
	processingTask = s.appendAuditData(processingTask, executionAuditRecords, executionTokenUsage)
	if err != nil {
		failedTask, failureBubble := s.failExecutionTask(processingTask, taskIntent, executionResult, err)
		return failedTask, failureBubble, nil, nil, nil
	}

	resultBubble := s.delivery.BuildBubbleMessage(
		processingTask.TaskID,
		"result",
		firstNonEmptyString(executionResult.BubbleText, resultBubbleText),
		processingTask.UpdatedAt.Format(dateTimeLayout),
	)
	executionArtifacts := delivery.EnsureArtifactIdentifiers(processingTask.TaskID, executionResult.Artifacts)
	updatedTask, ok := s.runEngine.CompleteTask(processingTask.TaskID, executionResult.DeliveryResult, resultBubble, executionArtifacts, executionResult.RecoveryPoint)
	if !ok {
		return runengine.TaskRecord{}, nil, nil, nil, ErrTaskNotFound
	}
	s.attachPostDeliveryHandoffs(updatedTask.TaskID, updatedTask.RunID, snapshot, taskIntent, executionResult.DeliveryResult, executionArtifacts)
	return updatedTask, resultBubble, executionResult.DeliveryResult, executionArtifacts, nil
}

func (s *Service) recordExecutionToolCalls(task runengine.TaskRecord, toolCalls []tools.ToolCallRecord) runengine.TaskRecord {
	for _, toolCall := range toolCalls {
		if toolCall.ToolName == "" {
			continue
		}
		if recordedTask, ok := s.runEngine.RecordToolCallLifecycle(
			task.TaskID,
			toolCall.ToolName,
			string(toolCall.Status),
			toolCall.Input,
			toolCall.Output,
			toolCall.DurationMS,
			toolCallErrorCode(toolCall),
		); ok {
			task = recordedTask
		}
	}
	return task
}

func executionStepName(taskIntent map[string]any) string {
	if stringValue(taskIntent, "name", "") == "agent_loop" {
		return "agent_loop"
	}
	return "generate_output"
}

func approvedExecutionFromTask(task runengine.TaskRecord) (string, string) {
	if len(task.PendingExecution) == 0 {
		return "", ""
	}
	return stringValue(task.PendingExecution, "operation_name", ""), stringValue(task.PendingExecution, "target_object", "")
}

func toolCallErrorCode(toolCall tools.ToolCallRecord) any {
	if toolCall.ErrorCode == nil {
		return nil
	}
	return *toolCall.ErrorCode
}

func (s *Service) failExecutionTask(task runengine.TaskRecord, taskIntent map[string]any, executionResult execution.Result, err error) (runengine.TaskRecord, map[string]any) {
	impactScope := s.buildImpactScope(task, task.PendingExecution)
	bubbleText := executionFailureBubble(err)
	securityStatus := "execution_error"
	stepName := "execution_failed"
	auditType := "execution"
	auditAction := "execute_task"
	auditTarget := impactScopeTarget(impactScope, targetPathFromIntent(taskIntent))
	auditResult := "failed"
	if errors.Is(err, execution.ErrRecoveryPointPrepareFailed) {
		securityStatus = "execution_error"
		stepName = "recovery_prepare_failed"
		auditType = "recovery"
		auditAction = "create_recovery_point"
		auditTarget = impactScopeTarget(impactScope, stringValue(executionResult.RecoveryPoint, "summary", "workspace"))
	}
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", bubbleText, task.UpdatedAt.Format(dateTimeLayout))
	updatedTask, ok := s.runEngine.FailTaskExecution(task.TaskID, stepName, securityStatus, bubbleText, impactScope, bubble, executionResult.RecoveryPoint)
	if !ok {
		return task, bubble
	}
	auditRecord := s.writeGovernanceAuditRecord(updatedTask.TaskID, updatedTask.RunID, auditType, auditAction, bubbleText, auditTarget, auditResult)
	updatedTask = s.appendAuditData(updatedTask, compactAuditRecords(auditRecord), nil)
	return updatedTask, bubble
}

func executionFailureBubble(err error) string {
	switch {
	case errors.Is(err, execution.ErrRecoveryPointPrepareFailed):
		return "执行失败：执行前恢复点创建失败，请稍后重试。"
	case errors.Is(err, tools.ErrWorkspaceBoundaryDenied):
		return "执行失败：目标超出工作区边界，已阻止本次操作。"
	case errors.Is(err, tools.ErrCommandNotAllowed):
		return "执行失败：命令存在高危风险，已被策略拦截。"
	case errors.Is(err, tools.ErrCapabilityDenied):
		return "执行失败：当前平台能力不可用，请检查环境后重试。"
	case errors.Is(err, tools.ErrToolExecutionFailed):
		return "执行失败：工具运行失败，请检查环境后重试。"
	default:
		return "执行失败：请稍后重试。"
	}
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
