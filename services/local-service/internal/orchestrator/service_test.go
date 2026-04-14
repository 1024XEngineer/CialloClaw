// 该测试文件验证主链路编排与对接点行为。
package orchestrator

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/execution"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/memory"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/risk"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskinspector"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/builtin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/sidecarclient"
)

// TestServiceStartTaskAndConfirmFlow 验证确认后的普通任务会继续执行并完成交付。
type stubModelClient struct {
	output string
}

func (s stubModelClient) GenerateText(_ context.Context, request model.GenerateTextRequest) (model.GenerateTextResponse, error) {
	return model.GenerateTextResponse{
		TaskID:     request.TaskID,
		RunID:      request.RunID,
		RequestID:  "req_test",
		Provider:   "openai_responses",
		ModelID:    "gpt-5.4",
		OutputText: s.output,
		Usage: model.TokenUsage{
			InputTokens:  12,
			OutputTokens: 24,
			TotalTokens:  36,
		},
		LatencyMS: 42,
	}, nil
}

func timePointer(value time.Time) *time.Time {
	return &value
}

func newTestServiceWithExecution(t *testing.T, modelOutput string) (*Service, string) {
	t.Helper()

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy: %v", err)
	}

	modelService := model.NewService(modelConfig(), stubModelClient{output: modelOutput})
	auditService := audit.NewService()
	deliveryService := delivery.NewService()
	toolRegistry := tools.NewRegistry()
	if err := builtin.RegisterBuiltinTools(toolRegistry); err != nil {
		t.Fatalf("register builtin tools: %v", err)
	}
	toolExecutor := tools.NewToolExecutor(toolRegistry)
	pluginService := plugin.NewService()
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	executor := execution.NewService(fileSystem, platform.LocalExecutionBackend{}, sidecarclient.NewNoopPlaywrightSidecarClient(), modelService, auditService, checkpoint.NewService(), deliveryService, toolRegistry, toolExecutor, pluginService)
	storageService := storage.NewService(platform.NewLocalStorageAdapter(filepath.Join(t.TempDir(), "service.db")))
	t.Cleanup(func() { _ = storageService.Close() })

	service := NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		deliveryService,
		memory.NewService(),
		risk.NewService(),
		modelService,
		toolRegistry,
		pluginService,
	).WithAudit(auditService).WithStorage(storageService).WithExecutor(executor).WithTaskInspector(taskinspector.NewService(fileSystem))

	return service, workspaceRoot
}

func newTestService() *Service {
	return NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		tools.NewRegistry(),
		plugin.NewService(),
	)
}

func TestServiceStartTaskAndConfirmFlow(t *testing.T) {
	service := NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		tools.NewRegistry(),
		plugin.NewService(),
	)

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "这里是一段需要解释的内容",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	startedTask := startResult["task"].(map[string]any)
	if startedTask["status"] != "confirming_intent" {
		t.Fatalf("expected confirming_intent status, got %v", startedTask["status"])
	}

	taskID := startedTask["task_id"].(string)
	confirmResult, err := service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": true,
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	confirmedTask := confirmResult["task"].(map[string]any)
	if confirmedTask["status"] != "completed" {
		t.Fatalf("expected completed status after confirmation, got %v", confirmedTask["status"])
	}

	deliveryResult, ok := confirmResult["delivery_result"].(map[string]any)
	if !ok {
		t.Fatal("expected confirmation flow to return delivery_result")
	}
	if deliveryResult["type"] != "bubble" {
		t.Fatalf("expected explain intent to deliver by bubble, got %v", deliveryResult["type"])
	}

	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected confirmed task to remain available in runtime")
	}
	if record.Status != "completed" {
		t.Fatalf("expected runtime task to be completed, got %s", record.Status)
	}
	if len(record.MemoryWritePlans) == 0 {
		t.Fatal("expected confirmation flow to attach memory write plans")
	}
	if record.DeliveryResult == nil {
		t.Fatal("expected confirmation flow to persist delivery result")
	}
}

func TestServiceSubmitInputKeepsUnknownShortTextInIntentConfirmation(t *testing.T) {
	service := newTestService()

	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_unknown_text",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "你好",
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	task := result["task"].(map[string]any)
	if task["status"] != "confirming_intent" {
		t.Fatalf("expected unknown short text to remain in confirming_intent, got %v", task["status"])
	}
	intentValue, ok := task["intent"].(map[string]any)
	if !ok || len(intentValue) != 0 {
		t.Fatalf("expected unknown short text task to keep empty intent payload, got %+v", task["intent"])
	}
	bubble := result["bubble_message"].(map[string]any)
	if bubble["text"] != "我还不确定你想如何处理这段内容，请确认目标。" {
		t.Fatalf("expected neutral confirmation prompt, got %v", bubble["text"])
	}
	if _, ok := result["delivery_result"]; ok {
		t.Fatalf("expected no delivery result before intent is confirmed, got %+v", result["delivery_result"])
	}
	if _, ok := service.runEngine.GetTask(task["task_id"].(string)); !ok {
		t.Fatal("expected task to remain available in runtime")
	}
}

