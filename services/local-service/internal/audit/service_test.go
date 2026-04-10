package audit

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubWriter struct {
	records []Record
	err     error
}

func (s *stubWriter) WriteAuditRecord(_ context.Context, record Record) error {
	if s.err != nil {
		return s.err
	}
	s.records = append(s.records, record)
	return nil
}

func TestServiceBuildRecord(t *testing.T) {
	service := NewService()

	tests := []struct {
		name    string
		input   RecordInput
		wantErr error
	}{
		{name: "missing_task_id", input: RecordInput{Type: "file", Action: "write_file", Result: "success"}, wantErr: ErrTaskIDRequired},
		{name: "missing_type", input: RecordInput{TaskID: "task_001", Action: "write_file", Result: "success"}, wantErr: ErrTypeRequired},
		{name: "missing_action", input: RecordInput{TaskID: "task_001", Type: "file", Result: "success"}, wantErr: ErrActionRequired},
		{name: "missing_result", input: RecordInput{TaskID: "task_001", Type: "file", Action: "write_file"}, wantErr: ErrResultRequired},
		{name: "valid_record", input: RecordInput{TaskID: "task_001", Type: "file", Action: "write_file", Summary: "write result file", Target: "D:/workspace/report.md", Result: "success"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			record, err := service.BuildRecord(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildRecord returned error: %v", err)
			}
			if record.AuditID == "" || record.CreatedAt == "" {
				t.Fatalf("expected generated audit id and created_at, got %+v", record)
			}
			if _, err := time.Parse(time.RFC3339, record.CreatedAt); err != nil {
				t.Fatalf("expected RFC3339 created_at, got %q", record.CreatedAt)
			}
		})
	}
}

func TestServiceWrite(t *testing.T) {
	writer := &stubWriter{}
	service := NewService(writer)

	record, err := service.Write(context.Background(), RecordInput{
		TaskID:  "task_001",
		Type:    "file",
		Action:  "write_file",
		Summary: "write result file",
		Target:  "D:/workspace/report.md",
		Result:  "success",
	})
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if len(writer.records) != 1 {
		t.Fatalf("expected 1 written record, got %d", len(writer.records))
	}
	if writer.records[0].AuditID != record.AuditID {
		t.Fatalf("expected persisted record to match returned record, got %+v vs %+v", writer.records[0], record)
	}
}

func TestServiceWritePropagatesWriterError(t *testing.T) {
	writer := &stubWriter{err: errors.New("write failed")}
	service := NewService(writer)

	_, err := service.Write(context.Background(), RecordInput{
		TaskID: "task_001",
		Type:   "file",
		Action: "write_file",
		Result: "success",
	})
	if err == nil {
		t.Fatal("expected writer error")
	}
}

func TestBuildRecordInputFromCandidate(t *testing.T) {
	tests := []struct {
		name      string
		taskID    string
		candidate map[string]any
		wantErr   error
	}{
		{name: "missing_task_id", taskID: "", candidate: map[string]any{"type": "file", "action": "write_file", "result": "success"}, wantErr: ErrTaskIDRequired},
		{name: "nil_candidate", taskID: "task_001", candidate: nil, wantErr: ErrCandidateInvalid},
		{name: "missing_type", taskID: "task_001", candidate: map[string]any{"action": "write_file", "result": "success"}, wantErr: ErrTypeRequired},
		{name: "missing_action", taskID: "task_001", candidate: map[string]any{"type": "file", "result": "success"}, wantErr: ErrActionRequired},
		{name: "missing_result", taskID: "task_001", candidate: map[string]any{"type": "file", "action": "write_file"}, wantErr: ErrResultRequired},
		{name: "valid_candidate", taskID: "task_001", candidate: map[string]any{"type": "file", "action": "write_file", "summary": "write file", "target": "D:/workspace/report.md", "result": "success"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input, err := BuildRecordInputFromCandidate(tc.taskID, tc.candidate)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildRecordInputFromCandidate returned error: %v", err)
			}
			if input.TaskID != "task_001" || input.Action != "write_file" {
				t.Fatalf("unexpected converted input: %+v", input)
			}
		})
	}
}
