// Package orchestrator assembles the owner-4 task-centric backend workflow.
package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
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
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/traceeval"
)

// Service is the task-centric orchestration entrypoint for the local-service
// backend.
type Service struct {
	context          *contextsvc.Service
	intent           *intent.Service
	runEngine        *runengine.Engine
	delivery         *delivery.Service
	memory           *memory.Service
	risk             *risk.Service
	model            *model.Service
	tools            *tools.Registry
	plugin           *plugin.Service
	audit            *audit.Service
	recommendation   *recommendation.Service
	traceEval        *traceeval.Service
	executor         *execution.Service
	inspector        *taskinspector.Service
	storage          *storage.Service
	modelMu          sync.RWMutex
	runtimeMu        sync.RWMutex
	executionTimeout time.Duration
	runtimeNextID    uint64
	runtimeTaps      map[uint64]func(taskID, method string, params map[string]any)
	taskStartTaps    map[uint64]func(taskID, sessionID, traceID string)
}

// NewService wires the main orchestration dependencies.
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
		context:          context,
		intent:           intent,
		runEngine:        runEngine,
		delivery:         delivery,
		memory:           memory,
		risk:             risk,
		model:            model,
		tools:            tools,
		plugin:           plugin,
		audit:            audit.NewService(),
		recommendation:   recommendation.NewService(),
		traceEval:        traceeval.NewService(nil, nil),
		inspector:        taskinspector.NewService(nil),
		executionTimeout: defaultTaskExecutionTimeout,
		runtimeTaps:      map[uint64]func(taskID, method string, params map[string]any){},
		taskStartTaps:    map[uint64]func(taskID, sessionID, traceID string){},
	}
}

// WithAudit attaches the shared audit service so runtime views do not fork
// their own counters.
func (s *Service) WithAudit(auditService *audit.Service) *Service {
	if auditService != nil {
		s.audit = auditService
	}
	return s
}

// WithExecutor attaches the execution service used by the main task loop.
func (s *Service) WithExecutor(executorService *execution.Service) *Service {
	s.executor = executorService
	if executorService != nil {
		executorService.WithNotificationEmitter(func(taskID, method string, params map[string]any) {
			s.publishRuntimeNotification(taskID, method, params)
			_, _ = s.runEngine.EmitRuntimeNotification(taskID, method, params)
		}).WithSteeringPoller(func(taskID string) []string {
			messages, ok := s.runEngine.DrainSteeringMessages(taskID)
			if !ok {
				return nil
			}
			return messages
		})
	}
	return s
}

// WithTaskInspector attaches the task-inspector runtime service.
func (s *Service) WithTaskInspector(inspectorService *taskinspector.Service) *Service {
	if inspectorService != nil {
		s.inspector = inspectorService
	}
	return s
}

// WithStorage attaches shared storage for governance and query-side hydration.
func (s *Service) WithStorage(storageService *storage.Service) *Service {
	if storageService != nil {
		s.storage = storageService
	}
	return s
}

// WithTraceEval attaches the owner-5 trace/eval recording service.
func (s *Service) WithTraceEval(traceEvalService *traceeval.Service) *Service {
	if traceEvalService != nil {
		s.traceEval = traceEvalService
	}
	return s
}

// Snapshot returns the minimal orchestrator summary used by debug and health
// endpoints.
func (s *Service) Snapshot() map[string]any {
	pendingApprovals, pendingTotal := s.runEngine.PendingApprovalRequests(100, 0)
	primaryWorker := ""
	if s.plugin != nil {
		if workers := s.plugin.Workers(); len(workers) > 0 {
			primaryWorker = workers[0]
		}
	}
	return map[string]any{
		"context_source":          s.context.Snapshot()["source"],
		"intent_state":            s.intent.Analyze("bootstrap"),
		"task_status":             s.runEngine.CurrentTaskStatus(),
		"run_state":               s.runEngine.CurrentState(),
		"delivery_type":           s.delivery.DefaultResultType(),
		"memory_backend":          s.memory.RetrievalBackend(),
		"risk_level":              s.risk.DefaultLevel(),
		"model":                   s.currentModelDescriptor(),
		"tool_count":              len(s.tools.Names()),
		"primary_worker":          primaryWorker,
		"pending_approvals":       pendingTotal,
		"latest_approval_request": firstMapOrNil(pendingApprovals),
	}
}

// RunEngine exposes the attached runtime engine for transport-layer tests and
// debug wiring that need to seed notifications or inspect task state.
func (s *Service) RunEngine() *runengine.Engine {
	return s.runEngine
}

