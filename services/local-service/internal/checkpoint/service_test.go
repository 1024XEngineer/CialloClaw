package checkpoint

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
)

type stubWriter struct {
	points []RecoveryPoint
	err    error
}

type failingSnapshotFS struct {
	base         SnapshotFileSystem
	failPath     string
	failAfter    int
	writesByPath map[string]int
}

func (s *stubWriter) WriteRecoveryPoint(_ context.Context, point RecoveryPoint) error {
	if s.err != nil {
		return s.err
	}
	s.points = append(s.points, point)
	return nil
}

func (s *failingSnapshotFS) ReadFile(path string) ([]byte, error) {
	return s.base.ReadFile(path)
}

func (s *failingSnapshotFS) WriteFile(path string, content []byte) error {
	if s.writesByPath == nil {
		s.writesByPath = map[string]int{}
	}
	s.writesByPath[path]++
	if path == s.failPath && s.writesByPath[path] == s.failAfter {
		if err := s.base.WriteFile(path, []byte("partial content")); err != nil {
			return err
		}
		return errors.New("forced write failure")
	}
	return s.base.WriteFile(path, content)
}

func (s *failingSnapshotFS) Remove(path string) error {
	return s.base.Remove(path)
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

func TestServiceCreateWithSnapshotsAndApply(t *testing.T) {
	writer := &stubWriter{}
	service := NewService(writer)

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)

	if err := fileSystem.WriteFile("notes/report.md", []byte("before content")); err != nil {
		t.Fatalf("seed source file: %v", err)
	}

	point, err := service.CreateWithSnapshots(context.Background(), fileSystem, CreateInput{
		TaskID:  "task_001",
		Summary: "before overwrite",
		Objects: []string{"notes/report.md"},
	})
	if err != nil {
		t.Fatalf("CreateWithSnapshots returned error: %v", err)
	}
	if len(writer.points) != 1 {
		t.Fatalf("expected persisted recovery point, got %d", len(writer.points))
	}
	backupPath := filepath.Join(workspaceRoot, ".recovery_points", point.RecoveryPointID, "notes", "report.md")
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup file: %v", err)
	}
	var snapshot snapshotPayload
	if err := json.Unmarshal(backupContent, &snapshot); err != nil {
		t.Fatalf("decode snapshot payload: %v", err)
	}
	if !snapshot.Exists || string(snapshot.Content) != "before content" {
		t.Fatalf("expected snapshot payload to preserve original content, got %+v", snapshot)
	}

	if err := fileSystem.WriteFile("notes/report.md", []byte("after content")); err != nil {
		t.Fatalf("overwrite source file: %v", err)
	}

	applyResult, err := service.Apply(context.Background(), fileSystem, point)
	if err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if applyResult.RecoveryPointID != point.RecoveryPointID {
		t.Fatalf("expected apply result to reference %s, got %+v", point.RecoveryPointID, applyResult)
	}
	if len(applyResult.RestoredObjects) != 1 || applyResult.RestoredObjects[0] != "notes/report.md" {
		t.Fatalf("expected restored object notes/report.md, got %+v", applyResult.RestoredObjects)
	}
	restoredContent, err := os.ReadFile(filepath.Join(workspaceRoot, "notes", "report.md"))
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}
	if string(restoredContent) != "before content" {
		t.Fatalf("expected restore to recover original content, got %q", string(restoredContent))
	}
}

func TestServiceCreateWithSnapshotsHandlesNewFileRestore(t *testing.T) {
	writer := &stubWriter{}
	service := NewService(writer)

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)

	point, err := service.CreateWithSnapshots(context.Background(), fileSystem, CreateInput{
		TaskID:  "task_new_file",
		Summary: "before create",
		Objects: []string{"workspace/notes/new.md"},
	})
	if err != nil {
		t.Fatalf("CreateWithSnapshots returned error: %v", err)
	}
	if err := fileSystem.WriteFile("notes/new.md", []byte("created later")); err != nil {
		t.Fatalf("write new file: %v", err)
	}
	if _, err := fileSystem.Stat("notes/new.md"); err != nil {
		t.Fatalf("expected new file to exist, got %v", err)
	}
	if _, err := service.Apply(context.Background(), fileSystem, point); err != nil {
		t.Fatalf("Apply returned error: %v", err)
	}
	if _, err := fileSystem.Stat("notes/new.md"); !errors.Is(err, os.ErrNotExist) && !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected restore to remove newly created file, got %v", err)
	}
}

func TestServiceApplyRollsBackEarlierObjectsWhenLaterRestoreFails(t *testing.T) {
	writer := &stubWriter{}
	service := NewService(writer)

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("new local path policy: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	if err := fileSystem.WriteFile("notes/one.md", []byte("before one")); err != nil {
		t.Fatalf("seed one: %v", err)
	}
	if err := fileSystem.WriteFile("notes/two.md", []byte("before two")); err != nil {
		t.Fatalf("seed two: %v", err)
	}
	point, err := service.CreateWithSnapshots(context.Background(), fileSystem, CreateInput{
		TaskID:  "task_multi",
		Summary: "before overwrite",
		Objects: []string{"workspace/notes/one.md", "workspace/notes/two.md"},
	})
	if err != nil {
		t.Fatalf("CreateWithSnapshots returned error: %v", err)
	}
	if err := fileSystem.WriteFile("notes/one.md", []byte("after one")); err != nil {
		t.Fatalf("overwrite one: %v", err)
	}
	if err := fileSystem.WriteFile("notes/two.md", []byte("after two")); err != nil {
		t.Fatalf("overwrite two: %v", err)
	}
	failingFS := &failingSnapshotFS{base: fileSystem, failPath: "notes/two.md", failAfter: 1}
	if _, err := service.Apply(context.Background(), failingFS, point); err == nil {
		t.Fatal("expected apply to fail on second object")
	}
	oneContent, err := os.ReadFile(filepath.Join(workspaceRoot, "notes", "one.md"))
	if err != nil {
		t.Fatalf("read one after rollback: %v", err)
	}
	if string(oneContent) != "after one" {
		t.Fatalf("expected first object to roll back to current content after failed restore, got %q", string(oneContent))
	}
	twoContent, err := os.ReadFile(filepath.Join(workspaceRoot, "notes", "two.md"))
	if err != nil {
		t.Fatalf("read two after rollback: %v", err)
	}
	if string(twoContent) != "after two" {
		t.Fatalf("expected failed object to be restored to current content after rollback, got %q", string(twoContent))
	}
}
