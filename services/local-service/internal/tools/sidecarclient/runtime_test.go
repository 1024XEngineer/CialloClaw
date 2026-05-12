package sidecarclient

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

type stubWorkerInvoker struct {
	response sidecarResponse
	err      error
	delay    time.Duration
	requests []sidecarRequest
}

func (s *stubWorkerInvoker) Invoke(ctx context.Context, request sidecarRequest) (sidecarResponse, error) {
	s.requests = append(s.requests, request)
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return sidecarResponse{}, ctx.Err()
		}
	}
	if s.err != nil {
		return sidecarResponse{}, s.err
	}
	return s.response, nil
}

func writeTempWorkerScript(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "worker.js")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write temp worker script: %v", err)
	}
	return path
}

func TestPlaywrightSidecarRuntimeLifecycle(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{"status": "ok"}}}
	if runtime.Name() != "playwright_sidecar" {
		t.Fatalf("unexpected runtime name: %q", runtime.Name())
	}
	if runtime.Ready() {
		t.Fatal("expected runtime to start as not ready")
	}
	if !runtime.Available() {
		t.Fatal("expected runtime to be available in repo checkout")
	}
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if !runtime.Ready() {
		t.Fatal("expected runtime to be ready after start")
	}
	if !osCapability.HasNamedPipe(runtime.PipeName()) {
		t.Fatal("expected named pipe to be registered after start")
	}
	if err := runtime.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if runtime.Ready() {
		t.Fatal("expected runtime to be not ready after stop")
	}
}

func TestPlaywrightSidecarRuntimeClientReturnsCapabilityErrorWhenNotReady(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	_, err = runtime.Client().ReadPage(t.Context(), "https://example.com")
	if err != tools.ErrPlaywrightSidecarFailed {
		t.Fatalf("expected ErrPlaywrightSidecarFailed, got %v", err)
	}
}

func TestUnavailablePlaywrightSidecarRuntimeDoesNotBlockLifecycle(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime := NewUnavailablePlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if runtime.Available() {
		t.Fatal("expected unavailable runtime to report unavailable")
	}
	if err := runtime.Start(); err != nil {
		t.Fatalf("expected unavailable runtime start to noop, got %v", err)
	}
	if runtime.Ready() {
		t.Fatal("expected unavailable runtime to remain not ready")
	}
	if err := runtime.Stop(); err != nil {
		t.Fatalf("expected unavailable runtime stop to noop, got %v", err)
	}
}
func TestPlaywrightSidecarRuntimeClientExecutesRealReadAndSearch(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	invoker := &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{
		"url":          "https://example.com",
		"title":        "Example Domain",
		"text_content": "example text",
		"mime_type":    "text/html",
		"text_type":    "text/html",
		"source":       "playwright_worker_http",
	}}}
	runtime.invoker = invoker
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	readResult, err := runtime.Client().ReadPage(t.Context(), "https://example.com")
	if err != nil {
		t.Fatalf("ReadPage returned error: %v", err)
	}
	if readResult.Title != "Example Domain" || readResult.Source != "playwright_worker_http" {
		t.Fatalf("unexpected read result: %+v", readResult)
	}
	invoker.response = sidecarResponse{OK: true, Result: map[string]any{
		"url":         "https://example.com",
		"query":       "example",
		"match_count": 1,
		"matches":     []any{"example text"},
		"source":      "playwright_worker_http",
	}}
	searchResult, err := runtime.Client().SearchPage(t.Context(), "https://example.com", "example", 3)
	if err != nil {
		t.Fatalf("SearchPage returned error: %v", err)
	}
	if searchResult.MatchCount != 1 || len(searchResult.Matches) != 1 {
		t.Fatalf("unexpected search result: %+v", searchResult)
	}
	if len(invoker.requests) < 3 || invoker.requests[1].Action != "page_read" || invoker.requests[2].Action != "page_search" {
		t.Fatalf("unexpected request sequence: %+v", invoker.requests)
	}
}

