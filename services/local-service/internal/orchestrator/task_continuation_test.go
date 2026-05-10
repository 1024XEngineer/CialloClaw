package orchestrator

import (
	"strings"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	taskcontext "github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
)

func TestResolveTaskContinuationContextUsesSingleActiveSession(t *testing.T) {
	service := newTestService()
	activeTask := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_active",
		Title:       "Analyze the current failure",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "agent_loop", "arguments": map[string]any{}},
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
	snapshot := taskcontext.TaskContextSnapshot{
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
		Snapshot: taskcontext.TaskContextSnapshot{
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
	}, taskContinuationOptions{})

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
	for _, expected := range []string{
		"resolved_intent_name=write_file",
		"resolved_delivery_type=workspace_document",
		"intent_name=agent_loop",
		"delivery_type=bubble",
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("expected prompt to include %q, got %s", expected, prompt)
		}
	}
	if strings.Contains(prompt, "continuation_markers=") {
		t.Fatalf("expected prompt to stop relying on continuation markers, got %s", prompt)
	}
}

func TestTaskContinuationInputSummaryUsesConfirmationPolicy(t *testing.T) {
	snapshot := taskcontext.TaskContextSnapshot{
		Trigger:   "file_drop",
		InputType: "file",
		Text:      "Summarize the attachment.",
		Files:     []string{"notes/source.md"},
	}

	directSummary := taskContinuationInputSummary(snapshot, nil, taskContinuationOptions{})
	if !strings.Contains(directSummary, "requires_confirmation=false") {
		t.Fatalf("expected described file input to keep direct-start confirmation semantics, got %s", directSummary)
	}

	forcedSummary := taskContinuationInputSummary(snapshot, nil, taskContinuationOptions{ConfirmRequired: true})
	if !strings.Contains(forcedSummary, "requires_confirmation=true") {
		t.Fatalf("expected explicit confirmation policy to reach continuation summary, got %s", forcedSummary)
	}
}

func TestCanContinueTaskOnlyAllowsExplicitFollowUpAndLoopProcessingStates(t *testing.T) {
	for _, status := range []string{"waiting_input", "confirming_intent"} {
		if !canContinueTask(runengine.TaskRecord{Status: status}) {
			t.Fatalf("expected %s to remain continuation-eligible", status)
		}
	}
	if !canContinueTask(runengine.TaskRecord{Status: "processing", Intent: map[string]any{"name": "agent_loop"}, CurrentStep: "agent_loop"}) {
		t.Fatal("expected agent-loop processing task to remain continuation-eligible")
	}
	if canContinueTask(runengine.TaskRecord{Status: "processing", Intent: map[string]any{"name": "agent_loop"}, CurrentStep: "generate_output"}) {
		t.Fatal("expected agent-loop prompt fallback to be excluded from continuation eligibility")
	}
	if canContinueTask(runengine.TaskRecord{Status: "processing", Intent: map[string]any{"name": "summarize"}}) {
		t.Fatal("expected prompt-path processing task to be excluded from continuation eligibility")
	}
	for _, status := range []string{"waiting_auth", "paused", "blocked", "failed", "completed"} {
		if canContinueTask(runengine.TaskRecord{Status: status}) {
			t.Fatalf("expected %s to be excluded from continuation eligibility", status)
		}
	}
}

func TestPendingContinuationRequiresConfirmAndIntentPreserveConfirmationTasks(t *testing.T) {
	task := runengine.TaskRecord{
		Status: "confirming_intent",
		Intent: map[string]any{"name": "summarize", "arguments": map[string]any{"style": "bullet_points"}},
	}
	snapshot := taskcontext.TaskContextSnapshot{
		Trigger:   "hover_text_input",
		InputType: "text",
		Text:      "Please focus on the outage timeline.",
	}

	if !pendingContinuationRequiresConfirm(task, snapshot, taskContinuationOptions{}) {
		t.Fatal("expected confirming_intent continuations to keep confirmation required")
	}
	if !pendingContinuationRequiresConfirm(task, snapshot, taskContinuationOptions{ForceConfirmRequired: true}) {
		t.Fatal("expected explicit force confirm option to remain true")
	}

	intent := pendingContinuationIntent(task, nil)
	if stringValue(intent, "name", "") != "summarize" {
		t.Fatalf("expected confirmation-stage continuation to preserve task intent, got %+v", intent)
	}
	intent["name"] = "mutated"
	if stringValue(task.Intent, "name", "") != "summarize" {
		t.Fatalf("expected continuation intent clone to avoid mutating task intent, got %+v", task.Intent)
	}

	explicitIntent := map[string]any{"name": "rewrite"}
	if got := pendingContinuationIntent(task, explicitIntent); stringValue(got, "name", "") != "rewrite" {
		t.Fatalf("expected explicit continuation intent to win, got %+v", got)
	}
}

