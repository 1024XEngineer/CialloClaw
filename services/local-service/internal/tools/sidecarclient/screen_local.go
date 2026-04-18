package sidecarclient

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

// localScreenCaptureClient is a real owner-5 local bridge that captures from a
// workspace-controlled source file into managed temp screen session paths.
type localScreenCaptureClient struct {
	mu         sync.Mutex
	now        func() time.Time
	nextID     int
	fileSystem platform.FileSystemAdapter
	sessions   map[string]tools.ScreenSessionState
	frameCount map[string]int
	tempPaths  map[string][]string
}

func NewLocalScreenCaptureClient(fileSystem platform.FileSystemAdapter) tools.ScreenCaptureClient {
	if fileSystem == nil {
		return NewNoopScreenCaptureClient()
	}
	return &localScreenCaptureClient{
		now:        time.Now,
		fileSystem: fileSystem,
		sessions:   map[string]tools.ScreenSessionState{},
		frameCount: map[string]int{},
		tempPaths:  map[string][]string{},
	}
}

func (c *localScreenCaptureClient) StartSession(_ context.Context, input tools.ScreenSessionStartInput) (tools.ScreenSessionState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now().UTC()
	if strings.TrimSpace(input.SessionID) == "" || strings.TrimSpace(input.TaskID) == "" {
		return tools.ScreenSessionState{}, tools.ErrScreenCaptureFailed
	}
	state := tools.ScreenSessionState{
		ScreenSessionID:    c.nextScreenSessionID(),
		SessionID:          input.SessionID,
		TaskID:             input.TaskID,
		RunID:              input.RunID,
		Source:             input.Source,
		Scope:              firstNonEmpty(input.Scope, "workspace_screen_source"),
		CaptureMode:        input.CaptureMode,
		AuthorizationState: tools.ScreenAuthorizationGranted,
		CreatedAt:          now,
		ExpiresAt:          now.Add(defaultTTL(input.TTL)),
	}
	c.sessions[state.ScreenSessionID] = state
	return state, nil
}

func (c *localScreenCaptureClient) GetSession(_ context.Context, screenSessionID string) (tools.ScreenSessionState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, ok := c.sessions[screenSessionID]
	if !ok {
		return tools.ScreenSessionState{}, tools.ErrScreenCaptureSessionExpired
	}
	if state.AuthorizationState == tools.ScreenAuthorizationExpired || state.AuthorizationState == tools.ScreenAuthorizationEnded {
		return tools.ScreenSessionState{}, tools.ErrScreenCaptureSessionExpired
	}
	if !state.ExpiresAt.IsZero() && c.now().UTC().After(state.ExpiresAt) {
		expired := expireState(state, c.now().UTC(), "session_ttl_expired")
		c.sessions[screenSessionID] = expired
		return tools.ScreenSessionState{}, tools.ErrScreenCaptureSessionExpired
	}
	return state, nil
}

func (c *localScreenCaptureClient) StopSession(_ context.Context, screenSessionID, reason string) (tools.ScreenSessionState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, ok := c.sessions[screenSessionID]
	if !ok {
		return tools.ScreenSessionState{}, tools.ErrScreenCaptureSessionExpired
	}
	stoppedAt := c.now().UTC()
	state.AuthorizationState = tools.ScreenAuthorizationEnded
	state.EndedAt = &stoppedAt
	state.TerminalReason = firstNonEmpty(reason, "stopped")
	c.sessions[screenSessionID] = state
	return state, nil
}

func (c *localScreenCaptureClient) ExpireSession(_ context.Context, screenSessionID, reason string) (tools.ScreenSessionState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, ok := c.sessions[screenSessionID]
	if !ok {
		return tools.ScreenSessionState{}, tools.ErrScreenCaptureSessionExpired
	}
	expired := expireState(state, c.now().UTC(), firstNonEmpty(reason, "expired"))
	c.sessions[screenSessionID] = expired
	return expired, nil
}

func (c *localScreenCaptureClient) CaptureScreenshot(_ context.Context, input tools.ScreenCaptureInput) (tools.ScreenFrameCandidate, error) {
	return c.captureFromWorkspaceSource(input, false)
}

func (c *localScreenCaptureClient) CaptureKeyframe(_ context.Context, input tools.ScreenCaptureInput) (tools.KeyframeCaptureResult, error) {
	candidate, err := c.captureFromWorkspaceSource(input, true)
	if err != nil {
		return tools.KeyframeCaptureResult{}, err
	}
	return tools.KeyframeCaptureResult{
		Candidate:         candidate,
		Promoted:          false,
		PromotionReason:   "review_pending",
		DedupeFingerprint: candidate.DedupeFingerprint,
	}, nil
}

