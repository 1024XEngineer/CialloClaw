package platform

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

const (
	defaultDockerSandboxImage      = "docker.io/library/golang:1.26-bookworm"
	dockerSandboxMountPath         = "/workspace"
	dockerSandboxHomePath          = "/tmp/home"
	dockerSandboxGoCachePath       = "/tmp/go-cache"
	dockerSandboxGoModCachePath    = "/tmp/go-mod-cache"
	dockerSandboxGoTmpPath         = "/tmp/go-tmp"
	dockerSandboxXDGCachePath      = "/tmp/xdg-cache"
	dockerSandboxDefaultCPU        = "1.0"
	dockerSandboxDefaultMemory     = "512m"
	dockerSandboxDefaultPIDsLimit  = 128
	dockerSandboxDefaultTmpfsBytes = 134217728
	dockerSandboxCleanupTimeout    = 5 * time.Second
)

var defaultDockerSandboxCommands = map[string]struct{}{
	"bash": {},
	"git":  {},
	"go":   {},
	"make": {},
	"sh":   {},
}

var defaultControlledLocalCommands = map[string]struct{}{
	"cmd":        {},
	"powershell": {},
	"pwsh":       {},
}

var (
	// ErrDockerSandboxUnavailable reports that the Docker CLI or runtime is not available.
	ErrDockerSandboxUnavailable = errors.New("docker sandbox unavailable")
	// ErrDockerSandboxCommandNotAllowed reports that the requested command is outside the sandbox allowlist.
	ErrDockerSandboxCommandNotAllowed = errors.New("docker sandbox command not allowed")
	// ErrDockerSandboxWorkingDirRequired reports that sandbox execution requires an explicit working directory.
	ErrDockerSandboxWorkingDirRequired = errors.New("docker sandbox working directory required")
	// ErrDockerSandboxWorkingDirOutsideWorkspace reports that the working directory escapes the mounted workspace root.
	ErrDockerSandboxWorkingDirOutsideWorkspace = errors.New("docker sandbox working directory outside workspace")
)

// DockerSandboxLimits defines the resource ceilings enforced for one sandbox run.
type DockerSandboxLimits struct {
	CPU       string
	Memory    string
	PIDsLimit int
	TmpfsSize int64
}

// DockerSandboxExecutionBackend routes command execution through a Docker sandbox.
//
// The backend mounts the workspace read-only, disables networking, drops Linux
// capabilities, and runs the requested command under explicit CPU/memory/pid
// limits. When the execution context is cancelled, it also issues a best-effort
// `docker rm -f` cleanup to keep the run recoverable.
type DockerSandboxExecutionBackend struct {
	workspaceRoot string
	image         string
	limits        DockerSandboxLimits
	allowed       map[string]struct{}
	runner        dockerSandboxRunner
	nameGenerator func() string
	sequence      atomic.Uint64
}

type dockerSandboxRunner interface {
	RunDocker(ctx context.Context, args []string) (tools.CommandExecutionResult, error)
	RemoveContainer(ctx context.Context, containerName string) error
}

type dockerCLIRunner struct{}

// ControlledExecutionBackend routes commands either to Docker sandbox execution
// or to a constrained local backend for Windows shell commands that cannot run
// inside the Linux container image.
type ControlledExecutionBackend struct {
	sandbox       *DockerSandboxExecutionBackend
	local         LocalExecutionBackend
	localCommands map[string]struct{}
}

// NewDockerSandboxExecutionBackend creates a Docker-backed command executor.
func NewDockerSandboxExecutionBackend(workspaceRoot string) *DockerSandboxExecutionBackend {
	backend := &DockerSandboxExecutionBackend{
		workspaceRoot: strings.TrimSpace(workspaceRoot),
		image:         dockerSandboxImage(),
		limits: DockerSandboxLimits{
			CPU:       dockerSandboxDefaultCPU,
			Memory:    dockerSandboxDefaultMemory,
			PIDsLimit: dockerSandboxDefaultPIDsLimit,
			TmpfsSize: dockerSandboxDefaultTmpfsBytes,
		},
		allowed: cloneDockerSandboxAllowlist(defaultDockerSandboxCommands),
		runner:  dockerCLIRunner{},
	}
	backend.nameGenerator = backend.defaultContainerName
	return backend
}

// NewControlledExecutionBackend creates the default command executor used by the
// service. Linux-friendly developer commands prefer Docker sandbox execution,
// while Windows shell commands keep using the controlled host path.
func NewControlledExecutionBackend(workspaceRoot string) *ControlledExecutionBackend {
	return &ControlledExecutionBackend{
		sandbox:       NewDockerSandboxExecutionBackend(workspaceRoot),
		local:         LocalExecutionBackend{},
		localCommands: cloneDockerSandboxAllowlist(defaultControlledLocalCommands),
	}
}

// Name reports the routed backend identifier.
func (*ControlledExecutionBackend) Name() string {
	return "controlled"
}

// RunCommand routes one command to the correct controlled execution path.
func (b *ControlledExecutionBackend) RunCommand(ctx context.Context, command string, args []string, workingDir string) (tools.CommandExecutionResult, error) {
	commandName := normalizeExecutionCommandName(command)
	if _, ok := b.localCommands[commandName]; ok {
		return b.local.RunCommand(ctx, command, args, workingDir)
	}
	return b.sandbox.RunCommand(ctx, command, args, workingDir)
}

// Name reports the execution backend identifier used by audit and tool traces.
func (*DockerSandboxExecutionBackend) Name() string {
	return "docker_sandbox"
}

