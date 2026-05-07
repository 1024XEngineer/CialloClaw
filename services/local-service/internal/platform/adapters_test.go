// This test file validates platform abstraction behavior.
package platform

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

var _ FileSystemAdapter = (*LocalFileSystemAdapter)(nil)

// TestEnsureWithinWorkspace validates workspace path containment.
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

func TestLocalFileSystemAdapterRejectsSymlinkWorkspaceEscapes(t *testing.T) {
	workspaceRoot := t.TempDir()
	outsideRoot := t.TempDir()
	policy, err := NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}
	adapter := NewLocalFileSystemAdapter(policy)

	outsideFile := filepath.Join(outsideRoot, "outside.md")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o644); err != nil {
		t.Fatalf("write outside file: %v", err)
	}
	outsideDir := filepath.Join(outsideRoot, "outside-dir")
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatalf("create outside dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outsideDir, "nested.md"), []byte("nested"), 0o644); err != nil {
		t.Fatalf("write outside nested file: %v", err)
	}

	if err := os.Symlink(outsideFile, filepath.Join(workspaceRoot, "file-link.md")); err != nil {
		t.Skipf("symlink creation is not available in this environment: %v", err)
	}
	if err := os.Symlink(outsideDir, filepath.Join(workspaceRoot, "dir-link")); err != nil {
		t.Skipf("symlink creation is not available in this environment: %v", err)
	}
	if err := adapter.WriteFile("inside.md", []byte("inside")); err != nil {
		t.Fatalf("write inside file: %v", err)
	}

	if _, err := adapter.ReadFile("file-link.md"); !errors.Is(err, ErrPathOutsideWorkspace) {
		t.Fatalf("expected read boundary error, got %v", err)
	}
	if _, err := adapter.ReadDir("dir-link"); !errors.Is(err, ErrPathOutsideWorkspace) {
		t.Fatalf("expected readdir boundary error, got %v", err)
	}
	if _, err := adapter.Stat("file-link.md"); !errors.Is(err, ErrPathOutsideWorkspace) {
		t.Fatalf("expected stat boundary error, got %v", err)
	}
	if err := adapter.WriteFile(filepath.Join("dir-link", "created.md"), []byte("blocked")); !errors.Is(err, ErrPathOutsideWorkspace) {
		t.Fatalf("expected write boundary error, got %v", err)
	}
	if err := adapter.Move("file-link.md", "moved-link.md"); !errors.Is(err, ErrPathOutsideWorkspace) {
		t.Fatalf("expected source move boundary error, got %v", err)
	}
	if err := adapter.Move("inside.md", filepath.Join("dir-link", "moved.md")); !errors.Is(err, ErrPathOutsideWorkspace) {
		t.Fatalf("expected destination move boundary error, got %v", err)
	}
	if err := adapter.Remove("file-link.md"); !errors.Is(err, ErrPathOutsideWorkspace) {
		t.Fatalf("expected remove boundary error, got %v", err)
	}
	if err := adapter.MkdirAll(filepath.Join("dir-link", "nested")); !errors.Is(err, ErrPathOutsideWorkspace) {
		t.Fatalf("expected mkdir boundary error, got %v", err)
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

func TestLocalPlatformAdaptersCoverUtilityMethods(t *testing.T) {
	workspaceRoot := t.TempDir()
	policy, err := NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}
	adapter := NewLocalFileSystemAdapter(policy)
	if adapter.Join("notes", "demo.md") != filepath.Join("notes", "demo.md") {
		t.Fatalf("unexpected join result")
	}
	if adapter.Clean(filepath.Join("notes", "..", "notes", "demo.md")) != filepath.Join("notes", "demo.md") {
		t.Fatalf("unexpected clean result")
	}
	absPath, err := adapter.Abs("notes")
	if err != nil {
		t.Fatalf("Abs returned error: %v", err)
	}
	if absPath == "" {
		t.Fatal("expected absolute path")
	}
	relPath, err := adapter.Rel(workspaceRoot, filepath.Join(workspaceRoot, "notes", "demo.md"))
	if err != nil {
		t.Fatalf("Rel returned error: %v", err)
	}
	if relPath != filepath.Join("notes", "demo.md") {
		t.Fatalf("unexpected relative path: %q", relPath)
	}
	if adapter.Normalize(filepath.Join("notes", "..", "notes", "demo.md")) != "notes/demo.md" {
		t.Fatalf("unexpected normalized path")
	}
	if _, err := adapter.EnsureWithinWorkspace(filepath.Join(workspaceRoot, "notes", "demo.md")); err != nil {
		t.Fatalf("EnsureWithinWorkspace returned error: %v", err)
	}
	if err := adapter.MkdirAll(filepath.Join(workspaceRoot, "notes")); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := adapter.WriteFile(filepath.Join(workspaceRoot, "notes", "demo.md"), []byte("demo")); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	openedFile, err := adapter.Open("notes/demo.md")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := openedFile.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if _, err := adapter.Stat(filepath.Join(workspaceRoot, "notes", "demo.md")); err != nil {
		t.Fatalf("Stat returned error: %v", err)
	}
	if err := adapter.Remove(filepath.Join(workspaceRoot, "notes", "demo.md")); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	osAdapter := NewLocalOSCapabilityAdapter()
	if err := osAdapter.Notify("title", "body"); err != nil {
		t.Fatalf("Notify returned error: %v", err)
	}
	if err := osAdapter.OpenExternal("https://example.com"); err != nil {
		t.Fatalf("OpenExternal returned error: %v", err)
	}
	if err := osAdapter.OpenExternal(""); err == nil {
		t.Fatal("expected empty target rejection")
	}
	legacyBackend := LocalExecutionBackend{}
	if legacyBackend.Name() != "local_host" {
		t.Fatalf("unexpected legacy backend name: %q", legacyBackend.Name())
	}
	result, err := legacyBackend.RunCommand(context.Background(), "go", []string{"env", "GOROOT"}, workspaceRoot)
	if err != nil {
		t.Fatalf("RunCommand returned error: %v", err)
	}
	if result.ExecutionBackend != "local_host" {
		t.Fatalf("expected local execution backend metadata, got %+v", result)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected zero exit code, got %+v", result)
	}
}

func TestLocalStorageAdapterBuildsDedicatedStrongholdPath(t *testing.T) {
	adapter := NewLocalStorageAdapter(filepath.Join("data", "cialloclaw.db"))
	if adapter.SecretStorePath() != filepath.Join("data", "cialloclaw.stronghold.db") {
		t.Fatalf("unexpected stronghold path: %q", adapter.SecretStorePath())
	}
}

func TestLocalStorageAdapterReturnsDatabaseAndExtensionlessStrongholdPaths(t *testing.T) {
	adapter := NewLocalStorageAdapter(filepath.Join("data", "cialloclaw"))
	if adapter.DatabasePath() != filepath.Join("data", "cialloclaw") {
		t.Fatalf("unexpected database path: %q", adapter.DatabasePath())
	}
	if adapter.SecretStorePath() != filepath.Join("data", "cialloclaw.stronghold.db") {
		t.Fatalf("unexpected extensionless stronghold path: %q", adapter.SecretStorePath())
	}
}
