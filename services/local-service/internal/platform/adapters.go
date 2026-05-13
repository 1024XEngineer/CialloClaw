// This file implements cross-platform abstraction interfaces and local adapters.
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
	"runtime"
	"strings"
	"sync"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

// FileSystemAdapter defines the policy-bounded file-system adapter used by
// local services.
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
	Remove(path string) error
	Move(src, dst string) error
	MkdirAll(path string) error
}

// PathPolicy defines the lexical and real-path boundary checks shared by local
// file adapters. Implementations may represent a strict workspace root or a
// broader tool-specific allow/deny policy.
type PathPolicy interface {
	Normalize(path string) string
	EnsureWithinWorkspace(path string) (string, error)
	ResolveExisting(path string) (string, error)
}

// OSCapabilityAdapter defines the minimal host OS capability boundary.
type OSCapabilityAdapter interface {
	Notify(title, body string) error
	OpenExternal(target string) error
	EnsureNamedPipe(pipeName string) error
	CloseNamedPipe(pipeName string) error
}

// ExecutionBackendAdapter defines the command execution backend boundary.
type ExecutionBackendAdapter interface {
	Name() string
	RunCommand(ctx context.Context, command string, args []string, workingDir string) (tools.CommandExecutionResult, error)
}

// StorageAdapter defines storage path accessors for local persistence.
type StorageAdapter interface {
	DatabasePath() string
	SecretStorePath() string
}

// LocalPathPolicy validates workspace paths against lexical and real-path escapes.
type LocalPathPolicy struct {
	workspaceRoot     string
	realWorkspaceRoot string
}

// NewLocalPathPolicy creates a local path policy for the provided workspace root.
func NewLocalPathPolicy(workspaceRoot string) (*LocalPathPolicy, error) {
	absRoot, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return nil, err
	}

	cleanRoot := filepath.Clean(absRoot)
	realRoot, err := filepath.EvalSymlinks(cleanRoot)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		realRoot = cleanRoot
	}

	return &LocalPathPolicy{workspaceRoot: cleanRoot, realWorkspaceRoot: filepath.Clean(realRoot)}, nil
}

