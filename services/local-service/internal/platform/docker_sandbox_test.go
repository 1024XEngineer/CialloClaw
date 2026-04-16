package platform

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

type stubDockerSandboxRunner struct {
	result            tools.CommandExecutionResult
	err               error
	lastArgs          []string
	removeCalls       []string
	blockUntilContext bool
}

type stubLocalExecutionRunner struct {
	result      tools.CommandExecutionResult
	err         error
	lastCommand string
	lastArgs    []string
	lastWorkdir string
}

func (s *stubDockerSandboxRunner) RunDocker(ctx context.Context, args []string) (tools.CommandExecutionResult, error) {
	s.lastArgs = append([]string(nil), args...)
	if s.blockUntilContext {
		<-ctx.Done()
		return s.result, ctx.Err()
	}
	return s.result, s.err
}

func (s *stubDockerSandboxRunner) RemoveContainer(_ context.Context, containerName string) error {
	s.removeCalls = append(s.removeCalls, containerName)
	return nil
}

func (s *stubLocalExecutionRunner) RunCommand(_ context.Context, command string, args []string, workingDir string) (tools.CommandExecutionResult, error) {
	s.lastCommand = command
	s.lastArgs = append([]string(nil), args...)
	s.lastWorkdir = workingDir
	return s.result, s.err
}

func TestDockerSandboxExecutionBackendBuildsRestrictedDockerRun(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	runner := &stubDockerSandboxRunner{result: tools.CommandExecutionResult{Stdout: "ok", ExitCode: 0}}
	backend := NewDockerSandboxExecutionBackend(workspaceRoot)
	backend.runner = runner
	backend.nameGenerator = func() string { return "sandbox-test-1" }
	workingDir := filepath.Join(workspaceRoot, "subdir")
	result, err := backend.RunCommand(context.Background(), "go", []string{"test", "./..."}, workingDir)
	if err != nil {
		t.Fatalf("RunCommand returned error: %v", err)
	}
	if result.ExecutionBackend != backend.Name() {
		t.Fatalf("expected execution backend metadata, got %+v", result)
	}
	joinedArgs := strings.Join(runner.lastArgs, " ")
	for _, fragment := range []string{"run", "--rm", "--network none", "--cpus 1.0", "--memory 512m", "--pids-limit 128", "--read-only", "--security-opt no-new-privileges", "HOME=/tmp/home", "XDG_CACHE_HOME=/tmp/xdg-cache", "GOCACHE=/tmp/go-cache", "GOMODCACHE=/tmp/go-mod-cache", "GOTMPDIR=/tmp/go-tmp", "target=/workspace,readonly", "--workdir /workspace/subdir", defaultDockerSandboxImage, "go test ./..."} {
		if !strings.Contains(joinedArgs, fragment) {
			t.Fatalf("expected docker args to contain %q, got %q", fragment, joinedArgs)
		}
	}
	if !strings.Contains(joinedArgs, filepath.ToSlash(workspaceRoot)) {
		t.Fatalf("expected workspace mount in docker args, got %q", joinedArgs)
	}
}

func TestDockerSandboxExecutionBackendRejectsCommandsOutsideAllowlist(t *testing.T) {
	runner := &stubDockerSandboxRunner{}
	backend := NewDockerSandboxExecutionBackend(filepath.Join(t.TempDir(), "workspace"))
	backend.runner = runner
	_, err := backend.RunCommand(context.Background(), "powershell", nil, filepath.Join(t.TempDir(), "workspace"))
	if !errors.Is(err, ErrDockerSandboxCommandNotAllowed) {
		t.Fatalf("expected ErrDockerSandboxCommandNotAllowed, got %v", err)
	}
	if len(runner.lastArgs) != 0 {
		t.Fatalf("expected denied command not to invoke docker, got %+v", runner.lastArgs)
	}
}

func TestDockerSandboxExecutionBackendFallsBackToWorkingDirAsMountRoot(t *testing.T) {
	workingDir := filepath.Join(t.TempDir(), "workspace")
	runner := &stubDockerSandboxRunner{result: tools.CommandExecutionResult{Stdout: "ok", ExitCode: 0}}
	backend := NewDockerSandboxExecutionBackend("")
	backend.runner = runner
	backend.nameGenerator = func() string { return "sandbox-test-2" }
	if _, err := backend.RunCommand(context.Background(), "git", []string{"status"}, workingDir); err != nil {
		t.Fatalf("RunCommand returned error: %v", err)
	}
	joinedArgs := strings.Join(runner.lastArgs, " ")
	if !strings.Contains(joinedArgs, "--workdir /workspace") {
		t.Fatalf("expected root workdir fallback, got %q", joinedArgs)
	}
	if !strings.Contains(joinedArgs, filepath.ToSlash(workingDir)) {
		t.Fatalf("expected working dir to become mount root, got %q", joinedArgs)
	}
}