func (c *localScreenCaptureClient) CleanupSessionArtifacts(_ context.Context, input tools.ScreenCleanupInput) (tools.ScreenCleanupResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	deleted := make([]string, 0)
	for _, tempPath := range c.tempPaths[input.ScreenSessionID] {
		if err := c.fileSystem.Remove(tempPath); err == nil {
			deleted = append(deleted, tempPath)
		}
	}
	delete(c.tempPaths, input.ScreenSessionID)
	return tools.ScreenCleanupResult{
		ScreenSessionID: input.ScreenSessionID,
		Reason:          firstNonEmpty(input.Reason, "session_cleanup"),
		DeletedPaths:    deleted,
		DeletedCount:    len(deleted),
	}, nil
}

func (c *localScreenCaptureClient) CleanupExpiredScreenTemps(_ context.Context, input tools.ScreenCleanupInput) (tools.ScreenCleanupResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	deleted := make([]string, 0)
	for sessionID, state := range c.sessions {
		if !state.ExpiresAt.IsZero() && !state.ExpiresAt.After(input.ExpiredBefore) {
			for _, tempPath := range c.tempPaths[sessionID] {
				if err := c.fileSystem.Remove(tempPath); err == nil {
					deleted = append(deleted, tempPath)
				}
			}
			delete(c.tempPaths, sessionID)
		}
	}
	return tools.ScreenCleanupResult{
		Reason:       firstNonEmpty(input.Reason, "expired_cleanup"),
		DeletedPaths: deleted,
		DeletedCount: len(deleted),
	}, nil
}

func (c *localScreenCaptureClient) captureFromWorkspaceSource(input tools.ScreenCaptureInput, keyframe bool) (tools.ScreenFrameCandidate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, ok := c.sessions[input.ScreenSessionID]
	if !ok {
		return tools.ScreenFrameCandidate{}, tools.ErrScreenCaptureSessionExpired
	}
	if state.AuthorizationState != tools.ScreenAuthorizationGranted {
		return tools.ScreenFrameCandidate{}, tools.ErrScreenCaptureUnauthorized
	}
	if !state.ExpiresAt.IsZero() && c.now().UTC().After(state.ExpiresAt) {
		expired := expireState(state, c.now().UTC(), "session_ttl_expired")
		c.sessions[input.ScreenSessionID] = expired
		return tools.ScreenFrameCandidate{}, tools.ErrScreenCaptureSessionExpired
	}
	sourcePath := strings.TrimSpace(input.SourcePath)
	if sourcePath == "" {
		return tools.ScreenFrameCandidate{}, tools.ErrScreenCaptureFailed
	}
	content, err := c.fileSystem.ReadFile(sourcePath)
	if err != nil {
		return tools.ScreenFrameCandidate{}, tools.ErrScreenCaptureFailed
	}
	c.frameCount[input.ScreenSessionID]++
	frameNumber := c.frameCount[input.ScreenSessionID]
	mode := input.CaptureMode
	if mode == "" {
		if keyframe {
			mode = tools.ScreenCaptureModeKeyframe
		} else {
			mode = tools.ScreenCaptureModeScreenshot
		}
	}
	frameID := fmt.Sprintf("frame_%04d", frameNumber)
	outputPath := filepath.ToSlash(filepath.Join("temp", input.ScreenSessionID, fmt.Sprintf("%s%s", frameID, filepath.Ext(sourcePath))))
	if err := c.fileSystem.WriteFile(outputPath, content); err != nil {
		return tools.ScreenFrameCandidate{}, tools.ErrScreenCaptureFailed
	}
	now := c.now().UTC()
	candidate := tools.ScreenFrameCandidate{
		FrameID:           frameID,
		ScreenSessionID:   input.ScreenSessionID,
		TaskID:            state.TaskID,
		RunID:             state.RunID,
		CaptureMode:       mode,
		Source:            firstNonEmpty(input.Source, state.Source),
		Path:              outputPath,
		CapturedAt:        now,
		IsKeyframe:        keyframe,
		DedupeFingerprint: fmt.Sprintf("%s:%s:%s:%d", input.ScreenSessionID, mode, sourcePath, frameNumber),
		RetentionPolicy:   tools.ScreenRetentionTemporary,
		CleanupRequired:   true,
	}
	c.tempPaths[input.ScreenSessionID] = append(c.tempPaths[input.ScreenSessionID], outputPath)
	return candidate, nil
}

func (c *localScreenCaptureClient) nextScreenSessionID() string {
	c.nextID++
	return fmt.Sprintf("screen_local_%04d", c.nextID)
}
