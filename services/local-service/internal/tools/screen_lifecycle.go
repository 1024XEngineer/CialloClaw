package tools

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ScreenArtifactMetadata is the minimal lifecycle metadata preserved when a
// screen frame candidate is promoted into an artifact-ready reference.
type ScreenArtifactMetadata struct {
	ScreenSessionID string                `json:"screen_session_id"`
	CaptureMode     ScreenCaptureMode     `json:"capture_mode"`
	Source          string                `json:"source"`
	CapturedAt      string                `json:"captured_at"`
	RetentionPolicy ScreenRetentionPolicy `json:"retention_policy"`
	EvidenceRole    string                `json:"evidence_role"`
	Extra           map[string]any        `json:"extra,omitempty"`
}

// ScreenLifecycleManager applies owner-5 lifecycle decisions to temporary
// screen outputs before later batches wire them into artifact persistence.
type ScreenLifecycleManager struct{}

// NewScreenLifecycleManager creates the minimal lifecycle helper used by batch 4.
func NewScreenLifecycleManager() *ScreenLifecycleManager {
	return &ScreenLifecycleManager{}
}

// PromoteFrameCandidate converts a keyframe or reviewed screenshot candidate
// into an artifact-ready reference plus minimal metadata. It does not persist
// the artifact; later layers remain responsible for formal artifact storage.
func (m *ScreenLifecycleManager) PromoteFrameCandidate(taskID string, candidate ScreenFrameCandidate, evidenceRole string, extra map[string]any) (ArtifactRef, ScreenArtifactMetadata, error) {
	if strings.TrimSpace(taskID) == "" {
		return ArtifactRef{}, ScreenArtifactMetadata{}, fmt.Errorf("task_id is required")
	}
	if strings.TrimSpace(candidate.Path) == "" || strings.TrimSpace(candidate.FrameID) == "" {
		return ArtifactRef{}, ScreenArtifactMetadata{}, fmt.Errorf("screen frame candidate is incomplete")
	}
	artifact := ArtifactRef{
		ArtifactType: "screen_capture",
		Title:        filepath.Base(candidate.Path),
		Path:         candidate.Path,
		MimeType:     guessScreenCaptureMimeType(candidate.Path),
	}
	metadata := ScreenArtifactMetadata{
		ScreenSessionID: candidate.ScreenSessionID,
		CaptureMode:     candidate.CaptureMode,
		Source:          candidate.Source,
		CapturedAt:      candidate.CapturedAt.UTC().Format(time.RFC3339),
		RetentionPolicy: firstNonEmptyRetention(candidate.RetentionPolicy, ScreenRetentionArtifact),
		EvidenceRole:    firstNonEmptyStringLocal(evidenceRole, "reviewable_evidence"),
		Extra:           cloneMapLocal(extra),
	}
	return artifact, metadata, nil
}

// BuildCleanupSummary converts the cleanup result into a stable summary payload
// that later audit/recovery layers can reuse without depending on raw temp path
// lists.
func (m *ScreenLifecycleManager) BuildCleanupSummary(result ScreenCleanupResult) map[string]any {
	return map[string]any{
		"screen_session_id": result.ScreenSessionID,
		"reason":            result.Reason,
		"deleted_count":     result.DeletedCount,
		"skipped_count":     result.SkippedCount,
		"deleted_paths":     append([]string(nil), result.DeletedPaths...),
		"skipped_paths":     append([]string(nil), result.SkippedPaths...),
	}
}

func firstNonEmptyRetention(values ...ScreenRetentionPolicy) ScreenRetentionPolicy {
	for _, value := range values {
		if strings.TrimSpace(string(value)) != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyStringLocal(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func cloneMapLocal(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func guessScreenCaptureMimeType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	default:
		return "image/png"
	}
}
