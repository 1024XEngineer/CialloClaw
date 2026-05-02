package sidecarclient

import (
	"context"
	"fmt"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

var supportedAttachedBrowserKinds = map[string]struct{}{
	"chrome": {},
	"edge":   {},
}

type BrowserAttachCurrentTool struct {
	meta tools.ToolMetadata
}

// NewBrowserAttachCurrentTool returns the attach-only tool that resolves the
// user's current Chromium tab selection before any mutating action is attempted.
func NewBrowserAttachCurrentTool() *BrowserAttachCurrentTool {
	return &BrowserAttachCurrentTool{meta: tools.ToolMetadata{
		Name:            "browser_attach_current",
		DisplayName:     "附着浏览器当前页",
		Description:     "通过 Playwright sidecar 附着用户当前 Chromium 浏览器页并返回匹配结果",
		Source:          tools.ToolSourceSidecar,
		RiskHint:        "green",
		TimeoutSec:      20,
		InputSchemaRef:  "tools/browser_attach_current/input",
		OutputSchemaRef: "tools/browser_attach_current/output",
		SupportsDryRun:  false,
	}}
}

func (t *BrowserAttachCurrentTool) Metadata() tools.ToolMetadata { return t.meta }

func (t *BrowserAttachCurrentTool) Validate(input map[string]any) error {
	_, err := attachConfigFromInput(input)
	return err
}

func (t *BrowserAttachCurrentTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.Playwright == nil {
		return nil, tools.ErrPlaywrightSidecarFailed
	}
	attach, err := attachConfigFromInput(input)
	if err != nil {
		return nil, err
	}
	result, err := execCtx.Playwright.AttachCurrentPage(ctx, attach)
	if err != nil {
		return nil, err
	}
	rawOutput := browserAttachedPageRawOutput(result)
	return &tools.ToolResult{
		ToolName:      t.meta.Name,
		RawOutput:     rawOutput,
		SummaryOutput: map[string]any{"url": result.URL, "title": result.Title, "page_index": result.PageIndex, "browser_kind": result.BrowserKind, "source": firstNonEmptyString(result.Source, "playwright_sidecar")},
	}, nil
}

type BrowserSnapshotTool struct {
	meta tools.ToolMetadata
}

// NewBrowserSnapshotTool returns the tool that reads the currently attached
// browser tab without navigating away from the user's real browser state.
func NewBrowserSnapshotTool() *BrowserSnapshotTool {
	return &BrowserSnapshotTool{meta: tools.ToolMetadata{
		Name:            "browser_snapshot",
		DisplayName:     "浏览器快照",
		Description:     "通过 Playwright sidecar 读取当前附着浏览器页的文本与结构化摘要",
		Source:          tools.ToolSourceSidecar,
		RiskHint:        "green",
		TimeoutSec:      20,
		InputSchemaRef:  "tools/browser_snapshot/input",
		OutputSchemaRef: "tools/browser_snapshot/output",
		SupportsDryRun:  false,
	}}
}

func (t *BrowserSnapshotTool) Metadata() tools.ToolMetadata { return t.meta }

func (t *BrowserSnapshotTool) Validate(input map[string]any) error {
	_, err := attachConfigFromInput(input)
	return err
}

func (t *BrowserSnapshotTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.Playwright == nil {
		return nil, tools.ErrPlaywrightSidecarFailed
	}
	attach, err := attachConfigFromInput(input)
	if err != nil {
		return nil, err
	}
	result, err := execCtx.Playwright.SnapshotBrowser(ctx, attach)
	if err != nil {
		return nil, err
	}
	rawOutput := browserAttachedPageRawOutput(result.BrowserAttachedPageResult)
	rawOutput["text_content"] = result.TextContent
	rawOutput["headings"] = append([]string(nil), result.Headings...)
	rawOutput["links"] = append([]string(nil), result.Links...)
	rawOutput["buttons"] = append([]string(nil), result.Buttons...)
	rawOutput["inputs"] = append([]string(nil), result.Inputs...)
	return &tools.ToolResult{
		ToolName:  t.meta.Name,
		RawOutput: rawOutput,
		SummaryOutput: map[string]any{
			"url":             result.URL,
			"title":           result.Title,
			"page_index":      result.PageIndex,
			"browser_kind":    result.BrowserKind,
			"content_preview": previewPageText(result.TextContent),
			"heading_count":   len(result.Headings),
			"link_count":      len(result.Links),
			"button_count":    len(result.Buttons),
			"input_count":     len(result.Inputs),
			"source":          firstNonEmptyString(result.Source, "playwright_sidecar"),
		},
	}, nil
}

type BrowserNavigateTool struct {
	meta tools.ToolMetadata
}

// NewBrowserNavigateTool returns the tool that navigates the currently
// attached browser tab through the real-browser CDP session.
func NewBrowserNavigateTool() *BrowserNavigateTool {
	return &BrowserNavigateTool{meta: tools.ToolMetadata{
		Name:            "browser_navigate",
		DisplayName:     "浏览器导航",
		Description:     "通过 Playwright sidecar 在当前附着浏览器页中导航到指定 URL",
		Source:          tools.ToolSourceSidecar,
		RiskHint:        "yellow",
		TimeoutSec:      30,
		InputSchemaRef:  "tools/browser_navigate/input",
		OutputSchemaRef: "tools/browser_navigate/output",
		SupportsDryRun:  false,
	}}
}

func (t *BrowserNavigateTool) Metadata() tools.ToolMetadata { return t.meta }

func (t *BrowserNavigateTool) Validate(input map[string]any) error {
	if _, err := attachConfigFromInput(input); err != nil {
		return err
	}
	url, ok := input["url"].(string)
	if !ok || strings.TrimSpace(url) == "" {
		return fmt.Errorf("input field 'url' must be a non-empty string")
	}
	return nil
}

func (t *BrowserNavigateTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.Playwright == nil {
		return nil, tools.ErrPlaywrightSidecarFailed
	}
	attach, err := attachConfigFromInput(input)
	if err != nil {
		return nil, err
	}
	request := tools.BrowserNavigateRequest{Attach: attach, URL: strings.TrimSpace(input["url"].(string))}
	result, err := execCtx.Playwright.NavigateBrowser(ctx, request)
	if err != nil {
		return nil, err
	}
	rawOutput := browserAttachedPageRawOutput(result.BrowserAttachedPageResult)
	rawOutput["text_content"] = result.TextContent
	rawOutput["mime_type"] = result.MIMEType
	rawOutput["text_type"] = result.TextType
	return &tools.ToolResult{
		ToolName:  t.meta.Name,
		RawOutput: rawOutput,
		SummaryOutput: map[string]any{
			"url":             result.URL,
			"title":           result.Title,
			"page_index":      result.PageIndex,
			"browser_kind":    result.BrowserKind,
			"content_preview": previewPageText(result.TextContent),
			"source":          firstNonEmptyString(result.Source, "playwright_sidecar"),
		},
	}, nil
}

type BrowserTabsListTool struct {
	meta tools.ToolMetadata
}

// NewBrowserTabsListTool returns the tool that lists attached browser tabs so
// the planner can reason about the user's real browsing context.
func NewBrowserTabsListTool() *BrowserTabsListTool {
	return &BrowserTabsListTool{meta: tools.ToolMetadata{
		Name:            "browser_tabs_list",
		DisplayName:     "浏览器标签页列表",
		Description:     "通过 Playwright sidecar 列出当前附着浏览器的标签页摘要",
		Source:          tools.ToolSourceSidecar,
		RiskHint:        "green",
		TimeoutSec:      20,
		InputSchemaRef:  "tools/browser_tabs_list/input",
		OutputSchemaRef: "tools/browser_tabs_list/output",
		SupportsDryRun:  false,
	}}
}

func (t *BrowserTabsListTool) Metadata() tools.ToolMetadata { return t.meta }

func (t *BrowserTabsListTool) Validate(input map[string]any) error {
	_, err := attachConfigFromInput(input)
	return err
}

func (t *BrowserTabsListTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.Playwright == nil {
		return nil, tools.ErrPlaywrightSidecarFailed
	}
	attach, err := attachConfigFromInput(input)
	if err != nil {
		return nil, err
	}
	result, err := execCtx.Playwright.ListBrowserTabs(ctx, attach)
	if err != nil {
		return nil, err
	}
	rawOutput := browserExecutionMetadataOutput(result.BrowserExecutionMetadata)
	rawOutput["tab_count"] = result.TabCount
	rawOutput["tabs"] = browserTabOutputItems(result.Tabs)
	rawOutput["source"] = firstNonEmptyString(result.Source, "playwright_sidecar")
	return &tools.ToolResult{
		ToolName:  t.meta.Name,
		RawOutput: rawOutput,
		SummaryOutput: map[string]any{
			"tab_count":     result.TabCount,
			"browser_kind":  result.BrowserKind,
			"first_tab_url": firstBrowserTabURL(result.Tabs),
			"source":        firstNonEmptyString(result.Source, "playwright_sidecar"),
		},
	}, nil
}

type BrowserTabFocusTool struct {
	meta tools.ToolMetadata
}

// NewBrowserTabFocusTool returns the tool that focuses one attached browser
// tab before follow-up inspection or interaction.
func NewBrowserTabFocusTool() *BrowserTabFocusTool {
	return &BrowserTabFocusTool{meta: tools.ToolMetadata{
		Name:            "browser_tab_focus",
		DisplayName:     "聚焦浏览器标签页",
		Description:     "通过 Playwright sidecar 聚焦当前附着浏览器中的目标标签页",
		Source:          tools.ToolSourceSidecar,
		RiskHint:        "yellow",
		TimeoutSec:      20,
		InputSchemaRef:  "tools/browser_tab_focus/input",
		OutputSchemaRef: "tools/browser_tab_focus/output",
		SupportsDryRun:  false,
	}}
}

func (t *BrowserTabFocusTool) Metadata() tools.ToolMetadata { return t.meta }

func (t *BrowserTabFocusTool) Validate(input map[string]any) error {
	_, err := attachConfigFromInput(input)
	return err
}

func (t *BrowserTabFocusTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.Playwright == nil {
		return nil, tools.ErrPlaywrightSidecarFailed
	}
	attach, err := attachConfigFromInput(input)
	if err != nil {
		return nil, err
	}
	result, err := execCtx.Playwright.FocusBrowserTab(ctx, attach)
	if err != nil {
		return nil, err
	}
	rawOutput := browserAttachedPageRawOutput(result)
	return &tools.ToolResult{
		ToolName:      t.meta.Name,
		RawOutput:     rawOutput,
		SummaryOutput: map[string]any{"url": result.URL, "title": result.Title, "page_index": result.PageIndex, "browser_kind": result.BrowserKind, "source": firstNonEmptyString(result.Source, "playwright_sidecar")},
	}, nil
}

type BrowserInteractTool struct {
	meta tools.ToolMetadata
}

// NewBrowserInteractTool returns the tool that performs controlled interactions
// against the currently attached real-browser tab.
func NewBrowserInteractTool() *BrowserInteractTool {
	return &BrowserInteractTool{meta: tools.ToolMetadata{
		Name:            "browser_interact",
		DisplayName:     "浏览器交互",
		Description:     "通过 Playwright sidecar 在当前附着浏览器页执行交互并返回最新页面摘要",
		Source:          tools.ToolSourceSidecar,
		RiskHint:        "yellow",
		TimeoutSec:      30,
		InputSchemaRef:  "tools/browser_interact/input",
		OutputSchemaRef: "tools/browser_interact/output",
		SupportsDryRun:  false,
	}}
}

func (t *BrowserInteractTool) Metadata() tools.ToolMetadata { return t.meta }

func (t *BrowserInteractTool) Validate(input map[string]any) error {
	if _, err := attachConfigFromInput(input); err != nil {
		return err
	}
	actions := mapSliceValue(input, "actions")
	if len(actions) == 0 {
		return fmt.Errorf("input field 'actions' must be a non-empty array")
	}
	for _, action := range actions {
		if strings.TrimSpace(stringValueMap(action, "type")) == "" {
			return fmt.Errorf("each browser interaction action must include a non-empty type")
		}
	}
	return nil
}

func (t *BrowserInteractTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.Playwright == nil {
		return nil, tools.ErrPlaywrightSidecarFailed
	}
	attach, err := attachConfigFromInput(input)
	if err != nil {
		return nil, err
	}
	request := tools.BrowserInteractRequest{Attach: attach, Actions: mapSliceValue(input, "actions")}
	result, err := execCtx.Playwright.InteractBrowser(ctx, request)
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
			"browser_kind":    result.BrowserKind,
			"content_preview": previewPageText(result.TextContent),
			"actions_applied": result.ActionsApplied,
			"source":          firstNonEmptyString(result.Source, "playwright_sidecar"),
		},
	}, nil
}

