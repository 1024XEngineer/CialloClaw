package sidecarclient

import (
	"context"
	"errors"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func attachedBrowserInput() map[string]any {
	return map[string]any{
		"attach": map[string]any{
			"mode":         "cdp",
			"browser_kind": "chrome",
			"target": map[string]any{
				"url": "https://example.com/docs",
			},
		},
	}
}

func attachedBrowserInputWithoutTarget() map[string]any {
	return map[string]any{
		"attach": map[string]any{
			"mode":         "cdp",
			"browser_kind": "chrome",
		},
	}
}

type stubPlaywrightClient struct {
	readResult       tools.BrowserPageReadResult
	searchResult     tools.BrowserPageSearchResult
	interactResult   tools.BrowserPageInteractResult
	structuredResult tools.BrowserStructuredDOMResult
	attachedRead     tools.BrowserPageReadResult
	attachedSearch   tools.BrowserPageSearchResult
	attachedInteract tools.BrowserPageInteractResult
	attachedDOM      tools.BrowserStructuredDOMResult
	attachResult     tools.BrowserAttachedPageResult
	snapshotResult   tools.BrowserSnapshotResult
	navigateResult   tools.BrowserNavigationResult
	tabsResult       tools.BrowserTabsListResult
	err              error
}

func (s stubPlaywrightClient) ReadPage(_ context.Context, url string) (tools.BrowserPageReadResult, error) {
	if s.err != nil {
		return tools.BrowserPageReadResult{}, s.err
	}
	result := s.readResult
	if result.URL == "" {
		result.URL = url
	}
	return result, nil
}

func (s stubPlaywrightClient) SearchPage(_ context.Context, url, query string, limit int) (tools.BrowserPageSearchResult, error) {
	if s.err != nil {
		return tools.BrowserPageSearchResult{}, s.err
	}
	result := s.searchResult
	if result.URL == "" {
		result.URL = url
	}
	if result.Query == "" {
		result.Query = query
	}
	if limit > 0 && len(result.Matches) > limit {
		result.Matches = result.Matches[:limit]
		result.MatchCount = len(result.Matches)
	}
	return result, nil
}

func (s stubPlaywrightClient) InteractPage(_ context.Context, url string, _ []map[string]any) (tools.BrowserPageInteractResult, error) {
	if s.err != nil {
		return tools.BrowserPageInteractResult{}, s.err
	}
	result := s.interactResult
	if result.URL == "" {
		result.URL = url
	}
	return result, nil
}

func (s stubPlaywrightClient) StructuredDOM(_ context.Context, url string) (tools.BrowserStructuredDOMResult, error) {
	if s.err != nil {
		return tools.BrowserStructuredDOMResult{}, s.err
	}
	result := s.structuredResult
	if result.URL == "" {
		result.URL = url
	}
	return result, nil
}

func (s stubPlaywrightClient) ReadPageAttached(_ context.Context, url string, attach tools.BrowserAttachConfig) (tools.BrowserPageReadResult, error) {
	if s.err != nil {
		return tools.BrowserPageReadResult{}, s.err
	}
	result := s.attachedRead
	if result.URL == "" {
		result.URL = url
	}
	result.Attached = true
	if result.BrowserKind == "" {
		result.BrowserKind = attach.BrowserKind
	}
	return result, nil
}

func (s stubPlaywrightClient) SearchPageAttached(_ context.Context, url, query string, limit int, attach tools.BrowserAttachConfig) (tools.BrowserPageSearchResult, error) {
	if s.err != nil {
		return tools.BrowserPageSearchResult{}, s.err
	}
	result := s.attachedSearch
	if result.URL == "" {
		result.URL = url
	}
	if result.Query == "" {
		result.Query = query
	}
	if limit > 0 && len(result.Matches) > limit {
		result.Matches = result.Matches[:limit]
		result.MatchCount = len(result.Matches)
	}
	result.Attached = true
	if result.BrowserKind == "" {
		result.BrowserKind = attach.BrowserKind
	}
	return result, nil
}

func (s stubPlaywrightClient) InteractPageAttached(_ context.Context, url string, _ []map[string]any, attach tools.BrowserAttachConfig) (tools.BrowserPageInteractResult, error) {
	if s.err != nil {
		return tools.BrowserPageInteractResult{}, s.err
	}
	result := s.attachedInteract
	if result.URL == "" {
		result.URL = url
	}
	result.Attached = true
	if result.BrowserKind == "" {
		result.BrowserKind = attach.BrowserKind
	}
	return result, nil
}

func (s stubPlaywrightClient) StructuredDOMAttached(_ context.Context, url string, attach tools.BrowserAttachConfig) (tools.BrowserStructuredDOMResult, error) {
	if s.err != nil {
		return tools.BrowserStructuredDOMResult{}, s.err
	}
	result := s.attachedDOM
	if result.URL == "" {
		result.URL = url
	}
	result.Attached = true
	if result.BrowserKind == "" {
		result.BrowserKind = attach.BrowserKind
	}
	return result, nil
}

func (s stubPlaywrightClient) AttachCurrentPage(_ context.Context, _ tools.BrowserAttachConfig) (tools.BrowserAttachedPageResult, error) {
	if s.err != nil {
		return tools.BrowserAttachedPageResult{}, s.err
	}
	return s.attachResult, nil
}

func (s stubPlaywrightClient) SnapshotBrowser(_ context.Context, _ tools.BrowserAttachConfig) (tools.BrowserSnapshotResult, error) {
	if s.err != nil {
		return tools.BrowserSnapshotResult{}, s.err
	}
	return s.snapshotResult, nil
}

func (s stubPlaywrightClient) NavigateBrowser(_ context.Context, _ tools.BrowserNavigateRequest) (tools.BrowserNavigationResult, error) {
	if s.err != nil {
		return tools.BrowserNavigationResult{}, s.err
	}
	return s.navigateResult, nil
}

func (s stubPlaywrightClient) ListBrowserTabs(_ context.Context, _ tools.BrowserAttachConfig) (tools.BrowserTabsListResult, error) {
	if s.err != nil {
		return tools.BrowserTabsListResult{}, s.err
	}
	return s.tabsResult, nil
}

func (s stubPlaywrightClient) FocusBrowserTab(_ context.Context, _ tools.BrowserAttachConfig) (tools.BrowserAttachedPageResult, error) {
	if s.err != nil {
		return tools.BrowserAttachedPageResult{}, s.err
	}
	return s.attachResult, nil
}

func (s stubPlaywrightClient) InteractBrowser(_ context.Context, _ tools.BrowserInteractRequest) (tools.BrowserPageInteractResult, error) {
	if s.err != nil {
		return tools.BrowserPageInteractResult{}, s.err
	}
	return s.interactResult, nil
}

func TestPageReadToolExecuteSuccess(t *testing.T) {
	tool := NewPageReadTool()
	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{
		Playwright: stubPlaywrightClient{readResult: tools.BrowserPageReadResult{
			Title:       "Demo Page",
			TextContent: "hello world from page",
			MIMEType:    "text/html",
			TextType:    "text/html",
			Source:      "playwright_sidecar",
		}},
	}, map[string]any{"url": "https://example.com"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.RawOutput["url"] != "https://example.com" {
		t.Fatalf("unexpected raw output: %+v", result.RawOutput)
	}
	if result.SummaryOutput["title"] != "Demo Page" {
		t.Fatalf("unexpected summary output: %+v", result.SummaryOutput)
	}
}

func TestPageReadToolReturnsSidecarErrorWhenUnavailable(t *testing.T) {
	tool := NewPageReadTool()
	_, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{}, map[string]any{"url": "https://example.com"})
	if !errors.Is(err, tools.ErrPlaywrightSidecarFailed) {
		t.Fatalf("expected ErrPlaywrightSidecarFailed, got %v", err)
	}
}

func TestPlaywrightNoopClientAndValidators(t *testing.T) {
	client := NewNoopPlaywrightSidecarClient()
	if _, err := client.ReadPage(context.Background(), "https://example.com"); !errors.Is(err, tools.ErrPlaywrightSidecarFailed) {
		t.Fatalf("expected noop read failure, got %v", err)
	}
	if _, err := client.InteractPage(context.Background(), "https://example.com", nil); !errors.Is(err, tools.ErrPlaywrightSidecarFailed) {
		t.Fatalf("expected noop interact failure, got %v", err)
	}
	if _, err := client.AttachCurrentPage(context.Background(), tools.BrowserAttachConfig{Mode: tools.BrowserAttachModeCDP}); !errors.Is(err, tools.ErrPlaywrightSidecarFailed) {
		t.Fatalf("expected noop attach failure, got %v", err)
	}
	if err := NewPageReadTool().Validate(map[string]any{"url": "https://example.com"}); err != nil {
		t.Fatalf("expected page_read validate to pass, got %v", err)
	}
	if err := NewPageSearchTool().Validate(map[string]any{"url": "https://example.com", "query": "demo"}); err != nil {
		t.Fatalf("expected page_search validate to pass, got %v", err)
	}
	if err := NewStructuredDOMTool().Validate(map[string]any{"url": "https://example.com"}); err != nil {
		t.Fatalf("expected structured_dom validate to pass, got %v", err)
	}
}

func TestPageSearchToolExecuteSuccess(t *testing.T) {
	tool := NewPageSearchTool()
	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{
		Playwright: stubPlaywrightClient{searchResult: tools.BrowserPageSearchResult{
			Matches:    []string{"alpha", "beta", "gamma"},
			MatchCount: 3,
			Source:     "playwright_sidecar",
		}},
	}, map[string]any{"url": "https://example.com", "query": "alpha", "limit": 2})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.RawOutput["match_count"] != 2 {
		t.Fatalf("expected limited match_count, got %+v", result.RawOutput)
	}
}

