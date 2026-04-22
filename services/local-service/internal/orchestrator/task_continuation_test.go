package orchestrator

import (
	"strings"
	"testing"
	"time"

	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

func TestResolveTaskContinuationContextUsesSingleActiveSession(t *testing.T) {
	service := newTestService()
	activeTask := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_active",
		Title:       "Analyze the current failure",
		SourceType:  "hover_input",
		Status:      "processing",
		CurrentStep: "agent_loop",
		RiskLevel:   "yellow",
	})

	continuationContext := service.resolveTaskContinuationContext("")
	if continuationContext.SessionID != activeTask.SessionID {
		t.Fatalf("expected active session %s, got %+v", activeTask.SessionID, continuationContext)
	}
	if continuationContext.SessionMode != "implicit_active" {
		t.Fatalf("expected implicit_active session mode, got %+v", continuationContext)
	}
	if len(continuationContext.Candidates) != 1 || continuationContext.Candidates[0].TaskID != activeTask.TaskID {
		t.Fatalf("expected active task to remain the only continuation candidate, got %+v", continuationContext.Candidates)
	}
}

func TestResolveTaskContinuationContextSkipsWaitingAuthorizationTasks(t *testing.T) {
	service := newTestService()
	service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_waiting_auth",
		Title:       "Write into a file after approval",
		SourceType:  "hover_input",
		Status:      "waiting_auth",
		CurrentStep: "waiting_authorization",
		RiskLevel:   "yellow",
	})

	continuationContext := service.resolveTaskContinuationContext("")
	if continuationContext.SessionID != "" || len(continuationContext.Candidates) != 0 {
		t.Fatalf("expected waiting_auth task to stay out of implicit continuation candidates, got %+v", continuationContext)
	}
}

func TestResolveTaskContinuationContextSkipsPausedTasks(t *testing.T) {
	service := newTestService()
	service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_paused",
		Title:       "Summarize the incident report",
		SourceType:  "hover_input",
		Status:      "paused",
		CurrentStep: "generate_output",
		RiskLevel:   "green",
	})

	continuationContext := service.resolveTaskContinuationContext("")
	if continuationContext.SessionID != "" || len(continuationContext.Candidates) != 0 {
		t.Fatalf("expected paused task to stay out of implicit continuation candidates, got %+v", continuationContext)
	}
}

func TestBuildTaskContinuationPromptRedactsSensitivePayloads(t *testing.T) {
	snapshot := contextsvc.TaskContextSnapshot{
		Trigger:       "hover_text_input",
		InputType:     "text",
		Text:          "Focus on the database timeout and keep the scope narrow.",
		SelectionText: "panic: dial tcp timeout",
		ErrorText:     "database timeout",
		Files:         []string{"logs/network.log"},
		PageTitle:     "Internal dashboard",
		PageURL:       "https://internal.example/tasks/1",
		ScreenSummary: "database panel shows critical errors",
	}
	candidate := runengine.TaskRecord{
		TaskID:      "task_001",
		SessionID:   "sess_secret",
		Title:       "Investigate the production timeout",
		Status:      "processing",
		CurrentStep: "agent_loop",
		SourceType:  "hover_input",
		UpdatedAt:   time.Now().Add(-30 * time.Second),
		Intent:      map[string]any{"name": "agent_loop"},
		Snapshot: contextsvc.TaskContextSnapshot{
			Files:     []string{"logs/private.log"},
			PageTitle: "Production dashboard",
		},
	}

	prompt := buildTaskContinuationPrompt(snapshot, map[string]any{
		"name": "write_file",
		"arguments": map[string]any{
			"path": "C:/secrets/todo.md",
		},
	}, taskContinuationContext{
		SessionMode: "implicit_active",
		Candidates:  []runengine.TaskRecord{candidate},
	})

	for _, sensitive := range []string{
		snapshot.Text,
		snapshot.SelectionText,
		snapshot.ErrorText,
		snapshot.PageURL,
		"logs/network.log",
		"logs/private.log",
		"C:/secrets/todo.md",
		candidate.SessionID,
		candidate.Title,
	} {
		if strings.Contains(prompt, sensitive) {
			t.Fatalf("expected prompt to redact %q, got %s", sensitive, prompt)
		}
	}
	if !strings.Contains(prompt, "task_id=task_001") {
		t.Fatalf("expected prompt to retain stable task identifiers, got %s", prompt)
	}
	if strings.Contains(prompt, "continuation_markers=") {
		t.Fatalf("expected prompt to stop relying on continuation markers, got %s", prompt)
	}
}

func TestCanContinueTaskOnlyAllowsExplicitFollowUpAndProcessingStates(t *testing.T) {
	for _, status := range []string{"waiting_input", "confirming_intent", "processing"} {
		if !canContinueTask(runengine.TaskRecord{Status: status}) {
			t.Fatalf("expected %s to remain continuation-eligible", status)
		}
	}
	for _, status := range []string{"waiting_auth", "paused", "blocked", "failed", "completed"} {
		if canContinueTask(runengine.TaskRecord{Status: status}) {
			t.Fatalf("expected %s to be excluded from continuation eligibility", status)
		}
	}
}

