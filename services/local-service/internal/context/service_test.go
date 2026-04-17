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
				"title":        " Editor ",
				"url":          " https://example.com/doc ",
				"app_name":     " desktop ",
				"window_title": " Browser - Example ",
				"visible_text": " visible paragraph ",
			},
			"clipboard": map[string]any{
				"text": " copied snippet ",
			},
			"screen": map[string]any{
				"summary":      " dashboard warning ",
				"hover_target": " export button ",
			},
			"behavior": map[string]any{
				"last_action":         " copy ",
				"dwell_millis":        15000,
				"copy_count":          2,
				"window_switch_count": 4,
				"page_switch_count":   3,
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
	if snapshot.WindowTitle != "Browser - Example" || snapshot.VisibleText != "visible paragraph" || snapshot.ScreenSummary != "dashboard warning" {
		t.Fatalf("expected richer perception fields to be normalized, got %+v", snapshot)
	}
	if snapshot.ClipboardText != "copied snippet" || snapshot.HoverTarget != "export button" || snapshot.LastAction != "copy" {
		t.Fatalf("expected clipboard and hover signals to be normalized, got %+v", snapshot)
	}
	if snapshot.DwellMillis != 15000 || snapshot.CopyCount != 2 || snapshot.WindowSwitches != 4 || snapshot.PageSwitches != 3 {
		t.Fatalf("expected numeric behavior counters to be normalized, got %+v", snapshot)
	}
}