func TestServiceConfirmTaskRejectsUnknownIntentWithoutCorrection(t *testing.T) {
	service := newTestService()

	startResult, err := service.SubmitInput(map[string]any{
		"session_id": "sess_unknown_confirm",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "你好",
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	confirmResult, err := service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": true,
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	task := confirmResult["task"].(map[string]any)
	if task["status"] != "confirming_intent" {
		t.Fatalf("expected task to remain in confirming_intent when no corrected intent is provided, got %v", task["status"])
	}
	bubble := confirmResult["bubble_message"].(map[string]any)
	if bubble["text"] != "请先明确告诉我你希望执行的处理方式。" {
		t.Fatalf("expected clarification bubble, got %v", bubble["text"])
	}
	if confirmResult["delivery_result"] != nil {
		t.Fatalf("expected no delivery result while intent is still missing, got %+v", confirmResult["delivery_result"])
	}
}

func TestServiceConfirmTaskCancelsUnknownIntentWhenRejected(t *testing.T) {
	service := newTestService()

	startResult, err := service.SubmitInput(map[string]any{
		"session_id": "sess_unknown_cancel",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "你好",
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	confirmResult, err := service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": false,
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	task := confirmResult["task"].(map[string]any)
	if task["status"] != "cancelled" {
		t.Fatalf("expected rejected unknown intent task to be cancelled, got %v", task["status"])
	}
}

func TestServiceConfirmTaskRewritesPlaceholderTitleAfterCorrection(t *testing.T) {
	service := newTestService()

	startResult, err := service.SubmitInput(map[string]any{
		"session_id": "sess_unknown_title",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "你好",
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	confirmResult, err := service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": true,
		"corrected_intent": map[string]any{
			"name":      "translate",
			"arguments": map[string]any{"target_language": "en"},
		},
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	task := confirmResult["task"].(map[string]any)
	if task["title"] != "翻译：你好" {
		t.Fatalf("expected corrected intent to rewrite placeholder title, got %v", task["title"])
	}
}

func TestTaskInspectorRunAggregatesRuntimeState(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecution(t, "inspector output")
	now := time.Now().UTC()
	dueToday := now.Add(15 * time.Minute)
	if dueToday.Day() != now.Day() {
		dueToday = now.Add(1 * time.Minute)
	}

	service.runEngine.ReplaceNotepadItems([]map[string]any{
		{
			"item_id":          "todo_today",
			"title":            "translate release notes",
			"bucket":           "upcoming",
			"status":           "normal",
			"type":             "todo_item",
			"due_at":           dueToday.Format(time.RFC3339),
			"agent_suggestion": "translate",
		},
	})

	todosDir := filepath.Join(workspaceRoot, "todos")
	if err := os.MkdirAll(todosDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(todosDir, "inbox.md"), []byte("- [ ] review task\n- [x] archive task\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	result, err := service.TaskInspectorRun(map[string]any{
		"reason":         "startup_scan",
		"target_sources": []any{"workspace/todos"},
	})
	if err != nil {
		t.Fatalf("TaskInspectorRun returned error: %v", err)
	}

	inspectionID, ok := result["inspection_id"].(string)
	if !ok || !strings.HasPrefix(inspectionID, "insp_") {
		t.Fatalf("expected runtime inspection_id, got %+v", result["inspection_id"])
	}

	summary, ok := result["summary"].(map[string]any)
	if !ok {
		t.Fatalf("expected summary payload, got %+v", result["summary"])
	}
	if summary["parsed_files"] != 1 {
		t.Fatalf("expected parsed_files to reflect workspace scan, got %+v", summary)
	}
	if summary["identified_items"] == nil || summary["identified_items"].(int) < 3 {
		t.Fatalf("expected identified_items to include file and notepad items, got %+v", summary)
	}
	if summary["due_today"] != 1 {
		t.Fatalf("expected due_today to reflect runtime notepad state, got %+v", summary)
	}

	suggestions, ok := result["suggestions"].([]string)
	if !ok || len(suggestions) == 0 {
		t.Fatalf("expected runtime suggestions, got %+v", result["suggestions"])
	}
}

func TestServiceNotepadListReturnsRuntimeItemsByBucket(t *testing.T) {
	service := newTestService()
	now := time.Now().UTC()
	service.runEngine.ReplaceNotepadItems([]map[string]any{
		{
			"item_id":          "todo_today",
			"title":            "translate daily notes",
			"bucket":           "upcoming",
			"status":           "normal",
			"type":             "todo_item",
			"due_at":           now.Add(2 * time.Hour).Format(time.RFC3339),
			"agent_suggestion": "translate",
		},
		{
			"item_id":          "todo_later",
			"title":            "rewrite later draft",
			"bucket":           "later",
			"status":           "normal",
			"type":             "todo_item",
			"due_at":           now.Add(48 * time.Hour).Format(time.RFC3339),
			"agent_suggestion": "rewrite",
		},
	})

	result, err := service.NotepadList(map[string]any{
		"group":  "upcoming",
		"limit":  float64(20),
		"offset": float64(0),
	})
	if err != nil {
		t.Fatalf("notepad list failed: %v", err)
	}

	items := result["items"].([]map[string]any)
	if len(items) != 1 {
		t.Fatalf("expected one upcoming notepad item, got %d", len(items))
	}
	if items[0]["item_id"] != "todo_today" {
		t.Fatalf("expected runtime list to keep todo_today, got %+v", items[0])
	}
	if items[0]["status"] != "due_today" {
		t.Fatalf("expected runtime list to normalize due_today status, got %v", items[0]["status"])
	}
}

func TestServiceNotepadConvertToTaskUsesRuntimeItemWithoutClosingTodo(t *testing.T) {
	service := newTestService()
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	service.runEngine.ReplaceNotepadItems([]map[string]any{
		{
			"item_id":          "todo_translate",
			"title":            "translate the meeting notes",
			"bucket":           "upcoming",
			"status":           "normal",
			"type":             "todo_item",
			"due_at":           now.Add(3 * time.Hour).Format(time.RFC3339),
			"agent_suggestion": "translate into English",
		},
	})

	result, err := service.NotepadConvertToTask(map[string]any{
		"item_id":   "todo_translate",
		"confirmed": true,
	})
	if err != nil {
		t.Fatalf("notepad convert failed: %v", err)
	}

	task := result["task"].(map[string]any)
	if task["title"] != "translate the meeting notes" {
		t.Fatalf("expected converted task title to come from runtime notepad item, got %v", task["title"])
	}
	if task["source_type"] != "todo" {
		t.Fatalf("expected converted task source_type todo, got %v", task["source_type"])
	}

	intentValue := task["intent"].(map[string]any)
	if intentValue["name"] != "translate" {
		t.Fatalf("expected runtime notepad conversion to infer translate intent, got %v", intentValue["name"])
	}

	taskID := task["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected converted task to remain available in runtime")
	}
	if len(record.MemoryReadPlans) == 0 {
		t.Fatal("expected converted task to attach memory read plans")
	}

	upcomingItems, total := service.runEngine.NotepadItems("upcoming", 10, 0)
	if total != 1 || len(upcomingItems) != 1 {
		t.Fatalf("expected converted todo item to stay open until task finishes, total=%d len=%d", total, len(upcomingItems))
	}
	if upcomingItems[0]["item_id"] != "todo_translate" || upcomingItems[0]["status"] == "completed" {
		t.Fatalf("expected notepad item to remain open, got %+v", upcomingItems[0])
	}
}

func TestServiceNotepadConvertToTaskRequiresConfirmedFlag(t *testing.T) {
	service := newTestService()
	service.runEngine.ReplaceNotepadItems([]map[string]any{{
		"item_id": "todo_confirm",
		"title":   "translate release draft",
		"bucket":  "upcoming",
		"status":  "normal",
		"type":    "todo_item",
	}})

	_, err := service.NotepadConvertToTask(map[string]any{
		"item_id":   "todo_confirm",
		"confirmed": false,
	})
	if err == nil {
		t.Fatal("expected convert_to_task to reject unconfirmed requests")
	}
	if err.Error() != "confirmed must be true to convert notepad item" {
		t.Fatalf("expected confirmed validation error, got %v", err)
	}

	items, total := service.runEngine.NotepadItems("upcoming", 10, 0)
	if total != 1 || len(items) != 1 {
		t.Fatalf("expected notepad item to remain untouched after rejected convert, total=%d len=%d", total, len(items))
	}
}

func TestServiceExecutionAuditIDsStayUniqueAcrossToolAndTaskRecords(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "runtime output")

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_audit",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "解释一下这段内容",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": true,
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	recordedTask, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain available")
	}
	if len(recordedTask.AuditRecords) == 0 {
		t.Fatal("expected task audit records to be appended")
	}

	firstTaskAuditID, _ := recordedTask.AuditRecords[0]["audit_id"].(string)
	if firstTaskAuditID == "" {
		t.Fatalf("expected persisted task audit id, got %+v", recordedTask.AuditRecords[0])
	}
	if firstTaskAuditID == "audit_001" {
		t.Fatalf("expected shared audit service to advance ids before task audit persistence, got %q", firstTaskAuditID)
	}
}

func TestServiceRecommendationGetUsesRuntimeTaskState(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_recommend",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "tiny note",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskTitle := startResult["task"].(map[string]any)["title"].(string)
	result, err := service.RecommendationGet(map[string]any{
		"source": "floating_ball",
		"scene":  "hover",
		"context": map[string]any{
			"page_title": "Dashboard",
			"app_name":   "desktop",
		},
	})
	if err != nil {
		t.Fatalf("recommendation get failed: %v", err)
	}

	items := result["items"].([]map[string]any)
	if len(items) == 0 {
		t.Fatal("expected runtime recommendation items")
	}
	if !strings.Contains(items[0]["text"].(string), taskTitle) {
		t.Fatalf("expected recommendation text to reference runtime task title, got %v", items[0]["text"])
	}
}

func TestServiceRecommendationFeedbackSubmitAppliesCooldown(t *testing.T) {
	service := newTestService()
	params := map[string]any{
		"source": "floating_ball",
		"scene":  "selected_text",
		"context": map[string]any{
			"page_title":     "Article",
			"app_name":       "desktop",
			"selection_text": "This paragraph should be translated before publishing externally.",
		},
	}

	first, err := service.RecommendationGet(params)
	if err != nil {
		t.Fatalf("recommendation get failed: %v", err)
	}
	items := first["items"].([]map[string]any)
	if len(items) == 0 {
		t.Fatal("expected recommendation items before feedback")
	}

	feedbackResult, err := service.RecommendationFeedbackSubmit(map[string]any{
		"recommendation_id": items[0]["recommendation_id"],
		"feedback":          "negative",
	})
	if err != nil {
		t.Fatalf("recommendation feedback submit failed: %v", err)
	}
	if feedbackResult["applied"] != true {
		t.Fatalf("expected recommendation feedback to apply, got %+v", feedbackResult)
	}

	second, err := service.RecommendationGet(params)
	if err != nil {
		t.Fatalf("second recommendation get failed: %v", err)
	}
	if second["cooldown_hit"] != true {
		t.Fatalf("expected cooldown hit after negative feedback, got %+v", second)
	}
	if len(second["items"].([]map[string]any)) != 0 {
		t.Fatalf("expected cooldown hit to suppress recommendation items, got %+v", second["items"])
	}
}

func TestServiceSubmitInputWithFilesDoesNotWaitForInput(t *testing.T) {
	service := newTestService()

	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_files",
		"source":     "floating_ball",
		"input": map[string]any{
			"files": []any{"workspace/notes.md"},
		},
		"context": map[string]any{
			"page": map[string]any{
				"title":    "Workspace",
				"app_name": "desktop",
			},
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	task := result["task"].(map[string]any)
	if task["status"] == "waiting_input" {
		t.Fatalf("expected file input to enter task flow instead of waiting_input, got %+v", task)
	}
	if task["source_type"] != "dragged_file" {
		t.Fatalf("expected file input to map to dragged_file source_type, got %v", task["source_type"])
	}
}

// TestServiceSubmitInputEmptyTextReturnsWaitingInput 验证空文本提交会进入 waiting_input。
func TestServiceSubmitInputEmptyTextReturnsWaitingInput(t *testing.T) {
	service := NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		tools.NewRegistry(),
		plugin.NewService(),
	)

	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type":       "text",
			"text":       "   ",
			"input_mode": "text",
		},
		"context": map[string]any{},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	task := result["task"].(map[string]any)
	if task["status"] != "waiting_input" {
		t.Fatalf("expected waiting_input status, got %v", task["status"])
	}
	if task["current_step"] != "collect_input" {
		t.Fatalf("expected collect_input current_step, got %v", task["current_step"])
	}
	if task["intent"] != nil {
		intentValue, ok := task["intent"].(map[string]any)
		if !ok || len(intentValue) != 0 {
			t.Fatalf("expected waiting_input task to keep empty intent, got %v", task["intent"])
		}
	}

	bubble := result["bubble_message"].(map[string]any)
	if bubble["type"] != "status" {
		t.Fatalf("expected waiting_input bubble type status, got %v", bubble["type"])
	}

	taskID := task["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected waiting_input task to exist in runtime")
	}
	if record.FinishedAt != nil {
		t.Fatal("expected waiting_input task to keep finished_at nil")
	}
	if len(record.MemoryReadPlans) != 0 || len(record.MemoryWritePlans) != 0 {
		t.Fatal("expected waiting_input task not to attach memory handoff plans")
	}
}

// TestServiceDirectStartBuildsMemoryAndDeliveryHandoffs 验证ServiceDirectStartBuildsMemoryAndDeliveryHandoffs。
func TestServiceDirectStartBuildsMemoryAndDeliveryHandoffs(t *testing.T) {
	service := NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		tools.NewRegistry(),
		plugin.NewService(),
	)

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "直接总结这段文字",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to exist in runtime")
	}
	if len(record.MemoryReadPlans) == 0 || len(record.MemoryWritePlans) == 0 {
		t.Fatal("expected memory handoff plans to be attached")
	}
	if record.StorageWritePlan == nil || len(record.ArtifactPlans) == 0 {
		t.Fatal("expected delivery handoff plans to be attached")
	}
	if record.FinishedAt == nil {
		t.Fatal("expected direct completion flow to set finished_at only after completion")
	}

	notifications, ok := service.runEngine.PendingNotifications(taskID)
	if !ok {
		t.Fatal("expected notifications to be available")
	}
	hasDeliveryReady := false
	for _, notification := range notifications {
		if notification.Method == "delivery.ready" {
			hasDeliveryReady = true
			break
		}
	}
	if !hasDeliveryReady {
		t.Fatal("expected delivery.ready notification to be queued")
	}
}

