package checkpoint

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubWriter struct {
	points []RecoveryPoint
	err    error
}

func (s *stubWriter) WriteRecoveryPoint(_ context.Context, point RecoveryPoint) error {
	if s.err != nil {
		return s.err
	}
	s.points = append(s.points, point)
	return nil
}

func TestServiceBuildRecoveryPoint(t *testing.T) {
	service := NewService()

	tests := []struct {
		name    string
		input   CreateInput
		wantErr error
	}{
		{name: "missing_task_id", input: CreateInput{Summary: "before overwrite"}, wantErr: ErrTaskIDRequired},
		{name: "missing_summary", input: CreateInput{TaskID: "task_001"}, wantErr: ErrSummaryRequired},
		{name: "valid_point", input: CreateInput{TaskID: "task_001", Summary: "before overwrite", Objects: []string{"D:/workspace/report.md", ""}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			point, err := service.BuildRecoveryPoint(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildRecoveryPoint returned error: %v", err)
			}
			if point.RecoveryPointID == "" || point.CreatedAt == "" {
				t.Fatalf("expected generated recovery point id and created_at, got %+v", point)
			}
			if _, err := time.Parse(time.RFC3339, point.CreatedAt); err != nil {
				t.Fatalf("expected RFC3339 created_at, got %q", point.CreatedAt)
			}
			if len(point.Objects) != 1 {
				t.Fatalf("expected trimmed objects, got %+v", point.Objects)
			}
		})
	}
}

func TestServiceCreate(t *testing.T) {
	writer := &stubWriter{}
	service := NewService(writer)

	point, err := service.Create(context.Background(), CreateInput{
		TaskID:  "task_001",
		Summary: "before overwrite",
		Objects: []string{"D:/workspace/report.md"},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if len(writer.points) != 1 {
		t.Fatalf("expected 1 recovery point, got %d", len(writer.points))
	}
	if writer.points[0].RecoveryPointID != point.RecoveryPointID {
		t.Fatalf("expected persisted point to match returned point, got %+v vs %+v", writer.points[0], point)
	}
}

func TestServiceCreatePropagatesWriterError(t *testing.T) {
	writer := &stubWriter{err: errors.New("write failed")}
	service := NewService(writer)

	_, err := service.Create(context.Background(), CreateInput{
		TaskID:  "task_001",
		Summary: "before overwrite",
	})
	if err == nil {
		t.Fatal("expected writer error")
	}
}

func TestBuildCreateInputFromCandidate(t *testing.T) {
	tests := []struct {
		name       string
		taskID     string
		candidate  map[string]any
		wantCreate bool
		wantErr    error
	}{
		{name: "missing_task_id", taskID: "", candidate: map[string]any{"required": true, "target_path": "D:/workspace/report.md"}, wantErr: ErrTaskIDRequired},
		{name: "nil_candidate", taskID: "task_001", candidate: nil, wantErr: ErrCandidateInvalid},
		{name: "not_required", taskID: "task_001", candidate: map[string]any{"required": false, "target_path": "D:/workspace/report.md"}, wantCreate: false},
		{name: "required_missing_target", taskID: "task_001", candidate: map[string]any{"required": true}, wantErr: ErrCandidateInvalid},
		{name: "required_with_reason", taskID: "task_001", candidate: map[string]any{"required": true, "target_path": "D:/workspace/report.md", "reason": "write_file_before_change"}, wantCreate: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input, shouldCreate, err := BuildCreateInputFromCandidate(tc.taskID, tc.candidate)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("BuildCreateInputFromCandidate returned error: %v", err)
			}
			if shouldCreate != tc.wantCreate {
				t.Fatalf("expected shouldCreate=%v, got %v", tc.wantCreate, shouldCreate)
			}
			if shouldCreate && (input.TaskID != "task_001" || len(input.Objects) != 1) {
				t.Fatalf("unexpected converted input: %+v", input)
			}
		})
	}
}
