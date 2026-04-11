// 该测试文件验证平台抽象层行为。
package platform

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
)

var _ FileSystemAdapter = (*LocalFileSystemAdapter)(nil)

// TestEnsureWithinWorkspace 验证EnsureWithinWorkspace。
func TestEnsureWithinWorkspace(t *testing.T) {
	workspaceRoot := t.TempDir()
	policy, err := NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}

	insidePath := filepath.Join(workspaceRoot, "notes", "demo.md")
	if _, err := policy.EnsureWithinWorkspace(insidePath); err != nil {
		t.Fatalf("expected inside path to pass: %v", err)
	}

	workspaceRelativePath := filepath.Join("notes", "demo.md")
	resolvedRelativePath, err := policy.EnsureWithinWorkspace(workspaceRelativePath)
	if err != nil {
		t.Fatalf("expected workspace-relative path to pass: %v", err)
	}
	if resolvedRelativePath != filepath.Join(workspaceRoot, "notes", "demo.md") {
		t.Fatalf("unexpected workspace-relative path resolution: %s", resolvedRelativePath)
	}

	outsidePath := filepath.Join(workspaceRoot, "..", "outside.md")
	if _, err := policy.EnsureWithinWorkspace(outsidePath); err == nil {
		t.Fatal("expected outside path to fail")
	}
}

func TestLocalFileSystemAdapterImplementsIOFS(t *testing.T) {
	workspaceRoot := t.TempDir()
	policy, err := NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}

	adapter := NewLocalFileSystemAdapter(policy)
	if err := adapter.WriteFile(filepath.Join("notes", "demo.md"), []byte("hello workspace")); err != nil {
		t.Fatalf("write workspace document: %v", err)
	}
	if err := adapter.WriteFile(filepath.Join("notes", "extra.md"), []byte("secondary")); err != nil {
		t.Fatalf("write extra document: %v", err)
	}

	content, err := fs.ReadFile(adapter, "notes/demo.md")
	if err != nil {
		t.Fatalf("read file through io/fs: %v", err)
	}
	if string(content) != "hello workspace" {
		t.Fatalf("unexpected file content: %s", string(content))
	}

	entries, err := fs.ReadDir(adapter, "notes")
	if err != nil {
		t.Fatalf("read dir through io/fs: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected two directory entries, got %d", len(entries))
	}

	nestedFS, err := fs.Sub(adapter, "notes")
	if err != nil {
		t.Fatalf("create sub fs: %v", err)
	}
	subContent, err := fs.ReadFile(nestedFS, "demo.md")
	if err != nil {
		t.Fatalf("read file through sub fs: %v", err)
	}
	if string(subContent) != "hello workspace" {
		t.Fatalf("unexpected sub fs file content: %s", string(subContent))
	}

	if err := adapter.Move(filepath.Join("notes", "extra.md"), filepath.Join("archive", "extra.md")); err != nil {
		t.Fatalf("move workspace file: %v", err)
	}
	movedContent, err := fs.ReadFile(adapter, "archive/extra.md")
	if err != nil {
		t.Fatalf("read moved file through io/fs: %v", err)
	}
	if string(movedContent) != "secondary" {
		t.Fatalf("unexpected moved file content: %s", string(movedContent))
	}

	if _, err := fs.ReadFile(adapter, "notes/extra.md"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected moved source file to be absent, got %v", err)
	}
}

func TestLocalFileSystemAdapterRejectsInvalidFSPaths(t *testing.T) {
	workspaceRoot := t.TempDir()
	policy, err := NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}

	adapter := NewLocalFileSystemAdapter(policy)

	if _, err := fs.ReadFile(adapter, "../outside.md"); !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("expected invalid path error, got %v", err)
	}

	if _, err := fs.ReadDir(adapter, "/absolute"); !errors.Is(err, fs.ErrInvalid) {
		t.Fatalf("expected invalid directory path error, got %v", err)
	}

	if err := adapter.WriteFile(filepath.Join("..", "outside.md"), []byte("blocked")); err == nil {
		t.Fatal("expected write outside workspace to fail")
	}
}

func TestLocalOSCapabilityAdapterNamedPipeState(t *testing.T) {
	adapter := NewLocalOSCapabilityAdapter()
	if err := adapter.EnsureNamedPipe("pipe_demo"); err != nil {
		t.Fatalf("ensure named pipe: %v", err)
	}
	if !adapter.HasNamedPipe("pipe_demo") {
		t.Fatal("expected pipe to be tracked")
	}
	if err := adapter.CloseNamedPipe("pipe_demo"); err != nil {
		t.Fatalf("close named pipe: %v", err)
	}
	if adapter.HasNamedPipe("pipe_demo") {
		t.Fatal("expected pipe to be removed")
	}
}