// TestServiceStartTaskWaitingAuthDoesNotSetFinishedAt 验证等待授权前不会提前写入 finished_at。
func TestServiceStartTaskRespectsPreferredDelivery(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "direct summarize with bubble delivery",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
		"delivery": map[string]any{
			"preferred": "bubble",
			"fallback":  "workspace_document",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	deliveryResult, ok := startResult["delivery_result"].(map[string]any)
	if !ok {
		t.Fatal("expected direct start to return delivery_result")
	}
	if deliveryResult["type"] != "bubble" {
		t.Fatalf("expected preferred bubble delivery, got %v", deliveryResult["type"])
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected direct start task to exist in runtime")
	}
	if record.PreferredDelivery != "bubble" {
		t.Fatalf("expected runtime task to persist preferred delivery, got %q", record.PreferredDelivery)
	}
	if record.FallbackDelivery != "workspace_document" {
		t.Fatalf("expected runtime task to persist fallback delivery, got %q", record.FallbackDelivery)
	}
	if record.StorageWritePlan != nil || len(record.ArtifactPlans) != 0 {
		t.Fatal("expected bubble delivery not to create document persistence plans")
	}
}

func TestServiceSubmitInputRespectsPreferredDelivery(t *testing.T) {
	service := newTestService()

	result, err := service.SubmitInput(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "translate this line",
		},
		"options": map[string]any{
			"confirm_required":   false,
			"preferred_delivery": "workspace_document",
		},
	})
	if err != nil {
		t.Fatalf("submit input failed: %v", err)
	}

	deliveryResult, ok := result["delivery_result"].(map[string]any)
	if !ok {
		t.Fatal("expected submit input to return delivery_result")
	}
	if deliveryResult["type"] != "workspace_document" {
		t.Fatalf("expected preferred workspace_document delivery, got %v", deliveryResult["type"])
	}

	payload, ok := deliveryResult["payload"].(map[string]any)
	if !ok {
		t.Fatal("expected delivery_result payload")
	}
	if payload["path"] == nil {
		t.Fatal("expected workspace_document delivery to include payload path")
	}

	taskID := result["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected submit input task to exist in runtime")
	}
	if record.PreferredDelivery != "workspace_document" {
		t.Fatalf("expected runtime task to persist preferred delivery, got %q", record.PreferredDelivery)
	}
}

func TestServiceConfirmTaskRespectsStoredPreferredDelivery(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "selected text for confirmation flow",
		},
		"delivery": map[string]any{
			"preferred": "workspace_document",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	confirmResult, err := service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": true,
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	deliveryResult, ok := confirmResult["delivery_result"].(map[string]any)
	if !ok {
		t.Fatal("expected confirm flow to return delivery_result")
	}
	if deliveryResult["type"] != "workspace_document" {
		t.Fatalf("expected stored preferred workspace_document delivery, got %v", deliveryResult["type"])
	}

	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected confirmed task to exist in runtime")
	}
	if record.PreferredDelivery != "workspace_document" {
		t.Fatalf("expected runtime task to keep preferred delivery, got %q", record.PreferredDelivery)
	}
	if record.DeliveryResult["type"] != "workspace_document" {
		t.Fatalf("expected runtime delivery result to use workspace_document, got %v", record.DeliveryResult["type"])
	}
}

