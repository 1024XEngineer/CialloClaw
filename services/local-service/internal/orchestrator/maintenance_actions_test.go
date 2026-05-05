package orchestrator

import (
	"context"
	"errors"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

func TestServiceSettingsRestoreDefaultsRequestsApproval(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "restore defaults")
	result, err := service.SettingsRestoreDefaults(map[string]any{"request_meta": map[string]any{"trace_id": "trace_restore_defaults_001"}})
	if err != nil {
		t.Fatalf("SettingsRestoreDefaults returned error: %v", err)
	}
	task := result["task"].(map[string]any)
	approval := result["approval_request"].(map[string]any)
	if task["status"] != "waiting_auth" || approval["operation_name"] != maintenanceOperationSettingsRestoreDefaults {
		t.Fatalf("expected waiting approval payload, got task=%+v approval=%+v", task, approval)
	}
}

func TestServiceSecurityRespondAppliesSettingsRestoreDefaults(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "restore defaults")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	if _, err := service.SettingsUpdate(map[string]any{
		"general": map[string]any{"language": "en-US"},
		"memory":  map[string]any{"enabled": false},
		"task_automation": map[string]any{
			"inspect_on_file_change": false,
		},
	}); err != nil {
		t.Fatalf("SettingsUpdate returned error: %v", err)
	}
	if err := service.storage.SecretStore().PutSecret(context.Background(), storage.SecretRecord{
		Namespace: "model",
		Key:       service.defaultSettingsProvider() + "_api_key",
		Value:     "sk-live",
		UpdatedAt: "2026-04-18T10:00:00Z",
	}); err != nil {
		t.Fatalf("PutSecret returned error: %v", err)
	}
	request, err := service.SettingsRestoreDefaults(map[string]any{})
	if err != nil {
		t.Fatalf("SettingsRestoreDefaults returned error: %v", err)
	}
	taskID := request["task"].(map[string]any)["task_id"].(string)
	respondResult, err := service.SecurityRespond(map[string]any{"task_id": taskID, "decision": "allow_once"})
	if err != nil {
		t.Fatalf("SecurityRespond returned error: %v", err)
	}
	if respondResult["task"].(map[string]any)["status"] != "completed" {
		t.Fatalf("expected completed maintenance task, got %+v", respondResult)
	}
	recoveryPoints, total, err := service.storage.RecoveryPointStore().ListRecoveryPoints(context.Background(), taskID, 20, 0)
	if err != nil {
		t.Fatalf("ListRecoveryPoints returned error: %v", err)
	}
	if total != 1 || len(recoveryPoints) != 1 || len(recoveryPoints[0].Objects) == 0 {
		t.Fatalf("expected one persisted maintenance recovery point, got total=%d items=%+v", total, recoveryPoints)
	}
	if _, err := service.SecurityRestoreApply(map[string]any{"task_id": taskID, "recovery_point_id": recoveryPoints[0].RecoveryPointID}); !errors.Is(err, ErrRecoveryPointManualOnly) {
		t.Fatalf("expected manual maintenance recovery point to reject automatic restore, got %v", err)
	}
	settings := service.runEngine.Settings()
	if mapValue(settings, "general")["language"] != "zh-CN" || boolValue(mapValue(settings, "memory"), "enabled", false) != true {
		t.Fatalf("expected settings reset to defaults, got %+v", settings)
	}
	if boolValue(mapValue(settings, "task_automation"), "inspect_on_file_change", false) != true {
		t.Fatalf("expected task automation defaults restored, got %+v", settings)
	}
	if _, err := service.storage.SecretStore().GetSecret(context.Background(), "model", service.defaultSettingsProvider()+"_api_key"); !errors.Is(err, storage.ErrSecretNotFound) {
		t.Fatalf("expected model secret to be removed, got %v", err)
	}
}

