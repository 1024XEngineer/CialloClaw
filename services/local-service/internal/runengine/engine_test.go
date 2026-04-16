// 该测试文件验证运行时状态机与通知队列行为。
package runengine

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

// TestEngineTaskLifecycle 验证EngineTaskLifecycle。
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
}

// TestEngineExecutionProgressAndToolCall 验证真实执行阶段的 timeline 和 tool_call 会被记录。
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

// TestEngineAuthorizationAndHandoffState 验证EngineAuthorizationAndHandoffState。
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

// TestEngineDefaultsUseWorkspaceRelativePaths 验证默认配置不会写入平台盘符路径。
func TestEngineDefaultsUseWorkspaceRelativePaths(t *testing.T) {
	engine := NewEngine()

	settings := engine.Settings()
	general := settings["general"].(map[string]any)
	download := general["download"].(map[string]any)
	if download["workspace_path"] != "workspace" {
		t.Fatalf("expected workspace_path default to be workspace, got %v", download["workspace_path"])
	}

	inspector := engine.InspectorConfig()
	taskSources := inspector["task_sources"].([]string)
	if len(taskSources) != 1 || taskSources[0] != "workspace/todos" {
		t.Fatalf("expected task_sources to default to workspace/todos, got %v", taskSources)
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
	if claimed["linked_task_id"] != "__claim__:todo_claim" {
		t.Fatalf("expected claim marker on item, got %+v", claimed)
	}

	_, handled, err = engine.ClaimNotepadItemTask("todo_claim")
	if !handled {
		t.Fatal("expected second claim to hit existing item")
	}
	if err == nil || err.Error() != "notepad item is already being converted: todo_claim" {
		t.Fatalf("expected in-flight conversion error, got %v", err)
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
	if restarted.DeliveryResult != nil || len(restarted.Artifacts) != 0 {
		t.Fatal("expected restart to clear finished delivery outputs")
	}
	if restarted.MemoryReadPlans != nil || restarted.MemoryWritePlans != nil || restarted.MirrorReferences != nil {
		t.Fatal("expected restart to clear handoff and mirror snapshots")
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