func TestPlaywrightSidecarRuntimeClientInteracts(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	invoker := &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{"status": "ok"}}}
	runtime.invoker = invoker
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	invoker.response = sidecarResponse{OK: true, Result: map[string]any{
		"url":             "https://example.com",
		"title":           "Interactive Page",
		"text_content":    "interaction complete",
		"actions_applied": 2,
		"source":          "playwright_worker_browser",
	}}
	interactResult, err := runtime.Client().InteractPage(t.Context(), "https://example.com", []map[string]any{{"type": "click", "selector": "button"}})
	if err != nil {
		t.Fatalf("InteractPage returned error: %v", err)
	}
	if interactResult.ActionsApplied != 2 {
		t.Fatalf("unexpected interaction result: %+v", interactResult)
	}
}

func TestPlaywrightSidecarRuntimeClientForwardsAttachForPageActions(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	invoker := &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{"status": "ok"}}}
	runtime.invoker = invoker
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	attach := tools.BrowserAttachConfig{Mode: tools.BrowserAttachModeCDP, BrowserKind: "chrome", Target: tools.BrowserAttachTarget{URL: "https://example.com/docs"}}

	invoker.response = sidecarResponse{OK: true, Result: map[string]any{"url": "https://example.com/docs", "title": "Docs", "text_content": "attached text", "source": "playwright_worker_cdp", "attached": true, "browser_kind": "chrome"}}
	if _, err := runtime.client.ReadPageAttached(t.Context(), "https://example.com/docs", attach); err != nil {
		t.Fatalf("ReadPageAttached returned error: %v", err)
	}
	invoker.response = sidecarResponse{OK: true, Result: map[string]any{"url": "https://example.com/docs", "query": "install", "match_count": 1, "matches": []any{"install guide"}, "source": "playwright_worker_cdp", "attached": true, "browser_kind": "chrome"}}
	if _, err := runtime.client.SearchPageAttached(t.Context(), "https://example.com/docs", "install", 2, attach); err != nil {
		t.Fatalf("SearchPageAttached returned error: %v", err)
	}
	invoker.response = sidecarResponse{OK: true, Result: map[string]any{"url": "https://example.com/docs", "title": "Docs", "text_content": "clicked", "actions_applied": 1, "source": "playwright_worker_cdp", "attached": true, "browser_kind": "chrome"}}
	if _, err := runtime.client.InteractPageAttached(t.Context(), "https://example.com/docs", []map[string]any{{"type": "click", "selector": "button"}}, attach); err != nil {
		t.Fatalf("InteractPageAttached returned error: %v", err)
	}
	for _, request := range invoker.requests[1:] {
		if request.Attach == nil || request.Attach.BrowserKind != "chrome" {
			t.Fatalf("expected attached request metadata, got %+v", request)
		}
	}
}

func TestPlaywrightSidecarRuntimeStartFailsHealthCheck(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	pluginService := plugin.NewService()
	runtime, err := NewPlaywrightSidecarRuntime(pluginService, osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{err: errors.New("health failed")}
	if err := runtime.Start(); !errors.Is(err, tools.ErrPlaywrightSidecarFailed) {
		t.Fatalf("expected sidecar failure from health check, got %v", err)
	}
	if runtime.Ready() {
		t.Fatal("expected runtime not ready after failed health check")
	}
	state, ok := pluginService.RuntimeState(plugin.RuntimeKindSidecar, "playwright_sidecar")
	if !ok || state.Health != plugin.RuntimeHealthFailed {
		t.Fatalf("expected plugin runtime cache to reflect health-check failure, got %+v ok=%v", state, ok)
	}
}

func TestPlaywrightSidecarRuntimeStartFailsWithoutOSCapability(t *testing.T) {
	pluginService := plugin.NewService()
	runtime, err := NewPlaywrightSidecarRuntime(pluginService, nil)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	if err := runtime.Start(); err == nil || err.Error() != "os capability adapter is required" {
		t.Fatalf("expected start without os capability to fail, got %v", err)
	}
	state, ok := pluginService.RuntimeState(plugin.RuntimeKindSidecar, "playwright_sidecar")
	if !ok || state.Health != plugin.RuntimeHealthFailed || state.Status != plugin.RuntimeStatusFailed {
		t.Fatalf("expected plugin runtime cache to reflect os capability failure, got %+v ok=%v", state, ok)
	}
}

func TestPlaywrightSidecarRuntimeRequestFailureKeepsReadyState(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{"status": "ok"}}}
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{err: sidecarRequestError{code: "http_404", message: "page not found"}}
	_, err = runtime.Client().ReadPage(t.Context(), "https://example.com")
	if !errors.Is(err, tools.ErrPlaywrightSidecarFailed) {
		t.Fatalf("expected wrapped sidecar failure, got %v", err)
	}
	if !runtime.Ready() {
		t.Fatal("expected runtime to stay ready after request failure")
	}
	if !osCapability.HasNamedPipe(runtime.PipeName()) {
		t.Fatal("expected named pipe to stay registered after request failure")
	}
}

