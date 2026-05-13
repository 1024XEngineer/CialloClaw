// Package builtin implements the in-process local tools.
//
// Builtin tools run inside the local service and do not depend on external
// workers or sidecars. Each tool must implement tools.Tool, use a snake_case
// name, and return output that can be mapped to /packages/protocol.
package builtin

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/textdecode"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

const readFilePreviewLimit = 200
const readFileMaxBytes int64 = 1 << 20
const readFileDefaultTextType = "text/plain"
const readFileDocumentExtractFailedUserMessage = "文档正文提取失败，请稍后重试；若仍失败，请改用 extract_text。"

// ---------------------------------------------------------------------------
// ReadFileTool: workspace file reader
// ---------------------------------------------------------------------------

// ReadFileTool reads file content from the current workspace.
//
// It demonstrates the tools.Tool contract:
//   - Metadata returns static tool metadata.
//   - Validate checks the tool input before execution.
//   - Execute reads through PlatformCapability.
//
// This tool never touches the filesystem directly. All platform capability is
// injected through ToolExecuteContext.Platform.
type ReadFileTool struct {
	meta tools.ToolMetadata
}

// NewReadFileTool creates the workspace file reader.
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

// Metadata returns static tool metadata.
func (t *ReadFileTool) Metadata() tools.ToolMetadata {
	return t.meta
}

// Validate checks read_file input.
//
// The input must include a non-empty "path" field.
func (t *ReadFileTool) Validate(input map[string]any) error {
	_, err := requireStringField(input, "path")
	return err
}

// Execute reads the target file through the injected platform adapter.
//
// The workspace boundary is validated before reading. Text bytes are decoded at
// this boundary so unsafe encodings cannot leak replacement characters into
// tool_call, event, delivery_result, or bubble previews.
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
	decodedContent, err := decodeReadFileContent(content)
	if err != nil {
		// Document-style attachments can legitimately fail strict text decoding even
		// when the OCR worker already knows how to extract their body text. Keep
		// read_file tolerant here so planner mistakes do not surface as fake
		// encoding failures for docx/pdf/image inputs.
		if extractedText, ok := tryExtractDocumentText(ctx, execCtx, safePath); ok {
			rawOutput := map[string]any{
				"path":      safePath,
				"content":   extractedText,
				"mime_type": mimeType,
				"text_type": textType,
			}
			return &tools.ToolResult{
				ToolName:      t.meta.Name,
				RawOutput:     rawOutput,
				SummaryOutput: buildReadFileSummary(rawOutput, ""),
			}, nil
		}
		rawOutput := map[string]any{
			"path":      safePath,
			"mime_type": mimeType,
			"text_type": textType,
		}
		contentPreview := textdecode.UnsupportedEncodingUserMessage
		if supportsReadFileDocumentExtraction(safePath) {
			contentPreview = readFileDocumentExtractFailedUserMessage
		}
		return &tools.ToolResult{
			ToolName:      t.meta.Name,
			RawOutput:     rawOutput,
			SummaryOutput: buildReadFileSummary(rawOutput, contentPreview),
			Error: &tools.ToolResultError{
				Message: contentPreview,
			},
		}, fmt.Errorf("%w: %w", tools.ErrToolExecutionFailed, err)
	}
	rawOutput := map[string]any{
		"path":      safePath,
		"content":   decodedContent,
		"mime_type": mimeType,
		"text_type": textType,
	}

	return &tools.ToolResult{
		ToolName:      t.meta.Name,
		RawOutput:     rawOutput,
		SummaryOutput: buildReadFileSummary(rawOutput, ""),
	}, nil
}

// DryRun validates the target path without reading file content.
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

func buildReadFileSummary(raw map[string]any, contentPreviewOverride string) map[string]any {
	content, _ := raw["content"].(string)
	contentPreview := previewReadFileText(content, readFilePreviewLimit)
	if contentPreviewOverride != "" {
		contentPreview = contentPreviewOverride
	}
	return map[string]any{
		"path":            raw["path"],
		"mime_type":       raw["mime_type"],
		"text_type":       raw["text_type"],
		"content_preview": contentPreview,
	}
}

func previewReadFileText(input string, limit int) string {
	return previewString(input, limit)
}

func detectReadFileTypes(path string, content []byte) (string, string) {
	mimeType := inferReadFileMimeType(path, content)
	return mimeType, inferReadFileTextType(mimeType)
}

func decodeReadFileContent(content []byte) (string, error) {
	decoded, err := textdecode.Decode(content)
	if err != nil {
		return "", err
	}
	return decoded.Text, nil
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

func tryExtractDocumentText(ctx context.Context, execCtx *tools.ToolExecuteContext, filePath string) (string, bool) {
	if execCtx == nil || execCtx.OCR == nil {
		return "", false
	}
	if !supportsReadFileDocumentExtraction(filePath) {
		return "", false
	}
	result, err := execCtx.OCR.ExtractText(ctx, filePath)
	if err != nil {
		return "", false
	}
	text := strings.TrimSpace(result.Text)
	if text == "" {
		return "", false
	}
	return text, true
}

func supportsReadFileDocumentExtraction(filePath string) bool {
	switch strings.ToLower(filepath.Ext(filePath)) {
	case ".bmp", ".doc", ".docx", ".gif", ".jpeg", ".jpg", ".pdf", ".png", ".pptx", ".tif", ".tiff", ".webp", ".xlsx":
		return true
	default:
		return false
	}
}
