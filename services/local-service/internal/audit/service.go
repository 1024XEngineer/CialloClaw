// 该文件负责审计层的最小骨架。
package audit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

var (
	ErrTaskIDRequired   = errors.New("audit: task_id is required")
	ErrTypeRequired     = errors.New("audit: type is required")
	ErrActionRequired   = errors.New("audit: action is required")
	ErrResultRequired   = errors.New("audit: result is required")
	ErrCandidateInvalid = errors.New("audit: candidate is invalid")
)

type noopWriter struct{}

func (noopWriter) WriteAuditRecord(_ context.Context, _ Record) error {
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

// BuildRecord 把 RecordInput 归一化为最小审计记录。
func (s *Service) BuildRecord(input RecordInput) (Record, error) {
	if err := validateRecordInput(input); err != nil {
		return Record{}, err
	}

	return Record{
		AuditID:   nextAuditID(),
		TaskID:    strings.TrimSpace(input.TaskID),
		Type:      strings.TrimSpace(input.Type),
		Action:    strings.TrimSpace(input.Action),
		Summary:   strings.TrimSpace(input.Summary),
		Target:    strings.TrimSpace(input.Target),
		Result:    strings.TrimSpace(input.Result),
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// BuildRecordInputFromCandidate 将上游 candidate 结构转换为最小 audit 输入。
//
// 当前主要用于消费 tools 模块产出的 audit_candidate，
// 不在此处扩展为通用协议解析器。
func BuildRecordInputFromCandidate(taskID string, candidate map[string]any) (RecordInput, error) {
	if strings.TrimSpace(taskID) == "" {
		return RecordInput{}, ErrTaskIDRequired
	}
	if candidate == nil {
		return RecordInput{}, ErrCandidateInvalid
	}

	typeValue, ok := candidate["type"].(string)
	if !ok || strings.TrimSpace(typeValue) == "" {
		return RecordInput{}, ErrTypeRequired
	}
	actionValue, ok := candidate["action"].(string)
	if !ok || strings.TrimSpace(actionValue) == "" {
		return RecordInput{}, ErrActionRequired
	}
	resultValue, ok := candidate["result"].(string)
	if !ok || strings.TrimSpace(resultValue) == "" {
		return RecordInput{}, ErrResultRequired
	}

	summaryValue, _ := candidate["summary"].(string)
	targetValue, _ := candidate["target"].(string)

	return RecordInput{
		TaskID:  strings.TrimSpace(taskID),
		Type:    strings.TrimSpace(typeValue),
		Action:  strings.TrimSpace(actionValue),
		Summary: strings.TrimSpace(summaryValue),
		Target:  strings.TrimSpace(targetValue),
		Result:  strings.TrimSpace(resultValue),
	}, nil
}

// Write 归一化并输出一条审计记录。
func (s *Service) Write(ctx context.Context, input RecordInput) (Record, error) {
	record, err := s.BuildRecord(input)
	if err != nil {
		return Record{}, err
	}
	if err := s.writer.WriteAuditRecord(ctx, record); err != nil {
		return Record{}, fmt.Errorf("audit: write record: %w", err)
	}
	return record, nil
}

func validateRecordInput(input RecordInput) error {
	if strings.TrimSpace(input.TaskID) == "" {
		return ErrTaskIDRequired
	}
	if strings.TrimSpace(input.Type) == "" {
		return ErrTypeRequired
	}
	if strings.TrimSpace(input.Action) == "" {
		return ErrActionRequired
	}
	if strings.TrimSpace(input.Result) == "" {
		return ErrResultRequired
	}
	return nil
}

var auditCounter uint64

func nextAuditID() string {
	seq := atomic.AddUint64(&auditCounter, 1)
	return fmt.Sprintf("audit_%d_%d", time.Now().UnixNano(), seq)
}
