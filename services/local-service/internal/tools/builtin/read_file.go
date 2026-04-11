// Package builtin 提供本地内置工具实现。
//
// 内置工具在进程内直接执行，不依赖外部 worker 或 sidecar。
// 每个内置工具必须实现 tools.Tool 接口，
// 工具名称使用 snake_case，输出必须能映射到 /packages/protocol。
package builtin

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

const readFilePreviewLimit = 200
const readFileMaxBytes int64 = 1 << 20
const readFileDefaultTextType = "text/plain"

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
	_, err := requireStringField(input, "path")
	return err
}

// Execute 执行文件读取。
//
// 通过 ToolExecuteContext.Platform.ReadFile 读取文件内容，
// 不直接调用 os.ReadFile 或任何平台 API。
// 读取前通过 Platform.EnsureWithinWorkspace 校验路径合法性。
func (t *ReadFileTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	_ = ctx

	pathStr, err := requireStringField(input, "path")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", tools.ErrToolValidationFailed, err)
	}
	if err := ensurePlatform(execCtx); err != nil {
		return nil, err
	}

	normalizedPath := normalizeWorkspaceToolPath(pathStr)
	safePath, err := execCtx.Platform.EnsureWithinWorkspace(normalizedPath)
	if err != nil {
		return nil, tools.ErrWorkspaceBoundaryDenied
	}
	readPath := readFileToolPath(pathStr, normalizedPath, safePath)
	if info, err := execCtx.Platform.Stat(readPath); err == nil {
		if info.Size() > readFileMaxBytes {
			return nil, fmt.Errorf("%w: file exceeds %d bytes", tools.ErrToolExecutionFailed, readFileMaxBytes)
		}
	}

	content, err := execCtx.Platform.ReadFile(readPath)
	if err != nil {
		return &tools.ToolResult{
			ToolName: t.meta.Name,
			Error: &tools.ToolResultError{
				Message: fmt.Sprintf("read file failed: %v", err),
			},
		}, fmt.Errorf("%w: %v", tools.ErrToolExecutionFailed, err)
	}

	mimeType, textType := detectReadFileTypes(readPath, content)
	rawOutput := map[string]any{
		"path":      safePath,
		"content":   string(content),
		"mime_type": mimeType,
		"text_type": textType,
	}

	return &tools.ToolResult{
		ToolName:      t.meta.Name,
		RawOutput:     rawOutput,
		SummaryOutput: buildReadFileSummary(rawOutput),
	}, nil
}

// DryRun 执行预检查，验证路径合法性但不实际读取文件。
func (t *ReadFileTool) DryRun(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	_ = ctx

	pathStr, err := requireStringField(input, "path")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", tools.ErrToolValidationFailed, err)
	}
	if err := ensurePlatform(execCtx); err != nil {
		return nil, err
	}

	normalizedPath := normalizeWorkspaceToolPath(pathStr)
	safePath, err := execCtx.Platform.EnsureWithinWorkspace(normalizedPath)
	if err != nil {
		return nil, tools.ErrWorkspaceBoundaryDenied
	}

	return &tools.ToolResult{
		ToolName: t.meta.Name,
		RawOutput: map[string]any{
			"dry_run":   true,
			"path":      safePath,
			"valid":     true,
			"mime_type": inferReadFileMimeType(pathStr, nil),
			"text_type": inferReadFileTextType(inferReadFileMimeType(pathStr, nil)),
		},
		SummaryOutput: map[string]any{
			"dry_run":   true,
			"path":      safePath,
			"valid":     true,
			"mime_type": inferReadFileMimeType(pathStr, nil),
		},
	}, nil
}

func readFileToolPath(originalPath, normalizedPath, safePath string) string {
	if isToolAbsolutePath(originalPath) {
		return safePath
	}
	if normalizedPath == "" {
		return safePath
	}
	return normalizedPath
}

func buildReadFileSummary(raw map[string]any) map[string]any {
	content, _ := raw["content"].(string)
	return map[string]any{
		"path":            raw["path"],
		"mime_type":       raw["mime_type"],
		"text_type":       raw["text_type"],
		"content_preview": previewReadFileText(content, readFilePreviewLimit),
	}
}

func previewReadFileText(input string, limit int) string {
	return previewString(input, limit)
}

func detectReadFileTypes(path string, content []byte) (string, string) {
	mimeType := inferReadFileMimeType(path, content)
	return mimeType, inferReadFileTextType(mimeType)
}

func inferReadFileMimeType(path string, content []byte) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	case ".json":
		return "application/json"
	case ".yaml", ".yml":
		return "application/yaml"
	case ".csv":
		return "text/csv"
	}

	if len(content) > 0 {
		sample := content
		if len(sample) > 512 {
			sample = sample[:512]
		}
		return http.DetectContentType(sample)
	}

	return readFileDefaultTextType
}

func inferReadFileTextType(mimeType string) string {
	switch {
	case strings.HasPrefix(mimeType, "text/"):
		return mimeType
	case mimeType == "application/json":
		return "structured_text"
	case mimeType == "application/yaml":
		return "structured_text"
	default:
		return readFileDefaultTextType
	}
}
