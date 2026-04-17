package recommendation

import (
	"strings"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/perception"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

func TestServiceGetBuildsRuntimeRecommendations(t *testing.T) {
	service := NewService()
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC) }

	result := service.Get(GenerateInput{
		Source:        "floating_ball",
		Scene:         "selected_text",
		PageTitle:     "Weekly Notes",
		AppName:       "editor",
		SelectionText: "This is a long block of selected text that should be summarized before any deeper editing happens.",
	})

	if result.CooldownHit {
		t.Fatal("expected first recommendation request to skip cooldown")
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected two recommendation items, got %d", len(result.Items))
	}
	if !strings.HasPrefix(result.Items[0]["recommendation_id"].(string), "rec_") {
		t.Fatalf("expected runtime recommendation id, got %+v", result.Items[0]["recommendation_id"])
	}
	if result.Items[0]["intent"].(map[string]any)["name"] != "summarize" {
		t.Fatalf("expected long selection to prefer summarize, got %+v", result.Items[0]["intent"])
	}
}

func TestServiceSubmitFeedbackEnforcesCooldown(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	input := GenerateInput{
		Source:        "floating_ball",
		Scene:         "selected_text",
		PageTitle:     "Design Doc",
		AppName:       "editor",
		SelectionText: "Translate this paragraph into English for the external summary.",
		UnfinishedTasks: []runengine.TaskRecord{
			{
				TaskID: "task_001",
				Title:  "Active runtime task",
				Status: "processing",
				Intent: map[string]any{"name": "rewrite"},
			},
		},
	}

	first := service.Get(input)
	recommendationID := first.Items[0]["recommendation_id"].(string)
	if !service.SubmitFeedback(recommendationID, "negative") {
		t.Fatal("expected negative feedback to be applied")
	}

	second := service.Get(input)
	if !second.CooldownHit {
		t.Fatal("expected repeated request after negative feedback to hit cooldown")
	}
	if len(second.Items) != 0 {
		t.Fatalf("expected cooldown request to suppress items, got %+v", second.Items)
	}
}

func TestServiceNegativeFeedbackExpiresAfterCooldown(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	input := GenerateInput{
		Source:        "floating_ball",
		Scene:         "selected_text",
		PageTitle:     "Design Doc",
		AppName:       "editor",
		SelectionText: "Translate this paragraph into English for the external summary.",
	}

	first := service.Get(input)
	if !service.SubmitFeedback(first.Items[0]["recommendation_id"].(string), "negative") {
		t.Fatal("expected negative feedback to be applied")
	}

	now = now.Add(negativeCooldown + time.Second)
	third := service.Get(input)
	if third.CooldownHit {
		t.Fatal("expected cooldown to expire")
	}
	if len(third.Items) == 0 {
		t.Fatal("expected recommendations after cooldown expiry")
	}
}

func TestRecommendationFingerprintIncludesRuntimeContext(t *testing.T) {
	base := GenerateInput{
		Source:    "floating_ball",
		Scene:     "hover",
		PageTitle: "Dashboard",
		AppName:   "desktop",
		UnfinishedTasks: []runengine.TaskRecord{
			{TaskID: "task_001", Status: "processing", Intent: map[string]any{"name": "rewrite"}},
		},
	}
	changed := base
	changed.UnfinishedTasks = []runengine.TaskRecord{
		{TaskID: "task_002", Status: "processing", Intent: map[string]any{"name": "rewrite"}},
	}

	if recommendationFingerprint(base) == recommendationFingerprint(changed) {
		t.Fatal("expected runtime task context to affect fingerprint")
	}
}

func TestServicePrunesExpiredRuntimeCaches(t *testing.T) {
	service := NewService()
	now := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	result := service.Get(GenerateInput{
		Source:       "floating_ball",
		Scene:        "hover",
		PageTitle:    "Dashboard",
		AppName:      "desktop",
		NotepadItems: []map[string]any{{"item_id": "todo_001", "title": "draft"}},
	})
	if len(result.Items) == 0 {
		t.Fatal("expected recommendation item to be created")
	}
	service.state["stale"] = fingerprintState{LastTouched: now.Add(-recommendationStateTTL - time.Minute)}

	now = now.Add(recommendationRecordTTL + time.Minute)
	service.Get(GenerateInput{
		Source:    "floating_ball",
		Scene:     "hover",
		PageTitle: "Dashboard",
		AppName:   "desktop",
	})

	if len(service.items) > 2 {
		t.Fatalf("expected expired recommendation items to be pruned, got %d", len(service.items))
	}
	if _, ok := service.state["stale"]; ok {
		t.Fatal("expected stale fingerprint state to be pruned")
	}
}

func TestServiceGetUsesCopyBehaviorAndSwitchSignals(t *testing.T) {
	service := NewService()
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC) }

	result := service.Get(GenerateInput{
		Source:        "floating_ball",
		Scene:         "hover",
		PageTitle:     "Release Checklist",
		AppName:       "browser",
		ClipboardText: "请 translate this paragraph into English before sharing externally.",
		VisibleText:   "Warning: release notes are incomplete.",
		DwellMillis:   16000,
		CopyCount:     1,
		Signals: perception.SignalSnapshot{
			Source:        "floating_ball",
			Scene:         "hover",
			PageTitle:     "Release Checklist",
			VisibleText:   "Warning: release notes are incomplete.",
			ClipboardText: "请 translate this paragraph into English before sharing externally.",
			DwellMillis:   16000,
			CopyCount:     1,
			LastAction:    "copy",
		},
	})

	if len(result.Items) == 0 {
		t.Fatal("expected recommendation items for copy behavior")
	}
	if result.Items[0]["intent"].(map[string]any)["name"] != "translate" {
		t.Fatalf("expected copy behavior to prioritize translate, got %+v", result.Items[0])
	}
}

func TestRecommendationFingerprintIncludesPerceptionSignals(t *testing.T) {
	base := GenerateInput{
		Source:    "floating_ball",
		Scene:     "hover",
		PageTitle: "Dashboard",
		Signals: perception.SignalSnapshot{
			Source:        "floating_ball",
			Scene:         "hover",
			PageTitle:     "Dashboard",
			ClipboardText: "copied text",
			CopyCount:     1,
		},
	}
	changed := base
	changed.Signals.CopyCount = 2

	if recommendationFingerprint(base) == recommendationFingerprint(changed) {
		t.Fatal("expected perception signals to affect fingerprint")
	}
}
