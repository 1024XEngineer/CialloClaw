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

func (noopPlaywrightSidecarClient) ReadPageAttached(_ context.Context, _ string, _ tools.BrowserAttachConfig) (tools.BrowserPageReadResult, error) {
	return tools.BrowserPageReadResult{}, tools.ErrPlaywrightSidecarFailed
}

func (noopPlaywrightSidecarClient) SearchPage(_ context.Context, _, _ string, _ int) (tools.BrowserPageSearchResult, error) {
	return tools.BrowserPageSearchResult{}, tools.ErrPlaywrightSidecarFailed
}

func (noopPlaywrightSidecarClient) SearchPageAttached(_ context.Context, _, _ string, _ int, _ tools.BrowserAttachConfig) (tools.BrowserPageSearchResult, error) {
	return tools.BrowserPageSearchResult{}, tools.ErrPlaywrightSidecarFailed
}

func (noopPlaywrightSidecarClient) SearchWeb(_ context.Context, _ tools.BrowserWebSearchRequest) (tools.BrowserWebSearchResult, error) {
	return tools.BrowserWebSearchResult{}, tools.ErrPlaywrightSidecarFailed
}

func (noopPlaywrightSidecarClient) InteractPage(_ context.Context, _ string, _ []map[string]any) (tools.BrowserPageInteractResult, error) {
	return tools.BrowserPageInteractResult{}, tools.ErrPlaywrightSidecarFailed
}

func (noopPlaywrightSidecarClient) InteractPageAttached(_ context.Context, _ string, _ []map[string]any, _ tools.BrowserAttachConfig) (tools.BrowserPageInteractResult, error) {
	return tools.BrowserPageInteractResult{}, tools.ErrPlaywrightSidecarFailed
}

func (noopPlaywrightSidecarClient) StructuredDOM(_ context.Context, _ string) (tools.BrowserStructuredDOMResult, error) {
	return tools.BrowserStructuredDOMResult{}, tools.ErrPlaywrightSidecarFailed
}

func (noopPlaywrightSidecarClient) StructuredDOMAttached(_ context.Context, _ string, _ tools.BrowserAttachConfig) (tools.BrowserStructuredDOMResult, error) {
	return tools.BrowserStructuredDOMResult{}, tools.ErrPlaywrightSidecarFailed
}

func (noopPlaywrightSidecarClient) AttachCurrentPage(_ context.Context, _ tools.BrowserAttachConfig) (tools.BrowserAttachedPageResult, error) {
	return tools.BrowserAttachedPageResult{}, tools.ErrPlaywrightSidecarFailed
}

func (noopPlaywrightSidecarClient) SnapshotBrowser(_ context.Context, _ tools.BrowserAttachConfig) (tools.BrowserSnapshotResult, error) {
	return tools.BrowserSnapshotResult{}, tools.ErrPlaywrightSidecarFailed
}

func (noopPlaywrightSidecarClient) NavigateBrowser(_ context.Context, _ tools.BrowserNavigateRequest) (tools.BrowserNavigationResult, error) {
	return tools.BrowserNavigationResult{}, tools.ErrPlaywrightSidecarFailed
}

func (noopPlaywrightSidecarClient) ListBrowserTabs(_ context.Context, _ tools.BrowserAttachConfig) (tools.BrowserTabsListResult, error) {
	return tools.BrowserTabsListResult{}, tools.ErrPlaywrightSidecarFailed
}

func (noopPlaywrightSidecarClient) FocusBrowserTab(_ context.Context, _ tools.BrowserAttachConfig) (tools.BrowserAttachedPageResult, error) {
	return tools.BrowserAttachedPageResult{}, tools.ErrPlaywrightSidecarFailed
}

func (noopPlaywrightSidecarClient) InteractBrowser(_ context.Context, _ tools.BrowserInteractRequest) (tools.BrowserPageInteractResult, error) {
	return tools.BrowserPageInteractResult{}, tools.ErrPlaywrightSidecarFailed
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
		RiskHint:        "green",
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
	if _, _, err := optionalAttachConfigFromInput(input); err != nil {
		return err
	}
	return nil
}

