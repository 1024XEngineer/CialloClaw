package sidecarclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	runtimesrc "runtime"
	"strings"
	"sync"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

const sidecarHealthTimeout = 5 * time.Second
const sidecarDefaultTimeout = 20 * time.Second
const playwrightWorkerRelativePath = "workers/playwright-worker/src/index.js"

type workerInvoker interface {
	Invoke(ctx context.Context, request sidecarRequest) (sidecarResponse, error)
}

type sidecarRequest struct {
	Action       string                     `json:"action"`
	URL          string                     `json:"url,omitempty"`
	Query        string                     `json:"query,omitempty"`
	Attach       *tools.BrowserAttachConfig `json:"attach,omitempty"`
	Path         string                     `json:"path,omitempty"`
	Language     string                     `json:"language,omitempty"`
	OutputPath   string                     `json:"output_path,omitempty"`
	OutputDir    string                     `json:"output_dir,omitempty"`
	Format       string                     `json:"format,omitempty"`
	Limit        int                        `json:"limit,omitempty"`
	EverySeconds float64                    `json:"every_seconds,omitempty"`
	Actions      []map[string]any           `json:"actions,omitempty"`
}

type sidecarResponse struct {
	OK     bool              `json:"ok"`
	Result map[string]any    `json:"result,omitempty"`
	Error  *sidecarErrorBody `json:"error,omitempty"`
}

type sidecarErrorBody struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type commandWorkerInvoker struct {
	entryPath string
	command   string
	args      []string
}

type sidecarTransportError struct {
	err error
}

func (e sidecarTransportError) Error() string {
	if e.err == nil {
		return "sidecar transport failed"
	}
	return e.err.Error()
}

func (e sidecarTransportError) Unwrap() error {
	return e.err
}

type sidecarRequestError struct {
	code    string
	message string
}

func (e sidecarRequestError) Error() string {
	if strings.TrimSpace(e.message) != "" {
		return strings.TrimSpace(e.message)
	}
	if strings.TrimSpace(e.code) != "" {
		return strings.TrimSpace(e.code)
	}
	return "sidecar request failed"
}

func newCommandWorkerInvoker(entryPath string) commandWorkerInvoker {
	return commandWorkerInvoker{
		entryPath: entryPath,
		command:   "node",
		args:      []string{entryPath},
	}
}

func (i commandWorkerInvoker) Invoke(ctx context.Context, request sidecarRequest) (sidecarResponse, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return sidecarResponse{}, err
	}
	if strings.TrimSpace(i.entryPath) == "" {
		return sidecarResponse{}, sidecarTransportError{err: errors.New("worker entry path is required")}
	}
	cmd := exec.CommandContext(ctx, i.command, i.args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = strings.NewReader(string(payload))
	err = cmd.Run()
	response, decodeErr := decodeSidecarResponse(stdout.Bytes())
	if decodeErr == nil {
		if !response.OK {
			return response, sidecarRequestError{code: stringValue(responseErrorMap(response.Error), "code"), message: firstNonEmptyString(stringValue(responseErrorMap(response.Error), "message"), strings.TrimSpace(stderr.String()))}
		}
		if err == nil {
			return response, nil
		}
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return sidecarResponse{}, sidecarRequestError{code: "timeout", message: context.DeadlineExceeded.Error()}
	}
	if err != nil {
		return sidecarResponse{}, sidecarTransportError{err: commandWorkerError(err, stderr.String())}
	}
	if decodeErr != nil {
		return sidecarResponse{}, sidecarTransportError{err: fmt.Errorf("decode worker response: %w", decodeErr)}
	}
	return response, nil
}

type runtimePlaywrightClient struct {
	runtime *PlaywrightSidecarRuntime
}

func (c runtimePlaywrightClient) invokeBrowserRequest(ctx context.Context, request sidecarRequest) (sidecarResponse, error) {
	if c.runtime == nil || !c.runtime.Available() {
		return sidecarResponse{}, tools.ErrPlaywrightSidecarFailed
	}
	if !c.runtime.Ready() {
		return sidecarResponse{}, tools.ErrPlaywrightSidecarFailed
	}
	response, err := c.runtime.invoke(ctx, request)
	if err != nil {
		if shouldMarkRuntimeFailure(err) {
			_ = c.runtime.markFailure()
		}
		return sidecarResponse{}, fmt.Errorf("%w: %v", tools.ErrPlaywrightSidecarFailed, err)
	}
	return response, nil
}