func (s *Service) handleScreenAnalyzeStart(params map[string]any, snapshot contextsvc.TaskContextSnapshot, explicitIntent map[string]any) (map[string]any, bool, error) {
	if stringValue(explicitIntent, "name", "") != "screen_analyze" || s.executor == nil || !s.executor.ScreenCapabilitySnapshot().Available {
		return nil, false, nil
	}
	resolvedIntent := s.resolveScreenAnalyzeIntent(snapshot, explicitIntent)
	task := s.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:         stringValue(params, "session_id", ""),
		RequestSource:     stringValue(params, "source", ""),
		RequestTrigger:    stringValue(params, "trigger", ""),
		Title:             firstNonEmptyString(stringValue(resolvedIntent, "title", ""), inferredScreenTaskTitle(snapshot)),
		SourceType:        "screen_capture",
		Status:            "waiting_auth",
		Intent:            cloneMap(resolvedIntent),
		PreferredDelivery: "bubble",
		FallbackDelivery:  "bubble",
		CurrentStep:       "waiting_authorization",
		RiskLevel:         "yellow",
		Timeline:          initialTimeline("waiting_auth", "waiting_authorization"),
		Snapshot:          snapshot,
	})
	if queuedTask, queueBubble, queued, queueErr := s.queueTaskIfSessionBusy(task); queueErr != nil {
		return nil, false, queueErr
	} else if queued {
		return map[string]any{
			"task":            taskMap(queuedTask),
			"bubble_message":  queueBubble,
			"delivery_result": nil,
		}, true, nil
	}
	approvalRequest, pendingExecution, bubble, err := s.buildScreenAnalysisApprovalState(task)
	if err != nil {
		return nil, false, err
	}
	updatedTask, ok := s.runEngine.MarkWaitingApprovalWithPlan(task.TaskID, approvalRequest, pendingExecution, bubble)
	if !ok {
		return nil, false, ErrTaskNotFound
	}
	if err := s.persistApprovalRequestState(updatedTask.TaskID, approvalRequest, mapValue(pendingExecution, "impact_scope")); err != nil {
		return nil, false, err
	}
	return map[string]any{
		"task":            taskMap(updatedTask),
		"bubble_message":  bubble,
		"delivery_result": nil,
	}, true, nil
}

func (s *Service) handleScreenAnalyzeSuggestion(params map[string]any, snapshot contextsvc.TaskContextSnapshot, suggestion intent.Suggestion) (map[string]any, bool, error) {
	if stringValue(suggestion.Intent, "name", "") != "screen_analyze" || suggestion.RequiresConfirm {
		return nil, false, nil
	}
	return s.handleScreenAnalyzeStart(params, snapshot, suggestion.Intent)
}

func (s *Service) normalizeSuggestedIntentForAvailability(snapshot contextsvc.TaskContextSnapshot, suggestion intent.Suggestion, confirmRequired bool) intent.Suggestion {
	if stringValue(suggestion.Intent, "name", "") != "screen_analyze" {
		return suggestion
	}
	if s.executor != nil && s.executor.ScreenCapabilitySnapshot().Available {
		return suggestion
	}
	fallback := suggestion
	fallback.Intent = map[string]any{
		"name":      "agent_loop",
		"arguments": map[string]any{},
	}
	fallback.IntentConfirmed = true
	// Preserve the caller's confirmation gate when screen-specific handling is
	// unavailable so the downgrade does not auto-execute a generic task.
	fallback.RequiresConfirm = confirmRequired
	fallback.TaskSourceType = "hover_input"
	fallback.TaskTitle = "处理：" + inferredScreenFallbackSubject(snapshot)
	fallback.DirectDeliveryType = "bubble"
	fallback.ResultTitle = "处理结果"
	fallback.ResultPreview = "结果已通过气泡返回"
	fallback.ResultBubbleText = "当前环境暂不支持受控屏幕查看，已改为按现有文本和页面上下文继续处理。"
	return fallback
}

func inferredScreenFallbackSubject(snapshot contextsvc.TaskContextSnapshot) string {
	return truncateText(firstNonEmptyString(strings.TrimSpace(snapshot.Text), screenSubjectFromSnapshot(snapshot)), subjectPreviewMaxLength)
}

