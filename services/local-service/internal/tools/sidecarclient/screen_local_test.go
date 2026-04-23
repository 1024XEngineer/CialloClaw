package sidecarclient

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
	expired, err := client.ExpireSession(context.Background(), session.ScreenSessionID, "ttl_hit")
	if err != nil || expired.TerminalReason != "ttl_hit" {
		t.Fatalf("expected explicit expire path, got session=%+v err=%v", expired, err)
	}
	cleanup, err := client.CleanupExpiredScreenTemps(context.Background(), tools.ScreenCleanupInput{Reason: "ttl_cleanup", ExpiredBefore: now.Add(time.Minute)})
	if err != nil || cleanup.DeletedCount != 2 {
		t.Fatalf("expected expired temp cleanup, got cleanup=%+v err=%v", cleanup, err)
	}
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
	if cleanup.DeletedCount != 2 {
		t.Fatalf("expected orphan cleanup to remove file and directory, got %+v", cleanup)
	}
	if _, err := os.Stat(orphanDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected orphan screen dir to be removed, got %v", err)
	}
}
