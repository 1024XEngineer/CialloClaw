// 该测试文件验证运行时状态机与通知队列行为。
package runengine

import (
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

	if _, ok := engine.ControlTask(first.TaskID, "pause", map[string]any{"task_id": first.TaskID, "type": "status"}); !ok {
		t.Fatal("expected first task update to succeed")
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