// buildScreenAnalysisApprovalState reconstructs the controlled approval plan
// from the task intent so queued resumes can re-enter the same authorization
// path instead of falling through to the generic executor.
func (s *Service) buildScreenAnalysisApprovalState(task runengine.TaskRecord) (map[string]any, map[string]any, map[string]any, error) {
	arguments := mapValue(task.Intent, "arguments")
	sourcePath := stringValue(arguments, "path", "")
	captureMode := screenCaptureModeForIntent(arguments)
	source := firstNonEmptyString(stringValue(arguments, "source", ""), "screen_capture")
	targetObject := screenTargetObject(arguments)
	approvalRequest := map[string]any{
		"approval_id":    fmt.Sprintf("appr_%s", task.TaskID),
		"task_id":        task.TaskID,
		"operation_name": "screen_capture",
		"risk_level":     "yellow",
		"target_object":  targetObject,
		"reason":         "screen_capture_requires_authorization",
		"status":         "pending",
		"created_at":     time.Now().Format(dateTimeLayout),
	}
	pendingExecution := map[string]any{
		"kind":           "screen_analysis",
		"operation_name": "screen_capture",
		"source_path":    sourcePath,
		"capture_mode":   string(captureMode),
		"source":         source,
		"target_object":  targetObject,
		"language":       firstNonEmptyString(stringValue(arguments, "language", ""), "eng"),
		"evidence_role":  firstNonEmptyString(stringValue(arguments, "evidence_role", ""), "error_evidence"),
		"delivery_type":  "bubble",
		"result_title":   "屏幕分析结果",
		"preview_text":   screenAnalysisPreviewText(captureMode),
		"impact_scope": map[string]any{
			"files":                    impactFilesForScreenTarget(sourcePath),
			"webpages":                 []string{},
			"apps":                     []string{},
			"out_of_workspace":         false,
			"overwrite_or_delete_risk": false,
		},
	}
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "屏幕截图分析属于敏感能力，请先确认授权。", task.UpdatedAt.Format(dateTimeLayout))
	return approvalRequest, pendingExecution, bubble, nil
}

func (s *Service) resolveScreenAnalyzeIntent(snapshot contextsvc.TaskContextSnapshot, current map[string]any) map[string]any {
	updatedIntent := cloneMap(current)
	arguments := cloneMap(mapValue(updatedIntent, "arguments"))
	if arguments == nil {
		arguments = map[string]any{}
	}
	if strings.TrimSpace(stringValue(arguments, "language", "")) == "" {
		arguments["language"] = "eng"
	}
	if strings.TrimSpace(stringValue(arguments, "capture_mode", "")) == "" {
		arguments["capture_mode"] = string(screenCaptureModeForIntent(arguments))
	}
	if strings.TrimSpace(stringValue(arguments, "evidence_role", "")) == "" {
		arguments["evidence_role"] = inferredScreenEvidenceRole(snapshot, arguments)
	}
	if strings.TrimSpace(stringValue(arguments, "page_title", "")) == "" && strings.TrimSpace(snapshot.PageTitle) != "" {
		arguments["page_title"] = snapshot.PageTitle
	}
	if strings.TrimSpace(stringValue(arguments, "window_title", "")) == "" && strings.TrimSpace(snapshot.WindowTitle) != "" {
		arguments["window_title"] = snapshot.WindowTitle
	}
	if strings.TrimSpace(stringValue(arguments, "visible_text", "")) == "" && strings.TrimSpace(snapshot.VisibleText) != "" {
		arguments["visible_text"] = snapshot.VisibleText
	}
	if strings.TrimSpace(stringValue(arguments, "screen_summary", "")) == "" && strings.TrimSpace(snapshot.ScreenSummary) != "" {
		arguments["screen_summary"] = snapshot.ScreenSummary
	}
	updatedIntent["arguments"] = arguments
	if strings.TrimSpace(stringValue(updatedIntent, "title", "")) == "" {
		updatedIntent["title"] = inferredScreenTaskTitle(snapshot)
	}
	return updatedIntent
}

func screenCaptureModeForIntent(arguments map[string]any) tools.ScreenCaptureMode {
	switch strings.ToLower(strings.TrimSpace(stringValue(arguments, "capture_mode", ""))) {
	case string(tools.ScreenCaptureModeClip):
		return tools.ScreenCaptureModeClip
	case string(tools.ScreenCaptureModeKeyframe):
		return tools.ScreenCaptureModeKeyframe
	case string(tools.ScreenCaptureModeScreenshot):
		return tools.ScreenCaptureModeScreenshot
	}
	if isClipScreenSourcePath(stringValue(arguments, "path", "")) {
		return tools.ScreenCaptureModeClip
	}
	return tools.ScreenCaptureModeScreenshot
}

func isClipScreenSourcePath(pathValue string) bool {
	trimmedPath := strings.ToLower(strings.TrimSpace(pathValue))
	switch path.Ext(trimmedPath) {
	case ".mp4", ".webm", ".mov", ".mkv", ".avi":
		return true
	default:
		return false
	}
}

func inferredScreenTaskTitle(snapshot contextsvc.TaskContextSnapshot) string {
	target := screenSubjectFromSnapshot(snapshot)
	if strings.TrimSpace(snapshot.ErrorText) != "" || strings.Contains(strings.ToLower(snapshot.Text), "错误") || strings.Contains(strings.ToLower(snapshot.Text), "报错") || strings.Contains(strings.ToLower(snapshot.Text), "error") {
		return fmt.Sprintf("查看屏幕报错：%s", truncateText(target, subjectPreviewMaxLength))
	}
	return fmt.Sprintf("查看当前屏幕：%s", truncateText(target, subjectPreviewMaxLength))
}