func TestPlaywrightSidecarRuntimeTransportFailureClearsReadyState(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{"status": "ok"}}}
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{err: sidecarTransportError{err: errors.New("worker crashed")}}
	_, err = runtime.Client().ReadPage(t.Context(), "https://example.com")
	if !errors.Is(err, tools.ErrPlaywrightSidecarFailed) {
		t.Fatalf("expected wrapped sidecar failure, got %v", err)
	}
	if runtime.Ready() {
		t.Fatal("expected runtime to leave ready state after transport failure")
	}
	if osCapability.HasNamedPipe(runtime.PipeName()) {
		t.Fatal("expected named pipe to be closed after transport failure")
	}
}

func TestPlaywrightSidecarRuntimeInvokeTimeoutKeepsReadyState(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{"status": "ok"}}}
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{delay: 20 * time.Millisecond}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Millisecond)
	defer cancel()
	_, err = runtime.Client().ReadPage(ctx, "https://example.com")
	if !errors.Is(err, tools.ErrPlaywrightSidecarFailed) {
		t.Fatalf("expected sidecar failure on timeout, got %v", err)
	}
	if !runtime.Ready() {
		t.Fatal("expected runtime to stay ready after request timeout")
	}
}

func TestPlaywrightSidecarRuntimeClientSupportsAttachedBrowserActions(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	invoker := &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{"status": "ok"}}}
	runtime.invoker = invoker
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	pageIndex := 1
	attach := tools.BrowserAttachConfig{
		Mode:        tools.BrowserAttachModeCDP,
		BrowserKind: "chrome",
		EndpointURL: "http://127.0.0.1:9222",
		Target: tools.BrowserAttachTarget{
			URL:           "https://example.com/docs",
			TitleContains: "docs",
			PageIndex:     &pageIndex,
		},
	}

	invoker.response = sidecarResponse{OK: true, Result: map[string]any{
		"attached":          true,
		"browser_kind":      "chrome",
		"browser_transport": "cdp",
		"endpoint_url":      "http://127.0.0.1:9222",
		"page_index":        1,
		"title":             "Docs",
		"url":               "https://example.com/docs",
		"source":            "playwright_worker_cdp",
	}}
	attachResult, err := runtime.Client().AttachCurrentPage(t.Context(), attach)
	if err != nil {
		t.Fatalf("AttachCurrentPage returned error: %v", err)
	}
	if !attachResult.Attached || attachResult.PageIndex != 1 || attachResult.URL != "https://example.com/docs" {
		t.Fatalf("unexpected attach result: %+v", attachResult)
	}

	invoker.response = sidecarResponse{OK: true, Result: map[string]any{
		"attached":          true,
		"browser_kind":      "chrome",
		"browser_transport": "cdp",
		"endpoint_url":      "http://127.0.0.1:9222",
		"page_index":        1,
		"title":             "Docs",
		"url":               "https://example.com/docs",
		"text_content":      "Installation guide",
		"headings":          []any{"Install"},
		"links":             []any{"Guide"},
		"buttons":           []any{"Next"},
		"inputs":            []any{"search"},
		"source":            "playwright_worker_cdp",
	}}
	snapshotResult, err := runtime.Client().SnapshotBrowser(t.Context(), attach)
	if err != nil {
		t.Fatalf("SnapshotBrowser returned error: %v", err)
	}
	if snapshotResult.TextContent != "Installation guide" || len(snapshotResult.Headings) != 1 {
		t.Fatalf("unexpected snapshot result: %+v", snapshotResult)
	}

	invoker.response = sidecarResponse{OK: true, Result: map[string]any{
		"attached":          true,
		"browser_kind":      "chrome",
		"browser_transport": "cdp",
		"endpoint_url":      "http://127.0.0.1:9222",
		"tab_count":         2,
		"tabs": []any{
			map[string]any{"page_index": 0, "title": "Home", "url": "https://example.com"},
			map[string]any{"page_index": 1, "title": "Docs", "url": "https://example.com/docs"},
		},
		"source": "playwright_worker_cdp",
	}}
	tabsResult, err := runtime.Client().ListBrowserTabs(t.Context(), attach)
	if err != nil {
		t.Fatalf("ListBrowserTabs returned error: %v", err)
	}
	if tabsResult.TabCount != 2 || len(tabsResult.Tabs) != 2 {
		t.Fatalf("unexpected tabs result: %+v", tabsResult)
	}

	invoker.response = sidecarResponse{OK: true, Result: map[string]any{
		"attached":          true,
		"browser_kind":      "chrome",
		"browser_transport": "cdp",
		"endpoint_url":      "http://127.0.0.1:9222",
		"page_index":        1,
		"title":             "Docs",
		"url":               "https://example.com/docs/start",
		"text_content":      "Getting started",
		"mime_type":         "text/html",
		"text_type":         "text/html",
		"source":            "playwright_worker_cdp",
	}}
	navigateResult, err := runtime.Client().NavigateBrowser(t.Context(), tools.BrowserNavigateRequest{Attach: attach, URL: "https://example.com/docs/start"})
	if err != nil {
		t.Fatalf("NavigateBrowser returned error: %v", err)
	}
	if navigateResult.URL != "https://example.com/docs/start" || navigateResult.PageIndex != 1 {
		t.Fatalf("unexpected navigate result: %+v", navigateResult)
	}

	invoker.response = sidecarResponse{OK: true, Result: map[string]any{
		"attached":          true,
		"browser_kind":      "chrome",
		"browser_transport": "cdp",
		"endpoint_url":      "http://127.0.0.1:9222",
		"page_index":        1,
		"title":             "Docs",
		"url":               "https://example.com/docs/start",
		"source":            "playwright_worker_cdp",
	}}
	focusResult, err := runtime.Client().FocusBrowserTab(t.Context(), attach)
	if err != nil {
		t.Fatalf("FocusBrowserTab returned error: %v", err)
	}
	if focusResult.PageIndex != 1 {
		t.Fatalf("unexpected focus result: %+v", focusResult)
	}

	invoker.response = sidecarResponse{OK: true, Result: map[string]any{
		"attached":          true,
		"browser_kind":      "chrome",
		"browser_transport": "cdp",
		"endpoint_url":      "http://127.0.0.1:9222",
		"title":             "Docs",
		"url":               "https://example.com/docs/start",
		"text_content":      "Action applied",
		"actions_applied":   1,
		"source":            "playwright_worker_cdp",
	}}
	interactResult, err := runtime.Client().InteractBrowser(t.Context(), tools.BrowserInteractRequest{Attach: attach, Actions: []map[string]any{{"type": "click", "selector": "a.next"}}})
	if err != nil {
		t.Fatalf("InteractBrowser returned error: %v", err)
	}
	if interactResult.ActionsApplied != 1 || !interactResult.Attached {
		t.Fatalf("unexpected browser interact result: %+v", interactResult)
	}

	if len(invoker.requests) != 7 {
		t.Fatalf("expected health plus six attached browser requests, got %+v", invoker.requests)
	}
	if invoker.requests[1].Action != "browser_attach_current" || invoker.requests[1].Attach == nil || invoker.requests[1].Attach.Target.PageIndex == nil || *invoker.requests[1].Attach.Target.PageIndex != 1 {
		t.Fatalf("unexpected attach request: %+v", invoker.requests[1])
	}
	if invoker.requests[4].Action != "browser_navigate" || invoker.requests[4].URL != "https://example.com/docs/start" {
		t.Fatalf("unexpected navigate request: %+v", invoker.requests[4])
	}
	if invoker.requests[6].Action != "browser_interact" || len(invoker.requests[6].Actions) != 1 {
		t.Fatalf("unexpected browser interact request: %+v", invoker.requests[6])
	}
	if invoker.requests[6].Actions[0]["selector"] != "a.next" {
		t.Fatalf("expected cloned browser interact action payload, got %+v", invoker.requests[6].Actions)
	}
}

