package intent

import (
	"strings"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
)

func TestSuggestInfersScreenAnalyzeFromVisualErrorRequest(t *testing.T) {
	service := NewService()

	suggestion := service.Suggest(taskcontext.TaskContextSnapshot{
		InputType:     "text",
		Text:          "帮我看看这个页面的报错",
		PageTitle:     "Build Dashboard",
		WindowTitle:   "Browser - Build Dashboard",
		VisibleText:   "Fatal build error: missing release asset",
		ScreenSummary: "release validation failed on current screen",
	}, nil, false)

	if got := stringValue(suggestion.Intent, "name"); got != "screen_analyze" {
		t.Fatalf("expected screen_analyze intent, got %q", got)
	}
	if suggestion.RequiresConfirm {
		t.Fatal("expected screen analyze suggestion to enter controlled flow without extra confirmation")
	}
	if suggestion.TaskSourceType != "hover_input" {
		t.Fatalf("expected hover_input source type, got %q", suggestion.TaskSourceType)
	}
	arguments, ok := suggestion.Intent["arguments"].(map[string]any)
	if !ok {
		t.Fatalf("expected screen analyze arguments, got %+v", suggestion.Intent)
	}
	if arguments["evidence_role"] != "error_evidence" {
		t.Fatalf("expected error_evidence role, got %+v", arguments)
	}
	if arguments["page_title"] != "Build Dashboard" {
		t.Fatalf("expected page title to be preserved, got %+v", arguments)
	}
}

func TestSuggestKeepsAgentLoopForPlainTextWithoutVisualSignals(t *testing.T) {
	service := NewService()

	suggestion := service.Suggest(taskcontext.TaskContextSnapshot{
		InputType: "text",
		Text:      "帮我整理今天的会议纪要",
	}, nil, false)

	if got := stringValue(suggestion.Intent, "name"); got != defaultAgentLoopIntent {
		t.Fatalf("expected default agent loop intent, got %q", got)
	}
}

func TestSuggestKeepsPlainFreeTextOnAgentLoopBeforeRouting(t *testing.T) {
	service := NewService()

	testCases := []string{"解释下", "整理会议纪要", "a.go", "v1.2", `C:\`, `@me`}
	for _, testCase := range testCases {
		t.Run(testCase, func(t *testing.T) {
			suggestion := service.Suggest(taskcontext.TaskContextSnapshot{
				InputType: "text",
				Text:      testCase,
			}, nil, false)

			if got := stringValue(suggestion.Intent, "name"); got != defaultAgentLoopIntent {
				t.Fatalf("expected short text to route through agent loop, got %q", got)
			}
			if suggestion.RequiresConfirm {
				t.Fatal("expected plain free text to skip forced confirmation before submit-time routing")
			}
		})
	}
}

func TestSuggestRespectsExplicitConfirmationRequestForFreeText(t *testing.T) {
	service := NewService()

	suggestion := service.Suggest(taskcontext.TaskContextSnapshot{
		InputType: "text",
		Text:      "你好",
	}, nil, true)

	if got := stringValue(suggestion.Intent, "name"); got != defaultAgentLoopIntent {
		t.Fatalf("expected explicit confirmation request to keep agent_loop intent, got %q", got)
	}
	if !suggestion.RequiresConfirm {
		t.Fatal("expected explicit confirmation request to preserve confirming_intent entry")
	}
}

func TestSuggestKeepsPlainTextSubjectAheadOfPageContextForAgentLoop(t *testing.T) {
	service := NewService()

	suggestion := service.Suggest(taskcontext.TaskContextSnapshot{
		InputType:   "text",
		Text:        "帮我整理今天的会议纪要",
		PageTitle:   "Build Dashboard",
		WindowTitle: "Browser - Build Dashboard",
	}, nil, false)

	if suggestion.TaskTitle != "整理今天的会议纪要" {
		t.Fatalf("expected task title to keep user text subject, got %q", suggestion.TaskTitle)
	}
}

func TestSuggestCompactsMergedConversationIntoShorterTaskTitle(t *testing.T) {
	service := NewService()

	suggestion := service.Suggest(taskcontext.TaskContextSnapshot{
		InputType: "text",
		Text: strings.Join([]string{
			"请帮我整理这次发布复盘",
			"重点补齐风险项和后续跟进安排",
		}, "\n\n"),
	}, nil, false)

	if suggestion.TaskTitle != "整理这次发布复盘 重点补齐风险项和后续跟进..." {
		t.Fatalf("expected fallback task title to remain bounded before model generation, got %q", suggestion.TaskTitle)
	}
}

func TestSuggestFallbackTitleUsesLaterContextWhenFirstSentenceIsLong(t *testing.T) {
	service := NewService()

	suggestion := service.Suggest(taskcontext.TaskContextSnapshot{
		InputType: "text",
		Text: strings.Join([]string{
			"请帮我把这段很长很长的发布说明重新整理成适合群里同步的版本并补齐背景信息",
			"重点保留回滚方案",
		}, "\n"),
	}, nil, true)

	if suggestion.TaskTitle != "把这段很长很长的发布说明重新整理成适合群里..." {
		t.Fatalf("expected local fallback to compact the full text instead of a pre-truncated first sentence, got %q", suggestion.TaskTitle)
	}
}
