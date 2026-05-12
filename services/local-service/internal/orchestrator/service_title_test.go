package orchestrator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/titlegen"
)

func TestServiceStartTaskUsesGeneratedTaskTitleFromFullContext(t *testing.T) {
	service, _ := newTestServiceWithModelClient(t, stubModelClient{
		generateText: func(request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
			if isTaskTitleGenerationRequest(request) {
				return model.GenerateTextResponse{
					OutputText: `{"title":"发布复盘风险跟进"}`,
					RequestID:  "req_task_title",
					Provider:   "openai",
					ModelID:    "gpt-title",
					Usage:      model.TokenUsage{InputTokens: 12, OutputTokens: 4, TotalTokens: 16},
					LatencyMS:  42,
				}, nil
			}
			return model.GenerateTextResponse{OutputText: "执行结果"}, nil
		},
	})
	service.WithTitleGenerator(titlegen.NewService(service.model))

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_generated_title",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"intent": map[string]any{
			"name": "summarize",
		},
		"input": map[string]any{
			"type": "text",
			"text": "请帮我整理这次发布复盘，并补齐风险项和后续跟进安排",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	task := startResult["task"].(map[string]any)
	taskID := task["task_id"].(string)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		record, ok := service.runEngine.GetTask(taskID)
		if ok && record.Title == "发布复盘风险跟进" &&
			record.TokenUsage["total_tokens"] == 16 &&
			hasTaskTitleAuditRecord(record.AuditRecords) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	record, _ := service.runEngine.GetTask(taskID)
	t.Fatalf("expected async task title refinement, got %+v", record)
}

func TestServiceStartTaskTitleRefreshDoesNotOverridePrimaryTokenMetadata(t *testing.T) {
	service, _ := newTestServiceWithModelClient(t, stubModelClient{
		generateText: func(request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
			if isTaskTitleGenerationRequest(request) {
				return model.GenerateTextResponse{
					OutputText: `{"title":"发布复盘风险跟进"}`,
					RequestID:  "req_title",
					Provider:   "openai",
					ModelID:    "gpt-title",
					Usage:      model.TokenUsage{InputTokens: 12, OutputTokens: 4, TotalTokens: 16},
					LatencyMS:  42,
				}, nil
			}
			return model.GenerateTextResponse{
				OutputText: "执行结果",
				RequestID:  "req_main",
				Provider:   "openai",
				ModelID:    "gpt-main",
				Usage:      model.TokenUsage{InputTokens: 30, OutputTokens: 10, TotalTokens: 40},
				LatencyMS:  180,
			}, nil
		},
	})
	service.WithTitleGenerator(titlegen.NewService(service.model))

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_task_title_token_metadata",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"intent": map[string]any{
			"name": "summarize",
		},
		"input": map[string]any{
			"type": "text",
			"text": "请帮我整理这次发布复盘，并补齐风险项和后续跟进安排",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		record, ok := service.runEngine.GetTask(taskID)
		if ok &&
			record.Title == "发布复盘风险跟进" &&
			record.TokenUsage["total_tokens"] == 56 &&
			hasTaskTitleAuditRecord(record.AuditRecords) &&
			record.TokenUsage["request_id"] == "req_main" &&
			record.TokenUsage["provider"] == "openai" &&
			record.TokenUsage["model_id"] == "gpt-main" &&
			record.TokenUsage["latency_ms"] == int64(180) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	record, _ := service.runEngine.GetTask(taskID)
	t.Fatalf("expected title refresh to preserve primary token metadata, got %+v", record)
}