func TestBuildTaskContinuationBubbleTextOmitsInternalReason(t *testing.T) {
	snapshot := taskcontext.TaskContextSnapshot{
		Trigger:   "hover_text_input",
		InputType: "text",
		Text:      "Use markdown headings in the next reply.",
	}
	decision := taskContinuationDecision{
		Decision: "continue",
		Reason:   "internal classifier detail should stay hidden",
	}

	bubble := buildTaskContinuationBubbleText(snapshot, decision)
	if !strings.Contains(bubble, "已把补充说明挂回当前任务。") {
		t.Fatalf("expected continuation subject to stay visible, got %q", bubble)
	}
	if strings.Contains(bubble, decision.Reason) {
		t.Fatalf("expected internal continuation reason to stay hidden, got %q", bubble)
	}
}

func TestClassifyTaskContinuationContinuesExplicitWaitingTaskWithoutSignalWords(t *testing.T) {
	service := newTestService()
	service.model = nil

	decision := service.classifyTaskContinuation(
		taskcontext.TaskContextSnapshot{
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
		taskContinuationOptions{},
	)

	if decision.Decision != "continue" || decision.TaskID != "task_001" {
		t.Fatalf("expected explicit waiting task to continue without signal words, got %+v", decision)
	}
}

func TestClassifyTaskContinuationStartsNewTaskForExplicitIntentWithoutAnchors(t *testing.T) {
	service := newTestService()
	service.model = nil

	decision := service.classifyTaskContinuation(
		taskcontext.TaskContextSnapshot{
			Trigger:   "hover_text_input",
			InputType: "text",
			Text:      "顺便帮我写一份周报。",
		},
		map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"target_path": "workspace/reports/weekly.md",
			},
		},
		taskContinuationContext{
			SessionMode: "implicit_active",
			Candidates: []runengine.TaskRecord{{
				TaskID:      "task_001",
				Status:      "waiting_input",
				CurrentStep: "collect_input",
				UpdatedAt:   time.Now().Add(-10 * time.Second),
			}},
		},
		taskContinuationOptions{},
	)

	if decision.Decision != "new_task" {
		t.Fatalf("expected explicit intent without anchors to open a new task, got %+v", decision)
	}
}

func TestClassifyTaskContinuationRejectsWaitingTaskWhenAnchorsConflict(t *testing.T) {
	service := newTestService()
	service.model = nil

	decision := service.classifyTaskContinuation(
		taskcontext.TaskContextSnapshot{
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
				Snapshot: taskcontext.TaskContextSnapshot{
					PageURL: "https://example.com/build-a",
					AppName: "Chrome",
				},
			}},
		},
		taskContinuationOptions{},
	)

	if decision.Decision != "new_task" {
		t.Fatalf("expected conflicting anchors to force a new task, got %+v", decision)
	}
}

func TestClassifyTaskContinuationContinuesConfirmRequiredPendingTaskWithMatchingAnchor(t *testing.T) {
	service := newTestService()
	service.model = nil

	decision := service.classifyTaskContinuation(
		taskcontext.TaskContextSnapshot{
			Trigger:   "file_drop",
			InputType: "file",
			Files:     []string{"logs/network.log"},
			PageTitle: "Build Dashboard",
			PageURL:   "https://example.com/build-a",
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
				Snapshot: taskcontext.TaskContextSnapshot{
					PageTitle:   "Build Dashboard",
					PageURL:     "https://example.com/build-a",
					AppName:     "Chrome",
					WindowTitle: "Browser - Build Dashboard",
				},
			}},
		},
		taskContinuationOptions{ConfirmRequired: true},
	)

	if decision.Decision != "continue" || decision.TaskID != "task_001" {
		t.Fatalf("expected matching anchored file intake to continue the pending task, got %+v", decision)
	}
}

func TestClassifyTaskContinuationStartsNewConfirmRequiredTaskWithoutTaskEvidence(t *testing.T) {
	service := newTestService()
	service.model = nil

	decision := service.classifyTaskContinuation(
		taskcontext.TaskContextSnapshot{
			Trigger:   "file_drop",
			InputType: "file",
			Files:     []string{"logs/network.log"},
			PageTitle: "Quick Intake",
			PageURL:   "local://shell-ball",
			AppName:   "desktop",
		},
		nil,
		taskContinuationContext{
			SessionMode: "implicit_active",
			Candidates: []runengine.TaskRecord{{
				TaskID:      "task_001",
				Status:      "waiting_input",
				CurrentStep: "collect_input",
				UpdatedAt:   time.Now().Add(-10 * time.Second),
				Snapshot: taskcontext.TaskContextSnapshot{
					PageTitle:   "Build Dashboard",
					PageURL:     "https://example.com/build-a",
					AppName:     "Chrome",
					WindowTitle: "Browser - Build Dashboard",
				},
			}},
		},
		taskContinuationOptions{ConfirmRequired: true},
	)

	if decision.Decision != "new_task" {
		t.Fatalf("expected shell-ball-only structured input to open a new task, got %+v", decision)
	}
}