// RunCommand executes one allowlisted command inside a constrained Docker sandbox.
func (b *DockerSandboxExecutionBackend) RunCommand(ctx context.Context, command string, args []string, workingDir string) (tools.CommandExecutionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	containerCommand, err := b.normalizeCommand(command)
	if err != nil {
		return tools.CommandExecutionResult{}, err
	}
	workspaceRoot, containerWorkingDir, err := b.resolveWorkspace(workingDir)
	if err != nil {
		return tools.CommandExecutionResult{}, err
	}
	containerName := b.nameGenerator()
	result, runErr := b.runner.RunDocker(ctx, b.buildDockerArgs(containerName, workspaceRoot, containerWorkingDir, containerCommand, args))
	result.ExecutionBackend = b.Name()
	result.SandboxContainer = containerName
	result.SandboxImage = b.image
	if runErr != nil {
		if errors.Is(runErr, exec.ErrNotFound) {
			return result, fmt.Errorf("%w: docker cli not found", ErrDockerSandboxUnavailable)
		}
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			result.Interrupted = true
			b.cleanupContainer(containerName)
		}
		return result, runErr
	}
	return result, nil
}

func (b *DockerSandboxExecutionBackend) resolveWorkspace(workingDir string) (string, string, error) {
	trimmedWorkingDir := strings.TrimSpace(workingDir)
	if trimmedWorkingDir == "" {
		return "", "", ErrDockerSandboxWorkingDirRequired
	}
	absoluteWorkingDir, err := filepath.Abs(trimmedWorkingDir)
	if err != nil {
		return "", "", err
	}
	workspaceRoot := strings.TrimSpace(b.workspaceRoot)
	if workspaceRoot == "" {
		workspaceRoot = absoluteWorkingDir
	}
	workspaceRoot, err = filepath.Abs(workspaceRoot)
	if err != nil {
		return "", "", err
	}
	relPath, err := filepath.Rel(workspaceRoot, absoluteWorkingDir)
	if err != nil {
		return "", "", err
	}
	if relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
		return "", "", ErrDockerSandboxWorkingDirOutsideWorkspace
	}
	containerWorkingDir := dockerSandboxMountPath
	if relPath != "." {
		containerWorkingDir = path.Join(dockerSandboxMountPath, filepath.ToSlash(relPath))
	}
	return workspaceRoot, containerWorkingDir, nil
}

func (b *DockerSandboxExecutionBackend) buildDockerArgs(containerName, workspaceRoot, containerWorkingDir, command string, args []string) []string {
	dockerArgs := []string{
		"run",
		"--rm",
		"--name", containerName,
		"--network", "none",
		"--cpus", b.limits.CPU,
		"--memory", b.limits.Memory,
		"--pids-limit", strconv.Itoa(b.limits.PIDsLimit),
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"--read-only",
		"--tmpfs", fmt.Sprintf("/tmp:rw,noexec,nosuid,size=%d", b.limits.TmpfsSize),
		"--env", fmt.Sprintf("HOME=%s", dockerSandboxHomePath),
		"--env", fmt.Sprintf("XDG_CACHE_HOME=%s", dockerSandboxXDGCachePath),
		"--env", fmt.Sprintf("GOCACHE=%s", dockerSandboxGoCachePath),
		"--env", fmt.Sprintf("GOMODCACHE=%s", dockerSandboxGoModCachePath),
		"--env", fmt.Sprintf("GOTMPDIR=%s", dockerSandboxGoTmpPath),
		"--mount", fmt.Sprintf("type=bind,src=%s,target=%s,readonly", filepath.ToSlash(workspaceRoot), dockerSandboxMountPath),
		"--workdir", containerWorkingDir,
		b.image,
		command,
	}
	return append(dockerArgs, args...)
}

func (b *DockerSandboxExecutionBackend) cleanupContainer(containerName string) {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), dockerSandboxCleanupTimeout)
	defer cancel()
	_ = b.runner.RemoveContainer(cleanupCtx, containerName)
}

func (b *DockerSandboxExecutionBackend) defaultContainerName() string {
	sequence := b.sequence.Add(1)
	return fmt.Sprintf("cialloclaw-sandbox-%d", sequence)
}

func dockerSandboxImage() string {
	if value := strings.TrimSpace(os.Getenv("CIALLOCLAW_SANDBOX_IMAGE")); value != "" {
		return value
	}
	return defaultDockerSandboxImage
}

func (b *DockerSandboxExecutionBackend) normalizeCommand(command string) (string, error) {
	baseName := normalizeExecutionCommandName(command)
	if baseName == "" {
		return "", ErrDockerSandboxCommandNotAllowed
	}
	if _, ok := b.allowed[baseName]; !ok {
		return "", fmt.Errorf("%w: %s", ErrDockerSandboxCommandNotAllowed, baseName)
	}
	return baseName, nil
}

func cloneDockerSandboxAllowlist(source map[string]struct{}) map[string]struct{} {
	result := make(map[string]struct{}, len(source))
	for key := range source {
		result[key] = struct{}{}
	}
	return result
}

func normalizeExecutionCommandName(command string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return ""
	}
	baseName := strings.ToLower(filepath.Base(trimmed))
	return strings.TrimSuffix(baseName, ".exe")
}

func (dockerCLIRunner) RunDocker(ctx context.Context, args []string) (tools.CommandExecutionResult, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
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

func (dockerCLIRunner) RemoveContainer(ctx context.Context, containerName string) error {
	trimmedName := strings.TrimSpace(containerName)
	if trimmedName == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", trimmedName)
	return cmd.Run()
}
