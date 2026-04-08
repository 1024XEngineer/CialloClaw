package platform

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type FileSystemAdapter interface {
	Join(parts ...string) string
	Clean(path string) string
	Abs(path string) (string, error)
	Rel(base, target string) (string, error)
	Normalize(path string) string
	EnsureWithinWorkspace(path string) (string, error)
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, content []byte) error
	Move(src, dst string) error
	MkdirAll(path string) error
}

type PathPolicy interface {
	Normalize(path string) string
	EnsureWithinWorkspace(path string) (string, error)
}

type OSCapabilityAdapter interface {
	Notify(title, body string) error
	OpenExternal(target string) error
	EnsureNamedPipe(pipeName string) error
	CloseNamedPipe(pipeName string) error
}

type ExecutionBackendAdapter interface {
	Name() string
}

type StorageAdapter interface {
	DatabasePath() string
}

type LocalPathPolicy struct {
	workspaceRoot string
}

func NewLocalPathPolicy(workspaceRoot string) (*LocalPathPolicy, error) {
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, err
	}

	return &LocalPathPolicy{workspaceRoot: filepath.Clean(absRoot)}, nil
}

func (p *LocalPathPolicy) Normalize(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

func (p *LocalPathPolicy) EnsureWithinWorkspace(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	cleanTarget := filepath.Clean(absPath)
	rootWithSeparator := p.workspaceRoot + string(os.PathSeparator)

	if cleanTarget == p.workspaceRoot || strings.HasPrefix(cleanTarget, rootWithSeparator) {
		return cleanTarget, nil
	}

	return "", errors.New("path outside workspace")
}

type LocalFileSystemAdapter struct {
	policy *LocalPathPolicy
}

func NewLocalFileSystemAdapter(policy *LocalPathPolicy) *LocalFileSystemAdapter {
	return &LocalFileSystemAdapter{policy: policy}
}

func (a *LocalFileSystemAdapter) Join(parts ...string) string {
	return filepath.Join(parts...)
}

func (a *LocalFileSystemAdapter) Clean(path string) string {
	return filepath.Clean(path)
}

func (a *LocalFileSystemAdapter) Abs(path string) (string, error) {
	return filepath.Abs(path)
}

func (a *LocalFileSystemAdapter) Rel(base, target string) (string, error) {
	return filepath.Rel(base, target)
}

func (a *LocalFileSystemAdapter) Normalize(path string) string {
	return a.policy.Normalize(path)
}

func (a *LocalFileSystemAdapter) EnsureWithinWorkspace(path string) (string, error) {
	return a.policy.EnsureWithinWorkspace(path)
}

func (a *LocalFileSystemAdapter) ReadFile(path string) ([]byte, error) {
	safePath, err := a.policy.EnsureWithinWorkspace(path)
	if err != nil {
		return nil, err
	}

	return os.ReadFile(safePath)
}

func (a *LocalFileSystemAdapter) WriteFile(path string, content []byte) error {
	safePath, err := a.policy.EnsureWithinWorkspace(path)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(safePath), 0o755); err != nil {
		return err
	}

	return os.WriteFile(safePath, content, 0o644)
}

func (a *LocalFileSystemAdapter) Move(src, dst string) error {
	safeSrc, err := a.policy.EnsureWithinWorkspace(src)
	if err != nil {
		return err
	}

	safeDst, err := a.policy.EnsureWithinWorkspace(dst)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(safeDst), 0o755); err != nil {
		return err
	}

	return os.Rename(safeSrc, safeDst)
}

func (a *LocalFileSystemAdapter) MkdirAll(path string) error {
	safePath, err := a.policy.EnsureWithinWorkspace(path)
	if err != nil {
		return err
	}

	return os.MkdirAll(safePath, 0o755)
}

type LocalExecutionBackend struct{}

func (LocalExecutionBackend) Name() string {
	return "docker"
}

type LocalStorageAdapter struct {
	databasePath string
}

func NewLocalStorageAdapter(databasePath string) *LocalStorageAdapter {
	return &LocalStorageAdapter{databasePath: databasePath}
}

func (a *LocalStorageAdapter) DatabasePath() string {
	return a.databasePath
}
