package sidecarclient

import (
	"context"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestNoopScreenCaptureClientReturnsCapabilityErrors(t *testing.T) {
	client := NewNoopScreenCaptureClient()
	if _, err := client.StartSession(context.Background(), tools.ScreenSessionStartInput{}); err != tools.ErrScreenCaptureNotSupported {
		t.Fatalf("expected unsupported error, got %v", err)
	}
	if _, err := client.CaptureScreenshot(context.Background(), tools.ScreenCaptureInput{}); err != tools.ErrScreenCaptureFailed {
		t.Fatalf("expected capture failed error, got %v", err)
	}
	if _, err := client.CleanupSessionArtifacts(context.Background(), tools.ScreenCleanupInput{}); err != tools.ErrScreenCleanupFailed {
		t.Fatalf("expected cleanup failed error, got %v", err)
	}
}

func TestInMemoryScreenCaptureClientSessionLifecycle(t *testing.T) {
	client := NewInMemoryScreenCaptureClient()
	now := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	client.now = func() time.Time { return now }

	session, err := client.StartSession(context.Background(), tools.ScreenSessionStartInput{
		SessionID:   "sess_001",
		TaskID:      "task_001",
		RunID:       "run_001",
		Source:      "voice",
		CaptureMode: tools.ScreenCaptureModeScreenshot,
		TTL:         time.Minute,
	})
	if err != nil {
		t.Fatalf("start session failed: %v", err)
	}
	if session.AuthorizationState != tools.ScreenAuthorizationGranted {
		t.Fatalf("expected granted session, got %+v", session)
	}

	loaded, err := client.GetSession(context.Background(), session.ScreenSessionID)
	if err != nil {
		t.Fatalf("get session failed: %v", err)
	}
	if loaded.ScreenSessionID != session.ScreenSessionID {
		t.Fatalf("expected same session id, got %+v", loaded)
	}

	stopped, err := client.StopSession(context.Background(), session.ScreenSessionID, "user_stop")
	if err != nil {
		t.Fatalf("stop session failed: %v", err)
	}
	if stopped.AuthorizationState != tools.ScreenAuthorizationEnded || stopped.TerminalReason != "user_stop" {
		t.Fatalf("expected ended session, got %+v", stopped)
	}
	if _, err := client.GetSession(context.Background(), session.ScreenSessionID); err != tools.ErrScreenCaptureSessionExpired {
		t.Fatalf("expected stopped session to be unavailable, got %v", err)
	}
}

func TestInMemoryScreenCaptureClientCaptureAndCleanup(t *testing.T) {
	client := NewInMemoryScreenCaptureClient()
	now := time.Date(2026, 4, 18, 13, 0, 0, 0, time.UTC)
	client.now = func() time.Time { return now }

	session, err := client.StartSession(context.Background(), tools.ScreenSessionStartInput{
		SessionID:   "sess_002",
		TaskID:      "task_002",
		RunID:       "run_002",
		Source:      "bubble",
		CaptureMode: tools.ScreenCaptureModeKeyframe,
		TTL:         2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("start session failed: %v", err)
	}

	screenshot, err := client.CaptureScreenshot(context.Background(), tools.ScreenCaptureInput{
		ScreenSessionID: session.ScreenSessionID,
		CaptureMode:     tools.ScreenCaptureModeScreenshot,
		Source:          "task_control",
	})
	if err != nil {
		t.Fatalf("capture screenshot failed: %v", err)
	}
	if screenshot.CaptureMode != tools.ScreenCaptureModeScreenshot || screenshot.Path == "" || !screenshot.CleanupRequired {
		t.Fatalf("unexpected screenshot candidate: %+v", screenshot)
	}

	keyframe, err := client.CaptureKeyframe(context.Background(), tools.ScreenCaptureInput{
		ScreenSessionID: session.ScreenSessionID,
		CaptureMode:     tools.ScreenCaptureModeKeyframe,
		Source:          "task_control",
	})
	if err != nil {
		t.Fatalf("capture keyframe failed: %v", err)
	}
	if !keyframe.Candidate.IsKeyframe || keyframe.Promoted {
		t.Fatalf("unexpected keyframe result: %+v", keyframe)
	}

	cleanup, err := client.CleanupSessionArtifacts(context.Background(), tools.ScreenCleanupInput{ScreenSessionID: session.ScreenSessionID, Reason: "task_finished"})
	if err != nil {
		t.Fatalf("cleanup session artifacts failed: %v", err)
	}
	if cleanup.DeletedCount != 2 {
		t.Fatalf("expected two deleted paths, got %+v", cleanup)
	}
}

func TestInMemoryScreenCaptureClientExpiresAndCleansExpiredTemps(t *testing.T) {
	client := NewInMemoryScreenCaptureClient()
	now := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	client.now = func() time.Time { return now }

	session, err := client.StartSession(context.Background(), tools.ScreenSessionStartInput{
		SessionID:   "sess_003",
		TaskID:      "task_003",
		RunID:       "run_003",
		Source:      "voice",
		CaptureMode: tools.ScreenCaptureModeScreenshot,
		TTL:         time.Minute,
	})
	if err != nil {
		t.Fatalf("start session failed: %v", err)
	}
	if _, err := client.CaptureScreenshot(context.Background(), tools.ScreenCaptureInput{ScreenSessionID: session.ScreenSessionID}); err != nil {
		t.Fatalf("capture screenshot failed: %v", err)
	}

	now = now.Add(2 * time.Minute)
	if _, err := client.GetSession(context.Background(), session.ScreenSessionID); err != tools.ErrScreenCaptureSessionExpired {
		t.Fatalf("expected expired session error, got %v", err)
	}

	cleanup, err := client.CleanupExpiredScreenTemps(context.Background(), tools.ScreenCleanupInput{Reason: "ttl_cleanup", ExpiredBefore: now})
	if err != nil {
		t.Fatalf("cleanup expired temps failed: %v", err)
	}
	if cleanup.DeletedCount != 1 {
		t.Fatalf("expected one deleted temp path, got %+v", cleanup)
	}
}

func TestInMemoryScreenCaptureClientCleansStoppedSessionTempsOnExpiredScan(t *testing.T) {
	client := NewInMemoryScreenCaptureClient()
	now := time.Date(2026, 4, 18, 15, 0, 0, 0, time.UTC)
	client.now = func() time.Time { return now }

	session, err := client.StartSession(context.Background(), tools.ScreenSessionStartInput{
		SessionID:   "sess_004",
		TaskID:      "task_004",
		RunID:       "run_004",
		Source:      "voice",
		CaptureMode: tools.ScreenCaptureModeScreenshot,
		TTL:         10 * time.Minute,
	})
	if err != nil {
		t.Fatalf("start session failed: %v", err)
	}
	if _, err := client.CaptureScreenshot(context.Background(), tools.ScreenCaptureInput{ScreenSessionID: session.ScreenSessionID}); err != nil {
		t.Fatalf("capture screenshot failed: %v", err)
	}
	if _, err := client.StopSession(context.Background(), session.ScreenSessionID, "analysis_completed"); err != nil {
		t.Fatalf("stop session failed: %v", err)
	}

	cleanup, err := client.CleanupExpiredScreenTemps(context.Background(), tools.ScreenCleanupInput{Reason: "residue_cleanup", ExpiredBefore: now})
	if err != nil {
		t.Fatalf("cleanup expired temps failed: %v", err)
	}
	if cleanup.DeletedCount != 1 {
		t.Fatalf("expected stopped session cleanup to remove one temp path, got %+v", cleanup)
	}
}