// Normalize returns a slash-separated clean path.
func (p *LocalPathPolicy) Normalize(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

// ErrPathOutsideWorkspace marks paths that escape the configured workspace boundary.
var ErrPathOutsideWorkspace = errors.New("path outside workspace")

// EnsureWithinWorkspace resolves path against the workspace and rejects symlink escapes.
func (p *LocalPathPolicy) EnsureWithinWorkspace(path string) (string, error) {
	return p.resolveWorkspacePath(path, false)
}

// ResolveExisting confirms an existing path remains inside the workspace after symlink evaluation.
func (p *LocalPathPolicy) ResolveExisting(path string) (string, error) {
	return p.resolveWorkspacePath(path, true)
}

func (p *LocalPathPolicy) resolveWorkspacePath(path string, requireExisting bool) (string, error) {
	var lastErr error
	for _, candidate := range p.workspaceCandidates(path) {
		safePath, err := p.ensureCandidateWithinWorkspace(candidate, requireExisting)
		if err == nil {
			return safePath, nil
		}
		lastErr = err
	}
	if lastErr != nil && !errors.Is(lastErr, ErrPathOutsideWorkspace) {
		return "", lastErr
	}
	return "", ErrPathOutsideWorkspace
}

func (p *LocalPathPolicy) workspaceCandidates(path string) []string {
	candidates := make([]string, 0, 2)
	if filepath.IsAbs(path) {
		candidates = append(candidates, filepath.Clean(path))
		return candidates
	}

	if absPath, err := filepath.Abs(path); err == nil {
		candidates = append(candidates, filepath.Clean(absPath))
	}
	candidates = append(candidates, filepath.Clean(filepath.Join(p.workspaceRoot, path)))
	return candidates
}

func (p *LocalPathPolicy) ensureCandidateWithinWorkspace(candidate string, requireExisting bool) (string, error) {
	cleanTarget := filepath.Clean(candidate)
	if !pathWithinRoot(cleanTarget, p.workspaceRoot) {
		return "", ErrPathOutsideWorkspace
	}

	realProbe, err := p.realPathForBoundaryCheck(cleanTarget, requireExisting)
	if err != nil {
		return "", err
	}
	if !pathWithinRoot(realProbe, p.realWorkspaceRoot) {
		return "", ErrPathOutsideWorkspace
	}

	return cleanTarget, nil
}

func (p *LocalPathPolicy) realPathForBoundaryCheck(cleanTarget string, requireExisting bool) (string, error) {
	realTarget, err := filepath.EvalSymlinks(cleanTarget)
	if err == nil {
		return filepath.Clean(realTarget), nil
	}
	if requireExisting || !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if cleanTarget == p.workspaceRoot {
		return p.realWorkspaceRoot, nil
	}

	probe := filepath.Dir(cleanTarget)
	for {
		realProbe, probeErr := filepath.EvalSymlinks(probe)
		if probeErr == nil {
			return filepath.Clean(realProbe), nil
		}
		if !errors.Is(probeErr, fs.ErrNotExist) {
			return "", probeErr
		}
		if filepath.Clean(probe) == p.workspaceRoot {
			return p.realWorkspaceRoot, nil
		}

		parent := filepath.Dir(probe)
		if parent == probe {
			return "", ErrPathOutsideWorkspace
		}
		probe = parent
	}
}

// LocalToolPathPolicy expands local tool access beyond the workspace while
// still denying protected system roots and sibling user-profile roots. Relative
// tool paths stay anchored to the workspace so existing task flows preserve
// their workspace-first behavior, while `~/Desktop`, `~/Documents`, and
// `~/Downloads` resolve through the current OS known-folder mapping.
type LocalToolPathPolicy struct {
	workspaceRoot     string
	realWorkspaceRoot string
	deniedRoots       []policyRoot
	knownFolders      map[string]string
}

type policyRoot struct {
	clean string
	real  string
}

// NewLocalToolPathPolicy creates the policy used by high-permission local file
// tools. Absolute paths may target any non-protected local folder, relative
// paths continue resolving from the workspace root, and OS-known-folder aliases
// keep redirected Desktop/Documents/Downloads paths stable for end users.
func NewLocalToolPathPolicy(workspaceRoot string) (*LocalToolPathPolicy, error) {
	cleanRoot, realRoot, err := resolvePolicyRoot(workspaceRoot)
	if err != nil {
		return nil, err
	}

	deniedRoots := make([]policyRoot, 0)
	for _, root := range protectedToolRoots(runtime.GOOS) {
		if policyRoot, ok := resolveOptionalPolicyRoot(root); ok {
			deniedRoots = append(deniedRoots, policyRoot)
		}
	}
	for _, root := range siblingProfileRoots() {
		if policyRoot, ok := resolveOptionalPolicyRoot(root); ok {
			deniedRoots = append(deniedRoots, policyRoot)
		}
	}

	return &LocalToolPathPolicy{
		workspaceRoot:     cleanRoot,
		realWorkspaceRoot: realRoot,
		deniedRoots:       uniquePolicyRoots(deniedRoots),
		knownFolders:      loadKnownFoldersForToolPolicy(),
	}, nil
}

// Normalize returns a slash-separated clean path.
func (p *LocalToolPathPolicy) Normalize(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

// EnsureWithinWorkspace validates the requested tool path against the broader
// local-tool policy boundary.
func (p *LocalToolPathPolicy) EnsureWithinWorkspace(path string) (string, error) {
	return p.resolveToolPath(path, false)
}

// ResolveExisting validates an already-existing path against the broader local
// tool boundary before read-only operations open it.
func (p *LocalToolPathPolicy) ResolveExisting(path string) (string, error) {
	return p.resolveToolPath(path, true)
}

func (p *LocalToolPathPolicy) resolveToolPath(path string, requireExisting bool) (string, error) {
	if p == nil {
		return "", ErrPathOutsideWorkspace
	}
	trimmed := strings.TrimSpace(path)
	candidate := filepath.Clean(filepath.Join(p.workspaceRoot, trimmed))
	if knownFolderPath, ok := resolveToolKnownFolderAlias(trimmed, p.knownFolders); ok {
		candidate = knownFolderPath
	} else if filepath.IsAbs(trimmed) {
		candidate = filepath.Clean(trimmed)
	}
	if p.isDeniedPath(candidate, false) {
		return "", ErrPathOutsideWorkspace
	}

	realProbe, err := realPathForBoundaryCheck(candidate, requireExisting, p.realWorkspaceRoot)
	if err != nil {
		return "", err
	}
	if p.isDeniedPath(realProbe, true) {
		return "", ErrPathOutsideWorkspace
	}
	return candidate, nil
}

func (p *LocalToolPathPolicy) isDeniedPath(candidate string, real bool) bool {
	cleanCandidate := filepath.Clean(candidate)
	for _, root := range p.deniedRoots {
		boundary := root.clean
		if real {
			boundary = root.real
		}
		if boundary != "" && pathWithinRoot(cleanCandidate, boundary) {
			return true
		}
	}
	return false
}

func resolvePolicyRoot(root string) (string, string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	cleanRoot := filepath.Clean(absRoot)
	realRoot, err := filepath.EvalSymlinks(cleanRoot)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return "", "", err
		}
		realRoot = cleanRoot
	}
	return cleanRoot, filepath.Clean(realRoot), nil
}

