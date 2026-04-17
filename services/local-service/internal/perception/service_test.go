package perception

import (
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

func TestCaptureContextSignalsNormalizesNestedSignals(t *testing.T) {
	snapshot := CaptureContextSignals("floating_ball", "hover", map[string]any{
		"page": map[string]any{
			"title":        "Article",
			"url":          "https://example.com/article",
			"app_name":     "browser",
			"window_title": "Browser - Example",
			"visible_text": "Important visible page text",
		},
		"clipboard": map[string]any{
			"text":      " copied text ",
			"mime_type": "text/plain",
		},
		"screen": map[string]any{
			"summary":      "A dashboard with a warning banner",
			"hover_target": "warning banner",
		},
		"behavior": map[string]any{
			"last_action":         "copy",
			"dwell_millis":        15000,
			"window_switch_count": 4,
			"page_switch_count":   2,
			"copy_count":          2,
		},
	})

	if snapshot.PageTitle != "Article" || snapshot.PageURL != "https://example.com/article" || snapshot.AppName != "browser" {
		t.Fatalf("expected page fields to be normalized, got %+v", snapshot)
	}
	if snapshot.ClipboardText != "copied text" || snapshot.ClipboardMimeType != "text/plain" {
		t.Fatalf("expected clipboard fields to be normalized, got %+v", snapshot)
	}
	if snapshot.DwellMillis != 15000 || snapshot.WindowSwitchCount != 4 || snapshot.CopyCount != 2 {
		t.Fatalf("expected behavior counters to be normalized, got %+v", snapshot)
	}
	if snapshot.HoverTarget != "warning banner" {
		t.Fatalf("expected hover target to be normalized, got %+v", snapshot)
	}
}

func TestBehaviorSignalsAndOpportunitiesReflectPerceptionContext(t *testing.T) {
	snapshot := SignalSnapshot{
		Source:            "floating_ball",
		Scene:             "hover",
		PageTitle:         "Release Checklist",
		WindowTitle:       "Editor - Release Checklist",
		VisibleText:       "Warning: release notes are incomplete.",
		ScreenSummary:     "A warning panel and copied paragraph are visible.",
		ClipboardText:     "Please translate this summary into English before publishing.",
		SelectionText:     "请帮我翻译成英文",
		HoverTarget:       "warning panel",
		LastAction:        "copy",
		DwellMillis:       18000,
		WindowSwitchCount: 3,
		CopyCount:         1,
	}

	signals := BehaviorSignals(snapshot)
	if len(signals) < 5 {
		t.Fatalf("expected rich behavior signals, got %+v", signals)
	}
	opportunities := IdentifyOpportunities(snapshot, nil, nil)
	if len(opportunities) == 0 {
		t.Fatal("expected perception opportunities to be identified")
	}
	if opportunities[0].Reason != "copy_behavior" || opportunities[0].IntentName != "translate" {
		t.Fatalf("expected copy behavior to rank first and prefer translate, got %+v", opportunities[0])
	}
	if SignalFingerprint(snapshot) == SignalFingerprint(SignalSnapshot{}) {
		t.Fatal("expected fingerprint to change with richer signals")
	}
}

func TestIdentifyOpportunitiesKeepsRuntimeContextDedupeStable(t *testing.T) {
	snapshot := SignalSnapshot{PageTitle: "Dashboard", DwellMillis: 14000, VisibleText: "Overview"}
	opportunities := IdentifyOpportunities(snapshot, []runengine.TaskRecord{{TaskID: "task_001", Status: "processing"}}, []map[string]any{{"item_id": "todo_001", "title": "draft"}})
	if len(opportunities) == 0 {
		t.Fatal("expected opportunities with runtime context")
	}
	seen := map[string]struct{}{}
	for _, opportunity := range opportunities {
		key := opportunity.IntentName + "|" + opportunity.Reason
		if _, ok := seen[key]; ok {
			t.Fatalf("expected opportunities to be deduped, got %+v", opportunities)
		}
		seen[key] = struct{}{}
	}
}
