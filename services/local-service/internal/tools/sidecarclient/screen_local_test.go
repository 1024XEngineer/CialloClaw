package sidecarclient

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestLocalScreenCaptureClientCapturesWorkspaceSourceAndCleansUp(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	policy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy failed: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(policy)
	if err := fileSystem.MkdirAll("inputs"); err != nil {
		t.Fatalf("mkdir inputs failed: %v", err)
	}
	if err := fileSystem.WriteFile("inputs/screen.png", []byte("fake-png")); err != nil {
		t.Fatalf("write source screenshot failed: %v", err)
	}

	client := NewLocalScreenCaptureClient(fileSystem).(*localScreenCaptureClient)
	client.now = func() time.Time { return time.Date(2026, 4, 18, 21, 0, 0, 0, time.UTC) }

	session, err := client.StartSession(context.Background(), tools.ScreenSessionStartInput{
		SessionID:   "sess_screen_001",
		TaskID:      "task_screen_001",
		RunID:       "run_screen_001",
		Source:      "voice",
		CaptureMode: tools.ScreenCaptureModeScreenshot,
		TTL:         2 * time.Minute,
	})
	if err != nil {
		t.Fatalf("start session failed: %v", err)
	}

	candidate, err := client.CaptureScreenshot(context.Background(), tools.ScreenCaptureInput{
		ScreenSessionID: session.ScreenSessionID,
		CaptureMode:     tools.ScreenCaptureModeScreenshot,
		Source:          "task_control",
		SourcePath:      "inputs/screen.png",
	})
	if err != nil {
		t.Fatalf("capture screenshot failed: %v", err)
	}
	if candidate.Path == "" || candidate.Path == "inputs/screen.png" {
		t.Fatalf("expected captured file to be copied into managed temp path, got %+v", candidate)
	}
	content, err := fileSystem.ReadFile(candidate.Path)
	if err != nil || string(content) != "fake-png" {
		t.Fatalf("expected captured content to exist in temp path, err=%v content=%q", err, string(content))
	}

	cleanup, err := client.CleanupSessionArtifacts(context.Background(), tools.ScreenCleanupInput{ScreenSessionID: session.ScreenSessionID, Reason: "task_finished"})
	if err != nil {
		t.Fatalf("cleanup session artifacts failed: %v", err)
	}
	if cleanup.DeletedCount != 2 {
		t.Fatalf("expected temp file and directory cleanup, got %+v", cleanup)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, filepath.FromSlash(candidate.Path))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected cleaned temp file to be removed, got %v", err)
	}
	if _, err := client.GetSession(context.Background(), session.ScreenSessionID); !errors.Is(err, tools.ErrScreenCaptureSessionExpired) {
		t.Fatalf("expected cleaned session to retire lifecycle state, got %v", err)
	}
}

func TestLocalScreenCaptureClientRejectsMissingWorkspaceSource(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	policy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy failed: %v", err)
	}
	client := NewLocalScreenCaptureClient(platform.NewLocalFileSystemAdapter(policy)).(*localScreenCaptureClient)
	client.now = func() time.Time { return time.Date(2026, 4, 18, 21, 30, 0, 0, time.UTC) }

	session, err := client.StartSession(context.Background(), tools.ScreenSessionStartInput{SessionID: "sess_screen_002", TaskID: "task_screen_002", RunID: "run_screen_002", CaptureMode: tools.ScreenCaptureModeScreenshot})
	if err != nil {
		t.Fatalf("start session failed: %v", err)
	}
	if _, err := client.CaptureScreenshot(context.Background(), tools.ScreenCaptureInput{ScreenSessionID: session.ScreenSessionID, SourcePath: "inputs/missing.png"}); !errors.Is(err, tools.ErrScreenCaptureFailed) {
		t.Fatalf("expected missing source to fail screen capture, got %v", err)
	}
}

