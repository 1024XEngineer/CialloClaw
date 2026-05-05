package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

const maintenancePendingExecutionKind = "maintenance_action"

const (
	maintenanceOperationSettingsRestoreDefaults = "settings_restore_defaults"
	maintenanceOperationMemoryDeleteAll         = "memory_delete_all"
	maintenanceOperationTaskHistoryDelete       = "task_history_delete"
)

type maintenanceActionSpec struct {
	OperationName string
	TaskTitle     string
	TargetObject  string
	Reason        string
	RiskLevel     string
	PromptText    string
	SuccessTitle  string
	SuccessText   string
	SuccessType   string
	ImpactScope   map[string]any
}

// SettingsRestoreDefaults handles agent.settings.restore_defaults.
func (s *Service) SettingsRestoreDefaults(params map[string]any) (map[string]any, error) {
	return s.requestMaintenanceAction(params, maintenanceActionSpec{
		OperationName: maintenanceOperationSettingsRestoreDefaults,
		TaskTitle:     "恢复默认设置",
		TargetObject:  "settings_snapshot",
		Reason:        "settings_restore_defaults_requires_authorization",
		RiskLevel:     "red",
		PromptText:    "恢复默认设置会重置模型路由、本地偏好与巡检策略，并清除当前模型 API Key。请先确认授权。",
		SuccessTitle:  "默认设置已恢复",
		SuccessText:   "已恢复默认设置，并清除了当前模型 API Key。",
		SuccessType:   "settings_restore_defaults.completed",
		ImpactScope:   maintenanceImpactScope(currentDatabasePath(s.storage), currentSecretStorePath(s.storage)),
	})
}

// MemoryDeleteAll handles agent.memory.delete_all.
func (s *Service) MemoryDeleteAll(params map[string]any) (map[string]any, error) {
	return s.requestMaintenanceAction(params, maintenanceActionSpec{
		OperationName: maintenanceOperationMemoryDeleteAll,
		TaskTitle:     "删除全部记忆",
		TargetObject:  "memory_store",
		Reason:        "memory_delete_all_requires_authorization",
		RiskLevel:     "yellow",
		PromptText:    "删除全部记忆会清空本地长期记忆与命中记录，且无法直接恢复。请先确认授权。",
		SuccessTitle:  "记忆已清理",
		SuccessText:   "已删除全部记忆与命中记录。",
		SuccessType:   "memory_delete_all.completed",
		ImpactScope:   maintenanceImpactScope(currentDatabasePath(s.storage)),
	})
}

// TaskHistoryDelete handles agent.task.history.delete.
func (s *Service) TaskHistoryDelete(params map[string]any) (map[string]any, error) {
	return s.requestMaintenanceAction(params, maintenanceActionSpec{
		OperationName: maintenanceOperationTaskHistoryDelete,
		TaskTitle:     "删除任务历史",
		TargetObject:  "task_history",
		Reason:        "task_history_delete_requires_authorization",
		RiskLevel:     "red",
		PromptText:    "删除任务历史会清空历史任务、执行记录与治理回看数据，但不会删除已写入工作区的文件。请先确认授权。",
		SuccessTitle:  "任务历史已清理",
		SuccessText:   "已删除历史任务与执行记录，工作区中的已生成文件保持不变。",
		SuccessType:   "task_history_delete.completed",
		ImpactScope:   maintenanceImpactScope(currentDatabasePath(s.storage)),
	})
}

func (s *Service) requestMaintenanceAction(params map[string]any, spec maintenanceActionSpec) (map[string]any, error) {
	task := s.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:         "",
		RequestSource:     "",
		RequestTrigger:    "",
		Title:             spec.TaskTitle,
		SourceType:        "hover_input",
		Status:            "waiting_auth",
		Intent:            maintenanceIntent(spec.OperationName),
		PreferredDelivery: "bubble",
		FallbackDelivery:  "bubble",
		CurrentStep:       "waiting_authorization",
		RiskLevel:         spec.RiskLevel,
		Timeline:          initialTimeline("waiting_auth", "waiting_authorization"),
		Snapshot:          contextsvc.TaskContextSnapshot{},
	})
	s.publishTaskStart(task.TaskID, task.SessionID, requestTraceID(params))
	approvalRequest := map[string]any{
		"approval_id":    fmt.Sprintf("appr_%s", task.TaskID),
		"task_id":        task.TaskID,
		"operation_name": spec.OperationName,
		"risk_level":     spec.RiskLevel,
		"target_object":  spec.TargetObject,
		"reason":         spec.Reason,
		"status":         "pending",
		"created_at":     time.Now().Format(dateTimeLayout),
	}
	pendingExecution := map[string]any{
		"kind":                 maintenancePendingExecutionKind,
		"operation_name":       spec.OperationName,
		"target_object":        spec.TargetObject,
		"impact_scope":         cloneMap(spec.ImpactScope),
		"result_title":         spec.SuccessTitle,
		"result_bubble_text":   spec.SuccessText,
		"success_audit_action": spec.SuccessType,
	}
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", spec.PromptText, time.Now().Format(dateTimeLayout))
	updatedTask, ok := s.runEngine.MarkWaitingApprovalWithPlan(task.TaskID, approvalRequest, pendingExecution, bubble)
	if !ok {
		return nil, ErrTaskNotFound
	}
	if err := s.persistApprovalRequestState(updatedTask.TaskID, approvalRequest, spec.ImpactScope); err != nil {
		return nil, err
	}
	return map[string]any{
		"task":             taskMap(updatedTask),
		"approval_request": cloneMap(approvalRequest),
		"bubble_message":   bubble,
		"impact_scope":     cloneMap(spec.ImpactScope),
	}, nil
}

