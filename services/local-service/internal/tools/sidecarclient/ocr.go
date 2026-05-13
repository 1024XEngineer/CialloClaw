package sidecarclient

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

const ocrWorkerRelativePath = "workers/ocr-worker/src/index.js"

type noopOCRWorkerClient struct{}

func NewNoopOCRWorkerClient() tools.OCRWorkerClient {
	return noopOCRWorkerClient{}
}

func (noopOCRWorkerClient) ExtractText(_ context.Context, _ string) (tools.OCRTextResult, error) {
	return tools.OCRTextResult{}, tools.ErrOCRWorkerFailed
}

func (noopOCRWorkerClient) OCRImage(_ context.Context, _, _ string) (tools.OCRTextResult, error) {
	return tools.OCRTextResult{}, tools.ErrOCRWorkerFailed
}

func (noopOCRWorkerClient) OCRPDF(_ context.Context, _, _ string) (tools.OCRTextResult, error) {
	return tools.OCRTextResult{}, tools.ErrOCRWorkerFailed
}

type runtimeOCRWorkerClient struct {
	runtime *OCRWorkerRuntime
}

func (c runtimeOCRWorkerClient) ExtractText(ctx context.Context, path string) (tools.OCRTextResult, error) {
	return c.runtime.invokeTextAction(ctx, sidecarRequest{Action: "extract_text", Path: path})
}

func (c runtimeOCRWorkerClient) OCRImage(ctx context.Context, path, language string) (tools.OCRTextResult, error) {
	return c.runtime.invokeTextAction(ctx, sidecarRequest{Action: "ocr_image", Path: path, Language: language})
}

func (c runtimeOCRWorkerClient) OCRPDF(ctx context.Context, path, language string) (tools.OCRTextResult, error) {
	return c.runtime.invokeTextAction(ctx, sidecarRequest{Action: "ocr_pdf", Path: path, Language: language})
}

// OCRWorkerRuntime manages the OCR worker lifecycle.
type OCRWorkerRuntime struct {
	mu        sync.Mutex
	plugins   *plugin.Service
	os        platform.OSCapabilityAdapter
	ready     bool
	available bool
	invoker   workerInvoker
	client    runtimeOCRWorkerClient
	name      string
}

func NewOCRWorkerRuntime(pluginService *plugin.Service, osCapability platform.OSCapabilityAdapter) (*OCRWorkerRuntime, error) {
	runtime := &OCRWorkerRuntime{
		plugins:   pluginService,
		os:        osCapability,
		ready:     false,
		available: false,
		name:      "ocr_worker",
	}
	runtime.client = runtimeOCRWorkerClient{runtime: runtime}
	entryPath, err := resolveRelativePathFromRoots(ocrWorkerRelativePath, workerSearchRoots())
	if err != nil {
		markPluginRuntimeFailed(pluginService, plugin.RuntimeKindWorker, "ocr_worker", err)
		return runtime, err
	}
	markPluginRuntimeStarting(pluginService, plugin.RuntimeKindWorker, "ocr_worker")
	runtime.available = true
	runtime.invoker = newCommandWorkerInvoker(entryPath)
	return runtime, nil
}

func NewUnavailableOCRWorkerRuntime(pluginService *plugin.Service, osCapability platform.OSCapabilityAdapter) *OCRWorkerRuntime {
	markPluginRuntimeUnavailable(pluginService, plugin.RuntimeKindWorker, "ocr_worker", "ocr worker unavailable")
	runtime := &OCRWorkerRuntime{
		plugins:   pluginService,
		os:        osCapability,
		ready:     false,
		available: false,
		name:      "ocr_worker",
	}
	runtime.client = runtimeOCRWorkerClient{runtime: runtime}
	return runtime
}

func (r *OCRWorkerRuntime) Name() string {
	return r.name
}

func (r *OCRWorkerRuntime) PipeName() string {
	return sidecarPipeName(r.name)
}

func (r *OCRWorkerRuntime) Ready() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ready
}

func (r *OCRWorkerRuntime) Available() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.available
}

