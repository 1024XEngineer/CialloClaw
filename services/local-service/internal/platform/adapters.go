// 该文件负责跨平台抽象接口或平台适配实现。
package platform

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

// FileSystemAdapter 定义当前模块的接口约束。
type FileSystemAdapter interface {
	fs.FS
	fs.ReadDirFS
	fs.ReadFileFS
	fs.StatFS
	fs.SubFS
	Join(parts ...string) string
	Clean(path string) string
	Abs(path string) (string, error)
	Rel(base, target string) (string, error)
	Normalize(path string) string
	EnsureWithinWorkspace(path string) (string, error)
	WriteFile(path string, content []byte) error
	Move(src, dst string) error
	MkdirAll(path string) error
}

// PathPolicy 定义当前模块的接口约束。
type PathPolicy interface {
	Normalize(path string) string
	EnsureWithinWorkspace(path string) (string, error)
}

// OSCapabilityAdapter 定义当前模块的接口约束。
type OSCapabilityAdapter interface {
	Notify(title, body string) error
	OpenExternal(target string) error
	EnsureNamedPipe(pipeName string) error
	CloseNamedPipe(pipeName string) error
}

// ExecutionBackendAdapter 定义当前模块的接口约束。
type ExecutionBackendAdapter interface {
	Name() string
	RunCommand(ctx context.Context, command string, args []string, workingDir string) (tools.CommandExecutionResult, error)
}

// StorageAdapter 定义当前模块的接口约束。
type StorageAdapter interface {
	DatabasePath() string
}

// LocalPathPolicy 定义当前模块的数据结构。
type LocalPathPolicy struct {
	workspaceRoot string
}

// NewLocalPathPolicy 创建并返回LocalPathPolicy。
func NewLocalPathPolicy(workspaceRoot string) (*LocalPathPolicy, error) {
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, err
	}

	return &LocalPathPolicy{workspaceRoot: filepath.Clean(absRoot)}, nil
}

// Normalize 处理当前模块的相关逻辑。
func (p *LocalPathPolicy) Normalize(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

// EnsureWithinWorkspace 处理当前模块的相关逻辑。
func (p *LocalPathPolicy) EnsureWithinWorkspace(path string) (string, error) {
	candidates := make([]string, 0, 2)
	if filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		candidates = append(candidates, absPath)
	} else {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return "", err
		}
		candidates = append(candidates, absPath)
		candidates = append(candidates, filepath.Join(p.workspaceRoot, path))
	}

	for _, candidate := range candidates {
		if safePath, ok := p.ensureCandidateWithinWorkspace(candidate); ok {
			return safePath, nil
		}
	}

	return "", errors.New("path outside workspace")
}

func (p *LocalPathPolicy) ensureCandidateWithinWorkspace(candidate string) (string, bool) {
	cleanTarget := filepath.Clean(candidate)
	rootWithSeparator := p.workspaceRoot + string(os.PathSeparator)

	if cleanTarget == p.workspaceRoot || strings.HasPrefix(cleanTarget, rootWithSeparator) {
		return cleanTarget, true
	}

	return "", false
}

// LocalFileSystemAdapter 定义当前模块的数据结构。
type LocalFileSystemAdapter struct {
	policy *LocalPathPolicy
}

// NewLocalFileSystemAdapter 创建并返回LocalFileSystemAdapter。
func NewLocalFileSystemAdapter(policy *LocalPathPolicy) *LocalFileSystemAdapter {
	return &LocalFileSystemAdapter{policy: policy}
}

// Open 处理当前模块的相关逻辑。
func (a *LocalFileSystemAdapter) Open(name string) (fs.File, error) {
	fsPath, err := a.resolveFSPath("open", name)
	if err != nil {
		return nil, err
	}

	return a.workspaceFS().Open(fsPath)
}

// Join 处理当前模块的相关逻辑。
func (a *LocalFileSystemAdapter) Join(parts ...string) string {
	return filepath.Join(parts...)
}

// Clean 处理当前模块的相关逻辑。
func (a *LocalFileSystemAdapter) Clean(path string) string {
	return filepath.Clean(path)
}

// Abs 处理当前模块的相关逻辑。
func (a *LocalFileSystemAdapter) Abs(path string) (string, error) {
	return filepath.Abs(path)
}

// Rel 处理当前模块的相关逻辑。
func (a *LocalFileSystemAdapter) Rel(base, target string) (string, error) {
	return filepath.Rel(base, target)
}