func (c runtimePlaywrightClient) ReadPage(ctx context.Context, url string) (tools.BrowserPageReadResult, error) {
	response, err := c.invokeBrowserRequest(ctx, sidecarRequest{Action: "page_read", URL: url})
	if err != nil {
		return tools.BrowserPageReadResult{}, err
	}
	return tools.BrowserPageReadResult{
		BrowserExecutionMetadata: browserExecutionMetadata(response.Result),
		URL:                      stringValue(response.Result, "url"),
		Title:                    stringValue(response.Result, "title"),
		TextContent:              stringValue(response.Result, "text_content"),
		MIMEType:                 stringValue(response.Result, "mime_type"),
		TextType:                 stringValue(response.Result, "text_type"),
		Source:                   firstNonEmptyString(stringValue(response.Result, "source"), "playwright_sidecar"),
	}, nil
}

func (c runtimePlaywrightClient) SearchPage(ctx context.Context, url, query string, limit int) (tools.BrowserPageSearchResult, error) {
	response, err := c.invokeBrowserRequest(ctx, sidecarRequest{Action: "page_search", URL: url, Query: query, Limit: limit})
	if err != nil {
		return tools.BrowserPageSearchResult{}, err
	}
	return tools.BrowserPageSearchResult{
		BrowserExecutionMetadata: browserExecutionMetadata(response.Result),
		URL:                      stringValue(response.Result, "url"),
		Query:                    stringValue(response.Result, "query"),
		MatchCount:               intValue(response.Result, "match_count"),
		Matches:                  stringSliceValue(response.Result, "matches"),
		Source:                   firstNonEmptyString(stringValue(response.Result, "source"), "playwright_sidecar"),
	}, nil
}

func (c runtimePlaywrightClient) InteractPage(ctx context.Context, url string, actions []map[string]any) (tools.BrowserPageInteractResult, error) {
	response, err := c.invokeBrowserRequest(ctx, sidecarRequest{Action: "page_interact", URL: url, Actions: cloneActionSlice(actions)})
	if err != nil {
		return tools.BrowserPageInteractResult{}, err
	}
	return tools.BrowserPageInteractResult{
		BrowserExecutionMetadata: browserExecutionMetadata(response.Result),
		URL:                      stringValue(response.Result, "url"),
		Title:                    stringValue(response.Result, "title"),
		TextContent:              stringValue(response.Result, "text_content"),
		ActionsApplied:           intValue(response.Result, "actions_applied"),
		Source:                   firstNonEmptyString(stringValue(response.Result, "source"), "playwright_sidecar"),
	}, nil
}

func (c runtimePlaywrightClient) StructuredDOM(ctx context.Context, url string) (tools.BrowserStructuredDOMResult, error) {
	response, err := c.invokeBrowserRequest(ctx, sidecarRequest{Action: "structured_dom", URL: url})
	if err != nil {
		return tools.BrowserStructuredDOMResult{}, err
	}
	return tools.BrowserStructuredDOMResult{
		BrowserExecutionMetadata: browserExecutionMetadata(response.Result),
		URL:                      stringValue(response.Result, "url"),
		Title:                    stringValue(response.Result, "title"),
		Headings:                 stringSliceValue(response.Result, "headings"),
		Links:                    stringSliceValue(response.Result, "links"),
		Buttons:                  stringSliceValue(response.Result, "buttons"),
		Inputs:                   stringSliceValue(response.Result, "inputs"),
		Source:                   firstNonEmptyString(stringValue(response.Result, "source"), "playwright_sidecar"),
	}, nil
}

func (c runtimePlaywrightClient) AttachCurrentPage(ctx context.Context, attach tools.BrowserAttachConfig) (tools.BrowserAttachedPageResult, error) {
	response, err := c.invokeBrowserRequest(ctx, sidecarRequest{Action: "browser_attach_current", Attach: cloneAttachConfigPtr(&attach)})
	if err != nil {
		return tools.BrowserAttachedPageResult{}, err
	}
	return tools.BrowserAttachedPageResult{
		BrowserExecutionMetadata: browserExecutionMetadata(response.Result),
		PageIndex:                intValue(response.Result, "page_index"),
		Title:                    stringValue(response.Result, "title"),
		URL:                      stringValue(response.Result, "url"),
		Source:                   firstNonEmptyString(stringValue(response.Result, "source"), "playwright_sidecar"),
	}, nil
}