func TestServiceStartTaskWaitingAuthDoesNotSetFinishedAt(t *testing.T) {
	service := NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		tools.NewRegistry(),
		plugin.NewService(),
	)

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "file_drop",
		"input": map[string]any{
			"type":  "file",
			"files": []any{"workspace/input.md"},
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
				"target_path":           "workspace_document",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	startedTask := startResult["task"].(map[string]any)
	if startedTask["status"] != "waiting_auth" {
		t.Fatalf("expected waiting_auth status, got %v", startedTask["status"])
	}
	if startedTask["finished_at"] != nil {
		t.Fatalf("expected waiting_auth task to keep finished_at nil, got %v", startedTask["finished_at"])
	}

	taskID := startedTask["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime")
	}
	if record.FinishedAt != nil {
		t.Fatal("expected runtime waiting_auth task to keep finished_at nil")
	}
}

// TestServiceConfirmCanEnterWaitingAuth 验证ServiceConfirmCanEnterWaitingAuth。
func TestServiceConfirmCanEnterWaitingAuth(t *testing.T) {
	service := NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		tools.NewRegistry(),
		plugin.NewService(),
	)

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "这里是一段需要确认处理方式的内容",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	confirmResult, err := service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": true,
		"corrected_intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
				"target_path":           "workspace_document",
			},
		},
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	confirmedTask := confirmResult["task"].(map[string]any)
	if confirmedTask["status"] != "waiting_auth" {
		t.Fatalf("expected waiting_auth status, got %v", confirmedTask["status"])
	}
	if confirmedTask["intent"].(map[string]any)["name"] != "write_file" {
		t.Fatalf("expected corrected intent to be persisted before waiting auth, got %v", confirmedTask["intent"])
	}

	notifications, ok := service.runEngine.PendingNotifications(taskID)
	if !ok {
		t.Fatal("expected notifications to exist for waiting task")
	}
	hasApprovalPending := false
	for _, notification := range notifications {
		if notification.Method == "approval.pending" {
			hasApprovalPending = true
			break
		}
	}
	if !hasApprovalPending {
		t.Fatal("expected approval.pending notification to be queued")
	}

	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime after entering waiting_auth")
	}
	if record.Intent["name"] != "write_file" {
		t.Fatalf("expected runtime task intent to be updated before waiting auth, got %v", record.Intent)
	}
}

// TestServiceSecurityRespondAllowOnceResumesAndCompletes 验证授权通过后任务会继续执行并完成交付。
func TestServiceSecurityRespondAllowOnceResumesAndCompletes(t *testing.T) {
	service := NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		tools.NewRegistry(),
		plugin.NewService(),
	)

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "需要授权后继续执行的内容",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": true,
		"corrected_intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
				"target_path":           "workspace_document",
			},
		},
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	respondResult, err := service.SecurityRespond(map[string]any{
		"task_id":       taskID,
		"approval_id":   "appr_001",
		"decision":      "allow_once",
		"remember_rule": false,
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}

	responseTask := respondResult["task"].(map[string]any)
	if responseTask["status"] != "completed" {
		t.Fatalf("expected response task to reflect finalized completion, got %v", responseTask["status"])
	}
	responseBubble := respondResult["bubble_message"].(map[string]any)
	if responseBubble["type"] != "result" {
		t.Fatalf("expected security respond to return the final result bubble, got %v", responseBubble["type"])
	}
	impactScope := respondResult["impact_scope"].(map[string]any)
	files := impactScope["files"].([]string)
	if len(files) != 1 || files[0] != "workspace/文件写入结果.md" {
		t.Fatalf("expected impact scope files to stay within workspace-relative paths, got %v", files)
	}

	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime after authorization")
	}
	if record.Status != "completed" {
		t.Fatalf("expected runtime task to complete after resume, got %s", record.Status)
	}
	if record.Authorization == nil {
		t.Fatal("expected authorization record to be stored on runtime task")
	}

	notifications, ok := service.runEngine.PendingNotifications(taskID)
	if !ok {
		t.Fatal("expected notifications to remain available after authorization")
	}
	hasProcessingUpdate := false
	hasDeliveryReady := false
	for _, notification := range notifications {
		if notification.Method == "task.updated" {
			if notification.Params["status"] == "processing" {
				hasProcessingUpdate = true
			}
		}
		if notification.Method == "delivery.ready" {
			hasDeliveryReady = true
		}
	}
	if !hasProcessingUpdate || !hasDeliveryReady {
		t.Fatal("expected resumed processing and delivery notifications to be queued")
	}
	if record.PendingExecution != nil {
		t.Fatal("expected pending execution plan to be cleared after successful authorization")
	}
}

// TestServiceSecurityRespondDenyOnceCancelsTask 验证拒绝授权后任务会结束。
func TestServiceSecurityRespondRespectsFallbackDelivery(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "authorization flow with delivery fallback",
		},
		"delivery": map[string]any{
			"preferred": "unsupported_delivery",
			"fallback":  "bubble",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": true,
		"corrected_intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style":                 "key_points",
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	_, err = service.SecurityRespond(map[string]any{
		"task_id":       taskID,
		"approval_id":   "appr_001",
		"decision":      "allow_once",
		"remember_rule": false,
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}

	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to exist in runtime after authorization")
	}
	if record.DeliveryResult["type"] != "bubble" {
		t.Fatalf("expected fallback bubble delivery after authorization, got %v", record.DeliveryResult["type"])
	}
	if record.StorageWritePlan != nil || len(record.ArtifactPlans) != 0 {
		t.Fatal("expected bubble fallback delivery not to create document persistence plans")
	}
}

func TestServiceSecurityRespondDenyOnceCancelsTask(t *testing.T) {
	service := NewService(
		contextsvc.NewService(),
		intent.NewService(),
		runengine.NewEngine(),
		delivery.NewService(),
		memory.NewService(),
		risk.NewService(),
		model.NewService(modelConfig()),
		tools.NewRegistry(),
		plugin.NewService(),
	)

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "需要授权后继续执行的内容",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.ConfirmTask(map[string]any{
		"task_id":   taskID,
		"confirmed": true,
		"corrected_intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
				"target_path":           "workspace_document",
			},
		},
	})
	if err != nil {
		t.Fatalf("confirm task failed: %v", err)
	}

	respondResult, err := service.SecurityRespond(map[string]any{
		"task_id":       taskID,
		"approval_id":   "appr_001",
		"decision":      "deny_once",
		"remember_rule": false,
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}

	responseTask := respondResult["task"].(map[string]any)
	if responseTask["status"] != "cancelled" {
		t.Fatalf("expected cancelled task in deny response, got %v", responseTask["status"])
	}

	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime after denial")
	}
	if record.Status != "cancelled" {
		t.Fatalf("expected runtime task to be cancelled after denial, got %s", record.Status)
	}
	if record.Authorization == nil {
		t.Fatal("expected denial decision to be stored as authorization record")
	}
	if record.PendingExecution != nil {
		t.Fatal("expected pending execution plan to be cleared after denial")
	}
}