func attachConfigFromInput(input map[string]any) (tools.BrowserAttachConfig, error) {
	attachInput, ok := input["attach"].(map[string]any)
	if !ok || len(attachInput) == 0 {
		return tools.BrowserAttachConfig{}, fmt.Errorf("input field 'attach' must be an object")
	}
	mode := tools.BrowserAttachMode(firstNonEmptyString(stringValueMap(attachInput, "mode"), string(tools.BrowserAttachModeCDP)))
	if mode != tools.BrowserAttachModeCDP {
		return tools.BrowserAttachConfig{}, fmt.Errorf("attach.mode must be 'cdp'")
	}
	browserKind := strings.ToLower(strings.TrimSpace(stringValueMap(attachInput, "browser_kind")))
	if browserKind == "" {
		return tools.BrowserAttachConfig{}, fmt.Errorf("attach.browser_kind must be a non-empty string")
	}
	if _, ok := supportedAttachedBrowserKinds[browserKind]; !ok {
		return tools.BrowserAttachConfig{}, fmt.Errorf("attach.browser_kind must be one of chrome or edge")
	}
	targetInput, ok := attachInput["target"].(map[string]any)
	if !ok || len(targetInput) == 0 {
		return tools.BrowserAttachConfig{}, fmt.Errorf("attach.target must be an object")
	}
	target := tools.BrowserAttachTarget{
		URL:           strings.TrimSpace(stringValueMap(targetInput, "url")),
		TitleContains: strings.TrimSpace(stringValueMap(targetInput, "title_contains")),
		PageIndex:     intPointerValue(targetInput, "page_index"),
	}
	if _, exists := targetInput["page_index"]; exists && target.PageIndex == nil {
		return tools.BrowserAttachConfig{}, fmt.Errorf("attach.target.page_index must be a non-negative integer")
	}
	if target.URL == "" && target.TitleContains == "" && target.PageIndex == nil {
		return tools.BrowserAttachConfig{}, fmt.Errorf("attach.target must include at least one of url, title_contains, or page_index")
	}
	return tools.BrowserAttachConfig{
		Mode:        mode,
		BrowserKind: browserKind,
		EndpointURL: strings.TrimSpace(stringValueMap(attachInput, "endpoint_url")),
		Target:      target,
	}, nil
}