func TestServiceSubmitInputUsesGeneratedTaskTitleFromFullContext(t *testing.T) {
	service, _ := newTestServiceWithModelClient(t, stubModelClient{
		generateText: func(request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
			if isTaskTitleGenerationRequest(request) {
				return model.GenerateTextResponse{
					OutputText: `{"title":"牛顿第一定律学习计划"}`,
					RequestID:  "req_submit_title",
					Provider:   "openai",
					ModelID:    "gpt-title",
					Usage:      model.TokenUsage{InputTokens: 14, OutputTokens: 5, TotalTokens: 19},
					LatencyMS:  37,
				}, nil
			}
			return model.GenerateTextResponse{OutputText: "先整理相关资料并给出学习摘要。"}, nil
		},
	})
	service.WithTitleGenerator(titlegen.NewService(service.model))

	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_submit_generated_title",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "现在我想复习高中物理，请帮我搜索相关资料，先给我总结牛顿第一定律相关内容",
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	taskID := result["task"].(map[string]any)["task_id"].(string)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		record, ok := service.runEngine.GetTask(taskID)
		if ok && record.Title == "牛顿第一定律学习计划" && hasTaskTitleAuditRecord(record.AuditRecords) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	record, _ := service.runEngine.GetTask(taskID)
	t.Fatalf("expected submit input to schedule async task title refinement, got %+v", record)
}

func TestServiceSubmitInputCompactsLongSingleSentenceFallbackTitle(t *testing.T) {
	service, _ := newTestServiceWithModelClient(t, stubModelClient{
		generateText: func(request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
			return model.GenerateTextResponse{OutputText: "先整理角色来源和代表同人作品。"}, nil
		},
	})

	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_submit_clause_fallback_title",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请详细介绍这次琪露诺是谁，出自哪部作品，出名的同人作有哪些",
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	task := result["task"].(map[string]any)
	if task["title"] != "请详细介绍这次琪露诺是谁 出自哪部作品" {
		t.Fatalf("expected submit input fallback title to compact long single sentence clauses, got %q", task["title"])
	}
}

func TestServiceConfirmTaskUsesGeneratedTaskTitleAfterConfirmation(t *testing.T) {
	service, _ := newTestServiceWithModelClient(t, stubModelClient{
		generateText: func(request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
			if isTaskTitleGenerationRequest(request) {
				return model.GenerateTextResponse{
					OutputText: `{"title":"发布复盘风险跟进"}`,
					RequestID:  "req_confirm_title",
					Provider:   "openai",
					ModelID:    "gpt-title",
					Usage:      model.TokenUsage{InputTokens: 12, OutputTokens: 4, TotalTokens: 16},
					LatencyMS:  42,
				}, nil
			}
			return model.GenerateTextResponse{OutputText: "执行结果"}, nil
		},
	})
	service.WithTitleGenerator(titlegen.NewService(service.model))

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_confirm_generated_title",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"options":    map[string]any{"confirm_required": true},
		"input": map[string]any{
			"type": "text",
			"text": "请帮我整理这次发布复盘，并补齐风险项和后续跟进安排",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	if _, err := service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": true,
	}); err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		record, ok := service.runEngine.GetTask(taskID)
		if ok && record.Title == "发布复盘风险跟进" && hasTaskTitleAuditRecord(record.AuditRecords) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	record, _ := service.runEngine.GetTask(taskID)
	t.Fatalf("expected confirm task to schedule async task title refinement, got %+v", record)
}

func TestServiceNotepadConvertToTaskUsesGeneratedTaskTitleFromFullContext(t *testing.T) {
	service, _ := newTestServiceWithModelClient(t, stubModelClient{
		generateText: func(request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
			if isTaskTitleGenerationRequest(request) {
				return model.GenerateTextResponse{
					OutputText: `{"title":"作业材料整理计划"}`,
					RequestID:  "req_notepad_title",
					Provider:   "openai",
					ModelID:    "gpt-title",
					Usage:      model.TokenUsage{InputTokens: 11, OutputTokens: 4, TotalTokens: 15},
					LatencyMS:  35,
				}, nil
			}
			return model.GenerateTextResponse{OutputText: "Converted notepad task finished."}, nil
		},
	})
	service.WithTitleGenerator(titlegen.NewService(service.model))
	service.runEngine.ReplaceNotepadItems([]map[string]any{{
		"item_id":          "todo_generated_title",
		"title":            "translate the meeting notes",
		"bucket":           "upcoming",
		"status":           "normal",
		"type":             "todo_item",
		"note_text":        "Finish the computer homework before tonight and use the materials in workspace/homework.",
		"agent_suggestion": "translate into English",
	}})

	result, err := service.NotepadConvertToTask(map[string]any{
		"item_id":   "todo_generated_title",
		"confirmed": true,
	})
	if err != nil {
		t.Fatalf("notepad convert failed: %v", err)
	}

	taskID := result["task"].(map[string]any)["task_id"].(string)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		record, ok := service.runEngine.GetTask(taskID)
		if ok && record.Title == "作业材料整理计划" && hasTaskTitleAuditRecord(record.AuditRecords) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	record, _ := service.runEngine.GetTask(taskID)
	t.Fatalf("expected notepad conversion to schedule async task title refinement, got %+v", record)
}

