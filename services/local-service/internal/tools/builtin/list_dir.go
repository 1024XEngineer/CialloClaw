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
			Description:     "列出受控工作区内目录的子项信息",
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
	if execCtx == nil || execCtx.Platform == nil {
		return nil, fmt.Errorf("%w: platform adapter is required", tools.ErrCapabilityDenied)
	}

	absPath, err := execCtx.Platform.Abs(pathStr)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve absolute path: %v", tools.ErrToolExecutionFailed, err)
	}

	safePath, err := execCtx.Platform.EnsureWithinWorkspace(absPath)
	if err != nil {
		// workspace 外路径必须显式映射为统一边界错误，
		// 这样上层 executor、测试和后续审批链路都可以稳定识别该场景。
		return nil, fmt.Errorf("%w: %v", tools.ErrWorkspaceBoundaryDenied, err)
	}

	entries, err := execCtx.Platform.ReadDir(safePath)
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
	if execCtx == nil || execCtx.Platform == nil {
		return nil, fmt.Errorf("%w: platform adapter is required", tools.ErrCapabilityDenied)
	}

	absPath, err := execCtx.Platform.Abs(pathStr)
	if err != nil {
		return nil, fmt.Errorf("%w: resolve absolute path: %v", tools.ErrToolExecutionFailed, err)
	}

	safePath, err := execCtx.Platform.EnsureWithinWorkspace(absPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", tools.ErrWorkspaceBoundaryDenied, err)
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
	pathValue, ok := input["path"].(string)
	if !ok || strings.TrimSpace(pathValue) == "" {
		return "", 0, fmt.Errorf("input field 'path' must be a non-empty string")
	}

	limit := defaultListDirMaxEntries
	if rawLimit, ok := input["limit"]; ok {
		switch v := rawLimit.(type) {
		case int:
			if v > 0 && v < limit {
				limit = v
			}
		case float64:
			if int(v) > 0 && int(v) < limit {
				limit = int(v)
			}
		default:
			return "", 0, fmt.Errorf("input field 'limit' must be a number when provided")
		}
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
		"path":           raw["path"],
		"entry_count":    raw["entry_count"],
		"returned_count": raw["returned_count"],
		"truncated":      raw["truncated"],
	}
}
