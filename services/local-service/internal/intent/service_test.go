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

func TestServiceSuggestInfersTranslateFromCommandText(t *testing.T) {
	service := NewService()

	suggestion := service.Suggest(contextsvc.TaskContextSnapshot{
		InputType: "text",
		Text:      "Translate this note into English",
	}, nil, false)

	if suggestion.Intent["name"] != "translate" {
		t.Fatalf("expected translate intent, got %+v", suggestion.Intent)
	}
	if suggestion.TaskTitle != "翻译：Translate this not..." {
		t.Fatalf("expected translated task title to reflect text subject, got %s", suggestion.TaskTitle)
	}
}

func TestServiceSuggestSummarizesLongSelectionIntoWorkspaceDocument(t *testing.T) {
	service := NewService()

	suggestion := service.Suggest(contextsvc.TaskContextSnapshot{
		InputType:     "text_selection",
		SelectionText: "Line one of the selected content.\nLine two adds more detail for a runtime summary decision.",
	}, nil, false)

	if suggestion.Intent["name"] != "summarize" {
		t.Fatalf("expected long selection to prefer summarize, got %+v", suggestion.Intent)
	}
	if !suggestion.RequiresConfirm {
		t.Fatal("expected long selected text summary to require confirmation")
	}
	if suggestion.DirectDeliveryType != "workspace_document" {
		t.Fatalf("expected long selection summary to prefer workspace_document, got %s", suggestion.DirectDeliveryType)
	}
}