func TestPlaywrightSidecarRuntimeClientSupportsAttachedPageActions(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	invoker := &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{"status": "ok"}}}
	runtime.invoker = invoker
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	pageIndex := 1
	attach := tools.BrowserAttachConfig{
		Mode:        tools.BrowserAttachModeCDP,
		BrowserKind: "chrome",
		EndpointURL: "http://127.0.0.1:9222",
		Target: tools.BrowserAttachTarget{
			URL:           "https://example.com/docs",
			TitleContains: "docs",
			PageIndex:     &pageIndex,
		},
	}

	invoker.response = sidecarResponse{OK: true, Result: map[string]any{
		"attached":          true,
		"browser_kind":      "chrome",
		"browser_transport": "cdp",
		"endpoint_url":      "http://127.0.0.1:9222",
		"url":               "https://example.com/docs",
		"title":             "Docs",
		"text_content":      "Installation guide",
		"mime_type":         "text/html",
		"text_type":         "text/html",
		"source":            "playwright_worker_cdp",
	}}
	if _, err := runtime.Client().ReadPageAttached(t.Context(), "https://example.com/docs", attach); err != nil {
		t.Fatalf("ReadPageAttached returned error: %v", err)
	}

	invoker.response = sidecarResponse{OK: true, Result: map[string]any{
		"attached":          true,
		"browser_kind":      "chrome",
		"browser_transport": "cdp",
		"endpoint_url":      "http://127.0.0.1:9222",
		"url":               "https://example.com/docs",
		"query":             "install",
		"match_count":       1,
		"matches":           []any{"install guide"},
		"source":            "playwright_worker_cdp",
	}}
	if _, err := runtime.Client().SearchPageAttached(t.Context(), "https://example.com/docs", "install", 2, attach); err != nil {
		t.Fatalf("SearchPageAttached returned error: %v", err)
	}

	invoker.response = sidecarResponse{OK: true, Result: map[string]any{
		"attached":          true,
		"browser_kind":      "chrome",
		"browser_transport": "cdp",
		"endpoint_url":      "http://127.0.0.1:9222",
		"url":               "https://example.com/docs",
		"title":             "Docs",
		"text_content":      "Action applied",
		"actions_applied":   1,
		"source":            "playwright_worker_cdp",
	}}
	if _, err := runtime.Client().InteractPageAttached(t.Context(), "https://example.com/docs", []map[string]any{{"type": "click", "selector": "a.next"}}, attach); err != nil {
		t.Fatalf("InteractPageAttached returned error: %v", err)
	}

	if len(invoker.requests) != 4 {
		t.Fatalf("expected health plus three attached page requests, got %+v", invoker.requests)
	}
	for _, request := range invoker.requests[1:] {
		if request.Attach == nil || request.Attach.EndpointURL != "http://127.0.0.1:9222" {
			t.Fatalf("expected attached page request metadata, got %+v", request)
		}
	}
	if invoker.requests[1].Action != "page_read" || invoker.requests[2].Action != "page_search" || invoker.requests[3].Action != "page_interact" {
		t.Fatalf("unexpected attached page request sequence: %+v", invoker.requests)
	}
}