func TestClassifyTaskContinuationStartsNewStructuredMultiCandidateWithoutUniqueMatch(t *testing.T) {
	var modelCalled bool
	service, _ := newTestServiceWithModelClient(t, stubModelClient{
		generateText: func(request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
			modelCalled = true
			return model.GenerateTextResponse{
				TaskID:     request.TaskID,
				RunID:      request.RunID,
				RequestID:  "req_structured_multi_candidate_no_match",
				Provider:   "openai_responses",
				ModelID:    "gpt-5.4",
				OutputText: `{"decision":"continue","task_id":"task_001","reason":"model must not choose unanchored structured continuation"}`,
				Usage:      model.TokenUsage{InputTokens: 9, OutputTokens: 13, TotalTokens: 22},
				LatencyMS:  25,
			}, nil
		},
	})

	decision := service.classifyTaskContinuation(
		taskcontext.TaskContextSnapshot{
			Trigger:     "file_drop",
			InputType:   "file",
			Files:       []string{"logs/network.log"},
			PageTitle:   "Quick Intake",
			PageURL:     "local://shell-ball",
			AppName:     "desktop",
			WindowTitle: "Shell Ball",
		},
		nil,
		taskContinuationContext{
			SessionMode: "explicit_active",
			Candidates: []runengine.TaskRecord{
				{
					TaskID:      "task_001",
					Status:      "waiting_input",
					CurrentStep: "collect_input",
					UpdatedAt:   time.Now().Add(-10 * time.Second),
					Snapshot: taskcontext.TaskContextSnapshot{
						PageTitle:   "Build Dashboard",
						PageURL:     "https://example.com/build",
						AppName:     "Chrome",
						WindowTitle: "Browser - Build Dashboard",
					},
				},
				{
					TaskID:      "task_002",
					Status:      "waiting_input",
					CurrentStep: "collect_input",
					UpdatedAt:   time.Now().Add(-9 * time.Second),
					Snapshot: taskcontext.TaskContextSnapshot{
						PageTitle:   "Issue Tracker",
						PageURL:     "https://example.com/issues",
						AppName:     "Chrome",
						WindowTitle: "Browser - Issue Tracker",
					},
				},
			},
		},
		taskContinuationOptions{},
	)

	if modelCalled {
		t.Fatal("expected unanchored structured multi-candidate input to bypass model continuation")
	}
	if decision.Decision != "new_task" {
		t.Fatalf("expected structured input without a unique task-specific match to open a new task, got %+v", decision)
	}
}

func TestClassifyTaskContinuationStartsNewTaskForProcessingStructuredEvidence(t *testing.T) {
	service := newTestService()
	service.model = nil

	decision := service.classifyTaskContinuation(
		taskcontext.TaskContextSnapshot{
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
				Snapshot: taskcontext.TaskContextSnapshot{
					PageURL: "https://example.com/build",
					AppName: "Chrome",
				},
			}},
		},
		taskContinuationOptions{},
	)

	if decision.Decision != "new_task" {
		t.Fatalf("expected structured evidence not to attach to the processing task, got %+v", decision)
	}
}

func TestHeuristicTaskContinuationDecisionDoesNotAutoMergeBareFileDropWithoutAnchors(t *testing.T) {
	decision := heuristicTaskContinuationDecision(
		taskcontext.TaskContextSnapshot{
			Trigger:   "file_drop",
			InputType: "file",
			Files:     []string{"logs/network.log"},
		},
		nil,
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
		taskcontext.TaskContextSnapshot{
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
		taskContinuationOptions{},
	)

	if decision.Decision != "new_task" {
		t.Fatalf("expected same explicit intent name to stay a new task in fallback mode, got %+v", decision)
	}
}

func TestClassifyTaskContinuationDoesNotAutoMergeGenericFocusCueWithoutContext(t *testing.T) {
	service := newTestService()
	service.model = nil

	decision := service.classifyTaskContinuation(
		taskcontext.TaskContextSnapshot{
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
		taskContinuationOptions{},
	)

	if decision.Decision != "new_task" {
		t.Fatalf("expected generic focus cue without shared context to stay a new task in fallback mode, got %+v", decision)
	}
}
