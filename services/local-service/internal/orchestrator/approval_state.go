package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/execution"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

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
	ctx := context.Background()
	if err := s.storage.ApprovalRequestStore().WriteApprovalRequest(ctx, record); err != nil {
		return err
	}
	return s.retireStalePendingApprovalRequests(ctx, record.TaskID, record.ApprovalID, record.UpdatedAt)
}

func (s *Service) retireStalePendingApprovalRequests(ctx context.Context, taskID, activeApprovalID, updatedAt string) error {
	if strings.TrimSpace(taskID) == "" || strings.TrimSpace(activeApprovalID) == "" {
		return nil
	}
	records, err := s.listAllApprovalRequestsForTask(ctx, taskID)
	if err != nil {
		return err
	}
	for _, record := range records {
		if record.Status != "pending" || record.ApprovalID == activeApprovalID {
			continue
		}
		// A rebuilt authorization cycle replaces stale pending records without
		// implying that the user approved or denied those old requests.
		if err := s.storage.ApprovalRequestStore().UpdateApprovalRequestStatus(ctx, record.ApprovalID, "resolved", updatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) listAllApprovalRequestsForTask(ctx context.Context, taskID string) ([]storage.ApprovalRequestRecord, error) {
	if s == nil || s.storage == nil || s.storage.ApprovalRequestStore() == nil || strings.TrimSpace(taskID) == "" {
		return nil, nil
	}
	const pageSize = 100
	items := make([]storage.ApprovalRequestRecord, 0, pageSize)
	for offset := 0; ; offset += pageSize {
		page, total, err := s.storage.ApprovalRequestStore().ListApprovalRequests(ctx, taskID, pageSize, offset)
		if err != nil {
			return nil, err
		}
		items = append(items, page...)
		if len(items) >= total || len(page) < pageSize {
			break
		}
	}
	return items, nil
}

func (s *Service) persistAuthorizationDecision(task runengine.TaskRecord, authorizationRecord map[string]any) error {
	if s == nil || s.storage == nil || len(authorizationRecord) == 0 {
		return nil
	}
	approvalID := stringValue(authorizationRecord, "approval_id", "")
	recordID := stringValue(authorizationRecord, "authorization_record_id", "")
	if recordID == "" && approvalID != "" {
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
