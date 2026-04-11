package sidecarclient

import (
	"context"
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

const defaultPageSearchLimit = 5
const pageTextPreviewLimit = 240

type noopPlaywrightSidecarClient struct{}

func NewNoopPlaywrightSidecarClient() tools.PlaywrightSidecarClient {
	return noopPlaywrightSidecarClient{}
}

func (noopPlaywrightSidecarClient) ReadPage(_ context.Context, _ string) (tools.BrowserPageReadResult, error) {
	return tools.BrowserPageReadResult{}, tools.ErrPlaywrightSidecarFailed
}

func (noopPlaywrightSidecarClient) SearchPage(_ context.Context, _, _ string, _ int) (tools.BrowserPageSearchResult, error) {
	return tools.BrowserPageSearchResult{}, tools.ErrPlaywrightSidecarFailed
}

type PageReadTool struct {
	meta tools.ToolMetadata
}

func NewPageReadTool() *PageReadTool {
	return &PageReadTool{meta: tools.ToolMetadata{
		Name:            "page_read",
		DisplayName:     "页面读取",
		Description:     "通过 Playwright sidecar 读取网页标题与主要文本内容",
		Source:          tools.ToolSourceSidecar,
		RiskHint:        "yellow",
		TimeoutSec:      20,
		InputSchemaRef:  "tools/page_read/input",
		OutputSchemaRef: "tools/page_read/output",
		SupportsDryRun:  false,
	}}
}

func (t *PageReadTool) Metadata() tools.ToolMetadata { return t.meta }

func (t *PageReadTool) Validate(input map[string]any) error {
	url, ok := input["url"].(string)
	if !ok || strings.TrimSpace(url) == "" {
		return fmt.Errorf("input field 'url' must be a non-empty string")
	}
	return nil
}

func (t *PageReadTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.Playwright == nil {
		return nil, tools.ErrPlaywrightSidecarFailed
	}
	url := strings.TrimSpace(input["url"].(string))
	result, err := execCtx.Playwright.ReadPage(ctx, url)
	if err != nil {
		return nil, err
	}
	rawOutput := map[string]any{
		"url":          result.URL,
		"title":        result.Title,
		"text_content": result.TextContent,
		"mime_type":    result.MIMEType,
		"text_type":    result.TextType,
		"source":       firstNonEmptyString(result.Source, "playwright_sidecar"),
	}
	return &tools.ToolResult{
		ToolName:      t.meta.Name,
		RawOutput:     rawOutput,
		SummaryOutput: map[string]any{"url": result.URL, "title": result.Title, "content_preview": previewPageText(result.TextContent), "source": firstNonEmptyString(result.Source, "playwright_sidecar")},
	}, nil
}

type PageSearchTool struct {
	meta tools.ToolMetadata
}

func NewPageSearchTool() *PageSearchTool {
	return &PageSearchTool{meta: tools.ToolMetadata{
		Name:            "page_search",
		DisplayName:     "页面搜索",
		Description:     "通过 Playwright sidecar 在页面中执行基础文本搜索",
		Source:          tools.ToolSourceSidecar,
		RiskHint:        "yellow",
		TimeoutSec:      20,
		InputSchemaRef:  "tools/page_search/input",
		OutputSchemaRef: "tools/page_search/output",
		SupportsDryRun:  false,
	}}
}

func (t *PageSearchTool) Metadata() tools.ToolMetadata { return t.meta }

func (t *PageSearchTool) Validate(input map[string]any) error {
	url, ok := input["url"].(string)
	if !ok || strings.TrimSpace(url) == "" {
		return fmt.Errorf("input field 'url' must be a non-empty string")
	}
	query, ok := input["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return fmt.Errorf("input field 'query' must be a non-empty string")
	}
	return nil
}

func (t *PageSearchTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.Playwright == nil {
		return nil, tools.ErrPlaywrightSidecarFailed
	}
	url := strings.TrimSpace(input["url"].(string))
	query := strings.TrimSpace(input["query"].(string))
	limit := defaultPageSearchLimit
	if rawLimit, ok := input["limit"]; ok {
		switch typed := rawLimit.(type) {
		case int:
			if typed > 0 {
				limit = typed
			}
		case float64:
			if int(typed) > 0 {
				limit = int(typed)
			}
		}
	}
	result, err := execCtx.Playwright.SearchPage(ctx, url, query, limit)
	if err != nil {
		return nil, err
	}
	rawOutput := map[string]any{
		"url":         result.URL,
		"query":       result.Query,
		"match_count": result.MatchCount,
		"matches":     append([]string(nil), result.Matches...),
		"source":      firstNonEmptyString(result.Source, "playwright_sidecar"),
	}
	return &tools.ToolResult{
		ToolName:      t.meta.Name,
		RawOutput:     rawOutput,
		SummaryOutput: map[string]any{"url": result.URL, "query": result.Query, "match_count": result.MatchCount, "source": firstNonEmptyString(result.Source, "playwright_sidecar")},
	}, nil
}

func RegisterPlaywrightTools(registry *tools.Registry) error {
	for _, tool := range []tools.Tool{NewPageReadTool(), NewPageSearchTool()} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

func previewPageText(input string) string {
	trimmed := strings.TrimSpace(input)
	if len(trimmed) <= pageTextPreviewLimit {
		return trimmed
	}
	return trimmed[:pageTextPreviewLimit]
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