func (c runtimePlaywrightClient) SnapshotBrowser(ctx context.Context, attach tools.BrowserAttachConfig) (tools.BrowserSnapshotResult, error) {
	response, err := c.invokeBrowserRequest(ctx, sidecarRequest{Action: "browser_snapshot", Attach: cloneAttachConfigPtr(&attach)})
	if err != nil {
		return tools.BrowserSnapshotResult{}, err
	}
	return tools.BrowserSnapshotResult{
		BrowserAttachedPageResult: tools.BrowserAttachedPageResult{
			BrowserExecutionMetadata: browserExecutionMetadata(response.Result),
			PageIndex:                intValue(response.Result, "page_index"),
			Title:                    stringValue(response.Result, "title"),
			URL:                      stringValue(response.Result, "url"),
			Source:                   firstNonEmptyString(stringValue(response.Result, "source"), "playwright_sidecar"),
		},
		TextContent: stringValue(response.Result, "text_content"),
		Headings:    stringSliceValue(response.Result, "headings"),
		Links:       stringSliceValue(response.Result, "links"),
		Buttons:     stringSliceValue(response.Result, "buttons"),
		Inputs:      stringSliceValue(response.Result, "inputs"),
	}, nil
}

func (c runtimePlaywrightClient) NavigateBrowser(ctx context.Context, request tools.BrowserNavigateRequest) (tools.BrowserNavigationResult, error) {
	response, err := c.invokeBrowserRequest(ctx, sidecarRequest{Action: "browser_navigate", URL: strings.TrimSpace(request.URL), Attach: cloneAttachConfigPtr(&request.Attach)})
	if err != nil {
		return tools.BrowserNavigationResult{}, err
	}
	return tools.BrowserNavigationResult{
		BrowserAttachedPageResult: tools.BrowserAttachedPageResult{
			BrowserExecutionMetadata: browserExecutionMetadata(response.Result),
			PageIndex:                intValue(response.Result, "page_index"),
			Title:                    stringValue(response.Result, "title"),
			URL:                      stringValue(response.Result, "url"),
			Source:                   firstNonEmptyString(stringValue(response.Result, "source"), "playwright_sidecar"),
		},
		TextContent: stringValue(response.Result, "text_content"),
		MIMEType:    stringValue(response.Result, "mime_type"),
		TextType:    stringValue(response.Result, "text_type"),
	}, nil
}

func (c runtimePlaywrightClient) ListBrowserTabs(ctx context.Context, attach tools.BrowserAttachConfig) (tools.BrowserTabsListResult, error) {
	response, err := c.invokeBrowserRequest(ctx, sidecarRequest{Action: "browser_tabs_list", Attach: cloneAttachConfigPtr(&attach)})
	if err != nil {
		return tools.BrowserTabsListResult{}, err
	}
	return tools.BrowserTabsListResult{
		BrowserExecutionMetadata: browserExecutionMetadata(response.Result),
		TabCount:                 intValue(response.Result, "tab_count"),
		Tabs:                     browserTabSliceValue(response.Result, "tabs"),
		Source:                   firstNonEmptyString(stringValue(response.Result, "source"), "playwright_sidecar"),
	}, nil
}

func (c runtimePlaywrightClient) FocusBrowserTab(ctx context.Context, attach tools.BrowserAttachConfig) (tools.BrowserAttachedPageResult, error) {
	response, err := c.invokeBrowserRequest(ctx, sidecarRequest{Action: "browser_tab_focus", Attach: cloneAttachConfigPtr(&attach)})
	if err != nil {
		return tools.BrowserAttachedPageResult{}, err
	}
	return tools.BrowserAttachedPageResult{
		BrowserExecutionMetadata: browserExecutionMetadata(response.Result),
		PageIndex:                intValue(response.Result, "page_index"),
		Title:                    stringValue(response.Result, "title"),
		URL:                      stringValue(response.Result, "url"),
		Source:                   firstNonEmptyString(stringValue(response.Result, "source"), "playwright_sidecar"),
	}, nil
}

