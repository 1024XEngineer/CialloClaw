package orchestrator

import (
	"strings"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/intent"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/languagepolicy"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
)

func TestBubbleTextForConfirmationUsesSettingsDefaultLanguage(t *testing.T) {
	service, _ := newTestServiceWithModelClient(t, stubModelClient{
		output: `{"intent":{"name":"summarize","arguments":{"style":"brief"}}}`,
	})
	if _, _, _, _, err := service.runEngine.UpdateSettings(map[string]any{
		"general": map[string]any{"language": languagepolicy.ReplyLanguageEnglish},
	}); err != nil {
		t.Fatalf("update settings failed: %v", err)
	}

	text := service.bubbleTextForConfirmation(
		runengine.TaskRecord{},
		taskcontext.TaskContextSnapshot{},
		intent.Suggestion{RequiresConfirm: true},
		true,
	)
	if !strings.Contains(text, "Please confirm the goal first.") {
		t.Fatalf("expected english clarification bubble from settings default, got %q", text)
	}
}

func TestBubbleTextForConfirmationUsesChineseDefaultLanguageWithoutInputSignal(t *testing.T) {
	service, _ := newTestServiceWithModelClient(t, stubModelClient{
		output: `{"intent":{"name":"summarize","arguments":{"style":"brief"}}}`,
	})

	text := service.bubbleTextForConfirmation(
		runengine.TaskRecord{},
		taskcontext.TaskContextSnapshot{},
		intent.Suggestion{RequiresConfirm: true},
		true,
	)
	if !strings.Contains(text, "请先确认") {
		t.Fatalf("expected chinese clarification bubble from default settings fallback, got %q", text)
	}
}

func TestBubbleTextForConfirmationPrefersRememberedSessionLanguage(t *testing.T) {
	service, _ := newTestServiceWithModelClient(t, stubModelClient{
		output: `{"intent":{"name":"summarize","arguments":{"style":"brief"}}}`,
	})
	if _, _, _, _, err := service.runEngine.UpdateSettings(map[string]any{
		"general": map[string]any{"language": languagepolicy.ReplyLanguageEnglish},
	}); err != nil {
		t.Fatalf("update settings failed: %v", err)
	}

	text := service.bubbleTextForConfirmation(
		runengine.TaskRecord{},
		taskcontext.TaskContextSnapshot{
			InputType:            "text",
			Text:                 "open it",
			SessionReplyLanguage: languagepolicy.ReplyLanguageChinese,
			SessionContextText:   "请继续上一段中文任务上下文",
		},
		intent.Suggestion{RequiresConfirm: true},
		true,
	)
	if !strings.Contains(text, "请先确认") {
		t.Fatalf("expected remembered chinese session language to override default english, got %q", text)
	}
}

func TestBubbleTextForConfirmationUsesEnglishPageContextWithoutTextSignal(t *testing.T) {
	service, _ := newTestServiceWithModelClient(t, stubModelClient{
		output: `{"intent":{"name":"summarize","arguments":{"style":"brief"}}}`,
	})

	text := service.bubbleTextForConfirmation(
		runengine.TaskRecord{},
		taskcontext.TaskContextSnapshot{
			PageTitle: "Release notes dashboard",
		},
		intent.Suggestion{RequiresConfirm: true},
		true,
	)
	if !strings.Contains(text, "Please confirm the goal first.") {
		t.Fatalf("expected english clarification bubble from page context, got %q", text)
	}
}

func TestFallbackConfirmIntentTextUsesEnglishAction(t *testing.T) {
	text := fallbackConfirmIntentText(
		taskcontext.TaskContextSnapshot{},
		intent.Suggestion{
			Intent: map[string]any{
				"name": "summarize",
			},
		},
		languagepolicy.ReplyLanguageEnglish,
	)
	if strings.Contains(text, "总结") || strings.Contains(text, "处理") {
		t.Fatalf("expected english fallback confirmation action, got %q", text)
	}
	if text != "Do you want me to summarize?" {
		t.Fatalf("expected english fallback confirmation question, got %q", text)
	}
}

func TestBuildTaskConfirmQuestionPromptUsesReplyLanguage(t *testing.T) {
	snapshot := taskcontext.TaskContextSnapshot{Text: "openclaw docs"}
	suggestion := intent.Suggestion{
		TaskTitle:          "Open docs",
		TaskSourceType:     "floating_ball",
		DirectDeliveryType: "bubble",
		Intent: map[string]any{
			"name": "open_page",
			"arguments": map[string]any{
				"url": "https://openclaw.example",
			},
		},
	}

	englishPrompt := buildTaskConfirmQuestionPrompt(snapshot, suggestion, languagepolicy.ReplyLanguageEnglish)
	if !strings.Contains(englishPrompt, "Return exactly one concise English question and nothing else.") {
		t.Fatalf("expected english confirm prompt instruction, got %q", englishPrompt)
	}
	chinesePrompt := buildTaskConfirmQuestionPrompt(snapshot, suggestion, languagepolicy.ReplyLanguageChinese)
	if !strings.Contains(chinesePrompt, "Return exactly one concise Chinese question and nothing else.") {
		t.Fatalf("expected chinese confirm prompt instruction, got %q", chinesePrompt)
	}
}
