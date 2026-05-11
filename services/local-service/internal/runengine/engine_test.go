// This test file covers runtime state-machine and notification queue behavior.
package runengine

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

type storageTestAdapter struct {
	databasePath string
}

type failingSettingsStore struct{}

type staticSettingsStore struct {
	snapshot map[string]any
}

func (failingSettingsStore) SaveSettingsSnapshot(context.Context, map[string]any) error {
	return errors.New("settings snapshot write failed")
}

func (failingSettingsStore) LoadSettingsSnapshot(context.Context) (map[string]any, error) {
	return nil, nil
}

func (s staticSettingsStore) SaveSettingsSnapshot(context.Context, map[string]any) error {
	return nil
}

func (s staticSettingsStore) LoadSettingsSnapshot(context.Context) (map[string]any, error) {
	return cloneMap(s.snapshot), nil
}

func (s storageTestAdapter) DatabasePath() string {
	return s.databasePath
}

func (s storageTestAdapter) SecretStorePath() string {
	if s.databasePath == "" {
		return ""
	}
	return s.databasePath + ".stronghold"
}

// TestEngineTaskLifecycle verifies the end-to-end task lifecycle.
func TestEngineTaskLifecycle(t *testing.T) {
	engine := NewEngine()
	fixedTime := time.Date(2026, 4, 8, 10, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return fixedTime }

	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_test",
		Title:       "整理测试任务",
		SourceType:  "selected_text",
		Status:      "confirming_intent",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{"style": "key_points"}},
		CurrentStep: "intent_confirmation",
		RiskLevel:   "green",
		Timeline: []TaskStepRecord{{
			Name:          "intent_confirmation",
			Status:        "pending",
			OrderIndex:    1,
			InputSummary:  "识别到文本对象",
			OutputSummary: "等待用户确认",
		}},
	})

	if task.TaskID == "" || task.RunID == "" {
		t.Fatal("expected task and run identifiers to be generated")
	}

	bubble := map[string]any{"task_id": task.TaskID, "type": "intent_confirm", "text": "请确认意图"}
	if _, ok := engine.SetPresentation(task.TaskID, bubble, nil, nil); !ok {
		t.Fatal("expected initial presentation to be stored")
	}

	confirmed, ok := engine.ConfirmTask(task.TaskID, "改写：整理测试任务", map[string]any{"name": "rewrite", "arguments": map[string]any{"tone": "professional"}}, map[string]any{"task_id": task.TaskID, "type": "status", "text": "开始处理"})
	if !ok {
		t.Fatal("expected task confirmation to succeed")
	}
	if confirmed.Title != "改写：整理测试任务" {
		t.Fatalf("expected confirmation to update task title, got %s", confirmed.Title)
	}
	if confirmed.Status != "processing" {
		t.Fatalf("expected processing status after confirmation, got %s", confirmed.Status)
	}
	if len(confirmed.Timeline) != 2 {
		t.Fatalf("expected timeline to append a generate step, got %d steps", len(confirmed.Timeline))
	}

	deliveryResult := map[string]any{"type": "workspace_document", "title": "测试结果", "payload": map[string]any{"path": "workspace/result.md", "task_id": task.TaskID}}
	artifacts := []map[string]any{{"artifact_id": "art_test", "task_id": task.TaskID, "artifact_type": "generated_doc"}}
	completed, ok := engine.CompleteTask(task.TaskID, deliveryResult, map[string]any{"task_id": task.TaskID, "type": "result", "text": "完成"}, artifacts)
	if !ok {
		t.Fatal("expected task completion to succeed")
	}
	if completed.Status != "completed" {
		t.Fatalf("expected completed status, got %s", completed.Status)
	}
	if completed.FinishedAt == nil {
		t.Fatal("expected finished_at to be set on completion")
	}

	finishedTasks, total := engine.ListTasks("finished", "updated_at", "desc", 10, 0)
	if total != 1 || len(finishedTasks) != 1 {
		t.Fatalf("expected completed task to appear in finished list, total=%d len=%d", total, len(finishedTasks))
	}

	notifications, ok := engine.PendingNotifications(task.TaskID)
	if !ok {
		t.Fatal("expected notifications to be available for task")
	}
	if len(notifications) < 3 {
		t.Fatalf("expected lifecycle notifications to be queued, got %d", len(notifications))
	}
	first := notifications[0]
	if first.Method != "task.updated" || first.Params["session_id"] != "sess_test" {
		t.Fatalf("expected task.updated notification to include session_id, got %+v", first)
	}
}

// TestEngineExecutionProgressAndToolCall verifies that execution-stage timeline
// and tool_call records are captured.
func TestEngineExecutionProgressAndToolCall(t *testing.T) {
	engine := NewEngine()
	engine.now = func() time.Time { return time.Date(2026, 4, 8, 10, 30, 0, 0, time.UTC) }

	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_exec",
		Title:       "执行测试任务",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{"style": "key_points"}},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
		Timeline: []TaskStepRecord{{
			Name:          "generate_output",
			Status:        "running",
			OrderIndex:    1,
			InputSummary:  "task input",
			OutputSummary: "等待生成",
		}},
	})

	started, ok := engine.BeginExecution(task.TaskID, "generate_output", "开始生成正式结果")
	if !ok {
		t.Fatal("expected begin execution to succeed")
	}
	if started.CurrentStep != "generate_output" {
		t.Fatalf("expected current step to remain generate_output, got %s", started.CurrentStep)
	}
	if started.Timeline[len(started.Timeline)-1].OutputSummary != "开始生成正式结果" {
		t.Fatalf("expected execution summary to update timeline, got %v", started.Timeline[len(started.Timeline)-1].OutputSummary)
	}

	recorded, ok := engine.RecordToolCall(task.TaskID, "write_file", map[string]any{"path": "workspace/result.md"}, map[string]any{"bytes": 128}, 32)
	if !ok {
		t.Fatal("expected tool call recording to succeed")
	}
	if recorded.LatestToolCall["tool_name"] != "write_file" {
		t.Fatalf("expected latest tool call to be write_file, got %v", recorded.LatestToolCall["tool_name"])
	}
	if recorded.LatestToolCall["duration_ms"] != int64(32) {
		t.Fatalf("expected duration_ms to be preserved, got %v", recorded.LatestToolCall["duration_ms"])
	}
	if recorded.LatestEvent["type"] != "tool_call.completed" {
		t.Fatalf("expected latest event to reflect tool_call.completed, got %v", recorded.LatestEvent["type"])
	}
	payload, ok := recorded.LatestEvent["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected latest event payload map, got %+v", recorded.LatestEvent)
	}
	if payload["tool_name"] != "write_file" || payload["path"] != "workspace/result.md" {
		t.Fatalf("expected tool event payload to carry output metadata, got %+v", payload)
	}
	notifications, ok := engine.PendingNotifications(task.TaskID)
	if !ok {
		t.Fatal("expected notifications to be available for task")
	}
	foundToolCallNotification := false
	for _, notification := range notifications {
		if notification.Method != "tool_call.completed" {
			continue
		}
		params := notification.Params
		if params["tool_name"] != "write_file" {
			t.Fatalf("expected tool_call.completed notification to carry tool name, got %+v", params)
		}
		foundToolCallNotification = true
	}
	if !foundToolCallNotification {
		t.Fatal("expected tool_call.completed notification to be queued")
	}
}

func TestEngineEmitRuntimeNotificationPersistsLoopStopReason(t *testing.T) {
	engine := NewEngine()
	engine.now = func() time.Time { return time.Date(2026, 4, 8, 11, 0, 0, 0, time.UTC) }

	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_runtime",
		Title:       "runtime stop reason",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "agent_loop"},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
	})

	record, ok := engine.EmitRuntimeNotification(task.TaskID, "loop.failed", map[string]any{
		"task_id":     task.TaskID,
		"stop_reason": "planner_error",
	})
	if !ok {
		t.Fatal("expected runtime notification to succeed")
	}
	if record.LoopStopReason != "planner_error" {
		t.Fatalf("expected loop stop reason to persist, got %+v", record)
	}

	listed, total := engine.ListTasks("unfinished", "updated_at", "desc", 10, 0)
	if total != 1 || len(listed) != 1 {
		t.Fatalf("expected runtime task to remain listed, total=%d len=%d", total, len(listed))
	}
	if listed[0].LoopStopReason != "planner_error" {
		t.Fatalf("expected task list to expose loop stop reason, got %+v", listed[0])
	}
}

// TestEngineAppendAuditDataPersistsAuditAndTokenUsage verifies audit and token
// usage persistence when runtime audit data is appended.
func TestEngineAppendAuditDataPersistsAuditAndTokenUsage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-run-audit.db")
	store, err := storage.NewSQLiteTaskRunStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteTaskRunStore returned error: %v", err)
	}

	engine, err := NewEngineWithStore(store)
	if err != nil {
		t.Fatalf("NewEngineWithStore returned error: %v", err)
	}
	engine.now = func() time.Time { return time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC) }

	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_audit",
		Title:       "persist audit",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize"},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
	})

	appended, ok := engine.AppendAuditData(task.TaskID, []map[string]any{{
		"audit_id":   "audit_001",
		"task_id":    task.TaskID,
		"type":       "model",
		"action":     "generate_text",
		"summary":    "generate text output",
		"target":     "summarize",
		"result":     "success",
		"created_at": time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}}, map[string]any{
		"total_tokens":   36,
		"estimated_cost": 0.0,
		"request_id":     "req_test",
	})
	if !ok {
		t.Fatal("expected append audit data to succeed")
	}
	if len(appended.AuditRecords) != 1 {
		t.Fatalf("expected audit record on runtime task, got %+v", appended.AuditRecords)
	}
	if appended.TokenUsage["total_tokens"] != 36 {
		t.Fatalf("expected token usage on runtime task, got %+v", appended.TokenUsage)
	}

	reloaded, err := NewEngineWithStore(store)
	if err != nil {
		t.Fatalf("NewEngineWithStore reload returned error: %v", err)
	}

	persisted, ok := reloaded.GetTask(task.TaskID)
	if !ok {
		t.Fatal("expected task to reload from sqlite")
	}
	if len(persisted.AuditRecords) != 1 {
		t.Fatalf("expected audit records to round-trip through storage, got %+v", persisted.AuditRecords)
	}
	if persisted.TokenUsage["total_tokens"] != float64(36) && persisted.TokenUsage["total_tokens"] != 36 {
		t.Fatalf("expected token usage to round-trip through storage, got %+v", persisted.TokenUsage)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}