func TestServiceNotepadConvertToTaskIgnoresExportedRuntimeMetadataInAsyncTitleRefresh(t *testing.T) {
	service, workspaceRoot := newTestServiceWithModelClient(t, stubModelClient{
		generateText: func(request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
			if isTaskTitleGenerationRequest(request) {
				title := "创新创业文档总结"
				if strings.Contains(request.Input, "created_at:") || strings.Contains(request.Input, "linked_task_id:") {
					title = "created_at: 2026-05-12..."
				}
				return model.GenerateTextResponse{
					OutputText: `{"title":"` + title + `"}`,
					RequestID:  "req_notepad_exported_title",
					Provider:   "openai",
					ModelID:    "gpt-title",
					Usage:      model.TokenUsage{InputTokens: 13, OutputTokens: 4, TotalTokens: 17},
					LatencyMS:  35,
				}, nil
			}
			return model.GenerateTextResponse{OutputText: "Converted notepad task finished."}, nil
		},
	})
	service.WithTitleGenerator(titlegen.NewService(service.model))

	if err := os.MkdirAll(filepath.Join(workspaceRoot, "todos"), 0o755); err != nil {
		t.Fatalf("mkdir todos: %v", err)
	}
	content := strings.Join([]string{
		"- [ ] 理解创新创业基础文档",
		"  附件:2025创新创业基础知行汇(4).doc，说明:我不太明白这个文档在讲什么，帮我总结一下下",
		"  created_at: 2026-05-12T01:00:00Z",
		"  updated_at: 2026-05-12T01:05:00Z",
		"  linked_task_id: task_old",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "exported.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write exported note: %v", err)
	}

	if _, err := service.TaskInspectorRun(map[string]any{
		"target_sources": []any{"workspace/todos"},
	}); err != nil {
		t.Fatalf("TaskInspectorRun failed: %v", err)
	}

	items, total := service.runEngine.NotepadItems("upcoming", 10, 0)
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected one synced note item, got total=%d items=%+v", total, items)
	}
	if strings.Contains(stringValue(items[0], "note_text", ""), "created_at:") {
		t.Fatalf("expected synced note_text to exclude exported runtime metadata, got %+v", items[0])
	}

	result, err := service.NotepadConvertToTask(map[string]any{
		"item_id":   stringValue(items[0], "item_id", ""),
		"confirmed": true,
	})
	if err != nil {
		t.Fatalf("notepad convert failed: %v", err)
	}

	taskID := result["task"].(map[string]any)["task_id"].(string)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		record, ok := service.runEngine.GetTask(taskID)
		if ok && record.Title == "创新创业文档总结" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	record, _ := service.runEngine.GetTask(taskID)
	t.Fatalf("expected async title refresh to ignore exported runtime metadata, got %+v", record)
}

func hasTaskTitleAuditRecord(records []map[string]any) bool {
	for _, record := range records {
		if stringValue(record, "action", "") == "title.generate" {
			return true
		}
	}
	return false
}

func hasTaskTitleAuditResult(records []map[string]any, want string) bool {
	for _, record := range records {
		if stringValue(record, "action", "") == "title.generate" &&
			stringValue(record, "result", "") == want {
			return true
		}
	}
	return false
}