func TestClassifyTaskContinuationContinuesExplicitWaitingTaskWithoutSignalWords(t *testing.T) {
	service := newTestService()
	service.model = nil

	decision := service.classifyTaskContinuation(
		contextsvc.TaskContextSnapshot{
			Trigger:   "hover_text_input",
			InputType: "text",
			Text:      "把输出换成表格格式。",
		},
		nil,
		taskContinuationContext{
			SessionID:   "sess_active",
			SessionMode: "explicit_active",
			Candidates: []runengine.TaskRecord{{
				TaskID:      "task_001",
				SessionID:   "sess_active",
				Status:      "waiting_input",
				CurrentStep: "collect_input",
				UpdatedAt:   time.Now().Add(-10 * time.Second),
			}},
		},
	)

	if decision.Decision != "continue" || decision.TaskID != "task_001" {
		t.Fatalf("expected explicit waiting task to continue without signal words, got %+v", decision)
	}
}

func TestClassifyTaskContinuationRejectsWaitingTaskWhenAnchorsConflict(t *testing.T) {
	service := newTestService()
	service.model = nil

	decision := service.classifyTaskContinuation(
		contextsvc.TaskContextSnapshot{
			Trigger:   "hover_text_input",
			InputType: "text",
			Text:      "检查新的报错。",
			PageURL:   "https://example.com/build-b",
			AppName:   "Chrome",
		},
		nil,
		taskContinuationContext{
			SessionMode: "implicit_active",
			Candidates: []runengine.TaskRecord{{
				TaskID:      "task_001",
				Status:      "waiting_input",
				CurrentStep: "collect_input",
				UpdatedAt:   time.Now().Add(-10 * time.Second),
				Snapshot: contextsvc.TaskContextSnapshot{
					PageURL: "https://example.com/build-a",
					AppName: "Chrome",
				},
			}},
		},
	)

	if decision.Decision != "new_task" {
		t.Fatalf("expected conflicting anchors to force a new task, got %+v", decision)
	}
}

func TestClassifyTaskContinuationContinuesProcessingTaskOnStrongAttachmentEvidence(t *testing.T) {
	service := newTestService()
	service.model = nil

	decision := service.classifyTaskContinuation(
		contextsvc.TaskContextSnapshot{
			Trigger:   "file_drop",
			InputType: "file",
			Files:     []string{"logs/network.log"},
			PageURL:   "https://example.com/build",
			AppName:   "Chrome",
		},
		nil,
		taskContinuationContext{
			SessionMode: "implicit_active",
			Candidates: []runengine.TaskRecord{{
				TaskID:      "task_001",
				Status:      "processing",
				CurrentStep: "agent_loop",
				UpdatedAt:   time.Now().Add(-10 * time.Second),
				Snapshot: contextsvc.TaskContextSnapshot{
					PageURL: "https://example.com/build",
					AppName: "Chrome",
				},
			}},
		},
	)

	if decision.Decision != "continue" || decision.TaskID != "task_001" {
		t.Fatalf("expected strong context plus attachment evidence to continue the processing task, got %+v", decision)
	}
}

func TestHeuristicTaskContinuationDecisionDoesNotAutoMergeBareFileDropWithoutAnchors(t *testing.T) {
	decision := heuristicTaskContinuationDecision(
		contextsvc.TaskContextSnapshot{
			Trigger:   "file_drop",
			InputType: "file",
			Files:     []string{"logs/network.log"},
		},
		taskContinuationContext{
			Candidates: []runengine.TaskRecord{{
				TaskID:      "task_001",
				Status:      "processing",
				CurrentStep: "agent_loop",
				SourceType:  "hover_input",
				UpdatedAt:   time.Now().Add(-10 * time.Second),
				Intent:      map[string]any{"name": "write_file"},
			}},
		},
	)

	if decision.Decision != "new_task" {
		t.Fatalf("expected bare file drop to stay a new task when fallback runs, got %+v", decision)
	}
}

func TestClassifyTaskContinuationDoesNotAutoMergeSameExplicitIntentName(t *testing.T) {
	service := newTestService()
	service.model = nil

	decision := service.classifyTaskContinuation(
		contextsvc.TaskContextSnapshot{
			Trigger:   "hover_text_input",
			InputType: "text",
			Text:      "Draft a new release checklist.",
		},
		map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"path": "C:/secrets/release-checklist.md",
			},
		},
		taskContinuationContext{
			Candidates: []runengine.TaskRecord{{
				TaskID:      "task_001",
				Status:      "processing",
				CurrentStep: "agent_loop",
				SourceType:  "hover_input",
				UpdatedAt:   time.Now().Add(-10 * time.Second),
				Intent:      map[string]any{"name": "write_file"},
			}},
		},
	)

	if decision.Decision != "new_task" {
		t.Fatalf("expected same explicit intent name to stay a new task in fallback mode, got %+v", decision)
	}
}

func TestClassifyTaskContinuationDoesNotAutoMergeGenericFocusCueWithoutContext(t *testing.T) {
	service := newTestService()
	service.model = nil

	decision := service.classifyTaskContinuation(
		contextsvc.TaskContextSnapshot{
			Trigger:   "hover_text_input",
			InputType: "text",
			Text:      "Focus on drafting a release checklist.",
		},
		nil,
		taskContinuationContext{
			Candidates: []runengine.TaskRecord{{
				TaskID:      "task_001",
				Status:      "processing",
				CurrentStep: "agent_loop",
				SourceType:  "hover_input",
				UpdatedAt:   time.Now().Add(-10 * time.Second),
			}},
		},
	)

	if decision.Decision != "new_task" {
		t.Fatalf("expected generic focus cue without shared context to stay a new task in fallback mode, got %+v", decision)
	}
}