func TestEngineAppendAuditDataMergesTokenUsage(t *testing.T) {
	engine := NewEngine()
	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_audit_merge",
		Title:       "merge audit",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize"},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
	})

	if _, ok := engine.AppendAuditData(task.TaskID, nil, map[string]any{
		"input_tokens":   12,
		"output_tokens":  4,
		"total_tokens":   16,
		"estimated_cost": 0.03,
		"request_id":     "req_first",
		"provider":       "openai",
		"model_id":       "gpt-first",
		"latency_ms":     int64(120),
	}); !ok {
		t.Fatal("expected first token usage append to succeed")
	}

	appended, ok := engine.AppendAuditData(task.TaskID, nil, map[string]any{
		"input_tokens":   8,
		"output_tokens":  3,
		"total_tokens":   11,
		"estimated_cost": 0.02,
		"request_id":     "req_second",
		"provider":       "openai",
		"model_id":       "gpt-second",
		"latency_ms":     int64(80),
	})
	if !ok {
		t.Fatal("expected second token usage append to succeed")
	}
	if appended.TokenUsage["input_tokens"] != 20 || appended.TokenUsage["output_tokens"] != 7 || appended.TokenUsage["total_tokens"] != 27 {
		t.Fatalf("expected token counts to merge, got %+v", appended.TokenUsage)
	}
	if appended.TokenUsage["estimated_cost"] != 0.05 {
		t.Fatalf("expected estimated_cost to merge, got %+v", appended.TokenUsage)
	}
	if appended.TokenUsage["request_id"] != "req_first" || appended.TokenUsage["model_id"] != "gpt-first" {
		t.Fatalf("expected representative request metadata to stay on the first execution call, got %+v", appended.TokenUsage)
	}
	if appended.TokenUsage["latency_ms"] != int64(120) {
		t.Fatalf("expected representative latency to stay on the first execution call, got %+v", appended.TokenUsage)
	}
}

func TestEngineUpdateIntentReservesNewTitleRefreshBeforeStatePublish(t *testing.T) {
	engine := NewEngine()
	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_title_refresh_reservation",
		Title:       "构建失败分析",
		SourceType:  "hover_input",
		Status:      "confirming_intent",
		Intent:      map[string]any{"name": "summarize"},
		CurrentStep: "intent_confirmation",
		RiskLevel:   "green",
	})
	initialToken := task.TitleRefreshToken
	if initialToken == 0 {
		t.Fatal("expected created task to carry a title refresh reservation")
	}

	updated, ok := engine.UpdateIntent(task.TaskID, task.Title, map[string]any{"name": "summarize", "arguments": map[string]any{"style": "bullets"}})
	if !ok {
		t.Fatal("expected UpdateIntent to succeed")
	}
	if updated.TitleRefreshToken == 0 || updated.TitleRefreshToken == initialToken {
		t.Fatalf("expected UpdateIntent to publish a newer title refresh reservation, got %+v", updated)
	}
	if _, ok := engine.UpdateTitleIfCurrent(task.TaskID, task.Title, initialToken, "旧上下文标题"); ok {
		t.Fatal("expected stale title refresh reservation to be rejected after UpdateIntent")
	}
	if refreshed, ok := engine.UpdateTitleIfCurrent(task.TaskID, task.Title, updated.TitleRefreshToken, "最新上下文标题"); !ok || refreshed.Title != "最新上下文标题" {
		t.Fatalf("expected latest reservation to remain valid, got %+v ok=%v", refreshed, ok)
	}
}

func TestEngineDefaultSettingsIncludeBudgetPolicy(t *testing.T) {
	engine := NewEngine()
	settings := engine.Settings()
	models := settings["models"].(map[string]any)
	credentials := models["credentials"].(map[string]any)
	policy := credentials["budget_policy"].(map[string]any)
	if credentials["budget_auto_downgrade"] != true {
		t.Fatalf("expected budget_auto_downgrade default to remain true, got %+v", credentials)
	}
	if policy["planner_retry_budget"] != 1 || policy["failure_signal_window"] != 2 || policy["token_pressure_threshold"] != 64 {
		t.Fatalf("expected default budget policy thresholds, got %+v", policy)
	}
	categories := policy["expensive_tool_categories"].([]string)
	if len(categories) != 3 || categories[0] != "command" {
		t.Fatalf("expected default expensive tool categories, got %+v", policy)
	}
}

func TestEngineUpdateSettingsMergesNestedBudgetPolicy(t *testing.T) {
	engine := NewEngine()
	effective, updatedKeys, applyMode, needRestart, err := engine.UpdateSettings(map[string]any{
		"models": map[string]any{
			"budget_policy": map[string]any{
				"failure_signal_window":     3,
				"planner_retry_budget":      2,
				"expensive_tool_categories": []any{"command", "browser_mutation", "media_heavy"},
			},
		},
	})
	if err != nil {
		t.Fatalf("update settings returned error: %v", err)
	}
	if applyMode != "immediate" || needRestart {
		t.Fatalf("expected budget policy update to stay immediate, got applyMode=%s needRestart=%v", applyMode, needRestart)
	}
	expectedKeys := []string{
		"models.credentials.budget_policy.expensive_tool_categories",
		"models.credentials.budget_policy.failure_signal_window",
		"models.credentials.budget_policy.planner_retry_budget",
	}
	if !reflect.DeepEqual(updatedKeys, expectedKeys) {
		t.Fatalf("expected nested budget policy leaf keys, got %+v", updatedKeys)
	}
	policyPatch := effective["models"].(map[string]any)["credentials"].(map[string]any)["budget_policy"].(map[string]any)
	if policyPatch["failure_signal_window"] != 3 || policyPatch["planner_retry_budget"] != 2 {
		t.Fatalf("expected effective settings to expose nested budget policy patch, got %+v", effective)
	}
	settings := engine.Settings()
	policy := settings["models"].(map[string]any)["credentials"].(map[string]any)["budget_policy"].(map[string]any)
	if policy["failure_signal_window"] != 3 || policy["planner_retry_budget"] != 2 {
		t.Fatalf("expected nested budget policy merge to persist, got %+v", policy)
	}
	categories := policy["expensive_tool_categories"].([]any)
	if len(categories) != 3 {
		t.Fatalf("expected updated expensive tool categories, got %+v", categories)
	}
}

func TestEngineUpdateSettingsEmitsLeafModelKeys(t *testing.T) {
	engine := NewEngine()
	_, updatedKeys, _, _, err := engine.UpdateSettings(map[string]any{
		"models": map[string]any{
			"provider": "anthropic",
			"base_url": "https://example.invalid/v1",
			"model":    "claude-test",
		},
	})
	if err != nil {
		t.Fatalf("update settings returned error: %v", err)
	}
	expectedKeys := []string{
		"models.credentials.base_url",
		"models.credentials.model",
		"models.provider",
	}
	if !reflect.DeepEqual(updatedKeys, expectedKeys) {
		t.Fatalf("expected leaf model keys, got %+v", updatedKeys)
	}
}

func TestEngineUpdateSettingsAccumulatesLegacyDataLogFields(t *testing.T) {
	engine := NewEngine()
	effective, updatedKeys, _, _, err := engine.UpdateSettings(map[string]any{
		"data_log": map[string]any{
			"provider":              "anthropic",
			"budget_auto_downgrade": false,
			"base_url":              "https://example.invalid/v1",
			"model":                 "claude-test",
			"budget_policy": map[string]any{
				"failure_signal_window": 4,
			},
		},
	})
	if err != nil {
		t.Fatalf("update settings returned error: %v", err)
	}
	expectedKeys := []string{
		"models.credentials.base_url",
		"models.credentials.budget_auto_downgrade",
		"models.credentials.budget_policy.failure_signal_window",
		"models.credentials.model",
		"models.provider",
	}
	if !reflect.DeepEqual(updatedKeys, expectedKeys) {
		t.Fatalf("expected legacy data_log keys to normalize into models credentials, got %+v", updatedKeys)
	}
	models := effective["models"].(map[string]any)
	credentials := models["credentials"].(map[string]any)
	if models["provider"] != "anthropic" {
		t.Fatalf("expected provider to normalize from legacy data_log, got %+v", models)
	}
	if credentials["budget_auto_downgrade"] != false || credentials["base_url"] != "https://example.invalid/v1" || credentials["model"] != "claude-test" {
		t.Fatalf("expected legacy data_log credentials to accumulate, got %+v", credentials)
	}
	budgetPolicy := credentials["budget_policy"].(map[string]any)
	if budgetPolicy["failure_signal_window"] != 4 {
		t.Fatalf("expected legacy budget policy to accumulate, got %+v", budgetPolicy)
	}
}

func TestEngineUpdateSettingsOnlyRequestsRestartWhenLanguageChanges(t *testing.T) {
	engine := NewEngine()
	_, updatedKeys, applyMode, needRestart, err := engine.UpdateSettings(map[string]any{
		"general": map[string]any{"language": "zh-CN"},
	})
	if err != nil {
		t.Fatalf("unchanged language update returned error: %v", err)
	}
	if applyMode != "immediate" || needRestart {
		t.Fatalf("expected unchanged language update to stay immediate, got applyMode=%s needRestart=%v updatedKeys=%+v", applyMode, needRestart, updatedKeys)
	}

	_, updatedKeys, applyMode, needRestart, err = engine.UpdateSettings(map[string]any{
		"general": map[string]any{"language": "en-US"},
	})
	if err != nil {
		t.Fatalf("changed language update returned error: %v", err)
	}
	if applyMode != "restart_required" || !needRestart {
		t.Fatalf("expected changed language update to require restart, got applyMode=%s needRestart=%v updatedKeys=%+v", applyMode, needRestart, updatedKeys)
	}
}

func TestEngineUpdateSettingsOnlyRequestsRestartWhenWorkspacePathChanges(t *testing.T) {
	engine := NewEngine()
	_, updatedKeys, applyMode, needRestart, err := engine.UpdateSettings(map[string]any{
		"general": map[string]any{
			"download": map[string]any{"workspace_path": "workspace"},
		},
	})
	if err != nil {
		t.Fatalf("unchanged workspace path update returned error: %v", err)
	}
	if applyMode != "immediate" || needRestart {
		t.Fatalf("expected unchanged workspace path update to stay immediate, got applyMode=%s needRestart=%v updatedKeys=%+v", applyMode, needRestart, updatedKeys)
	}

	_, updatedKeys, applyMode, needRestart, err = engine.UpdateSettings(map[string]any{
		"general": map[string]any{
			"download": map[string]any{"workspace_path": "workspace-next"},
		},
	})
	if err != nil {
		t.Fatalf("changed workspace path update returned error: %v", err)
	}
	if applyMode != "restart_required" || !needRestart {
		t.Fatalf("expected changed workspace path update to require restart, got applyMode=%s needRestart=%v updatedKeys=%+v", applyMode, needRestart, updatedKeys)
	}
}

func TestEngineSettingsStorePersistsAndReloadsSnapshot(t *testing.T) {
	storageService := storage.NewService(storageTestAdapter{databasePath: filepath.Join(t.TempDir(), "settings-persist.db")})
	defer func() { _ = storageService.Close() }()
	engine := NewEngine()
	if err := engine.WithSettingsStore(storageService.SettingsStore()); err != nil {
		t.Fatalf("attach settings store failed: %v", err)
	}
	if _, _, _, _, err := engine.UpdateSettings(map[string]any{
		"general": map[string]any{"language": "en-US"},
		"models":  map[string]any{"provider": "anthropic", "budget_auto_downgrade": false, "model": "claude-3-7-sonnet"},
	}); err != nil {
		t.Fatalf("update settings with persistence returned error: %v", err)
	}
	reloaded := NewEngine()
	if err := reloaded.WithSettingsStore(storageService.SettingsStore()); err != nil {
		t.Fatalf("reload settings store failed: %v", err)
	}
	settings := reloaded.Settings()
	if settings["general"].(map[string]any)["language"] != "en-US" {
		t.Fatalf("expected persisted general settings, got %+v", settings)
	}
	models := settings["models"].(map[string]any)
	credentials := models["credentials"].(map[string]any)
	if models["provider"] != "anthropic" || credentials["budget_auto_downgrade"] != false || credentials["model"] != "claude-3-7-sonnet" {
		t.Fatalf("expected persisted model settings, got %+v", settings)
	}
}