func TestServiceStartTaskConfirmingIntentDoesNotGenerateTitleBeforeConfirmation(t *testing.T) {
	modelClient := &blockingModelClient{
		started:  make(chan string, 1),
		released: make(chan struct{}, 1),
	}
	service, _ := newTestServiceWithModelClient(t, modelClient)
	service.WithTitleGenerator(titlegen.NewService(service.model))

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_confirm_title_boundary",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"intent":     map[string]any{"name": "translate"},
		"options":    map[string]any{"confirm_required": true},
		"input": map[string]any{
			"type": "text",
			"text": "请帮我翻译这次发布复盘，并补齐风险项和后续跟进安排",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	select {
	case taskID := <-modelClient.started:
		t.Fatalf("expected confirming_intent start to avoid title generation, got %s", taskID)
	case <-time.After(200 * time.Millisecond):
	}

	task := startResult["task"].(map[string]any)
	if task["status"] != "confirming_intent" {
		t.Fatalf("expected confirming_intent status, got %+v", task)
	}
}

func TestServiceStartTaskDoesNotBlockOnAsyncTitleGeneration(t *testing.T) {
	titleStarted := make(chan string, 1)
	titleReleased := make(chan struct{}, 1)
	titleAllowReturn := make(chan struct{})
	service, _ := newTestServiceWithModelClient(t, &titleBlockingModelClient{
		started:         titleStarted,
		released:        titleReleased,
		allowReturn:     titleAllowReturn,
		immediateOutput: "执行结果",
	})
	service.WithTitleGenerator(titlegen.NewService(service.model))

	resultCh := make(chan map[string]any, 1)
	errCh := make(chan error, 1)
	go func() {
		startResult, err := service.StartTask(map[string]any{
			"session_id": "sess_non_blocking_title",
			"source":     "floating_ball",
			"trigger":    "hover_text_input",
			"intent": map[string]any{
				"name": "summarize",
			},
			"input": map[string]any{
				"type": "text",
				"text": "请帮我整理这次发布复盘，并补齐风险项和后续跟进安排",
			},
		})
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- startResult
	}()

	select {
	case <-titleStarted:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected async title generation to start")
	}

	select {
	case err := <-errCh:
		t.Fatalf("start task failed: %v", err)
	case startResult := <-resultCh:
		select {
		case <-titleReleased:
			t.Fatal("expected task start to return while title generation was still blocked")
		default:
		}
		task := startResult["task"].(map[string]any)
		if task["title"] == "发布复盘风险跟进" {
			t.Fatalf("expected hot path to return fallback title before async refinement, got %+v", task)
		}
	case <-time.After(time.Second):
		t.Fatal("expected task start to return before title generation was released")
	}

	close(titleAllowReturn)
	select {
	case <-titleReleased:
	case <-time.After(time.Second):
		t.Fatal("expected background title generation to exit after release")
	}
}

func TestServiceStartTaskAuthorizationGateDefersTitleGenerationUntilApproval(t *testing.T) {
	titleStarted := make(chan string, 2)
	service, _ := newTestServiceWithModelClient(t, stubModelClient{
		generateText: func(request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
			if isTaskTitleGenerationRequest(request) {
				select {
				case titleStarted <- request.TaskID:
				default:
				}
				return model.GenerateTextResponse{OutputText: `{"title":"发布复盘风险跟进"}`}, nil
			}
			return model.GenerateTextResponse{OutputText: "授权后执行完成"}, nil
		},
	})
	service.WithTitleGenerator(titlegen.NewService(service.model))

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_title_waiting_auth_start",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
		"input": map[string]any{
			"type": "text",
			"text": "请帮我整理这次发布复盘，并补齐风险项和后续跟进安排",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	task := startResult["task"].(map[string]any)
	taskID := task["task_id"].(string)
	if task["status"] != "waiting_auth" {
		t.Fatalf("expected authorization-gated task to pause before execution, got %+v", task)
	}

	select {
	case startedTaskID := <-titleStarted:
		t.Fatalf("expected waiting_auth start to avoid title generation before approval, got %s", startedTaskID)
	case <-time.After(200 * time.Millisecond):
	}

	if _, err := service.SecurityRespond(map[string]any{
		"task_id":     taskID,
		"approval_id": activeApprovalIDForTask(t, service, taskID),
		"decision":    "allow_once",
	}); err != nil {
		t.Fatalf("security respond failed: %v", err)
	}

	select {
	case <-titleStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected title generation to start after approval")
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		record, ok := service.runEngine.GetTask(taskID)
		if ok && record.Title == "发布复盘风险跟进" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	record, _ := service.runEngine.GetTask(taskID)
	t.Fatalf("expected approved task title refinement, got %+v", record)
}

func TestServiceConfirmTaskAuthorizationGateDefersTitleGenerationUntilApproval(t *testing.T) {
	titleStarted := make(chan string, 2)
	service, _ := newTestServiceWithModelClient(t, stubModelClient{
		generateText: func(request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
			if isTaskTitleGenerationRequest(request) {
				select {
				case titleStarted <- request.TaskID:
				default:
				}
				return model.GenerateTextResponse{OutputText: `{"title":"发布复盘风险跟进"}`}, nil
			}
			return model.GenerateTextResponse{OutputText: "授权后执行完成"}, nil
		},
	})
	service.WithTitleGenerator(titlegen.NewService(service.model))

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_title_waiting_auth_confirm",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"options":    map[string]any{"confirm_required": true},
		"input": map[string]any{
			"type": "text",
			"text": "请帮我整理这次发布复盘，并补齐风险项和后续跟进安排",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}
	taskID := startResult["task"].(map[string]any)["task_id"].(string)

	confirmResult, err := service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": false,
		"corrected_intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}
	task := confirmResult["task"].(map[string]any)
	if task["status"] != "waiting_auth" {
		t.Fatalf("expected confirm path to stop at waiting_auth, got %+v", task)
	}

	select {
	case startedTaskID := <-titleStarted:
		t.Fatalf("expected confirm path to avoid title generation before approval, got %s", startedTaskID)
	case <-time.After(200 * time.Millisecond):
	}

	if _, err := service.SecurityRespond(map[string]any{
		"task_id":     taskID,
		"approval_id": activeApprovalIDForTask(t, service, taskID),
		"decision":    "allow_once",
	}); err != nil {
		t.Fatalf("security respond failed: %v", err)
	}

	select {
	case <-titleStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected title generation to start after approval")
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		record, ok := service.runEngine.GetTask(taskID)
		if ok && record.Title == "发布复盘风险跟进" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	record, _ := service.runEngine.GetTask(taskID)
	t.Fatalf("expected confirmed task title refinement after approval, got %+v", record)
}

func TestServiceTaskTitleRefreshKeepsLatestReservation(t *testing.T) {
	client := &stagedTitleModelClient{
		firstStarted:    make(chan struct{}),
		allowFirstReply: make(chan struct{}),
	}
	service, _ := newTestServiceWithModelClient(t, client)
	service.WithTitleGenerator(titlegen.NewService(service.model))

	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_title_refresh_race",
		Title:       "构建失败分析",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{}},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
		Snapshot: taskcontext.TaskContextSnapshot{
			InputType: "text",
			Text:      "请分析构建失败",
			Trigger:   "hover_text_input",
		},
	})

	service.scheduleTaskTitleRefresh(task, task.Snapshot, task.Intent, task.Title)
	select {
	case <-client.firstStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected first title refresh to start")
	}

	service.scheduleTaskTitleRefresh(task, taskcontext.TaskContextSnapshot{
		InputType: "text",
		Text:      "请分析构建失败，并补充最新客户影响",
		Trigger:   "hover_text_input",
	}, task.Intent, task.Title)

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		record, ok := service.runEngine.GetTask(task.TaskID)
		if ok && record.Title == "最新上下文标题" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	record, _ := service.runEngine.GetTask(task.TaskID)
	t.Fatalf("expected newest async title refresh to win, got %+v", record)
}

func TestServiceTaskTitleRefreshCancelsSupersededModelCall(t *testing.T) {
	client := &cancelAwareTitleModelClient{
		firstStarted:  make(chan struct{}),
		firstCanceled: make(chan struct{}),
		secondStarted: make(chan struct{}),
	}
	service, _ := newTestServiceWithModelClient(t, client)
	service.WithTitleGenerator(titlegen.NewService(service.model))

	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_title_refresh_cancel",
		Title:       "构建失败分析",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{}},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
		Snapshot: taskcontext.TaskContextSnapshot{
			InputType: "text",
			Text:      "请分析构建失败",
			Trigger:   "hover_text_input",
		},
	})

	service.scheduleTaskTitleRefresh(task, task.Snapshot, task.Intent, task.Title)
	select {
	case <-client.firstStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected first title refresh to start")
	}

	updatedTask, ok := service.runEngine.ContinueTask(task.TaskID, runengine.ContinuationUpdate{
		Snapshot: taskcontext.TaskContextSnapshot{
			InputType: "text",
			Text:      "请分析构建失败，并补充最新客户影响",
			Trigger:   "hover_text_input",
		},
		Title:       task.Title,
		Intent:      task.Intent,
		Status:      task.Status,
		CurrentStep: task.CurrentStep,
	})
	if !ok {
		t.Fatal("expected continuation update to succeed")
	}
	service.scheduleTaskTitleRefresh(updatedTask, snapshotFromTask(updatedTask), updatedTask.Intent, updatedTask.Title)

	select {
	case <-client.firstCanceled:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected superseded title refresh to be canceled")
	}
	select {
	case <-client.secondStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected second title refresh to start")
	}

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		record, ok := service.runEngine.GetTask(task.TaskID)
		if ok && record.Title == "最新上下文标题" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	record, _ := service.runEngine.GetTask(task.TaskID)
	t.Fatalf("expected canceled stale refresh and successful latest title update, got %+v", record)
}