func TestDockerSandboxExecutionBackendRejectsWorkingDirOutsideWorkspace(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	backend := NewDockerSandboxExecutionBackend(workspaceRoot)
	_, err := backend.RunCommand(context.Background(), "go", []string{"test", "./..."}, filepath.Join(t.TempDir(), "outside"))
	if !errors.Is(err, ErrDockerSandboxWorkingDirOutsideWorkspace) {
		t.Fatalf("expected ErrDockerSandboxWorkingDirOutsideWorkspace, got %v", err)
	}
}

func TestDockerSandboxExecutionBackendCleansUpInterruptedContainer(t *testing.T) {
	runner := &stubDockerSandboxRunner{blockUntilContext: true}
	backend := NewDockerSandboxExecutionBackend(filepath.Join(t.TempDir(), "workspace"))
	backend.runner = runner
	backend.nameGenerator = func() string { return "sandbox-test-interrupted" }
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	result, err := backend.RunCommand(ctx, "go", []string{"test", "./..."}, backend.workspaceRoot)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline error, got %v", err)
	}
	if !result.Interrupted {
		t.Fatalf("expected interrupted sandbox result, got %+v", result)
	}
	if len(runner.removeCalls) != 1 || runner.removeCalls[0] != "sandbox-test-interrupted" {
		t.Fatalf("expected interrupted sandbox cleanup, got %+v", runner.removeCalls)
	}
}

func TestDockerSandboxExecutionBackendReportsMissingDockerCLI(t *testing.T) {
	runner := &stubDockerSandboxRunner{err: exec.ErrNotFound}
	backend := NewDockerSandboxExecutionBackend(filepath.Join(t.TempDir(), "workspace"))
	backend.runner = runner
	backend.nameGenerator = func() string { return "sandbox-test-missing-docker" }
	_, err := backend.RunCommand(context.Background(), "go", []string{"test", "./..."}, backend.workspaceRoot)
	if !errors.Is(err, ErrDockerSandboxUnavailable) {
		t.Fatalf("expected ErrDockerSandboxUnavailable, got %v", err)
	}
}

func TestControlledExecutionBackendRoutesWindowsShellCommandsToLocalHost(t *testing.T) {
	workspaceRoot := t.TempDir()
	backend := NewControlledExecutionBackend(workspaceRoot)
	localRunner := &stubLocalExecutionRunner{result: tools.CommandExecutionResult{Stdout: "ok", ExitCode: 0, ExecutionBackend: "local_host"}}
	backend.local = localRunner
	result, err := backend.RunCommand(context.Background(), "cmd", []string{"/c", "echo", "ok"}, workspaceRoot)
	if err != nil {
		t.Fatalf("RunCommand returned error: %v", err)
	}
	if result.ExecutionBackend != "local_host" {
		t.Fatalf("expected local_host backend, got %+v", result)
	}
	if localRunner.lastCommand != "cmd" || localRunner.lastWorkdir != workspaceRoot {
		t.Fatalf("expected local runner invocation, got %+v", localRunner)
	}
}

func TestControlledExecutionBackendRoutesLinuxCommandsToSandbox(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	runner := &stubDockerSandboxRunner{result: tools.CommandExecutionResult{Stdout: "ok", ExitCode: 0}}
	backend := NewControlledExecutionBackend(workspaceRoot)
	backend.sandbox.runner = runner
	backend.sandbox.nameGenerator = func() string { return "sandbox-test-routed" }
	result, err := backend.RunCommand(context.Background(), "go", []string{"test", "./..."}, workspaceRoot)
	if err != nil {
		t.Fatalf("RunCommand returned error: %v", err)
	}
	if result.ExecutionBackend != "docker_sandbox" {
		t.Fatalf("expected docker_sandbox backend, got %+v", result)
	}
	if len(runner.lastArgs) == 0 {
		t.Fatal("expected sandbox runner to be invoked")
	}
}