func resolveOptionalPolicyRoot(root string) (policyRoot, bool) {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		return policyRoot{}, false
	}
	cleanRoot, realRoot, err := resolvePolicyRoot(trimmed)
	if err != nil {
		return policyRoot{}, false
	}
	return policyRoot{clean: cleanRoot, real: realRoot}, true
}

func uniquePolicyRoots(roots []policyRoot) []policyRoot {
	if len(roots) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(roots))
	unique := make([]policyRoot, 0, len(roots))
	for _, root := range roots {
		key := root.clean + "\n" + root.real
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, root)
	}
	return unique
}

func protectedToolRoots(goos string) []string {
	switch goos {
	case "windows":
		return []string{
			os.Getenv("WINDIR"),
			os.Getenv("SystemRoot"),
			os.Getenv("ProgramFiles"),
			os.Getenv("ProgramFiles(x86)"),
			os.Getenv("ProgramW6432"),
			os.Getenv("ProgramData"),
		}
	case "darwin":
		return []string{"/Applications", "/Library", "/System", "/bin", "/sbin", "/usr", "/private/etc", "/private/var/db"}
	default:
		return []string{"/bin", "/boot", "/dev", "/etc", "/lib", "/lib64", "/proc", "/root", "/run", "/sbin", "/sys", "/usr", "/var/lib"}
	}
}

func siblingProfileRoots() []string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	homeDir = strings.TrimSpace(homeDir)
	if homeDir == "" {
		return nil
	}
	homeDir = filepath.Clean(homeDir)
	parentDir := filepath.Dir(homeDir)
	if parentDir == homeDir {
		return nil
	}
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		return nil
	}
	roots := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		candidate := filepath.Join(parentDir, entry.Name())
		if filepath.Clean(candidate) == homeDir {
			continue
		}
		roots = append(roots, candidate)
	}
	return roots
}

func realPathForBoundaryCheck(cleanTarget string, requireExisting bool, fallbackRoot string) (string, error) {
	realTarget, err := filepath.EvalSymlinks(cleanTarget)
	if err == nil {
		return filepath.Clean(realTarget), nil
	}
	if requireExisting || !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if cleanTarget == fallbackRoot {
		return filepath.Clean(fallbackRoot), nil
	}

	probe := filepath.Dir(cleanTarget)
	for {
		realProbe, probeErr := filepath.EvalSymlinks(probe)
		if probeErr == nil {
			return filepath.Clean(realProbe), nil
		}
		if !errors.Is(probeErr, fs.ErrNotExist) {
			return "", probeErr
		}
		if filepath.Clean(probe) == filepath.Clean(fallbackRoot) {
			return filepath.Clean(fallbackRoot), nil
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return filepath.Clean(probe), nil
		}
		probe = parent
	}
}

func pathWithinRoot(target, root string) bool {
	cleanTarget := filepath.Clean(target)
	cleanRoot := filepath.Clean(root)
	rootWithSeparator := cleanRoot + string(os.PathSeparator)
	return cleanTarget == cleanRoot || strings.HasPrefix(cleanTarget, rootWithSeparator)
}