func TestServiceTaskTitleRefreshReservesNewTokenForEachSchedule(t *testing.T) {
	client := &orderedTitleModelClient{
		firstStarted:     make(chan struct{}),
		secondStarted:    make(chan struct{}),
		allowFirstReply:  make(chan struct{}),
		allowSecondReply: make(chan struct{}),
	}
	service, _ := newTestServiceWithModelClient(t, client)
	service.WithTitleGenerator(titlegen.NewService(service.model))

	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_title_refresh_new_token",
		Title:       "构建失败分析",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{}},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
		Snapshot: taskcontext.TaskContextSnapshot{
			InputType: "text",
			Text:      "请分析构建失败",
			Trigger:   "hover_text_input",
		},
	})

	service.scheduleTaskTitleRefresh(task, task.Snapshot, task.Intent, task.Title)
	select {
	case <-client.firstStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected first title refresh to start")
	}

	service.scheduleTaskTitleRefresh(task, taskcontext.TaskContextSnapshot{
		InputType: "text",
		Text:      "请分析构建失败，并补充最新客户影响",
		Trigger:   "hover_text_input",
	}, task.Intent, task.Title)
	select {
	case <-client.secondStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected second title refresh to start")
	}

	close(client.allowFirstReply)
	time.Sleep(50 * time.Millisecond)
	record, _ := service.runEngine.GetTask(task.TaskID)
	if record.Title == "旧上下文标题" {
		t.Fatalf("expected older refresh to lose once a newer schedule reserved ownership, got %+v", record)
	}

	close(client.allowSecondReply)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		record, ok := service.runEngine.GetTask(task.TaskID)
		if ok && record.Title == "最新上下文标题" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	record, _ = service.runEngine.GetTask(task.TaskID)
	t.Fatalf("expected latest refresh to win after older reply arrived first, got %+v", record)
}