func TestEngineUpdateSettingsDoesNotMutateRuntimeWhenSnapshotPersistenceFails(t *testing.T) {
	engine := NewEngine()
	if err := engine.WithSettingsStore(failingSettingsStore{}); err != nil {
		t.Fatalf("attach failing settings store failed: %v", err)
	}
	before := engine.Settings()
	if _, _, _, _, err := engine.UpdateSettings(map[string]any{
		"general": map[string]any{"language": "en-US"},
		"models":  map[string]any{"provider": "anthropic", "model": "claude-3-7-sonnet"},
	}); err == nil {
		t.Fatal("expected update settings to fail when snapshot persistence fails")
	}
	after := engine.Settings()
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("expected runtime settings to remain unchanged on persistence failure, before=%+v after=%+v", before, after)
	}
}

func TestEngineSessionStorePersistsSessionLifecycle(t *testing.T) {
	storageService := storage.NewService(storageTestAdapter{databasePath: filepath.Join(t.TempDir(), "sessions-persist.db")})
	defer func() { _ = storageService.Close() }()
	engine := NewEngine()
	if err := engine.WithSessionStore(storageService.SessionStore()); err != nil {
		t.Fatalf("attach session store failed: %v", err)
	}
	task := engine.CreateTask(CreateTaskInput{SessionID: "sess_001", Title: "Session task", SourceType: "hover_input", Status: "processing", CurrentStep: "run", RiskLevel: "green"})
	session, err := storageService.SessionStore().GetSession(context.Background(), "sess_001")
	if err != nil || session.Status != "active" || session.Title != "Session task" {
		t.Fatalf("expected active session record after task creation, session=%+v err=%v", session, err)
	}
	if _, ok := engine.CompleteTask(task.TaskID, nil, nil, nil); !ok {
		t.Fatal("expected complete task to succeed")
	}
	session, err = storageService.SessionStore().GetSession(context.Background(), "sess_001")
	if err != nil || session.Status != "idle" {
		t.Fatalf("expected idle session record after completion, session=%+v err=%v", session, err)
	}
}

func TestEngineSessionStoreUsesMostRecentlyUpdatedTaskForSessionSnapshot(t *testing.T) {
	storageService := storage.NewService(storageTestAdapter{databasePath: filepath.Join(t.TempDir(), "sessions-freshness.db")})
	defer func() { _ = storageService.Close() }()
	engine := NewEngine()
	baseTime := time.Date(2026, 4, 22, 1, 23, 54, 0, time.UTC)
	tick := 0
	engine.now = func() time.Time {
		value := baseTime.Add(time.Duration(tick) * time.Second)
		tick++
		return value
	}
	if err := engine.WithSessionStore(storageService.SessionStore()); err != nil {
		t.Fatalf("attach session store failed: %v", err)
	}
	olderTask := engine.CreateTask(CreateTaskInput{SessionID: "sess_latest", Title: "Older task", SourceType: "hover_input", Status: "processing", CurrentStep: "collect_input", RiskLevel: "green"})
	newerTask := engine.CreateTask(CreateTaskInput{SessionID: "sess_latest", Title: "Newer task", SourceType: "hover_input", Status: "processing", CurrentStep: "collect_input", RiskLevel: "green"})
	if _, ok := engine.UpdateIntent(olderTask.TaskID, "Older task updated", map[string]any{"name": "summarize", "arguments": map[string]any{"style": "key_points"}}); !ok {
		t.Fatal("expected UpdateIntent to succeed for older task")
	}
	session, err := storageService.SessionStore().GetSession(context.Background(), "sess_latest")
	if err != nil {
		t.Fatalf("get session failed: %v", err)
	}
	if session.Title != "Older task updated" {
		t.Fatalf("expected session title to come from most recently updated task, got %+v", session)
	}
	newerRecord, ok := engine.GetTask(newerTask.TaskID)
	if !ok {
		t.Fatal("expected newer task to remain in runtime")
	}
	if session.UpdatedAt == newerRecord.UpdatedAt.Format(time.RFC3339Nano) {
		t.Fatalf("expected session updated_at to move beyond stale newer task snapshot, got %+v newer=%+v", session, newerRecord)
	}
}

func TestEngineInspectorConfigAndRuntimeBuffers(t *testing.T) {
	engine := NewEngine()
	updatedInspector := engine.UpdateInspectorConfig(map[string]any{
		"task_sources":           []any{"D:/workspace/todos", "D:/workspace/backlog"},
		"inspection_interval":    map[string]any{"unit": "minute", "value": 10},
		"inspect_on_file_change": false,
		"inspect_on_startup":     true,
		"remind_before_deadline": false,
		"remind_when_stale":      true,
	})
	if !reflect.DeepEqual(updatedInspector["task_sources"], []string{"D:/workspace/todos", "D:/workspace/backlog"}) {
		t.Fatalf("expected inspector task sources to update, got %+v", updatedInspector)
	}

	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_runtime_buffers",
		Title:       "Runtime buffer task",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{}},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
	})
	if _, ok := engine.SetCitations(task.TaskID, []map[string]any{{"citation_id": "cit_001"}}); !ok {
		t.Fatal("expected SetCitations to succeed")
	}
	if _, ok := engine.RecordLoopLifecycle(task.TaskID, "loop.round.completed", "waiting_for_retry", map[string]any{"round": 1}); !ok {
		t.Fatal("expected RecordLoopLifecycle to succeed")
	}
	if _, ok := engine.AppendSteeringMessage(task.TaskID, "Please adjust the summary.", map[string]any{"type": "status", "text": "Adjusting summary"}); !ok {
		t.Fatal("expected AppendSteeringMessage to succeed")
	}
	if _, ok := engine.UpdateSecuritySummary(task.TaskID, map[string]any{"security_status": "warning", "risk_level": "yellow"}); !ok {
		t.Fatal("expected UpdateSecuritySummary to succeed")
	}
	if _, ok := engine.MarkWaitingApproval(task.TaskID, map[string]any{"approval_id": "appr_001"}, map[string]any{"type": "status", "text": "Waiting approval"}); !ok {
		t.Fatal("expected MarkWaitingApproval to succeed")
	}
	approvals, total := engine.PendingApprovalRequests(10, 0)
	if total != 1 || len(approvals) != 1 || approvals[0]["approval_id"] != "appr_001" {
		t.Fatalf("expected one pending approval request, got total=%d approvals=%+v", total, approvals)
	}
	detail, ok := engine.TaskDetail(task.TaskID)
	if !ok || len(detail.Citations) != 1 || detail.SecuritySummary["pending_authorizations"] != 1 {
		t.Fatalf("expected task detail snapshot to expose citations and security summary, got %+v", detail)
	}
	notifications, ok := engine.PendingNotifications(task.TaskID)
	if !ok || len(notifications) == 0 {
		t.Fatalf("expected pending notifications after runtime events, got %+v ok=%v", notifications, ok)
	}
	drained, ok := engine.DrainNotifications(task.TaskID)
	if !ok || len(drained) != len(notifications) {
		t.Fatalf("expected DrainNotifications to return all buffered notifications, got %+v ok=%v", drained, ok)
	}
	notifications, ok = engine.PendingNotifications(task.TaskID)
	if !ok || len(notifications) != 0 {
		t.Fatalf("expected notification buffer to be empty after draining, got %+v ok=%v", notifications, ok)
	}
	if err := engine.DeleteTask(task.TaskID); err != nil {
		t.Fatalf("expected DeleteTask to succeed, got %v", err)
	}
	if _, ok := engine.TaskDetail(task.TaskID); ok {
		t.Fatal("expected deleted task to be removed from runtime")
	}
}

func TestEngineAuthorizationAndHandoffState(t *testing.T) {
	engine := NewEngine()
	fixedTime := time.Date(2026, 4, 8, 11, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return fixedTime }
	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_auth",
		Title:       "需要授权的任务",
		SourceType:  "dragged_file",
		Status:      "processing",
		Intent:      map[string]any{"name": "write_file", "arguments": map[string]any{"require_authorization": true}},
		CurrentStep: "generate_output",
		RiskLevel:   "red",
		Timeline: []TaskStepRecord{{
			Name:          "generate_output",
			Status:        "running",
			OrderIndex:    1,
			InputSummary:  "开始处理文件",
			OutputSummary: "等待后续处理",
		}},
	})

	approvalRequest := map[string]any{
		"approval_id":    "appr_test",
		"task_id":        task.TaskID,
		"operation_name": "write_file",
		"risk_level":     "red",
		"target_object":  "workspace_document",
		"reason":         "policy_requires_authorization",
		"status":         "pending",
	}
	pendingExecution := map[string]any{
		"task_id":            task.TaskID,
		"delivery_type":      "workspace_document",
		"result_title":       "文件写入结果",
		"preview_text":       "已为你写入文档并打开",
		"result_bubble_text": "文件已经生成，可直接查看。",
	}
	bubble := map[string]any{"task_id": task.TaskID, "type": "status", "text": "等待授权"}
	waitingTask, ok := engine.MarkWaitingApprovalWithPlan(task.TaskID, approvalRequest, pendingExecution, bubble)
	if !ok {
		t.Fatal("expected waiting approval transition to succeed")
	}
	if waitingTask.Status != "waiting_auth" {
		t.Fatalf("expected waiting_auth status, got %s", waitingTask.Status)
	}
	if waitingTask.PendingExecution["delivery_type"] != "workspace_document" {
		t.Fatal("expected pending execution plan to be stored with waiting task")
	}

	memoryReadPlans := []map[string]any{{"kind": "retrieval", "task_id": task.TaskID}}
	memoryWritePlans := []map[string]any{{"kind": "summary_write", "task_id": task.TaskID}}
	if _, ok := engine.SetMemoryPlans(task.TaskID, memoryReadPlans, memoryWritePlans); !ok {
		t.Fatal("expected memory handoff plans to be stored")
	}

	storagePlan := map[string]any{"task_id": task.TaskID, "target_path": "workspace/result.md"}
	artifactPlans := []map[string]any{{"task_id": task.TaskID, "artifact_id": "art_test"}}
	if _, ok := engine.SetDeliveryPlans(task.TaskID, storagePlan, artifactPlans); !ok {
		t.Fatal("expected delivery handoff plans to be stored")
	}

	record, ok := engine.GetTask(task.TaskID)
	if !ok {
		t.Fatal("expected task to remain available")
	}
	if len(record.MemoryReadPlans) != 1 || len(record.MemoryWritePlans) != 1 {
		t.Fatal("expected memory handoff plans to be present on task record")
	}
	if record.StorageWritePlan["target_path"] != "workspace/result.md" {
		t.Fatal("expected storage handoff target path to be stored")
	}

	notifications, ok := engine.PendingNotifications(task.TaskID)
	if !ok {
		t.Fatal("expected approval notifications to be available")
	}
	lastNotification := notifications[len(notifications)-1]
	if lastNotification.Method != "approval.pending" {
		t.Fatalf("expected last notification to be approval.pending, got %s", lastNotification.Method)
	}

	processingBubble := map[string]any{"task_id": task.TaskID, "type": "status", "text": "继续执行"}
	resumedTask, ok := engine.ResumeAfterApproval(task.TaskID, map[string]any{"decision": "allow_once"}, map[string]any{"files": []string{}}, processingBubble)
	if !ok {
		t.Fatal("expected authorized task to resume")
	}
	if resumedTask.Status != "processing" {
		t.Fatalf("expected resumed task to return to processing, got %s", resumedTask.Status)
	}

	deniedEngine := NewEngine()
	deniedTask := deniedEngine.CreateTask(CreateTaskInput{
		SessionID:   "sess_deny",
		Title:       "拒绝授权的任务",
		SourceType:  "dragged_file",
		Status:      "processing",
		Intent:      map[string]any{"name": "write_file", "arguments": map[string]any{"require_authorization": true}},
		CurrentStep: "generate_output",
		RiskLevel:   "red",
		Timeline: []TaskStepRecord{{
			Name:          "generate_output",
			Status:        "running",
			OrderIndex:    1,
			InputSummary:  "开始处理文件",
			OutputSummary: "等待后续处理",
		}},
	})
	deniedApprovalRequest := map[string]any{
		"approval_id":    "appr_deny",
		"task_id":        deniedTask.TaskID,
		"operation_name": "write_file",
		"risk_level":     "red",
		"target_object":  "workspace_document",
		"reason":         "policy_requires_authorization",
		"status":         "pending",
	}
	_, _ = deniedEngine.MarkWaitingApproval(deniedTask.TaskID, deniedApprovalRequest, bubble)
	deniedResult, ok := deniedEngine.DenyAfterApproval(deniedTask.TaskID, map[string]any{"decision": "deny_once"}, map[string]any{"files": []string{}}, map[string]any{"task_id": deniedTask.TaskID, "type": "status", "text": "已拒绝"})
	if !ok {
		t.Fatal("expected deny flow to succeed")
	}
	if deniedResult.Status != "cancelled" {
		t.Fatalf("expected denied task to be cancelled, got %s", deniedResult.Status)
	}
}