func (s *Service) executeMaintenanceActionAfterApproval(task runengine.TaskRecord, pendingExecution map[string]any) (runengine.TaskRecord, map[string]any, map[string]any, error) {
	switch strings.TrimSpace(stringValue(pendingExecution, "operation_name", "")) {
	case maintenanceOperationSettingsRestoreDefaults:
		updatedTask, bubble, deliveryResult := s.executeSettingsRestoreDefaults(task, pendingExecution)
		return updatedTask, bubble, deliveryResult, nil
	case maintenanceOperationMemoryDeleteAll:
		updatedTask, bubble, deliveryResult := s.executeMemoryDeleteAll(task, pendingExecution)
		return updatedTask, bubble, deliveryResult, nil
	case maintenanceOperationTaskHistoryDelete:
		updatedTask, bubble, deliveryResult := s.executeTaskHistoryDelete(task, pendingExecution)
		return updatedTask, bubble, deliveryResult, nil
	default:
		updatedTask, bubble := s.failMaintenanceAction(task, pendingExecution, fmt.Errorf("unsupported maintenance action"), nil)
		return updatedTask, bubble, nil, nil
	}
}

func (s *Service) executeSettingsRestoreDefaults(task runengine.TaskRecord, pendingExecution map[string]any) (runengine.TaskRecord, map[string]any, map[string]any) {
	recoveryPoint, err := s.prepareMaintenanceRecoveryPoint(task, pendingExecution)
	if err != nil {
		updatedTask, bubble := s.failMaintenanceAction(task, pendingExecution, err, nil)
		return updatedTask, bubble, nil
	}
	latestRestorePoint := maintenanceLatestRestorePoint(recoveryPoint)
	defaults := runengine.DefaultSettingsSnapshot()
	currentProvider := providerFromSettings(modelSettingsSection(s.runEngine.Settings()), s.defaultSettingsProvider())
	rollbacks := make([]modelSecretRollback, 0, 1)
	if currentProvider != "" {
		rollback, rollbackErr := s.captureModelSecretRollback(currentProvider)
		if rollbackErr != nil {
			updatedTask, bubble := s.failMaintenanceAction(task, pendingExecution, rollbackErr, latestRestorePoint)
			return updatedTask, bubble, nil
		}
		if err := s.deleteModelSecretIfPresent(currentProvider); err != nil {
			updatedTask, bubble := s.failMaintenanceAction(task, pendingExecution, err, latestRestorePoint)
			return updatedTask, bubble, nil
		}
		rollbacks = append(rollbacks, rollback)
	}
	if _, err := s.SettingsUpdate(defaults); err != nil {
		s.rollbackModelSecretMutations(rollbacks)
		updatedTask, bubble := s.failMaintenanceAction(task, pendingExecution, err, latestRestorePoint)
		return updatedTask, bubble, nil
	}
	s.runEngine.UpdateInspectorConfig(mapValue(defaults, "task_automation"))
	return s.completeMaintenanceAction(task, pendingExecution, latestRestorePoint)
}

func (s *Service) executeMemoryDeleteAll(task runengine.TaskRecord, pendingExecution map[string]any) (runengine.TaskRecord, map[string]any, map[string]any) {
	recoveryPoint, err := s.prepareMaintenanceRecoveryPoint(task, pendingExecution)
	if err != nil {
		updatedTask, bubble := s.failMaintenanceAction(task, pendingExecution, err, nil)
		return updatedTask, bubble, nil
	}
	latestRestorePoint := maintenanceLatestRestorePoint(recoveryPoint)
	if s.storage == nil {
		updatedTask, bubble := s.failMaintenanceAction(task, pendingExecution, fmt.Errorf("memory storage is unavailable"), latestRestorePoint)
		return updatedTask, bubble, nil
	}
	if err := s.storage.DeleteAllMemory(context.Background()); err != nil {
		updatedTask, bubble := s.failMaintenanceAction(task, pendingExecution, err, latestRestorePoint)
		return updatedTask, bubble, nil
	}
	return s.completeMaintenanceAction(task, pendingExecution, latestRestorePoint)
}