func TestServiceTaskTitleRefreshRejectsStaleSchedulerAfterNewStateVisible(t *testing.T) {
	service, _ := newTestServiceWithModelClient(t, stubModelClient{
		generateText: func(request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
			if !isTaskTitleGenerationRequest(request) {
				return model.GenerateTextResponse{OutputText: "执行结果"}, nil
			}
			if strings.Contains(request.Input, "最新客户影响") {
				return model.GenerateTextResponse{OutputText: `{"title":"最新上下文标题"}`}, nil
			}
			return model.GenerateTextResponse{OutputText: `{"title":"旧上下文标题"}`}, nil
		},
	})
	service.WithTitleGenerator(titlegen.NewService(service.model))

	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_stale_title_scheduler",
		Title:       "构建失败分析",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{}},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
		Snapshot: taskcontext.TaskContextSnapshot{
			InputType: "text",
			Text:      "请分析构建失败",
			Trigger:   "hover_text_input",
		},
	})
	initialTask := task
	updatedTask, ok := service.runEngine.ContinueTask(task.TaskID, runengine.ContinuationUpdate{
		Snapshot: taskcontext.TaskContextSnapshot{
			InputType: "text",
			Text:      "请分析构建失败，并补充最新客户影响",
			Trigger:   "hover_text_input",
		},
		Title:       task.Title,
		Intent:      task.Intent,
		Status:      task.Status,
		CurrentStep: task.CurrentStep,
	})
	if !ok {
		t.Fatal("expected continuation update to succeed")
	}

	service.scheduleTaskTitleRefresh(initialTask, initialTask.Snapshot, initialTask.Intent, initialTask.Title)
	time.Sleep(50 * time.Millisecond)
	record, _ := service.runEngine.GetTask(task.TaskID)
	if record.Title == "旧上下文标题" {
		t.Fatalf("expected stale scheduler to lose after newer state was published, got %+v", record)
	}

	service.scheduleTaskTitleRefresh(updatedTask, snapshotFromTask(updatedTask), updatedTask.Intent, updatedTask.Title)
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		record, ok := service.runEngine.GetTask(task.TaskID)
		if ok && record.Title == "最新上下文标题" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	record, _ = service.runEngine.GetTask(task.TaskID)
	t.Fatalf("expected newest visible state to own the final title refresh, got %+v", record)
}

