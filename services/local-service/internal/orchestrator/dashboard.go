package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/perception"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

// DashboardOverviewGet builds the dashboard's task-centric overview from
// runtime, storage, governance, memory, and plugin snapshots. It is a read-side
// adapter and must not create new task or delivery truth.
func (s *Service) DashboardOverviewGet(params map[string]any) (map[string]any, error) {
	queryViews := newTaskQueryViews(s)
	unfinishedTasks := queryViews.tasks("unfinished", "updated_at", "desc")
	finishedTasks := queryViews.tasks("finished", "finished_at", "desc")
	_, runtimePendingTotal := s.runEngine.PendingApprovalRequests(20, 0)
	needStorageFallback := !queryViews.hasRuntimeState()

	pendingApprovals := pendingApprovalsFromTasks(unfinishedTasks)
	pendingTotal := mergedPendingApprovalTotal(unfinishedTasks, runtimePendingTotal)
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
			"workspace_path":         currentRuntimeWorkspaceRoot(s.executor),
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

func approvalRequestRecordsToItems(records []storage.ApprovalRequestRecord) []map[string]any {
	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		item := map[string]any{
			"approval_id":    record.ApprovalID,
			"task_id":        record.TaskID,
			"operation_name": record.OperationName,
			"risk_level":     record.RiskLevel,
			"target_object":  record.TargetObject,
			"reason":         record.Reason,
			"status":         record.Status,
			"created_at":     record.CreatedAt,
			"updated_at":     record.UpdatedAt,
		}
		if strings.TrimSpace(record.ImpactScopeJSON) != "" {
			var scope map[string]any
			if err := json.Unmarshal([]byte(record.ImpactScopeJSON), &scope); err == nil && len(scope) > 0 {
				item["impact_scope"] = scope
			}
		}
		items = append(items, item)
	}
	return items
}

// mergedPendingApprovalTotal prefers the task-centric merged view so mixed
// runtime and storage snapshots report one stable pending-authorization count.
func mergedPendingApprovalTotal(unfinishedTasks []runengine.TaskRecord, runtimePendingTotal int) int {
	pendingTotal := countPendingApprovalTasks(unfinishedTasks)
	if pendingTotal == 0 && runtimePendingTotal > 0 {
		return runtimePendingTotal
	}
	return pendingTotal
}

// DashboardModuleGet returns one dashboard module payload while preserving the
// same merged runtime/storage semantics as the full overview.
func (s *Service) DashboardModuleGet(params map[string]any) (map[string]any, error) {
	module := stringValue(params, "module", "mirror")
	tab := stringValue(params, "tab", "daily_summary")
	queryViews := newTaskQueryViews(s)
	finishedTasks := queryViews.tasks("finished", "finished_at", "desc")
	unfinishedTasks := queryViews.tasks("unfinished", "updated_at", "desc")
	_, runtimePendingTotal := s.runEngine.PendingApprovalRequests(20, 0)
	pendingTotal := mergedPendingApprovalTotal(unfinishedTasks, runtimePendingTotal)
	latestAudit := latestAuditRecordFromTasks(append(append([]runengine.TaskRecord{}, unfinishedTasks...), finishedTasks...))
	if latestAudit == nil {
		latestAudit = s.latestAuditRecordFromStorage("")
	}
	pluginSummary := s.pluginRuntimeSummary()
	summary := map[string]any{
		"completed_tasks":     len(finishedTasks),
		"generated_outputs":   countGeneratedOutputs(finishedTasks),
		"authorizations_used": countAuthorizedTasks(unfinishedTasks, finishedTasks),
		"exceptions":          countExceptionTasks(unfinishedTasks, finishedTasks),
		"plugin_runtime":      pluginSummary,
	}
	highlights := buildDashboardModuleHighlightsWithAudit(unfinishedTasks, finishedTasks, pendingTotal, latestAudit)
	if module == "tasks" {
		summary = s.buildDashboardTaskModuleSummary(unfinishedTasks, finishedTasks, summary)
		highlights = s.buildDashboardTaskModuleHighlights(unfinishedTasks, finishedTasks, pendingTotal, latestAudit)
	}
	return map[string]any{
		"module":     module,
		"tab":        tab,
		"summary":    summary,
		"highlights": highlights,
	}, nil
}

// buildDashboardTaskModuleSummary keeps the generic dashboard module summary
// while exposing one task-focused runtime summary for the current focus task.
func (s *Service) buildDashboardTaskModuleSummary(unfinishedTasks, finishedTasks []runengine.TaskRecord, baseSummary map[string]any) map[string]any {
	summary := cloneMap(baseSummary)
	summary["processing_tasks"] = countTasksWithStatus(unfinishedTasks, "processing")
	summary["waiting_auth_tasks"] = countTasksWithStatus(unfinishedTasks, "waiting_auth")
	summary["blocked_tasks"] = countTasksWithStatus(unfinishedTasks, "blocked", "failed", "ended_unfinished", "paused")
	focusTask, ok := focusTaskForOverview(unfinishedTasks, finishedTasks)
	if !ok {
		return summary
	}
	summary["focus_task_id"] = focusTask.TaskID
	summary["focus_runtime_summary"] = s.buildDashboardFocusRuntimeSummary(focusTask)
	return summary
}