func TestLocalScreenCaptureClientLookupAndCaptureErrorBranches(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	policy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy failed: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(policy)
	client := NewLocalScreenCaptureClient(fileSystem).(*localScreenCaptureClient)
	now := time.Date(2026, 4, 18, 21, 45, 0, 0, time.UTC)
	client.now = func() time.Time { return now }

	if _, err := client.GetSession(context.Background(), "screen_sess_missing"); !errors.Is(err, tools.ErrScreenCaptureSessionExpired) {
		t.Fatalf("expected missing session lookup to fail, got %v", err)
	}
	session, err := client.StartSession(context.Background(), tools.ScreenSessionStartInput{SessionID: "sess_screen_002b", TaskID: "task_screen_002b", RunID: "run_screen_002b", CaptureMode: tools.ScreenCaptureModeScreenshot, TTL: time.Minute})
	if err != nil {
		t.Fatalf("start session failed: %v", err)
	}
	if _, err := client.CaptureScreenshot(context.Background(), tools.ScreenCaptureInput{ScreenSessionID: session.ScreenSessionID}); !errors.Is(err, tools.ErrScreenCaptureFailed) {
		t.Fatalf("expected blank source path to fail capture, got %v", err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := client.GetSession(context.Background(), session.ScreenSessionID); !errors.Is(err, tools.ErrScreenCaptureSessionExpired) {
		t.Fatalf("expected ttl-expired session lookup to fail, got %v", err)
	}
}

func TestLocalScreenCaptureClientGetExpireAndKeyframeBranches(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	policy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy failed: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(policy)
	if err := fileSystem.MkdirAll("inputs"); err != nil {
		t.Fatalf("mkdir inputs failed: %v", err)
	}
	if err := fileSystem.WriteFile("inputs/frame.png", []byte("fake-frame")); err != nil {
		t.Fatalf("write source frame failed: %v", err)
	}
	client := NewLocalScreenCaptureClient(fileSystem).(*localScreenCaptureClient)
	now := time.Date(2026, 4, 18, 22, 0, 0, 0, time.UTC)
	client.now = func() time.Time { return now }

	session, err := client.StartSession(context.Background(), tools.ScreenSessionStartInput{SessionID: "sess_screen_003", TaskID: "task_screen_003", RunID: "run_screen_003", CaptureMode: tools.ScreenCaptureModeKeyframe, TTL: time.Minute})
	if err != nil {
		t.Fatalf("start session failed: %v", err)
	}
	loaded, err := client.GetSession(context.Background(), session.ScreenSessionID)
	if err != nil || loaded.ScreenSessionID != session.ScreenSessionID {
		t.Fatalf("expected live session lookup, got session=%+v err=%v", loaded, err)
	}
	keyframe, err := client.CaptureKeyframe(context.Background(), tools.ScreenCaptureInput{ScreenSessionID: session.ScreenSessionID, SourcePath: "inputs/frame.png"})
	if err != nil {
		t.Fatalf("capture keyframe failed: %v", err)
	}
	if !keyframe.Candidate.IsKeyframe || keyframe.PromotionReason != "review_pending" {
		t.Fatalf("expected keyframe capture result, got %+v", keyframe)
	}
	if err := fileSystem.WriteFile("inputs/clip.webm", []byte("fake-clip")); err != nil {
		t.Fatalf("write source clip failed: %v", err)
	}
	clip, err := client.CaptureScreenshot(context.Background(), tools.ScreenCaptureInput{ScreenSessionID: session.ScreenSessionID, CaptureMode: tools.ScreenCaptureModeClip, SourcePath: "inputs/clip.webm", AllowPersist: true})
	if err != nil {
		t.Fatalf("capture retained clip failed: %v", err)
	}
	if !strings.HasSuffix(clip.Path, ".webm") || clip.RetentionPolicy != tools.ScreenRetentionArtifact || clip.CleanupRequired {
		t.Fatalf("expected retained clip capture result, got %+v", clip)
	}
	expired, err := client.ExpireSession(context.Background(), session.ScreenSessionID, "ttl_hit")
	if err != nil || expired.TerminalReason != "ttl_hit" {
		t.Fatalf("expected explicit expire path, got session=%+v err=%v", expired, err)
	}
	cleanup, err := client.CleanupExpiredScreenTemps(context.Background(), tools.ScreenCleanupInput{Reason: "ttl_cleanup", ExpiredBefore: now.Add(time.Minute)})
	if err != nil || cleanup.DeletedCount != 1 {
		t.Fatalf("expected expired temp cleanup, got cleanup=%+v err=%v", cleanup, err)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, filepath.FromSlash(clip.Path))); err != nil {
		t.Fatalf("expected retained clip artifact to remain on disk, got %v", err)
	}
}

func TestLocalScreenCaptureClientStopAndHelperBranches(t *testing.T) {
	if _, ok := NewLocalScreenCaptureClient(nil).(noopScreenCaptureClient); !ok {
		t.Fatal("expected nil filesystem local client constructor to fall back to noop client")
	}
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	policy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy failed: %v", err)
	}
	client := NewLocalScreenCaptureClient(platform.NewLocalFileSystemAdapter(policy)).(*localScreenCaptureClient)
	client.now = func() time.Time { return time.Date(2026, 4, 18, 23, 0, 0, 0, time.UTC) }
	session, err := client.StartSession(context.Background(), tools.ScreenSessionStartInput{SessionID: "sess_screen_004", TaskID: "task_screen_004", RunID: "run_screen_004"})
	if err != nil {
		t.Fatalf("start session failed: %v", err)
	}
	stopped, err := client.StopSession(context.Background(), session.ScreenSessionID, "user_stop")
	if err != nil {
		t.Fatalf("stop session failed: %v", err)
	}
	if stopped.AuthorizationState != tools.ScreenAuthorizationEnded || stopped.TerminalReason != "user_stop" {
		t.Fatalf("expected stopped session state, got %+v", stopped)
	}
	if got := uniqueScreenPaths([]string{"a", " ", "a", "b"}); len(got) != 2 || got[1] != "b" {
		t.Fatalf("expected uniqueScreenPaths to trim and dedupe, got %+v", got)
	}
}

