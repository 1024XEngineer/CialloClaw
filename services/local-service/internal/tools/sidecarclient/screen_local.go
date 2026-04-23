package sidecarclient

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
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
	skipped := make([]string, 0)
	paths := screenCleanupPaths(c.tempPaths[input.ScreenSessionID], input.Paths)
	for _, tempPath := range paths {
		removedPaths, err := removeLocalScreenCleanupPath(c.fileSystem, tempPath)
		if err == nil {
			deleted = append(deleted, removedPaths...)
			continue
		}
		if len(input.Paths) > 0 && errors.Is(err, fs.ErrNotExist) {
			deleted = append(deleted, tempPath)
		} else {
			skipped = append(skipped, tempPath)
		}
	}
	remaining := removeScreenPaths(c.tempPaths[input.ScreenSessionID], deleted)
	remaining = append(remaining, skipped...)
	remaining = uniqueScreenPaths(remaining)
	if len(input.Paths) == 0 && len(remaining) == 0 {
		if sessionDir := screenSessionTempDir(input.ScreenSessionID); strings.TrimSpace(sessionDir) != "" {
			if err := c.fileSystem.Remove(sessionDir); err == nil {
				deleted = append(deleted, sessionDir)
			}
		}
	}
	if len(remaining) == 0 {
		delete(c.tempPaths, input.ScreenSessionID)
		delete(c.frameCount, input.ScreenSessionID)
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
		SkippedPaths:    skipped,
		DeletedCount:    len(deleted),
		SkippedCount:    len(skipped),
	}, nil
}

func (c *localScreenCaptureClient) CleanupExpiredScreenTemps(_ context.Context, input tools.ScreenCleanupInput) (tools.ScreenCleanupResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	cutoff := cleanupCutoffTime(input.ExpiredBefore, c.now().UTC())
	deleted := make([]string, 0)
	skipped := make([]string, 0)
	for sessionID, state := range c.sessions {
		if shouldCleanupScreenSessionState(state, cutoff) {
			expired := expireState(state, c.now().UTC(), firstNonEmpty(input.Reason, "expired_cleanup"))
			sessionDeleted := make([]string, 0)
			sessionSkipped := make([]string, 0)
			paths := uniqueScreenPaths(c.tempPaths[sessionID])
			for _, tempPath := range paths {
				removedPaths, err := removeLocalScreenCleanupPath(c.fileSystem, tempPath)
				if err == nil {
					sessionDeleted = append(sessionDeleted, removedPaths...)
				} else {
					sessionSkipped = append(sessionSkipped, tempPath)
				}
			}
			deleted = append(deleted, sessionDeleted...)
			skipped = append(skipped, sessionSkipped...)
			remaining := uniqueScreenPaths(append(removeScreenPaths(c.tempPaths[sessionID], sessionDeleted), sessionSkipped...))
			if len(remaining) == 0 {
				if sessionDir := screenSessionTempDir(sessionID); strings.TrimSpace(sessionDir) != "" {
					if err := c.fileSystem.Remove(sessionDir); err == nil {
						sessionDeleted = append(sessionDeleted, sessionDir)
						deleted = append(deleted, sessionDir)
					}
				}
				delete(c.tempPaths, sessionID)
				delete(c.frameCount, sessionID)
				delete(c.sessions, sessionID)
				continue
			}
			c.tempPaths[sessionID] = remaining
			c.sessions[sessionID] = expired
		}
	}
	orphanDeleted, orphanSkipped := c.cleanupOrphanTempPathsLocked(cutoff)
	deleted = append(deleted, orphanDeleted...)
	skipped = append(skipped, orphanSkipped...)
	return tools.ScreenCleanupResult{
		Reason:       firstNonEmpty(input.Reason, "expired_cleanup"),
		DeletedPaths: deleted,
		SkippedPaths: skipped,
		DeletedCount: len(deleted),
		SkippedCount: len(skipped),
	}, nil
}

// cleanupOrphanedSessionTemps reclaims leftover temp screen directories that no
// longer have live session state after crashes or interrupted cleanup flows.
func (c *localScreenCaptureClient) cleanupOrphanedSessionTemps(cutoff time.Time) []string {
	if c == nil || c.fileSystem == nil {
		return nil
	}
	entries, err := fs.ReadDir(c.fileSystem, "temp")
	if err != nil {
		return nil
	}
	deleted := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() || !isManagedScreenTempDir(entry.Name()) {
			continue
		}
		sessionID := entry.Name()
		if state, ok := c.sessions[sessionID]; ok && !shouldCleanupScreenSessionState(state, cutoff) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if _, ok := c.sessions[sessionID]; !ok && info.ModTime().After(cutoff) {
			continue
		}
		removedPaths, err := removeLocalScreenCleanupPath(c.fileSystem, path.Join("temp", sessionID))
		if err != nil {
			continue
		}
		deleted = append(deleted, removedPaths...)
	}
	return deleted
}

func screenSessionTempDir(screenSessionID string) string {
	screenSessionID = strings.TrimSpace(screenSessionID)
	if screenSessionID == "" {
		return ""
	}
	return path.Join("temp", screenSessionID)
}