func (t *PageReadTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.Playwright == nil {
		return nil, tools.ErrPlaywrightSidecarFailed
	}
	url := strings.TrimSpace(input["url"].(string))
	attach, attached, err := optionalAttachConfigFromInput(input)
	if err != nil {
		return nil, err
	}
	var result tools.BrowserPageReadResult
	if attached {
		result, err = execCtx.Playwright.ReadPageAttached(ctx, url, attach)
	} else {
		result, err = execCtx.Playwright.ReadPage(ctx, url)
	}
	if err != nil {
		return nil, err
	}
	rawOutput := browserExecutionMetadataOutput(result.BrowserExecutionMetadata)
	rawOutput["url"] = result.URL
	rawOutput["title"] = result.Title
	rawOutput["text_content"] = result.TextContent
	rawOutput["mime_type"] = result.MIMEType
	rawOutput["text_type"] = result.TextType
	rawOutput["source"] = firstNonEmptyString(result.Source, "playwright_sidecar")
	return &tools.ToolResult{
		ToolName:  t.meta.Name,
		RawOutput: rawOutput,
		SummaryOutput: map[string]any{
			"url":             result.URL,
			"title":           result.Title,
			"attached":        result.Attached,
			"browser_kind":    result.BrowserKind,
			"content_preview": previewPageText(result.TextContent),
			"source":          firstNonEmptyString(result.Source, "playwright_sidecar"),
		},
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
		RiskHint:        "green",
		TimeoutSec:      20,
		InputSchemaRef:  "tools/page_search/input",
		OutputSchemaRef: "tools/page_search/output",
		SupportsDryRun:  false,
	}}
}

type PageInteractTool struct {
	meta tools.ToolMetadata
}

type WebSearchTool struct {
	meta tools.ToolMetadata
}

func NewWebSearchTool() *WebSearchTool {
	return &WebSearchTool{meta: tools.ToolMetadata{
		Name:            "web_search",
		DisplayName:     "联网搜索",
		Description:     "通过 Playwright sidecar 执行互联网搜索并返回结构化结果摘要",
		Source:          tools.ToolSourceSidecar,
		RiskHint:        "green",
		TimeoutSec:      20,
		InputSchemaRef:  "tools/web_search/input",
		OutputSchemaRef: "tools/web_search/output",
		SupportsDryRun:  false,
	}}
}

func (t *WebSearchTool) Metadata() tools.ToolMetadata { return t.meta }

func (t *WebSearchTool) Validate(input map[string]any) error {
	query, ok := input["query"].(string)
	if !ok || strings.TrimSpace(query) == "" {
		return fmt.Errorf("input field 'query' must be a non-empty string")
	}
	return nil
}

func (t *WebSearchTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.Playwright == nil {
		return nil, tools.ErrPlaywrightSidecarFailed
	}

	request := tools.BrowserWebSearchRequest{
		Query: strings.TrimSpace(input["query"].(string)),
		URL:   strings.TrimSpace(stringValueMap(input, "url")),
		Limit: intValueMap(input, "limit"),
	}
	if request.Limit <= 0 {
		request.Limit = defaultPageSearchLimit
	}
	result, err := execCtx.Playwright.SearchWeb(ctx, request)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]any, 0, len(result.Results))
	for _, item := range result.Results {
		items = append(items, map[string]any{
			"title":   item.Title,
			"url":     item.URL,
			"snippet": item.Snippet,
		})
	}
	rawOutput := browserExecutionMetadataOutput(result.BrowserExecutionMetadata)
	rawOutput["query"] = result.Query
	rawOutput["search_url"] = result.SearchURL
	rawOutput["result_count"] = result.ResultCount
	rawOutput["results"] = items
	rawOutput["source"] = firstNonEmptyString(result.Source, "playwright_sidecar")
	return &tools.ToolResult{
		ToolName:  t.meta.Name,
		RawOutput: rawOutput,
		SummaryOutput: map[string]any{
			"query":           result.Query,
			"search_url":      result.SearchURL,
			"result_count":    result.ResultCount,
			"content_preview": previewWebSearchResult(result.Results),
			"source":          firstNonEmptyString(result.Source, "playwright_sidecar"),
		},
	}, nil
}

func NewPageInteractTool() *PageInteractTool {
	return &PageInteractTool{meta: tools.ToolMetadata{
		Name:            "page_interact",
		DisplayName:     "页面操作",
		Description:     "通过 Playwright sidecar 执行页面交互并返回最新页面摘要",
		Source:          tools.ToolSourceSidecar,
		RiskHint:        "yellow",
		TimeoutSec:      30,
		InputSchemaRef:  "tools/page_interact/input",
		OutputSchemaRef: "tools/page_interact/output",
		SupportsDryRun:  false,
	}}
}

