//go:build windows

package taskinspector

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
)

func TestSourceToFSPathNormalizesWindowsVirtualWorkspacePaths(t *testing.T) {
	fsPath, err := sourceToFSPath(nil, `workspace\notes`)
	if err != nil || fsPath != "notes" {
		t.Fatalf("expected backslash workspace source to normalize to notes, path=%q err=%v", fsPath, err)
	}
	rootPath, err := sourceToFSPath(nil, `\workspace`)
	if err != nil || rootPath != "." {
		t.Fatalf("expected windows workspace root to normalize to dot, path=%q err=%v", rootPath, err)
	}
	_, err = sourceToFSPath(nil, `\workspace\..\outside`)
	if !errors.Is(err, ErrInspectionSourceOutsideWorkspace) {
		t.Fatalf("expected windows workspace escape path to be rejected, got %v", err)
	}
}

func TestSourceToFSPathAcceptsWindowsAbsoluteWorkspaceSources(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("NewLocalPathPolicy returned error: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	absoluteSource := filepath.Join(workspaceRoot, "todos")

	fsPath, err := sourceToFSPath(nil, absoluteSource)
	if !errors.Is(err, ErrInspectionFileSystemUnavailable) {
		t.Fatalf("expected windows absolute source without filesystem to require binding, path=%q err=%v", fsPath, err)
	}
	workspaceRelativeSlash, err := sourceToFSPath(fileSystem, filepath.ToSlash(filepath.Clean(absoluteSource)))
	if err != nil || workspaceRelativeSlash != "todos" {
		t.Fatalf("expected slash-normalized windows absolute source inside workspace to resolve to todos, path=%q err=%v", workspaceRelativeSlash, err)
	}
	workspaceRelative, err := sourceToFSPath(fileSystem, absoluteSource)
	if err != nil || workspaceRelative != "todos" {
		t.Fatalf("expected windows absolute source inside workspace to resolve to todos, path=%q err=%v", workspaceRelative, err)
	}
	subPath, err := sourceToFSPath(fileSystem, `sub\a.md`)
	if err != nil || subPath != "sub/a.md" {
		t.Fatalf("expected windows relative source to normalize to slash form, path=%q err=%v", subPath, err)
	}
	_, err = sourceToFSPath(fileSystem, `workspace/..\evil`)
	if !errors.Is(err, ErrInspectionSourceOutsideWorkspace) {
		t.Fatalf("expected mixed-separator workspace escape to be rejected, got %v", err)
	}
	_, err = sourceToFSPath(fileSystem, `..\evil`)
	if !errors.Is(err, ErrInspectionSourceOutsideWorkspace) {
		t.Fatalf("expected windows parent traversal to be rejected, got %v", err)
	}
	_, err = sourceToFSPath(fileSystem, filepath.Join(t.TempDir(), "outside"))
	if !errors.Is(err, ErrInspectionSourceOutsideWorkspace) {
		t.Fatalf("expected windows absolute source outside workspace to be rejected, got %v", err)
	}
}