func isManagedScreenTempDir(name string) bool {
	name = strings.TrimSpace(name)
	return strings.HasPrefix(name, "screen_local_") || strings.HasPrefix(name, "screen_sess_")
}

func removeLocalScreenCleanupPath(fileSystem platform.FileSystemAdapter, cleanupPath string) ([]string, error) {
	if fileSystem == nil {
		return nil, fmt.Errorf("file system unavailable")
	}
	cleanupPath = strings.TrimSpace(cleanupPath)
	if cleanupPath == "" {
		return nil, nil
	}
	entries, err := fs.ReadDir(fileSystem, cleanupPath)
	if err == nil {
		deleted := make([]string, 0, len(entries)+1)
		for _, entry := range entries {
			childDeleted, childErr := removeLocalScreenCleanupPath(fileSystem, path.Join(cleanupPath, entry.Name()))
			deleted = append(deleted, childDeleted...)
			if childErr != nil {
				return deleted, childErr
			}
		}
		if err := fileSystem.Remove(cleanupPath); err != nil {
			return deleted, err
		}
		return append(deleted, cleanupPath), nil
	}
	if err := fileSystem.Remove(cleanupPath); err != nil {
		return nil, err
	}
	return []string{cleanupPath}, nil
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
	outputPath := filepath.ToSlash(filepath.Join("temp", input.ScreenSessionID, fmt.Sprintf("%s%s", frameID, screenCaptureExtension(mode, sourcePath))))
	if err := c.fileSystem.WriteFile(outputPath, content); err != nil {
		return tools.ScreenFrameCandidate{}, tools.ErrScreenCaptureFailed
	}
	now := c.now().UTC()
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
		Path:              outputPath,
		CapturedAt:        now,
		IsKeyframe:        keyframe,
		DedupeFingerprint: fmt.Sprintf("%s:%s:%s:%d", input.ScreenSessionID, mode, sourcePath, frameNumber),
		RetentionPolicy:   retentionPolicy,
		CleanupRequired:   cleanupRequired,
	}
	if cleanupRequired {
		c.tempPaths[input.ScreenSessionID] = append(c.tempPaths[input.ScreenSessionID], outputPath)
	}
	return candidate, nil
}

func (c *localScreenCaptureClient) nextScreenSessionID() string {
	c.nextID++
	return fmt.Sprintf("screen_local_%04d", c.nextID)
}

func (c *localScreenCaptureClient) cleanupOrphanTempPathsLocked(expiredBefore time.Time) ([]string, []string) {
	if c.fileSystem == nil || expiredBefore.IsZero() {
		return nil, nil
	}
	entries, err := fs.ReadDir(c.fileSystem, "temp")
	if err != nil {
		return nil, nil
	}
	trackedSessions := make(map[string]struct{}, len(c.sessions))
	for sessionID := range c.sessions {
		trackedSessions[sessionID] = struct{}{}
	}
	deleted := make([]string, 0)
	skipped := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sessionID := strings.TrimSpace(entry.Name())
		if sessionID == "" {
			continue
		}
		if !isLocalScreenSessionDir(sessionID) {
			continue
		}
		if _, ok := trackedSessions[sessionID]; ok {
			continue
		}
		sessionDir := filepath.ToSlash(filepath.Join("temp", sessionID))
		sessionDeleted, sessionSkipped := cleanupNestedOrphanTempPaths(c.fileSystem, sessionDir, expiredBefore)
		deleted = append(deleted, sessionDeleted...)
		skipped = append(skipped, sessionSkipped...)
	}
	return deleted, skipped
}

func isLocalScreenSessionDir(name string) bool {
	return isManagedScreenTempDir(name)
}

// cleanupNestedOrphanTempPaths reclaims nested clip-frame directories after the
// owning session record disappears so temp growth does not survive crashes.
func cleanupNestedOrphanTempPaths(fileSystem platform.FileSystemAdapter, sessionDir string, expiredBefore time.Time) ([]string, []string) {
	if fileSystem == nil || strings.TrimSpace(sessionDir) == "" {
		return nil, nil
	}
	deleted := make([]string, 0)
	skipped := make([]string, 0)
	directories := make([]string, 0)
	if err := fs.WalkDir(fileSystem, sessionDir, func(candidatePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		normalizedPath := filepath.ToSlash(candidatePath)
		if entry.IsDir() {
			if normalizedPath != sessionDir {
				directories = append(directories, normalizedPath)
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(expiredBefore) {
			return nil
		}
		if err := fileSystem.Remove(normalizedPath); err != nil {
			skipped = append(skipped, normalizedPath)
			return nil
		}
		deleted = append(deleted, normalizedPath)
		return nil
	}); err != nil {
		return deleted, skipped
	}
	directories = append(directories, sessionDir)
	sort.SliceStable(directories, func(i, j int) bool {
		return len(directories[i]) > len(directories[j])
	})
	for _, dirPath := range directories {
		_ = fileSystem.Remove(dirPath)
	}
	return deleted, skipped
}

func uniqueScreenPaths(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}
