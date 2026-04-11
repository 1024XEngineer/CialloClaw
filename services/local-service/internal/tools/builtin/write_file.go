package builtin

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

const defaultWriteFileTextType = "text/plain"

type WriteFileInput struct {
	Path    string
	Content string
}

type WriteFileOutput struct {
	Path                string
	BytesWritten        int
	Created             bool
	Overwritten         bool
	MIMEType            string
	TextType            string
	ArtifactCandidate   map[string]any
	AuditCandidate      map[string]any
	CheckpointCandidate map[string]any
}

type WriteFileTool struct {
	meta tools.ToolMetadata
}

func NewWriteFileTool() *WriteFileTool {
	return &WriteFileTool{
		meta: tools.ToolMetadata{
			Name:            "write_file",
			DisplayName:     "写入文件",
			Description:     "在受控工作区内创建或覆盖文本文件",
			Source:          tools.ToolSourceBuiltin,
			RiskHint:        "yellow",
			TimeoutSec:      10,
			InputSchemaRef:  "tools/write_file/input",
			OutputSchemaRef: "tools/write_file/output",
			SupportsDryRun:  true,
		},
	}
}

func (t *WriteFileTool) Metadata() tools.ToolMetadata {
	return t.meta
}

func (t *WriteFileTool) Validate(input map[string]any) error {
	_, err := parseWriteFileInput(input)
	return err
}

func (t *WriteFileTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	_ = ctx

	parsed, err := parseWriteFileInput(input)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", tools.ErrToolValidationFailed, err)
	}
	if err := ensurePlatform(execCtx); err != nil {
		return nil, err
	}

	normalizedPath := normalizeWorkspaceToolPath(parsed.Path)
	safePath, err := execCtx.Platform.EnsureWithinWorkspace(normalizedPath)
	if err != nil {
		return nil, tools.ErrWorkspaceBoundaryDenied
	}

	statPath := writeFileStatPath(parsed.Path, normalizedPath, safePath)
	created, overwritten, err := detectWriteMode(execCtx.Platform, statPath)
	if err != nil {
		return nil, fmt.Errorf("%w: inspect target path: %v", tools.ErrToolExecutionFailed, err)
	}

	contentBytes := []byte(parsed.Content)
	if err := execCtx.Platform.WriteFile(safePath, contentBytes); err != nil {
		return nil, fmt.Errorf("%w: write file failed: %v", tools.ErrToolExecutionFailed, err)
	}

	output := buildWriteFileOutput(safePath, len(contentBytes), created, overwritten)
	return &tools.ToolResult{
		ToolName:      t.meta.Name,
		RawOutput:     output,
		SummaryOutput: buildWriteFileSummary(output),
	}, nil
}

func (t *WriteFileTool) DryRun(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	_ = ctx

	parsed, err := parseWriteFileInput(input)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", tools.ErrToolValidationFailed, err)
	}
	if err := ensurePlatform(execCtx); err != nil {
		return nil, err
	}

	normalizedPath := normalizeWorkspaceToolPath(parsed.Path)
	safePath, err := execCtx.Platform.EnsureWithinWorkspace(normalizedPath)
	if err != nil {
		return nil, tools.ErrWorkspaceBoundaryDenied
	}

	statPath := writeFileStatPath(parsed.Path, normalizedPath, safePath)
	created, overwritten, err := detectWriteMode(execCtx.Platform, statPath)
	if err != nil {
		return nil, fmt.Errorf("%w: inspect target path: %v", tools.ErrToolExecutionFailed, err)
	}

	output := buildWriteFileOutput(safePath, len([]byte(parsed.Content)), created, overwritten)
	output["dry_run"] = true

	return &tools.ToolResult{
		ToolName:      t.meta.Name,
		RawOutput:     output,
		SummaryOutput: buildWriteFileSummary(output),
	}, nil
}

func parseWriteFileInput(input map[string]any) (WriteFileInput, error) {
	pathValue, err := requireStringField(input, "path")
	if err != nil {
		return WriteFileInput{}, err
	}
	contentValue, ok := input["content"]
	if !ok {
		return WriteFileInput{}, errors.New("input field 'content' must be a string")
	}
	contentString, ok := contentValue.(string)
	if !ok {
		return WriteFileInput{}, errors.New("input field 'content' must be a string")
	}

	return WriteFileInput{Path: pathValue, Content: contentString}, nil
}

func detectWriteMode(platform tools.PlatformCapability, safePath string) (created bool, overwritten bool, err error) {
	_, statErr := platform.Stat(safePath)
	if statErr == nil {
		return false, true, nil
	}
	if errors.Is(statErr, fs.ErrNotExist) {
		return true, false, nil
	}
	return false, false, statErr
}

func buildWriteFileOutput(path string, bytesWritten int, created bool, overwritten bool) map[string]any {
	action := "create"
	if overwritten {
		action = "overwrite"
	}

	return map[string]any{
		"path":          path,
		"bytes_written": bytesWritten,
		"created":       created,
		"overwritten":   overwritten,
		"mime_type":     defaultWriteFileTextType,
		"text_type":     defaultWriteFileTextType,
		"artifact_candidate": map[string]any{
			"artifact_type": "generated_file",
			"title":         path,
			"path":          path,
			"mime_type":     defaultWriteFileTextType,
		},
		"audit_candidate": map[string]any{
			"type":    "file",
			"action":  "write_file",
			"summary": action + " file",
			"target":  path,
			"result":  "success",
		},
		"checkpoint_candidate": map[string]any{
			"required":    overwritten,
			"target_path": path,
			"reason":      "write_file_before_change",
		},
	}
}

func buildWriteFileSummary(raw map[string]any) map[string]any {
	return map[string]any{
		"path":          raw["path"],
		"bytes_written": raw["bytes_written"],
		"created":       raw["created"],
		"overwritten":   raw["overwritten"],
		"mime_type":     raw["mime_type"],
	}
}

func writeFileStatPath(originalPath, normalizedPath, safePath string) string {
	if isToolAbsolutePath(originalPath) {
		return safePath
	}
	if normalizedPath == "" {
		return safePath
	}
	return normalizedPath
}
