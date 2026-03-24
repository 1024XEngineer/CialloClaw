package skills

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"minimal-agent/pkg/providers"
)

// ScriptTool 执行 skill 中的脚本
type ScriptTool struct {
	Name        string
	Command     string
	Description string
	Args        []ArgDef
	SkillDir    string
}

func (st *ScriptTool) GetName() string        { return st.Name }
func (st *ScriptTool) GetDescription() string { return st.Description }

func (st *ScriptTool) GetParameters() map[string]any {
	properties := map[string]any{}
	required := []string{}

	for _, arg := range st.Args {
		properties[arg.Name] = map[string]any{
			"type":        arg.Type,
			"description": arg.Description,
		}
		required = append(required, arg.Name)
	}

	// 如果没有定义参数，至少支持 command 参数
	if len(st.Args) == 0 {
		properties["command"] = map[string]any{
			"type":        "string",
			"description": "Full command to execute",
		}
	}

	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   required,
	}
}

func (st *ScriptTool) Execute(ctx context.Context, args map[string]any) *providers.ToolResult {
	// 构建命令
	var cmdStr string
	if cmd, ok := args["command"].(string); ok && cmd != "" {
		// 使用完整的 command 参数
		cmdStr = cmd
	} else {
		// 替换变量
		cmdStr = st.Command
		for k, v := range args {
			placeholder := "{{" + k + "}}"
			value := fmt.Sprintf("%v", v)
			cmdStr = strings.ReplaceAll(cmdStr, placeholder, value)
		}
	}

	// 执行命令
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", cmdStr)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", cmdStr)
	}

	// 设置工作目录
	if st.SkillDir != "" {
		cmd.Dir = st.SkillDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		out := stdout.String()
		if stderr.Len() > 0 {
			out += "\nSTDERR:\n" + stderr.String()
		}
		if out == "" {
			out = err.Error()
		}
		return &providers.ToolResult{
			Name:    st.Name,
			Content: out,
			IsError: true,
		}
	}

	out := stdout.String()
	if stderr.Len() > 0 {
		out += "\nSTDERR:\n" + stderr.String()
	}
	if strings.TrimSpace(out) == "" {
		out = "(command executed successfully, no output)"
	}

	return &providers.ToolResult{
		Name:    st.Name,
		Content: out,
		IsError: false,
	}
}
