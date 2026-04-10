// 该文件负责恢复点层的最小骨架。
package checkpoint

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

var (
	ErrTaskIDRequired  = errors.New("checkpoint: task_id is required")
	ErrSummaryRequired = errors.New("checkpoint: summary is required")
)

type noopWriter struct{}

func (noopWriter) WriteRecoveryPoint(_ context.Context, _ RecoveryPoint) error {
	return nil
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
		Objects:         objects,
	}, nil
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