func (c runtimePlaywrightClient) InteractBrowser(ctx context.Context, request tools.BrowserInteractRequest) (tools.BrowserPageInteractResult, error) {
	response, err := c.invokeBrowserRequest(ctx, sidecarRequest{Action: "browser_interact", Attach: cloneAttachConfigPtr(&request.Attach), Actions: cloneActionSlice(request.Actions)})
	if err != nil {
		return tools.BrowserPageInteractResult{}, err
	}
	return tools.BrowserPageInteractResult{
		BrowserExecutionMetadata: browserExecutionMetadata(response.Result),
		URL:                      stringValue(response.Result, "url"),
		Title:                    stringValue(response.Result, "title"),
		TextContent:              stringValue(response.Result, "text_content"),
		ActionsApplied:           intValue(response.Result, "actions_applied"),
		Source:                   firstNonEmptyString(stringValue(response.Result, "source"), "playwright_sidecar"),
	}, nil
}

// PlaywrightSidecarRuntime 是当前阶段的最小运行时骨架。
//
// 它负责表达：
// - 当前 sidecar 名称
// - 当前 plugin 声明中是否存在该 sidecar
// - 当前最小 transport 规格
// - 当前 sidecar 是否已进入 ready 状态
type PlaywrightSidecarRuntime struct {
	mu        sync.Mutex
	plugins   *plugin.Service
	spec      plugin.SidecarSpec
	os        platform.OSCapabilityAdapter
	ready     bool
	available bool
	invoker   workerInvoker
	client    runtimePlaywrightClient
}

// NewPlaywrightSidecarRuntime 创建并返回最小运行时骨架。
func NewPlaywrightSidecarRuntime(pluginService *plugin.Service, osCapability platform.OSCapabilityAdapter) (*PlaywrightSidecarRuntime, error) {
	spec, ok := pluginService.SidecarSpec("playwright_sidecar")
	if !ok {
		return nil, errors.New("playwright sidecar not declared")
	}
	markPluginRuntimeStarting(pluginService, plugin.RuntimeKindSidecar, spec.Name)
	runtime := &PlaywrightSidecarRuntime{
		plugins:   pluginService,
		spec:      spec,
		os:        osCapability,
		ready:     false,
		available: false,
	}
	runtime.client = runtimePlaywrightClient{runtime: runtime}
	entryPath, err := resolveWorkerEntryPath()
	if err != nil {
		markPluginRuntimeFailed(pluginService, plugin.RuntimeKindSidecar, spec.Name, err)
		return runtime, err
	}
	runtime.available = true
	runtime.invoker = newCommandWorkerInvoker(entryPath)
	return runtime, nil
}

// NewUnavailablePlaywrightSidecarRuntime returns a disabled runtime placeholder.
func NewUnavailablePlaywrightSidecarRuntime(pluginService *plugin.Service, osCapability platform.OSCapabilityAdapter) *PlaywrightSidecarRuntime {
	spec, _ := pluginService.SidecarSpec("playwright_sidecar")
	markPluginRuntimeUnavailable(pluginService, plugin.RuntimeKindSidecar, spec.Name, "playwright sidecar unavailable")
	runtime := &PlaywrightSidecarRuntime{
		plugins:   pluginService,
		spec:      spec,
		os:        osCapability,
		ready:     false,
		available: false,
	}
	runtime.client = runtimePlaywrightClient{runtime: runtime}
	return runtime
}

// Name 返回当前 sidecar 名称。
func (r *PlaywrightSidecarRuntime) Name() string {
	return r.spec.Name
}

// PipeName 返回当前最小传输骨架使用的命名管道名。
func (r *PlaywrightSidecarRuntime) PipeName() string {
	return sidecarPipeName(r.spec.Name)
}

// Ready 返回当前 sidecar 是否已进入 ready 状态。
func (r *PlaywrightSidecarRuntime) Ready() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ready
}

// Available reports whether the runtime can attempt to start.
func (r *PlaywrightSidecarRuntime) Available() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.available
}