func TestServiceSecurityRespondDeletesAllMemory(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "delete memory")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	if err := service.storage.MemoryStore().SaveSummary(context.Background(), storage.MemorySummaryRecord{MemorySummaryID: "mem_001", TaskID: "task_001", RunID: "run_001", Summary: "memory", CreatedAt: "2026-04-18T10:00:00Z"}); err != nil {
		t.Fatalf("SaveSummary returned error: %v", err)
	}
	request, err := service.MemoryDeleteAll(map[string]any{})
	if err != nil {
		t.Fatalf("MemoryDeleteAll returned error: %v", err)
	}
	taskID := request["task"].(map[string]any)["task_id"].(string)
	respondResult, err := service.SecurityRespond(map[string]any{"task_id": taskID, "decision": "allow_once"})
	if err != nil {
		t.Fatalf("SecurityRespond returned error: %v", err)
	}
	if respondResult["task"].(map[string]any)["status"] != "completed" {
		t.Fatalf("expected completed delete-memory task, got %+v", respondResult)
	}
	recoveryPoints, total, err := service.storage.RecoveryPointStore().ListRecoveryPoints(context.Background(), taskID, 20, 0)
	if err != nil {
		t.Fatalf("ListRecoveryPoints returned error: %v", err)
	}
	if total != 1 || len(recoveryPoints) != 1 {
		t.Fatalf("expected one persisted memory recovery point, got total=%d items=%+v", total, recoveryPoints)
	}
	summaries, err := service.storage.MemoryStore().ListRecentSummaries(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListRecentSummaries returned error: %v", err)
	}
	if len(summaries) != 0 {
		t.Fatalf("expected memory cleanup to clear summaries, got %+v", summaries)
	}
}

func TestServiceSecurityRespondDeletesTaskHistory(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "old task history")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	if _, err := service.SubmitInput(map[string]any{
		"request_meta": map[string]any{"trace_id": "trace_old_history_001"},
		"session_id":   "session_history_001",
		"source":       "dashboard",
		"trigger":      "voice_commit",
		"input": map[string]any{
			"type":       "text",
			"text":       "Summarize the weekly update.",
			"input_mode": "voice",
		},
	}); err != nil {
		t.Fatalf("SubmitInput returned error: %v", err)
	}
	request, err := service.TaskHistoryDelete(map[string]any{})
	if err != nil {
		t.Fatalf("TaskHistoryDelete returned error: %v", err)
	}
	taskID := request["task"].(map[string]any)["task_id"].(string)
	respondResult, err := service.SecurityRespond(map[string]any{"task_id": taskID, "decision": "allow_once"})
	if err != nil {
		t.Fatalf("SecurityRespond returned error: %v", err)
	}
	if respondResult["task"].(map[string]any)["status"] != "completed" {
		t.Fatalf("expected completed delete-history task, got %+v", respondResult)
	}
	tasks, total, err := service.storage.TaskStore().ListTasks(context.Background(), 20, 0)
	if err != nil {
		t.Fatalf("ListTasks returned error: %v", err)
	}
	if total != 1 || len(tasks) != 1 || tasks[0].TaskID != taskID {
		t.Fatalf("expected only maintenance task to remain, got total=%d items=%+v", total, tasks)
	}
	mirrorItems, mirrorTotal, err := service.storage.MirrorConversationStore().ListMirrorConversations(context.Background(), "", "", "", 20, 0)
	if err != nil {
		t.Fatalf("ListMirrorConversations returned error: %v", err)
	}
	if mirrorTotal != 0 || len(mirrorItems) != 0 {
		t.Fatalf("expected mirror history cleared, got total=%d items=%+v", mirrorTotal, mirrorItems)
	}
	recoveryPoints, recoveryTotal, err := service.storage.RecoveryPointStore().ListRecoveryPoints(context.Background(), taskID, 20, 0)
	if err != nil {
		t.Fatalf("ListRecoveryPoints returned error: %v", err)
	}
	if recoveryTotal != 1 || len(recoveryPoints) != 1 {
		t.Fatalf("expected task-history cleanup to preserve maintenance recovery point, got total=%d items=%+v", recoveryTotal, recoveryPoints)
	}
	authorizations, authTotal, err := service.storage.AuthorizationRecordStore().ListAuthorizationRecords(context.Background(), taskID, 20, 0)
	if err != nil {
		t.Fatalf("ListAuthorizationRecords returned error: %v", err)
	}
	if authTotal != 1 || len(authorizations) != 1 {
		t.Fatalf("expected maintenance authorization record to be preserved, got total=%d items=%+v", authTotal, authorizations)
	}
}
