// 该文件负责恢复点层的最小骨架。
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

// Service 提供当前模块的服务能力。
type Service struct {
	writer Writer
}

// NewService 创建并返回Service。
func NewService(writers ...Writer) *Service {
	writer := Writer(noopWriter{})
	if len(writers) > 0 && writers[0] != nil {
		writer = writers[0]
	}
	return &Service{writer: writer}
}

// Status 处理当前模块的相关逻辑。
func (s *Service) Status() string {
	return "ready"
}

// BuildRecoveryPoint 把 CreateInput 归一化为最小恢复点结构。
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
		Mode:            "workspace_snapshot",
		Objects:         objects,
	}, nil
}

// BuildCreateInputFromCandidate 将上游 checkpoint candidate 转换为最小 checkpoint 输入。
//
// shouldCreate 表示当前 candidate 是否要求真正创建恢复点；
// 当前主要用于消费 tools 模块中的 checkpoint_candidate。
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

// Create 归一化并输出一条恢复点记录。
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

// CreateWithSnapshots 会在持久化 recovery_point 前先把目标对象快照落到工作区恢复目录。
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

// Apply 按 recovery_point 的对象清单恢复工作区快照。
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
	if strings.HasPrefix(normalized, "workspace/") {
		normalized = strings.TrimPrefix(normalized, "workspace/")
	}
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
