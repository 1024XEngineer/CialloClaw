package builtin

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

type stubReadFilePlatform struct {
	workspaceRoot string
	files         map[string][]byte
	outOfScope    map[string]bool
	readErr       error
}

func newStubReadFilePlatform(workspaceRoot string) *stubReadFilePlatform {
	return &stubReadFilePlatform{
		workspaceRoot: workspaceRoot,
		files:         make(map[string][]byte),
		outOfScope:    make(map[string]bool),
	}
}

func (s *stubReadFilePlatform) Join(elem ...string) string { return filepath.Join(elem...) }
func (s *stubReadFilePlatform) Abs(path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(s.workspaceRoot, path), nil
}
func (s *stubReadFilePlatform) EnsureWithinWorkspace(path string) (string, error) {
	clean := filepath.Clean(path)
	if s.outOfScope[clean] {
		return "", errors.New("outside workspace")
	}
	if filepath.IsAbs(clean) {
		return clean, nil
	}
	return filepath.Join(s.workspaceRoot, clean), nil
}
func (s *stubReadFilePlatform) ReadDir(path string) ([]fs.DirEntry, error) { return nil, nil }
func (s *stubReadFilePlatform) ReadFile(path string) ([]byte, error) {
	if s.readErr != nil {
		return nil, s.readErr
	}
	content, ok := s.files[filepath.Clean(path)]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return append([]byte(nil), content...), nil
}
func (s *stubReadFilePlatform) WriteFile(path string, content []byte) error { return nil }
func (s *stubReadFilePlatform) Stat(path string) (fs.FileInfo, error)       { return nil, fs.ErrNotExist }

func TestReadFileToolExecuteSuccess(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubReadFilePlatform(workspace)
	target := filepath.Join(workspace, "notes", "demo.txt")
	platform.files[target] = []byte("hello world")
	tool := NewReadFileTool()

	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{"path": target})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.RawOutput["path"] != target || result.RawOutput["mime_type"] != readFileTextType {
		t.Fatalf("unexpected raw output: %+v", result.RawOutput)
	}
	if result.SummaryOutput["content_preview"] != "hello world" {
		t.Fatalf("unexpected summary output: %+v", result.SummaryOutput)
	}
}

func TestReadFileToolRejectsOutsideWorkspace(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubReadFilePlatform(workspace)
	outside := filepath.Clean("D:/outside/demo.txt")
	platform.outOfScope[outside] = true
	tool := NewReadFileTool()

	_, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{"path": outside})
	if !errors.Is(err, tools.ErrWorkspaceBoundaryDenied) {
		t.Fatalf("expected ErrWorkspaceBoundaryDenied, got %v", err)
	}
}

func TestReadFileToolReturnsExecutionErrorWhenReadFails(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubReadFilePlatform(workspace)
	platform.readErr = errors.New("read failed")
	tool := NewReadFileTool()

	_, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{"path": "notes/demo.txt"})
	if !errors.Is(err, tools.ErrToolExecutionFailed) {
		t.Fatalf("expected ErrToolExecutionFailed, got %v", err)
	}
}

func TestReadFileToolRequiresPlatform(t *testing.T) {
	tool := NewReadFileTool()

	_, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{}, map[string]any{"path": "notes/demo.txt"})
	if !errors.Is(err, tools.ErrCapabilityDenied) {
		t.Fatalf("expected ErrCapabilityDenied, got %v", err)
	}
}
