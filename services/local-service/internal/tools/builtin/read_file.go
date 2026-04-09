// Package builtin 提供本地内置工具实现。
//
// 内置工具在进程内直接执行，不依赖外部 worker 或 sidecar。
// 每个内置工具必须实现 tools.Tool 接口，
// 工具名称使用 snake_case，输出必须能映射到 /packages/protocol。
package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

// ---------------------------------------------------------------------------
// ReadFileTool：读取工作区内文件的内置工具
// ---------------------------------------------------------------------------

// ReadFileTool 是一个最小示例工具，用于读取工作区内的文件内容。
//
// 它演示了如何实现 tools.Tool 接口：
//   - Metadata 返回工具元信息
//   - Validate 校验输入参数
//   - Execute 通过 PlatformCapability 读取文件
//
// 本工具不直接操作文件系统，所有平台能力通过
// ToolExecuteContext.Platform 注入。
type ReadFileTool struct {
	meta tools.ToolMetadata
}

// NewReadFileTool 创建并返回 ReadFileTool。
func NewReadFileTool() *ReadFileTool {
	return &ReadFileTool{
		meta: tools.ToolMetadata{
			Name:            "read_file",
			DisplayName:     "读取文件",
			Description:     "读取工作区内指定路径的文件内容",
			Source:          tools.ToolSourceBuiltin,
			RiskHint:        "green",
			TimeoutSec:      10,
			InputSchemaRef:  "tools/read_file/input",
			OutputSchemaRef: "tools/read_file/output",
			SupportsDryRun:  true,
		},
	}
}

// Metadata 返回 ReadFileTool 的静态元信息。
func (t *ReadFileTool) Metadata() tools.ToolMetadata {
	return t.meta
}

// Validate 校验 read_file 的输入参数。
//
// 必须包含 "path" 字段且不为空。
func (t *ReadFileTool) Validate(input map[string]any) error {
	pathVal, ok := input["path"]
	if !ok {
		return fmt.Errorf("input field 'path' is required")
	}
	pathStr, ok := pathVal.(string)
	if !ok || strings.TrimSpace(pathStr) == "" {
		return fmt.Errorf("input field 'path' must be a non-empty string")
	}
	return nil
}

// Execute 执行文件读取。
//
// 通过 ToolExecuteContext.Platform.ReadFile 读取文件内容，
// 不直接调用 os.ReadFile 或任何平台 API。
// 读取前通过 Platform.EnsureWithinWorkspace 校验路径合法性。
func (t *ReadFileTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	pathStr := input["path"].(string)

	if execCtx.Platform != nil {
		absPath, err := execCtx.Platform.Abs(pathStr)
		if err != nil {
			return nil, fmt.Errorf("resolve absolute path: %w", err)
		}
		pathStr = absPath

		safePath, err := execCtx.Platform.EnsureWithinWorkspace(pathStr)
		if err != nil {
			return nil, fmt.Errorf("path outside workspace: %w", err)
		}
		pathStr = safePath
	}

	if execCtx.Platform != nil {
		content, err := execCtx.Platform.ReadFile(pathStr)
		if err != nil {
			return &tools.ToolResult{
				ToolName: t.meta.Name,
				Error: &tools.ToolResultError{
					Message: fmt.Sprintf("read file failed: %v", err),
				},
			}, err
		}

		return &tools.ToolResult{
			ToolName: t.meta.Name,
			Output: map[string]any{
				"path":    pathStr,
				"content": string(content),
			},
		}, nil
	}

	// 当 Platform 不可用时返回占位结果，不直接操作文件系统
	return &tools.ToolResult{
		ToolName: t.meta.Name,
		Output: map[string]any{
			"path":    pathStr,
			"content": "",
		},
	}, nil
}

// DryRun 执行预检查，验证路径合法性但不实际读取文件。
func (t *ReadFileTool) DryRun(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	pathStr := input["path"].(string)

	if execCtx.Platform != nil {
		absPath, err := execCtx.Platform.Abs(pathStr)
		if err != nil {
			return nil, fmt.Errorf("resolve absolute path: %w", err)
		}
		if _, err := execCtx.Platform.EnsureWithinWorkspace(absPath); err != nil {
			return nil, fmt.Errorf("path outside workspace: %w", err)
		}
	}

	return &tools.ToolResult{
		ToolName: t.meta.Name,
		Output: map[string]any{
			"dry_run": true,
			"path":    pathStr,
			"valid":   true,
		},
	}, nil
}