// LocalFileSystemAdapter implements filesystem operations constrained by the
// injected path policy.
type LocalFileSystemAdapter struct {
	policy PathPolicy
}

// NewLocalFileSystemAdapter creates a filesystem adapter backed by the supplied
// path policy.
func NewLocalFileSystemAdapter(policy PathPolicy) *LocalFileSystemAdapter {
	return &LocalFileSystemAdapter{policy: policy}
}

// Open validates and opens an existing path without following symlinks outside
// the configured policy boundary.
func (a *LocalFileSystemAdapter) Open(name string) (fs.File, error) {
	safePath, err := a.resolveExistingBoundedPath("open", name)
	if err != nil {
		return nil, err
	}

	return os.Open(safePath)
}

// Join combines path components using the host path separator.
func (a *LocalFileSystemAdapter) Join(parts ...string) string {
	return filepath.Join(parts...)
}

// Clean normalizes a host path without resolving symlinks.
func (a *LocalFileSystemAdapter) Clean(path string) string {
	return filepath.Clean(path)
}

// Abs returns the absolute host path for the input.
func (a *LocalFileSystemAdapter) Abs(path string) (string, error) {
	return filepath.Abs(path)
}

// Rel returns a relative path from base to target.
func (a *LocalFileSystemAdapter) Rel(base, target string) (string, error) {
	return filepath.Rel(base, target)
}

// Normalize returns a slash-separated clean path.
func (a *LocalFileSystemAdapter) Normalize(path string) string {
	return a.policy.Normalize(path)
}

// EnsureWithinWorkspace applies the adapter path policy.
func (a *LocalFileSystemAdapter) EnsureWithinWorkspace(path string) (string, error) {
	return a.policy.EnsureWithinWorkspace(path)
}

// ReadFile validates the real target path before reading content.
func (a *LocalFileSystemAdapter) ReadFile(path string) ([]byte, error) {
	safePath, err := a.resolveExistingBoundedPath("read", path)
	if err != nil {
		return nil, err
	}

	return os.ReadFile(safePath)
}

// ReadDir validates the real directory path before listing directory entries.
func (a *LocalFileSystemAdapter) ReadDir(path string) ([]fs.DirEntry, error) {
	safePath, err := a.resolveExistingBoundedPath("readdir", path)
	if err != nil {
		return nil, err
	}

	return os.ReadDir(safePath)
}

// Stat validates the real target path before returning metadata.
func (a *LocalFileSystemAdapter) Stat(path string) (fs.FileInfo, error) {
	safePath, err := a.resolveExistingBoundedPath("stat", path)
	if err != nil {
		return nil, err
	}

	return os.Stat(safePath)
}

// Sub creates a bounded adapter rooted at an existing validated subdirectory.
func (a *LocalFileSystemAdapter) Sub(dir string) (fs.FS, error) {
	safePath, err := a.resolveExistingBoundedPath("sub", dir)
	if err != nil {
		return nil, err
	}

	policy, err := NewLocalPathPolicy(safePath)
	if err != nil {
		return nil, err
	}
	return NewLocalFileSystemAdapter(policy), nil
}

// WriteFile validates the destination and existing parent chain before writing content.
func (a *LocalFileSystemAdapter) WriteFile(path string, content []byte) error {
	safePath, err := a.policy.EnsureWithinWorkspace(path)
	if err != nil {
		return err
	}

	safeDir, err := a.policy.EnsureWithinWorkspace(filepath.Dir(safePath))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(safeDir, 0o755); err != nil {
		return err
	}

	return os.WriteFile(safePath, content, 0o644)
}

// Remove validates the existing real target before deleting content.
func (a *LocalFileSystemAdapter) Remove(path string) error {
	safePath, err := a.policy.ResolveExisting(path)
	if err != nil {
		return err
	}
	return os.Remove(safePath)
}

// Move validates both the existing source and destination parent chain before renaming.
func (a *LocalFileSystemAdapter) Move(src, dst string) error {
	safeSrc, err := a.policy.ResolveExisting(src)
	if err != nil {
		return err
	}

	safeDst, err := a.policy.EnsureWithinWorkspace(dst)
	if err != nil {
		return err
	}

	safeDir, err := a.policy.EnsureWithinWorkspace(filepath.Dir(safeDst))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(safeDir, 0o755); err != nil {
		return err
	}

	return os.Rename(safeSrc, safeDst)
}

