// 该测试文件验证运行时状态机与通知队列行为。
package runengine

import (
	"errors"
	"testing"
	"time"
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

	confirmed, ok := engine.ConfirmTask(task.TaskID, map[string]any{"name": "rewrite", "arguments": map[string]any{"tone": "professional"}}, map[string]any{"task_id": task.TaskID, "type": "status", "text": "开始处理"})
	if !ok {
		t.Fatal("expected task confirmation to succeed")
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
}

// TestEngineAuthorizationAndHandoffState 验证EngineAuthorizationAndHandoffState。
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