func (s *Service) executeTaskHistoryDelete(task runengine.TaskRecord, pendingExecution map[string]any) (runengine.TaskRecord, map[string]any, map[string]any) {
	recoveryPoint, err := s.prepareMaintenanceRecoveryPoint(task, pendingExecution)
	if err != nil {
		updatedTask, bubble := s.failMaintenanceAction(task, pendingExecution, err, nil)
		return updatedTask, bubble, nil
	}
	latestRestorePoint := maintenanceLatestRestorePoint(recoveryPoint)
	if s.storage == nil {
		updatedTask, bubble := s.failMaintenanceAction(task, pendingExecution, fmt.Errorf("task history storage is unavailable"), latestRestorePoint)
		return updatedTask, bubble, nil
	}
	if err := s.storage.DeleteAllTaskHistory(context.Background()); err != nil {
		_ = s.persistMaintenanceRecoveryPoint(recoveryPoint)
		updatedTask, bubble := s.failMaintenanceAction(task, pendingExecution, err, latestRestorePoint)
		return updatedTask, bubble, nil
	}
	if err := s.persistMaintenanceRecoveryPoint(recoveryPoint); err != nil {
		updatedTask, bubble := s.failMaintenanceAction(task, pendingExecution, err, latestRestorePoint)
		return updatedTask, bubble, nil
	}
	if err := s.clearRuntimeTaskHistory(task.TaskID); err != nil {
		updatedTask, bubble := s.failMaintenanceAction(task, pendingExecution, err, latestRestorePoint)
		return updatedTask, bubble, nil
	}
	return s.completeMaintenanceAction(task, pendingExecution, latestRestorePoint)
}

func (s *Service) completeMaintenanceAction(task runengine.TaskRecord, pendingExecution map[string]any, latestRestorePoint map[string]any) (runengine.TaskRecord, map[string]any, map[string]any) {
	resultTitle := firstNonEmptyString(stringValue(pendingExecution, "result_title", ""), "维护动作已完成")
	resultText := firstNonEmptyString(stringValue(pendingExecution, "result_bubble_text", ""), "操作已完成。")
	deliveryResult := s.delivery.BuildDeliveryResult(task.TaskID, "bubble", resultTitle, resultText)
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "result", resultText, time.Now().Format(dateTimeLayout))
	updatedTask, ok := s.runEngine.CompleteTask(task.TaskID, deliveryResult, bubble, nil, latestRestorePoint)
	if !ok {
		return task, bubble, deliveryResult
	}
	auditRecord := s.writeGovernanceAuditRecord(
		updatedTask.TaskID,
		updatedTask.RunID,
		"maintenance",
		stringValue(pendingExecution, "success_audit_action", stringValue(pendingExecution, "operation_name", "maintenance_action")),
		resultText,
		firstNonEmptyString(stringValue(pendingExecution, "target_object", ""), "maintenance_scope"),
		"success",
	)
	updatedTask = s.appendAuditData(updatedTask, compactAuditRecords(auditRecord), nil)
	return updatedTask, bubble, deliveryResult
}

func (s *Service) failMaintenanceAction(task runengine.TaskRecord, pendingExecution map[string]any, executionErr error, latestRestorePoint map[string]any) (runengine.TaskRecord, map[string]any) {
	impactScope := cloneMap(mapValue(pendingExecution, "impact_scope"))
	if len(impactScope) == 0 {
		impactScope = maintenanceImpactScope(currentDatabasePath(s.storage))
	}
	bubbleText := fmt.Sprintf("维护动作执行失败：%s", strings.TrimSpace(executionErr.Error()))
	bubble := s.delivery.BuildBubbleMessage(task.TaskID, "status", bubbleText, time.Now().Format(dateTimeLayout))
	updatedTask, ok := s.runEngine.FailTaskExecution(task.TaskID, "maintenance_failed", "execution_error", bubbleText, impactScope, bubble, latestRestorePoint)
	if !ok {
		return task, bubble
	}
	auditRecord := s.writeGovernanceAuditRecord(
		updatedTask.TaskID,
		updatedTask.RunID,
		"maintenance",
		stringValue(pendingExecution, "operation_name", "maintenance_action"),
		bubbleText,
		firstNonEmptyString(stringValue(pendingExecution, "target_object", ""), "maintenance_scope"),
		"failed",
	)
	updatedTask = s.appendAuditData(updatedTask, compactAuditRecords(auditRecord), nil)
	return updatedTask, bubble
}

