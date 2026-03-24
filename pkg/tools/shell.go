package tools

import (
	"bytes"
	"context"
	"os/exec"
	"runtime"
	"strings"

	"minimal-agent/pkg/providers"
)

// Tool represents a callable tool.
type Tool interface {
	GetName() string
	GetDescription() string
	GetParameters() map[string]any
	Execute(ctx context.Context, args map[string]any) *providers.ToolResult
}

// ShellTool executes shell commands.
type ShellTool struct{}

// GetName returns the tool name.
func (t *ShellTool) GetName() string {
	return "shell"
}

// GetDescription returns the tool description.
func (t *ShellTool) GetDescription() string {
	return "Execute a shell command and return its output. Use this to run terminal commands."
}

// GetParameters returns the parameter schema.
func (t *ShellTool) GetParameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute",
			},
		},
		"required": []string{"command"},
	}
}

// Execute runs the shell command.
func (t *ShellTool) Execute(ctx context.Context, args map[string]any) *providers.ToolResult {
	cmdStr, _ := args["command"].(string)
	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return &providers.ToolResult{
			Name:    t.GetName(),
			Content: "Error: command is required",
			IsError: true,
		}
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		out := stdout.String()
		if stderr.Len() > 0 {
			if out != "" {
				out += "\nSTDERR:\n" + stderr.String()
			} else {
				out = "STDERR:\n" + stderr.String()
			}
		}
		if out == "" {
			out = err.Error()
		}
		return &providers.ToolResult{
			Name:    t.GetName(),
			Content: out,
			IsError: true,
		}
	}

	out := stdout.String()
	if strings.TrimSpace(stderr.String()) != "" {
		if out != "" {
			out += "\nSTDERR:\n" + stderr.String()
		} else {
			out = "STDERR:\n" + stderr.String()
		}
	}
	if strings.TrimSpace(out) == "" {
		out = "(command executed successfully, no output)"
	}
	return &providers.ToolResult{
		Name:    t.GetName(),
		Content: out,
		IsError: false,
	}
}