func (t *PageInteractTool) Metadata() tools.ToolMetadata { return t.meta }

func (t *PageInteractTool) Validate(input map[string]any) error {
	url, ok := input["url"].(string)
	if !ok || strings.TrimSpace(url) == "" {
		return fmt.Errorf("input field 'url' must be a non-empty string")
	}
	if _, _, err := optionalAttachConfigFromInput(input); err != nil {
		return err
	}
	actions := mapSliceValue(input, "actions")
	if len(actions) == 0 {
		return fmt.Errorf("input field 'actions' must be a non-empty array")
	}
	for _, action := range actions {
		if strings.TrimSpace(stringValueMap(action, "type")) == "" {
			return fmt.Errorf("each page interaction action must include a non-empty type")
		}
	}
	return nil
}

func (t *PageInteractTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.Playwright == nil {
		return nil, tools.ErrPlaywrightSidecarFailed
	}
	url := strings.TrimSpace(input["url"].(string))
	actions := mapSliceValue(input, "actions")
	attach, attached, err := optionalAttachConfigFromInput(input)
	if err != nil {
		return nil, err
	}
	var result tools.BrowserPageInteractResult
	if attached {
		result, err = execCtx.Playwright.InteractPageAttached(ctx, url, actions, attach)
	} else {
		result, err = execCtx.Playwright.InteractPage(ctx, url, actions)
	}
	if err != nil {
		return nil, err
	}
	rawOutput := browserExecutionMetadataOutput(result.BrowserExecutionMetadata)
	rawOutput["url"] = result.URL
	rawOutput["title"] = result.Title
	rawOutput["text_content"] = result.TextContent
	rawOutput["actions_applied"] = result.ActionsApplied
	rawOutput["source"] = firstNonEmptyString(result.Source, "playwright_sidecar")
	return &tools.ToolResult{
		ToolName:  t.meta.Name,
		RawOutput: rawOutput,
		SummaryOutput: map[string]any{
			"url":             result.URL,
			"title":           result.Title,
			"attached":        result.Attached,
			"browser_kind":    result.BrowserKind,
			"content_preview": previewPageText(result.TextContent),
			"actions_applied": result.ActionsApplied,
			"source":          firstNonEmptyString(result.Source, "playwright_sidecar"),
		},
	}, nil
}

type StructuredDOMTool struct {
	meta tools.ToolMetadata
}

func NewStructuredDOMTool() *StructuredDOMTool {
	return &StructuredDOMTool{meta: tools.ToolMetadata{
		Name:            "structured_dom",
		DisplayName:     "结构化页面",
		Description:     "通过 Playwright sidecar 提取页面标题、标题层级、链接与交互元素摘要",
		Source:          tools.ToolSourceSidecar,
		RiskHint:        "yellow",
		TimeoutSec:      20,
		InputSchemaRef:  "tools/structured_dom/input",
		OutputSchemaRef: "tools/structured_dom/output",
		SupportsDryRun:  false,
	}}
}

func (t *StructuredDOMTool) Metadata() tools.ToolMetadata { return t.meta }

func (t *StructuredDOMTool) Validate(input map[string]any) error {
	url, ok := input["url"].(string)
	if !ok || strings.TrimSpace(url) == "" {
		return fmt.Errorf("input field 'url' must be a non-empty string")
	}
	if _, _, err := optionalAttachConfigFromInput(input); err != nil {
		return err
	}
	return nil
}