func TestPageInteractToolExecuteSuccess(t *testing.T) {
	tool := NewPageInteractTool()
	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{
		Playwright: stubPlaywrightClient{interactResult: tools.BrowserPageInteractResult{
			Title:          "Demo Page",
			TextContent:    "interaction complete",
			ActionsApplied: 2,
			Source:         "playwright_sidecar",
		}},
	}, map[string]any{"url": "https://example.com", "actions": []any{map[string]any{"type": "click", "selector": "button"}, map[string]any{"type": "fill", "selector": "input", "value": "demo"}}})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.RawOutput["actions_applied"] != 2 {
		t.Fatalf("expected applied action count, got %+v", result.RawOutput)
	}
}

func TestStructuredDOMToolExecuteSuccess(t *testing.T) {
	tool := NewStructuredDOMTool()
	result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{
		Playwright: stubPlaywrightClient{structuredResult: tools.BrowserStructuredDOMResult{
			Title:    "Demo Page",
			Headings: []string{"Heading A"},
			Links:    []string{"Link A"},
			Buttons:  []string{"Submit"},
			Inputs:   []string{"email"},
			Source:   "playwright_sidecar",
		}},
	}, map[string]any{"url": "https://example.com"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.SummaryOutput["heading_count"] != 1 {
		t.Fatalf("expected heading count summary, got %+v", result.SummaryOutput)
	}
}

func TestPageToolsExecuteAttachedBrowserSuccess(t *testing.T) {
	client := stubPlaywrightClient{
		attachedRead:     tools.BrowserPageReadResult{Title: "Attached Read", TextContent: "attached text", Source: "playwright_worker_cdp"},
		attachedSearch:   tools.BrowserPageSearchResult{Matches: []string{"attached match"}, MatchCount: 1, Source: "playwright_worker_cdp"},
		attachedInteract: tools.BrowserPageInteractResult{Title: "Attached Interact", TextContent: "clicked", ActionsApplied: 1, Source: "playwright_worker_cdp"},
		attachedDOM:      tools.BrowserStructuredDOMResult{Title: "Attached DOM", Headings: []string{"Heading"}, Links: []string{"Docs"}, Buttons: []string{"Submit"}, Inputs: []string{"search"}, Source: "playwright_worker_cdp"},
	}
	execCtx := &tools.ToolExecuteContext{Playwright: client}

	readResult, err := NewPageReadTool().Execute(context.Background(), execCtx, map[string]any{"url": "https://example.com/docs", "attach": attachedBrowserInput()["attach"]})
	if err != nil || readResult.RawOutput["attached"] != true {
		t.Fatalf("expected attached page_read result, got result=%+v err=%v", readResult, err)
	}
	searchResult, err := NewPageSearchTool().Execute(context.Background(), execCtx, map[string]any{"url": "https://example.com/docs", "query": "attached", "attach": attachedBrowserInput()["attach"]})
	if err != nil || searchResult.RawOutput["attached"] != true {
		t.Fatalf("expected attached page_search result, got result=%+v err=%v", searchResult, err)
	}
	interactResult, err := NewPageInteractTool().Execute(context.Background(), execCtx, map[string]any{"url": "https://example.com/docs", "actions": []any{map[string]any{"type": "click", "selector": "button"}}, "attach": attachedBrowserInput()["attach"]})
	if err != nil || interactResult.RawOutput["attached"] != true {
		t.Fatalf("expected attached page_interact result, got result=%+v err=%v", interactResult, err)
	}
	domResult, err := NewStructuredDOMTool().Execute(context.Background(), execCtx, map[string]any{"url": "https://example.com/docs", "attach": attachedBrowserInput()["attach"]})
	if err != nil || domResult.RawOutput["attached"] != true {
		t.Fatalf("expected attached structured_dom result, got result=%+v err=%v", domResult, err)
	}
}

func TestBrowserAttachToolsExecuteSuccess(t *testing.T) {
	execCtx := &tools.ToolExecuteContext{Playwright: stubPlaywrightClient{
		attachResult: tools.BrowserAttachedPageResult{
			BrowserExecutionMetadata: tools.BrowserExecutionMetadata{Attached: true, BrowserKind: "chrome", BrowserTransport: "cdp", EndpointURL: "http://127.0.0.1:9222"},
			PageIndex:                2,
			Title:                    "Docs",
			URL:                      "https://example.com/docs",
			Source:                   "playwright_worker_cdp",
		},
		snapshotResult: tools.BrowserSnapshotResult{
			BrowserAttachedPageResult: tools.BrowserAttachedPageResult{
				BrowserExecutionMetadata: tools.BrowserExecutionMetadata{Attached: true, BrowserKind: "chrome", BrowserTransport: "cdp", EndpointURL: "http://127.0.0.1:9222"},
				PageIndex:                2,
				Title:                    "Docs",
				URL:                      "https://example.com/docs",
				Source:                   "playwright_worker_cdp",
			},
			TextContent: "Install guide",
			Headings:    []string{"Install"},
			Links:       []string{"Guide"},
			Buttons:     []string{"Next"},
			Inputs:      []string{"search"},
		},
		navigateResult: tools.BrowserNavigationResult{
			BrowserAttachedPageResult: tools.BrowserAttachedPageResult{
				BrowserExecutionMetadata: tools.BrowserExecutionMetadata{Attached: true, BrowserKind: "chrome", BrowserTransport: "cdp", EndpointURL: "http://127.0.0.1:9222"},
				PageIndex:                2,
				Title:                    "Start",
				URL:                      "https://example.com/docs/start",
				Source:                   "playwright_worker_cdp",
			},
			TextContent: "Getting started",
			MIMEType:    "text/html",
			TextType:    "text/html",
		},
		tabsResult: tools.BrowserTabsListResult{
			BrowserExecutionMetadata: tools.BrowserExecutionMetadata{Attached: true, BrowserKind: "chrome", BrowserTransport: "cdp", EndpointURL: "http://127.0.0.1:9222"},
			TabCount:                 2,
			Tabs:                     []tools.BrowserTabInfo{{PageIndex: 0, Title: "Home", URL: "https://example.com"}, {PageIndex: 2, Title: "Docs", URL: "https://example.com/docs"}},
			Source:                   "playwright_worker_cdp",
		},
		interactResult: tools.BrowserPageInteractResult{
			BrowserExecutionMetadata: tools.BrowserExecutionMetadata{Attached: true, BrowserKind: "chrome", BrowserTransport: "cdp", EndpointURL: "http://127.0.0.1:9222"},
			Title:                    "Docs",
			URL:                      "https://example.com/docs",
			TextContent:              "Clicked next",
			ActionsApplied:           1,
			Source:                   "playwright_worker_cdp",
		},
	}}

	attachResult, err := NewBrowserAttachCurrentTool().Execute(context.Background(), execCtx, attachedBrowserInput())
	if err != nil || attachResult.RawOutput["page_index"] != 2 {
		t.Fatalf("unexpected browser_attach_current result=%+v err=%v", attachResult, err)
	}

	snapshotResult, err := NewBrowserSnapshotTool().Execute(context.Background(), execCtx, attachedBrowserInput())
	if err != nil || snapshotResult.SummaryOutput["heading_count"] != 1 {
		t.Fatalf("unexpected browser_snapshot result=%+v err=%v", snapshotResult, err)
	}

	navigateInput := attachedBrowserInput()
	navigateInput["url"] = "https://example.com/docs/start"
	navigateResult, err := NewBrowserNavigateTool().Execute(context.Background(), execCtx, navigateInput)
	if err != nil || navigateResult.SummaryOutput["content_preview"] != "Getting started" {
		t.Fatalf("unexpected browser_navigate result=%+v err=%v", navigateResult, err)
	}

	tabsResult, err := NewBrowserTabsListTool().Execute(context.Background(), execCtx, attachedBrowserInputWithoutTarget())
	if err != nil || tabsResult.SummaryOutput["tab_count"] != 2 {
		t.Fatalf("unexpected browser_tabs_list result=%+v err=%v", tabsResult, err)
	}

	focusInput := attachedBrowserInput()
	focusInput["attach"].(map[string]any)["target"] = map[string]any{"page_index": 2.0}
	focusResult, err := NewBrowserTabFocusTool().Execute(context.Background(), execCtx, focusInput)
	if err != nil || focusResult.SummaryOutput["page_index"] != 2 {
		t.Fatalf("unexpected browser_tab_focus result=%+v err=%v", focusResult, err)
	}

	interactInput := attachedBrowserInput()
	interactInput["actions"] = []any{map[string]any{"type": "click", "selector": "a.next"}}
	interactResult, err := NewBrowserInteractTool().Execute(context.Background(), execCtx, interactInput)
	if err != nil || interactResult.RawOutput["actions_applied"] != 1 {
		t.Fatalf("unexpected browser_interact result=%+v err=%v", interactResult, err)
	}
}

func TestBrowserAttachToolsValidateAttachContract(t *testing.T) {
	if err := NewBrowserAttachCurrentTool().Validate(map[string]any{}); err == nil {
		t.Fatal("expected browser_attach_current validate to fail without attach")
	}
	if err := NewBrowserSnapshotTool().Validate(attachedBrowserInputWithoutTarget()); err != nil {
		t.Fatalf("expected browser_snapshot validate to allow attach without target filters, got %v", err)
	}
	if err := NewBrowserTabsListTool().Validate(attachedBrowserInputWithoutTarget()); err != nil {
		t.Fatalf("expected browser_tabs_list validate to allow attach without target filters, got %v", err)
	}
	if err := NewBrowserNavigateTool().Validate(attachedBrowserInput()); err == nil {
		t.Fatal("expected browser_navigate validate to require url")
	}
	interactInput := attachedBrowserInput()
	interactInput["actions"] = []any{map[string]any{"selector": "a.next"}}
	if err := NewBrowserInteractTool().Validate(interactInput); err == nil {
		t.Fatal("expected browser_interact validate to require action type")
	}
}

func TestRegisterPlaywrightTools(t *testing.T) {
	registry := tools.NewRegistry()
	if err := RegisterPlaywrightTools(registry); err != nil {
		t.Fatalf("RegisterPlaywrightTools returned error: %v", err)
	}
	if _, err := registry.Get("page_read"); err != nil {
		t.Fatalf("expected page_read to be registered, got %v", err)
	}
	if _, err := registry.Get("page_search"); err != nil {
		t.Fatalf("expected page_search to be registered, got %v", err)
	}
	if _, err := registry.Get("page_interact"); err != nil {
		t.Fatalf("expected page_interact to be registered, got %v", err)
	}
	if _, err := registry.Get("structured_dom"); err != nil {
		t.Fatalf("expected structured_dom to be registered, got %v", err)
	}
	for _, name := range []string{"browser_attach_current", "browser_snapshot", "browser_navigate", "browser_tabs_list", "browser_tab_focus", "browser_interact"} {
		if _, err := registry.Get(name); err != nil {
			t.Fatalf("expected %s to be registered, got %v", name, err)
		}
	}
}
