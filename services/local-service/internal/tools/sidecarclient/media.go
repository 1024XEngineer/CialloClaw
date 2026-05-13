package sidecarclient

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

const mediaWorkerRelativePath = "workers/media-worker/src/index.js"

type noopMediaWorkerClient struct{}

func NewNoopMediaWorkerClient() tools.MediaWorkerClient {
	return noopMediaWorkerClient{}
}

func (noopMediaWorkerClient) TranscodeMedia(_ context.Context, _, _, _ string) (tools.MediaTranscodeResult, error) {
	return tools.MediaTranscodeResult{}, tools.ErrMediaWorkerFailed
}

func (noopMediaWorkerClient) NormalizeRecording(_ context.Context, _, _ string) (tools.MediaTranscodeResult, error) {
	return tools.MediaTranscodeResult{}, tools.ErrMediaWorkerFailed
}

func (noopMediaWorkerClient) ExtractFrames(_ context.Context, _, _ string, _ float64, _ int) (tools.MediaFrameExtractResult, error) {
	return tools.MediaFrameExtractResult{}, tools.ErrMediaWorkerFailed
}

type runtimeMediaWorkerClient struct {
	runtime *MediaWorkerRuntime
}

func (c runtimeMediaWorkerClient) TranscodeMedia(ctx context.Context, inputPath, outputPath, format string) (tools.MediaTranscodeResult, error) {
	return c.runtime.invokeTranscodeAction(ctx, sidecarRequest{Action: "transcode_media", Path: inputPath, OutputPath: outputPath, Format: format})
}

func (c runtimeMediaWorkerClient) NormalizeRecording(ctx context.Context, inputPath, outputPath string) (tools.MediaTranscodeResult, error) {
	return c.runtime.invokeTranscodeAction(ctx, sidecarRequest{Action: "normalize_recording", Path: inputPath, OutputPath: outputPath})
}

func (c runtimeMediaWorkerClient) ExtractFrames(ctx context.Context, inputPath, outputDir string, everySeconds float64, limit int) (tools.MediaFrameExtractResult, error) {
	if c.runtime == nil || !c.runtime.Available() {
		return tools.MediaFrameExtractResult{}, tools.ErrMediaWorkerFailed
	}
	if !c.runtime.Ready() {
		return tools.MediaFrameExtractResult{}, tools.ErrMediaWorkerFailed
	}
	response, err := c.runtime.invoke(ctx, sidecarRequest{Action: "extract_frames", Path: inputPath, OutputDir: outputDir, EverySeconds: everySeconds, Limit: limit})
	if err != nil {
		if shouldMarkRuntimeFailure(err) {
			_ = c.runtime.markFailure()
		}
		return tools.MediaFrameExtractResult{}, fmt.Errorf("%w: %v", tools.ErrMediaWorkerFailed, err)
	}
	return tools.MediaFrameExtractResult{
		InputPath:  stringValue(response.Result, "input_path"),
		OutputDir:  stringValue(response.Result, "output_dir"),
		FramePaths: stringSliceValue(response.Result, "frame_paths"),
		FrameCount: intValue(response.Result, "frame_count"),
		Source:     firstNonEmptyString(stringValue(response.Result, "source"), "media_worker"),
	}, nil
}

// MediaWorkerRuntime manages the media worker lifecycle.
type MediaWorkerRuntime struct {
	mu        sync.Mutex
	plugins   *plugin.Service
	os        platform.OSCapabilityAdapter
	ready     bool
	available bool
	invoker   workerInvoker
	client    runtimeMediaWorkerClient
	name      string
}

func NewMediaWorkerRuntime(pluginService *plugin.Service, osCapability platform.OSCapabilityAdapter) (*MediaWorkerRuntime, error) {
	runtime := &MediaWorkerRuntime{
		plugins:   pluginService,
		os:        osCapability,
		ready:     false,
		available: false,
		name:      "media_worker",
	}
	runtime.client = runtimeMediaWorkerClient{runtime: runtime}
	entryPath, err := resolveRelativePathFromRoots(mediaWorkerRelativePath, workerSearchRoots())
	if err != nil {
		markPluginRuntimeFailed(pluginService, plugin.RuntimeKindWorker, "media_worker", err)
		return runtime, err
	}
	markPluginRuntimeStarting(pluginService, plugin.RuntimeKindWorker, "media_worker")
	runtime.available = true
	runtime.invoker = newCommandWorkerInvoker(entryPath)
	return runtime, nil
}