func TestLocalScreenCaptureClientCleanupSessionArtifactsTreatsPromotedCaptureAsResolved(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	policy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy failed: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(policy)
	if err := fileSystem.MkdirAll("inputs"); err != nil {
		t.Fatalf("mkdir inputs failed: %v", err)
	}
	if err := fileSystem.WriteFile("inputs/frame.png", []byte("fake-frame")); err != nil {
		t.Fatalf("write frame source failed: %v", err)
	}
	if err := fileSystem.MkdirAll("artifacts/screen/task_cleanup"); err != nil {
		t.Fatalf("mkdir artifacts failed: %v", err)
	}
	client := NewLocalScreenCaptureClient(fileSystem).(*localScreenCaptureClient)
	client.now = func() time.Time { return time.Date(2026, 4, 18, 23, 15, 0, 0, time.UTC) }
	session, err := client.StartSession(context.Background(), tools.ScreenSessionStartInput{SessionID: "sess_screen_cleanup", TaskID: "task_screen_cleanup", RunID: "run_screen_cleanup", CaptureMode: tools.ScreenCaptureModeScreenshot})
	if err != nil {
		t.Fatalf("start session failed: %v", err)
	}
	candidate, err := client.CaptureScreenshot(context.Background(), tools.ScreenCaptureInput{ScreenSessionID: session.ScreenSessionID, SourcePath: "inputs/frame.png"})
	if err != nil {
		t.Fatalf("capture screenshot failed: %v", err)
	}
	if err := fileSystem.Move(candidate.Path, "artifacts/screen/task_cleanup/frame.png"); err != nil {
		t.Fatalf("move promoted capture failed: %v", err)
	}
	cleanup, err := client.CleanupSessionArtifacts(context.Background(), tools.ScreenCleanupInput{
		ScreenSessionID: session.ScreenSessionID,
		Reason:          "analysis_completed",
		Paths:           []string{candidate.Path},
	})
	if err != nil {
		t.Fatalf("cleanup promoted capture failed: %v", err)
	}
	if cleanup.DeletedCount != 1 || cleanup.SkippedCount != 0 {
		t.Fatalf("expected promoted capture cleanup to clear tracked residue, got %+v", cleanup)
	}
	if _, err := client.GetSession(context.Background(), session.ScreenSessionID); !errors.Is(err, tools.ErrScreenCaptureSessionExpired) {
		t.Fatalf("expected cleaned session to be removed from active state, got %v", err)
	}
}