func (t *StructuredDOMTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.Playwright == nil {
		return nil, tools.ErrPlaywrightSidecarFailed
	}
	url := strings.TrimSpace(input["url"].(string))
	attach, attached, err := optionalAttachConfigFromInput(input)
	if err != nil {
		return nil, err
	}
	var result tools.BrowserStructuredDOMResult
	if attached {
		result, err = execCtx.Playwright.StructuredDOMAttached(ctx, url, attach)
	} else {
		result, err = execCtx.Playwright.StructuredDOM(ctx, url)
	}
	if err != nil {
		return nil, err
	}
	rawOutput := browserExecutionMetadataOutput(result.BrowserExecutionMetadata)
	rawOutput["url"] = result.URL
	rawOutput["title"] = result.Title
	rawOutput["headings"] = append([]string(nil), result.Headings...)
	rawOutput["links"] = append([]string(nil), result.Links...)
	rawOutput["buttons"] = append([]string(nil), result.Buttons...)
	rawOutput["inputs"] = append([]string(nil), result.Inputs...)
	rawOutput["source"] = firstNonEmptyString(result.Source, "playwright_sidecar")
	return &tools.ToolResult{
		ToolName:      t.meta.Name,
		RawOutput:     rawOutput,
		SummaryOutput: map[string]any{"url": result.URL, "title": result.Title, "attached": result.Attached, "browser_kind": result.BrowserKind, "heading_count": len(result.Headings), "link_count": len(result.Links), "button_count": len(result.Buttons), "input_count": len(result.Inputs), "source": firstNonEmptyString(result.Source, "playwright_sidecar")},
	}, nil
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
	if _, _, err := optionalAttachConfigFromInput(input); err != nil {
		return err
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
	attach, attached, err := optionalAttachConfigFromInput(input)
	if err != nil {
		return nil, err
	}
	var result tools.BrowserPageSearchResult
	if attached {
		result, err = execCtx.Playwright.SearchPageAttached(ctx, url, query, limit, attach)
	} else {
		result, err = execCtx.Playwright.SearchPage(ctx, url, query, limit)
	}
	if err != nil {
		return nil, err
	}
	rawOutput := browserExecutionMetadataOutput(result.BrowserExecutionMetadata)
	rawOutput["url"] = result.URL
	rawOutput["query"] = result.Query
	rawOutput["match_count"] = result.MatchCount
	rawOutput["matches"] = append([]string(nil), result.Matches...)
	rawOutput["source"] = firstNonEmptyString(result.Source, "playwright_sidecar")
	return &tools.ToolResult{
		ToolName:      t.meta.Name,
		RawOutput:     rawOutput,
		SummaryOutput: map[string]any{"url": result.URL, "query": result.Query, "attached": result.Attached, "browser_kind": result.BrowserKind, "match_count": result.MatchCount, "source": firstNonEmptyString(result.Source, "playwright_sidecar")},
	}, nil
}

func RegisterPlaywrightTools(registry *tools.Registry) error {
	for _, tool := range []tools.Tool{
		NewPageReadTool(),
		NewPageSearchTool(),
		NewWebSearchTool(),
		NewPageInteractTool(),
		NewStructuredDOMTool(),
		NewBrowserAttachCurrentTool(),
		NewBrowserSnapshotTool(),
		NewBrowserNavigateTool(),
		NewBrowserTabsListTool(),
		NewBrowserTabFocusTool(),
		NewBrowserInteractTool(),
	} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

func previewWebSearchResult(results []tools.BrowserSearchResultItem) string {
	if len(results) == 0 {
		return ""
	}
	first := strings.TrimSpace(results[0].Title)
	if snippet := strings.TrimSpace(results[0].Snippet); snippet != "" {
		if first != "" {
			return previewPageText(first + ": " + snippet)
		}
		return previewPageText(snippet)
	}
	return previewPageText(first)
}

func mapSliceValue(values map[string]any, key string) []map[string]any {
	switch typed := values[key].(type) {
	case []any:
		items := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if action, ok := item.(map[string]any); ok {
				items = append(items, cloneActionMap(action))
			}
		}
		return items
	case []map[string]any:
		return cloneActionSlice(typed)
	default:
		return nil
	}
}

func cloneActionMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneActionSlice(values []map[string]any) []map[string]any {
	if len(values) == 0 {
		return nil
	}
	items := make([]map[string]any, 0, len(values))
	for _, value := range values {
		items = append(items, cloneActionMap(value))
	}
	return items
}

func stringValueMap(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func intValueMap(values map[string]any, key string) int {
	if len(values) == 0 {
		return 0
	}
	switch value := values[key].(type) {
	case int:
		return value
	case float64:
		return int(value)
	default:
		return 0
	}
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

func optionalAttachConfigFromInput(input map[string]any) (tools.BrowserAttachConfig, bool, error) {
	if _, ok := input["attach"]; !ok {
		return tools.BrowserAttachConfig{}, false, nil
	}
	attach, err := attachConfigFromInput(input)
	if err != nil {
		return tools.BrowserAttachConfig{}, false, err
	}
	return attach, true, nil
}