// MkdirAll validates the requested directory and existing parent chain before creating directories.
func (a *LocalFileSystemAdapter) MkdirAll(path string) error {
	safePath, err := a.policy.EnsureWithinWorkspace(path)
	if err != nil {
		return err
	}

	return os.MkdirAll(safePath, 0o755)
}

func (a *LocalFileSystemAdapter) resolveExistingBoundedPath(op, name string) (string, error) {
	if filepath.IsAbs(name) {
		return a.policy.ResolveExisting(name)
	}

	fsPath, err := normalizeFSPath(op, filepath.ToSlash(name))
	if err != nil {
		return "", err
	}
	return a.policy.ResolveExisting(filepath.FromSlash(fsPath))
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

// LocalExecutionBackend runs commands on the local host.
type LocalExecutionBackend struct{}

// Name returns the execution backend identifier.
func (LocalExecutionBackend) Name() string {
	return "local_host"
}

// RunCommand executes a minimally controlled local command.
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
		Stdout:           stdout.String(),
		Stderr:           stderr.String(),
		ExecutionBackend: (LocalExecutionBackend{}).Name(),
	}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		return result, err
	}
	return result, nil
}

// LocalOSCapabilityAdapter is the current minimal local OS capability implementation.
//
// It does not manage a complete sidecar lifecycle yet and only provides:
// - non-empty named pipe validation
// - minimal in-process state tracking
// - local no-op or minimized host behavior placeholders
type LocalOSCapabilityAdapter struct {
	mu          sync.Mutex
	openedPipes map[string]struct{}
}

// NewLocalOSCapabilityAdapter creates the minimal OS capability adapter.
func NewLocalOSCapabilityAdapter() *LocalOSCapabilityAdapter {
	return &LocalOSCapabilityAdapter{openedPipes: make(map[string]struct{})}
}

// Notify is the current minimal no-op implementation.
func (a *LocalOSCapabilityAdapter) Notify(title, body string) error {
	_ = title
	_ = body
	return nil
}

// OpenExternal is the current minimal no-op implementation.
func (a *LocalOSCapabilityAdapter) OpenExternal(target string) error {
	if strings.TrimSpace(target) == "" {
		return errors.New("target is required")
	}
	return nil
}

// EnsureNamedPipe records a named pipe as declared available.
func (a *LocalOSCapabilityAdapter) EnsureNamedPipe(pipeName string) error {
	if strings.TrimSpace(pipeName) == "" {
		return errors.New("pipe name is required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.openedPipes[pipeName] = struct{}{}
	return nil
}

// CloseNamedPipe removes a named pipe from the minimal local state.
func (a *LocalOSCapabilityAdapter) CloseNamedPipe(pipeName string) error {
	if strings.TrimSpace(pipeName) == "" {
		return errors.New("pipe name is required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.openedPipes, pipeName)
	return nil
}

// HasNamedPipe supports tests and minimal upper-layer probes.
func (a *LocalOSCapabilityAdapter) HasNamedPipe(pipeName string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.openedPipes[pipeName]
	return ok
}

// LocalStorageAdapter stores local persistence paths.
type LocalStorageAdapter struct {
	databasePath string
}

// NewLocalStorageAdapter creates a local storage adapter.
func NewLocalStorageAdapter(databasePath string) *LocalStorageAdapter {
	return &LocalStorageAdapter{databasePath: databasePath}
}

// DatabasePath returns the configured database path.
func (a *LocalStorageAdapter) DatabasePath() string {
	return a.databasePath
}

// SecretStorePath returns the dedicated Stronghold-compatible secret store path.
func (a *LocalStorageAdapter) SecretStorePath() string {
	trimmed := strings.TrimSpace(a.databasePath)
	if trimmed == "" {
		return ""
	}
	ext := filepath.Ext(trimmed)
	if ext == "" {
		return trimmed + ".stronghold.db"
	}
	return strings.TrimSuffix(trimmed, ext) + ".stronghold" + ext
}