func (s *Service) clearRuntimeTaskHistory(keepTaskID string) error {
	seen := make(map[string]struct{})
	for _, group := range []string{"unfinished", "finished"} {
		tasks, _ := s.runEngine.ListTasks(group, "updated_at", "desc", 0, 0)
		for _, task := range tasks {
			if task.TaskID == keepTaskID {
				continue
			}
			if _, exists := seen[task.TaskID]; exists {
				continue
			}
			seen[task.TaskID] = struct{}{}
			if err := s.runEngine.DeleteTask(task.TaskID); err != nil && err != runengine.ErrTaskNotFound {
				return err
			}
		}
	}
	return nil
}

func (s *Service) prepareMaintenanceRecoveryPoint(task runengine.TaskRecord, pendingExecution map[string]any) (checkpoint.RecoveryPoint, error) {
	if s == nil || s.storage == nil {
		return checkpoint.RecoveryPoint{}, nil
	}
	files := []string{currentDatabasePath(s.storage)}
	if stringValue(pendingExecution, "operation_name", "") == maintenanceOperationSettingsRestoreDefaults {
		files = append(files, currentSecretStorePath(s.storage))
	}
	return s.storage.CreateMaintenanceRecoveryPoint(context.Background(), task.TaskID, maintenanceRecoverySummary(pendingExecution), files)
}

func (s *Service) persistMaintenanceRecoveryPoint(point checkpoint.RecoveryPoint) error {
	if s == nil || s.storage == nil || point.RecoveryPointID == "" {
		return nil
	}
	if err := s.storage.RecoveryPointWriter().WriteRecoveryPoint(context.Background(), point); err != nil {
		return fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
	}
	return nil
}

func maintenanceRecoverySummary(pendingExecution map[string]any) string {
	operationName := stringValue(pendingExecution, "operation_name", "maintenance_action")
	return fmt.Sprintf("manual recovery assets before %s", operationName)
}

func maintenanceLatestRestorePoint(point checkpoint.RecoveryPoint) map[string]any {
	if strings.TrimSpace(point.RecoveryPointID) == "" {
		return nil
	}
	return recoveryPointMap(point)
}

func (s *Service) deleteModelSecretIfPresent(provider string) error {
	resolvedProvider := model.CanonicalProviderName(strings.TrimSpace(provider))
	if resolvedProvider == "" || s.storage == nil || s.storage.SecretStore() == nil {
		return nil
	}
	if err := s.storage.SecretStore().DeleteSecret(context.Background(), "model", resolvedProvider+"_api_key"); err != nil {
		normalizedErr := storage.NormalizeSecretStoreError(err)
		if normalizedErr == storage.ErrSecretNotFound {
			return nil
		}
		if normalizedErr == storage.ErrStrongholdAccessFailed || normalizedErr == storage.ErrStrongholdUnavailable || normalizedErr == storage.ErrSecretStoreAccessFailed {
			return ErrStrongholdAccessFailed
		}
		return normalizedErr
	}
	return nil
}

func (s *Service) persistStandaloneAuthorizationRecord(authorizationRecord map[string]any) error {
	if s == nil || s.storage == nil || len(authorizationRecord) == 0 {
		return nil
	}
	record := storage.AuthorizationRecordRecord{
		AuthorizationRecordID: stringValue(authorizationRecord, "authorization_record_id", ""),
		TaskID:                stringValue(authorizationRecord, "task_id", ""),
		ApprovalID:            stringValue(authorizationRecord, "approval_id", ""),
		Decision:              stringValue(authorizationRecord, "decision", "allow_once"),
		Operator:              stringValue(authorizationRecord, "operator", "user"),
		RememberRule:          boolValue(authorizationRecord, "remember_rule", false),
		CreatedAt:             stringValue(authorizationRecord, "created_at", time.Now().Format(dateTimeLayout)),
	}
	if err := s.storage.AuthorizationRecordStore().WriteAuthorizationRecord(context.Background(), record); err != nil {
		return fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
	}
	return nil
}

func maintenanceIntent(name string) map[string]any {
	return map[string]any{
		"name":      name,
		"arguments": map[string]any{},
	}
}

func maintenanceImpactScope(files ...string) map[string]any {
	cleaned := make([]string, 0, len(files))
	for _, file := range files {
		trimmed := strings.TrimSpace(file)
		if trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return map[string]any{
		"files":                    cleaned,
		"webpages":                 []string{},
		"apps":                     []string{},
		"out_of_workspace":         false,
		"overwrite_or_delete_risk": true,
	}
}
