package orchestrator

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/agentloop"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/execution"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/presentation"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/traceeval"
)

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
		decision.Summary = presentation.Text(presentation.MessageBudgetDowngradeProvider, nil)
		decision.Trace = buildBudgetDecisionTrace(task, decision, policy, 0, 0)
		return decision
	}
	failureSignals := recentBudgetFailureCount(task)
	if failureSignals >= intValue(policy, "failure_signal_window", 2) {
		decision.Applied = true
		decision.TriggerReason = "failure_pressure"
		decision.DegradeActions = budgetDegradeActionsForReason(policy, "failure_pressure")
		decision.Summary = presentation.Text(presentation.MessageBudgetDowngradeFailure, nil)
		decision.Trace = buildBudgetDecisionTrace(task, decision, policy, failureSignals, 0)
		return decision
	}
	totalTokens := intValueFromAny(task.TokenUsage["total_tokens"])
	estimatedCost := floatValueFromAny(task.TokenUsage["estimated_cost"])
	if totalTokens >= intValue(policy, "token_pressure_threshold", 64) || estimatedCost >= floatValueFromAny(policy["cost_pressure_threshold"]) {
		decision.Applied = true
		decision.TriggerReason = "budget_pressure"
		decision.DegradeActions = budgetDegradeActionsForReason(policy, "budget_pressure")
		decision.Summary = presentation.Text(presentation.MessageBudgetDowngradePressure, nil)
		decision.Trace = buildBudgetDecisionTrace(task, decision, policy, failureSignals, map[string]any{"total_tokens": totalTokens, "estimated_cost": estimatedCost})
	}
	return decision
}

