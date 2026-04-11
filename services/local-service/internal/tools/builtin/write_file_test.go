package builtin

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

type stubWriteFilePlatform struct {
	workspaceRoot string
	files         map[string][]byte
	writeErr      error
	outOfScope    map[string]bool
	statErr       map[string]error
}

func newStubWriteFilePlatform(workspaceRoot string) *stubWriteFilePlatform {
	return &stubWriteFilePlatform{
		workspaceRoot: workspaceRoot,
		files:         make(map[string][]byte),
		outOfScope:    make(map[string]bool),
		statErr:       make(map[string]error),
	}
}

func (s *stubWriteFilePlatform) Join(elem ...string) string { return filepath.Join(elem...) }

func (s *stubWriteFilePlatform) Abs(path string) (string, error) {
	if isStubAbsolutePath(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(s.workspaceRoot, path), nil
}

func (s *stubWriteFilePlatform) EnsureWithinWorkspace(path string) (string, error) {
	clean := filepath.Clean(path)
	if s.outOfScope[clean] {
		return "", errors.New("path outside workspace")
	}
	if isStubAbsolutePath(clean) {
		return clean, nil
	}
	return filepath.Join(s.workspaceRoot, clean), nil
}

func (s *stubWriteFilePlatform) ReadFile(path string) ([]byte, error) {
	content, ok := s.files[filepath.Clean(path)]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return append([]byte(nil), content...), nil
}

func (s *stubWriteFilePlatform) ReadDir(path string) ([]fs.DirEntry, error) {
	return nil, nil
}

func (s *stubWriteFilePlatform) WriteFile(path string, content []byte) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	s.files[filepath.Clean(path)] = append([]byte(nil), content...)
	return nil
}

func (s *stubWriteFilePlatform) Stat(path string) (fs.FileInfo, error) {
	clean := filepath.Clean(path)
	if err, ok := s.statErr[clean]; ok {
		return nil, err
	}
	content, ok := s.files[clean]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return stubFileInfo{name: filepath.Base(clean), size: int64(len(content))}, nil
}

type stubFileInfo struct {
	name string
	size int64
}

func (s stubFileInfo) Name() string       { return s.name }
func (s stubFileInfo) Size() int64        { return s.size }
func (s stubFileInfo) Mode() fs.FileMode  { return 0o644 }
func (s stubFileInfo) ModTime() time.Time { return time.Time{} }
func (s stubFileInfo) IsDir() bool        { return false }
func (s stubFileInfo) Sys() any           { return nil }

func TestWriteFileToolWorkspaceCreate(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubWriteFilePlatform(workspace)
	tool := NewWriteFileTool()

	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{
		"path":    "notes/demo.txt",
		"content": "hello world",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.RawOutput["created"] != true || result.RawOutput["overwritten"] != false {
		t.Fatalf("unexpected create flags: %+v", result.RawOutput)
	}
	if result.RawOutput["bytes_written"] != len([]byte("hello world")) {
		t.Fatalf("unexpected bytes_written: %+v", result.RawOutput)
	}
	if result.RawOutput["artifact_candidate"] == nil || result.RawOutput["audit_candidate"] == nil || result.RawOutput["checkpoint_candidate"] == nil {
		t.Fatalf("expected candidate placeholders, got %+v", result.RawOutput)
	}
	if content := string(platform.files[filepath.Join(workspace, "notes", "demo.txt")]); content != "hello world" {
		t.Fatalf("unexpected written content: %q", content)
	}
}

func TestWriteFileToolWorkspaceOverwrite(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubWriteFilePlatform(workspace)
	target := filepath.Join(workspace, "notes", "demo.txt")
	platform.files[target] = []byte("old")
	tool := NewWriteFileTool()

	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{
		"path":    target,
		"content": "new-content",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.RawOutput["created"] != false || result.RawOutput["overwritten"] != true {
		t.Fatalf("unexpected overwrite flags: %+v", result.RawOutput)
	}
	checkpoint := result.RawOutput["checkpoint_candidate"].(map[string]any)
	if checkpoint["required"] != true {
		t.Fatalf("expected checkpoint required for overwrite, got %+v", checkpoint)
	}
	auditCandidate := result.RawOutput["audit_candidate"].(map[string]any)
	if auditCandidate["summary"] != "overwrite file" {
		t.Fatalf("expected overwrite audit summary, got %+v", auditCandidate)
	}
}

func TestWriteFileToolDryRunCreate(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubWriteFilePlatform(workspace)
	tool := NewWriteFileTool()

	result, err := tool.DryRun(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{
		"path":    "notes/demo.txt",
		"content": "hello world",
	})
	if err != nil {
		t.Fatalf("DryRun returned error: %v", err)
	}
	if result.RawOutput["dry_run"] != true {
		t.Fatalf("expected dry_run flag, got %+v", result.RawOutput)
	}
	if result.RawOutput["created"] != true || result.RawOutput["overwritten"] != false {
		t.Fatalf("unexpected create flags for dry run: %+v", result.RawOutput)
	}
	if _, exists := platform.files[filepath.Join(workspace, "notes", "demo.txt")]; exists {
		t.Fatal("expected dry run not to write any file")
	}
}

func TestWriteFileToolDryRunOverwrite(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubWriteFilePlatform(workspace)
	target := filepath.Join(workspace, "notes", "demo.txt")
	platform.files[target] = []byte("old")
	tool := NewWriteFileTool()

	result, err := tool.DryRun(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{
		"path":    target,
		"content": "new-content",
	})
	if err != nil {
		t.Fatalf("DryRun returned error: %v", err)
	}
	if result.RawOutput["created"] != false || result.RawOutput["overwritten"] != true {
		t.Fatalf("unexpected overwrite flags for dry run: %+v", result.RawOutput)
	}
	checkpoint := result.RawOutput["checkpoint_candidate"].(map[string]any)
	if checkpoint["required"] != true {
		t.Fatalf("expected checkpoint required for overwrite dry run, got %+v", checkpoint)
	}
	if content := string(platform.files[target]); content != "old" {
		t.Fatalf("expected dry run not to modify existing file, got %q", content)
	}
}

func TestWriteFileToolRejectsOutsideWorkspace(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubWriteFilePlatform(workspace)
	outside := filepath.Clean("D:/outside/demo.txt")
	platform.outOfScope[outside] = true
	tool := NewWriteFileTool()

	_, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{
		"path":    outside,
		"content": "blocked",
	})
	if !errors.Is(err, tools.ErrWorkspaceBoundaryDenied) {
		t.Fatalf("expected ErrWorkspaceBoundaryDenied, got %v", err)
	}
}

func TestWriteFileToolReturnsAdapterError(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubWriteFilePlatform(workspace)
	platform.writeErr = errors.New("disk full")
	tool := NewWriteFileTool()

	_, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{
		"path":    "notes/demo.txt",
		"content": "cannot write",
	})
	if !errors.Is(err, tools.ErrToolExecutionFailed) {
		t.Fatalf("expected ErrToolExecutionFailed, got %v", err)
	}
}
