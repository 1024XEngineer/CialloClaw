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
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskinspector"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/builtin"
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

func newTestServiceWithExecution(t *testing.T, modelOutput string) (*Service, string) {
	t.Helper()

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy: %v", err)
	}

	modelService := model.NewService(modelConfig(), stubModelClient{output: modelOutput})
	deliveryService := delivery.NewService()
	toolRegistry := tools.NewRegistry()
	if err := builtin.RegisterBuiltinTools(toolRegistry); err != nil {
		t.Fatalf("RegisterBuiltinTools returned error: %v", err)
	}
	pluginService := plugin.NewService()
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	auditService := audit.NewService()
	executor := execution.NewService(fileSystem, modelService, auditService, deliveryService, toolRegistry, nil, pluginService)

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
	).WithAudit(auditService).WithExecutor(executor).WithTaskInspector(taskinspector.NewService(fileSystem))

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

func TestTaskInspectorRunAggregatesRuntimeState(t *testing.T) {
	service, workspaceRoot := newTestServiceWithExecution(t, "inspector output")

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
		"task_id": taskID,
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
		"task_id": taskID,
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
		"task_id": taskID,
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
		"task_id": taskID,
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
	}
	if output["model_invocation"] == nil {
		t.Fatalf("expected latest tool call to include model invocation, got %+v", output)
	}
	if output["audit_record"] == nil {
		t.Fatalf("expected latest tool call to include audit record, got %+v", output)
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