// modelConfig 处理当前模块的相关逻辑。
func TestServiceTaskListSupportsSortParams(t *testing.T) {
	service := newTestService()

	firstResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "first finished task",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start first task failed: %v", err)
	}
	time.Sleep(5 * time.Millisecond)

	secondResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "second finished task",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start second task failed: %v", err)
	}

	listResult, err := service.TaskList(map[string]any{
		"group":      "finished",
		"limit":      float64(10),
		"offset":     float64(0),
		"sort_by":    "started_at",
		"sort_order": "asc",
	})
	if err != nil {
		t.Fatalf("task list failed: %v", err)
	}

	items := listResult["items"].([]map[string]any)
	if len(items) < 2 {
		t.Fatalf("expected at least two finished tasks, got %d", len(items))
	}

	firstTaskID := firstResult["task"].(map[string]any)["task_id"].(string)
	secondTaskID := secondResult["task"].(map[string]any)["task_id"].(string)
	if items[0]["task_id"] != firstTaskID || items[1]["task_id"] != secondTaskID {
		t.Fatalf("expected started_at asc order %s -> %s, got %v -> %v", firstTaskID, secondTaskID, items[0]["task_id"], items[1]["task_id"])
	}
}

func TestServiceTaskListFallsBackToStoredTaskRuns(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored task list")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_stored_001",
		SessionID:   "sess_stored",
		RunID:       "run_stored_001",
		Title:       "stored finished task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 9, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 9, 5, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 14, 9, 6, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("save task run failed: %v", err)
	}

	listResult, err := service.TaskList(map[string]any{
		"group":      "finished",
		"limit":      float64(10),
		"offset":     float64(0),
		"sort_by":    "updated_at",
		"sort_order": "desc",
	})
	if err != nil {
		t.Fatalf("task list failed: %v", err)
	}

	items := listResult["items"].([]map[string]any)
	if len(items) != 1 || items[0]["task_id"] != "task_stored_001" {
		t.Fatalf("expected storage-backed task list item, got %+v", items)
	}
}

func TestServiceTaskListDoesNotFallbackWhenOffsetExceedsRuntimePage(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "runtime paging")

	_, err := service.StartTask(map[string]any{
		"session_id": "sess_page",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "runtime finished task",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start runtime task failed: %v", err)
	}
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	err = service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_stored_extra",
		SessionID:   "sess_page",
		RunID:       "run_stored_extra",
		Title:       "stored finished task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 14, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 14, 5, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 14, 14, 6, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("save task run failed: %v", err)
	}

	listResult, err := service.TaskList(map[string]any{
		"group":      "finished",
		"limit":      float64(10),
		"offset":     float64(100),
		"sort_by":    "updated_at",
		"sort_order": "desc",
	})
	if err != nil {
		t.Fatalf("task list failed: %v", err)
	}

	items := listResult["items"].([]map[string]any)
	if len(items) != 0 {
		t.Fatalf("expected empty page beyond runtime total, got %+v", items)
	}
	page := listResult["page"].(map[string]any)
	if page["total"] != 1 {
		t.Fatalf("expected runtime total to stay unchanged, got %+v", page)
	}
}

func TestServiceTaskListFallbackMatchesRuntimeUnknownGroupSemantics(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored unknown group")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_stored_unfinished",
		SessionID:   "sess_group",
		RunID:       "run_stored_unfinished",
		Title:       "stored unfinished task",
		SourceType:  "hover_input",
		Status:      "processing",
		CurrentStep: "generate_output",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 15, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 15, 5, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("save unfinished task run failed: %v", err)
	}
	err = service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_stored_finished",
		SessionID:   "sess_group",
		RunID:       "run_stored_finished",
		Title:       "stored finished task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 15, 10, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 15, 15, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 14, 15, 16, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("save finished task run failed: %v", err)
	}

	listResult, err := service.TaskList(map[string]any{
		"group":      "unknown_group",
		"limit":      float64(10),
		"offset":     float64(0),
		"sort_by":    "updated_at",
		"sort_order": "desc",
	})
	if err != nil {
		t.Fatalf("task list failed: %v", err)
	}

	items := listResult["items"].([]map[string]any)
	if len(items) != 1 || items[0]["task_id"] != "task_stored_unfinished" {
		t.Fatalf("expected unknown group fallback to match runtime unfinished semantics, got %+v", items)
	}
}

func TestServiceTaskListFallbackMatchesRuntimeSortTieBreaker(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored sort tie")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	finishedAt := time.Date(2026, 4, 14, 16, 0, 0, 0, time.UTC)
	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_sort_older_update",
		SessionID:   "sess_sort",
		RunID:       "run_sort_old",
		Title:       "older update task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 15, 30, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 15, 40, 0, 0, time.UTC),
		FinishedAt:  timePointer(finishedAt),
	})
	if err != nil {
		t.Fatalf("save first task run failed: %v", err)
	}
	err = service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_sort_newer_update",
		SessionID:   "sess_sort",
		RunID:       "run_sort_new",
		Title:       "newer update task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 15, 35, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 15, 50, 0, 0, time.UTC),
		FinishedAt:  timePointer(finishedAt),
	})
	if err != nil {
		t.Fatalf("save second task run failed: %v", err)
	}

	listResult, err := service.TaskList(map[string]any{
		"group":      "finished",
		"limit":      float64(10),
		"offset":     float64(0),
		"sort_by":    "finished_at",
		"sort_order": "desc",
	})
	if err != nil {
		t.Fatalf("task list failed: %v", err)
	}

	items := listResult["items"].([]map[string]any)
	if len(items) < 2 || items[0]["task_id"] != "task_sort_newer_update" || items[1]["task_id"] != "task_sort_older_update" {
		t.Fatalf("expected fallback sort tie-breaker to prefer newer updated_at, got %+v", items)
	}
}

func TestServiceDashboardOverviewUsesRuntimeAggregation(t *testing.T) {
	service := newTestService()

	completedResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "completed task for dashboard overview",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start completed task failed: %v", err)
	}

	waitingResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "waiting authorization task for dashboard overview",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start waiting auth task failed: %v", err)
	}

	result, err := service.DashboardOverviewGet(map[string]any{})
	if err != nil {
		t.Fatalf("dashboard overview failed: %v", err)
	}

	overview := result["overview"].(map[string]any)
	focusSummary := overview["focus_summary"].(map[string]any)
	waitingTaskID := waitingResult["task"].(map[string]any)["task_id"].(string)
	if focusSummary["task_id"] != waitingTaskID {
		t.Fatalf("expected focus summary to point at latest unfinished task %s, got %v", waitingTaskID, focusSummary["task_id"])
	}
	if focusSummary["status"] != "waiting_auth" {
		t.Fatalf("expected focus summary status waiting_auth, got %v", focusSummary["status"])
	}

	trustSummary := overview["trust_summary"].(map[string]any)
	if trustSummary["pending_authorizations"] != 1 {
		t.Fatalf("expected one pending authorization, got %v", trustSummary["pending_authorizations"])
	}
	if trustSummary["has_restore_point"] != true {
		t.Fatalf("expected completed task to provide restore point, got %v", trustSummary["has_restore_point"])
	}
	if trustSummary["workspace_path"] != "workspace" {
		t.Fatalf("expected workspace-relative path in trust summary, got %v", trustSummary["workspace_path"])
	}

	quickActions := overview["quick_actions"].([]string)
	if len(quickActions) == 0 || quickActions[0] != "处理待授权操作" {
		t.Fatalf("expected dashboard quick actions to prioritize authorization handling, got %v", quickActions)
	}

	highValueSignals := overview["high_value_signal"].([]string)
	if len(highValueSignals) == 0 {
		t.Fatal("expected runtime-derived high value signals")
	}

	completedTaskID := completedResult["task"].(map[string]any)["task_id"].(string)
	if completedTaskID == waitingTaskID {
		t.Fatal("expected completed and waiting tasks to be distinct runtime records")
	}
}