func (r *OCRWorkerRuntime) Start() error {
	if !r.Available() {
		return nil
	}
	if r.os == nil {
		return errors.New("os capability adapter is required")
	}
	if err := r.os.EnsureNamedPipe(r.PipeName()); err != nil {
		markPluginRuntimeFailed(r.plugins, plugin.RuntimeKindWorker, r.name, err)
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), sidecarHealthTimeout)
	defer cancel()
	if _, err := r.invoke(ctx, sidecarRequest{Action: "health"}); err != nil {
		_ = r.os.CloseNamedPipe(r.PipeName())
		markPluginRuntimeFailed(r.plugins, plugin.RuntimeKindWorker, r.name, err)
		return fmt.Errorf("%w: %v", tools.ErrOCRWorkerFailed, err)
	}
	r.mu.Lock()
	r.ready = true
	r.mu.Unlock()
	markPluginRuntimeHealthy(r.plugins, plugin.RuntimeKindWorker, r.name)
	return nil
}

func (r *OCRWorkerRuntime) Stop() error {
	if !r.Available() {
		return nil
	}
	if r.os == nil {
		return nil
	}
	if err := r.os.CloseNamedPipe(r.PipeName()); err != nil {
		return err
	}
	r.mu.Lock()
	r.ready = false
	r.mu.Unlock()
	markPluginRuntimeStopped(r.plugins, plugin.RuntimeKindWorker, r.name)
	return nil
}

func (r *OCRWorkerRuntime) Client() tools.OCRWorkerClient {
	return r.client
}