func TestLocalScreenCaptureClientCleansOrphanTempFiles(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	policy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy failed: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(policy)
	if err := fileSystem.MkdirAll("temp/screen_local_orphan_session"); err != nil {
		t.Fatalf("mkdir orphan temp dir failed: %v", err)
	}
	if err := fileSystem.MkdirAll("temp/screen_local_orphan_session/frame_0001_clip_frames"); err != nil {
		t.Fatalf("mkdir orphan nested temp dir failed: %v", err)
	}
	if err := fileSystem.MkdirAll("temp/tool_cache"); err != nil {
		t.Fatalf("mkdir unrelated temp dir failed: %v", err)
	}
	if err := fileSystem.MkdirAll("inputs"); err != nil {
		t.Fatalf("mkdir inputs failed: %v", err)
	}
	if err := fileSystem.WriteFile("temp/screen_local_orphan_session/orphan.png", []byte("orphan")); err != nil {
		t.Fatalf("write orphan temp file failed: %v", err)
	}
	if err := fileSystem.WriteFile("temp/screen_local_orphan_session/frame_0001_clip_frames/frame-001.jpg", []byte("orphan-frame")); err != nil {
		t.Fatalf("write orphan nested temp file failed: %v", err)
	}
	if err := fileSystem.WriteFile("temp/tool_cache/shared.tmp", []byte("shared-temp")); err != nil {
		t.Fatalf("write unrelated temp file failed: %v", err)
	}
	if err := fileSystem.WriteFile("inputs/live.png", []byte("live")); err != nil {
		t.Fatalf("write live source file failed: %v", err)
	}
	client := NewLocalScreenCaptureClient(fileSystem).(*localScreenCaptureClient)
	client.now = func() time.Time { return time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC) }
	liveSession, err := client.StartSession(context.Background(), tools.ScreenSessionStartInput{SessionID: "sess_screen_live", TaskID: "task_screen_live", RunID: "run_screen_live", TTL: 365 * 24 * time.Hour})
	if err != nil {
		t.Fatalf("start live session failed: %v", err)
	}
	liveCandidate, err := client.CaptureScreenshot(context.Background(), tools.ScreenCaptureInput{ScreenSessionID: liveSession.ScreenSessionID, SourcePath: "inputs/live.png"})
	if err != nil {
		t.Fatalf("capture live screenshot failed: %v", err)
	}
	cleanup, err := client.CleanupExpiredScreenTemps(context.Background(), tools.ScreenCleanupInput{Reason: "ttl_cleanup", ExpiredBefore: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatalf("cleanup expired temps failed: %v", err)
	}
	if cleanup.DeletedCount != 2 {
		t.Fatalf("expected orphan temp cleanup to remove only orphan file, got %+v", cleanup)
	}
	if !containsString(cleanup.DeletedPaths, "temp/screen_local_orphan_session/orphan.png") || !containsString(cleanup.DeletedPaths, "temp/screen_local_orphan_session/frame_0001_clip_frames/frame-001.jpg") {
		t.Fatalf("expected orphan temp cleanup to remove both top-level and nested orphan files, got %+v", cleanup)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, filepath.FromSlash("temp/screen_local_orphan_session/orphan.png"))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected orphan temp file to be removed, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, filepath.FromSlash("temp/screen_local_orphan_session/frame_0001_clip_frames/frame-001.jpg"))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected orphan nested temp file to be removed, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, filepath.FromSlash("temp/tool_cache/shared.tmp"))); err != nil {
		t.Fatalf("expected unrelated temp file to remain, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, filepath.FromSlash(liveCandidate.Path))); err != nil {
		t.Fatalf("expected tracked live temp file to remain, got %v", err)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestLocalScreenCaptureClientStopSessionLeavesResidueForExpiredCleanupScan(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	policy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy failed: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(policy)
	if err := fileSystem.MkdirAll("inputs"); err != nil {
		t.Fatalf("mkdir inputs failed: %v", err)
	}
	if err := fileSystem.WriteFile("inputs/frame.png", []byte("fake-frame")); err != nil {
		t.Fatalf("write frame source failed: %v", err)
	}
	client := NewLocalScreenCaptureClient(fileSystem).(*localScreenCaptureClient)
	now := time.Date(2026, 4, 18, 22, 30, 0, 0, time.UTC)
	client.now = func() time.Time { return now }

	session, err := client.StartSession(context.Background(), tools.ScreenSessionStartInput{SessionID: "sess_screen_004", TaskID: "task_screen_004", RunID: "run_screen_004", CaptureMode: tools.ScreenCaptureModeScreenshot, TTL: 10 * time.Minute})
	if err != nil {
		t.Fatalf("start session failed: %v", err)
	}
	candidate, err := client.CaptureScreenshot(context.Background(), tools.ScreenCaptureInput{ScreenSessionID: session.ScreenSessionID, SourcePath: "inputs/frame.png"})
	if err != nil {
		t.Fatalf("capture screenshot failed: %v", err)
	}
	stopped, err := client.StopSession(context.Background(), session.ScreenSessionID, "analysis_completed")
	if err != nil || stopped.TerminalReason != "analysis_completed" {
		t.Fatalf("expected explicit stop session state, got session=%+v err=%v", stopped, err)
	}
	if _, err := client.GetSession(context.Background(), session.ScreenSessionID); !errors.Is(err, tools.ErrScreenCaptureSessionExpired) {
		t.Fatalf("expected stopped session lookup to fail, got %v", err)
	}
	cleanup, err := client.CleanupExpiredScreenTemps(context.Background(), tools.ScreenCleanupInput{Reason: "residue_cleanup", ExpiredBefore: now})
	if err != nil || cleanup.DeletedCount != 2 {
		t.Fatalf("expected stopped session residue cleanup, got cleanup=%+v err=%v", cleanup, err)
	}
	if _, err := os.Stat(filepath.Join(workspaceRoot, filepath.FromSlash(candidate.Path))); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected stopped session temp file to be removed, got %v", err)
	}
}

func TestNewLocalScreenCaptureClientFallsBackToNoopWithoutFilesystem(t *testing.T) {
	client := NewLocalScreenCaptureClient(nil)
	if _, ok := client.(noopScreenCaptureClient); !ok {
		t.Fatalf("expected nil filesystem constructor to return noop client, got %T", client)
	}
	if screenSessionTempDir("") != "" {
		t.Fatalf("expected blank session id to yield empty temp dir")
	}
}

func TestLocalScreenCaptureClientCleanupHelpersCoverFreshOrphanBranches(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	policy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy failed: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(policy)
	freshOrphanDir := filepath.Join(workspaceRoot, "temp", "screen_local_fresh_001")
	if err := os.MkdirAll(freshOrphanDir, 0o755); err != nil {
		t.Fatalf("mkdir fresh orphan dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(freshOrphanDir, "frame_0001.png"), []byte("fresh-frame"), 0o644); err != nil {
		t.Fatalf("write fresh orphan frame failed: %v", err)
	}
	client := NewLocalScreenCaptureClient(fileSystem).(*localScreenCaptureClient)
	now := time.Date(2026, 4, 18, 23, 0, 0, 0, time.UTC)
	client.now = func() time.Time { return now }
	if deleted := client.cleanupOrphanedSessionTemps(now.Add(-time.Minute)); len(deleted) != 0 {
		t.Fatalf("expected fresh orphan dir to be preserved, got %+v", deleted)
	}
	if _, err := os.Stat(freshOrphanDir); err != nil {
		t.Fatalf("expected fresh orphan dir to remain, got %v", err)
	}
	if _, err := removeLocalScreenCleanupPath(nil, "temp/screen_local_nil"); err == nil {
		t.Fatal("expected nil filesystem cleanup helper to fail")
	}
	deleted, err := removeLocalScreenCleanupPath(fileSystem, "")
	if err != nil || len(deleted) != 0 {
		t.Fatalf("expected blank cleanup path to no-op, got deleted=%+v err=%v", deleted, err)
	}
	if !isManagedScreenTempDir("screen_local_0001") || !isManagedScreenTempDir("screen_sess_0001") || isManagedScreenTempDir("temp_misc_0001") {
		t.Fatal("expected managed screen temp dir detection to cover local and in-memory prefixes")
	}
}

func TestLocalScreenCaptureClientCleanupExpiredTempsReclaimsOrphanSessionDirs(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	policy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy failed: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(policy)
	orphanDir := filepath.Join(workspaceRoot, "temp", "screen_local_orphan_001")
	if err := os.MkdirAll(orphanDir, 0o755); err != nil {
		t.Fatalf("mkdir orphan dir failed: %v", err)
	}
	orphanFile := filepath.Join(orphanDir, "frame_0001.png")
	if err := os.WriteFile(orphanFile, []byte("fake-frame"), 0o644); err != nil {
		t.Fatalf("write orphan frame failed: %v", err)
	}
	orphanTime := time.Date(2026, 4, 18, 20, 0, 0, 0, time.UTC)
	if err := os.Chtimes(orphanDir, orphanTime, orphanTime); err != nil {
		t.Fatalf("chtimes orphan dir failed: %v", err)
	}
	if err := os.Chtimes(orphanFile, orphanTime, orphanTime); err != nil {
		t.Fatalf("chtimes orphan file failed: %v", err)
	}

	client := NewLocalScreenCaptureClient(fileSystem).(*localScreenCaptureClient)
	client.now = func() time.Time { return orphanTime.Add(10 * time.Minute) }

	cleanup, err := client.CleanupExpiredScreenTemps(context.Background(), tools.ScreenCleanupInput{Reason: "orphan_cleanup", ExpiredBefore: orphanTime.Add(time.Minute)})
	if err != nil {
		t.Fatalf("cleanup orphaned temps failed: %v", err)
	}
	if cleanup.DeletedCount != 1 {
		t.Fatalf("expected orphan cleanup to remove orphaned files, got %+v", cleanup)
	}
	if _, err := os.Stat(orphanDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected orphan screen dir to be removed, got %v", err)
	}
}
