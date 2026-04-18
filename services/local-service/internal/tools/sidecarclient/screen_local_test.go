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
	if cleanup.DeletedCount != 1 {
		t.Fatalf("expected one deleted temp file, got %+v", cleanup)
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
	if err != nil || cleanup.DeletedCount != 1 {
		t.Fatalf("expected expired temp cleanup, got cleanup=%+v err=%v", cleanup, err)
	}
}
