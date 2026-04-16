package sidecarclient

import (
	"context"
	"errors"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

type stubPlaywrightClient struct {
	readResult       tools.BrowserPageReadResult
	searchResult     tools.BrowserPageSearchResult
	interactResult   tools.BrowserPageInteractResult
	structuredResult tools.BrowserStructuredDOMResult
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
}