// Normalize 处理当前模块的相关逻辑。
func (a *LocalFileSystemAdapter) Normalize(path string) string {
	return a.policy.Normalize(path)
}

// EnsureWithinWorkspace 处理当前模块的相关逻辑。
func (a *LocalFileSystemAdapter) EnsureWithinWorkspace(path string) (string, error) {
	return a.policy.EnsureWithinWorkspace(path)
}

// ReadFile 处理当前模块的相关逻辑。
func (a *LocalFileSystemAdapter) ReadFile(path string) ([]byte, error) {
	fsPath, err := a.resolveFSPath("read", path)
	if err != nil {
		return nil, err
	}

	return fs.ReadFile(a.workspaceFS(), fsPath)
}

// ReadDir 处理当前模块的相关逻辑。
func (a *LocalFileSystemAdapter) ReadDir(path string) ([]fs.DirEntry, error) {
	fsPath, err := a.resolveFSPath("readdir", path)
	if err != nil {
		return nil, err
	}

	return fs.ReadDir(a.workspaceFS(), fsPath)
}

// Stat 处理当前模块的相关逻辑。
func (a *LocalFileSystemAdapter) Stat(path string) (fs.FileInfo, error) {
	fsPath, err := a.resolveFSPath("stat", path)
	if err != nil {
		return nil, err
	}

	return fs.Stat(a.workspaceFS(), fsPath)
}

// Sub 处理当前模块的相关逻辑。
func (a *LocalFileSystemAdapter) Sub(dir string) (fs.FS, error) {
	fsPath, err := a.resolveFSPath("sub", dir)
	if err != nil {
		return nil, err
	}

	return fs.Sub(a.workspaceFS(), fsPath)
}

// WriteFile 处理当前模块的相关逻辑。
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

// Move 处理当前模块的相关逻辑。
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

// MkdirAll 处理当前模块的相关逻辑。
func (a *LocalFileSystemAdapter) MkdirAll(path string) error {
	safePath, err := a.policy.EnsureWithinWorkspace(path)
	if err != nil {
		return err
	}

	return os.MkdirAll(safePath, 0o755)
}

func (a *LocalFileSystemAdapter) workspaceFS() fs.FS {
	return os.DirFS(a.policy.workspaceRoot)
}

func (a *LocalFileSystemAdapter) resolveFSPath(op, name string) (string, error) {
	if filepath.IsAbs(name) {
		relPath, err := filepath.Rel(a.policy.workspaceRoot, name)
		if err == nil {
			cleanRel := filepath.Clean(relPath)
			if cleanRel == "." {
				return ".", nil
			}
			if cleanRel != ".." && !strings.HasPrefix(cleanRel, ".."+string(os.PathSeparator)) {
				name = filepath.ToSlash(cleanRel)
			}
		}
	}

	return normalizeFSPath(op, filepath.ToSlash(name))
}

func normalizeFSPath(op, name string) (string, error) {
	if name == "." {
		return ".", nil
	}
	if name == "" {
		return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}

	if name != filepath.ToSlash(name) {
		return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}

	normalized := path.Clean(name)
	if normalized != name || !fs.ValidPath(normalized) {
		return "", &fs.PathError{Op: op, Path: name, Err: fs.ErrInvalid}
	}

	return normalized, nil
}

// LocalExecutionBackend 定义当前模块的数据结构。
type LocalExecutionBackend struct{}

// Name 处理当前模块的相关逻辑。
func (LocalExecutionBackend) Name() string {
	return "docker"
}

// RunCommand 执行最小受控命令。
func (LocalExecutionBackend) RunCommand(ctx context.Context, command string, args []string, workingDir string) (tools.CommandExecutionResult, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	if strings.TrimSpace(workingDir) != "" {
		cmd.Dir = workingDir
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := tools.CommandExecutionResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		return result, err
	}
	return result, nil
}

// LocalStorageAdapter 定义当前模块的数据结构。
type LocalStorageAdapter struct {
	databasePath string
}

// NewLocalStorageAdapter 创建并返回LocalStorageAdapter。
func NewLocalStorageAdapter(databasePath string) *LocalStorageAdapter {
	return &LocalStorageAdapter{databasePath: databasePath}
}

// DatabasePath 处理当前模块的相关逻辑。
func (a *LocalStorageAdapter) DatabasePath() string {
	return a.databasePath
}