func screenSubjectFromSnapshot(snapshot contextsvc.TaskContextSnapshot) string {
	return firstNonEmptyString(
		snapshot.PageTitle,
		firstNonEmptyString(
			snapshot.WindowTitle,
			firstNonEmptyString(snapshot.ScreenSummary, firstNonEmptyString(snapshot.VisibleText, "当前屏幕")),
		),
	)
}

func screenTargetObject(arguments map[string]any) string {
	if sourcePath := stringValue(arguments, "path", ""); strings.TrimSpace(sourcePath) != "" {
		return sourcePath
	}
	for _, value := range []string{
		stringValue(arguments, "page_title", ""),
		stringValue(arguments, "window_title", ""),
		stringValue(arguments, "screen_summary", ""),
		stringValue(arguments, "visible_text", ""),
	} {
		if strings.TrimSpace(value) != "" {
			return truncateText(value, 64)
		}
	}
	return "current_screen"
}

func screenCaptureModeFromArguments(arguments map[string]any) tools.ScreenCaptureMode {
	mode := tools.ScreenCaptureMode(strings.TrimSpace(stringValue(arguments, "capture_mode", string(tools.ScreenCaptureModeScreenshot))))
	switch mode {
	case tools.ScreenCaptureModeScreenshot, tools.ScreenCaptureModeKeyframe, tools.ScreenCaptureModeClip:
		return mode
	default:
		return tools.ScreenCaptureModeScreenshot
	}
}

func screenAnalysisPreviewText(captureMode tools.ScreenCaptureMode) string {
	if captureMode == tools.ScreenCaptureModeClip {
		return "已准备分析屏幕录屏片段"
	}
	return "已准备分析屏幕截图"
}

func impactFilesForScreenTarget(sourcePath string) []string {
	if strings.TrimSpace(sourcePath) == "" {
		return []string{}
	}
	return []string{sourcePath}
}

func inferredScreenEvidenceRole(snapshot contextsvc.TaskContextSnapshot, arguments map[string]any) string {
	if role := stringValue(arguments, "evidence_role", ""); strings.TrimSpace(role) != "" {
		return role
	}
	combined := strings.ToLower(strings.Join([]string{snapshot.Text, snapshot.ErrorText, snapshot.VisibleText, snapshot.ScreenSummary}, " "))
	if strings.Contains(combined, "error") || strings.Contains(combined, "warning") || strings.Contains(combined, "报错") || strings.Contains(combined, "错误") || strings.Contains(combined, "异常") {
		return "error_evidence"
	}
	return "page_context"
}

// RecommendationGet handles agent.recommendation.get and returns lightweight
// recommendation actions derived from current context signals.
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
		Observations:    s.recommendationObservations(signals),
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

func (s *Service) recommendationObservations(signals perception.SignalSnapshot) []string {
	observations := perception.BehaviorSignals(signals)
	if hasErrorOpportunity := strings.TrimSpace(signals.ErrorText) != "" || strings.Contains(strings.ToLower(strings.Join([]string{signals.VisibleText, signals.ScreenSummary}, " ")), "error") || strings.Contains(strings.ToLower(strings.Join([]string{signals.VisibleText, signals.ScreenSummary}, " ")), "报错"); hasErrorOpportunity {
		observations = append(observations, "当前上下文包含可解释的视觉错误信号。")
	}
	if strings.TrimSpace(signals.ScreenSummary) != "" {
		observations = append(observations, fmt.Sprintf("screen:%s", truncateText(signals.ScreenSummary, 48)))
	}
	if strings.TrimSpace(signals.VisibleText) != "" {
		observations = append(observations, fmt.Sprintf("visible:%s", truncateText(signals.VisibleText, 48)))
	}
	return uniqueTrimmedStrings(observations)
}

func uniqueTrimmedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
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

// RecommendationFeedbackSubmit handles agent.recommendation.feedback.submit.
func (s *Service) RecommendationFeedbackSubmit(params map[string]any) (map[string]any, error) {
	return map[string]any{
		"applied": s.recommendation.SubmitFeedback(
			stringValue(params, "recommendation_id", ""),
			stringValue(params, "feedback", ""),
		),
	}, nil
}

// NotepadList handles agent.notepad.list.
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

// NotepadUpdate handles agent.notepad.update.
func (s *Service) NotepadUpdate(params map[string]any) (map[string]any, error) {
	itemID := stringValue(params, "item_id", "")
	if itemID == "" {
		return nil, fmt.Errorf("item_id is required")
	}

	action := stringValue(params, "action", "")
	if action == "" {
		return nil, fmt.Errorf("action is required")
	}

	updatedItem, refreshGroups, deletedItemID, handled, err := s.runEngine.UpdateNotepadItem(itemID, action)
	if err != nil {
		return nil, err
	}
	if !handled {
		return nil, fmt.Errorf("notepad item not found: %s", itemID)
	}

	response := map[string]any{
		"notepad_item":    any(nil),
		"refresh_groups":  refreshGroups,
		"deleted_item_id": nil,
	}
	if updatedItem != nil {
		response["notepad_item"] = updatedItem
	}
	if deletedItemID != "" {
		response["deleted_item_id"] = deletedItemID
	}
	return response, nil
}