func (r *OCRWorkerRuntime) invoke(ctx context.Context, request sidecarRequest) (sidecarResponse, error) {
	if r == nil || r.invoker == nil {
		return sidecarResponse{}, errors.New("ocr worker invoker is not available")
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

func (r *OCRWorkerRuntime) markFailure() error {
	r.mu.Lock()
	r.ready = false
	r.mu.Unlock()
	markPluginRuntimeFailed(r.plugins, plugin.RuntimeKindWorker, r.name, errors.New("ocr worker marked failed"))
	if r.os == nil {
		return nil
	}
	return r.os.CloseNamedPipe(r.PipeName())
}

func (r *OCRWorkerRuntime) invokeTextAction(ctx context.Context, request sidecarRequest) (tools.OCRTextResult, error) {
	if r == nil || !r.Available() {
		return tools.OCRTextResult{}, tools.ErrOCRWorkerFailed
	}
	if !r.Ready() {
		return tools.OCRTextResult{}, tools.ErrOCRWorkerFailed
	}
	response, err := r.invoke(ctx, request)
	if err != nil {
		if shouldMarkRuntimeFailure(err) {
			_ = r.markFailure()
		}
		return tools.OCRTextResult{}, fmt.Errorf("%w: %v", tools.ErrOCRWorkerFailed, err)
	}
	return tools.OCRTextResult{
		Path:      stringValue(response.Result, "path"),
		Text:      stringValue(response.Result, "text"),
		Language:  stringValue(response.Result, "language"),
		Source:    firstNonEmptyString(stringValue(response.Result, "source"), "ocr_worker"),
		PageCount: intValue(response.Result, "page_count"),
	}, nil
}

type ExtractTextTool struct{ meta tools.ToolMetadata }

func NewExtractTextTool() *ExtractTextTool {
	return &ExtractTextTool{meta: tools.ToolMetadata{
		Name:            "extract_text",
		DisplayName:     "文本提取",
		Description:     "通过 OCR worker 从文本文件、PDF 或图片中提取正文内容",
		Source:          tools.ToolSourceWorker,
		RiskHint:        "green",
		TimeoutSec:      tools.DefaultTimeoutSec,
		InputSchemaRef:  "tools/extract_text/input",
		OutputSchemaRef: "tools/extract_text/output",
		SupportsDryRun:  false,
	}}
}

func (t *ExtractTextTool) Metadata() tools.ToolMetadata { return t.meta }

func (t *ExtractTextTool) Validate(input map[string]any) error {
	path, ok := input["path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		return fmt.Errorf("input field 'path' must be a non-empty string")
	}
	return nil
}

func (t *ExtractTextTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.OCR == nil {
		return nil, tools.ErrOCRWorkerFailed
	}
	path := strings.TrimSpace(input["path"].(string))
	result, err := execCtx.OCR.ExtractText(ctx, path)
	if err != nil {
		return nil, err
	}
	return buildOCRToolResult(t.meta.Name, result), nil
}

type OCRImageTool struct{ meta tools.ToolMetadata }

func NewOCRImageTool() *OCRImageTool {
	return &OCRImageTool{meta: tools.ToolMetadata{
		Name:            "ocr_image",
		DisplayName:     "图片 OCR",
		Description:     "通过 OCR worker 对图片执行文字识别",
		Source:          tools.ToolSourceWorker,
		RiskHint:        "green",
		TimeoutSec:      tools.DefaultTimeoutSec,
		InputSchemaRef:  "tools/ocr_image/input",
		OutputSchemaRef: "tools/ocr_image/output",
		SupportsDryRun:  false,
	}}
}

func (t *OCRImageTool) Metadata() tools.ToolMetadata { return t.meta }

func (t *OCRImageTool) Validate(input map[string]any) error {
	path, ok := input["path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		return fmt.Errorf("input field 'path' must be a non-empty string")
	}
	return nil
}

func (t *OCRImageTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.OCR == nil {
		return nil, tools.ErrOCRWorkerFailed
	}
	result, err := execCtx.OCR.OCRImage(ctx, strings.TrimSpace(input["path"].(string)), stringValueMap(input, "language"))
	if err != nil {
		return nil, err
	}
	return buildOCRToolResult(t.meta.Name, result), nil
}

type OCRPDFTool struct{ meta tools.ToolMetadata }

func NewOCRPDFTool() *OCRPDFTool {
	return &OCRPDFTool{meta: tools.ToolMetadata{
		Name:            "ocr_pdf",
		DisplayName:     "PDF OCR",
		Description:     "通过 OCR worker 对 PDF 执行文本提取或 OCR 识别",
		Source:          tools.ToolSourceWorker,
		RiskHint:        "green",
		TimeoutSec:      tools.DefaultTimeoutSec,
		InputSchemaRef:  "tools/ocr_pdf/input",
		OutputSchemaRef: "tools/ocr_pdf/output",
		SupportsDryRun:  false,
	}}
}

func (t *OCRPDFTool) Metadata() tools.ToolMetadata { return t.meta }

func (t *OCRPDFTool) Validate(input map[string]any) error {
	path, ok := input["path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		return fmt.Errorf("input field 'path' must be a non-empty string")
	}
	return nil
}

func (t *OCRPDFTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.OCR == nil {
		return nil, tools.ErrOCRWorkerFailed
	}
	result, err := execCtx.OCR.OCRPDF(ctx, strings.TrimSpace(input["path"].(string)), stringValueMap(input, "language"))
	if err != nil {
		return nil, err
	}
	return buildOCRToolResult(t.meta.Name, result), nil
}

func RegisterOCRTools(registry *tools.Registry) error {
	for _, tool := range []tools.Tool{NewExtractTextTool(), NewOCRImageTool(), NewOCRPDFTool()} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

func buildOCRToolResult(toolName string, result tools.OCRTextResult) *tools.ToolResult {
	rawOutput := map[string]any{
		"path":       result.Path,
		"text":       result.Text,
		"language":   result.Language,
		"page_count": result.PageCount,
		"source":     firstNonEmptyString(result.Source, "ocr_worker"),
	}
	return &tools.ToolResult{
		ToolName:      toolName,
		RawOutput:     rawOutput,
		SummaryOutput: map[string]any{"path": result.Path, "language": result.Language, "page_count": result.PageCount, "content_preview": previewPageText(result.Text), "source": firstNonEmptyString(result.Source, "ocr_worker")},
	}
}