func TestEngineSessionQueueBlocksAndResumesQueuedTasks(t *testing.T) {
	engine := NewEngine()
	now := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }

	active := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_queue",
		Title:       "active task",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "agent_loop"},
		CurrentStep: "agent_loop",
		RiskLevel:   "green",
		Timeline: []TaskStepRecord{{
			Name:          "agent_loop",
			Status:        "running",
			OrderIndex:    1,
			InputSummary:  "input",
			OutputSummary: "running",
		}},
	})

	queued := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_queue",
		Title:       "queued task",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "agent_loop"},
		CurrentStep: "agent_loop",
		RiskLevel:   "green",
		Timeline: []TaskStepRecord{{
			Name:          "agent_loop",
			Status:        "running",
			OrderIndex:    1,
			InputSummary:  "input",
			OutputSummary: "running",
		}},
	})

	activeTask, ok := engine.ActiveSessionTask("sess_queue", queued.TaskID)
	if !ok || activeTask.TaskID != active.TaskID {
		t.Fatalf("expected active session task to be the first task, got %+v ok=%v", activeTask, ok)
	}

	blocked, ok := engine.QueueTaskForSession(queued.TaskID, active.TaskID, map[string]any{"task_id": queued.TaskID, "type": "status", "text": "queued"})
	if !ok {
		t.Fatal("expected queue transition to succeed")
	}
	if blocked.Status != "blocked" || blocked.CurrentStep != "session_queue" {
		t.Fatalf("expected queued task to enter blocked/session_queue, got %+v", blocked)
	}

	next, ok := engine.NextQueuedTaskForSession("sess_queue")
	if !ok || next.TaskID != queued.TaskID {
		t.Fatalf("expected next queued task lookup to find queued task, got %+v ok=%v", next, ok)
	}

	resumed, ok := engine.ResumeQueuedTask(queued.TaskID, "agent_loop", map[string]any{"task_id": queued.TaskID, "type": "status", "text": "resume"})
	if !ok {
		t.Fatal("expected queued task resume to succeed")
	}
	if resumed.Status != "processing" || resumed.CurrentStep != "agent_loop" {
		t.Fatalf("expected resumed task to return to processing/agent_loop, got %+v", resumed)
	}
	if resumed.LatestEvent["type"] != "task.session_resumed" {
		t.Fatalf("expected session resumed event, got %+v", resumed.LatestEvent)
	}
}

func TestEngineEscalateHumanLoopBlocksTaskWithStructuredPayload(t *testing.T) {
	engine := NewEngine()
	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_hitl",
		Title:       "trace escalation",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "agent_loop"},
		CurrentStep: "agent_loop",
		RiskLevel:   "yellow",
	})
	bubble := map[string]any{"task_id": task.TaskID, "type": "status", "text": "需要人工介入"}
	escalated, ok := engine.EscalateHumanLoop(task.TaskID, map[string]any{"reason": "doom_loop", "status": "pending"}, bubble)
	if !ok {
		t.Fatal("expected human escalation to succeed")
	}
	if escalated.Status != "blocked" || escalated.CurrentStep != "human_in_loop" {
		t.Fatalf("expected blocked human_in_loop task, got %+v", escalated)
	}
	if escalated.PendingExecution["kind"] != "human_in_loop" {
		t.Fatalf("expected pending execution to carry human loop kind, got %+v", escalated.PendingExecution)
	}
	payload, ok := escalated.PendingExecution["escalation"].(map[string]any)
	if !ok || payload["reason"] != "doom_loop" {
		t.Fatalf("expected escalation payload to be preserved, got %+v", escalated.PendingExecution)
	}
}

func TestEngineControlTaskResumeSupportsHumanLoopBlockedTasks(t *testing.T) {
	engine := NewEngine()
	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_hitl_resume",
		Title:       "trace escalation",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "agent_loop"},
		CurrentStep: "agent_loop",
		RiskLevel:   "yellow",
	})
	if _, ok := engine.EscalateHumanLoop(task.TaskID, map[string]any{"reason": "doom_loop", "status": "pending"}, map[string]any{"task_id": task.TaskID, "type": "status", "text": "需要人工介入"}); !ok {
		t.Fatal("expected human escalation to succeed")
	}

	resumed, err := engine.ControlTask(task.TaskID, "resume", map[string]any{"task_id": task.TaskID, "type": "status", "text": "人工复核完成"})
	if err != nil {
		t.Fatalf("expected resume from human_in_loop to succeed, got %v", err)
	}
	if resumed.Status != "processing" || resumed.CurrentStep != "agent_loop" {
		t.Fatalf("expected human loop resume to restore processing/agent_loop, got %+v", resumed)
	}
	if resumed.PendingExecution == nil || resumed.PendingExecution["kind"] != "human_in_loop" {
		t.Fatalf("expected human loop resume to preserve pending escalation payload until orchestrator consumes it, got %+v", resumed.PendingExecution)
	}
}

func TestEngineControlTaskResumePreservesPromptFallbackStep(t *testing.T) {
	engine := NewEngine()
	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_prompt_fallback_resume",
		Title:       "agent loop prompt fallback",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "agent_loop"},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
	})
	if _, err := engine.ControlTask(task.TaskID, "pause", map[string]any{"task_id": task.TaskID, "type": "status", "text": "paused"}); err != nil {
		t.Fatalf("expected pause to succeed, got %v", err)
	}
	resumed, err := engine.ControlTask(task.TaskID, "resume", map[string]any{"task_id": task.TaskID, "type": "status", "text": "resumed"})
	if err != nil {
		t.Fatalf("expected resume to succeed, got %v", err)
	}
	if resumed.Status != "processing" || resumed.CurrentStep != "generate_output" {
		t.Fatalf("expected prompt fallback resume to preserve generate_output, got %+v", resumed)
	}
}

func TestEngineCompleteTaskClearsPendingHumanLoopPayload(t *testing.T) {
	engine := NewEngine()
	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_hitl_complete",
		Title:       "trace escalation",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize"},
		CurrentStep: "generate_output",
		RiskLevel:   "yellow",
	})
	if _, ok := engine.EscalateHumanLoop(task.TaskID, map[string]any{"reason": "doom_loop", "status": "pending", "suggested_action": "review_and_replan"}, map[string]any{"task_id": task.TaskID, "type": "status", "text": "需要人工介入"}); !ok {
		t.Fatal("expected human escalation to succeed")
	}
	if _, err := engine.ControlTask(task.TaskID, "resume", map[string]any{"task_id": task.TaskID, "type": "status", "text": "人工复核完成"}); err != nil {
		t.Fatalf("expected resume from human loop to succeed, got %v", err)
	}
	completed, ok := engine.CompleteTask(task.TaskID, map[string]any{"type": "workspace_document"}, map[string]any{"task_id": task.TaskID, "type": "result", "text": "完成"}, nil)
	if !ok {
		t.Fatal("expected complete task to succeed")
	}
	if completed.PendingExecution != nil || completed.ApprovalRequest != nil {
		t.Fatalf("expected completion to clear pending human loop payload, got %+v", completed)
	}
}