// NotepadConvertToTask handles agent.notepad.convert_to_task.
func (s *Service) NotepadConvertToTask(params map[string]any) (map[string]any, error) {
	itemID := stringValue(params, "item_id", "")
	if itemID == "" {
		return nil, fmt.Errorf("item_id is required")
	}
	if !boolValue(params, "confirmed", false) {
		return nil, fmt.Errorf("confirmed must be true to convert notepad item")
	}

	item, handled, claimErr := s.runEngine.ClaimNotepadItemTask(itemID)
	if claimErr != nil {
		return nil, claimErr
	}
	if !handled {
		return nil, fmt.Errorf("notepad item not found: %s", itemID)
	}
	claimed := true
	defer func() {
		if claimed {
			s.runEngine.ReleaseNotepadItemClaim(itemID)
		}
	}()

	itemTitle := stringValue(item, "title", "待办事项")
	taskIntent := notepadIntent(item)
	task := s.runEngine.CreateTask(runengine.CreateTaskInput{
		RequestSource: "dashboard",
		Title:         itemTitle,
		SourceType:    "todo",
		Status:        "confirming_intent",
		Intent:        taskIntent,
		CurrentStep:   "intent_confirmation",
		RiskLevel:     s.risk.DefaultLevel(),
		Timeline:      initialTimeline("confirming_intent", "intent_confirmation"),
	})
	s.attachMemoryReadPlans(task.TaskID, task.RunID, notepadSnapshot(item), taskIntent)
	updatedItem, ok := s.runEngine.LinkNotepadItemTask(itemID, task.TaskID)
	if !ok {
		linkErr := fmt.Errorf("failed to link notepad item to task: %s", itemID)
		if rollbackErr := s.runEngine.DeleteTask(task.TaskID); rollbackErr != nil {
			return nil, errors.Join(linkErr, fmt.Errorf("rollback task %s: %w", task.TaskID, rollbackErr))
		}
		return nil, linkErr
	}
	claimed = false

	return map[string]any{
		"task":           taskMap(task),
		"notepad_item":   updatedItem,
		"refresh_groups": []string{stringValue(updatedItem, "bucket", "upcoming")},
	}, nil
}

// PluginRuntimeList exposes the smallest backend query surface for runtime
// plugin visibility so dashboard work can consume health and metric snapshots
// without depending on static worker declarations only.
func (s *Service) PluginRuntimeList(params map[string]any) (map[string]any, error) {
	_ = params
	snapshots := pluginCatalogSnapshots(s.plugin)
	if len(snapshots) == 0 {
		return map[string]any{"items": []map[string]any{}, "metrics": []map[string]any{}, "events": []map[string]any{}}, nil
	}
	runtimes := pluginSnapshotRuntimes(snapshots)
	metrics := pluginSnapshotMetrics(snapshots)
	events := pluginSnapshotEvents(snapshots)
	return map[string]any{
		"items":   pluginRuntimeItems(runtimes),
		"metrics": pluginMetricItems(metrics),
		"events":  pluginEventItems(events),
	}, nil
}

// SecuritySummaryGet handles `agent.security.summary.get`.
func (s *Service) SecuritySummaryGet() (map[string]any, error) {
	_, runtimePendingTotal := s.runEngine.PendingApprovalRequests(20, 0)
	queryViews := newTaskQueryViews(s)
	unfinishedTasks := queryViews.tasks("unfinished", "updated_at", "desc")
	finishedTasks := queryViews.tasks("finished", "finished_at", "desc")
	pendingTotal := mergedPendingApprovalTotal(unfinishedTasks, runtimePendingTotal)
	allTasks := append(append([]runengine.TaskRecord{}, unfinishedTasks...), finishedTasks...)
	modelCredentials := modelCredentialSettings(s.runEngine.Settings())
	latestRestorePoint := latestRestorePointFromTasks(allTasks)
	if latestRestorePoint == nil {
		latestRestorePoint = s.latestRestorePointFromStorage("")
	}
	return map[string]any{
		"summary": map[string]any{
			"security_status":        aggregateSecurityStatus(allTasks, pendingTotal),
			"pending_authorizations": pendingTotal,
			"latest_restore_point":   latestRestorePoint,
			"token_cost_summary":     aggregateTokenCostSummary(unfinishedTasks, finishedTasks, boolValue(modelCredentials, "budget_auto_downgrade", true)),
		},
	}, nil
}

