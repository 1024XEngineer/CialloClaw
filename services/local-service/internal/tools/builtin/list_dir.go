package builtin

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

const defaultListDirMaxEntries = 50

type ListDirTool struct {
	meta tools.ToolMetadata
}

func NewListDirTool() *ListDirTool {
	return &ListDirTool{
		meta: tools.ToolMetadata{
			Name:            "list_dir",
			DisplayName:     "列出目录",
			Description:     "列出受控本地路径范围内目录的子项信息",
			Source:          tools.ToolSourceBuiltin,
			RiskHint:        "green",
			TimeoutSec:      10,
			InputSchemaRef:  "tools/list_dir/input",
			OutputSchemaRef: "tools/list_dir/output",
			SupportsDryRun:  true,
		},
	}
}

func (t *ListDirTool) Metadata() tools.ToolMetadata {
	return t.meta
}

func (t *ListDirTool) Validate(input map[string]any) error {
	_, _, err := parseListDirInput(input)
	return err
}

func (t *ListDirTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	_ = ctx

	pathStr, limit, err := parseListDirInput(input)
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

	readPath := normalizedPath
	if isToolAbsolutePath(pathStr) || readPath == "" {
		readPath = safePath
	}
	entries, err := execCtx.Platform.ReadDir(readPath)
	if err != nil {
		return nil, fmt.Errorf("%w: list directory failed: %v", tools.ErrToolExecutionFailed, err)
	}

	items, total := summarizeDirEntries(entries, limit)
	rawOutput := map[string]any{
		"path":           safePath,
		"entry_count":    total,
		"returned_count": len(items),
		"truncated":      total > len(items),
		"entries":        items,
	}

	return &tools.ToolResult{
		ToolName:      t.meta.Name,
		RawOutput:     rawOutput,
		SummaryOutput: buildListDirSummary(rawOutput),
	}, nil
}

func (t *ListDirTool) DryRun(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	_ = ctx

	pathStr, limit, err := parseListDirInput(input)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", tools.ErrToolValidationFailed, err)
	}
	if err := ensurePlatform(execCtx); err != nil {
		return nil, err
	}

	safePath, err := execCtx.Platform.EnsureWithinWorkspace(normalizeWorkspaceToolPath(pathStr))
	if err != nil {
		return nil, tools.ErrWorkspaceBoundaryDenied
	}

	rawOutput := map[string]any{
		"path":      safePath,
		"limit":     limit,
		"dry_run":   true,
		"validated": true,
	}

	return &tools.ToolResult{
		ToolName:      t.meta.Name,
		RawOutput:     rawOutput,
		SummaryOutput: buildListDirSummary(rawOutput),
	}, nil
}

func parseListDirInput(input map[string]any) (string, int, error) {
	pathValue, err := requireStringField(input, "path")
	if err != nil {
		return "", 0, err
	}

	limit, err := optionalPositiveLimitField(input, "limit", defaultListDirMaxEntries)
	if err != nil {
		return "", 0, err
	}

	return pathValue, limit, nil
}

func summarizeDirEntries(entries []fs.DirEntry, limit int) ([]map[string]any, int) {
	total := len(entries)
	if limit <= 0 || limit > total {
		limit = total
	}

	items := make([]map[string]any, 0, limit)
	for _, entry := range entries[:limit] {
		item := map[string]any{
			"name":   entry.Name(),
			"is_dir": entry.IsDir(),
		}
		if info, err := entry.Info(); err == nil {
			item["size"] = info.Size()
		}
		items = append(items, item)
	}

	return items, total
}

func buildListDirSummary(raw map[string]any) map[string]any {
	return map[string]any{
		"path":                 raw["path"],
		"entry_count":          raw["entry_count"],
		"returned_count":       raw["returned_count"],
		"truncated":            raw["truncated"],
		"entries_preview":      summarizeListDirEntryNames(raw["entries"]),
		"entries_preview_text": strings.Join(summarizeListDirEntryNames(raw["entries"]), ", "),
	}
}

func summarizeListDirEntryNames(rawEntries any) []string {
	entries, ok := rawEntries.([]map[string]any)
	if !ok || len(entries) == 0 {
		return nil
	}
	preview := make([]string, 0, len(entries))
	for _, entry := range entries {
		name, _ := entry["name"].(string)
		if name == "" {
			continue
		}
		if isDir, _ := entry["is_dir"].(bool); isDir {
			preview = append(preview, name+"/")
			continue
		}
		preview = append(preview, name)
	}
	if len(preview) == 0 {
		return nil
	}
	return preview
}