func TestEngineReopenIntentConfirmationResetsExecutionState(t *testing.T) {
	engine := NewEngine()
	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_replan",
		Title:       "old title",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize"},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
		DeliveryResult: map[string]any{
			"type": "workspace_document",
		},
		Artifacts:        []map[string]any{{"artifact_id": "art_001"}},
		MirrorReferences: []map[string]any{{"memory_id": "mem_001"}},
	})
	if _, ok := engine.ResolveAuthorization(task.TaskID, map[string]any{"decision": "allow_once"}, map[string]any{"files": []string{"workspace/out.md"}}); !ok {
		t.Fatal("expected authorization record to be stored before reopen")
	}
	if _, ok := engine.SetMemoryPlans(task.TaskID, []map[string]any{{"memory_id": "read_001"}}, []map[string]any{{"memory_id": "write_001"}}); !ok {
		t.Fatal("expected memory plans to be stored before reopen")
	}
	if _, ok := engine.SetDeliveryPlans(task.TaskID, map[string]any{"target_path": "workspace/out.md"}, []map[string]any{{"artifact_id": "plan_001"}}); !ok {
		t.Fatal("expected delivery plans to be stored before reopen")
	}
	if updated, ok := engine.SetMirrorReferences(task.TaskID, []map[string]any{{"memory_id": "mirror_001"}}); !ok {
		t.Fatal("expected mirror references to be stored before reopen")
	} else {
		task = updated
	}
	if _, ok := engine.EscalateHumanLoop(task.TaskID, map[string]any{"reason": "doom_loop", "status": "pending"}, map[string]any{"task_id": task.TaskID, "type": "status", "text": "需要人工介入"}); !ok {
		t.Fatal("expected human escalation to succeed before reopen")
	}
	reopened, ok := engine.ReopenIntentConfirmation(task.TaskID, "new title", map[string]any{"name": "translate"}, map[string]any{"task_id": task.TaskID, "type": "status", "text": "需要重新确认"})
	if !ok {
		t.Fatal("expected reopen intent confirmation to succeed")
	}
	if reopened.Status != "confirming_intent" || reopened.CurrentStep != "confirming_intent" {
		t.Fatalf("expected task to return to confirming_intent, got %+v", reopened)
	}
	if reopened.PendingExecution != nil || reopened.DeliveryResult != nil || len(reopened.Artifacts) != 0 {
		t.Fatalf("expected reopen to clear execution outputs, got %+v", reopened)
	}
	if reopened.Authorization != nil || reopened.ImpactScope != nil {
		t.Fatalf("expected reopen to clear authorization state, got %+v", reopened)
	}
	if reopened.StorageWritePlan != nil || len(reopened.ArtifactPlans) != 0 || len(reopened.MemoryReadPlans) != 0 || len(reopened.MemoryWritePlans) != 0 || len(reopened.MirrorReferences) != 0 {
		t.Fatalf("expected reopen to clear handoff plans, got %+v", reopened)
	}
	if reopened.Title != "new title" || stringValue(reopened.Intent, "name", "") != "translate" {
		t.Fatalf("expected reopen to persist updated title/intent, got %+v", reopened)
	}
}

func TestEngineReopenIntentConfirmationReturnsFalseForMissingTask(t *testing.T) {
	engine := NewEngine()
	if reopened, ok := engine.ReopenIntentConfirmation("task_missing", "new title", map[string]any{"name": "translate"}, nil); ok || reopened.TaskID != "" {
		t.Fatalf("expected reopen intent confirmation to fail for missing task, got %+v ok=%v", reopened, ok)
	}
}

func TestEngineResolveAuthorizationClearsPendingPlanAndKeepsRestorePoint(t *testing.T) {
	engine := NewEngine()
	now := time.Date(2026, 4, 11, 9, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }

	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_resolve",
		Title:       "待授权任务",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "write_file"},
		CurrentStep: "generate_output",
		RiskLevel:   "yellow",
	})
	approvalRequest := map[string]any{"approval_id": "appr_resolve", "task_id": task.TaskID, "status": "pending"}
	pendingExecution := map[string]any{"operation_name": "write_file", "target_object": "workspace/notes/a.md"}
	bubble := map[string]any{"task_id": task.TaskID, "type": "status", "text": "等待授权"}
	if _, ok := engine.MarkWaitingApprovalWithPlan(task.TaskID, approvalRequest, pendingExecution, bubble); !ok {
		t.Fatal("expected waiting approval transition to succeed")
	}

	record, ok := engine.GetTask(task.TaskID)
	if !ok {
		t.Fatal("expected task record")
	}
	record.SecuritySummary = map[string]any{
		"security_status":        "pending_authorization",
		"risk_level":             "yellow",
		"pending_authorizations": 1,
		"latest_restore_point": map[string]any{
			"recovery_point_id": "rp_keep",
		},
	}
	engine.tasks[task.TaskID] = &record

	resolved, ok := engine.ResolveAuthorization(task.TaskID, map[string]any{"decision": "allow_once"}, map[string]any{"files": []string{"workspace/notes/a.md"}})
	if !ok {
		t.Fatal("expected resolve authorization to succeed")
	}
	if resolved.PendingExecution != nil || resolved.ApprovalRequest != nil {
		t.Fatalf("expected pending authorization data cleared, got %+v", resolved)
	}
	if resolved.Authorization["decision"] != "allow_once" {
		t.Fatalf("expected authorization stored, got %+v", resolved.Authorization)
	}
	latestRestore, _ := resolved.SecuritySummary["latest_restore_point"].(map[string]any)
	if latestRestore["recovery_point_id"] != "rp_keep" {
		t.Fatalf("expected latest restore point to be preserved, got %+v", resolved.SecuritySummary)
	}

	plan, ok := engine.PendingExecutionPlan(task.TaskID)
	if ok || plan != nil {
		t.Fatalf("expected no pending execution plan after resolve, got %+v", plan)
	}
}

func TestEngineApplyRecoveryOutcomeSetsTerminalAndNonTerminalStatus(t *testing.T) {
	engine := NewEngine()
	now := time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }

	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_restore",
		Title:       "恢复任务",
		SourceType:  "hover_input",
		Status:      "waiting_auth",
		Intent:      map[string]any{"name": "restore_apply"},
		CurrentStep: "restore_apply",
		RiskLevel:   "red",
	})
	if _, ok := engine.MarkWaitingApprovalWithPlan(task.TaskID, map[string]any{"approval_id": "appr_restore"}, map[string]any{"operation_name": "restore_apply"}, map[string]any{"text": "等待授权"}); !ok {
		t.Fatal("expected waiting approval state")
	}

	recoveryPoint := map[string]any{"recovery_point_id": "rp_done"}
	completed, ok := engine.ApplyRecoveryOutcome(task.TaskID, "completed", "recovered", recoveryPoint, map[string]any{"text": "恢复完成"})
	if !ok {
		t.Fatal("expected apply recovery outcome to succeed")
	}
	if completed.Status != "completed" || completed.FinishedAt == nil {
		t.Fatalf("expected completed terminal state with finished_at, got %+v", completed)
	}
	if completed.PendingExecution != nil || completed.ApprovalRequest != nil {
		t.Fatalf("expected approval artifacts cleared, got %+v", completed)
	}
	if completed.LatestEvent["type"] != "recovery.applied" {
		t.Fatalf("expected recovery.applied event, got %+v", completed.LatestEvent)
	}

	processingTask := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_restore_retry",
		Title:       "恢复重试",
		SourceType:  "hover_input",
		Status:      "waiting_auth",
		Intent:      map[string]any{"name": "restore_apply"},
		CurrentStep: "restore_apply",
		RiskLevel:   "red",
	})
	processing, ok := engine.ApplyRecoveryOutcome(processingTask.TaskID, "processing", "recovery_failed", recoveryPoint, map[string]any{"text": "恢复失败"})
	if !ok {
		t.Fatal("expected non-terminal recovery outcome to succeed")
	}
	if processing.Status != "processing" || processing.FinishedAt != nil {
		t.Fatalf("expected non-terminal state without finished_at, got %+v", processing)
	}
	if processing.LatestEvent["type"] != "recovery.failed" {
		t.Fatalf("expected recovery.failed event, got %+v", processing.LatestEvent)
	}
}

// TestEngineDefaultsUseWorkspaceRelativePaths verifies defaults resolve from the
// canonical runtime root instead of repo-relative placeholders.
func TestEngineDefaultsUseWorkspaceRelativePaths(t *testing.T) {
	runtimeRoot := filepath.Join(t.TempDir(), "runtime-root")
	t.Setenv("CIALLOCLAW_RUNTIME_ROOT", runtimeRoot)
	engine := NewEngine()

	settings := engine.Settings()
	general := settings["general"].(map[string]any)
	download := general["download"].(map[string]any)
	expectedWorkspaceRoot := filepath.ToSlash(filepath.Join(runtimeRoot, "workspace"))
	if download["workspace_path"] != expectedWorkspaceRoot {
		t.Fatalf("expected workspace_path default to be %q, got %v", expectedWorkspaceRoot, download["workspace_path"])
	}

	inspector := engine.InspectorConfig()
	taskSources := inspector["task_sources"].([]string)
	expectedTaskSource := filepath.ToSlash(filepath.Join(runtimeRoot, "workspace", "todos"))
	if len(taskSources) != 1 || taskSources[0] != expectedTaskSource {
		t.Fatalf("expected task_sources to default to %q, got %v", expectedTaskSource, taskSources)
	}
}

func TestEngineWithSettingsStoreMigratesLegacyWorkspaceDefaults(t *testing.T) {
	runtimeRoot := filepath.Join(t.TempDir(), "runtime-root")
	t.Setenv("CIALLOCLAW_RUNTIME_ROOT", runtimeRoot)
	engine := NewEngine()
	if err := engine.WithSettingsStore(staticSettingsStore{snapshot: map[string]any{
		"general": map[string]any{
			"download": map[string]any{"workspace_path": "workspace"},
		},
		"task_automation": map[string]any{
			"task_sources": []string{"workspace/todos", "workspace/review"},
		},
	}}); err != nil {
		t.Fatalf("WithSettingsStore returned error: %v", err)
	}
	settings := engine.Settings()
	general := settings["general"].(map[string]any)
	download := general["download"].(map[string]any)
	expectedWorkspaceRoot := filepath.ToSlash(filepath.Join(runtimeRoot, "workspace"))
	if download["workspace_path"] != expectedWorkspaceRoot {
		t.Fatalf("expected legacy workspace_path to migrate to %q, got %v", expectedWorkspaceRoot, download["workspace_path"])
	}
	taskAutomation := settings["task_automation"].(map[string]any)
	if !reflect.DeepEqual(taskAutomation["task_sources"], []string{
		filepath.ToSlash(filepath.Join(runtimeRoot, "workspace", "todos")),
		filepath.ToSlash(filepath.Join(runtimeRoot, "workspace", "review")),
	}) {
		t.Fatalf("expected legacy task_sources to migrate under runtime workspace, got %+v", taskAutomation)
	}
	if got := serviceconfig.DefaultWorkspaceRoot(); got != filepath.Join(runtimeRoot, "workspace") {
		t.Fatalf("expected config default workspace root to honor runtime override, got %q", got)
	}
}

func TestEngineWithSettingsStoreRejectsWorkspaceEscapesOutsideRuntimeRoot(t *testing.T) {
	runtimeRoot := filepath.Join(t.TempDir(), "runtime-root")
	t.Setenv("CIALLOCLAW_RUNTIME_ROOT", runtimeRoot)
	engine := NewEngine()
	if err := engine.WithSettingsStore(staticSettingsStore{snapshot: map[string]any{
		"general": map[string]any{
			"download": map[string]any{"workspace_path": "../outside"},
		},
		"task_automation": map[string]any{
			"task_sources": []string{"../outside", "workspace/todos"},
		},
	}}); err != nil {
		t.Fatalf("WithSettingsStore returned error: %v", err)
	}
	settings := engine.Settings()
	general := settings["general"].(map[string]any)
	download := general["download"].(map[string]any)
	expectedWorkspaceRoot := filepath.ToSlash(filepath.Join(runtimeRoot, "workspace"))
	if download["workspace_path"] != expectedWorkspaceRoot {
		t.Fatalf("expected unsafe workspace_path to reset to %q, got %v", expectedWorkspaceRoot, download["workspace_path"])
	}
	taskAutomation := settings["task_automation"].(map[string]any)
	if !reflect.DeepEqual(taskAutomation["task_sources"], []string{filepath.ToSlash(filepath.Join(runtimeRoot, "workspace", "todos"))}) {
		t.Fatalf("expected unsafe task_sources to be dropped during migration, got %+v", taskAutomation)
	}
}