// applyBudgetAutoDowngrade mutates the execution request shape so the downgrade
// decision changes the real path instead of only updating settings summaries.
func (s *Service) applyBudgetAutoDowngrade(task runengine.TaskRecord, snapshot taskcontext.TaskContextSnapshot, taskIntent map[string]any, decision budgetDowngradeDecision) (runengine.TaskRecord, taskcontext.TaskContextSnapshot, map[string]any) {
	if !decision.Applied {
		return task, snapshot, taskIntent
	}
	updatedTask := task
	updatedTask.PreferredDelivery = "bubble"
	updatedTask.FallbackDelivery = "bubble"
	updatedIntent := cloneMap(taskIntent)
	arguments := cloneMap(mapValue(updatedIntent, "arguments"))
	if arguments == nil {
		arguments = map[string]any{}
	}
	if containsString(decision.DegradeActions, "skip_expensive_tools") {
		arguments["disable_tool_calls"] = true
	}
	arguments["budget_auto_downgrade_applied"] = true
	updatedIntent["arguments"] = arguments
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

func (s *Service) executeTask(task runengine.TaskRecord, snapshot taskcontext.TaskContextSnapshot, taskIntent map[string]any) (runengine.TaskRecord, map[string]any, map[string]any, []map[string]any, error) {
	return s.executeTaskAttempt(task, task, snapshot, taskIntent)
}

// executeTaskAttempt runs the current task state while preserving the previous
// task snapshot for execution segment classification. Restart needs this split:
// the new run must execute, but the executor still needs the old run_id to mark
// the segment as restart instead of initial.
func (s *Service) executeTaskAttempt(previousTask, task runengine.TaskRecord, snapshot taskcontext.TaskContextSnapshot, taskIntent map[string]any) (runengine.TaskRecord, map[string]any, map[string]any, []map[string]any, error) {
	var processingTask runengine.TaskRecord
	ok := false
	if s.isPreparedRestartAttempt(task) {
		processingTask, ok = s.runEngine.BeginPreparedExecution(task, s.activeExecutionStepName(taskIntent), presentation.Text(presentation.MessageTimelineStartOutput, nil))
	} else {
		processingTask, ok = s.runEngine.BeginExecution(task.TaskID, s.activeExecutionStepName(taskIntent), presentation.Text(presentation.MessageTimelineStartOutput, nil))
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
	if err == nil {
		executionResult = s.normalizeExecutionFormalDeliveryResult(processingTask.TaskID, deliveryType, resultTitle, executionResult)
	}
	processingTask = s.recordExecutionToolCalls(processingTask, executionResult.ToolCalls)
	s.persistExecutionToolCallEvents(processingTask, taskIntent, executionResult.ToolCalls)
	auditDeliveryResult := executionResult.DeliveryResult
	if err != nil || executionResult.LoopStopReason == string(agentloop.StopReasonNeedUserInput) {
		auditDeliveryResult = nil
	}
	executionAuditRecords, executionTokenUsage := s.buildExecutionAudit(processingTask, executionResult.ToolCalls, auditDeliveryResult)
	if len(executionResult.BudgetFailure) > 0 {
		executionAuditRecords = append(executionAuditRecords, cloneMap(executionResult.BudgetFailure))
	}
	executionAuditRecords = append(executionAuditRecords, s.buildBudgetDowngradeAudit(processingTask, budgetDecision))
	processingTask = s.appendAuditData(processingTask, executionAuditRecords, executionTokenUsage)
	processingTask = s.recordBudgetDowngradeEvent(processingTask, budgetDecision)
	traceResult := executionResult
	if traceResult.LoopStopReason == string(agentloop.StopReasonNeedUserInput) {
		traceResult.DeliveryResult = nil
	}
	traceCapture, traceErr := s.captureExecutionTrace(processingTask, snapshot, taskIntent, traceResult, err)
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

// normalizeExecutionFormalDeliveryResult keeps result-page delivery semantics at
// the orchestrator boundary even when legacy direct-tool execution helpers still
// emit bubble-shaped delivery results.
func (s *Service) normalizeExecutionFormalDeliveryResult(taskID, deliveryType, resultTitle string, result execution.Result) execution.Result {
	if strings.TrimSpace(deliveryType) != "result_page" {
		return result
	}
	if stringValue(result.DeliveryResult, "type", "") == "result_page" {
		return result
	}
	result.DeliveryResult = s.delivery.BuildDeliveryResultWithTargetPath(
		taskID,
		deliveryType,
		resultTitle,
		previewTextForDeliveryType(deliveryType),
		"",
	)
	return result
}

// shouldBoundTaskExecution limits the outer orchestrator timeout to synchronous
// shell-ball submits that still resolve to bubble delivery. Longer structured
// flows already carry their own internal timeouts and should not inherit the
// short near-field deadline.
func shouldBoundTaskExecution(task runengine.TaskRecord, snapshot taskcontext.TaskContextSnapshot, taskIntent map[string]any, deliveryType string) bool {
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
		presentation.Text(presentation.MessageBubbleInputNeedGoal, nil),
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

func (s *Service) captureExecutionTrace(task runengine.TaskRecord, snapshot taskcontext.TaskContextSnapshot, taskIntent map[string]any, result execution.Result, executionErr error) (traceeval.CaptureResult, error) {
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
	task = s.appendAuditData(task, compactAuditRecords(s.buildHumanLoopReviewAudit(task, escalation, reviewDecision)), nil)
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
		replanBubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", presentation.Text(presentation.MessageBubbleReviewReplan, nil), task.UpdatedAt.Format(dateTimeLayout))
		replannedTask, ok := s.runEngine.ReopenIntentConfirmation(task.TaskID, updatedTitle, intentValue, replanBubble)
		if !ok {
			return runengine.TaskRecord{}, nil, nil, false, ErrTaskNotFound
		}
		return replannedTask, replanBubble, nil, true, nil
	}
	resultBubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", presentation.Text(presentation.MessageBubbleReviewContinue, nil), task.UpdatedAt.Format(dateTimeLayout))
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

func (s *Service) buildHumanLoopReviewAudit(task runengine.TaskRecord, escalation, reviewDecision map[string]any) map[string]any {
	decision := stringValue(escalation, "review_result", stringValue(reviewDecision, "decision", ""))
	if decision == "" {
		return nil
	}
	reviewedAt := stringValue(escalation, "reviewed_at", currentTimeFromTask(s.runEngine, task.TaskID))
	details := map[string]any{
		"escalation_id":    stringValue(escalation, "escalation_id", ""),
		"suggested_action": stringValue(escalation, "suggested_action", ""),
		"review_result":    decision,
	}
	if reviewerID := stringValue(escalation, "reviewer_id", ""); reviewerID != "" {
		details["reviewer_id"] = reviewerID
	}
	if notes := stringValue(escalation, "review_notes", ""); notes != "" {
		details["review_notes"] = notes
	}
	if correctedIntent := mapValue(escalation, "corrected_intent"); len(correctedIntent) > 0 {
		details["corrected_intent"] = cloneMap(correctedIntent)
	}
	return map[string]any{
		"audit_record_id": fmt.Sprintf("audit_human_loop_%s_%d", task.TaskID, time.Now().UnixNano()),
		"task_id":         task.TaskID,
		"run_id":          task.RunID,
		"category":        "human_in_loop",
		"action":          "human_in_loop.review",
		"result":          decision,
		"reason":          stringValue(escalation, "reason", "human_review"),
		"created_at":      reviewedAt,
		"details":         details,
	}
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
	case "write_file", "exec_command", "page_interact", "browser_navigate", "browser_tab_focus", "browser_interact", "transcode_media", "normalize_recording", "extract_frames":
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
		return presentation.Text(presentation.MessageExecutionFailureCheckpoint, nil)
	case errors.Is(err, tools.ErrWorkspaceBoundaryDenied):
		return presentation.Text(presentation.MessageExecutionFailureBoundary, nil)
	case errors.Is(err, tools.ErrCommandNotAllowed):
		return presentation.Text(presentation.MessageExecutionFailureCommand, nil)
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, tools.ErrToolExecutionTimeout):
		return presentation.Text(presentation.MessageExecutionFailureTimeout, nil)
	case errors.Is(err, context.Canceled):
		return presentation.Text(presentation.MessageExecutionFailureCanceled, nil)
	case errors.Is(err, tools.ErrCapabilityDenied):
		return presentation.Text(presentation.MessageExecutionFailurePlatform, nil)
	case errors.Is(err, tools.ErrToolExecutionFailed):
		return presentation.Text(presentation.MessageExecutionFailureTool, nil)
	default:
		if detail := modelExecutionFailureBubble(err); detail != "" {
			return detail
		}
		return presentation.Text(presentation.MessageExecutionFailureGeneric, nil)
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
		return presentation.Text(presentation.MessageExecutionFailureModelSetup, nil)
	case errors.Is(err, model.ErrToolCallingNotSupported):
		return presentation.Text(presentation.MessageExecutionFailureToolCall, nil)
	case errors.Is(err, model.ErrOpenAIResponseInvalid):
		return presentation.Text(presentation.MessageExecutionFailureInvalid, nil)
	case errors.Is(err, model.ErrOpenAIRequestTimeout):
		return presentation.Text(presentation.MessageExecutionFailureModelTime, nil)
	case errors.Is(err, model.ErrOpenAIRequestFailed):
		return presentation.Text(presentation.MessageExecutionFailureRequest, nil)
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
		return presentation.Text(presentation.MessageExecutionFailureRejected, presentation.DetailParam(safeMessage))
	case 401, 403:
		return presentation.Text(presentation.MessageExecutionFailureAuth, presentation.DetailParam(safeMessage))
	case 404:
		return presentation.Text(presentation.MessageExecutionFailureEndpoint, presentation.DetailParam(safeMessage))
	case 408, 504:
		return presentation.Text(presentation.MessageExecutionFailureModelTime, nil)
	case 429:
		return presentation.Text(presentation.MessageExecutionFailureRate, presentation.DetailParam(safeMessage))
	case 500, 502, 503:
		return presentation.Text(presentation.MessageExecutionFailureUpstream, presentation.DetailParam(safeMessage))
	default:
		return presentation.Text(presentation.MessageExecutionFailureModel, presentation.DetailParam(safeMessage))
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
	reason := budgetFailureAuditReason(executionErr)
	if reason == "" {
		return nil
	}
	return map[string]any{
		"audit_record_id": fmt.Sprintf("audit_budget_failure_%s_%d", task.TaskID, time.Now().UnixNano()),
		"task_id":         task.TaskID,
		"run_id":          task.RunID,
		"category":        "budget_auto_downgrade",
		"action":          "budget_auto_downgrade.failure_signal",
		"result":          "failed",
		"reason":          reason,
		"created_at":      time.Now().Format(dateTimeLayout),
	}
}

func budgetFailureAuditReason(executionErr error) string {
	switch {
	case errors.Is(executionErr, model.ErrClientNotConfigured):
		return "client_not_configured"
	case errors.Is(executionErr, model.ErrToolCallingNotSupported):
		return "tool_calling_not_supported"
	case errors.Is(executionErr, model.ErrModelProviderUnsupported):
		return "model_provider_unsupported"
	case errors.Is(executionErr, model.ErrSecretNotFound):
		return "secret_not_found"
	case errors.Is(executionErr, model.ErrSecretSourceFailed):
		return "secret_source_failed"
	default:
		return ""
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
