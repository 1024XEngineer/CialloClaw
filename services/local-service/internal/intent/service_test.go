package intent

import (
	"testing"

	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
)

func TestServiceAnalyzeSnapshotWaitsWhenInputMissing(t *testing.T) {
	service := NewService()

	if state := service.AnalyzeSnapshot(contextsvc.TaskContextSnapshot{}); state != "waiting_input" {
		t.Fatalf("expected empty snapshot to wait for input, got %s", state)
	}
}

func TestServiceSuggestRoutesCommandTextToAgentLoop(t *testing.T) {
	service := NewService()

	suggestion := service.Suggest(contextsvc.TaskContextSnapshot{
		InputType: "text",
		Text:      "Translate this note into English",
	}, nil, false)

	if suggestion.Intent["name"] != defaultAgentLoopIntent {
		t.Fatalf("expected generic agent loop intent, got %+v", suggestion.Intent)
	}
	if suggestion.TaskTitle != "处理：Translate this not..." {
		t.Fatalf("expected generic task title to reflect text subject, got %s", suggestion.TaskTitle)
	}
}

func TestServiceSuggestRoutesLongSelectionToAgentLoopWorkspaceDelivery(t *testing.T) {
	service := NewService()

	suggestion := service.Suggest(contextsvc.TaskContextSnapshot{
		InputType:     "text_selection",
		SelectionText: "Line one of the selected content.\nLine two adds more detail for a runtime summary decision.",
	}, nil, false)

	if suggestion.Intent["name"] != defaultAgentLoopIntent {
		t.Fatalf("expected long selection to prefer agent loop, got %+v", suggestion.Intent)
	}
	if suggestion.RequiresConfirm {
		t.Fatal("expected long selected text to go directly into the agent loop")
	}
	if suggestion.DirectDeliveryType != "workspace_document" {
		t.Fatalf("expected long selection to prefer workspace_document, got %s", suggestion.DirectDeliveryType)
	}
}

func TestServiceSuggestShortTextKeepsIntentUnconfirmed(t *testing.T) {
	service := NewService()

	suggestion := service.Suggest(contextsvc.TaskContextSnapshot{
		InputType: "text",
		Text:      "你好",
	}, nil, false)

	if suggestion.IntentConfirmed {
		t.Fatalf("expected short free text to keep intent unconfirmed, got %+v", suggestion)
	}
	if len(suggestion.Intent) != 0 {
		t.Fatalf("expected short free text not to infer formal intent, got %+v", suggestion.Intent)
	}
	if !suggestion.RequiresConfirm {
		t.Fatal("expected short free text to require confirmation")
	}
	if suggestion.TaskTitle != "确认处理方式：你好" {
		t.Fatalf("expected confirmation-oriented task title, got %s", suggestion.TaskTitle)
	}
}

func TestServiceSuggestKeepsGenericAgentLoopForExplicitSummarizeLanguage(t *testing.T) {
	service := NewService()

	suggestion := service.Suggest(contextsvc.TaskContextSnapshot{
		InputType: "text",
		Text:      "总结一下这段内容",
	}, nil, false)

	if suggestion.Intent["name"] != defaultAgentLoopIntent {
		t.Fatalf("expected free-form summarize request to keep agent loop intent, got %+v", suggestion.Intent)
	}
	if !suggestion.IntentConfirmed {
		t.Fatalf("expected free-form summarize request to keep agent loop intent confirmed, got %+v", suggestion)
	}
}