func (s *Service) pluginRuntimeSummary() map[string]any {
	snapshots := pluginCatalogSnapshots(s.plugin)
	if len(snapshots) == 0 {
		return map[string]any{
			"total":       0,
			"healthy":     0,
			"failed":      0,
			"unavailable": 0,
		}
	}
	runtimes := pluginSnapshotRuntimes(snapshots)
	summary := map[string]any{
		"total":       len(runtimes),
		"healthy":     0,
		"failed":      0,
		"unavailable": 0,
	}
	for _, runtime := range runtimes {
		switch runtime.Health {
		case plugin.RuntimeHealthHealthy:
			summary["healthy"] = intValue(summary, "healthy", 0) + 1
		case plugin.RuntimeHealthFailed:
			summary["failed"] = intValue(summary, "failed", 0) + 1
		case plugin.RuntimeHealthUnavailable:
			summary["unavailable"] = intValue(summary, "unavailable", 0) + 1
		}
	}
	return summary
}

func pluginRuntimeItems(items []plugin.RuntimeState) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		entry := map[string]any{
			"name":         item.Name,
			"kind":         item.Kind,
			"status":       item.Status,
			"transport":    item.Transport,
			"health":       item.Health,
			"last_seen_at": item.LastSeenAt,
			"last_error":   item.LastError,
			"capabilities": append([]string(nil), item.Capabilities...),
		}
		if item.Manifest != nil {
			entry["manifest"] = map[string]any{
				"plugin_id":    item.Manifest.PluginID,
				"name":         item.Manifest.Name,
				"version":      item.Manifest.Version,
				"entry":        item.Manifest.Entry,
				"source":       item.Manifest.Source,
				"capabilities": append([]string(nil), item.Manifest.Capabilities...),
				"permissions":  append([]string(nil), item.Manifest.Permissions...),
			}
		}
		result = append(result, entry)
	}
	return result
}

func pluginMetricItems(items []plugin.MetricSnapshot) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"name":            item.Name,
			"kind":            item.Kind,
			"start_count":     item.StartCount,
			"success_count":   item.SuccessCount,
			"failure_count":   item.FailureCount,
			"last_started_at": item.LastStartedAt,
			"last_failed_at":  item.LastFailedAt,
			"last_seen_at":    item.LastSeenAt,
		})
	}
	return result
}

func pluginEventItems(items []plugin.RuntimeEvent) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"name":       item.Name,
			"kind":       item.Kind,
			"event_type": item.EventType,
			"payload":    cloneMap(item.Payload),
			"created_at": item.CreatedAt,
		})
	}
	return result
}

func (s *Service) executeScreenAnalysisAfterApproval(task runengine.TaskRecord, pendingExecution map[string]any) (runengine.TaskRecord, map[string]any, map[string]any, error) {
	if s.executor == nil || s.executor.ScreenClient() == nil {
		failedTask, failureBubble := s.failExecutionTask(task, map[string]any{"name": "screen_analyze"}, execution.Result{}, tools.ErrScreenCaptureNotSupported)
		return failedTask, failureBubble, nil, nil
	}
	screenClient := s.executor.ScreenClient()
	cleanupExpiredScreenTemps(screenClient, "expired_session_scan", time.Now().UTC())
	captureMode := screenCaptureModeFromArguments(pendingExecution)
	source := firstNonEmptyString(stringValue(pendingExecution, "source", ""), "screen_capture")
	screenSession, err := screenClient.StartSession(context.Background(), tools.ScreenSessionStartInput{
		SessionID:   task.SessionID,
		TaskID:      task.TaskID,
		RunID:       task.RunID,
		Source:      source,
		CaptureMode: captureMode,
	})
	if err != nil {
		failedTask, failureBubble := s.failExecutionTask(task, map[string]any{"name": "screen_analyze"}, execution.Result{}, err)
		return failedTask, failureBubble, nil, nil
	}
	candidate, err := captureScreenCandidateAfterApproval(screenClient, screenSession.ScreenSessionID, task, pendingExecution, captureMode)
	if err != nil {
		expireAndCleanupScreenSession(screenClient, screenSession.ScreenSessionID, "capture_failed")
		failedTask, failureBubble := s.failExecutionTask(task, map[string]any{"name": "screen_analyze"}, execution.Result{}, err)
		return failedTask, failureBubble, nil, nil
	}
	execIntent := map[string]any{
		"name": "screen_analyze_candidate",
		"arguments": map[string]any{
			"task_id":           task.TaskID,
			"run_id":            task.RunID,
			"screen_session_id": screenSession.ScreenSessionID,
			"frame_id":          candidate.FrameID,
			"path":              candidate.Path,
			"capture_mode":      string(candidate.CaptureMode),
			"source":            candidate.Source,
			"captured_at":       candidate.CapturedAt.UTC().Format(time.RFC3339),
			"retention_policy":  string(candidate.RetentionPolicy),
			"language":          stringValue(pendingExecution, "language", "eng"),
			"evidence_role":     stringValue(pendingExecution, "evidence_role", "error_evidence"),
			"target_object":     stringValue(pendingExecution, "target_object", "current_screen"),
		},
	}
	updatedTask, bubble, deliveryResult, _, err := s.executeTask(task, snapshotFromTask(task), execIntent)
	if err != nil {
		expireAndCleanupScreenSession(screenClient, screenSession.ScreenSessionID, "analysis_failed")
		return runengine.TaskRecord{}, nil, nil, err
	}
	// Successful analyses stop the session so stale authorizations do not linger.
	// Failed terminal attempts still expire and clean temp session outputs because
	// no durable artifact handoff completed for that branch.
	if updatedTask.Status == "completed" {
		stopScreenSession(screenClient, screenSession.ScreenSessionID, "analysis_completed")
		cleanupSuccessfulScreenSession(screenClient, screenSession.ScreenSessionID, candidate.Path)
	} else if taskIsTerminal(updatedTask.Status) {
		expireAndCleanupScreenSession(screenClient, screenSession.ScreenSessionID, "analysis_failed")
	}
	return updatedTask, bubble, deliveryResult, nil
}

