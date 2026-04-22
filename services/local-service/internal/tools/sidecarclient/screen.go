package sidecarclient

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

type noopScreenCaptureClient struct{}

// NewNoopScreenCaptureClient returns a non-functional default capability used
// until the real screen capture bridge lands.
func NewNoopScreenCaptureClient() tools.ScreenCaptureClient {
	return noopScreenCaptureClient{}
}

func (noopScreenCaptureClient) StartSession(context.Context, tools.ScreenSessionStartInput) (tools.ScreenSessionState, error) {
	return tools.ScreenSessionState{}, tools.ErrScreenCaptureNotSupported
}

func (noopScreenCaptureClient) GetSession(context.Context, string) (tools.ScreenSessionState, error) {
	return tools.ScreenSessionState{}, tools.ErrScreenCaptureSessionExpired
}

func (noopScreenCaptureClient) StopSession(context.Context, string, string) (tools.ScreenSessionState, error) {
	return tools.ScreenSessionState{}, tools.ErrScreenCaptureSessionExpired
}

func (noopScreenCaptureClient) ExpireSession(context.Context, string, string) (tools.ScreenSessionState, error) {
	return tools.ScreenSessionState{}, tools.ErrScreenCaptureSessionExpired
}

func (noopScreenCaptureClient) CaptureScreenshot(context.Context, tools.ScreenCaptureInput) (tools.ScreenFrameCandidate, error) {
	return tools.ScreenFrameCandidate{}, tools.ErrScreenCaptureFailed
}

func (noopScreenCaptureClient) CaptureKeyframe(context.Context, tools.ScreenCaptureInput) (tools.KeyframeCaptureResult, error) {
	return tools.KeyframeCaptureResult{}, tools.ErrScreenKeyframeSamplingFailed
}

func (noopScreenCaptureClient) CleanupSessionArtifacts(context.Context, tools.ScreenCleanupInput) (tools.ScreenCleanupResult, error) {
	return tools.ScreenCleanupResult{}, tools.ErrScreenCleanupFailed
}

func (noopScreenCaptureClient) CleanupExpiredScreenTemps(context.Context, tools.ScreenCleanupInput) (tools.ScreenCleanupResult, error) {
	return tools.ScreenCleanupResult{}, tools.ErrScreenCleanupFailed
}

// InMemoryScreenCaptureClient is a batch-3 capability skeleton for focused
// tests. It models session/capture/cleanup semantics without freezing any real
// platform bridge or persistence shape.
type InMemoryScreenCaptureClient struct {
	mu          sync.Mutex
	now         func() time.Time
	nextID      int
	sessions    map[string]tools.ScreenSessionState
	frameCounts map[string]int
	tempPaths   map[string][]string
}

func NewInMemoryScreenCaptureClient() *InMemoryScreenCaptureClient {
	return &InMemoryScreenCaptureClient{
		now:         time.Now,
		sessions:    map[string]tools.ScreenSessionState{},
		frameCounts: map[string]int{},
		tempPaths:   map[string][]string{},
	}
}

func (c *InMemoryScreenCaptureClient) StartSession(_ context.Context, input tools.ScreenSessionStartInput) (tools.ScreenSessionState, error) {
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
		Scope:              firstNonEmpty(input.Scope, "current_screen"),
		CaptureMode:        input.CaptureMode,
		AuthorizationState: tools.ScreenAuthorizationGranted,
		CreatedAt:          now,
		ExpiresAt:          now.Add(defaultTTL(input.TTL)),
	}
	c.sessions[state.ScreenSessionID] = state
	return state, nil
}