func intPointerValue(values map[string]any, key string) *int {
	if len(values) == 0 {
		return nil
	}
	var value int
	switch typed := values[key].(type) {
	case int:
		value = typed
	case float64:
		if typed != float64(int(typed)) {
			return nil
		}
		value = int(typed)
	default:
		return nil
	}
	if value < 0 {
		return nil
	}
	return &value
}

func browserExecutionMetadataOutput(metadata tools.BrowserExecutionMetadata) map[string]any {
	return map[string]any{
		"attached":          metadata.Attached,
		"browser_kind":      metadata.BrowserKind,
		"browser_transport": metadata.BrowserTransport,
		"endpoint_url":      metadata.EndpointURL,
	}
}

func browserAttachedPageRawOutput(result tools.BrowserAttachedPageResult) map[string]any {
	rawOutput := browserExecutionMetadataOutput(result.BrowserExecutionMetadata)
	rawOutput["page_index"] = result.PageIndex
	rawOutput["title"] = result.Title
	rawOutput["url"] = result.URL
	rawOutput["source"] = firstNonEmptyString(result.Source, "playwright_sidecar")
	return rawOutput
}

func browserTabOutputItems(items []tools.BrowserTabInfo) []map[string]any {
	if len(items) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"page_index": item.PageIndex,
			"title":      item.Title,
			"url":        item.URL,
		})
	}
	return result
}

func firstBrowserTabURL(items []tools.BrowserTabInfo) string {
	if len(items) == 0 {
		return ""
	}
	return strings.TrimSpace(items[0].URL)
}