func TestServiceDashboardOverviewRespectsIncludeFilter(t *testing.T) {
	service := newTestService()

	result, err := service.DashboardOverviewGet(map[string]any{
		"include": []any{"focus_summary", "quick_actions"},
	})
	if err != nil {
		t.Fatalf("dashboard overview failed: %v", err)
	}

	overview := result["overview"].(map[string]any)
	if _, ok := overview["focus_summary"]; !ok {
		t.Fatal("expected focus_summary field to be present")
	}
	if _, ok := overview["quick_actions"]; !ok {
		t.Fatal("expected quick_actions field to be present")
	}
	if overview["trust_summary"] != nil {
		t.Fatalf("expected trust_summary placeholder to be nil when not requested, got %+v", overview["trust_summary"])
	}
	globalState, ok := overview["global_state"].(map[string]any)
	if !ok || len(globalState) != 0 {
		t.Fatalf("expected global_state placeholder to be empty map when not requested, got %+v", overview["global_state"])
	}
	highValueSignal, ok := overview["high_value_signal"].([]string)
	if !ok || len(highValueSignal) != 0 {
		t.Fatalf("expected high_value_signal placeholder to be empty slice when not requested, got %+v", overview["high_value_signal"])
	}
}

func TestServiceDashboardOverviewFocusModeNarrowsSecondaryData(t *testing.T) {
	service := newTestService()

	_, err := service.StartTask(map[string]any{
		"session_id": "sess_focus",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "completed task for focus mode",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start completed task failed: %v", err)
	}

	_, err = service.StartTask(map[string]any{
		"session_id": "sess_focus",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "waiting authorization task for focus mode",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start waiting auth task failed: %v", err)
	}

	result, err := service.DashboardOverviewGet(map[string]any{
		"focus_mode": true,
	})
	if err != nil {
		t.Fatalf("dashboard overview failed: %v", err)
	}

	overview := result["overview"].(map[string]any)
	quickActions := overview["quick_actions"].([]string)
	for _, action := range quickActions {
		if action == "查看最近结果" {
			t.Fatalf("expected focus mode to drop secondary quick action, got %v", quickActions)
		}
	}
	highValueSignals := overview["high_value_signal"].([]string)
	if len(highValueSignals) > 2 {
		t.Fatalf("expected focus mode to narrow signal list, got %v", highValueSignals)
	}
}

func TestServiceMirrorOverviewUsesRuntimeMirrorReferences(t *testing.T) {
	service := newTestService()

	_, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "mirror overview should reuse runtime memory plans",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	result, err := service.MirrorOverviewGet(map[string]any{})
	if err != nil {
		t.Fatalf("mirror overview failed: %v", err)
	}

	memoryReferences := result["memory_references"].([]map[string]any)
	if len(memoryReferences) == 0 {
		t.Fatal("expected runtime-derived mirror references")
	}
	if !strings.HasPrefix(memoryReferences[0]["memory_id"].(string), "mem_") {
		t.Fatalf("expected memory reference to come from runtime plans, got %v", memoryReferences[0]["memory_id"])
	}

	historySummary := result["history_summary"].([]string)
	if len(historySummary) == 0 {
		t.Fatal("expected history summary to be derived from runtime tasks")
	}

	profile := result["profile"].(map[string]any)
	if profile["preferred_output"] != "workspace_document" {
		t.Fatalf("expected profile to infer workspace_document preference, got %v", profile["preferred_output"])
	}
}

func TestServiceMirrorOverviewFallsBackToStoredFinishedTasks(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored mirror overview")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_mirror_stored",
		SessionID:   "sess_stored",
		RunID:       "run_mirror_stored",
		Title:       "stored mirror task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 11, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 11, 5, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 14, 11, 6, 0, 0, time.UTC)),
		MirrorReferences: []map[string]any{{
			"memory_id": "mem_stored_001",
			"reason":    "stored memory hit",
			"summary":   "stored mirror reference",
		}},
		DeliveryResult: map[string]any{
			"type": "workspace_document",
			"payload": map[string]any{
				"path": "workspace/stored-result.md",
			},
		},
	})
	if err != nil {
		t.Fatalf("save task run failed: %v", err)
	}

	result, err := service.MirrorOverviewGet(map[string]any{})
	if err != nil {
		t.Fatalf("mirror overview failed: %v", err)
	}

	memoryReferences := result["memory_references"].([]map[string]any)
	if len(memoryReferences) != 1 || memoryReferences[0]["memory_id"] != "mem_stored_001" {
		t.Fatalf("expected storage-backed mirror references, got %+v", memoryReferences)
	}
	historySummary := result["history_summary"].([]string)
	if len(historySummary) == 0 {
		t.Fatal("expected storage-backed mirror history summary")
	}
	profile := result["profile"].(map[string]any)
	if profile["preferred_output"] != "workspace_document" {
		t.Fatalf("expected storage-backed mirror profile to infer workspace_document, got %+v", profile)
	}
}

func TestServiceSecuritySummaryUsesRuntimeTaskState(t *testing.T) {
	service := newTestService()

	_, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "completed task for security summary restore point",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start completed task failed: %v", err)
	}

	waitingResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "security summary intercepted task",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start intercepted task failed: %v", err)
	}

	waitingTaskID := waitingResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.SecurityRespond(map[string]any{
		"task_id":       waitingTaskID,
		"approval_id":   "appr_001",
		"decision":      "deny_once",
		"remember_rule": false,
	})
	if err != nil {
		t.Fatalf("security respond failed: %v", err)
	}

	result, err := service.SecuritySummaryGet()
	if err != nil {
		t.Fatalf("security summary failed: %v", err)
	}

	summary := result["summary"].(map[string]any)
	if summary["security_status"] != "intercepted" {
		t.Fatalf("expected intercepted security status from runtime task state, got %v", summary["security_status"])
	}
	if summary["pending_authorizations"] != 0 {
		t.Fatalf("expected no pending authorizations after denial, got %v", summary["pending_authorizations"])
	}
	if summary["latest_restore_point"] == nil {
		t.Fatal("expected latest restore point to come from completed runtime task")
	}

	tokenCostSummary := summary["token_cost_summary"].(map[string]any)
	if tokenCostSummary["budget_auto_downgrade"] != true {
		t.Fatalf("expected token summary to reflect settings snapshot, got %v", tokenCostSummary["budget_auto_downgrade"])
	}
}

func TestServiceSecuritySummaryIncludesRuntimeTokenUsage(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed token summary")

	result, err := service.StartTask(map[string]any{
		"session_id": "sess_exec",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "collect runtime token summary",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := result["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected completed runtime task to remain available")
	}
	if len(record.AuditRecords) == 0 {
		t.Fatal("expected executor-backed task to carry audit records")
	}

	securityResult, err := service.SecuritySummaryGet()
	if err != nil {
		t.Fatalf("security summary failed: %v", err)
	}

	tokenCostSummary := securityResult["summary"].(map[string]any)["token_cost_summary"].(map[string]any)
	if tokenCostSummary["current_task_tokens"] != 36 {
		t.Fatalf("expected current_task_tokens to reflect runtime usage, got %+v", tokenCostSummary)
	}
	if tokenCostSummary["today_tokens"] != 36 {
		t.Fatalf("expected today_tokens to reflect runtime usage, got %+v", tokenCostSummary)
	}
	if tokenCostSummary["budget_auto_downgrade"] != true {
		t.Fatalf("expected budget_auto_downgrade to remain true, got %+v", tokenCostSummary)
	}
}

func TestServiceSecuritySummaryFallsBackToStoredRecoveryPoint(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed summary")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	err := service.storage.RecoveryPointWriter().WriteRecoveryPoint(context.Background(), checkpoint.RecoveryPoint{
		RecoveryPointID: "rp_001",
		TaskID:          "task_external",
		Summary:         "stored recovery point",
		CreatedAt:       "2026-04-08T10:00:00Z",
		Objects:         []string{"workspace/result.md"},
	})
	if err != nil {
		t.Fatalf("write recovery point failed: %v", err)
	}

	result, err := service.SecuritySummaryGet()
	if err != nil {
		t.Fatalf("security summary failed: %v", err)
	}

	summary := result["summary"].(map[string]any)
	latestRestorePoint := summary["latest_restore_point"].(map[string]any)
	if latestRestorePoint["recovery_point_id"] != "rp_001" {
		t.Fatalf("expected storage-backed recovery point, got %+v", latestRestorePoint)
	}
}