func (c *InMemoryScreenCaptureClient) GetSession(_ context.Context, screenSessionID string) (tools.ScreenSessionState, error) {
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

func (c *InMemoryScreenCaptureClient) StopSession(_ context.Context, screenSessionID, reason string) (tools.ScreenSessionState, error) {
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

func (c *InMemoryScreenCaptureClient) ExpireSession(_ context.Context, screenSessionID, reason string) (tools.ScreenSessionState, error) {
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

func (c *InMemoryScreenCaptureClient) CaptureScreenshot(_ context.Context, input tools.ScreenCaptureInput) (tools.ScreenFrameCandidate, error) {
	return c.captureFrame(input, false)
}

func (c *InMemoryScreenCaptureClient) CaptureKeyframe(_ context.Context, input tools.ScreenCaptureInput) (tools.KeyframeCaptureResult, error) {
	candidate, err := c.captureFrame(input, true)
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

func (c *InMemoryScreenCaptureClient) CleanupSessionArtifacts(_ context.Context, input tools.ScreenCleanupInput) (tools.ScreenCleanupResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	paths := screenCleanupPaths(c.tempPaths[input.ScreenSessionID], input.Paths)
	deleted := append([]string(nil), paths...)
	remaining := removeScreenPaths(c.tempPaths[input.ScreenSessionID], paths)
	if len(remaining) == 0 {
		delete(c.tempPaths, input.ScreenSessionID)
		delete(c.frameCounts, input.ScreenSessionID)
		delete(c.sessions, input.ScreenSessionID)
	} else {
		c.tempPaths[input.ScreenSessionID] = remaining
		if state, ok := c.sessions[input.ScreenSessionID]; ok {
			stoppedAt := c.now().UTC()
			state.AuthorizationState = tools.ScreenAuthorizationEnded
			state.EndedAt = &stoppedAt
			state.TerminalReason = firstNonEmpty(input.Reason, "session_cleanup_pending_retry")
			c.sessions[input.ScreenSessionID] = state
		}
	}
	return tools.ScreenCleanupResult{
		ScreenSessionID: input.ScreenSessionID,
		Reason:          firstNonEmpty(input.Reason, "session_cleanup"),
		DeletedPaths:    deleted,
		DeletedCount:    len(deleted),
	}, nil
}

func (c *InMemoryScreenCaptureClient) CleanupExpiredScreenTemps(_ context.Context, input tools.ScreenCleanupInput) (tools.ScreenCleanupResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	deleted := make([]string, 0)
	for sessionID, state := range c.sessions {
		if !state.ExpiresAt.IsZero() && !state.ExpiresAt.After(input.ExpiredBefore) {
			deleted = append(deleted, c.tempPaths[sessionID]...)
			delete(c.tempPaths, sessionID)
			delete(c.frameCounts, sessionID)
			delete(c.sessions, sessionID)
		}
	}
	return tools.ScreenCleanupResult{
		Reason:       firstNonEmpty(input.Reason, "expired_cleanup"),
		DeletedPaths: deleted,
		DeletedCount: len(deleted),
	}, nil
}

func (c *InMemoryScreenCaptureClient) captureFrame(input tools.ScreenCaptureInput, keyframe bool) (tools.ScreenFrameCandidate, error) {
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
	c.frameCounts[input.ScreenSessionID]++
	frameNumber := c.frameCounts[input.ScreenSessionID]
	mode := input.CaptureMode
	if mode == "" {
		if keyframe {
			mode = tools.ScreenCaptureModeKeyframe
		} else {
			mode = tools.ScreenCaptureModeScreenshot
		}
	}
	now := c.now().UTC()
	frameID := fmt.Sprintf("frame_%04d", frameNumber)
	path := filepath.ToSlash(filepath.Join("temp", input.ScreenSessionID, fmt.Sprintf("%s%s", frameID, screenCaptureExtension(mode, input.SourcePath))))
	retentionPolicy := tools.ScreenRetentionTemporary
	cleanupRequired := true
	if input.AllowPersist {
		retentionPolicy = tools.ScreenRetentionArtifact
		cleanupRequired = false
	}
	candidate := tools.ScreenFrameCandidate{
		FrameID:           frameID,
		ScreenSessionID:   input.ScreenSessionID,
		TaskID:            state.TaskID,
		RunID:             state.RunID,
		CaptureMode:       mode,
		Source:            firstNonEmpty(input.Source, state.Source),
		Path:              path,
		CapturedAt:        now,
		IsKeyframe:        keyframe,
		DedupeFingerprint: fmt.Sprintf("%s:%s:%s:%d", input.ScreenSessionID, mode, strings.TrimSpace(input.SourcePath), frameNumber),
		RetentionPolicy:   retentionPolicy,
		CleanupRequired:   cleanupRequired,
	}
	if cleanupRequired {
		c.tempPaths[input.ScreenSessionID] = append(c.tempPaths[input.ScreenSessionID], path)
	}
	return candidate, nil
}

func (c *InMemoryScreenCaptureClient) nextScreenSessionID() string {
	c.nextID++
	return fmt.Sprintf("screen_sess_%04d", c.nextID)
}

func defaultTTL(ttl time.Duration) time.Duration {
	if ttl > 0 {
		return ttl
	}
	return 5 * time.Minute
}

func expireState(state tools.ScreenSessionState, endedAt time.Time, reason string) tools.ScreenSessionState {
	state.AuthorizationState = tools.ScreenAuthorizationExpired
	state.EndedAt = &endedAt
	state.TerminalReason = reason
	return state
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func screenCleanupPaths(defaultPaths, explicitPaths []string) []string {
	if len(explicitPaths) > 0 {
		return append([]string(nil), explicitPaths...)
	}
	return append([]string(nil), defaultPaths...)
}

func removeScreenPaths(existingPaths, removedPaths []string) []string {
	if len(existingPaths) == 0 || len(removedPaths) == 0 {
		return append([]string(nil), existingPaths...)
	}
	removed := make(map[string]int, len(removedPaths))
	for _, pathValue := range removedPaths {
		trimmed := strings.TrimSpace(pathValue)
		if trimmed == "" {
			continue
		}
		removed[trimmed]++
	}
	remaining := make([]string, 0, len(existingPaths))
	for _, pathValue := range existingPaths {
		trimmed := strings.TrimSpace(pathValue)
		if trimmed == "" {
			continue
		}
		if removed[trimmed] > 0 {
			removed[trimmed]--
			continue
		}
		remaining = append(remaining, trimmed)
	}
	if len(remaining) == 0 {
		return nil
	}
	return remaining
}

func screenCaptureExtension(mode tools.ScreenCaptureMode, sourcePath string) string {
	if ext := strings.TrimSpace(filepath.Ext(sourcePath)); ext != "" {
		return ext
	}
	switch mode {
	case tools.ScreenCaptureModeClip:
		return ".webm"
	case tools.ScreenCaptureModeKeyframe, tools.ScreenCaptureModeScreenshot:
		return ".png"
	default:
		return ".png"
	}
}
