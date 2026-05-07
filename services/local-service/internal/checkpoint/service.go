// Package checkpoint builds and applies recovery points for workspace-scoped
// changes before risky actions commit.
package checkpoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"
	"sync/atomic"
	"time"
)

var (
	ErrTaskIDRequired   = errors.New("checkpoint: task_id is required")
	ErrSummaryRequired  = errors.New("checkpoint: summary is required")
	ErrCandidateInvalid = errors.New("checkpoint: candidate is invalid")
	ErrSnapshotFSNil    = errors.New("checkpoint: snapshot file system is required")
	ErrObjectsRequired  = errors.New("checkpoint: objects are required")
)

const snapshotRoot = ".recovery_points"

type noopWriter struct{}

func (noopWriter) WriteRecoveryPoint(_ context.Context, _ RecoveryPoint) error {
	return nil
}

type snapshotPayload struct {
	Exists  bool   `json:"exists"`
	Content []byte `json:"content,omitempty"`
}

// Service owns recovery-point construction and persistence without depending
// on a concrete storage backend.
type Service struct {
	writer Writer
}

// NewService creates a checkpoint service. When no writer is supplied, the
// service can still build recovery-point payloads for dry-run and tests.
func NewService(writers ...Writer) *Service {
	writer := Writer(noopWriter{})
	if len(writers) > 0 && writers[0] != nil {
		writer = writers[0]
	}
	return &Service{writer: writer}
}

// Status returns the module readiness string consumed by service health
// snapshots.
func (s *Service) Status() string {
	return "ready"
}

// BuildRecoveryPoint normalizes validated input into the recovery_point shape
// used by the governance chain.
func (s *Service) BuildRecoveryPoint(input CreateInput) (RecoveryPoint, error) {
	if err := validateCreateInput(input); err != nil {
		return RecoveryPoint{}, err
	}

	objects := make([]string, 0, len(input.Objects))
	for _, object := range input.Objects {
		trimmed := strings.TrimSpace(object)
		if trimmed != "" {
			objects = append(objects, trimmed)
		}
	}

	return RecoveryPoint{
		RecoveryPointID: nextRecoveryPointID(),
		TaskID:          strings.TrimSpace(input.TaskID),
		Summary:         strings.TrimSpace(input.Summary),
		CreatedAt:       time.Now().UTC().Format(time.RFC3339),
		Objects:         objects,
	}, nil
}

// BuildCreateInputFromCandidate adapts a tool checkpoint_candidate payload into
// the internal CreateInput contract.
//
// shouldCreate is true only when the candidate explicitly requires a persisted
// recovery point, which keeps optional tool hints from creating audit noise.
func BuildCreateInputFromCandidate(taskID string, candidate map[string]any) (input CreateInput, shouldCreate bool, err error) {
	if strings.TrimSpace(taskID) == "" {
		return CreateInput{}, false, ErrTaskIDRequired
	}
	if candidate == nil {
		return CreateInput{}, false, ErrCandidateInvalid
	}

	if required, ok := candidate["required"].(bool); ok {
		shouldCreate = required
	}
	if !shouldCreate {
		return CreateInput{}, false, nil
	}

	targetPath, _ := candidate["target_path"].(string)
	reason, _ := candidate["reason"].(string)
	trimmedTarget := strings.TrimSpace(targetPath)
	trimmedReason := strings.TrimSpace(reason)
	if trimmedTarget == "" {
		return CreateInput{}, false, ErrCandidateInvalid
	}

	summary := trimmedReason
	if summary == "" {
		summary = "checkpoint_requested"
	}

	return CreateInput{
		TaskID:  strings.TrimSpace(taskID),
		Summary: summary,
		Objects: []string{trimmedTarget},
	}, true, nil
}

// Create validates input, persists one recovery point, and returns the
// normalized record used by authorization and audit flows.
func (s *Service) Create(ctx context.Context, input CreateInput) (RecoveryPoint, error) {
	point, err := s.BuildRecoveryPoint(input)
	if err != nil {
		return RecoveryPoint{}, err
	}
	if err := s.writer.WriteRecoveryPoint(ctx, point); err != nil {
		return RecoveryPoint{}, fmt.Errorf("checkpoint: write recovery point: %w", err)
	}
	return point, nil
}

// CreateWithSnapshots writes object snapshots before persisting the
// recovery_point so an approved action has a rollback target if execution
// later fails.
func (s *Service) CreateWithSnapshots(ctx context.Context, fileSystem SnapshotFileSystem, input CreateInput) (RecoveryPoint, error) {
	if fileSystem == nil {
		return RecoveryPoint{}, ErrSnapshotFSNil
	}
	point, err := s.BuildRecoveryPoint(input)
	if err != nil {
		return RecoveryPoint{}, err
	}
	if len(point.Objects) == 0 {
		return RecoveryPoint{}, ErrObjectsRequired
	}
	for _, objectPath := range point.Objects {
		normalizedObject := normalizeSnapshotObjectPath(objectPath)
		if normalizedObject == "" {
			return RecoveryPoint{}, ErrCandidateInvalid
		}
		payload := snapshotPayload{Exists: true}
		content, err := fileSystem.ReadFile(normalizedObject)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				payload.Exists = false
			} else {
				return RecoveryPoint{}, fmt.Errorf("checkpoint: snapshot source %s: %w", normalizedObject, err)
			}
		} else {
			payload.Content = content
		}
		encodedPayload, err := json.Marshal(payload)
		if err != nil {
			return RecoveryPoint{}, fmt.Errorf("checkpoint: encode snapshot %s: %w", normalizedObject, err)
		}
		if err := fileSystem.WriteFile(snapshotPath(point.RecoveryPointID, normalizedObject), encodedPayload); err != nil {
			return RecoveryPoint{}, fmt.Errorf("checkpoint: write snapshot %s: %w", normalizedObject, err)
		}
	}
	if err := s.writer.WriteRecoveryPoint(ctx, point); err != nil {
		return RecoveryPoint{}, fmt.Errorf("checkpoint: write recovery point: %w", err)
	}
	return point, nil
}