func TestEngineWithSettingsStorePreservesWindowsAbsolutePathsAcrossHosts(t *testing.T) {
	runtimeRoot := filepath.Join(t.TempDir(), "runtime-root")
	t.Setenv("CIALLOCLAW_RUNTIME_ROOT", runtimeRoot)
	engine := NewEngine()
	if err := engine.WithSettingsStore(staticSettingsStore{snapshot: map[string]any{
		"general": map[string]any{
			"download": map[string]any{"workspace_path": "D:/legacy-workspace"},
		},
		"task_automation": map[string]any{
			"task_sources": []string{"D:/legacy-workspace/todos"},
		},
	}}); err != nil {
		t.Fatalf("WithSettingsStore returned error: %v", err)
	}
	settings := engine.Settings()
	general := settings["general"].(map[string]any)
	download := general["download"].(map[string]any)
	if download["workspace_path"] != "D:/legacy-workspace" {
		t.Fatalf("expected windows-style absolute workspace path to stay absolute, got %v", download["workspace_path"])
	}
	taskAutomation := settings["task_automation"].(map[string]any)
	if !reflect.DeepEqual(taskAutomation["task_sources"], []string{"D:/legacy-workspace/todos"}) {
		t.Fatalf("expected windows-style absolute task source to stay absolute, got %+v", taskAutomation)
	}
	if migratedWorkspace, _ := migrateWorkspaceRootSetting("D:/legacy-workspace"); migratedWorkspace != "D:/legacy-workspace" {
		t.Fatalf("expected windows-style absolute workspace path to stay absolute during migration, got %q", migratedWorkspace)
	}
	if migratedSource, _ := migrateTaskSourceSetting("D:/legacy-workspace/todos"); migratedSource != "D:/legacy-workspace/todos" {
		t.Fatalf("expected windows-style absolute task source to stay absolute during migration, got %q", migratedSource)
	}
	if isSafeRuntimeRelativePath("D:/legacy-workspace") {
		t.Fatal("expected windows-style absolute path to be rejected as runtime-relative")
	}
	if isSafeRuntimeRelativePath("D:relative") {
		t.Fatal("expected drive-prefixed relative path to be rejected as runtime-relative")
	}
	if migratedWorkspace, changed := migrateWorkspaceRootSetting("D:relative"); !changed || migratedWorkspace != defaultSettingsWorkspaceRoot() {
		t.Fatalf("expected drive-prefixed relative workspace path to reset to default runtime workspace, migrated=%q changed=%v", migratedWorkspace, changed)
	}
	if migrated, changed := migrateTaskSourceSetting("D:relative"); !changed || migrated != "" {
		t.Fatalf("expected drive-prefixed relative source to be dropped during migration, migrated=%q changed=%v", migrated, changed)
	}
}

func TestEngineNotepadItemsNormalizeAndSortRuntimeState(t *testing.T) {
	engine := NewEngine()
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }

	engine.ReplaceNotepadItems([]map[string]any{
		{
			"item_id":          "todo_later",
			"title":            "later item",
			"bucket":           "later",
			"status":           "normal",
			"type":             "todo_item",
			"due_at":           now.Add(48 * time.Hour).Format(time.RFC3339),
			"agent_suggestion": "review later",
		},
		{
			"item_id":          "todo_overdue",
			"title":            "overdue item",
			"bucket":           "upcoming",
			"status":           "normal",
			"type":             "todo_item",
			"due_at":           now.Add(-2 * time.Hour).Format(time.RFC3339),
			"agent_suggestion": "finish now",
		},
		{
			"item_id":          "todo_today",
			"title":            "today item",
			"bucket":           "upcoming",
			"status":           "normal",
			"type":             "todo_item",
			"due_at":           now.Add(3 * time.Hour).Format(time.RFC3339),
			"agent_suggestion": "translate",
		},
	})

	items, total := engine.NotepadItems("", 10, 0)
	if total != 3 || len(items) != 3 {
		t.Fatalf("expected three runtime notepad items, total=%d len=%d", total, len(items))
	}
	if items[0]["item_id"] != "todo_overdue" || items[0]["status"] != "overdue" {
		t.Fatalf("expected overdue item to sort first by due time, got %+v", items[0])
	}
	if items[1]["item_id"] != "todo_today" || items[1]["status"] != "due_today" {
		t.Fatalf("expected due_today item to remain normalized, got %+v", items[1])
	}

	upcomingItems, total := engine.NotepadItems("upcoming", 10, 0)
	if total != 2 || len(upcomingItems) != 2 {
		t.Fatalf("expected two upcoming items, total=%d len=%d", total, len(upcomingItems))
	}
}

func TestEngineCompleteNotepadItemMovesItemToClosedBucket(t *testing.T) {
	engine := NewEngine()
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }
	engine.ReplaceNotepadItems([]map[string]any{
		{
			"item_id":          "todo_convert",
			"title":            "convert item",
			"bucket":           "upcoming",
			"status":           "normal",
			"type":             "todo_item",
			"due_at":           now.Add(2 * time.Hour).Format(time.RFC3339),
			"agent_suggestion": "summarize",
		},
	})

	completed, ok := engine.CompleteNotepadItem("todo_convert")
	if !ok {
		t.Fatal("expected notepad item completion to succeed")
	}
	if completed["bucket"] != "closed" || completed["status"] != "completed" {
		t.Fatalf("expected completed notepad item to move to closed bucket, got %+v", completed)
	}
	if completed["due_at"] != nil {
		t.Fatalf("expected completed notepad item to clear due_at, got %+v", completed["due_at"])
	}

	closedItems, total := engine.NotepadItems("closed", 10, 0)
	if total != 1 || len(closedItems) != 1 {
		t.Fatalf("expected one closed item after completion, total=%d len=%d", total, len(closedItems))
	}
	if closedItems[0]["item_id"] != "todo_convert" {
		t.Fatalf("expected closed list to contain completed item, got %+v", closedItems[0])
	}
	if closedItems[0]["ended_at"] == nil {
		t.Fatalf("expected completed item to carry ended_at, got %+v", closedItems[0])
	}
}

func TestEngineLinkNotepadItemTaskPersistsReference(t *testing.T) {
	engine := NewEngine()
	engine.ReplaceNotepadItems([]map[string]any{{
		"item_id": "todo_link",
		"title":   "link me",
		"bucket":  "upcoming",
		"status":  "normal",
		"type":    "todo_item",
	}})

	if _, handled, err := engine.ClaimNotepadItemTask("todo_link"); err != nil || !handled {
		t.Fatalf("expected claim before link to succeed, handled=%v err=%v", handled, err)
	}

	linked, ok := engine.LinkNotepadItemTask("todo_link", "task_123")
	if !ok {
		t.Fatal("expected LinkNotepadItemTask to succeed")
	}
	if linked["linked_task_id"] != "task_123" {
		t.Fatalf("expected linked_task_id on returned item, got %+v", linked)
	}

	items, total := engine.NotepadItems("upcoming", 10, 0)
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected one linked item, total=%d len=%d", total, len(items))
	}
	if items[0]["linked_task_id"] != "task_123" {
		t.Fatalf("expected linked_task_id to persist in runtime list, got %+v", items[0])
	}
}

func TestEngineClaimNotepadItemTaskRejectsSecondClaim(t *testing.T) {
	engine := NewEngine()
	engine.ReplaceNotepadItems([]map[string]any{{
		"item_id": "todo_claim",
		"title":   "claim me",
		"bucket":  "upcoming",
		"status":  "normal",
		"type":    "todo_item",
	}})

	claimed, handled, err := engine.ClaimNotepadItemTask("todo_claim")
	if err != nil || !handled {
		t.Fatalf("expected first claim to succeed, handled=%v err=%v", handled, err)
	}
	if _, exists := claimed["linked_task_id"]; exists {
		t.Fatalf("expected claim marker to stay internal, got %+v", claimed)
	}

	_, handled, err = engine.ClaimNotepadItemTask("todo_claim")
	if !handled {
		t.Fatal("expected second claim to hit existing item")
	}
	if err == nil || err.Error() != "notepad item is already being converted: todo_claim" {
		t.Fatalf("expected in-flight conversion error, got %v", err)
	}
}

func TestEngineMarkNotepadClosedTracksLatestRestoreState(t *testing.T) {
	engine := NewEngine()
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }
	engine.ReplaceNotepadItems([]map[string]any{{
		"item_id":         "todo_reclose",
		"title":           "reclose me",
		"bucket":          "closed",
		"status":          "completed",
		"type":            "todo_item",
		"previous_bucket": "later",
		"previous_due_at": now.Add(24 * time.Hour).Format(time.RFC3339),
		"previous_status": "normal",
		"ended_at":        now.Add(-2 * time.Hour).Format(time.RFC3339),
	}})

	restored, _, _, handled, err := engine.UpdateNotepadItem("todo_reclose", "restore")
	if err != nil || !handled {
		t.Fatalf("expected initial restore to succeed, handled=%v err=%v", handled, err)
	}
	if restored["bucket"] != "later" {
		t.Fatalf("expected restore to return item to later bucket, got %+v", restored)
	}

	moved, _, _, handled, err := engine.UpdateNotepadItem("todo_reclose", "move_upcoming")
	if err != nil || !handled {
		t.Fatalf("expected move_upcoming after restore to succeed, handled=%v err=%v", handled, err)
	}
	if moved["bucket"] != "upcoming" {
		t.Fatalf("expected move_upcoming to switch to upcoming, got %+v", moved)
	}

	reclosed, _, _, handled, err := engine.UpdateNotepadItem("todo_reclose", "complete")
	if err != nil || !handled {
		t.Fatalf("expected second close to succeed, handled=%v err=%v", handled, err)
	}
	if reclosed["bucket"] != "closed" {
		t.Fatalf("expected reclosed item to be closed, got %+v", reclosed)
	}

	restoredAgain, _, _, handled, err := engine.UpdateNotepadItem("todo_reclose", "restore")
	if err != nil || !handled {
		t.Fatalf("expected second restore to succeed, handled=%v err=%v", handled, err)
	}
	if restoredAgain["bucket"] != "upcoming" {
		t.Fatalf("expected restore to use latest pre-close bucket, got %+v", restoredAgain)
	}
}

func TestEngineUpdateNotepadItemMovesLaterItemToUpcoming(t *testing.T) {
	engine := NewEngine()
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }
	engine.ReplaceNotepadItems([]map[string]any{{
		"item_id": "todo_move",
		"title":   "move me",
		"bucket":  "later",
		"status":  "normal",
		"type":    "todo_item",
		"due_at":  now.Add(48 * time.Hour).Format(time.RFC3339),
	}})

	updated, refreshGroups, deletedItemID, handled, err := engine.UpdateNotepadItem("todo_move", "move_upcoming")
	if err != nil || !handled {
		t.Fatalf("expected move_upcoming to succeed, handled=%v err=%v", handled, err)
	}
	if deletedItemID != "" {
		t.Fatalf("expected no deleted item id, got %q", deletedItemID)
	}
	if updated["bucket"] != "upcoming" {
		t.Fatalf("expected moved item to be in upcoming bucket, got %+v", updated)
	}
	if len(refreshGroups) != 2 || refreshGroups[0] != "later" || refreshGroups[1] != "upcoming" {
		t.Fatalf("expected refresh groups for source and target buckets, got %+v", refreshGroups)
	}
}

