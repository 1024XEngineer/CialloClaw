package runengine

import (
	"testing"
	"time"
)

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

	deliveryResult := map[string]any{"type": "workspace_document", "title": "测试结果", "payload": map[string]any{"path": "D:/CialloClawWorkspace/result.md", "task_id": task.TaskID}}
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

	finishedTasks, total := engine.ListTasks("finished", 10, 0)
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

func TestEngineAuthorizationAndHandoffState(t *testing.T) {
	engine := NewEngine()
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
	bubble := map[string]any{"task_id": task.TaskID, "type": "status", "text": "等待授权"}
	waitingTask, ok := engine.MarkWaitingApproval(task.TaskID, approvalRequest, bubble)
	if !ok {
		t.Fatal("expected waiting approval transition to succeed")
	}
	if waitingTask.Status != "waiting_auth" {
		t.Fatalf("expected waiting_auth status, got %s", waitingTask.Status)
	}

	memoryReadPlans := []map[string]any{{"kind": "retrieval", "task_id": task.TaskID}}
	memoryWritePlans := []map[string]any{{"kind": "summary_write", "task_id": task.TaskID}}
	if _, ok := engine.SetMemoryPlans(task.TaskID, memoryReadPlans, memoryWritePlans); !ok {
		t.Fatal("expected memory handoff plans to be stored")
	}

	storagePlan := map[string]any{"task_id": task.TaskID, "target_path": "D:/CialloClawWorkspace/result.md"}
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
	if record.StorageWritePlan["target_path"] != "D:/CialloClawWorkspace/result.md" {
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
}
