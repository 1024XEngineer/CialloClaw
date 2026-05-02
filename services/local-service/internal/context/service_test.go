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
				"browser_kind": " chrome ",
				"process_path": " C:/Program Files/Google/Chrome/Application/chrome.exe ",
				"process_id":   float64(4242),
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
	if snapshot.BrowserKind != "chrome" || snapshot.ProcessPath != "C:/Program Files/Google/Chrome/Application/chrome.exe" || snapshot.ProcessID != 4242 {
		t.Fatalf("expected browser attach hints to be normalized, got %+v", snapshot)
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

func TestServiceCapturePrefersInputPageContextAndFlatFallbackSignals(t *testing.T) {
	service := NewService()

	snapshot := service.Capture(map[string]any{
		"source": "floating_ball",
		"input": map[string]any{
			"type": "text",
			"text": "看看当前屏幕上哪里出错了",
			"page_context": map[string]any{
				"title":        " Build Pipeline ",
				"url":          " https://example.com/build ",
				"app_name":     " Chrome ",
				"browser_kind": " edge ",
				"process_path": " C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe ",
				"process_id":   float64(5150),
				"window_title": " Build Pipeline - Browser ",
			},
		},
		"context": map[string]any{
			"page": map[string]any{
				"title":        " Legacy Page Title ",
				"url":          " https://example.com/legacy ",
				"app_name":     " Legacy Browser ",
				"browser_kind": " chrome ",
				"process_path": " C:/Legacy/browser.exe ",
				"process_id":   float64(99),
				"window_title": " Legacy Window ",
				"visible_text": " fallback visible text ",
			},
			"screen_summary":      " build failed before release ",
			"hover_target":        " publish button ",
			"last_action":         " switch_window ",
			"dwell_millis":        float64(8200),
			"copy_count":          float64(1),
			"window_switch_count": float64(3),
			"page_switch_count":   float64(2),
			"screen": map[string]any{
				"visible_text": " fatal build error ",
			},
		},
	})

	if snapshot.PageTitle != "Build Pipeline" || snapshot.PageURL != "https://example.com/build" || snapshot.AppName != "Chrome" {
		t.Fatalf("expected input.page_context to stay authoritative, got %+v", snapshot)
	}
	if snapshot.BrowserKind != "edge" || snapshot.ProcessPath != "C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe" || snapshot.ProcessID != 5150 {
		t.Fatalf("expected input.page_context attach hints to stay authoritative, got %+v", snapshot)
	}
	if snapshot.WindowTitle != "Build Pipeline - Browser" {
		t.Fatalf("expected input.page_context window title to win, got %+v", snapshot)
	}
	if snapshot.VisibleText != "fallback visible text" {
		t.Fatalf("expected context.page visible_text fallback when page_context omits it, got %+v", snapshot)
	}
	if snapshot.ScreenSummary != "build failed before release" || snapshot.HoverTarget != "publish button" {
		t.Fatalf("expected flat screen fallbacks to stay normalized, got %+v", snapshot)
	}
	if snapshot.LastAction != "switch_window" || snapshot.DwellMillis != 8200 || snapshot.CopyCount != 1 || snapshot.WindowSwitches != 3 || snapshot.PageSwitches != 2 {
		t.Fatalf("expected flat behavior counters to be normalized, got %+v", snapshot)
	}
}

func TestServiceSnapshotAndCaptureInferenceHelpers(t *testing.T) {
	service := NewService()
	if snapshot := service.Snapshot(); snapshot["source"] != "desktop" {
		t.Fatalf("expected snapshot descriptor to report desktop source, got %+v", snapshot)
	}

	selectionOnly := service.Capture(map[string]any{
		"context": map[string]any{
			"selection": map[string]any{"text": " selected line "},
		},
	})
	if selectionOnly.InputType != "text_selection" || selectionOnly.Trigger != "text_selected_click" || selectionOnly.Text != "selected line" {
		t.Fatalf("expected selection-only payload to infer text selection input, got %+v", selectionOnly)
	}

	errorOnly := service.Capture(map[string]any{
		"context": map[string]any{
			"error": map[string]any{"message": " build failed "},
		},
	})
	if errorOnly.InputType != "error" || errorOnly.Trigger != "error_detected" || errorOnly.Text != "build failed" {
		t.Fatalf("expected error-only payload to infer error input, got %+v", errorOnly)
	}
}

func TestContextPrimitiveHelpersCoverAdditionalBranches(t *testing.T) {
	if values := stringSliceValue([]string{" demo ", "demo", "notes"}); len(values) != 2 || values[0] != "demo" || values[1] != "notes" {
		t.Fatalf("expected []string branch to trim and dedupe, got %+v", values)
	}
	if values := stringSliceValue("  single-item  "); len(values) != 1 || values[0] != "single-item" {
		t.Fatalf("expected string branch to trim values, got %+v", values)
	}
	if values := stringSliceValue("   "); values != nil {
		t.Fatalf("expected blank string branch to return nil, got %+v", values)
	}
	if value := intValue(map[string]any{"count": int64(9)}, "count", 1); value != 9 {
		t.Fatalf("expected int64 branch to decode value, got %d", value)
	}
	if value := intValue(map[string]any{"count": "invalid"}, "count", 3); value != 3 {
		t.Fatalf("expected fallback branch to preserve fallback, got %d", value)
	}
}