func TestPlaywrightSidecarRuntimeClientSupportsAttachWithoutBrowserKind(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	invoker := &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{"status": "ok"}}}
	runtime.invoker = invoker
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	attach := tools.BrowserAttachConfig{
		Mode:        tools.BrowserAttachModeCDP,
		EndpointURL: "http://127.0.0.1:9222",
		Target: tools.BrowserAttachTarget{
			URL: "https://example.com/docs",
		},
	}
	invoker.response = sidecarResponse{OK: true, Result: map[string]any{
		"attached":          true,
		"browser_transport": "cdp",
		"url":               "https://example.com/docs",
		"title":             "Docs",
		"text_content":      "Installation guide",
		"mime_type":         "text/html",
		"text_type":         "text/html",
		"source":            "playwright_worker_cdp",
	}}
	if _, err := runtime.Client().ReadPageAttached(t.Context(), "https://example.com/docs", attach); err != nil {
		t.Fatalf("ReadPageAttached returned error: %v", err)
	}
	if got := invoker.requests[1].Attach.BrowserKind; got != "" {
		t.Fatalf("expected empty browser kind to pass through unchanged, got %q", got)
	}
}
func TestResolveRelativePathFromRootsFindsWorkerEntry(t *testing.T) {
	root := t.TempDir()
	entryPath := filepath.Join(root, "workers", "playwright-worker", "src", "index.js")
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
		t.Fatalf("mkdir worker path: %v", err)
	}
	if err := os.WriteFile(entryPath, []byte("console.log('ok')\n"), 0o644); err != nil {
		t.Fatalf("write worker entry: %v", err)
	}
	resolved, err := resolveRelativePathFromRoots(playwrightWorkerRelativePath, []string{filepath.Join(root, "services", "local-service")})
	if err != nil {
		t.Fatalf("resolveRelativePathFromRoots returned error: %v", err)
	}
	if resolved != entryPath {
		t.Fatalf("expected resolved entry %q, got %q", entryPath, resolved)
	}
	if _, err := resolveRelativePathFromRoots(playwrightWorkerRelativePath, []string{filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("expected missing worker entry lookup to fail")
	}
}

