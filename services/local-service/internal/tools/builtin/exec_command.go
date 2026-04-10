package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

const commandOutputPreviewLimit = 200

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
			TimeoutSec:      20,
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
	if execCtx == nil || execCtx.Execution == nil {
		return nil, fmt.Errorf("%w: execution adapter is required", tools.ErrCapabilityDenied)
	}

	result, err := execCtx.Execution.RunCommand(ctx, command, args, workingDir)
	if err != nil {
		return nil, fmt.Errorf("%w: command execution failed: %v", tools.ErrToolExecutionFailed, err)
	}

	rawOutput := map[string]any{
		"command":     command,
		"args":        args,
		"working_dir": workingDir,
		"stdout":      result.Stdout,
		"stderr":      result.Stderr,
		"exit_code":   result.ExitCode,
	}

	return &tools.ToolResult{
		ToolName:      t.meta.Name,
		RawOutput:     rawOutput,
		SummaryOutput: buildExecCommandSummary(command, args, workingDir, result),
	}, nil
}

func (t *ExecCommandTool) DryRun(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	_ = ctx

	command, args, workingDir, err := parseExecCommandInput(input)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", tools.ErrToolValidationFailed, err)
	}
	if execCtx == nil || execCtx.Execution == nil {
		return nil, fmt.Errorf("%w: execution adapter is required", tools.ErrCapabilityDenied)
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
		SummaryOutput: map[string]any{"command": command, "arg_count": len(args), "dry_run": true},
	}, nil
}

func parseExecCommandInput(input map[string]any) (string, []string, string, error) {
	command, ok := input["command"].(string)
	if !ok || strings.TrimSpace(command) == "" {
		return "", nil, "", fmt.Errorf("input field 'command' must be a non-empty string")
	}

	args := make([]string, 0)
	if rawArgs, ok := input["args"]; ok {
		switch typed := rawArgs.(type) {
		case []string:
			args = append(args, typed...)
		case []any:
			for _, item := range typed {
				arg, ok := item.(string)
				if !ok {
					return "", nil, "", fmt.Errorf("input field 'args' must contain only strings")
				}
				args = append(args, arg)
			}
		default:
			return "", nil, "", fmt.Errorf("input field 'args' must be a string array when provided")
		}
	}

	workingDir := ""
	if rawWorkingDir, ok := input["working_dir"]; ok {
		value, ok := rawWorkingDir.(string)
		if !ok {
			return "", nil, "", fmt.Errorf("input field 'working_dir' must be a string when provided")
		}
		workingDir = strings.TrimSpace(value)
	}

	return strings.TrimSpace(command), args, workingDir, nil
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

func previewText(input string, limit int) string {
	trimmed := strings.TrimSpace(input)
	if len(trimmed) <= limit {
		return trimmed
	}
	return trimmed[:limit]
}