func NewUnavailableMediaWorkerRuntime(pluginService *plugin.Service, osCapability platform.OSCapabilityAdapter) *MediaWorkerRuntime {
	markPluginRuntimeUnavailable(pluginService, plugin.RuntimeKindWorker, "media_worker", "media worker unavailable")
	runtime := &MediaWorkerRuntime{plugins: pluginService, os: osCapability, ready: false, available: false, name: "media_worker"}
	runtime.client = runtimeMediaWorkerClient{runtime: runtime}
	return runtime
}

func (r *MediaWorkerRuntime) Name() string { return r.name }

func (r *MediaWorkerRuntime) PipeName() string { return sidecarPipeName(r.name) }

func (r *MediaWorkerRuntime) Ready() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ready
}

func (r *MediaWorkerRuntime) Available() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.available
}

func (r *MediaWorkerRuntime) Start() error {
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
		return fmt.Errorf("%w: %v", tools.ErrMediaWorkerFailed, err)
	}
	r.mu.Lock()
	r.ready = true
	r.mu.Unlock()
	markPluginRuntimeHealthy(r.plugins, plugin.RuntimeKindWorker, r.name)
	return nil
}

func (r *MediaWorkerRuntime) Stop() error {
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

func (r *MediaWorkerRuntime) Client() tools.MediaWorkerClient { return r.client }

func (r *MediaWorkerRuntime) invoke(ctx context.Context, request sidecarRequest) (sidecarResponse, error) {
	if r == nil || r.invoker == nil {
		return sidecarResponse{}, errors.New("media worker invoker is not available")
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

func (r *MediaWorkerRuntime) markFailure() error {
	r.mu.Lock()
	r.ready = false
	r.mu.Unlock()
	markPluginRuntimeFailed(r.plugins, plugin.RuntimeKindWorker, r.name, errors.New("media worker marked failed"))
	if r.os == nil {
		return nil
	}
	return r.os.CloseNamedPipe(r.PipeName())
}

func (r *MediaWorkerRuntime) invokeTranscodeAction(ctx context.Context, request sidecarRequest) (tools.MediaTranscodeResult, error) {
	if r == nil || !r.Available() {
		return tools.MediaTranscodeResult{}, tools.ErrMediaWorkerFailed
	}
	if !r.Ready() {
		return tools.MediaTranscodeResult{}, tools.ErrMediaWorkerFailed
	}
	response, err := r.invoke(ctx, request)
	if err != nil {
		if shouldMarkRuntimeFailure(err) {
			_ = r.markFailure()
		}
		return tools.MediaTranscodeResult{}, fmt.Errorf("%w: %v", tools.ErrMediaWorkerFailed, err)
	}
	return tools.MediaTranscodeResult{
		InputPath:  stringValue(response.Result, "input_path"),
		OutputPath: stringValue(response.Result, "output_path"),
		Format:     stringValue(response.Result, "format"),
		Source:     firstNonEmptyString(stringValue(response.Result, "source"), "media_worker"),
	}, nil
}

type TranscodeMediaTool struct{ meta tools.ToolMetadata }

func NewTranscodeMediaTool() *TranscodeMediaTool {
	return &TranscodeMediaTool{meta: tools.ToolMetadata{Name: "transcode_media", DisplayName: "媒体转码", Description: "通过 Media worker 执行音视频转码", Source: tools.ToolSourceWorker, RiskHint: "green", TimeoutSec: tools.DefaultTimeoutSec, InputSchemaRef: "tools/transcode_media/input", OutputSchemaRef: "tools/transcode_media/output"}}
}

func (t *TranscodeMediaTool) Metadata() tools.ToolMetadata { return t.meta }

func (t *TranscodeMediaTool) Validate(input map[string]any) error {
	return validateMediaIO(input, true)
}

func (t *TranscodeMediaTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.Media == nil {
		return nil, tools.ErrMediaWorkerFailed
	}
	result, err := execCtx.Media.TranscodeMedia(ctx, stringValueMap(input, "path"), stringValueMap(input, "output_path"), stringValueMap(input, "format"))
	if err != nil {
		return nil, err
	}
	return buildMediaTranscodeResult(t.meta.Name, result), nil
}

type NormalizeRecordingTool struct{ meta tools.ToolMetadata }

func NewNormalizeRecordingTool() *NormalizeRecordingTool {
	return &NormalizeRecordingTool{meta: tools.ToolMetadata{Name: "normalize_recording", DisplayName: "录屏归一化", Description: "通过 Media worker 归一化录屏或录音结果", Source: tools.ToolSourceWorker, RiskHint: "green", TimeoutSec: tools.DefaultTimeoutSec, InputSchemaRef: "tools/normalize_recording/input", OutputSchemaRef: "tools/normalize_recording/output"}}
}

func (t *NormalizeRecordingTool) Metadata() tools.ToolMetadata { return t.meta }

func (t *NormalizeRecordingTool) Validate(input map[string]any) error {
	return validateMediaIO(input, true)
}

func (t *NormalizeRecordingTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.Media == nil {
		return nil, tools.ErrMediaWorkerFailed
	}
	result, err := execCtx.Media.NormalizeRecording(ctx, stringValueMap(input, "path"), stringValueMap(input, "output_path"))
	if err != nil {
		return nil, err
	}
	return buildMediaTranscodeResult(t.meta.Name, result), nil
}

type ExtractFramesTool struct{ meta tools.ToolMetadata }

func NewExtractFramesTool() *ExtractFramesTool {
	return &ExtractFramesTool{meta: tools.ToolMetadata{Name: "extract_frames", DisplayName: "抽取帧", Description: "通过 Media worker 从视频中抽取关键帧", Source: tools.ToolSourceWorker, RiskHint: "green", TimeoutSec: tools.DefaultTimeoutSec, InputSchemaRef: "tools/extract_frames/input", OutputSchemaRef: "tools/extract_frames/output"}}
}

func (t *ExtractFramesTool) Metadata() tools.ToolMetadata { return t.meta }

func (t *ExtractFramesTool) Validate(input map[string]any) error {
	return validateMediaOutputDir(input)
}

func (t *ExtractFramesTool) Execute(ctx context.Context, execCtx *tools.ToolExecuteContext, input map[string]any) (*tools.ToolResult, error) {
	if execCtx == nil || execCtx.Media == nil {
		return nil, tools.ErrMediaWorkerFailed
	}
	everySeconds := 1.0
	if rawValue, ok := input["every_seconds"].(float64); ok && rawValue > 0 {
		everySeconds = rawValue
	}
	limit := 5
	if rawValue, ok := input["limit"].(float64); ok && int(rawValue) > 0 {
		limit = int(rawValue)
	}
	result, err := execCtx.Media.ExtractFrames(ctx, stringValueMap(input, "path"), stringValueMap(input, "output_dir"), everySeconds, limit)
	if err != nil {
		return nil, err
	}
	rawOutput := map[string]any{"input_path": result.InputPath, "output_dir": result.OutputDir, "frame_paths": append([]string(nil), result.FramePaths...), "frame_count": result.FrameCount, "source": firstNonEmptyString(result.Source, "media_worker")}
	return &tools.ToolResult{ToolName: t.meta.Name, RawOutput: rawOutput, SummaryOutput: map[string]any{"output_dir": result.OutputDir, "frame_count": result.FrameCount, "source": firstNonEmptyString(result.Source, "media_worker")}}, nil
}

func RegisterMediaTools(registry *tools.Registry) error {
	for _, tool := range []tools.Tool{NewTranscodeMediaTool(), NewNormalizeRecordingTool(), NewExtractFramesTool()} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

func validateMediaIO(input map[string]any, requireOutput bool) error {
	path, ok := input["path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		return fmt.Errorf("input field 'path' must be a non-empty string")
	}
	if requireOutput {
		outputPath, ok := input["output_path"].(string)
		if !ok || strings.TrimSpace(outputPath) == "" {
			return fmt.Errorf("input field 'output_path' must be a non-empty string")
		}
	}
	return nil
}

func validateMediaOutputDir(input map[string]any) error {
	path, ok := input["path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		return fmt.Errorf("input field 'path' must be a non-empty string")
	}
	outputDir, ok := input["output_dir"].(string)
	if !ok || strings.TrimSpace(outputDir) == "" {
		return fmt.Errorf("input field 'output_dir' must be a non-empty string")
	}
	return nil
}

func buildMediaTranscodeResult(toolName string, result tools.MediaTranscodeResult) *tools.ToolResult {
	rawOutput := map[string]any{"input_path": result.InputPath, "output_path": result.OutputPath, "format": result.Format, "source": firstNonEmptyString(result.Source, "media_worker")}
	artifact := tools.ArtifactRef{ArtifactType: "generated_file", Title: filepath.Base(result.OutputPath), Path: result.OutputPath, MimeType: guessMediaMimeType(result.OutputPath)}
	return &tools.ToolResult{ToolName: toolName, RawOutput: rawOutput, SummaryOutput: map[string]any{"output_path": result.OutputPath, "format": result.Format, "source": firstNonEmptyString(result.Source, "media_worker")}, Artifacts: []tools.ArtifactRef{artifact}}
}

func guessMediaMimeType(path string) string {
	trimmed := strings.ToLower(strings.TrimSpace(path))
	switch {
	case strings.HasSuffix(trimmed, ".mp4"):
		return "video/mp4"
	case strings.HasSuffix(trimmed, ".mp3"):
		return "audio/mpeg"
	case strings.HasSuffix(trimmed, ".wav"):
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}