// Apply restores all objects recorded by a recovery point and rolls back
// partial restore writes if any object restore fails.
func (s *Service) Apply(ctx context.Context, fileSystem SnapshotFileSystem, point RecoveryPoint) (ApplyResult, error) {
	_ = ctx
	if fileSystem == nil {
		return ApplyResult{}, ErrSnapshotFSNil
	}
	if strings.TrimSpace(point.RecoveryPointID) == "" {
		return ApplyResult{}, ErrCandidateInvalid
	}
	if len(point.Objects) == 0 {
		return ApplyResult{}, ErrObjectsRequired
	}

	backupPayloads := make(map[string]snapshotPayload, len(point.Objects))
	restoreBackups := make(map[string]snapshotPayload, len(point.Objects))
	orderedObjects := make([]string, 0, len(point.Objects))
	for _, objectPath := range point.Objects {
		normalizedObject := normalizeSnapshotObjectPath(objectPath)
		if normalizedObject == "" {
			return ApplyResult{}, ErrCandidateInvalid
		}
		content, err := fileSystem.ReadFile(snapshotPath(point.RecoveryPointID, normalizedObject))
		if err != nil {
			return ApplyResult{}, fmt.Errorf("checkpoint: read snapshot %s: %w", normalizedObject, err)
		}
		var payload snapshotPayload
		if err := json.Unmarshal(content, &payload); err != nil {
			return ApplyResult{}, fmt.Errorf("checkpoint: decode snapshot %s: %w", normalizedObject, err)
		}
		currentPayload := snapshotPayload{Exists: true}
		currentContent, err := fileSystem.ReadFile(normalizedObject)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				currentPayload.Exists = false
			} else {
				return ApplyResult{}, fmt.Errorf("checkpoint: read current object %s: %w", normalizedObject, err)
			}
		} else {
			currentPayload.Content = currentContent
		}
		backupPayloads[normalizedObject] = payload
		restoreBackups[normalizedObject] = currentPayload
		orderedObjects = append(orderedObjects, normalizedObject)
	}

	for _, objectPath := range orderedObjects {
		if err := applySnapshotPayload(fileSystem, objectPath, backupPayloads[objectPath]); err != nil {
			for rollbackIndex := len(orderedObjects) - 1; rollbackIndex >= 0; rollbackIndex-- {
				rollbackPath := orderedObjects[rollbackIndex]
				_ = applySnapshotPayload(fileSystem, rollbackPath, restoreBackups[rollbackPath])
			}
			return ApplyResult{}, fmt.Errorf("checkpoint: restore object %s: %w", objectPath, err)
		}
	}

	return ApplyResult{
		RecoveryPointID: point.RecoveryPointID,
		RestoredObjects: orderedObjects,
	}, nil
}

func validateCreateInput(input CreateInput) error {
	if strings.TrimSpace(input.TaskID) == "" {
		return ErrTaskIDRequired
	}
	if strings.TrimSpace(input.Summary) == "" {
		return ErrSummaryRequired
	}
	return nil
}

var recoveryPointCounter uint64

func nextRecoveryPointID() string {
	seq := atomic.AddUint64(&recoveryPointCounter, 1)
	return fmt.Sprintf("recovery_point_%d_%d", time.Now().UnixNano(), seq)
}

func snapshotPath(recoveryPointID, objectPath string) string {
	normalizedObject := normalizeSnapshotObjectPath(objectPath)
	if normalizedObject == "" {
		return ""
	}
	return path.Join(snapshotRoot, recoveryPointID, normalizedObject)
}

func normalizeSnapshotObjectPath(objectPath string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(objectPath, "\\", "/"))
	if normalized == "" {
		return ""
	}
	normalized = strings.TrimPrefix(normalized, "workspace/")
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimPrefix(normalized, "/")
	if len(normalized) >= 2 && normalized[1] == ':' {
		normalized = normalized[2:]
		normalized = strings.TrimPrefix(normalized, "/")
	}
	cleaned := path.Clean(normalized)
	if cleaned == "." || cleaned == "" || strings.HasPrefix(cleaned, "../") {
		return ""
	}
	return cleaned
}

func applySnapshotPayload(fileSystem SnapshotFileSystem, objectPath string, payload snapshotPayload) error {
	if payload.Exists {
		return fileSystem.WriteFile(objectPath, payload.Content)
	}
	err := fileSystem.Remove(objectPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
