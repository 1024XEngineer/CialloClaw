package tools

import (
	"testing"
	"time"
)

func TestScreenLifecycleManagerPromoteFrameCandidate(t *testing.T) {
	manager := NewScreenLifecycleManager()
	artifact, metadata, err := manager.PromoteFrameCandidate("task_001", ScreenFrameCandidate{
		FrameID:         "frame_001",
		ScreenSessionID: "screen_sess_001",
		CaptureMode:     ScreenCaptureModeKeyframe,
		Source:          "voice",
		Path:            "temp/screen_sess_001/frame_001.png",
		CapturedAt:      time.Date(2026, 4, 18, 16, 0, 0, 0, time.UTC),
		RetentionPolicy: ScreenRetentionReview,
	}, "error_evidence", map[string]any{"region_count": 2})
	if err != nil {
		t.Fatalf("promote frame candidate failed: %v", err)
	}
	if artifact.ArtifactType != "screen_capture" || artifact.Path == "" {
		t.Fatalf("unexpected artifact ref: %+v", artifact)
	}
	if metadata.ScreenSessionID != "screen_sess_001" || metadata.EvidenceRole != "error_evidence" {
		t.Fatalf("unexpected lifecycle metadata: %+v", metadata)
	}
	if metadata.RetentionPolicy != ScreenRetentionReview {
		t.Fatalf("expected retention policy to round-trip, got %+v", metadata)
	}
	if metadata.Extra["region_count"] != 2 {
		t.Fatalf("expected extra metadata to survive, got %+v", metadata.Extra)
	}
}

func TestScreenLifecycleManagerBuildCleanupSummary(t *testing.T) {
	manager := NewScreenLifecycleManager()
	summary := manager.BuildCleanupSummary(ScreenCleanupResult{
		ScreenSessionID: "screen_sess_002",
		Reason:          "task_finished",
		DeletedPaths:    []string{"temp/a.png", "temp/b.png"},
		SkippedPaths:    []string{"temp/c.png"},
		DeletedCount:    2,
		SkippedCount:    1,
	})
	if summary["screen_session_id"] != "screen_sess_002" || summary["deleted_count"] != 2 {
		t.Fatalf("unexpected cleanup summary: %+v", summary)
	}
	deletedPaths, ok := summary["deleted_paths"].([]string)
	if !ok || len(deletedPaths) != 2 {
		t.Fatalf("expected deleted paths slice, got %+v", summary)
	}
}
