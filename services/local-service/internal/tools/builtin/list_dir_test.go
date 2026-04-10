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

type stubDirEntry struct {
	name  string
	isDir bool
	size  int64
}

func (s stubDirEntry) Name() string      { return s.name }
func (s stubDirEntry) IsDir() bool       { return s.isDir }
func (s stubDirEntry) Type() fs.FileMode { return 0 }
func (s stubDirEntry) Info() (fs.FileInfo, error) {
	return stubDirInfo{name: s.name, size: s.size, dir: s.isDir}, nil
}

type stubDirInfo struct {
	name string
	size int64
	dir  bool
}

func (s stubDirInfo) Name() string { return s.name }
func (s stubDirInfo) Size() int64  { return s.size }
func (s stubDirInfo) Mode() fs.FileMode {
	if s.dir {
		return fs.ModeDir
	}
	return 0o644
}
func (s stubDirInfo) ModTime() time.Time { return time.Time{} }
func (s stubDirInfo) IsDir() bool        { return s.dir }
func (s stubDirInfo) Sys() any           { return nil }

type stubListDirPlatform struct {
	workspaceRoot string
	outOfScope    map[string]bool
	entries       map[string][]fs.DirEntry
	readDirErr    error
}

func newStubListDirPlatform(workspaceRoot string) *stubListDirPlatform {
	return &stubListDirPlatform{
		workspaceRoot: workspaceRoot,
		outOfScope:    make(map[string]bool),
		entries:       make(map[string][]fs.DirEntry),
	}
}

func (s *stubListDirPlatform) Join(elem ...string) string { return filepath.Join(elem...) }
func (s *stubListDirPlatform) Abs(path string) (string, error) {
	if isStubAbsolutePath(path) {
		return filepath.Clean(path), nil
	}
	return filepath.Join(s.workspaceRoot, path), nil
}
func (s *stubListDirPlatform) EnsureWithinWorkspace(path string) (string, error) {
	clean := filepath.Clean(path)
	if s.outOfScope[clean] {
		return "", errors.New("outside workspace")
	}
	if isStubAbsolutePath(clean) {
		return clean, nil
	}
	return filepath.Join(s.workspaceRoot, clean), nil
}
func (s *stubListDirPlatform) ReadDir(path string) ([]fs.DirEntry, error) {
	if s.readDirErr != nil {
		return nil, s.readDirErr
	}
	return s.entries[filepath.Clean(path)], nil
}
func (s *stubListDirPlatform) ReadFile(path string) ([]byte, error)        { return nil, fs.ErrNotExist }
func (s *stubListDirPlatform) WriteFile(path string, content []byte) error { return nil }
func (s *stubListDirPlatform) Stat(path string) (fs.FileInfo, error)       { return nil, fs.ErrNotExist }

func TestListDirToolExecuteWithinWorkspace(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubListDirPlatform(workspace)
	target := filepath.Join(workspace, "notes")
	platform.entries[target] = []fs.DirEntry{
		stubDirEntry{name: "a.txt", isDir: false, size: 12},
		stubDirEntry{name: "b", isDir: true, size: 0},
	}
	tool := NewListDirTool()

	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{"path": "notes", "limit": 10})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.RawOutput["entry_count"] != 2 || result.RawOutput["returned_count"] != 2 {
		t.Fatalf("unexpected list output: %+v", result.RawOutput)
	}
	entries := result.RawOutput["entries"].([]map[string]any)
	if len(entries) != 2 || entries[0]["name"] != "a.txt" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestListDirToolRejectsOutsideWorkspace(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubListDirPlatform(workspace)
	outside := filepath.Clean("D:/outside")
	platform.outOfScope[outside] = true
	tool := NewListDirTool()

	_, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{"path": outside})
	if !errors.Is(err, tools.ErrWorkspaceBoundaryDenied) {
		t.Fatalf("expected ErrWorkspaceBoundaryDenied, got %v", err)
	}
}

func TestListDirToolReturnsAdapterError(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubListDirPlatform(workspace)
	platform.readDirErr = errors.New("adapter failed")
	tool := NewListDirTool()

	_, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{"path": "notes"})
	if !errors.Is(err, tools.ErrToolExecutionFailed) {
		t.Fatalf("expected ErrToolExecutionFailed, got %v", err)
	}
}

func TestListDirToolLimitTruncatesEntries(t *testing.T) {
	workspace := filepath.Clean("D:/workspace")
	platform := newStubListDirPlatform(workspace)
	target := filepath.Join(workspace, "notes")
	platform.entries[target] = []fs.DirEntry{
		stubDirEntry{name: "a.txt", isDir: false, size: 12},
		stubDirEntry{name: "b.txt", isDir: false, size: 13},
		stubDirEntry{name: "c.txt", isDir: false, size: 14},
	}
	tool := NewListDirTool()

	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{WorkspacePath: workspace, Platform: platform}, map[string]any{"path": "notes", "limit": 2})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.RawOutput["returned_count"] != 2 || result.RawOutput["truncated"] != true {
		t.Fatalf("unexpected truncation output: %+v", result.RawOutput)
	}
}
