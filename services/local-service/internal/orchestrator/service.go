// Package orchestrator assembles the owner-4 task-centric backend workflow.
package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/agentloop"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
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

// budgetDowngradeDecision describes one real execution-time downgrade decision
// so orchestrator can apply lighter execution paths instead of treating the
// setting as a display-only summary field.
type budgetDowngradeDecision struct {
	Enabled        bool
	Applied        bool
	TriggerReason  string
	TriggerStage   string
	DegradeActions []string
	Summary        string
	Trace          map[string]any
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

// SubscribeRuntimeNotifications registers a temporary tap for execution-time
// runtime notifications so transports can mirror in-flight loop events without
// waiting for the enclosing RPC response to finish.
func (s *Service) SubscribeRuntimeNotifications(listener func(taskID, method string, params map[string]any)) func() {
	if s == nil || listener == nil {
		return func() {}
	}

	s.runtimeMu.Lock()
	s.runtimeNextID++
	listenerID := s.runtimeNextID
	s.runtimeTaps[listenerID] = listener
	s.runtimeMu.Unlock()

	return func() {
		s.runtimeMu.Lock()
		delete(s.runtimeTaps, listenerID)
		s.runtimeMu.Unlock()
	}
}

// SubscribeTaskStarts registers a temporary tap that reports newly created
// tasks before execution continues, allowing transports to associate follow-on
// runtime notifications with requests that did not yet know their task_id.
func (s *Service) SubscribeTaskStarts(listener func(taskID, sessionID, traceID string)) func() {
	if s == nil || listener == nil {
		return func() {}
	}

	s.runtimeMu.Lock()
	s.runtimeNextID++
	listenerID := s.runtimeNextID
	s.taskStartTaps[listenerID] = listener
	s.runtimeMu.Unlock()

	return func() {
		s.runtimeMu.Lock()
		delete(s.taskStartTaps, listenerID)
		s.runtimeMu.Unlock()
	}
}

func (s *Service) publishRuntimeNotification(taskID, method string, params map[string]any) {
	if s == nil {
		return
	}

	s.runtimeMu.RLock()
	if len(s.runtimeTaps) == 0 {
		s.runtimeMu.RUnlock()
		return
	}
	listeners := make([]func(taskID, method string, params map[string]any), 0, len(s.runtimeTaps))
	for _, listener := range s.runtimeTaps {
		listeners = append(listeners, listener)
	}
	s.runtimeMu.RUnlock()

	for _, listener := range listeners {
		listener(taskID, method, cloneMap(params))
	}
}

func (s *Service) publishTaskStart(taskID, sessionID, traceID string) {
	if s == nil {
		return
	}

	s.runtimeMu.RLock()
	if len(s.taskStartTaps) == 0 {
		s.runtimeMu.RUnlock()
		return
	}
	listeners := make([]func(taskID, sessionID, traceID string), 0, len(s.taskStartTaps))
	for _, listener := range s.taskStartTaps {
		listeners = append(listeners, listener)
	}
	s.runtimeMu.RUnlock()

	for _, listener := range listeners {
		listener(taskID, sessionID, traceID)
	}
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

func (s *Service) resumeQueuedControlledTask(task runengine.TaskRecord) (runengine.TaskRecord, bool, error) {
	if stringValue(task.Intent, "name", "") != "screen_analyze" {
		return task, false, nil
	}
	approvalRequest, pendingExecution, bubble, err := s.buildScreenAnalysisApprovalState(task)
	if err != nil {
		failedTask, _ := s.failExecutionTask(task, map[string]any{"name": "screen_analyze"}, execution.Result{}, err)
		return failedTask, true, nil
	}
	updatedTask, ok := s.runEngine.MarkWaitingApprovalWithPlan(task.TaskID, approvalRequest, pendingExecution, bubble)
	if !ok {
		return runengine.TaskRecord{}, true, ErrTaskNotFound
	}
	if err := s.persistApprovalRequestState(updatedTask.TaskID, approvalRequest, mapValue(pendingExecution, "impact_scope")); err != nil {
		return runengine.TaskRecord{}, true, err
	}
	return updatedTask, true, nil
}

func (s *Service) persistApprovalRequestState(taskID string, approvalRequest map[string]any, impactScope map[string]any) error {
	if s.storage == nil {
		return nil
	}
	if err := s.persistApprovalRequest(taskID, approvalRequest, impactScope); err != nil {
		return fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
	}
	return nil
}

func (s *Service) persistAuthorizationState(task runengine.TaskRecord, authorizationRecord map[string]any) error {
	if s.storage == nil {
		return nil
	}
	if err := s.persistAuthorizationDecision(task, authorizationRecord); err != nil {
		return fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
	}
	return nil
}

func (s *Service) persistApprovalRequest(taskID string, approvalRequest map[string]any, impactScope map[string]any) error {
	if s == nil || s.storage == nil || len(approvalRequest) == 0 {
		return nil
	}
	impactScopeJSON := ""
	if len(impactScope) > 0 {
		if encoded, err := json.Marshal(impactScope); err == nil {
			impactScopeJSON = string(encoded)
		}
	}
	record := storage.ApprovalRequestRecord{
		ApprovalID:      stringValue(approvalRequest, "approval_id", ""),
		TaskID:          firstNonEmptyString(stringValue(approvalRequest, "task_id", ""), taskID),
		OperationName:   stringValue(approvalRequest, "operation_name", ""),
		RiskLevel:       stringValue(approvalRequest, "risk_level", ""),
		TargetObject:    stringValue(approvalRequest, "target_object", ""),
		Reason:          stringValue(approvalRequest, "reason", ""),
		Status:          stringValue(approvalRequest, "status", "pending"),
		ImpactScopeJSON: impactScopeJSON,
		CreatedAt:       stringValue(approvalRequest, "created_at", time.Now().Format(dateTimeLayout)),
		UpdatedAt:       firstNonEmptyString(stringValue(approvalRequest, "updated_at", ""), stringValue(approvalRequest, "created_at", time.Now().Format(dateTimeLayout))),
	}
	return s.storage.ApprovalRequestStore().WriteApprovalRequest(context.Background(), record)
}

func (s *Service) persistAuthorizationDecision(task runengine.TaskRecord, authorizationRecord map[string]any) error {
	if s == nil || s.storage == nil || len(authorizationRecord) == 0 {
		return nil
	}
	approvalID := stringValue(authorizationRecord, "approval_id", "")
	recordID := stringValue(authorizationRecord, "authorization_record_id", "")
	if approvalID != "" {
		recordID = fmt.Sprintf("auth_%s_%d", approvalID, time.Now().UnixNano())
	}
	createdAt := stringValue(authorizationRecord, "created_at", time.Now().Format(dateTimeLayout))
	record := storage.AuthorizationRecordRecord{
		AuthorizationRecordID: recordID,
		TaskID:                firstNonEmptyString(stringValue(authorizationRecord, "task_id", ""), task.TaskID),
		RunID:                 firstNonEmptyString(stringValue(authorizationRecord, "run_id", ""), task.RunID),
		ApprovalID:            approvalID,
		Decision:              stringValue(authorizationRecord, "decision", ""),
		Operator:              stringValue(authorizationRecord, "operator", "user"),
		RememberRule:          boolValue(authorizationRecord, "remember_rule", false),
		CreatedAt:             createdAt,
	}
	decision := record.Decision
	status := "resolved"
	if decision == "deny_once" || decision == "deny_always" {
		status = "denied"
	} else if decision == "allow_once" || decision == "allow_always" {
		status = "approved"
	}
	return s.storage.AuthorizationRecordStore().WriteAuthorizationDecision(context.Background(), record, status, createdAt)
}

func (s *Service) activeApprovalIDForTask(task runengine.TaskRecord) (string, bool) {
	if task.Status != "waiting_auth" || task.CurrentStep != "waiting_authorization" {
		return "", false
	}
	approvalID := strings.TrimSpace(stringValue(task.ApprovalRequest, "approval_id", ""))
	if approvalID == "" {
		return "", false
	}
	return approvalID, true
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

// TaskInspectorConfigGet handles agent.task_inspector.config.get.
func (s *Service) TaskInspectorConfigGet() (map[string]any, error) {
	return inspectorConfigFromSettings(s.runEngine.Settings()), nil
}

// TaskInspectorConfigUpdate handles agent.task_inspector.config.update.
func (s *Service) TaskInspectorConfigUpdate(params map[string]any) (map[string]any, error) {
	settingsPatch := taskAutomationSettingsPatchFromInspectorConfig(params)
	if _, _, _, _, err := s.runEngine.UpdateSettings(settingsPatch); err != nil {
		return nil, err
	}
	effective := inspectorConfigFromSettings(s.runEngine.Settings())
	return map[string]any{
		"updated":          true,
		"effective_config": effective,
	}, nil
}

// TaskInspectorRun handles agent.task_inspector.run and returns the inspection
// summary plus suggestions.
func (s *Service) TaskInspectorRun(params map[string]any) (map[string]any, error) {
	config := inspectorConfigFromSettings(s.runEngine.Settings())
	targetSources := stringSliceValue(params["target_sources"])
	notepadItems, _ := s.runEngine.NotepadItems("", 0, 0)
	unfinishedTasks, _ := s.runEngine.ListTasks("unfinished", "updated_at", "desc", 0, 0)
	finishedTasks, _ := s.runEngine.ListTasks("finished", "finished_at", "desc", 0, 0)

	result, err := s.inspector.Run(taskinspector.RunInput{
		Reason:          stringValue(params, "reason", ""),
		TargetSources:   targetSources,
		Config:          config,
		UnfinishedTasks: unfinishedTasks,
		FinishedTasks:   finishedTasks,
		NotepadItems:    notepadItems,
	})
	if err != nil {
		return nil, err
	}
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

// SecurityPendingList handles `agent.security.pending.list` and keeps the
// pending-authorization list aligned with the merged task-centric read model.
func (s *Service) SecurityPendingList(params map[string]any) (map[string]any, error) {
	limit := clampListLimit(intValue(params, "limit", 20))
	offset := clampListOffset(intValue(params, "offset", 0))
	unfinishedTasks := newTaskQueryViews(s).tasks("unfinished", "updated_at", "desc")
	items := pendingApprovalsFromTasks(unfinishedTasks)
	total := len(items)

	// Keep the legacy runtime response as a safety net when runtime approval
	// requests exist but the task snapshots do not expose a structured payload.
	if total == 0 {
		if s.storage != nil {
			storedRecords, storedTotal, err := s.storage.ApprovalRequestStore().ListPendingApprovalRequests(context.Background(), limit, offset)
			if err == nil && storedTotal > 0 {
				items = approvalRequestRecordsToItems(storedRecords)
				total = storedTotal
			} else {
				runtimeItems, runtimeTotal := s.runEngine.PendingApprovalRequests(limit, offset)
				items = runtimeItems
				total = runtimeTotal
			}
		} else {
			runtimeItems, runtimeTotal := s.runEngine.PendingApprovalRequests(limit, offset)
			items = runtimeItems
			total = runtimeTotal
		}
	} else if offset >= total {
		items = []map[string]any{}
	} else {
		end := offset + limit
		if end > total {
			end = total
		}
		items = items[offset:end]
	}

	return map[string]any{
		"items": items,
		"page":  pageMap(limit, offset, total),
	}, nil
}

// SecurityAuditList handles agent.security.audit.list.
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
	runIDFilter := ""
	task := runengine.TaskRecord{}
	if loadedTask, ok := formalReadTask(taskID, s.runEngine, s.taskDetailFromStorage); ok {
		task = loadedTask
		runIDFilter = taskAttemptRunIDFilter(task)
	}
	records, total, err := s.storage.AuditStore().ListAuditRecords(context.Background(), taskID, runIDFilter, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
	}
	if total == 0 && runIDFilter != "" && len(task.AuditRecords) > 0 {
		items := paginateTaskAuditItems(task.AuditRecords, limit, offset)
		return map[string]any{
			"items": items,
			"page":  pageMap(limit, offset, len(task.AuditRecords)),
		}, nil
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

func paginateTaskAuditItems(items []map[string]any, limit, offset int) []map[string]any {
	if len(items) == 0 || offset >= len(items) {
		return []map[string]any{}
	}
	end := len(items)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return cloneMapSlice(items[offset:end])
}

// SecurityRestorePointsList handles agent.security.restore_points.list.
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

// SecurityRestoreApply handles agent.security.restore.apply.
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
		persistedTask, found := s.taskDetailFromStorage(resolvedTaskID)
		if !found {
			return nil, ErrTaskNotFound
		}
		task = s.runEngine.HydrateTaskFromStorage(persistedTask)
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
	if err := s.persistApprovalRequestState(updatedTask.TaskID, approvalRequest, assessment.ImpactScope); err != nil {
		return nil, err
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
	auditRecord := s.writeRestoreAuditRecord(updatedTask.TaskID, updatedTask.RunID, point, applied, bubbleText)
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

// PendingNotifications returns the buffered notification list for a task
// without consuming it. Debug transports use this read-only path when they need
// to inspect pending events but must not disturb the ordered replay pipeline.
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

// DrainNotifications returns and clears the buffered notification list for a
// task. The orchestrator exposes this explicit destructive read so transports
// can replay notifications exactly once instead of coupling queue semantics to
// ordinary task detail or list reads.
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

// SecurityRespond handles agent.security.respond. It is the single resume
// entrypoint for risk-gated tasks, so it must translate allow/deny decisions
// into runtime state changes, delivery continuation, impact scope reporting,
// and audit data in one place instead of letting transports or callers stitch
// those pieces together inconsistently.
func (s *Service) SecurityRespond(params map[string]any) (map[string]any, error) {
	taskID := stringValue(params, "task_id", "")
	task, ok := s.runEngine.GetTask(taskID)
	if !ok {
		return nil, ErrTaskNotFound
	}
	approvalID, ok := s.activeApprovalIDForTask(task)
	if !ok {
		return nil, ErrTaskStatusInvalid
	}

	decision := stringValue(params, "decision", "allow_once")
	rememberRule := boolValue(params, "remember_rule", false)
	authorizationRecord := map[string]any{
		"authorization_record_id": fmt.Sprintf("auth_%s_%d", task.TaskID, time.Now().UnixNano()),
		"task_id":                 task.TaskID,
		"run_id":                  task.RunID,
		"approval_id":             approvalID,
		"decision":                decision,
		"remember_rule":           rememberRule,
		"operator":                "user",
		"created_at":              time.Now().Format(dateTimeLayout),
	}
	if err := s.persistAuthorizationState(task, authorizationRecord); err != nil {
		return nil, err
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
	if stringValue(pendingExecution, "kind", "") == "screen_analysis" {
		updatedTask, bubble, deliveryResult, err := s.executeScreenAnalysisAfterApproval(processingTask, pendingExecution)
		if err != nil {
			return nil, err
		}
		if updatedTask.Status == "completed" {
			updatedTask, _ = s.runEngine.ResolveAuthorization(task.TaskID, authorizationRecord, impactScope)
		}
		if taskIsTerminal(updatedTask.Status) {
			if queueErr := s.drainSessionQueue(updatedTask.SessionID); queueErr != nil {
				return nil, queueErr
			}
		}
		return map[string]any{
			"authorization_record": authorizationRecord,
			"task":                 taskMap(updatedTask),
			"bubble_message":       bubble,
			"impact_scope":         impactScope,
			"delivery_result":      deliveryResult,
		}, nil
	}

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

func inspectorConfigFromSettings(settings map[string]any) map[string]any {
	taskAutomation := cloneMap(mapValue(normalizeSettingsSnapshot(settings), "task_automation"))
	if taskAutomation == nil {
		taskAutomation = map[string]any{}
	}
	return map[string]any{
		"task_sources":           inspectorTaskSourcesFromSettings(taskAutomation["task_sources"]),
		"inspection_interval":    cloneMap(mapValue(taskAutomation, "inspection_interval")),
		"inspect_on_file_change": boolValue(taskAutomation, "inspect_on_file_change", true),
		"inspect_on_startup":     boolValue(taskAutomation, "inspect_on_startup", true),
		"remind_before_deadline": boolValue(taskAutomation, "remind_before_deadline", true),
		"remind_when_stale":      boolValue(taskAutomation, "remind_when_stale", false),
	}
}

// inspectorTaskSourcesFromSettings keeps compatibility RPCs aligned with the
// formal task_automation snapshot shape while preserving workspace-relative
// sources instead of eagerly migrating them to runtime absolute paths.
func inspectorTaskSourcesFromSettings(rawValue any) []string {
	sources, recognized := optionalStringSliceValue(rawValue)
	if recognized {
		result := make([]string, 0, len(sources))
		for _, source := range sources {
			result = append(result, presentInspectorTaskSource(source))
		}
		return result
	}
	return stringSliceValue(rawValue)
}

// presentInspectorTaskSource maps persisted runtime-absolute task sources back to
// the compatibility RPC shape expected by desktop inspector settings so the UI
// continues to reason about workspace-formal paths instead of host-specific
// runtime locations.
func presentInspectorTaskSource(source string) string {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return ""
	}
	if !filepath.IsAbs(trimmed) {
		return trimmed
	}
	cleanSource := filepath.Clean(trimmed)
	workspaceRoot := filepath.Clean(serviceconfig.DefaultWorkspaceRoot())
	if relative, ok := relativizePathWithinRoot(cleanSource, workspaceRoot); ok {
		if relative == "" {
			return "workspace"
		}
		return filepath.ToSlash(path.Join("workspace", filepath.ToSlash(relative)))
	}
	runtimeRoot := filepath.Clean(serviceconfig.DefaultRuntimeRoot())
	if relative, ok := relativizePathWithinRoot(cleanSource, runtimeRoot); ok {
		if relative == "" {
			return "."
		}
		return filepath.ToSlash(relative)
	}
	return filepath.ToSlash(cleanSource)
}

func relativizePathWithinRoot(candidate, root string) (string, bool) {
	if root == "" {
		return "", false
	}
	if candidate == root {
		return "", true
	}
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return "", false
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	return relative, true
}

func taskAutomationSettingsPatchFromInspectorConfig(params map[string]any) map[string]any {
	patch := map[string]any{}
	if rawSources, ok := params["task_sources"]; ok {
		if sources, recognized := optionalStringSliceValue(rawSources); recognized {
			patch["task_sources"] = sources
		}
	}
	if interval := cloneMap(mapValue(params, "inspection_interval")); len(interval) > 0 {
		patch["inspection_interval"] = interval
	}
	for _, key := range []string{"inspect_on_file_change", "inspect_on_startup", "remind_before_deadline", "remind_when_stale"} {
		if value, ok := params[key].(bool); ok {
			patch[key] = value
		}
	}
	if len(patch) == 0 {
		return map[string]any{}
	}
	return map[string]any{"task_automation": patch}
}

// optionalStringSliceValue preserves the difference between an omitted field and
// an explicitly empty list so compatibility RPCs can clear task sources without
// leaving stale workspace scan roots behind.
func optionalStringSliceValue(rawValue any) ([]string, bool) {
	switch values := rawValue.(type) {
	case []string:
		result := make([]string, 0, len(values))
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				continue
			}
			result = append(result, value)
		}
		return result, true
	case []any:
		result := make([]string, 0, len(values))
		for _, rawItem := range values {
			item, ok := rawItem.(string)
			if !ok || strings.TrimSpace(item) == "" {
				continue
			}
			result = append(result, item)
		}
		return result, true
	default:
		return nil, false
	}
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
	items, _, err := s.storage.AuditStore().ListAuditRecords(context.Background(), taskID, "", 1, 0)
	if err != nil || len(items) == 0 {
		return nil
	}
	return normalizeTaskDetailAuditRecord(taskID, items[0].Map())
}

func (s *Service) loadAttemptAuditRecordsFromStorage(task runengine.TaskRecord, limit, offset int) []map[string]any {
	if s == nil || s.storage == nil || s.storage.AuditStore() == nil || strings.TrimSpace(task.TaskID) == "" {
		return nil
	}
	items, _, err := s.storage.AuditStore().ListAuditRecords(context.Background(), task.TaskID, taskAttemptRunIDFilter(task), limit, offset)
	if err != nil {
		return nil
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, item.Map())
	}
	return result
}

func (s *Service) latestToolCallFromStorage(taskID, runID string) map[string]any {
	if s == nil || s.storage == nil || s.storage.ToolCallSink() == nil || strings.TrimSpace(taskID) == "" {
		return nil
	}
	items, _, err := s.storage.ToolCallStore().ListToolCalls(context.Background(), taskID, runID, 1, 0)
	if err != nil || len(items) == 0 {
		return nil
	}
	item := items[0]
	return map[string]any{
		"tool_call_id": item.ToolCallID,
		"run_id":       item.RunID,
		"task_id":      item.TaskID,
		"step_id":      item.StepID,
		"tool_name":    item.ToolName,
		"status":       item.Status,
		"input":        cloneMap(item.Input),
		"output":       cloneMap(item.Output),
		"error_code":   item.ErrorCode,
		"duration_ms":  item.DurationMS,
	}
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

func (s *Service) writeRestoreAuditRecord(taskID, runID string, point checkpoint.RecoveryPoint, applied bool, summary string) map[string]any {
	if s.audit == nil {
		return nil
	}
	input := audit.RecordInput{
		TaskID:  taskID,
		RunID:   runID,
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

func normalizeTaskDetailAuthorizationRecord(taskID string, authorizationRecord map[string]any) map[string]any {
	if len(authorizationRecord) == 0 {
		return nil
	}

	recordID := strings.TrimSpace(stringValue(authorizationRecord, "authorization_record_id", ""))
	recordTaskID := strings.TrimSpace(stringValue(authorizationRecord, "task_id", ""))
	approvalID := strings.TrimSpace(stringValue(authorizationRecord, "approval_id", ""))
	decision := normalizeTaskDetailAuthorizationDecision(stringValue(authorizationRecord, "decision", ""))
	operator := strings.TrimSpace(stringValue(authorizationRecord, "operator", ""))
	createdAt := strings.TrimSpace(stringValue(authorizationRecord, "created_at", ""))
	if recordID == "" || recordTaskID != taskID || approvalID == "" || decision == "" || operator == "" || createdAt == "" {
		return nil
	}

	return map[string]any{
		"authorization_record_id": recordID,
		"task_id":                 recordTaskID,
		"approval_id":             approvalID,
		"decision":                decision,
		"remember_rule":           boolValue(authorizationRecord, "remember_rule", false),
		"operator":                operator,
		"created_at":              createdAt,
	}
}

func normalizeTaskDetailAuthorizationDecision(decision string) string {
	switch strings.TrimSpace(decision) {
	case "allow_once", "allow_always":
		return "allow_once"
	case "deny_once", "deny_always":
		return "deny_once"
	default:
		return ""
	}
}

func normalizeTaskDetailAuditRecord(taskID string, auditRecord map[string]any) map[string]any {
	if len(auditRecord) == 0 {
		return nil
	}

	recordID := strings.TrimSpace(firstNonEmptyString(stringValue(auditRecord, "audit_id", ""), stringValue(auditRecord, "audit_record_id", "")))
	recordTaskID := strings.TrimSpace(stringValue(auditRecord, "task_id", ""))
	recordType := strings.TrimSpace(firstNonEmptyString(stringValue(auditRecord, "type", ""), stringValue(auditRecord, "category", "")))
	action := strings.TrimSpace(stringValue(auditRecord, "action", ""))
	summary := strings.TrimSpace(firstNonEmptyString(stringValue(auditRecord, "summary", ""), stringValue(auditRecord, "reason", "")))
	target := strings.TrimSpace(firstNonEmptyString(stringValue(auditRecord, "target", ""), impactScopeTarget(mapValue(auditRecord, "impact_scope"), "")))
	result := strings.TrimSpace(stringValue(auditRecord, "result", ""))
	createdAt := strings.TrimSpace(stringValue(auditRecord, "created_at", ""))
	if recordID == "" || recordTaskID != taskID || recordType == "" || action == "" || summary == "" || target == "" || result == "" || createdAt == "" {
		return nil
	}

	return map[string]any{
		"audit_id":   recordID,
		"task_id":    recordTaskID,
		"type":       recordType,
		"action":     action,
		"summary":    summary,
		"target":     target,
		"result":     result,
		"created_at": createdAt,
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
	trimmedPath := strings.TrimSpace(filePath)
	if trimmedPath == "" {
		return false
	}
	if hasWindowsDriveLetterPrefix(trimmedPath) {
		if !isWindowsStyleAbsolutePath(trimmedPath) {
			return false
		}
	}
	if !filepath.IsAbs(trimmedPath) && !isWindowsStyleAbsolutePath(trimmedPath) {
		if strings.HasPrefix(trimmedPath, "\\") || strings.HasPrefix(trimmedPath, "/") {
			return false
		}
	}
	normalizedPath := strings.Trim(strings.ReplaceAll(filePath, "\\", "/"), "/")
	if normalizedPath == "" {
		return false
	}
	if normalizedPath == "workspace" || strings.HasPrefix(normalizedPath, "workspace/") {
		return true
	}
	if filepath.IsAbs(trimmedPath) || isWindowsStyleAbsolutePath(trimmedPath) {
		cleanRoot := filepath.Clean(strings.TrimSpace(workspaceRoot))
		if cleanRoot == "" {
			return false
		}
		cleanPath := filepath.Clean(trimmedPath)
		rootWithSeparator := cleanRoot + string(filepath.Separator)
		return cleanPath == cleanRoot || strings.HasPrefix(cleanPath, rootWithSeparator)
	}
	cleanRelative := path.Clean(normalizedPath)
	// Runtime temp artifacts remain openable from the desktop host, but governance
	// must not classify them as workspace-contained when computing trust scope.
	if cleanRelative == "temp" || strings.HasPrefix(cleanRelative, "temp/") {
		return false
	}
	return cleanRelative != ".." && !strings.HasPrefix(cleanRelative, "../")
}

func hasWindowsDriveLetterPrefix(value string) bool {
	if len(value) < 2 {
		return false
	}
	letter := value[0]
	return ((letter >= 'A' && letter <= 'Z') || (letter >= 'a' && letter <= 'z')) && value[1] == ':'
}

func isWindowsStyleAbsolutePath(value string) bool {
	return hasWindowsDriveLetterPrefix(value) && len(value) >= 3 && (value[2] == '\\' || value[2] == '/')
}

func hasOverwriteOrDeleteRisk(taskIntent map[string]any) bool {
	if stringValue(taskIntent, "name", "") == "write_file" {
		return true
	}
	arguments := mapValue(taskIntent, "arguments")
	return boolValue(arguments, "overwrite", false) || boolValue(arguments, "delete", false)
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

// buildApprovalRequest creates the normalized approval_request payload. The
// object must already be protocol-facing here because it is persisted, replayed
// to transports, and later echoed back through agent.security.respond.
func buildApprovalRequest(taskID string, taskIntent map[string]any, assessment execution.GovernanceAssessment) map[string]any {
	arguments := mapValue(taskIntent, "arguments")
	targetObject := firstNonEmptyString(assessment.TargetObject, stringValue(arguments, "target_path", "workspace_document"))
	if targetObject == "" {
		targetObject = "workspace_document"
	}

	return map[string]any{
		"approval_id":    fmt.Sprintf("appr_%s_%d", taskID, time.Now().UnixNano()),
		"task_id":        taskID,
		"operation_name": firstNonEmptyString(assessment.OperationName, firstNonEmptyString(stringValue(taskIntent, "name", ""), "write_file")),
		"risk_level":     firstNonEmptyString(assessment.RiskLevel, "red"),
		"target_object":  targetObject,
		"reason":         firstNonEmptyString(assessment.Reason, "policy_requires_authorization"),
		"status":         "pending",
		"created_at":     time.Now().Format(dateTimeLayout),
	}
}

// buildImpactScope derives the minimal impact summary used by authorization
// results and the security views. It intentionally normalizes files around the
// workspace root so policy, audit, and restore flows all reason about one scope
// shape instead of transport- or tool-specific paths.
func (s *Service) buildImpactScope(task runengine.TaskRecord, pendingExecution map[string]any) map[string]any {
	if impactScope, ok := pendingExecution["impact_scope"].(map[string]any); ok && len(impactScope) > 0 {
		return cloneMap(impactScope)
	}
	files := deriveImpactScopeFiles(task, pendingExecution, s.delivery)
	workspacePath := currentRuntimeWorkspaceRoot(s.executor)
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
		SourceType:   task.SourceType,
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
	updatedTask := runengine.TaskRecord{}
	changed := false
	if s.isPreparedRestartAttempt(task) {
		updatedTask, changed = s.runEngine.MarkPreparedTaskWaitingApprovalWithPlan(task, approvalRequest, pendingExecution, bubble)
	} else {
		updatedTask, changed = s.runEngine.MarkWaitingApprovalWithPlan(task.TaskID, approvalRequest, pendingExecution, bubble)
	}
	if !changed {
		return task, nil, false, ErrTaskNotFound
	}
	if err := s.persistApprovalRequestState(updatedTask.TaskID, approvalRequest, assessment.ImpactScope); err != nil {
		return task, nil, false, err
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
	updatedTask := runengine.TaskRecord{}
	ok := false
	if s.isPreparedRestartAttempt(task) {
		updatedTask, ok = s.runEngine.BlockPreparedTaskByPolicy(task, assessment.RiskLevel, bubbleText, assessment.ImpactScope, bubble)
	} else {
		updatedTask, ok = s.runEngine.BlockTaskByPolicy(task.TaskID, assessment.RiskLevel, bubbleText, assessment.ImpactScope, bubble)
	}
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
		RunID:   runID,
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
		RunID:   runID,
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

// evaluateBudgetAutoDowngrade decides whether the visible budget setting should
// become a real execution downgrade before the task reaches model/tool work.
// The first P1 slice keeps the trigger set intentionally small and auditable:
// provider/API-key unavailability and token/cost pressure on the current task.
func (s *Service) evaluateBudgetAutoDowngrade(task runengine.TaskRecord, taskIntent map[string]any) budgetDowngradeDecision {
	modelSettings := modelSettingsSection(s.runEngine.Settings())
	modelCredentials := modelCredentialSettings(s.runEngine.Settings())
	if !boolValue(modelCredentials, "budget_auto_downgrade", true) {
		return budgetDowngradeDecision{}
	}
	policy := budgetPolicySettings(modelCredentials)
	decision := budgetDowngradeDecision{
		Enabled:      true,
		TriggerStage: "execution_preflight",
	}
	provider := providerFromSettings(modelSettings, model.OpenAIResponsesProvider)
	if !supportsBudgetProvider(provider) {
		decision.Applied = true
		decision.TriggerReason = "provider_unavailable"
		decision.DegradeActions = budgetDegradeActionsForReason(policy, "provider_unavailable")
		decision.Summary = "预算降级已生效：当前模型提供方不可用，任务改走轻量交付路径。"
		decision.Trace = buildBudgetDecisionTrace(task, decision, policy, 0, 0)
		return decision
	}
	failureSignals := recentBudgetFailureCount(task)
	if failureSignals >= intValue(policy, "failure_signal_window", 2) {
		decision.Applied = true
		decision.TriggerReason = "failure_pressure"
		decision.DegradeActions = budgetDegradeActionsForReason(policy, "failure_pressure")
		decision.Summary = "预算降级已生效：最近出现模型/提供方失败，任务改走轻量保守执行路径。"
		decision.Trace = buildBudgetDecisionTrace(task, decision, policy, failureSignals, 0)
		return decision
	}
	totalTokens := intValueFromAny(task.TokenUsage["total_tokens"])
	estimatedCost := floatValueFromAny(task.TokenUsage["estimated_cost"])
	if totalTokens >= intValue(policy, "token_pressure_threshold", 64) || estimatedCost >= floatValueFromAny(policy["cost_pressure_threshold"]) {
		decision.Applied = true
		decision.TriggerReason = "budget_pressure"
		decision.DegradeActions = budgetDegradeActionsForReason(policy, "budget_pressure")
		decision.Summary = "预算降级已生效：当前任务命中 token/成本压力，改为轻量交付并压缩上下文。"
		decision.Trace = buildBudgetDecisionTrace(task, decision, policy, failureSignals, map[string]any{"total_tokens": totalTokens, "estimated_cost": estimatedCost})
	}
	return decision
}

// applyBudgetAutoDowngrade mutates the execution request shape so the downgrade
// decision changes the real path instead of only updating settings summaries.
func (s *Service) applyBudgetAutoDowngrade(task runengine.TaskRecord, snapshot contextsvc.TaskContextSnapshot, taskIntent map[string]any, decision budgetDowngradeDecision) (runengine.TaskRecord, contextsvc.TaskContextSnapshot, map[string]any) {
	if !decision.Applied {
		return task, snapshot, taskIntent
	}
	updatedTask := task
	updatedTask.PreferredDelivery = "bubble"
	updatedTask.FallbackDelivery = "bubble"
	updatedIntent := cloneMap(taskIntent)
	arguments := cloneMap(mapValue(updatedIntent, "arguments"))
	if len(arguments) > 0 {
		if containsString(decision.DegradeActions, "skip_expensive_tools") {
			arguments["disable_tool_calls"] = true
		}
		arguments["budget_auto_downgrade_applied"] = true
		updatedIntent["arguments"] = arguments
	}
	updatedSnapshot := snapshot
	if containsString(decision.DegradeActions, "shrink_context") {
		updatedSnapshot.Text = truncateText(updatedSnapshot.Text, 160)
		updatedSnapshot.SelectionText = truncateText(updatedSnapshot.SelectionText, 160)
	}
	updatedTask.SecuritySummary = mergeBudgetDowngradeSummary(updatedTask.SecuritySummary, decision)
	return updatedTask, updatedSnapshot, updatedIntent
}

func mergeBudgetDowngradeSummary(current map[string]any, decision budgetDowngradeDecision) map[string]any {
	updated := cloneMap(current)
	if updated == nil {
		updated = map[string]any{}
	}
	updated["budget_auto_downgrade_applied"] = decision.Applied
	updated["budget_auto_downgrade_reason"] = decision.TriggerReason
	updated["budget_auto_downgrade_actions"] = append([]string(nil), decision.DegradeActions...)
	updated["budget_auto_downgrade_summary"] = decision.Summary
	updated["budget_auto_downgrade_trace"] = cloneMap(decision.Trace)
	return updated
}

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func supportsBudgetProvider(provider string) bool {
	switch model.CanonicalProviderName(provider) {
	case "", model.OpenAIResponsesProvider:
		return true
	default:
		return false
	}
}

func budgetPolicySettings(modelCredentials map[string]any) map[string]any {
	policy := cloneMap(mapValue(modelCredentials, "budget_policy"))
	if policy == nil {
		policy = map[string]any{}
	}
	if _, ok := policy["planner_retry_budget"]; !ok {
		policy["planner_retry_budget"] = 1
	}
	if _, ok := policy["failure_signal_window"]; !ok {
		policy["failure_signal_window"] = 2
	}
	if _, ok := policy["token_pressure_threshold"]; !ok {
		policy["token_pressure_threshold"] = 64
	}
	if _, ok := policy["cost_pressure_threshold"]; !ok {
		policy["cost_pressure_threshold"] = 0.05
	}
	if _, ok := policy["expensive_tool_categories"]; !ok {
		policy["expensive_tool_categories"] = []string{"command", "browser_mutation", "media_heavy"}
	}
	return policy
}

func budgetDegradeActionsForReason(policy map[string]any, reason string) []string {
	actions := []string{"lightweight_delivery"}
	switch reason {
	case "provider_unavailable", "failure_pressure":
		actions = append(actions, "skip_expensive_tools", "shrink_context")
	case "budget_pressure":
		actions = append(actions, "shrink_context")
	}
	if len(stringSliceValue(policy["expensive_tool_categories"])) > 0 && !containsString(actions, "skip_expensive_tools") && reason != "budget_pressure" {
		actions = append(actions, "skip_expensive_tools")
	}
	return actions
}

func buildBudgetDecisionTrace(task runengine.TaskRecord, decision budgetDowngradeDecision, policy map[string]any, failureSignals int, pressure any) map[string]any {
	return map[string]any{
		"task_id":                   task.TaskID,
		"run_id":                    task.RunID,
		"trigger_reason":            decision.TriggerReason,
		"trigger_stage":             decision.TriggerStage,
		"degrade_actions":           append([]string(nil), decision.DegradeActions...),
		"failure_signal_count":      failureSignals,
		"planner_retry_budget":      intValue(policy, "planner_retry_budget", 1),
		"failure_signal_window":     intValue(policy, "failure_signal_window", 2),
		"token_pressure_threshold":  intValue(policy, "token_pressure_threshold", 64),
		"cost_pressure_threshold":   floatValueFromAny(policy["cost_pressure_threshold"]),
		"expensive_tool_categories": stringSliceValue(policy["expensive_tool_categories"]),
		"pressure":                  pressure,
	}
}

func recentBudgetFailureCount(task runengine.TaskRecord) int {
	count := 0
	for _, record := range task.AuditRecords {
		if stringValue(record, "category", "") != "budget_auto_downgrade" {
			continue
		}
		if stringValue(record, "result", "") != "failed" {
			continue
		}
		count++
	}
	return count
}

func firstNonEmptyString(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

// dateTimeLayout is the shared timestamp layout exposed by orchestrator RPC
// payloads.
func (s *Service) executeTask(task runengine.TaskRecord, snapshot contextsvc.TaskContextSnapshot, taskIntent map[string]any) (runengine.TaskRecord, map[string]any, map[string]any, []map[string]any, error) {
	return s.executeTaskAttempt(task, task, snapshot, taskIntent)
}

// executeTaskAttempt runs the current task state while preserving the previous
// task snapshot for execution segment classification. Restart needs this split:
// the new run must execute, but the executor still needs the old run_id to mark
// the segment as restart instead of initial.
func (s *Service) executeTaskAttempt(previousTask, task runengine.TaskRecord, snapshot contextsvc.TaskContextSnapshot, taskIntent map[string]any) (runengine.TaskRecord, map[string]any, map[string]any, []map[string]any, error) {
	var processingTask runengine.TaskRecord
	ok := false
	if s.isPreparedRestartAttempt(task) {
		processingTask, ok = s.runEngine.BeginPreparedExecution(task, s.activeExecutionStepName(taskIntent), "开始生成正式结果")
	} else {
		processingTask, ok = s.runEngine.BeginExecution(task.TaskID, s.activeExecutionStepName(taskIntent), "开始生成正式结果")
	}
	if !ok {
		return runengine.TaskRecord{}, nil, nil, nil, ErrTaskNotFound
	}
	budgetDecision := s.evaluateBudgetAutoDowngrade(processingTask, taskIntent)
	processingTask, snapshot, taskIntent = s.applyBudgetAutoDowngrade(processingTask, snapshot, taskIntent, budgetDecision)
	if budgetDecision.Applied {
		_, _ = s.runEngine.UpdateSecuritySummary(processingTask.TaskID, processingTask.SecuritySummary)
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
		auditRecords := compactAuditRecords(s.audit.BuildDeliveryAudit(processingTask.TaskID, processingTask.RunID, deliveryResult), s.buildBudgetDowngradeAudit(processingTask, budgetDecision))
		processingTask = s.appendAuditData(processingTask, auditRecords, nil)
		processingTask = s.recordBudgetDowngradeEvent(processingTask, budgetDecision)
		traceCapture, traceErr := s.captureExecutionTrace(processingTask, snapshot, taskIntent, execution.Result{
			Content:        previewTextForDeliveryType(deliveryType),
			DeliveryResult: deliveryResult,
			Artifacts:      artifacts,
		}, nil)
		if traceErr != nil {
			failedTask, failureBubble := s.failExecutionTask(processingTask, taskIntent, execution.Result{}, traceErr)
			return failedTask, failureBubble, nil, nil, nil
		}
		if escalatedTask, escalatedBubble, ok := s.maybeEscalateHumanLoop(processingTask, traceCapture); ok {
			return escalatedTask, escalatedBubble, nil, nil, nil
		}
		updatedTask, ok := s.runEngine.CompleteTask(processingTask.TaskID, deliveryResult, resultBubble, artifacts)
		if !ok {
			return runengine.TaskRecord{}, nil, nil, nil, ErrTaskNotFound
		}
		updatedTask = s.attachFormalCitations(processingTask, updatedTask, nil, nil, deliveryResult, artifacts)
		s.attachPostDeliveryHandoffs(updatedTask.TaskID, updatedTask.RunID, snapshot, taskIntent, deliveryResult, artifacts)
		return updatedTask, resultBubble, deliveryResult, artifacts, nil
	}

	approvedOperation, approvedTargetObject := approvedExecutionFromTask(processingTask)
	executionCtx := context.Background()
	if shouldBoundTaskExecution(processingTask, snapshot, taskIntent, deliveryType) {
		executionTimeout := s.executionTimeout
		if executionTimeout <= 0 {
			executionTimeout = defaultTaskExecutionTimeout
		}
		boundedCtx, cancelExecution := context.WithTimeout(context.Background(), executionTimeout)
		defer cancelExecution()
		executionCtx = boundedCtx
	}

	executionResult, err := s.executor.Execute(executionCtx, execution.Request{
		TaskID:               processingTask.TaskID,
		RunID:                processingTask.RunID,
		SourceType:           processingTask.SourceType,
		Title:                processingTask.Title,
		Intent:               taskIntent,
		AttemptIndex:         executionAttemptIndex(previousTask, processingTask),
		SegmentKind:          executionSegmentKind(previousTask, processingTask),
		Snapshot:             snapshot,
		MemoryReadPlans:      cloneMapSlice(processingTask.MemoryReadPlans),
		SteeringMessages:     append([]string(nil), processingTask.SteeringMessages...),
		DeliveryType:         deliveryType,
		ResultTitle:          resultTitle,
		ApprovalGranted:      processingTask.Authorization != nil,
		ApprovedOperation:    approvedOperation,
		ApprovedTargetObject: approvedTargetObject,
		BudgetDowngrade: map[string]any{
			"enabled":         budgetDecision.Enabled,
			"applied":         budgetDecision.Applied,
			"trigger_reason":  budgetDecision.TriggerReason,
			"trigger_stage":   budgetDecision.TriggerStage,
			"degrade_actions": append([]string(nil), budgetDecision.DegradeActions...),
			"summary":         budgetDecision.Summary,
			"trace":           cloneMap(budgetDecision.Trace),
		},
	})
	processingTask = s.recordExecutionToolCalls(processingTask, executionResult.ToolCalls)
	s.persistExecutionToolCallEvents(processingTask, taskIntent, executionResult.ToolCalls)
	auditDeliveryResult := executionResult.DeliveryResult
	if err != nil {
		auditDeliveryResult = nil
	}
	executionAuditRecords, executionTokenUsage := s.buildExecutionAudit(processingTask, executionResult.ToolCalls, auditDeliveryResult)
	if len(executionResult.BudgetFailure) > 0 {
		executionAuditRecords = append(executionAuditRecords, cloneMap(executionResult.BudgetFailure))
	}
	executionAuditRecords = append(executionAuditRecords, s.buildBudgetDowngradeAudit(processingTask, budgetDecision))
	processingTask = s.appendAuditData(processingTask, executionAuditRecords, executionTokenUsage)
	processingTask = s.recordBudgetDowngradeEvent(processingTask, budgetDecision)
	traceCapture, traceErr := s.captureExecutionTrace(processingTask, snapshot, taskIntent, executionResult, err)
	if traceErr != nil {
		failedTask, failureBubble := s.failExecutionTask(processingTask, taskIntent, executionResult, traceErr)
		return failedTask, failureBubble, nil, nil, nil
	}
	if escalatedTask, escalatedBubble, ok := s.maybeEscalateHumanLoop(processingTask, traceCapture, executionResult); ok {
		return escalatedTask, escalatedBubble, nil, nil, nil
	}
	if err != nil {
		failedTask, failureBubble := s.failExecutionTask(processingTask, taskIntent, executionResult, err)
		return failedTask, failureBubble, nil, nil, nil
	}
	if executionResult.LoopStopReason == string(agentloop.StopReasonNeedUserInput) {
		waitingTask, waitingBubble, ok := s.reopenTaskForUserInput(processingTask, taskIntent, executionResult)
		if !ok {
			return runengine.TaskRecord{}, nil, nil, nil, ErrTaskNotFound
		}
		return waitingTask, waitingBubble, nil, nil, nil
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
	s.persistExecutionDeliveryResult(updatedTask, taskIntent, executionResult.DeliveryResult)
	updatedTask = s.attachFormalCitations(processingTask, updatedTask, executionResult.ToolCalls, executionResult.ToolOutput, executionResult.DeliveryResult, executionArtifacts)
	s.attachPostDeliveryHandoffs(updatedTask.TaskID, updatedTask.RunID, snapshot, taskIntent, executionResult.DeliveryResult, executionArtifacts)
	return updatedTask, resultBubble, executionResult.DeliveryResult, executionArtifacts, nil
}

// shouldBoundTaskExecution limits the outer orchestrator timeout to synchronous
// shell-ball submits that still resolve to bubble delivery. Longer structured
// flows already carry their own internal timeouts and should not inherit the
// short near-field deadline.
func shouldBoundTaskExecution(task runengine.TaskRecord, snapshot contextsvc.TaskContextSnapshot, taskIntent map[string]any, deliveryType string) bool {
	if strings.TrimSpace(stringValue(taskIntent, "name", "")) == "screen_analyze_candidate" {
		return false
	}
	if strings.TrimSpace(deliveryType) != "bubble" {
		return false
	}
	if strings.TrimSpace(snapshot.Trigger) == "hover_text_input" {
		return true
	}
	switch strings.TrimSpace(task.SourceType) {
	case "hover_input", "floating_ball":
		return true
	default:
		return false
	}
}

// reopenTaskForUserInput keeps the current task open when the agent loop stops
// because the user's goal is still underspecified. The same task/session stays
// alive so follow-up input can continue the mainline instead of creating a fake
// completed delivery record.
func (s *Service) reopenTaskForUserInput(task runengine.TaskRecord, taskIntent map[string]any, executionResult execution.Result) (runengine.TaskRecord, map[string]any, bool) {
	clarificationText := firstNonEmptyString(
		firstNonEmptyString(executionResult.BubbleText, stringValue(executionResult.DeliveryResult, "preview_text", "")),
		"请补充你的目标。",
	)
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", clarificationText, task.UpdatedAt.Format(dateTimeLayout))
	updatedTask, ok := s.runEngine.ReopenWaitingInput(task.TaskID, task.Title, taskIntent, bubble)
	return updatedTask, bubble, ok
}

// attachFormalCitations upgrades execution-side citation seeds into protocol-facing
// citation objects so task detail can expose stable evidence references without
// leaking raw tool outputs or worker-only payloads.
func (s *Service) attachFormalCitations(sourceTask runengine.TaskRecord, persistedTask runengine.TaskRecord, toolCalls []tools.ToolCallRecord, toolOutput map[string]any, deliveryResult map[string]any, artifacts []map[string]any) runengine.TaskRecord {
	citations := buildTaskCitations(sourceTask, toolCalls, toolOutput, deliveryResult, artifacts)
	s.persistFormalCitations(persistedTask.TaskID, citations)
	if _, ok := s.runEngine.SetCitations(persistedTask.TaskID, citations); ok {
		if updatedTask, exists := s.runEngine.GetTask(persistedTask.TaskID); exists {
			return updatedTask
		}
	}
	return persistedTask
}

// persistFormalCitations keeps the current first-class citation chain queryable
// even after task_run compatibility snapshots have been compacted away. The
// persisted citation set is intentionally task-scoped replacement today, so a
// restarted attempt publishes its own chain instead of retaining every prior
// attempt's citation history.
func (s *Service) persistFormalCitations(taskID string, citations []map[string]any) {
	if s == nil || s.storage == nil || s.storage.LoopRuntimeStore() == nil || strings.TrimSpace(taskID) == "" {
		return
	}
	records := make([]storage.CitationRecord, 0, len(citations))
	for index, citation := range citations {
		records = append(records, storage.CitationRecord{
			CitationID:      stringValue(citation, "citation_id", ""),
			TaskID:          firstNonEmptyString(stringValue(citation, "task_id", ""), taskID),
			RunID:           stringValue(citation, "run_id", ""),
			SourceType:      stringValue(citation, "source_type", "context"),
			SourceRef:       stringValue(citation, "source_ref", ""),
			Label:           stringValue(citation, "label", ""),
			ArtifactID:      stringValue(citation, "artifact_id", ""),
			ArtifactType:    stringValue(citation, "artifact_type", ""),
			EvidenceRole:    stringValue(citation, "evidence_role", ""),
			ExcerptText:     stringValue(citation, "excerpt_text", ""),
			ScreenSessionID: stringValue(citation, "screen_session_id", ""),
			OrderIndex:      index,
		})
	}
	_ = s.storage.LoopRuntimeStore().ReplaceTaskCitations(context.Background(), taskID, records)
}

func buildTaskCitations(task runengine.TaskRecord, toolCalls []tools.ToolCallRecord, toolOutput map[string]any, deliveryResult map[string]any, artifacts []map[string]any) []map[string]any {
	citations := make([]map[string]any, 0)
	seen := make(map[string]struct{})
	artifactsByID := make(map[string]map[string]any, len(artifacts))
	for _, artifact := range artifacts {
		artifactID := stringValue(artifact, "artifact_id", "")
		if strings.TrimSpace(artifactID) != "" {
			artifactsByID[artifactID] = cloneMap(artifact)
		}
	}
	for _, call := range toolCalls {
		seed := mapValue(call.Output, "citation_seed")
		if len(seed) == 0 {
			continue
		}
		citation := citationFromSeed(task, seed, artifactsByID, deliveryResult)
		if len(citation) == 0 {
			continue
		}
		citationID := stringValue(citation, "citation_id", "")
		if _, ok := seen[citationID]; ok {
			continue
		}
		seen[citationID] = struct{}{}
		citations = append(citations, citation)
	}
	if seed := mapValue(toolOutput, "citation_seed"); len(seed) > 0 {
		citation := citationFromSeed(task, seed, artifactsByID, deliveryResult)
		if len(citation) > 0 {
			citationID := stringValue(citation, "citation_id", "")
			if _, ok := seen[citationID]; !ok {
				seen[citationID] = struct{}{}
				citations = append(citations, citation)
			}
		}
	}
	if latestSeed := mapValue(task.LatestToolCall, "output"); len(latestSeed) > 0 {
		seed := mapValue(latestSeed, "citation_seed")
		if len(seed) > 0 {
			citation := citationFromSeed(task, seed, artifactsByID, deliveryResult)
			if len(citation) > 0 {
				citationID := stringValue(citation, "citation_id", "")
				if _, ok := seen[citationID]; !ok {
					citations = append(citations, citation)
				}
			}
		}
	}
	return citations
}

func citationFromSeed(task runengine.TaskRecord, seed map[string]any, artifactsByID map[string]map[string]any, deliveryResult map[string]any) map[string]any {
	artifactID := stringValue(seed, "artifact_id", "")
	artifactType := stringValue(seed, "artifact_type", "")
	evidenceRole := stringValue(seed, "evidence_role", "")
	ocrExcerpt := stringValue(seed, "ocr_excerpt", "")
	sourceRef := firstNonEmptyString(artifactID, stringValue(seed, "screen_session_id", ""))
	if strings.TrimSpace(sourceRef) == "" {
		sourceRef = stringValue(mapValue(deliveryResult, "payload"), "task_id", task.TaskID)
	}
	labelParts := make([]string, 0, 3)
	if strings.TrimSpace(evidenceRole) != "" {
		labelParts = append(labelParts, evidenceRole)
	}
	if strings.TrimSpace(artifactType) != "" {
		labelParts = append(labelParts, artifactType)
	}
	if strings.TrimSpace(ocrExcerpt) != "" {
		labelParts = append(labelParts, truncateText(ocrExcerpt, 64))
	}
	label := strings.Join(labelParts, " | ")
	if strings.TrimSpace(label) == "" {
		label = "screen evidence"
	}
	sourceType := "context"
	if _, ok := artifactsByID[artifactID]; ok {
		sourceType = "file"
	}
	identity := stableCitationIdentity(task.TaskID, sourceType, sourceRef, seed)
	result := map[string]any{
		"citation_id": fmt.Sprintf("cit_%s_%s", task.TaskID, identity),
		"task_id":     task.TaskID,
		"run_id":      task.RunID,
		"source_type": sourceType,
		"source_ref":  sourceRef,
		"label":       label,
	}
	if strings.TrimSpace(artifactID) != "" {
		result["artifact_id"] = artifactID
	}
	if strings.TrimSpace(artifactType) != "" {
		result["artifact_type"] = artifactType
	}
	if strings.TrimSpace(evidenceRole) != "" {
		result["evidence_role"] = evidenceRole
	}
	if strings.TrimSpace(ocrExcerpt) != "" {
		result["excerpt_text"] = ocrExcerpt
	}
	if screenSessionID := strings.TrimSpace(stringValue(seed, "screen_session_id", "")); screenSessionID != "" {
		result["screen_session_id"] = screenSessionID
	}
	return result
}

// stableCitationIdentity derives a deterministic citation fingerprint from the
// full formal seed so identical seeds collapse while distinct references on the
// same artifact remain separately addressable.
func stableCitationIdentity(taskID, sourceType, sourceRef string, seed map[string]any) string {
	normalized := map[string]any{
		"task_id":           taskID,
		"source_type":       strings.TrimSpace(sourceType),
		"source_ref":        strings.TrimSpace(sourceRef),
		"artifact_id":       strings.TrimSpace(stringValue(seed, "artifact_id", "")),
		"artifact_type":     strings.TrimSpace(stringValue(seed, "artifact_type", "")),
		"evidence_role":     strings.TrimSpace(stringValue(seed, "evidence_role", "")),
		"ocr_excerpt":       strings.TrimSpace(stringValue(seed, "ocr_excerpt", "")),
		"screen_session_id": strings.TrimSpace(stringValue(seed, "screen_session_id", "")),
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return "evidence"
	}
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("%x", sum[:8])
}

func executionAttemptIndex(previousTask, processingTask runengine.TaskRecord) int {
	if processingTask.ExecutionAttempt > 0 {
		return processingTask.ExecutionAttempt
	}
	if previousTask.ExecutionAttempt > 0 {
		if strings.TrimSpace(previousTask.RunID) == "" || previousTask.RunID == processingTask.RunID {
			return previousTask.ExecutionAttempt
		}
		return previousTask.ExecutionAttempt + 1
	}
	if strings.TrimSpace(previousTask.RunID) == "" || previousTask.RunID == processingTask.RunID {
		return 1
	}
	return 2
}

func executionSegmentKind(previousTask, processingTask runengine.TaskRecord) string {
	if strings.TrimSpace(previousTask.RunID) != "" && previousTask.RunID != processingTask.RunID {
		return executionSegmentRestart
	}
	if previousTask.Status == "paused" || taskIsBlockedHumanLoop(previousTask) {
		return executionSegmentResume
	}
	if processingTask.ExecutionAttempt > 1 {
		return executionSegmentRestart
	}
	return executionSegmentInitial
}

// dateTimeLayout is the shared timestamp layout exposed by orchestrator RPC
// payloads.

func (s *Service) captureExecutionTrace(task runengine.TaskRecord, snapshot contextsvc.TaskContextSnapshot, taskIntent map[string]any, result execution.Result, executionErr error) (traceeval.CaptureResult, error) {
	if s.traceEval == nil {
		return traceeval.CaptureResult{}, nil
	}
	capture, err := s.traceEval.Capture(traceeval.CaptureInput{
		TaskID:          task.TaskID,
		RunID:           task.RunID,
		IntentName:      stringValue(taskIntent, "name", ""),
		Snapshot:        snapshot,
		OutputText:      result.Content,
		DeliveryResult:  cloneMap(result.DeliveryResult),
		Artifacts:       cloneMapSlice(result.Artifacts),
		ExtensionAssets: extensionAssetReferencesFromMaps(result.ExtensionAssets),
		ModelInvocation: cloneMap(result.ModelInvocation),
		ToolCalls:       append([]tools.ToolCallRecord(nil), result.ToolCalls...),
		TokenUsage:      cloneMap(task.TokenUsage),
		DurationMS:      result.DurationMS,
		ExecutionError:  executionErr,
	})
	if err != nil {
		return traceeval.CaptureResult{}, err
	}
	if err := s.traceEval.Record(context.Background(), capture); err != nil {
		return traceeval.CaptureResult{}, err
	}
	return capture, nil
}

func (s *Service) resumeHumanLoopTask(task runengine.TaskRecord, reviewDecision map[string]any) (runengine.TaskRecord, map[string]any, map[string]any, bool, error) {
	if !resumedFromHumanLoop(task) {
		return runengine.TaskRecord{}, nil, nil, false, nil
	}
	pendingExecution, ok := s.runEngine.PendingExecutionPlan(task.TaskID)
	if !ok {
		return runengine.TaskRecord{}, nil, nil, false, nil
	}
	escalation := mapValue(pendingExecution, "escalation")
	if len(escalation) == 0 {
		return runengine.TaskRecord{}, nil, nil, false, nil
	}
	decision := strings.TrimSpace(stringValue(reviewDecision, "decision", ""))
	if decision == "" {
		return runengine.TaskRecord{}, nil, nil, false, fmt.Errorf("review.decision is required for human review resume")
	}
	if decision != "approve" && decision != "replan" {
		return runengine.TaskRecord{}, nil, nil, false, fmt.Errorf("unsupported review decision: %s", decision)
	}
	escalation["review_result"] = decision
	escalation["reviewed_at"] = currentTimeFromTask(s.runEngine, task.TaskID)
	if reviewerID := strings.TrimSpace(stringValue(reviewDecision, "reviewer_id", "")); reviewerID != "" {
		escalation["reviewer_id"] = reviewerID
	}
	if notes := strings.TrimSpace(stringValue(reviewDecision, "notes", "")); notes != "" {
		escalation["review_notes"] = notes
	}
	if correctedIntent := mapValue(reviewDecision, "corrected_intent"); len(correctedIntent) > 0 {
		escalation["corrected_intent"] = cloneMap(correctedIntent)
	}
	suggestedAction := firstNonEmptyString(stringValue(escalation, "suggested_action", ""), "review_and_replan")
	if suggestedAction != "review_and_replan" {
		return runengine.TaskRecord{}, nil, nil, false, nil
	}
	if decision == "replan" {
		intentValue := cloneMap(task.Intent)
		if correctedIntent := mapValue(escalation, "corrected_intent"); len(correctedIntent) > 0 {
			intentValue = correctedIntent
		}
		updatedTitle := s.intent.Suggest(snapshotFromTask(task), intentValue, false).TaskTitle
		replanBubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "人工复核要求重新规划，请确认新的处理意图。", task.UpdatedAt.Format(dateTimeLayout))
		replannedTask, ok := s.runEngine.ReopenIntentConfirmation(task.TaskID, updatedTitle, intentValue, replanBubble)
		if !ok {
			return runengine.TaskRecord{}, nil, nil, false, ErrTaskNotFound
		}
		return replannedTask, replanBubble, nil, true, nil
	}
	resultBubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", "人工复核完成，任务继续执行。", task.UpdatedAt.Format(dateTimeLayout))
	updatedTask, bubble, deliveryResult, _, err := s.executeTask(task, snapshotFromTask(task), task.Intent)
	if err != nil {
		return runengine.TaskRecord{}, nil, nil, false, err
	}
	if bubble == nil {
		bubble = resultBubble
	}
	return updatedTask, bubble, deliveryResult, true, nil
}

func humanReviewDecisionFromParams(arguments map[string]any) (map[string]any, error) {
	decision := mapValue(arguments, "review")
	if len(decision) == 0 {
		decision = mapValue(arguments, "human_review")
	}
	if len(decision) == 0 {
		return nil, fmt.Errorf("review decision is required to resume a human review task")
	}
	if strings.TrimSpace(stringValue(decision, "decision", "")) == "" {
		return nil, fmt.Errorf("review.decision is required to resume a human review task")
	}
	decisionValue := strings.TrimSpace(stringValue(decision, "decision", ""))
	if decisionValue != "approve" && decisionValue != "replan" {
		return nil, fmt.Errorf("unsupported review decision: %s", decisionValue)
	}
	if decisionValue == "replan" {
		if correctedIntent := mapValue(decision, "corrected_intent"); len(correctedIntent) == 0 {
			return nil, fmt.Errorf("review.corrected_intent is required when decision is replan")
		}
	}
	return cloneMap(decision), nil
}

func (s *Service) maybeEscalateHumanLoop(task runengine.TaskRecord, capture traceeval.CaptureResult, executionResult ...execution.Result) (runengine.TaskRecord, map[string]any, bool) {
	if capture.HumanInLoop == nil {
		return runengine.TaskRecord{}, nil, false
	}
	if len(executionResult) > 0 && executionAttemptHasSideEffects(executionResult[0]) {
		return runengine.TaskRecord{}, nil, false
	}
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", capture.HumanInLoop.Summary, task.UpdatedAt.Format(dateTimeLayout))
	escalation := map[string]any{
		"escalation_id":    capture.HumanInLoop.EscalationID,
		"reason":           capture.HumanInLoop.Reason,
		"review_result":    capture.HumanInLoop.ReviewResult,
		"status":           capture.HumanInLoop.Status,
		"summary":          capture.HumanInLoop.Summary,
		"suggested_action": capture.HumanInLoop.SuggestedAction,
		"created_at":       capture.HumanInLoop.CreatedAt,
	}
	updatedTask, ok := s.runEngine.EscalateHumanLoop(task.TaskID, escalation, bubble)
	if !ok {
		return runengine.TaskRecord{}, nil, false
	}
	return updatedTask, bubble, true
}

func resumedFromHumanLoop(task runengine.TaskRecord) bool {
	if task.Status != "processing" || task.CurrentStep != executionStepName(task.Intent) {
		return false
	}
	return true
}

func taskIsBlockedHumanLoop(task runengine.TaskRecord) bool {
	if task.Status != "blocked" || task.CurrentStep != "human_in_loop" {
		return false
	}
	return stringValue(task.PendingExecution, "kind", "") == "human_in_loop"
}

func executionAttemptHasSideEffects(result execution.Result) bool {
	if len(result.ToolCalls) == 0 {
		return false
	}
	for _, toolCall := range result.ToolCalls {
		if !isMutatingToolCall(toolCall.ToolName) {
			continue
		}
		return true
	}
	return false
}

func isMutatingToolCall(toolName string) bool {
	switch strings.TrimSpace(toolName) {
	case "write_file", "exec_command", "page_interact", "transcode_media", "normalize_recording", "extract_frames":
		return true
	default:
		return false
	}
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

func (s *Service) persistExecutionToolCallEvents(task runengine.TaskRecord, taskIntent map[string]any, toolCalls []tools.ToolCallRecord) {
	if s == nil || s.storage == nil || s.storage.LoopRuntimeStore() == nil || isAgentLoopTaskIntent(taskIntent) || len(toolCalls) == 0 {
		return
	}
	startedAt := time.Now().UTC()
	records := make([]storage.EventRecord, 0, len(toolCalls))
	for index, toolCall := range toolCalls {
		if strings.TrimSpace(toolCall.ToolName) == "" {
			continue
		}
		createdAt := startedAt.Add(time.Duration(index) * time.Millisecond)
		records = append(records, storage.EventRecord{
			EventID:     executionToolCallEventID(task.TaskID, toolCall, index, createdAt),
			RunID:       task.RunID,
			TaskID:      task.TaskID,
			StepID:      toolCall.StepID,
			Type:        "tool_call.completed",
			Level:       executionToolCallEventLevel(toolCall),
			PayloadJSON: marshalOrchestratorEventPayload(executionToolCallEventPayload(task.TaskID, toolCall)),
			CreatedAt:   createdAt.Format(time.RFC3339Nano),
		})
	}
	if len(records) == 0 {
		return
	}
	_ = s.storage.LoopRuntimeStore().SaveEvents(context.Background(), records)
}

func executionToolCallEventID(taskID string, toolCall tools.ToolCallRecord, index int, createdAt time.Time) string {
	if sanitizedToolCallID := strings.TrimSpace(strings.ReplaceAll(toolCall.ToolCallID, ".", "_")); sanitizedToolCallID != "" {
		return fmt.Sprintf("evt_%s_%s_%d", taskID, sanitizedToolCallID, index)
	}
	sanitizedToolName := strings.TrimSpace(strings.ReplaceAll(toolCall.ToolName, ".", "_"))
	if sanitizedToolName == "" {
		sanitizedToolName = "tool_call"
	}
	sanitizedStepID := strings.TrimSpace(strings.ReplaceAll(toolCall.StepID, ".", "_"))
	if sanitizedStepID == "" {
		sanitizedStepID = "task_scope"
	}
	return fmt.Sprintf("evt_%s_%s_%s_%d_%d_%d", taskID, sanitizedToolName, sanitizedStepID, index, createdAt.UnixNano(), persistedToolCallEventSeq.Add(1))
}

func (s *Service) persistExecutionDeliveryResult(task runengine.TaskRecord, taskIntent map[string]any, deliveryResult map[string]any) {
	if s == nil || s.storage == nil || s.storage.LoopRuntimeStore() == nil || isAgentLoopTaskIntent(taskIntent) || len(deliveryResult) == 0 {
		return
	}
	createdAt := time.Now().UTC()
	deliveryResultID := fmt.Sprintf("delivery_result_%s_%d", task.TaskID, createdAt.UnixNano())
	payloadJSON := marshalOrchestratorEventPayload(mapValue(deliveryResult, "payload"))
	_ = s.storage.LoopRuntimeStore().SaveDeliveryResult(context.Background(), storage.DeliveryResultRecord{
		DeliveryResultID: deliveryResultID,
		TaskID:           task.TaskID,
		RunID:            task.RunID,
		Type:             stringValue(deliveryResult, "type", "bubble"),
		Title:            stringValue(deliveryResult, "title", ""),
		PayloadJSON:      payloadJSON,
		PreviewText:      stringValue(deliveryResult, "preview_text", ""),
		CreatedAt:        createdAt.Format(time.RFC3339Nano),
	})
	_ = s.storage.LoopRuntimeStore().SaveEvents(context.Background(), []storage.EventRecord{{
		EventID:     fmt.Sprintf("evt_%s_delivery_ready_%d", task.TaskID, createdAt.UnixNano()),
		RunID:       task.RunID,
		TaskID:      task.TaskID,
		Type:        "delivery.ready",
		Level:       "info",
		PayloadJSON: marshalOrchestratorEventPayload(executionDeliveryReadyPayload(task.TaskID, deliveryResultID, deliveryResult)),
		CreatedAt:   createdAt.Add(time.Millisecond).Format(time.RFC3339Nano),
	}})
}

func executionToolCallEventLevel(toolCall tools.ToolCallRecord) string {
	switch toolCall.Status {
	case tools.ToolCallStatusFailed, tools.ToolCallStatusTimeout:
		return "error"
	default:
		return "info"
	}
}

func executionToolCallEventPayload(taskID string, toolCall tools.ToolCallRecord) map[string]any {
	payload := map[string]any{
		"task_id":      taskID,
		"tool_call_id": toolCall.ToolCallID,
		"tool_name":    toolCall.ToolName,
		"status":       string(toolCall.Status),
		"tool_status":  string(toolCall.Status),
		"input":        cloneMapOrEmpty(toolCall.Input),
		"output":       cloneMapOrEmpty(toolCall.Output),
		"duration_ms":  toolCall.DurationMS,
	}
	if strings.TrimSpace(toolCall.StepID) != "" {
		payload["step_id"] = toolCall.StepID
	}
	if toolCall.ErrorCode != nil {
		payload["error_code"] = *toolCall.ErrorCode
	}
	for _, key := range []string{"path", "url", "output_path", "output_dir", "source", "execution_backend", "page_count", "frame_count"} {
		if value, ok := toolCall.Output[key]; ok {
			payload[key] = value
			continue
		}
		if value, ok := toolCall.Input[key]; ok {
			payload[key] = value
		}
	}
	if summaryOutput, ok := toolCall.Output["summary_output"].(map[string]any); ok && len(summaryOutput) > 0 {
		payload["summary_output"] = cloneMap(summaryOutput)
	}
	return payload
}

func executionDeliveryReadyPayload(taskID, deliveryResultID string, deliveryResult map[string]any) map[string]any {
	payload := map[string]any{
		"task_id":            taskID,
		"delivery_result_id": deliveryResultID,
		"delivery_type":      stringValue(deliveryResult, "type", "bubble"),
		"preview_text":       stringValue(deliveryResult, "preview_text", ""),
	}
	deliveryPayload := mapValue(deliveryResult, "payload")
	for _, key := range []string{"path", "url"} {
		if value, ok := deliveryPayload[key]; ok {
			payload[key] = value
		}
	}
	return payload
}

func marshalOrchestratorEventPayload(payload map[string]any) string {
	if len(payload) == 0 {
		return "{}"
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func isAgentLoopTaskIntent(taskIntent map[string]any) bool {
	return stringValue(taskIntent, "name", "") == "agent_loop"
}

func executionStepName(taskIntent map[string]any) string {
	if stringValue(taskIntent, "name", "") == "agent_loop" {
		return "agent_loop"
	}
	return "generate_output"
}

// activeExecutionStepName records the execution step that can actually consume
// live follow-up steering. Agent-loop intent may still fall back to prompt
// generation, so processing tasks must not advertise a pollable loop unless the
// executor confirms that runtime mode.
func (s *Service) activeExecutionStepName(taskIntent map[string]any) string {
	if s != nil && s.executor != nil && s.executor.CanConsumeActiveSteering(taskIntent) {
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
	failureCode, failureCategory := classifyScreenFailure(task, err)
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
	updatedTask = s.attachFormalCitations(task, updatedTask, executionResult.ToolCalls, executionResult.ToolOutput, executionResult.DeliveryResult, executionResult.Artifacts)
	auditRecord := s.writeGovernanceAuditRecord(updatedTask.TaskID, updatedTask.RunID, auditType, auditAction, bubbleText, auditTarget, auditResult)
	if len(auditRecord) > 0 {
		metadata := cloneMap(mapValue(auditRecord, "metadata"))
		if metadata == nil {
			metadata = map[string]any{}
		}
		if failureCode != "" {
			metadata["failure_code"] = failureCode
		}
		if failureCategory != "" {
			metadata["failure_category"] = failureCategory
		}
		if len(metadata) > 0 {
			auditRecord["metadata"] = metadata
		}
	}
	budgetFailureAudit := s.buildBudgetFailureAudit(updatedTask, err)
	updatedTask = s.appendAuditData(updatedTask, compactAuditRecords(auditRecord, budgetFailureAudit), nil)
	return updatedTask, bubble
}

// classifyScreenFailure keeps screen-task runtime summaries and governance
// metadata aligned with the formal protocol error names while still exposing a
// task-facing failure category for UI grouping.
func classifyScreenFailure(task runengine.TaskRecord, err error) (string, string) {
	if stringValue(task.Intent, "name", "") != "screen_analyze" && task.SourceType != "screen_capture" {
		return "", ""
	}
	lowerError := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, tools.ErrApprovalRequired), errors.Is(err, tools.ErrScreenCaptureUnauthorized):
		return "APPROVAL_REQUIRED", "screen_authorization"
	case errors.Is(err, tools.ErrScreenCaptureNotSupported):
		return "PLATFORM_NOT_SUPPORTED", "screen_capability"
	case errors.Is(err, tools.ErrOCRWorkerFailed):
		return "OCR_WORKER_FAILED", "screen_ocr"
	case errors.Is(err, tools.ErrMediaWorkerFailed):
		return "MEDIA_WORKER_FAILED", "screen_media"
	case errors.Is(err, tools.ErrPlaywrightSidecarFailed), errors.Is(err, tools.ErrScreenCaptureFailed), errors.Is(err, tools.ErrScreenKeyframeSamplingFailed):
		return "PLAYWRIGHT_SIDECAR_FAILED", "screen_capture"
	case errors.Is(err, tools.ErrCapabilityDenied):
		return "CAPABILITY_DENIED", "screen_capability"
	case errors.Is(err, tools.ErrToolOutputInvalid):
		return "TOOL_OUTPUT_INVALID", "screen_observation"
	case errors.Is(err, tools.ErrScreenCaptureSessionExpired), strings.Contains(lowerError, "session"):
		return "TOOL_EXECUTION_FAILED", "screen_session"
	case strings.Contains(lowerError, "incomplete") || strings.Contains(lowerError, "empty") || strings.Contains(lowerError, "未识别"):
		return "TOOL_OUTPUT_INVALID", "screen_observation"
	default:
		return "TOOL_EXECUTION_FAILED", "screen_analysis"
	}
}

func executionFailureBubble(err error) string {
	switch {
	case errors.Is(err, execution.ErrRecoveryPointPrepareFailed):
		return "执行失败：执行前恢复点创建失败，请稍后重试。"
	case errors.Is(err, tools.ErrWorkspaceBoundaryDenied):
		return "执行失败：目标超出工作区边界，已阻止本次操作。"
	case errors.Is(err, tools.ErrCommandNotAllowed):
		return "执行失败：命令存在高危风险，已被策略拦截。"
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, tools.ErrToolExecutionTimeout):
		return "执行失败：本地任务执行超时，请重试。"
	case errors.Is(err, context.Canceled):
		return "执行失败：本地任务已取消。"
	case errors.Is(err, tools.ErrCapabilityDenied):
		return "执行失败：当前平台能力不可用，请检查环境后重试。"
	case errors.Is(err, tools.ErrToolExecutionFailed):
		return "执行失败：工具运行失败，请检查环境后重试。"
	default:
		if detail := modelExecutionFailureBubble(err); detail != "" {
			return detail
		}
		return "执行失败：请稍后重试。"
	}
}

// modelExecutionFailureBubble keeps upstream model failures actionable without
// exposing raw transport details or secrets in the task-facing bubble copy.
func modelExecutionFailureBubble(err error) string {
	if err == nil {
		return ""
	}
	var statusErr *model.OpenAIHTTPStatusError
	switch {
	case errors.Is(err, model.ErrClientNotConfigured):
		return "执行失败：当前模型未完成配置，请检查 Provider、Base URL、Model 和 API Key。"
	case errors.Is(err, model.ErrToolCallingNotSupported):
		return "执行失败：当前模型接口不支持工具调用，请切换到兼容工具调用的模型或关闭相关工具路径。"
	case errors.Is(err, model.ErrOpenAIResponseInvalid):
		return "执行失败：模型返回内容无法解析，请检查上游接口兼容性。"
	case errors.Is(err, model.ErrOpenAIRequestTimeout):
		return "执行失败：模型请求超时，请稍后重试。"
	case errors.Is(err, model.ErrOpenAIRequestFailed):
		return "执行失败：模型请求发送失败，请检查网络连接或上游地址。"
	case errors.As(err, &statusErr):
		return modelHTTPStatusFailureBubble(statusErr)
	default:
		return ""
	}
}

func modelHTTPStatusFailureBubble(statusErr *model.OpenAIHTTPStatusError) string {
	if statusErr == nil {
		return ""
	}
	safeMessage := sanitizeModelProviderMessage(statusErr.Message)
	switch statusErr.StatusCode {
	case 400:
		if safeMessage != "" {
			return "执行失败：模型请求被上游拒绝（" + safeMessage + "）。"
		}
		return "执行失败：模型请求被上游拒绝，请检查输入内容、模型能力和接口兼容性。"
	case 401, 403:
		if safeMessage != "" {
			return "执行失败：模型鉴权失败（" + safeMessage + "），请检查 API Key 或访问权限。"
		}
		return "执行失败：模型鉴权失败，请检查 API Key 或访问权限。"
	case 404:
		if safeMessage != "" {
			return "执行失败：模型接口不存在（" + safeMessage + "），请检查 Base URL 或接口兼容性。"
		}
		return "执行失败：模型接口不存在，请检查 Base URL 或接口兼容性。"
	case 408, 504:
		return "执行失败：模型请求超时，请稍后重试。"
	case 429:
		if safeMessage != "" {
			return "执行失败：模型请求过于频繁（" + safeMessage + "），请稍后重试。"
		}
		return "执行失败：模型请求过于频繁，请稍后重试。"
	case 500, 502, 503:
		if safeMessage != "" {
			return "执行失败：模型服务暂时不可用（" + safeMessage + "），请稍后重试。"
		}
		return "执行失败：模型服务暂时不可用，请稍后重试。"
	default:
		if safeMessage != "" {
			return "执行失败：模型调用失败（" + safeMessage + "）。"
		}
		return "执行失败：模型调用失败，请稍后重试。"
	}
}

func sanitizeModelProviderMessage(message string) string {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return ""
	}
	trimmed = strings.Join(strings.Fields(trimmed), " ")
	trimmed = strings.ReplaceAll(trimmed, "\r", " ")
	trimmed = strings.ReplaceAll(trimmed, "\n", " ")
	lowerTrimmed := strings.ToLower(trimmed)
	for _, secretMarker := range []string{"api key", "authorization", "bearer ", "sk-"} {
		if strings.Contains(lowerTrimmed, secretMarker) {
			return ""
		}
	}
	if len(trimmed) > 120 {
		trimmed = strings.TrimSpace(trimmed[:120]) + "..."
	}
	return trimmed
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

func (s *Service) buildBudgetDowngradeAudit(task runengine.TaskRecord, decision budgetDowngradeDecision) map[string]any {
	if !decision.Applied {
		return nil
	}
	return map[string]any{
		"audit_record_id": fmt.Sprintf("audit_budget_%s_%d", task.TaskID, time.Now().UnixNano()),
		"task_id":         task.TaskID,
		"run_id":          task.RunID,
		"category":        "budget_auto_downgrade",
		"action":          "budget_auto_downgrade.applied",
		"result":          "applied",
		"reason":          decision.TriggerReason,
		"created_at":      time.Now().Format(dateTimeLayout),
		"details": map[string]any{
			"trigger_stage":   decision.TriggerStage,
			"degrade_actions": append([]string(nil), decision.DegradeActions...),
			"summary":         decision.Summary,
			"trace":           cloneMap(decision.Trace),
		},
	}
}

func (s *Service) buildBudgetFailureAudit(task runengine.TaskRecord, executionErr error) map[string]any {
	if executionErr == nil {
		return nil
	}
	if !errors.Is(executionErr, model.ErrClientNotConfigured) && !errors.Is(executionErr, model.ErrToolCallingNotSupported) && !errors.Is(executionErr, model.ErrModelProviderUnsupported) && !errors.Is(executionErr, model.ErrSecretNotFound) && !errors.Is(executionErr, model.ErrSecretSourceFailed) {
		return nil
	}
	return map[string]any{
		"audit_record_id": fmt.Sprintf("audit_budget_failure_%s_%d", task.TaskID, time.Now().UnixNano()),
		"task_id":         task.TaskID,
		"run_id":          task.RunID,
		"category":        "budget_auto_downgrade",
		"action":          "budget_auto_downgrade.failure_signal",
		"result":          "failed",
		"reason":          executionErr.Error(),
		"created_at":      time.Now().Format(dateTimeLayout),
	}
}

func (s *Service) recordBudgetDowngradeEvent(task runengine.TaskRecord, decision budgetDowngradeDecision) runengine.TaskRecord {
	if !decision.Applied {
		return task
	}
	s.publishRuntimeNotification(task.TaskID, "budget.downgrade.applied", map[string]any{
		"task_id":          task.TaskID,
		"run_id":           task.RunID,
		"trigger_reason":   decision.TriggerReason,
		"trigger_stage":    decision.TriggerStage,
		"degrade_actions":  append([]string(nil), decision.DegradeActions...),
		"summary":          decision.Summary,
		"trace":            cloneMap(decision.Trace),
		"budget_auto_down": true,
	})
	updatedTask, ok := s.runEngine.EmitRuntimeNotification(task.TaskID, "budget.downgrade.applied", map[string]any{
		"task_id":          task.TaskID,
		"run_id":           task.RunID,
		"trigger_reason":   decision.TriggerReason,
		"trigger_stage":    decision.TriggerStage,
		"degrade_actions":  append([]string(nil), decision.DegradeActions...),
		"summary":          decision.Summary,
		"trace":            cloneMap(decision.Trace),
		"budget_auto_down": true,
	})
	if !ok {
		return task
	}
	return updatedTask
}

// dateTimeLayout is the shared timestamp layout exposed by orchestrator RPC
// payloads.
const dateTimeLayout = time.RFC3339