func TestEngineUpdateNotepadItemTogglesRecurringRule(t *testing.T) {
	engine := NewEngine()
	engine.ReplaceNotepadItems([]map[string]any{{
		"item_id":           "todo_recurring",
		"title":             "recurring note",
		"bucket":            "recurring_rule",
		"status":            "normal",
		"type":              "recurring",
		"recurring_enabled": true,
	}})

	updated, _, _, handled, err := engine.UpdateNotepadItem("todo_recurring", "toggle_recurring")
	if err != nil || !handled {
		t.Fatalf("expected toggle_recurring to succeed, handled=%v err=%v", handled, err)
	}
	if updated["recurring_enabled"] != false || updated["status"] != "cancelled" {
		t.Fatalf("expected recurring rule to pause, got %+v", updated)
	}

	updated, _, _, handled, err = engine.UpdateNotepadItem("todo_recurring", "toggle_recurring")
	if err != nil || !handled {
		t.Fatalf("expected second toggle_recurring to succeed, handled=%v err=%v", handled, err)
	}
	if updated["recurring_enabled"] != true || updated["status"] != "normal" {
		t.Fatalf("expected recurring rule to resume, got %+v", updated)
	}
}

func TestEngineUpdateNotepadItemRestoresClosedItem(t *testing.T) {
	engine := NewEngine()
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return now }
	engine.ReplaceNotepadItems([]map[string]any{{
		"item_id":         "todo_restore",
		"title":           "restore me",
		"bucket":          "closed",
		"status":          "cancelled",
		"type":            "todo_item",
		"previous_bucket": "later",
		"previous_due_at": now.Add(24 * time.Hour).Format(time.RFC3339),
		"previous_status": "normal",
		"ended_at":        now.Format(time.RFC3339),
	}})

	updated, refreshGroups, deletedItemID, handled, err := engine.UpdateNotepadItem("todo_restore", "restore")
	if err != nil || !handled {
		t.Fatalf("expected restore to succeed, handled=%v err=%v", handled, err)
	}
	if deletedItemID != "" {
		t.Fatalf("expected restore not to delete item, got %q", deletedItemID)
	}
	if updated["bucket"] != "later" || updated["ended_at"] != nil {
		t.Fatalf("expected restored item to return to previous bucket, got %+v", updated)
	}
	if len(refreshGroups) != 2 || refreshGroups[0] != "closed" || refreshGroups[1] != "later" {
		t.Fatalf("expected restore refresh groups, got %+v", refreshGroups)
	}
}

func TestEngineUpdateNotepadItemDeletesClosedItem(t *testing.T) {
	engine := NewEngine()
	engine.ReplaceNotepadItems([]map[string]any{{
		"item_id": "todo_delete",
		"title":   "delete me",
		"bucket":  "closed",
		"status":  "completed",
		"type":    "todo_item",
	}})

	updated, refreshGroups, deletedItemID, handled, err := engine.UpdateNotepadItem("todo_delete", "delete")
	if err != nil || !handled {
		t.Fatalf("expected delete to succeed, handled=%v err=%v", handled, err)
	}
	if updated != nil {
		t.Fatalf("expected deleted item payload to be nil, got %+v", updated)
	}
	if deletedItemID != "todo_delete" {
		t.Fatalf("expected deleted item id, got %q", deletedItemID)
	}
	if len(refreshGroups) != 1 || refreshGroups[0] != "closed" {
		t.Fatalf("expected closed refresh group, got %+v", refreshGroups)
	}
	items, total := engine.NotepadItems("closed", 10, 0)
	if total != 0 || len(items) != 0 {
		t.Fatalf("expected deleted item to disappear from closed bucket, total=%d len=%d", total, len(items))
	}
}

func TestEngineListTasksSupportsSorting(t *testing.T) {
	engine := NewEngine()
	currentTime := time.Date(2026, 4, 8, 9, 0, 0, 0, time.UTC)
	engine.now = func() time.Time { return currentTime }

	createTask := func(title string) TaskRecord {
		task := engine.CreateTask(CreateTaskInput{
			SessionID:   "sess_sort",
			Title:       title,
			SourceType:  "hover_input",
			Status:      "processing",
			Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{"style": "key_points"}},
			CurrentStep: "return_result",
			RiskLevel:   "green",
			Timeline: []TaskStepRecord{{
				Name:          "return_result",
				Status:        "running",
				OrderIndex:    1,
				InputSummary:  "task input",
				OutputSummary: "task output",
			}},
		})
		currentTime = currentTime.Add(time.Minute)
		return task
	}

	first := createTask("first")
	second := createTask("second")
	third := createTask("third")

	if _, err := engine.ControlTask(first.TaskID, "pause", map[string]any{"task_id": first.TaskID, "type": "status"}); err != nil {
		t.Fatalf("expected first task update to succeed: %v", err)
	}
	currentTime = currentTime.Add(time.Minute)
	if _, ok := engine.CompleteTask(second.TaskID, map[string]any{"type": "bubble"}, map[string]any{"task_id": second.TaskID, "type": "result"}, nil); !ok {
		t.Fatal("expected second task completion to succeed")
	}
	currentTime = currentTime.Add(time.Minute)
	if _, ok := engine.CompleteTask(third.TaskID, map[string]any{"type": "bubble"}, map[string]any{"task_id": third.TaskID, "type": "result"}, nil); !ok {
		t.Fatal("expected third task completion to succeed")
	}

	updatedAsc, _ := engine.ListTasks("unfinished", "updated_at", "asc", 10, 0)
	if len(updatedAsc) != 1 || updatedAsc[0].TaskID != first.TaskID {
		t.Fatalf("expected unfinished list to keep first task after update sort, got %+v", updatedAsc)
	}

	startedAsc, _ := engine.ListTasks("finished", "started_at", "asc", 10, 0)
	if len(startedAsc) != 2 {
		t.Fatalf("expected two finished tasks, got %d", len(startedAsc))
	}
	if startedAsc[0].TaskID != second.TaskID || startedAsc[1].TaskID != third.TaskID {
		t.Fatalf("expected started_at asc order second -> third, got %s -> %s", startedAsc[0].TaskID, startedAsc[1].TaskID)
	}

	finishedDesc, _ := engine.ListTasks("finished", "finished_at", "desc", 10, 0)
	if finishedDesc[0].TaskID != third.TaskID || finishedDesc[1].TaskID != second.TaskID {
		t.Fatalf("expected finished_at desc order third -> second, got %s -> %s", finishedDesc[0].TaskID, finishedDesc[1].TaskID)
	}

	defaultSorted, _ := engine.ListTasks("finished", "unknown_field", "unknown_order", 10, 0)
	if defaultSorted[0].TaskID != third.TaskID || defaultSorted[1].TaskID != second.TaskID {
		t.Fatalf("expected invalid sort options to fall back to updated_at desc, got %s -> %s", defaultSorted[0].TaskID, defaultSorted[1].TaskID)
	}
}

func TestEngineControlTaskRejectsInvalidTransitions(t *testing.T) {
	engine := NewEngine()
	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_control",
		Title:       "task control transition test",
		SourceType:  "hover_input",
		Status:      "confirming_intent",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{"style": "key_points"}},
		CurrentStep: "intent_confirmation",
		RiskLevel:   "green",
		Timeline: []TaskStepRecord{{
			Name:          "intent_confirmation",
			Status:        "pending",
			OrderIndex:    1,
			InputSummary:  "task input",
			OutputSummary: "waiting for confirm",
		}},
	})

	if _, err := engine.ControlTask(task.TaskID, "resume", map[string]any{"task_id": task.TaskID, "type": "status"}); !errors.Is(err, ErrTaskStatusInvalid) {
		t.Fatalf("expected resume from confirming_intent to be invalid, got %v", err)
	}

	if _, err := engine.ControlTask(task.TaskID, "pause", map[string]any{"task_id": task.TaskID, "type": "status"}); !errors.Is(err, ErrTaskStatusInvalid) {
		t.Fatalf("expected pause from confirming_intent to be invalid, got %v", err)
	}

	completed, ok := engine.CompleteTask(task.TaskID, map[string]any{"type": "bubble"}, map[string]any{"task_id": task.TaskID, "type": "result"}, nil)
	if !ok || completed.Status != "completed" {
		t.Fatalf("expected task to complete for finished-state checks, got %#v ok=%v", completed, ok)
	}

	if _, err := engine.ControlTask(task.TaskID, "cancel", map[string]any{"task_id": task.TaskID, "type": "status"}); !errors.Is(err, ErrTaskAlreadyFinished) {
		t.Fatalf("expected cancel on completed task to return ErrTaskAlreadyFinished, got %v", err)
	}
}

func TestEngineControlTaskRestartResetsFinishedOutputs(t *testing.T) {
	engine := NewEngine()
	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_restart",
		Title:       "restart finished task",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{"style": "key_points"}},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
		Timeline: []TaskStepRecord{{
			Name:          "generate_output",
			Status:        "running",
			OrderIndex:    1,
			InputSummary:  "task input",
			OutputSummary: "generating output",
		}},
	})

	deliveryResult := map[string]any{"type": "workspace_document", "payload": map[string]any{"path": "workspace/result.md"}}
	artifacts := []map[string]any{{"artifact_id": "art_test", "task_id": task.TaskID, "path": "workspace/result.md"}}
	completed, ok := engine.CompleteTask(task.TaskID, deliveryResult, map[string]any{"task_id": task.TaskID, "type": "result"}, artifacts)
	if !ok || completed.Status != "completed" {
		t.Fatalf("expected task to complete before restart, got %#v ok=%v", completed, ok)
	}
	if _, ok := engine.SetMemoryPlans(task.TaskID, []map[string]any{{"kind": "retrieval"}}, []map[string]any{{"kind": "summary_write"}}); !ok {
		t.Fatal("expected memory plans to be stored before restart")
	}
	if _, ok := engine.SetMirrorReferences(task.TaskID, []map[string]any{{"memory_id": "mem_write_task_001_1"}}); !ok {
		t.Fatal("expected mirror references to be stored before restart")
	}
	originalRunID := task.RunID

	restarted, err := engine.ControlTask(task.TaskID, "restart", map[string]any{"task_id": task.TaskID, "type": "status"})
	if err != nil {
		t.Fatalf("expected restart on completed task to succeed: %v", err)
	}
	if restarted.Status != "processing" {
		t.Fatalf("expected restarted task to return to processing, got %s", restarted.Status)
	}
	if restarted.FinishedAt != nil {
		t.Fatal("expected restart to clear finished_at")
	}
	if restarted.RunID == originalRunID {
		t.Fatalf("expected restart to allocate a new run_id, got %s", restarted.RunID)
	}
	if restarted.DeliveryResult != nil || len(restarted.Artifacts) != 0 {
		t.Fatal("expected restart to clear finished delivery outputs")
	}
	if restarted.MemoryReadPlans != nil || restarted.MemoryWritePlans != nil || restarted.MirrorReferences != nil {
		t.Fatal("expected restart to clear handoff and mirror snapshots")
	}
	if restarted.LoopStopReason != "" {
		t.Fatalf("expected restart to clear loop stop reason, got %q", restarted.LoopStopReason)
	}
	if restarted.ExecutionAttempt != 2 {
		t.Fatalf("expected first restart to increment attempt to 2, got %d", restarted.ExecutionAttempt)
	}

	completedAgain, ok := engine.CompleteTask(task.TaskID, deliveryResult, map[string]any{"task_id": task.TaskID, "type": "result"}, artifacts)
	if !ok || completedAgain.Status != "completed" {
		t.Fatalf("expected task to complete again before second restart, got %#v ok=%v", completedAgain, ok)
	}
	restartedAgain, err := engine.ControlTask(task.TaskID, "restart", map[string]any{"task_id": task.TaskID, "type": "status"})
	if err != nil {
		t.Fatalf("expected second restart to succeed: %v", err)
	}
	if restartedAgain.ExecutionAttempt != 3 {
		t.Fatalf("expected second restart to increment attempt to 3, got %d", restartedAgain.ExecutionAttempt)
	}
}