func TestServiceSecuritySummaryFallsBackToStoredTaskRuns(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored security summary")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_security_finished",
		SessionID:   "sess_stored",
		RunID:       "run_security_finished",
		Title:       "stored security task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "yellow",
		StartedAt:   time.Date(2026, 4, 14, 13, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 13, 5, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 14, 13, 6, 0, 0, time.UTC)),
		SecuritySummary: map[string]any{
			"security_status": "recoverable",
			"latest_restore_point": map[string]any{
				"recovery_point_id": "rp_security_001",
				"task_id":           "task_security_finished",
				"summary":           "stored security recovery point",
				"created_at":        "2026-04-14T13:06:00Z",
				"objects":           []string{"workspace/security.md"},
			},
		},
		TokenUsage: map[string]any{
			"total_tokens":   88,
			"estimated_cost": 0.42,
		},
	})
	if err != nil {
		t.Fatalf("save task run failed: %v", err)
	}

	result, err := service.SecuritySummaryGet()
	if err != nil {
		t.Fatalf("security summary failed: %v", err)
	}

	summary := result["summary"].(map[string]any)
	if summary["security_status"] != "recoverable" {
		t.Fatalf("expected storage-backed security status, got %+v", summary)
	}
	latestRestorePoint := summary["latest_restore_point"].(map[string]any)
	if latestRestorePoint["recovery_point_id"] != "rp_security_001" {
		t.Fatalf("expected storage-backed recovery point from task run, got %+v", latestRestorePoint)
	}
	tokenCostSummary := summary["token_cost_summary"].(map[string]any)
	if tokenCostSummary["current_task_tokens"] != 88 {
		t.Fatalf("expected storage-backed token usage, got %+v", tokenCostSummary)
	}
}

func TestServiceSecuritySummaryCountsStoredPendingAuthorizations(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored waiting auth")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_waiting_auth_stored",
		SessionID:   "sess_waiting",
		RunID:       "run_waiting_auth_stored",
		Title:       "stored waiting auth task",
		SourceType:  "hover_input",
		Status:      "waiting_auth",
		CurrentStep: "waiting_authorization",
		RiskLevel:   "yellow",
		StartedAt:   time.Date(2026, 4, 14, 17, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 17, 5, 0, 0, time.UTC),
		ApprovalRequest: map[string]any{
			"approval_id": "appr_waiting_001",
			"task_id":     "task_waiting_auth_stored",
			"risk_level":  "yellow",
		},
		SecuritySummary: map[string]any{
			"security_status": "pending_confirmation",
		},
	})
	if err != nil {
		t.Fatalf("save waiting auth task run failed: %v", err)
	}

	result, err := service.SecuritySummaryGet()
	if err != nil {
		t.Fatalf("security summary failed: %v", err)
	}

	summary := result["summary"].(map[string]any)
	if summary["pending_authorizations"] != 1 {
		t.Fatalf("expected stored waiting_auth task to count as pending authorization, got %+v", summary)
	}
}

func TestServiceDashboardModuleHighlightsIncludeAuditTrail(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "dashboard audit trail")

	_, err := service.StartTask(map[string]any{
		"session_id": "sess_exec",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "build dashboard audit trail",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	moduleResult, err := service.DashboardModuleGet(map[string]any{
		"module": "security",
		"tab":    "audit",
	})
	if err != nil {
		t.Fatalf("dashboard module get failed: %v", err)
	}

	highlights := moduleResult["highlights"].([]string)
	foundAuditHighlight := false
	for _, highlight := range highlights {
		if strings.Contains(highlight, "generate_text") || strings.Contains(highlight, "publish_result") {
			foundAuditHighlight = true
			break
		}
	}
	if !foundAuditHighlight {
		t.Fatalf("expected dashboard highlights to expose runtime audit trail, got %+v", highlights)
	}
}

func TestServiceDashboardModuleFallsBackToStoredTaskRuns(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored dashboard module")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_dashboard_finished",
		SessionID:   "sess_stored",
		RunID:       "run_dashboard_finished",
		Title:       "stored dashboard task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 12, 5, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 14, 12, 6, 0, 0, time.UTC)),
		DeliveryResult: map[string]any{
			"type": "workspace_document",
			"payload": map[string]any{
				"path": "workspace/dashboard.md",
			},
		},
		AuditRecords: []map[string]any{{
			"audit_id":   "audit_dashboard_001",
			"task_id":    "task_dashboard_finished",
			"action":     "write_file",
			"summary":    "stored dashboard audit",
			"created_at": "2026-04-14T12:06:00Z",
			"result":     "success",
			"target":     "workspace/dashboard.md",
		}},
	})
	if err != nil {
		t.Fatalf("save finished task run failed: %v", err)
	}

	moduleResult, err := service.DashboardModuleGet(map[string]any{
		"module": "security",
		"tab":    "audit",
	})
	if err != nil {
		t.Fatalf("dashboard module get failed: %v", err)
	}

	summary := moduleResult["summary"].(map[string]any)
	if summary["completed_tasks"] != 1 {
		t.Fatalf("expected storage-backed completed task count, got %+v", summary)
	}
	if summary["generated_outputs"] != 1 {
		t.Fatalf("expected storage-backed generated output count, got %+v", summary)
	}
	highlights := moduleResult["highlights"].([]string)
	if len(highlights) == 0 {
		t.Fatal("expected storage-backed dashboard highlights")
	}
}

func TestServiceSecurityAuditListFallsBackToStoredAuditRecords(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed summary")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}
	err := service.storage.AuditWriter().WriteAuditRecord(context.Background(), audit.Record{
		AuditID:   "audit_001",
		TaskID:    "task_external",
		Type:      "file",
		Action:    "write_file",
		Summary:   "stored audit record",
		Target:    "workspace/result.md",
		Result:    "success",
		CreatedAt: "2026-04-08T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("write audit record failed: %v", err)
	}

	result, err := service.SecurityAuditList(map[string]any{"task_id": "task_external", "limit": 20, "offset": 0})
	if err != nil {
		t.Fatalf("security audit list failed: %v", err)
	}

	items := result["items"].([]map[string]any)
	if len(items) != 1 || items[0]["audit_id"] != "audit_001" {
		t.Fatalf("expected storage-backed audit record, got %+v", items)
	}
}

func TestServiceSecurityAuditListRequiresTaskID(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "executor-backed summary")
	_, err := service.SecurityAuditList(map[string]any{"limit": 20, "offset": 0})
	if err == nil || err.Error() != "task_id is required" {
		t.Fatalf("expected task_id required error, got %v", err)
	}
}