func TestServiceTaskTitleRefreshAuditMarksFallbackResult(t *testing.T) {
	service, _ := newTestServiceWithModelClient(t, stubModelClient{
		generateText: func(request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
			if isTaskTitleGenerationRequest(request) {
				return model.GenerateTextResponse{
					TaskID:     request.TaskID,
					RunID:      request.RunID,
					RequestID:  "req_title_fallback",
					Provider:   "openai_responses",
					ModelID:    "gpt-5.4",
					OutputText: `{"title":""}`,
				}, nil
			}
			return model.GenerateTextResponse{OutputText: "执行结果"}, nil
		},
	})
	service.WithTitleGenerator(titlegen.NewService(service.model))

	task := service.runEngine.CreateTask(runengine.CreateTaskInput{
		SessionID:   "sess_title_audit_fallback",
		Title:       "构建失败分析",
		SourceType:  "hover_input",
		Status:      "processing",
		Intent:      map[string]any{"name": "summarize", "arguments": map[string]any{}},
		CurrentStep: "generate_output",
		RiskLevel:   "green",
		Snapshot: taskcontext.TaskContextSnapshot{
			InputType: "text",
			Text:      "请分析构建失败",
			Trigger:   "hover_text_input",
		},
	})

	service.scheduleTaskTitleRefresh(task, task.Snapshot, task.Intent, task.Title)

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		record, ok := service.runEngine.GetTask(task.TaskID)
		if ok && hasTaskTitleAuditResult(record.AuditRecords, "fallback") {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	record, _ := service.runEngine.GetTask(task.TaskID)
	t.Fatalf("expected fallback title generation audit result, got %+v", record.AuditRecords)
}
