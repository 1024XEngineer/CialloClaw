package context

import "testing"

func TestServiceCaptureNormalizesNestedContext(t *testing.T) {
	service := NewService()

	snapshot := service.Capture(map[string]any{
		"source": "floating_ball",
		"input": map[string]any{
			"files": []any{" workspace/report.md ", "workspace/report.md"},
		},
		"context": map[string]any{
			"selection": map[string]any{
				"text": " selected text ",
			},
			"page": map[string]any{
				"title":    " Editor ",
				"url":      " https://example.com/doc ",
				"app_name": " desktop ",
			},
		},
	})

	if snapshot.InputType != "file" {
		t.Fatalf("expected file input type, got %s", snapshot.InputType)
	}
	if snapshot.Trigger != "file_drop" {
		t.Fatalf("expected inferred file_drop trigger, got %s", snapshot.Trigger)
	}
	if snapshot.SelectionText != "selected text" {
		t.Fatalf("expected selection text to be trimmed, got %q", snapshot.SelectionText)
	}
	if len(snapshot.Files) != 1 || snapshot.Files[0] != "workspace/report.md" {
		t.Fatalf("expected files to be deduped and trimmed, got %+v", snapshot.Files)
	}
	if snapshot.PageTitle != "Editor" || snapshot.PageURL != "https://example.com/doc" || snapshot.AppName != "desktop" {
		t.Fatalf("expected page fields to be normalized, got %+v", snapshot)
	}
}
