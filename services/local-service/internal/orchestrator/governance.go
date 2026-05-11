package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/agentloop"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/execution"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/presentation"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

var approvalIDSequence atomic.Uint64

// buildApprovalRequest creates the normalized approval_request payload. The
// object must already be protocol-facing here because it is persisted, replayed
// to transports, and later echoed back through agent.security.respond.
func buildApprovalRequest(taskID string, taskIntent map[string]any, assessment execution.GovernanceAssessment) map[string]any {
	arguments := mapValue(taskIntent, "arguments")
	targetObject := firstNonEmptyString(assessment.TargetObject, stringValue(arguments, "target_path", "workspace_document"))
	if targetObject == "" {
		targetObject = "workspace_document"
	}
	now := time.Now()

	return map[string]any{
		"approval_id":    fmt.Sprintf("appr_%s_%d_%d", taskID, now.UnixNano(), approvalIDSequence.Add(1)),
		"task_id":        taskID,
		"operation_name": firstNonEmptyString(assessment.OperationName, firstNonEmptyString(stringValue(taskIntent, "name", ""), "write_file")),
		"risk_level":     firstNonEmptyString(assessment.RiskLevel, "red"),
		"target_object":  targetObject,
		"reason":         firstNonEmptyString(assessment.Reason, "policy_requires_authorization"),
		"status":         "pending",
		"created_at":     now.Format(dateTimeLayout),
	}
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

func (s *Service) handleTaskGovernanceDecision(task runengine.TaskRecord, taskIntent map[string]any) (runengine.TaskRecord, TaskEntryResponse, bool, error) {
	assessment, ok, err := s.assessTaskGovernance(task, taskIntent)
	if err != nil {
		return task, TaskEntryResponse{}, false, err
	}
	if !ok {
		assessment, ok = s.fallbackGovernanceAssessment(task, taskIntent)
		if !ok {
			return task, TaskEntryResponse{}, false, nil
		}
	}
	if assessment.Deny {
		response, blockedTask, blockErr := s.blockTaskByAssessment(task, assessment)
		return blockedTask, response, true, blockErr
	}
	if !assessment.ApprovalRequired {
		return task, TaskEntryResponse{}, false, nil
	}
	pendingExecution := s.applyGovernanceAssessment(s.buildPendingExecution(task, taskIntent), assessment)
	approvalRequest := buildApprovalRequest(task.TaskID, taskIntent, assessment)
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", presentation.Text(presentation.MessageBubbleGovernancePending, nil), task.UpdatedAt.Format(dateTimeLayout))
	updatedTask := runengine.TaskRecord{}
	changed := false
	if s.isPreparedRestartAttempt(task) {
		updatedTask, changed = s.runEngine.MarkPreparedTaskWaitingApprovalWithPlan(task, approvalRequest, pendingExecution, bubble)
	} else {
		updatedTask, changed = s.runEngine.MarkWaitingApprovalWithPlan(task.TaskID, approvalRequest, pendingExecution, bubble)
	}
	if !changed {
		return task, TaskEntryResponse{}, false, ErrTaskNotFound
	}
	if err := s.persistApprovalRequestState(updatedTask.TaskID, approvalRequest, assessment.ImpactScope); err != nil {
		return task, TaskEntryResponse{}, false, err
	}
	response, err := buildTaskEntryResponse(&updatedTask, bubble, nil)
	if err != nil {
		return task, TaskEntryResponse{}, false, err
	}
	return updatedTask, response, true, nil
}

func (s *Service) maybePauseForRuntimeApproval(task runengine.TaskRecord, taskIntent map[string]any, result execution.Result) (runengine.TaskRecord, map[string]any, bool, error) {
	if result.LoopStopReason != string(agentloop.StopReasonNeedAuthorization) {
		return task, nil, false, nil
	}
	// Runtime tool approval differs from preflight approval: the model selected
	// the concrete tool and input during the Agent Loop, so the approval anchor
	// must be rebuilt from the recorded tool call instead of the broad task
	// intent. This keeps the paused task resumable without accepting unrelated
	// follow-up tool choices.
	assessment, ok, err := s.runtimeApprovalAssessment(task, result)
	if err != nil {
		return task, nil, false, err
	}
	if !ok {
		assessment = s.fallbackRuntimeApprovalAssessment(result)
	}
	pendingExecution := s.applyGovernanceAssessment(s.buildPendingExecution(task, taskIntent), assessment)
	if blockedToolCall, found := latestApprovalRequiredToolCall(result.ToolCalls); found {
		pendingExecution = applyRuntimeApprovedToolInput(pendingExecution, blockedToolCall.Input)
	}
	approvalRequest := buildApprovalRequest(task.TaskID, taskIntent, assessment)
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", presentation.Text(presentation.MessageBubbleGovernancePending, nil), task.UpdatedAt.Format(dateTimeLayout))
	updatedTask, changed := s.runEngine.MarkWaitingApprovalWithPlan(task.TaskID, approvalRequest, pendingExecution, bubble)
	if !changed {
		return task, nil, false, ErrTaskNotFound
	}
	if err := s.persistApprovalRequestState(updatedTask.TaskID, approvalRequest, assessment.ImpactScope); err != nil {
		return task, nil, false, err
	}
	return updatedTask, bubble, true, nil
}

// runtimeApprovalAssessment reconstructs the formal approval_request boundary
// from the tool call that stopped the Agent Loop. The executor is asked to
// re-assess the exact operation/input pair so approval replay uses the same
// target matching rules as normal preflight governance.
func (s *Service) runtimeApprovalAssessment(task runengine.TaskRecord, result execution.Result) (execution.GovernanceAssessment, bool, error) {
	toolCall, ok := latestApprovalRequiredToolCall(result.ToolCalls)
	if !ok {
		return execution.GovernanceAssessment{}, false, nil
	}
	runtimeIntent := map[string]any{
		"name":      toolCall.ToolName,
		"arguments": cloneMap(toolCall.Input),
	}
	if s.executor == nil {
		return normalizeRuntimeApprovalAssessment(execution.GovernanceAssessment{}, toolCall), true, nil
	}
	resultTitle, _, _ := resultSpecFromIntent(runtimeIntent)
	assessment, ok, err := s.executor.AssessGovernance(context.Background(), execution.Request{
		TaskID:       task.TaskID,
		RunID:        task.RunID,
		SourceType:   task.SourceType,
		Title:        task.Title,
		Intent:       runtimeIntent,
		Snapshot:     snapshotFromTask(task),
		DeliveryType: deliveryTypeFromIntent(runtimeIntent),
		ResultTitle:  resultTitle,
	})
	if err != nil {
		return execution.GovernanceAssessment{}, false, err
	}
	if !ok {
		return normalizeRuntimeApprovalAssessment(execution.GovernanceAssessment{}, toolCall), true, nil
	}
	return normalizeRuntimeApprovalAssessment(assessment, toolCall), true, nil
}

func (s *Service) fallbackRuntimeApprovalAssessment(result execution.Result) execution.GovernanceAssessment {
	toolCall, _ := latestApprovalRequiredToolCall(result.ToolCalls)
	return normalizeRuntimeApprovalAssessment(execution.GovernanceAssessment{}, toolCall)
}

func normalizeRuntimeApprovalAssessment(assessment execution.GovernanceAssessment, toolCall tools.ToolCallRecord) execution.GovernanceAssessment {
	assessment.ApprovalRequired = true
	if strings.TrimSpace(assessment.OperationName) == "" {
		assessment.OperationName = firstNonEmptyString(toolCall.ToolName, "agent_loop")
	}
	if strings.TrimSpace(assessment.TargetObject) == "" {
		assessment.TargetObject = runtimeApprovalTargetObject(toolCall)
	}
	if strings.TrimSpace(assessment.RiskLevel) == "" || assessment.RiskLevel == tools.RiskLevelGreen {
		assessment.RiskLevel = firstNonEmptyString(stringValue(toolCall.Output, "risk_level", ""), tools.RiskLevelYellow)
	}
	if strings.TrimSpace(assessment.Reason) == "" {
		assessment.Reason = firstNonEmptyString(stringValue(toolCall.Output, "reason", ""), firstNonEmptyString(stringValue(toolCall.Output, "deny_reason", ""), "policy_requires_authorization"))
	}
	if len(assessment.ImpactScope) == 0 {
		assessment.ImpactScope = cloneMap(mapValue(toolCall.Output, "impact_scope"))
	}
	return assessment
}

func latestApprovalRequiredToolCall(toolCalls []tools.ToolCallRecord) (tools.ToolCallRecord, bool) {
	for index := len(toolCalls) - 1; index >= 0; index-- {
		toolCall := toolCalls[index]
		if toolCallRequiresApproval(toolCall) {
			return toolCall, true
		}
	}
	return tools.ToolCallRecord{}, false
}

func toolCallRequiresApproval(toolCall tools.ToolCallRecord) bool {
	if toolCall.ErrorCode != nil && *toolCall.ErrorCode == tools.ToolErrorCodeApprovalRequired {
		return true
	}
	return toolCall.Status == tools.ToolCallStatusFailed && boolValue(toolCall.Output, "approval_required", false)
}

func runtimeApprovalTargetObject(toolCall tools.ToolCallRecord) string {
	impactScope := mapValue(toolCall.Output, "impact_scope")
	if webpages := stringSliceValue(impactScope["webpages"]); len(webpages) > 0 {
		return webpages[0]
	}
	if apps := stringSliceValue(impactScope["apps"]); len(apps) > 0 {
		return apps[0]
	}
	return impactScopeTarget(impactScope, execution.GovernanceTargetObject(toolCall.ToolName, toolCall.Input, nil))
}

// applyRuntimeApprovedToolInput persists the exact blocked tool payload onto the
// waiting-auth resume plan. Resume execution may still replay agent_loop, but
// governance bypass is then restricted to this concrete input instead of any
// later tool call that only shares the same target object.
func applyRuntimeApprovedToolInput(plan map[string]any, toolInput map[string]any) map[string]any {
	updatedPlan := cloneMap(plan)
	if updatedPlan == nil {
		updatedPlan = map[string]any{}
	}
	if len(toolInput) == 0 {
		delete(updatedPlan, "approved_tool_input")
		return updatedPlan
	}
	updatedPlan["approved_tool_input"] = cloneMap(toolInput)
	return updatedPlan
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

func (s *Service) blockTaskByAssessment(task runengine.TaskRecord, assessment execution.GovernanceAssessment) (TaskEntryResponse, runengine.TaskRecord, error) {
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
		return TaskEntryResponse{}, task, ErrTaskNotFound
	}
	auditRecord := s.writeGovernanceAuditRecord(updatedTask.TaskID, updatedTask.RunID, "risk", "intercept_operation", bubbleText, impactScopeTarget(assessment.ImpactScope, assessment.TargetObject), "denied")
	updatedTask = s.appendAuditData(updatedTask, compactAuditRecords(auditRecord), nil)
	response, err := buildTaskEntryResponse(&updatedTask, bubble, nil)
	if err != nil {
		return TaskEntryResponse{}, task, err
	}
	return response, updatedTask, nil
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
