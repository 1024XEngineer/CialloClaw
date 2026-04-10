package tools

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ToolCallRecorder records minimal tool_call lifecycle snapshots to a sink.
type ToolCallRecorder struct {
	sink ToolCallSink
}

// NewToolCallRecorder creates a recorder with a safe default sink.
func NewToolCallRecorder(sink ToolCallSink) *ToolCallRecorder {
	if sink == nil {
		sink = NoopToolCallSink{}
	}
	return &ToolCallRecorder{sink: sink}
}

// Start creates and persists a started tool_call record.
func (r *ToolCallRecorder) Start(ctx context.Context, execCtx *ToolExecuteContext, toolName string, input map[string]any) ToolCallRecord {
	record := ToolCallRecord{
		ToolCallID: nextToolCallID(),
		ToolName:   toolName,
		Status:     ToolCallStatusStarted,
		Input:      summarizeMap(input),
	}
	if execCtx != nil {
		record.RunID = execCtx.RunID
		record.TaskID = execCtx.TaskID
		record.StepID = execCtx.StepID
	}
	_ = r.sink.SaveToolCall(ctx, record)
	return record
}

// Finish persists the final tool_call state.
func (r *ToolCallRecorder) Finish(ctx context.Context, record ToolCallRecord, status ToolCallStatus, output map[string]any, duration time.Duration, errorCode *int) ToolCallRecord {
	record.Status = status
	record.Output = summarizeMap(output)
	record.DurationMS = duration.Milliseconds()
	if record.DurationMS == 0 && duration > 0 {
		record.DurationMS = 1
	}
	record.ErrorCode = errorCode
	_ = r.sink.SaveToolCall(ctx, record)
	return record
}

// NoopToolCallSink drops all records.
type NoopToolCallSink struct{}

// SaveToolCall implements ToolCallSink.
func (NoopToolCallSink) SaveToolCall(_ context.Context, _ ToolCallRecord) error {
	return nil
}

// InMemoryToolCallSink stores records in memory for tests.
type InMemoryToolCallSink struct {
	mu      sync.Mutex
	Records []ToolCallRecord
}

// SaveToolCall implements ToolCallSink.
func (s *InMemoryToolCallSink) SaveToolCall(_ context.Context, record ToolCallRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Records = append(s.Records, record)
	return nil
}

// Snapshot returns a copy of all records.
func (s *InMemoryToolCallSink) Snapshot() []ToolCallRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]ToolCallRecord, len(s.Records))
	copy(items, s.Records)
	return items
}

var toolCallCounter uint64

func nextToolCallID() string {
	seq := atomic.AddUint64(&toolCallCounter, 1)
	return fmt.Sprintf("tool_call_%d_%d", time.Now().UnixNano(), seq)
}

func summarizeMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	summary := make(map[string]any, len(input))
	for key, value := range input {
		summary[key] = summarizeValue(value)
	}
	return summary
}

func summarizeValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case string:
		if len(v) > 256 {
			return map[string]any{"type": "string", "length": len(v)}
		}
		return v
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return v
	case []any:
		return map[string]any{"type": "array", "length": len(v)}
	case []string:
		return map[string]any{"type": "array", "length": len(v)}
	case map[string]any:
		return map[string]any{"type": "object", "keys": len(v)}
	default:
		return fmt.Sprintf("%T", value)
	}
}