// captureScreenCandidateAfterApproval keeps the controlled screen entry on one
// orchestrator path while still selecting the owner-5 capture primitive that
// matches the approved screen analysis mode.
func captureScreenCandidateAfterApproval(screenClient tools.ScreenCaptureClient, screenSessionID string, task runengine.TaskRecord, pendingExecution map[string]any, captureMode tools.ScreenCaptureMode) (tools.ScreenFrameCandidate, error) {
	input := tools.ScreenCaptureInput{
		ScreenSessionID: screenSessionID,
		TaskID:          task.TaskID,
		RunID:           task.RunID,
		CaptureMode:     captureMode,
		Source:          firstNonEmptyString(stringValue(pendingExecution, "source", ""), "screen_capture"),
		SourcePath:      stringValue(pendingExecution, "source_path", ""),
	}
	switch captureMode {
	case tools.ScreenCaptureModeKeyframe:
		result, err := screenClient.CaptureKeyframe(context.Background(), input)
		if err != nil {
			return tools.ScreenFrameCandidate{}, err
		}
		return result.Candidate, nil
	default:
		return screenClient.CaptureScreenshot(context.Background(), input)
	}
}

func stopScreenSession(screenClient tools.ScreenCaptureClient, screenSessionID, reason string) {
	if screenClient == nil || strings.TrimSpace(screenSessionID) == "" {
		return
	}
	_, _ = screenClient.StopSession(context.Background(), screenSessionID, reason)
}

// cleanupSuccessfulScreenSession only clears the tracked capture file that the
// screen client still owns after execution has already promoted durable
// artifacts. Deferred execution cleanup plans keep managing any extra temp clip
// derivatives, so this path must not recursively wipe the whole session dir.
func cleanupSuccessfulScreenSession(screenClient tools.ScreenCaptureClient, screenSessionID, capturePath string) {
	if screenClient == nil || strings.TrimSpace(screenSessionID) == "" || strings.TrimSpace(capturePath) == "" {
		return
	}
	_, _ = screenClient.CleanupSessionArtifacts(context.Background(), tools.ScreenCleanupInput{
		ScreenSessionID: screenSessionID,
		Reason:          "analysis_completed",
		Paths:           []string{capturePath},
	})
}

// expireAndCleanupScreenSession keeps failed screen-analysis attempts from
// leaving temporary session state behind when no durable artifact is produced.
func expireAndCleanupScreenSession(screenClient tools.ScreenCaptureClient, screenSessionID, reason string) {
	if screenClient == nil || strings.TrimSpace(screenSessionID) == "" {
		return
	}
	_, _ = screenClient.ExpireSession(context.Background(), screenSessionID, reason)
	_, _ = screenClient.CleanupSessionArtifacts(context.Background(), tools.ScreenCleanupInput{
		ScreenSessionID: screenSessionID,
		Reason:          reason,
	})
}

// cleanupExpiredScreenTemps keeps new screen-analysis executions from piling up
// abandoned temp outputs left behind by older expired sessions.
func cleanupExpiredScreenTemps(screenClient tools.ScreenCaptureClient, reason string, expiredBefore time.Time) {
	if screenClient == nil {
		return
	}
	if expiredBefore.IsZero() {
		expiredBefore = time.Now().UTC()
	}
	_, _ = screenClient.CleanupExpiredScreenTemps(context.Background(), tools.ScreenCleanupInput{
		Reason:        reason,
		ExpiredBefore: expiredBefore.UTC(),
	})
}

// defaultIntentMap creates a minimal default intent payload for notepad
// conversions.
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