func TestCommandWorkerInvokerInvokeReturnsStructuredRequestError(t *testing.T) {
	entryPath := writeTempWorkerScript(t, `process.stdin.resume(); process.stdin.on("end", () => { process.stdout.write(JSON.stringify({ ok: false, error: { code: "http_404", message: "page not found" } })); process.exitCode = 1; });`)
	invoker := newCommandWorkerInvoker(entryPath)
	response, err := invoker.Invoke(context.Background(), sidecarRequest{Action: "page_read", URL: "https://example.com"})
	if response.OK {
		t.Fatalf("expected request error response, got %+v", response)
	}
	var requestErr sidecarRequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("expected request error, got %v", err)
	}
	if requestErr.code != "http_404" {
		t.Fatalf("expected request error code http_404, got %+v", requestErr)
	}
	if shouldMarkRuntimeFailure(err) {
		t.Fatal("expected request error not to mark runtime failure")
	}
}

func TestCommandWorkerInvokerInvokeReturnsTransportErrorForInvalidOutput(t *testing.T) {
	entryPath := writeTempWorkerScript(t, `process.stderr.write("crashed\n"); process.stdout.write("not-json"); process.exitCode = 1;`)
	invoker := newCommandWorkerInvoker(entryPath)
	_, err := invoker.Invoke(context.Background(), sidecarRequest{Action: "health"})
	var transportErr sidecarTransportError
	if !errors.As(err, &transportErr) {
		t.Fatalf("expected transport error, got %v", err)
	}
	if !strings.Contains(err.Error(), "crashed") {
		t.Fatalf("expected stderr in transport error, got %v", err)
	}
	if !shouldMarkRuntimeFailure(err) {
		t.Fatal("expected transport error to mark runtime failure")
	}
}

func TestCommandWorkerInvokerInvokeTimeoutReturnsRequestError(t *testing.T) {
	entryPath := writeTempWorkerScript(t, `setTimeout(() => { process.stdout.write(JSON.stringify({ ok: true, result: { status: "late" } })); }, 200);`)
	invoker := newCommandWorkerInvoker(entryPath)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := invoker.Invoke(ctx, sidecarRequest{Action: "health"})
	var requestErr sidecarRequestError
	if !errors.As(err, &requestErr) {
		t.Fatalf("expected timeout request error, got %v", err)
	}
	if requestErr.code != "timeout" {
		t.Fatalf("expected timeout request error code, got %+v", requestErr)
	}
}