func TestEnginePrepareRestartLeavesLiveTaskStableUntilCommit(t *testing.T) {
	engine := NewEngine()
	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_prepare_restart",
		Title:       "prepare restart task",
		SourceType:  "hover_input",
		Status:      "completed",
		Intent:      map[string]any{"name": "agent_loop", "arguments": map[string]any{}},
		CurrentStep: "return_result",
		RiskLevel:   "green",
	})
	if _, ok := engine.SetPresentation(task.TaskID, nil, map[string]any{
		"type":         "bubble",
		"title":        "stable result",
		"preview_text": "stable preview",
		"payload":      map[string]any{"task_id": task.TaskID},
	}, []map[string]any{{
		"artifact_id": "art_prepare_restart",
		"task_id":     task.TaskID,
		"path":        "workspace/stable.md",
	}}); !ok {
		t.Fatal("expected stable presentation before restart")
	}
	if _, ok := engine.SetCitations(task.TaskID, []map[string]any{{
		"citation_id": "cit_prepare_restart",
		"task_id":     task.TaskID,
		"label":       "stable citation",
	}}); !ok {
		t.Fatal("expected stable citations before restart")
	}
	if _, ok := engine.AppendAuditData(task.TaskID, []map[string]any{{
		"audit_id": "audit_prepare_restart",
		"task_id":  task.TaskID,
		"summary":  "stable audit",
	}}, map[string]any{
		"total_tokens":   36,
		"estimated_cost": 0.12,
	}); !ok {
		t.Fatal("expected stable audit before restart")
	}

	previous, prepared, err := engine.PrepareRestart(task.TaskID, map[string]any{"task_id": task.TaskID, "type": "status"})
	if err != nil {
		t.Fatalf("prepare restart failed: %v", err)
	}
	if prepared.RunID == previous.RunID {
		t.Fatalf("expected prepared restart to allocate a fresh run_id, got %s", prepared.RunID)
	}
	if prepared.ExecutionAttempt != previous.ExecutionAttempt+1 {
		t.Fatalf("expected prepared restart to increment attempt, got before=%d after=%d", previous.ExecutionAttempt, prepared.ExecutionAttempt)
	}
	if prepared.DeliveryResult != nil || len(prepared.Artifacts) != 0 || len(prepared.Citations) != 0 || len(prepared.AuditRecords) != 0 {
		t.Fatalf("expected prepared restart copy to clear formal outputs, got %+v", prepared)
	}
	if len(prepared.TokenUsage) != 0 {
		t.Fatalf("expected prepared restart copy to clear token usage, got %+v", prepared.TokenUsage)
	}
	if len(prepared.Notifications) != 0 {
		t.Fatalf("expected prepared restart copy to start with a clean notification queue, got %+v", prepared.Notifications)
	}

	liveTask, ok := engine.GetTask(task.TaskID)
	if !ok {
		t.Fatal("expected live task to remain readable")
	}
	if liveTask.RunID != previous.RunID || liveTask.ExecutionAttempt != previous.ExecutionAttempt {
		t.Fatalf("expected live task identity to stay on previous attempt, got %+v", liveTask)
	}
	if liveTask.DeliveryResult == nil || len(liveTask.Artifacts) != 1 || len(liveTask.Citations) != 1 || len(liveTask.AuditRecords) != 1 {
		t.Fatalf("expected live task to preserve finished outputs before commit, got %+v", liveTask)
	}

	processingTask, changed := engine.BeginPreparedExecution(prepared, "agent_loop", "restart committed")
	if !changed {
		t.Fatal("expected prepared restart execution to commit")
	}
	if processingTask.RunID != prepared.RunID || processingTask.ExecutionAttempt != prepared.ExecutionAttempt {
		t.Fatalf("expected committed restart to preserve prepared identity, got %+v", processingTask)
	}
}

func TestEngineRestartPersistsExecutionAttemptAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-run-attempts.db")
	store, err := storage.NewSQLiteTaskRunStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteTaskRunStore returned error: %v", err)
	}
	engine, err := NewEngineWithStore(store)
	if err != nil {
		t.Fatalf("NewEngineWithStore returned error: %v", err)
	}
	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_restart_attempts",
		Title:       "restart attempts",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize"},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
		Timeline:    []TaskStepRecord{{Name: "generate_output", Status: "running", OrderIndex: 1}},
	})
	completed, ok := engine.CompleteTask(task.TaskID, map[string]any{"type": "bubble"}, map[string]any{"task_id": task.TaskID, "type": "result"}, nil)
	if !ok || completed.Status != "completed" {
		t.Fatalf("expected initial completion before restart, got %#v ok=%v", completed, ok)
	}
	restarted, err := engine.ControlTask(task.TaskID, "restart", map[string]any{"task_id": task.TaskID, "type": "status"})
	if err != nil {
		t.Fatalf("first restart failed: %v", err)
	}
	completedAgain, ok := engine.CompleteTask(restarted.TaskID, map[string]any{"type": "bubble"}, map[string]any{"task_id": restarted.TaskID, "type": "result"}, nil)
	if !ok || completedAgain.Status != "completed" {
		t.Fatalf("expected second completion before restart, got %#v ok=%v", completedAgain, ok)
	}
	restartedAgain, err := engine.ControlTask(completedAgain.TaskID, "restart", map[string]any{"task_id": completedAgain.TaskID, "type": "status"})
	if err != nil {
		t.Fatalf("second restart failed: %v", err)
	}
	if restartedAgain.ExecutionAttempt != 3 {
		t.Fatalf("expected in-memory execution attempt to reach 3, got %d", restartedAgain.ExecutionAttempt)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close sqlite store before reload: %v", err)
	}
	reloadStore, err := storage.NewSQLiteTaskRunStore(path)
	if err != nil {
		t.Fatalf("reopen sqlite task run store: %v", err)
	}
	defer func() { _ = reloadStore.Close() }()
	records, err := reloadStore.LoadTaskRuns(context.Background())
	if err != nil {
		t.Fatalf("load task runs before engine reload: %v", err)
	}
	if len(records) != 1 || records[0].ExecutionAttempt != 3 {
		t.Fatalf("expected persisted store attempt to be 3 before engine reload, got %+v", records)
	}

	reloaded, err := NewEngineWithStore(reloadStore)
	if err != nil {
		t.Fatalf("reloading engine failed: %v", err)
	}
	persisted, ok := reloaded.GetTask(task.TaskID)
	if !ok {
		t.Fatal("expected task to reload from store")
	}
	if persisted.ExecutionAttempt != 3 {
		t.Fatalf("expected persisted execution attempt to stay 3 after reload, got %d", persisted.ExecutionAttempt)
	}
}

func TestEngineWithStorePersistsTaskLifecycleAcrossReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "task-run-engine.db")
	store, err := storage.NewSQLiteTaskRunStore(path)
	if err != nil {
		t.Fatalf("NewSQLiteTaskRunStore returned error: %v", err)
	}

	engine, err := NewEngineWithStore(store)
	if err != nil {
		t.Fatalf("NewEngineWithStore returned error: %v", err)
	}
	engine.now = func() time.Time { return time.Date(2026, 4, 10, 8, 0, 0, 0, time.UTC) }

	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_persist",
		Title:       "persist me",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize"},
		CurrentStep: "generate_output",
		RiskLevel:   "yellow",
		Timeline: []TaskStepRecord{{
			Name:          "generate_output",
			Status:        "running",
			OrderIndex:    1,
			InputSummary:  "input",
			OutputSummary: "working",
		}},
	})

	if _, ok := engine.MarkWaitingApprovalWithPlan(
		task.TaskID,
		map[string]any{
			"approval_id": "appr_001",
			"task_id":     task.TaskID,
			"risk_level":  "yellow",
			"status":      "pending",
		},
		map[string]any{
			"task_id":       task.TaskID,
			"delivery_type": "workspace_document",
		},
		map[string]any{"task_id": task.TaskID, "type": "status", "text": "waiting auth"},
	); !ok {
		t.Fatal("expected task to enter waiting_auth")
	}
	if _, ok := engine.SetMemoryPlans(task.TaskID, []map[string]any{{"kind": "retrieval"}}, []map[string]any{{"kind": "summary_write"}}); !ok {
		t.Fatal("expected memory plans to persist")
	}
	if _, ok := engine.SetDeliveryPlans(task.TaskID, map[string]any{"target_path": "workspace/result.md"}, []map[string]any{{"artifact_id": "art_001"}}); !ok {
		t.Fatal("expected delivery plans to persist")
	}

	reloaded, err := NewEngineWithStore(store)
	if err != nil {
		t.Fatalf("NewEngineWithStore reload returned error: %v", err)
	}

	persisted, ok := reloaded.GetTask(task.TaskID)
	if !ok {
		t.Fatal("expected persisted task to reload from sqlite")
	}
	if persisted.RunID != task.RunID {
		t.Fatalf("expected run_id to round-trip, got %s want %s", persisted.RunID, task.RunID)
	}
	if persisted.Status != "waiting_auth" {
		t.Fatalf("expected waiting_auth to round-trip, got %s", persisted.Status)
	}
	if persisted.PendingExecution["delivery_type"] != "workspace_document" {
		t.Fatalf("expected pending execution to round-trip, got %+v", persisted.PendingExecution)
	}
	if len(persisted.MemoryReadPlans) != 1 || len(persisted.ArtifactPlans) != 1 {
		t.Fatalf("expected persisted plans to reload, got %+v", persisted)
	}

	nextTask := reloaded.CreateTask(CreateTaskInput{
		SessionID:   "sess_persist_2",
		Title:       "new task after reload",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "rewrite"},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
	})
	if nextTask.TaskID == task.TaskID {
		t.Fatalf("expected identifier allocation to continue after reload, got duplicate %s", nextTask.TaskID)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
}
