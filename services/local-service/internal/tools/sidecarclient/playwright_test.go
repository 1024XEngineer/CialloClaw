package sidecarclient

import (
	"context"
	"errors"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

type stubPlaywrightClient struct {
	readResult   tools.BrowserPageReadResult
	searchResult tools.BrowserPageSearchResult
	err          error
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
}