// buildDashboardTaskModuleHighlights turns the current focus task runtime into
// human-readable dashboard hints without adding a new protocol method.
func (s *Service) buildDashboardTaskModuleHighlights(unfinishedTasks, finishedTasks []runengine.TaskRecord, pendingTotal int, latestAudit map[string]any) []string {
	highlights := make([]string, 0, 6)
	focusTask, ok := focusTaskForOverview(unfinishedTasks, finishedTasks)
	if ok {
		runtimeSummary := s.buildDashboardFocusRuntimeSummary(focusTask)
		if focusTask.Status == "waiting_auth" {
			highlights = append(highlights, "焦点任务当前正在等待授权确认。")
		} else if focusTask.Status == "processing" {
			highlights = append(highlights, fmt.Sprintf("焦点任务仍在执行中，当前步骤为 %s。", firstNonEmptyString(focusTask.CurrentStep, "generate_output")))
		} else if focusTask.Status == "blocked" || focusTask.Status == "failed" || focusTask.Status == "paused" || focusTask.Status == "ended_unfinished" {
			highlights = append(highlights, fmt.Sprintf("焦点任务当前状态为 %s。", focusTask.Status))
		}
		if stopReason := strings.TrimSpace(stringValue(runtimeSummary, "loop_stop_reason", "")); stopReason != "" {
			highlights = append(highlights, fmt.Sprintf("最近停止原因：%s。", stopReason))
		}
		if latestEventType := strings.TrimSpace(stringValue(runtimeSummary, "latest_event_type", "")); latestEventType != "" {
			highlights = append(highlights, fmt.Sprintf("最近运行事件：%s。", latestEventType))
		}
		if steeringCount := intValue(runtimeSummary, "active_steering_count", 0); steeringCount > 0 {
			highlights = append(highlights, fmt.Sprintf("当前仍有 %d 条追加要求待消费。", steeringCount))
		}
	}
	highlights = append(highlights, buildDashboardModuleHighlightsWithAudit(unfinishedTasks, finishedTasks, pendingTotal, latestAudit)...)
	return dedupeStringSlice(highlights)
}

// buildDashboardFocusRuntimeSummary reuses the task detail runtime summary but
// allows dashboard cards to fall back to the latest in-memory runtime event
// when persistence has not yet flushed a loop event row.
func (s *Service) buildDashboardFocusRuntimeSummary(task runengine.TaskRecord) map[string]any {
	summary := s.buildTaskRuntimeSummary(task)
	if strings.TrimSpace(stringValue(summary, "latest_event_type", "")) != "" {
		return summary
	}
	latestEventType := strings.TrimSpace(stringValue(task.LatestEvent, "type", ""))
	if strings.HasPrefix(latestEventType, "loop.") || latestEventType == "task.steered" {
		summary["latest_event_type"] = latestEventType
	}
	return summary
}

// MirrorOverviewGet returns the memory-oriented mirror summary derived from task
// history and stored memory references; it does not write memory summaries.
func (s *Service) MirrorOverviewGet(params map[string]any) (map[string]any, error) {
	_ = params
	finishedTasks := newTaskQueryViews(s).tasks("finished", "finished_at", "desc")
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

func countTasksWithStatus(tasks []runengine.TaskRecord, statuses ...string) int {
	if len(statuses) == 0 {
		return 0
	}
	allowed := make(map[string]struct{}, len(statuses))
	for _, status := range statuses {
		if strings.TrimSpace(status) == "" {
			continue
		}
		allowed[status] = struct{}{}
	}
	total := 0
	for _, task := range tasks {
		if _, ok := allowed[task.Status]; ok {
			total++
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

type tokenCostUsageSummary struct {
	TodayTokens int
	TodayCost   float64
}

func aggregateTokenCostSummary(unfinishedTasks, finishedTasks []runengine.TaskRecord, budgetAutoDowngrade bool, extraTodayUsage tokenCostUsageSummary) map[string]any {
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
	todayTokens += extraTodayUsage.TodayTokens
	todayCost += extraTodayUsage.TodayCost

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

// inspectorTitleGenerationUsage keeps manual inspector note-title generation
// visible in day-level quota summaries without collapsing these calls into the
// current task token view.
func (s *Service) inspectorTitleGenerationUsage(ctx context.Context) tokenCostUsageSummary {
	if s == nil || s.storage == nil {
		return tokenCostUsageSummary{}
	}

	traceRecords, _, err := s.storage.TraceStore().ListTraceRecords(ctx, "", 0, 0)
	if err != nil {
		return tokenCostUsageSummary{}
	}

	now := time.Now().UTC()
	relevantTraceIDs := make(map[string]struct{}, len(traceRecords))
	summary := tokenCostUsageSummary{}
	for _, record := range traceRecords {
		if !sameDay(parseRFC3339Time(record.CreatedAt), now) {
			continue
		}
		if !strings.HasPrefix(strings.TrimSpace(record.TaskID), "insp_") {
			continue
		}
		if strings.TrimSpace(record.LLMInputSummary) != "task_inspector.generate_note_title" {
			continue
		}
		relevantTraceIDs[record.TraceID] = struct{}{}
		summary.TodayCost += record.Cost
	}
	if len(relevantTraceIDs) == 0 {
		return summary
	}

	evalSnapshots, _, err := s.storage.EvalStore().ListEvalSnapshots(ctx, "", 0, 0)
	if err != nil {
		return summary
	}
	for _, snapshot := range evalSnapshots {
		if !sameDay(parseRFC3339Time(snapshot.CreatedAt), now) {
			continue
		}
		if _, ok := relevantTraceIDs[snapshot.TraceID]; !ok {
			continue
		}
		summary.TodayTokens += totalTokensFromEvalMetrics(snapshot.MetricsJSON)
	}
	return summary
}

func totalTokensFromEvalMetrics(metricsJSON string) int {
	metricsJSON = strings.TrimSpace(metricsJSON)
	if metricsJSON == "" {
		return 0
	}
	metrics := map[string]any{}
	if err := json.Unmarshal([]byte(metricsJSON), &metrics); err != nil {
		return 0
	}
	return intValueFromAny(metrics["total_tokens"])
}

func parseRFC3339Time(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return parsed
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