// Start 进入当前阶段的最小 ready 状态。
func (r *PlaywrightSidecarRuntime) Start() error {
	if !r.Available() {
		markPluginRuntimeUnavailable(r.plugins, plugin.RuntimeKindSidecar, r.spec.Name, "playwright sidecar unavailable")
		return nil
	}
	if r.os == nil {
		markPluginRuntimeFailed(r.plugins, plugin.RuntimeKindSidecar, r.spec.Name, errors.New("os capability adapter is required"))
		return errors.New("os capability adapter is required")
	}
	if err := r.os.EnsureNamedPipe(sidecarPipeName(r.spec.Name)); err != nil {
		markPluginRuntimeFailed(r.plugins, plugin.RuntimeKindSidecar, r.spec.Name, err)
		return err
	}
	healthCtx, cancel := context.WithTimeout(context.Background(), sidecarHealthTimeout)
	defer cancel()
	if _, err := r.invoke(healthCtx, sidecarRequest{Action: "health"}); err != nil {
		_ = r.os.CloseNamedPipe(sidecarPipeName(r.spec.Name))
		markPluginRuntimeFailed(r.plugins, plugin.RuntimeKindSidecar, r.spec.Name, err)
		return fmt.Errorf("%w: %v", tools.ErrPlaywrightSidecarFailed, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ready = true
	markPluginRuntimeHealthy(r.plugins, plugin.RuntimeKindSidecar, r.spec.Name)
	return nil
}

// Stop 退出 ready 状态并关闭最小传输骨架。
func (r *PlaywrightSidecarRuntime) Stop() error {
	if !r.Available() {
		return nil
	}
	if r.os == nil {
		return nil
	}
	if err := r.os.CloseNamedPipe(sidecarPipeName(r.spec.Name)); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ready = false
	markPluginRuntimeStopped(r.plugins, plugin.RuntimeKindSidecar, r.spec.Name)
	return nil
}

// Client 返回当前运行时关联的最小 client。
func (r *PlaywrightSidecarRuntime) Client() tools.PlaywrightSidecarClient {
	return r.client
}

func (r *PlaywrightSidecarRuntime) invoke(ctx context.Context, request sidecarRequest) (sidecarResponse, error) {
	if r == nil || r.invoker == nil {
		return sidecarResponse{}, errors.New("playwright sidecar invoker is not available")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		boundedCtx, cancel := context.WithTimeout(ctx, sidecarDefaultTimeout)
		defer cancel()
		ctx = boundedCtx
	}
	return r.invoker.Invoke(ctx, request)
}

func (r *PlaywrightSidecarRuntime) markFailure() error {
	r.mu.Lock()
	r.ready = false
	r.mu.Unlock()
	markPluginRuntimeFailed(r.plugins, plugin.RuntimeKindSidecar, r.spec.Name, errors.New("playwright sidecar marked failed"))
	if r.os == nil {
		return nil
	}
	return r.os.CloseNamedPipe(sidecarPipeName(r.spec.Name))
}

func stringValue(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	value, _ := values[key].(string)
	return strings.TrimSpace(value)
}

func intValue(values map[string]any, key string) int {
	if len(values) == 0 {
		return 0
	}
	switch typed := values[key].(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func browserExecutionMetadata(values map[string]any) tools.BrowserExecutionMetadata {
	if len(values) == 0 {
		return tools.BrowserExecutionMetadata{}
	}
	return tools.BrowserExecutionMetadata{
		Attached:         boolValue(values, "attached"),
		BrowserKind:      stringValue(values, "browser_kind"),
		BrowserTransport: stringValue(values, "browser_transport"),
		EndpointURL:      stringValue(values, "endpoint_url"),
	}
}

func boolValue(values map[string]any, key string) bool {
	if len(values) == 0 {
		return false
	}
	value, _ := values[key].(bool)
	return value
}

func stringSliceValue(values map[string]any, key string) []string {
	if len(values) == 0 {
		return nil
	}
	raw, ok := values[key].([]any)
	if ok {
		items := make([]string, 0, len(raw))
		for _, item := range raw {
			if value, ok := item.(string); ok && strings.TrimSpace(value) != "" {
				items = append(items, strings.TrimSpace(value))
			}
		}
		return items
	}
	if typed, ok := values[key].([]string); ok {
		return append([]string(nil), typed...)
	}
	return nil
}

func browserTabSliceValue(values map[string]any, key string) []tools.BrowserTabInfo {
	if len(values) == 0 {
		return nil
	}
	rawItems, ok := values[key].([]any)
	if !ok {
		return nil
	}
	items := make([]tools.BrowserTabInfo, 0, len(rawItems))
	for _, rawItem := range rawItems {
		typed, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		items = append(items, tools.BrowserTabInfo{
			PageIndex: intValue(typed, "page_index"),
			Title:     stringValue(typed, "title"),
			URL:       stringValue(typed, "url"),
		})
	}
	return items
}

func cloneAttachConfigPtr(input *tools.BrowserAttachConfig) *tools.BrowserAttachConfig {
	if input == nil {
		return nil
	}
	cloned := *input
	if input.Target.PageIndex != nil {
		pageIndex := *input.Target.PageIndex
		cloned.Target.PageIndex = &pageIndex
	}
	return &cloned
}

func resolveWorkerEntryPath() (string, error) {
	return resolveRelativePathFromRoots(playwrightWorkerRelativePath, workerSearchRoots())
}

func workerSearchRoots() []string {
	roots := make([]string, 0, 3)
	if exePath, err := os.Executable(); err == nil && strings.TrimSpace(exePath) != "" {
		roots = append(roots, filepath.Dir(exePath))
	}
	if _, file, _, ok := runtimesrc.Caller(0); ok && strings.TrimSpace(file) != "" {
		roots = append(roots, filepath.Dir(file))
	}
	if workingDir, err := os.Getwd(); err == nil && strings.TrimSpace(workingDir) != "" {
		roots = append(roots, workingDir)
	}
	return roots
}

func resolveRelativePathFromRoots(relativePath string, roots []string) (string, error) {
	seen := make(map[string]struct{})
	for _, root := range roots {
		candidate, ok := searchUpwardsForRelativePath(root, relativePath, seen)
		if ok {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("playwright worker entry not found: %s", relativePath)
}

func searchUpwardsForRelativePath(start, relativePath string, seen map[string]struct{}) (string, bool) {
	if strings.TrimSpace(start) == "" {
		return "", false
	}
	current := filepath.Clean(start)
	for {
		if _, ok := seen[current]; ok {
			return "", false
		}
		seen[current] = struct{}{}
		candidate := filepath.Join(current, relativePath)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

func markPluginRuntimeHealthy(service *plugin.Service, kind plugin.RuntimeKind, name string) {
	if service == nil {
		return
	}
	service.MarkRuntimeHealthy(kind, name)
}

func markPluginRuntimeStarting(service *plugin.Service, kind plugin.RuntimeKind, name string) {
	if service == nil {
		return
	}
	service.MarkRuntimeStarting(kind, name)
}

func markPluginRuntimeFailed(service *plugin.Service, kind plugin.RuntimeKind, name string, err error) {
	if service == nil {
		return
	}
	service.MarkRuntimeFailed(kind, name, err)
}

func markPluginRuntimeUnavailable(service *plugin.Service, kind plugin.RuntimeKind, name, reason string) {
	if service == nil {
		return
	}
	service.MarkRuntimeUnavailable(kind, name, reason)
}

func markPluginRuntimeStopped(service *plugin.Service, kind plugin.RuntimeKind, name string) {
	if service == nil {
		return
	}
	service.MarkRuntimeStopped(kind, name)
}

func decodeSidecarResponse(payload []byte) (sidecarResponse, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return sidecarResponse{}, errors.New("empty worker response")
	}
	var response sidecarResponse
	if err := json.Unmarshal(trimmed, &response); err != nil {
		return sidecarResponse{}, err
	}
	return response, nil
}

func responseErrorMap(errBody *sidecarErrorBody) map[string]any {
	if errBody == nil {
		return nil
	}
	return map[string]any{
		"code":    errBody.Code,
		"message": errBody.Message,
	}
}

func commandWorkerError(err error, stderr string) error {
	trimmed := strings.TrimSpace(stderr)
	if trimmed != "" {
		return fmt.Errorf("worker command failed: %s", trimmed)
	}
	return err
}

func shouldMarkRuntimeFailure(err error) bool {
	var transportErr sidecarTransportError
	return errors.As(err, &transportErr)
}

func sidecarPipeName(name string) string {
	return fmt.Sprintf("cialloclaw-%s", name)
}
