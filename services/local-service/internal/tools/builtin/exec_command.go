package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

const commandOutputPreviewLimit = 200
const commandOutputRawLimit = 4096

type ExecCommandTool struct {
	meta tools.ToolMetadata
}

func NewExecCommandTool() *ExecCommandTool {
	return &ExecCommandTool{
		meta: tools.ToolMetadata{
			Name:            "exec_command",
			DisplayName:     "执行命令",
			Description:     "通过受控执行后端运行命令并返回最小结果摘要",
			Source:          tools.ToolSourceBuiltin,
			RiskHint:        "red",
			TimeoutSec:      tools.DefaultTimeoutSec,
			InputSchemaRef:  "tools/exec_command/input",
			OutputSchemaRef: "tools/exec_command/output",
			SupportsDryRun:  true,
		},
	}
}

func (t *ExecCommandTool) Metadata() tools.ToolMetadata {
	return t.meta
}

func (t *ExecCommandTool) Validate(input map[string]any) error {
	_, _, _, err := parseExecCommandInput(input)
	return err
}

func (t *ExecCommandTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	command, args, workingDir, err := parseExecCommandInput(input)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", tools.ErrToolValidationFailed, err)
	}
	if err := ensureExecution(execCtx); err != nil {
		return nil, err
	}

	workingDir, err = resolveExecCommandWorkingDir(execCtx, workingDir)
	if err != nil {
		return nil, err
	}

	result, err := execCtx.Execution.RunCommand(ctx, command, args, workingDir)
	if err != nil {
		return nil, fmt.Errorf("%w: command execution failed: %v", tools.ErrToolExecutionFailed, err)
	}

	rawStdout, stdoutTruncated, stdoutBytes := truncateCommandOutput(result.Stdout)
	rawStderr, stderrTruncated, stderrBytes := truncateCommandOutput(result.Stderr)

	rawOutput := map[string]any{
		"command":          command,
		"args":             args,
		"working_dir":      workingDir,
		"stdout":           rawStdout,
		"stderr":           rawStderr,
		"stdout_bytes":     stdoutBytes,
		"stderr_bytes":     stderrBytes,
		"stdout_truncated": stdoutTruncated,
		"stderr_truncated": stderrTruncated,
		"exit_code":        result.ExitCode,
		"audit_candidate": map[string]any{
			"type":    "command",
			"action":  "exec_command",
			"summary": buildExecCommandAuditSummary(command, result.ExitCode),
			"target":  workingDir,
			"result":  commandAuditResult(result.ExitCode),
		},
	}
	if backendName := executionBackendName(result); backendName != "" {
		rawOutput["execution_backend"] = backendName
	}
	if sandboxInfo := sandboxResultInfo(result); len(sandboxInfo) > 0 {
		rawOutput["sandbox"] = sandboxInfo
	}
	summaryOutput := buildExecCommandSummary(command, args, workingDir, result)
	if backendName := executionBackendName(result); backendName != "" {
		summaryOutput["execution_backend"] = backendName
	}

	return &tools.ToolResult{
		ToolName:      t.meta.Name,
		RawOutput:     rawOutput,
		SummaryOutput: summaryOutput,
	}, nil
}

func (t *ExecCommandTool) DryRun(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	_ = ctx

	command, args, workingDir, err := parseExecCommandInput(input)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", tools.ErrToolValidationFailed, err)
	}
	if err := ensureExecution(execCtx); err != nil {
		return nil, err
	}

	workingDir, err = resolveExecCommandWorkingDir(execCtx, workingDir)
	if err != nil {
		return nil, err
	}

	rawOutput := map[string]any{
		"dry_run":     true,
		"command":     command,
		"args":        args,
		"working_dir": workingDir,
	}

	return &tools.ToolResult{
		ToolName:      t.meta.Name,
		RawOutput:     rawOutput,
		SummaryOutput: map[string]any{"command": command, "arg_count": len(args), "dry_run": true, "working_dir": workingDir},
	}, nil
}

func parseExecCommandInput(input map[string]any) (string, []string, string, error) {
	command, err := requireStringField(input, "command")
	if err != nil {
		return "", nil, "", err
	}

	args, err := optionalStringSliceField(input, "args")
	if err != nil {
		return "", nil, "", err
	}

	workingDir, err := optionalStringField(input, "working_dir")
	if err != nil {
		return "", nil, "", err
	}

	return command, args, workingDir, nil
}

func buildExecCommandSummary(command string, args []string, workingDir string, result tools.CommandExecutionResult) map[string]any {
	return map[string]any{
		"command":        command,
		"arg_count":      len(args),
		"working_dir":    workingDir,
		"exit_code":      result.ExitCode,
		"stdout_preview": previewText(result.Stdout, commandOutputPreviewLimit),
		"stderr_preview": previewText(result.Stderr, commandOutputPreviewLimit),
	}
}

func buildExecCommandAuditSummary(command string, exitCode int) string {
	if exitCode == 0 {
		return fmt.Sprintf("execute command: %s", command)
	}
	return fmt.Sprintf("command exited with code %d: %s", exitCode, command)
}

func commandAuditResult(exitCode int) string {
	if exitCode == 0 {
		return "success"
	}
	return "failed"
}

func resolveExecCommandWorkingDir(execCtx *tools.ToolExecuteContext, workingDir string) (string, error) {
	resolved := strings.TrimSpace(workingDir)
	if resolved == "" {
		resolved = strings.TrimSpace(execCtx.WorkspacePath)
	}
	if resolved == "" {
		return "", fmt.Errorf("%w: workspace path is required", tools.ErrCapabilityDenied)
	}
	resolved = normalizeWorkspaceToolPath(resolved)
	safePath, err := execCtx.Platform.EnsureWithinWorkspace(resolved)
	if err != nil {
		return "", tools.ErrWorkspaceBoundaryDenied
	}
	return safePath, nil
}

func truncateCommandOutput(input string) (string, bool, int) {
	byteCount := len(input)
	if byteCount <= commandOutputRawLimit {
		return input, false, byteCount
	}
	return input[:commandOutputRawLimit], true, byteCount
}

func previewText(input string, limit int) string {
	trimmed := strings.TrimSpace(input)
	if len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit]
}

func executionBackendName(result tools.CommandExecutionResult) string {
	return strings.TrimSpace(result.ExecutionBackend)
}

func sandboxResultInfo(result tools.CommandExecutionResult) map[string]any {
	if strings.TrimSpace(result.SandboxContainer) == "" && strings.TrimSpace(result.SandboxImage) == "" && !result.Interrupted {
		return nil
	}
	info := map[string]any{
		"interrupted": result.Interrupted,
	}
	if strings.TrimSpace(result.SandboxContainer) != "" {
		info["container_name"] = strings.TrimSpace(result.SandboxContainer)
	}
	if strings.TrimSpace(result.SandboxImage) != "" {
		info["image"] = strings.TrimSpace(result.SandboxImage)
	}
	return info
}