func TestServiceTaskDetailGetPreservesStableContractShape(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "task detail delivery")

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_detail",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "collect detail view payload",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": taskID})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}

	if _, ok := detailResult["delivery_result"]; ok {
		t.Fatalf("expected task detail response not to expose undeclared delivery_result field, got %+v", detailResult["delivery_result"])
	}
	if _, ok := detailResult["audit_records"]; ok {
		t.Fatalf("expected task detail response not to expose undeclared audit_records field, got %+v", detailResult["audit_records"])
	}
	if detailResult["task"].(map[string]any)["task_id"] != taskID {
		t.Fatalf("expected task detail task_id to match request, got %+v", detailResult["task"])
	}
}

func TestServiceTaskDetailGetFallsBackToStoredTaskRun(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "stored task detail")
	if service.storage == nil {
		t.Fatal("expected storage service to be wired")
	}

	err := service.storage.TaskRunStore().SaveTaskRun(context.Background(), storage.TaskRunRecord{
		TaskID:      "task_stored_detail",
		SessionID:   "sess_stored",
		RunID:       "run_stored_detail",
		Title:       "stored detail task",
		SourceType:  "hover_input",
		Status:      "completed",
		CurrentStep: "deliver_result",
		RiskLevel:   "green",
		StartedAt:   time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 4, 14, 10, 5, 0, 0, time.UTC),
		FinishedAt:  timePointer(time.Date(2026, 4, 14, 10, 6, 0, 0, time.UTC)),
		Timeline: []storage.TaskStepSnapshot{{
			StepID:        "step_deliver_result",
			TaskID:        "task_stored_detail",
			Name:          "deliver_result",
			Status:        "completed",
			OrderIndex:    1,
			InputSummary:  "stored input",
			OutputSummary: "stored output",
		}},
	})
	if err != nil {
		t.Fatalf("save task run failed: %v", err)
	}

	detailResult, err := service.TaskDetailGet(map[string]any{"task_id": "task_stored_detail"})
	if err != nil {
		t.Fatalf("task detail get failed: %v", err)
	}

	task := detailResult["task"].(map[string]any)
	if task["task_id"] != "task_stored_detail" || task["title"] != "stored detail task" {
		t.Fatalf("expected storage-backed task detail, got %+v", task)
	}
	timeline := detailResult["timeline"].([]map[string]any)
	if len(timeline) != 1 || timeline[0]["name"] != "deliver_result" {
		t.Fatalf("expected storage-backed timeline, got %+v", timeline)
	}
}

func TestServiceTaskControlRejectsInvalidStatusTransition(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "text_selected_click",
		"input": map[string]any{
			"type": "text_selection",
			"text": "this task still requires confirmation",
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.TaskControl(map[string]any{
		"task_id": taskID,
		"action":  "pause",
	})
	if !errors.Is(err, ErrTaskStatusInvalid) {
		t.Fatalf("expected pause from confirming_intent to return ErrTaskStatusInvalid, got %v", err)
	}
}

func TestServiceTaskControlRequiresTaskID(t *testing.T) {
	service := newTestService()

	_, err := service.TaskControl(map[string]any{
		"action": "pause",
	})
	if err == nil || err.Error() != "task_id is required" {
		t.Fatalf("expected task_id required error, got %v", err)
	}
}

func TestServiceTaskControlRequiresAction(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "task control needs action",
		},
		"intent": map[string]any{
			"name": "write_file",
			"arguments": map[string]any{
				"require_authorization": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.TaskControl(map[string]any{
		"task_id": taskID,
	})
	if err == nil || err.Error() != "action is required" {
		t.Fatalf("expected action required error, got %v", err)
	}
}

func TestServiceTaskControlRejectsFinishedTaskOperations(t *testing.T) {
	service := newTestService()

	startResult, err := service.StartTask(map[string]any{
		"session_id": "sess_demo",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "completed task for task control error mapping",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	taskID := startResult["task"].(map[string]any)["task_id"].(string)
	_, err = service.TaskControl(map[string]any{
		"task_id": taskID,
		"action":  "cancel",
	})
	if !errors.Is(err, ErrTaskAlreadyFinished) {
		t.Fatalf("expected cancel on completed task to return ErrTaskAlreadyFinished, got %v", err)
	}
}

func TestServiceStartTaskWithExecutorWritesWorkspaceDocument(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecution(t, "第一点\n第二点\n第三点")

	result, err := service.StartTask(map[string]any{
		"session_id": "sess_exec",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请整理成文档",
		},
		"intent": map[string]any{
			"name": "summarize",
			"arguments": map[string]any{
				"style": "key_points",
			},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	deliveryResult := result["delivery_result"].(map[string]any)
	payload := deliveryResult["payload"].(map[string]any)
	outputPath := payload["path"].(string)
	if outputPath == "" {
		t.Fatal("expected workspace document delivery to carry a payload path")
	}

	content, err := os.ReadFile(filepath.Join(workspaceRoot, strings.TrimPrefix(outputPath, "workspace/")))
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if !strings.Contains(string(content), "# 处理结果") {
		t.Fatalf("expected written file to contain title header, got %s", string(content))
	}

	taskID := result["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime")
	}
	if record.LatestToolCall["tool_name"] != "write_file" {
		t.Fatalf("expected runtime task to record write_file tool call, got %v", record.LatestToolCall["tool_name"])
	}
	output, ok := record.LatestToolCall["output"].(map[string]any)
	if !ok {
		t.Fatalf("expected latest tool call output map, got %+v", record.LatestToolCall)
		if output["summary_output"] == nil {
			t.Fatalf("expected write_file tool output to include summary_output, got %+v", output)
		}
		if output["model_invocation"] == nil {
			t.Fatalf("expected latest tool call to include model invocation, got %+v", output)
		}
		if output["audit_record"] == nil {
			t.Fatalf("expected latest tool call to include audit record, got %+v", output)
		}
		if output["recovery_point"] != nil {
			t.Fatalf("expected no recovery_point for create flow, got %+v", output)
		}
	}
}

func TestServiceStartTaskWithExecutorReturnsGeneratedBubble(t *testing.T) {
	service, _ := newTestServiceWithExecution(t, "这段内容主要在解释当前问题的原因和处理方向。")

	result, err := service.StartTask(map[string]any{
		"session_id": "sess_exec",
		"source":     "floating_ball",
		"trigger":    "hover_text_input",
		"input": map[string]any{
			"type": "text",
			"text": "请解释这段内容",
		},
		"intent": map[string]any{
			"name":      "explain",
			"arguments": map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("start task failed: %v", err)
	}

	bubble := result["bubble_message"].(map[string]any)
	if bubble["text"] != "这段内容主要在解释当前问题的原因和处理方向。" {
		t.Fatalf("expected bubble text to use generated output, got %v", bubble["text"])
	}

	taskID := result["task"].(map[string]any)["task_id"].(string)
	record, ok := service.runEngine.GetTask(taskID)
	if !ok {
		t.Fatal("expected task to remain in runtime")
	}
	if record.LatestToolCall["tool_name"] != "generate_text" {
		t.Fatalf("expected runtime task to record generate_text tool call, got %v", record.LatestToolCall["tool_name"])
	}
	output, ok := record.LatestToolCall["output"].(map[string]any)
	if !ok {
		t.Fatalf("expected latest tool call output map, got %+v", record.LatestToolCall)
	}
	if output["model_invocation"] == nil {
		t.Fatalf("expected latest tool call to include model invocation, got %+v", output)
	}
	if output["audit_record"] == nil {
		t.Fatalf("expected latest tool call to include audit record, got %+v", output)
	}
}

func modelConfig() serviceconfig.ModelConfig {
	return serviceconfig.ModelConfig{
		Provider: "openai_responses",
		ModelID:  "gpt-5.4",
		Endpoint: "https://api.openai.com/v1/responses",
	}
}
