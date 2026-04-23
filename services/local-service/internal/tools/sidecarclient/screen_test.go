package sidecarclient

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestNoopScreenCaptureClientReturnsCapabilityErrors(t *testing.T) {
	client := NewNoopScreenCaptureClient()
	if _, err := client.StartSession(context.Background(), tools.ScreenSessionStartInput{}); err != tools.ErrScreenCaptureNotSupported {
		t.Fatalf("expected unsupported error, got %v", err)
	}
	if _, err := client.GetSession(context.Background(), "screen_sess_missing"); err != tools.ErrScreenCaptureSessionExpired {
		t.Fatalf("expected expired session error, got %v", err)
	}
	if _, err := client.StopSession(context.Background(), "screen_sess_missing", "stopped"); err != tools.ErrScreenCaptureSessionExpired {
		t.Fatalf("expected stop session error, got %v", err)
	}
	if _, err := client.ExpireSession(context.Background(), "screen_sess_missing", "expired"); err != tools.ErrScreenCaptureSessionExpired {
		t.Fatalf("expected expire session error, got %v", err)
	}
	if _, err := client.CaptureScreenshot(context.Background(), tools.ScreenCaptureInput{}); err != tools.ErrScreenCaptureFailed {
		t.Fatalf("expected capture failed error, got %v", err)
	}
	if _, err := client.CaptureKeyframe(context.Background(), tools.ScreenCaptureInput{}); err != tools.ErrScreenKeyframeSamplingFailed {
		t.Fatalf("expected keyframe failed error, got %v", err)
	}
	if _, err := client.CleanupSessionArtifacts(context.Background(), tools.ScreenCleanupInput{}); err != tools.ErrScreenCleanupFailed {
		t.Fatalf("expected cleanup failed error, got %v", err)
	}
	if _, err := client.CleanupExpiredScreenTemps(context.Background(), tools.ScreenCleanupInput{}); err != tools.ErrScreenCleanupFailed {
		t.Fatalf("expected expired cleanup failed error, got %v", err)
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

func TestInMemoryScreenCaptureClientExplicitExpirePathCleansTemps(t *testing.T) {
	client := NewInMemoryScreenCaptureClient()
	now := time.Date(2026, 4, 18, 16, 0, 0, 0, time.UTC)
	client.now = func() time.Time { return now }

	session, err := client.StartSession(context.Background(), tools.ScreenSessionStartInput{
		SessionID:   "sess_005",
		TaskID:      "task_005",
		RunID:       "run_005",
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
	expired, err := client.ExpireSession(context.Background(), session.ScreenSessionID, "ttl_hit")
	if err != nil || expired.TerminalReason != "ttl_hit" {
		t.Fatalf("expected explicit expire state, got session=%+v err=%v", expired, err)
	}
	cleanup, err := client.CleanupExpiredScreenTemps(context.Background(), tools.ScreenCleanupInput{Reason: "expired_cleanup", ExpiredBefore: now})
	if err != nil {
		t.Fatalf("cleanup expired temps failed: %v", err)
	}
	if cleanup.DeletedCount != 1 {
		t.Fatalf("expected explicit expire cleanup to remove one temp path, got %+v", cleanup)
	}
}

func TestScreenCleanupHelpersCoverBranchMatrix(t *testing.T) {
	now := time.Date(2026, 4, 18, 17, 0, 0, 0, time.UTC)
	if cutoff := cleanupCutoffTime(time.Time{}, now); !cutoff.Equal(now) {
		t.Fatalf("expected zero cutoff to fallback to now, got %v", cutoff)
	}
	explicit := now.Add(-time.Minute)
	if cutoff := cleanupCutoffTime(explicit, now); !cutoff.Equal(explicit) {
		t.Fatalf("expected explicit cutoff to round-trip, got %v", cutoff)
	}
	if !shouldCleanupScreenSessionState(tools.ScreenSessionState{AuthorizationState: tools.ScreenAuthorizationEnded}, now) {
		t.Fatal("expected ended session without ended_at to be cleanup eligible")
	}
	endedAfter := now.Add(time.Minute)
	if shouldCleanupScreenSessionState(tools.ScreenSessionState{AuthorizationState: tools.ScreenAuthorizationEnded, EndedAt: &endedAfter}, now) {
		t.Fatal("expected future ended_at to block cleanup")
	}
	endedBefore := now.Add(-time.Minute)
	if !shouldCleanupScreenSessionState(tools.ScreenSessionState{AuthorizationState: tools.ScreenAuthorizationExpired, EndedAt: &endedBefore}, now) {
		t.Fatal("expected expired session before cutoff to be cleanup eligible")
	}
	if shouldCleanupScreenSessionState(tools.ScreenSessionState{}, now) {
		t.Fatal("expected active session without expiry to skip cleanup")
	}
	if !shouldCleanupScreenSessionState(tools.ScreenSessionState{ExpiresAt: now.Add(-time.Minute)}, now) {
		t.Fatal("expected cutoff to cleanup expired session")
	}
}

func TestInMemoryScreenCaptureClientCaptureErrorBranches(t *testing.T) {
	client := NewInMemoryScreenCaptureClient()
	now := time.Date(2026, 4, 18, 18, 0, 0, 0, time.UTC)
	client.now = func() time.Time { return now }

	if _, err := client.CaptureKeyframe(context.Background(), tools.ScreenCaptureInput{ScreenSessionID: "screen_sess_missing"}); !errors.Is(err, tools.ErrScreenCaptureSessionExpired) {
		t.Fatalf("expected missing session error, got %v", err)
	}
	session, err := client.StartSession(context.Background(), tools.ScreenSessionStartInput{SessionID: "sess_006", TaskID: "task_006", RunID: "run_006", CaptureMode: tools.ScreenCaptureModeScreenshot, TTL: time.Minute})
	if err != nil {
		t.Fatalf("start session failed: %v", err)
	}
	if _, err := client.StopSession(context.Background(), session.ScreenSessionID, "stopped"); err != nil {
		t.Fatalf("stop session failed: %v", err)
	}
	if _, err := client.CaptureScreenshot(context.Background(), tools.ScreenCaptureInput{ScreenSessionID: session.ScreenSessionID}); !errors.Is(err, tools.ErrScreenCaptureUnauthorized) {
		t.Fatalf("expected stopped session capture to be unauthorized, got %v", err)
	}
	expiringSession, err := client.StartSession(context.Background(), tools.ScreenSessionStartInput{SessionID: "sess_007", TaskID: "task_007", RunID: "run_007", CaptureMode: tools.ScreenCaptureModeScreenshot, TTL: time.Minute})
	if err != nil {
		t.Fatalf("start expiring session failed: %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := client.CaptureScreenshot(context.Background(), tools.ScreenCaptureInput{ScreenSessionID: expiringSession.ScreenSessionID}); !errors.Is(err, tools.ErrScreenCaptureSessionExpired) {
		t.Fatalf("expected expired session capture to fail, got %v", err)
	}
}