func (s *Service) materializeMemoryReadReferences(taskID, runID string, snapshot contextsvc.TaskContextSnapshot) ([]map[string]any, []memory.RetrievalHit, error) {
	if s.memory == nil {
		return nil, nil, memory.ErrStoreNotConfigured
	}
	hits, err := s.memory.Search(context.Background(), memory.RetrievalQuery{
		TaskID: taskID,
		RunID:  runID,
		Query:  memoryQueryFromSnapshot(snapshot),
		Limit:  memory.DefaultSearchLimit,
	})
	if err != nil {
		return nil, nil, err
	}
	persistedHits := cloneRetrievalHitsForTask(taskID, runID, hits)
	if err := s.memory.WriteRetrievalHits(context.Background(), persistedHits); err != nil {
		return nil, nil, err
	}
	return mirrorReferencesFromRetrievalHits(persistedHits), persistedHits, nil
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

// attachMemoryReadPlans registers the retrieval plans attached at task start or
// confirmation time. Read plans are persisted before execution so later mirror,
// debug, or storage-backed views can explain what memory lookup the task was
// supposed to perform even if execution changes or the process restarts.
func (s *Service) attachMemoryReadPlans(taskID, runID string, snapshot contextsvc.TaskContextSnapshot, taskIntent map[string]any) {
	readPlans := buildMemoryReadPlans(s.memory, taskID, runID, snapshot, taskIntent, nil)
	_, _ = s.runEngine.SetMemoryPlans(taskID, readPlans, nil)
	references, hits, err := s.materializeMemoryReadReferences(taskID, runID, snapshot)
	if err == nil {
		_, _ = s.runEngine.SetMemoryPlans(taskID, buildMemoryReadPlans(s.memory, taskID, runID, snapshot, taskIntent, hits), nil)
	}
	s.syncTaskReadMirrorReferences(taskID, references, err)
}

func buildMemoryReadPlans(memoryService *memory.Service, taskID, runID string, snapshot contextsvc.TaskContextSnapshot, taskIntent map[string]any, hits []memory.RetrievalHit) []map[string]any {
	readPlan := map[string]any{
		"kind":           "retrieval",
		"task_id":        taskID,
		"run_id":         runID,
		"query":          memoryQueryFromSnapshot(snapshot),
		"reason":         "任务开始前准备记忆召回",
		"intent_name":    stringValue(taskIntent, "name", "summarize"),
		"selection_text": snapshot.SelectionText,
		"input_text":     snapshot.Text,
		"source_type":    snapshot.Trigger,
	}
	if memoryService != nil {
		readPlan["backend"] = memoryService.RetrievalBackend()
	}
	if contextItems := retrievalContextItems(hits); len(contextItems) > 0 {
		readPlan["retrieval_context"] = contextItems
	}

	return []map[string]any{readPlan}
}

func retrievalContextItems(hits []memory.RetrievalHit) []map[string]any {
	if len(hits) == 0 {
		return nil
	}

	items := make([]map[string]any, 0, len(hits))
	for _, hit := range hits {
		summary := strings.TrimSpace(hit.Summary)
		if summary == "" {
			continue
		}
		items = append(items, map[string]any{
			"memory_id": hit.MemoryID,
			"source":    hit.Source,
			"summary":   summary,
			"score":     hit.Score,
		})
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

// attachPostDeliveryHandoffs registers memory-write and delivery persistence
// handoffs after a task finishes. Keeping these side effects in one post-
// delivery step prevents runtime execution from mixing formal delivery with
// memory persistence details while still leaving a durable handoff trail.
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

// memoryQueryFromSnapshot selects the most representative retrieval query from
// the current context snapshot. The fallback order intentionally prefers direct
// user focus, then file context, then broader perception signals so memory
// lookup stays anchored to what most likely triggered the task.
func memoryQueryFromSnapshot(snapshot contextsvc.TaskContextSnapshot) string {
	for _, value := range []string{snapshot.SelectionText, snapshot.Text, snapshot.ErrorText} {
		if value != "" {
			return truncateText(value, 64)
		}
	}

	if len(snapshot.Files) > 0 {
		return snapshot.Files[0]
	}

	for _, value := range []string{snapshot.VisibleText, snapshot.ScreenSummary, snapshot.PageTitle, snapshot.WindowTitle, snapshot.ClipboardText} {
		if value != "" {
			return truncateText(value, 64)
		}
	}

	return "task_context"
}

// buildMemorySummary creates the short post-task memory summary written after
// delivery completes. It keeps the output compact on purpose because this text
// is later used as durable memory material rather than a full-fidelity trace.
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
		return fmt.Sprintf("任务完成，意图=%s，输入=%s，交付=%s，结果摘要=%s", intentName, truncateText(query, 48), title, truncateText(preview, resultPreviewMaxLength))
	}
	return fmt.Sprintf("任务完成，意图=%s，输入=%s，感知=%s，交付=%s，结果摘要=%s", intentName, truncateText(query, 48), strings.Join(perceptionSummary, ", "), title, truncateText(preview, resultPreviewMaxLength))
}

// dateTimeLayout is the shared timestamp layout exposed by orchestrator RPC
// payloads.
