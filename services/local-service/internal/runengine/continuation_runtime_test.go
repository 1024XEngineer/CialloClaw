package runengine

import (
	"strings"
	"testing"

	taskcontext "github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
)

func TestEngineContinueTaskMergesContinuationStateAndDrainsSteeringMessages(t *testing.T) {
	engine := NewEngine()
	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_continue",
		Title:       "Original title",
		SourceType:  "hover_input",
		Status:      "processing",
		CurrentStep: "collect_input",
		RiskLevel:   "green",
		Snapshot: taskcontext.TaskContextSnapshot{
			Source:      "floating_ball",
			Text:        "original text",
			BrowserKind: "chrome",
			ProcessPath: "C:/Program Files/Google/Chrome/Application/chrome.exe",
			ProcessID:   4242,
		},
	})

	updated, ok := engine.ContinueTask(task.TaskID, ContinuationUpdate{
		Title:       "Updated title",
		Status:      "processing",
		CurrentStep: "execute_task",
		Intent:      map[string]any{"name": "summarize"},
		BubbleMessage: map[string]any{
			"content": "continuation bubble",
		},
		Snapshot: taskcontext.TaskContextSnapshot{
			Text:          "updated text",
			SelectionText: "important paragraph",
			BrowserKind:   "edge",
			ProcessPath:   "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe",
			ProcessID:     5150,
		},
		SteeringMessage: "focus on highlights",
	})
	if !ok {
		t.Fatal("expected continue task to update active task")
	}
	if updated.Title != "Updated title" || updated.CurrentStep != "execute_task" {
		t.Fatalf("expected continuation metadata to update, got %+v", updated)
	}
	if updated.Snapshot.SelectionText != "important paragraph" || updated.Snapshot.Source != "floating_ball" {
		t.Fatalf("expected continuation snapshot to merge with prior source, got %+v", updated.Snapshot)
	}
	if updated.Snapshot.BrowserKind != "edge" || updated.Snapshot.ProcessPath != "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe" || updated.Snapshot.ProcessID != 5150 {
		t.Fatalf("expected continuation snapshot to keep latest attach hints, got %+v", updated.Snapshot)
	}
	if !strings.Contains(updated.Snapshot.Text, "original text") || !strings.Contains(updated.Snapshot.Text, "updated text") {
		t.Fatalf("expected continuation snapshot to merge with prior source, got %+v", updated.Snapshot)
	}
	if len(updated.SteeringMessages) != 1 || updated.SteeringMessages[0] != "focus on highlights" {
		t.Fatalf("expected continuation steering message to queue, got %+v", updated.SteeringMessages)
	}
	if updated.LatestEvent["type"] != "task.steered" {
		t.Fatalf("expected continuation event to record steering, got %+v", updated.LatestEvent)
	}

	messages, ok := engine.DrainSteeringMessages(task.TaskID)
	if !ok || len(messages) != 1 || messages[0] != "focus on highlights" {
		t.Fatalf("expected drain steering to return queued message, got ok=%v messages=%+v", ok, messages)
	}
	messages, ok = engine.DrainSteeringMessages(task.TaskID)
	if !ok || len(messages) != 0 {
		t.Fatalf("expected second drain steering call to clear queue, got ok=%v messages=%+v", ok, messages)
	}
}

func TestEngineFailTaskExecutionRecordsFailureState(t *testing.T) {
	engine := NewEngine()
	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_fail",
		Title:       "Failure case",
		SourceType:  "hover_input",
		Status:      "processing",
		CurrentStep: "execute_task",
		RiskLevel:   "yellow",
	})

	failed, ok := engine.FailTaskExecution(
		task.TaskID,
		"command_exec",
		"execution_error",
		"command failed",
		map[string]any{"scope": "workspace"},
		map[string]any{"content": "failure bubble"},
		map[string]any{"recovery_point_id": "rp_001"},
	)
	if !ok {
		t.Fatal("expected fail task execution to update existing task")
	}
	if failed.Status != "failed" || failed.CurrentStep != "command_exec" || failed.FinishedAt == nil {
		t.Fatalf("expected fail task execution to mark task failed, got %+v", failed)
	}
	if failed.BubbleMessage["content"] != "failure bubble" || failed.ImpactScope["scope"] != "workspace" {
		t.Fatalf("expected fail task execution to preserve failure payloads, got bubble=%+v impact=%+v", failed.BubbleMessage, failed.ImpactScope)
	}
	if failed.SecuritySummary["security_status"] != "execution_error" || failed.SecuritySummary["risk_level"] != "yellow" {
		t.Fatalf("expected fail task execution to record security summary, got %+v", failed.SecuritySummary)
	}
}

func TestEngineBlockTaskByPolicyRecordsInterceptedCancellation(t *testing.T) {
	engine := NewEngine()
	task := engine.CreateTask(CreateTaskInput{
		SessionID:   "sess_block",
		Title:       "Blocked case",
		SourceType:  "hover_input",
		Status:      "processing",
		CurrentStep: "risk_check",
		RiskLevel:   "yellow",
	})

	blocked, ok := engine.BlockTaskByPolicy(
		task.TaskID,
		"red",
		"policy blocked",
		map[string]any{"category": "filesystem"},
		map[string]any{"content": "blocked bubble"},
	)
	if !ok {
		t.Fatal("expected block task by policy to update existing task")
	}
	if blocked.Status != "cancelled" || blocked.CurrentStep != "risk_blocked" || blocked.FinishedAt == nil {
		t.Fatalf("expected block task by policy to cancel task, got %+v", blocked)
	}
	if blocked.RiskLevel != "red" {
		t.Fatalf("expected block task by policy to override risk level, got %+v", blocked)
	}
	if blocked.SecuritySummary["security_status"] != "intercepted" || blocked.BubbleMessage["content"] != "blocked bubble" {
		t.Fatalf("expected block task by policy to preserve intercepted summary, got security=%+v bubble=%+v", blocked.SecuritySummary, blocked.BubbleMessage)
	}
}
