package sidecarclient

import (
	"context"
	"errors"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestMediaWorkerRuntimeClientExtractFrames(t *testing.T) {
	runtime := &MediaWorkerRuntime{
		os:        platform.NewLocalOSCapabilityAdapter(),
		ready:     true,
		available: true,
		invoker: &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{
			"input_path":  "workspace/demo.mp4",
			"output_dir":  "workspace/frames",
			"frame_paths": []any{"workspace/frames/frame-001.jpg", "workspace/frames/frame-002.jpg"},
			"frame_count": 2,
			"source":      "media_worker_frames",
		}}},
		name: "media_worker",
	}
	runtime.client = runtimeMediaWorkerClient{runtime: runtime}
	result, err := runtime.Client().ExtractFrames(context.Background(), "workspace/demo.mp4", "workspace/frames", 1, 2)
	if err != nil {
		t.Fatalf("ExtractFrames returned error: %v", err)
	}
	if result.FrameCount != 2 || len(result.FramePaths) != 2 {
		t.Fatalf("unexpected frame extraction result: %+v", result)
	}
}

func TestMediaWorkerRuntimeLifecycle(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewMediaWorkerRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewMediaWorkerRuntime returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{"status": "ok"}}}
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if !runtime.Ready() || !osCapability.HasNamedPipe(runtime.PipeName()) {
		t.Fatalf("expected media runtime ready after start")
	}
	if err := runtime.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
}

func TestMediaWorkerRuntimeTranscodeAndFailureHandling(t *testing.T) {
	runtime := &MediaWorkerRuntime{
		os:        platform.NewLocalOSCapabilityAdapter(),
		ready:     true,
		available: true,
		invoker: &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{
			"input_path":  "workspace/demo.mov",
			"output_path": "workspace/demo.mp4",
			"format":      "mp4",
			"source":      "media_worker_ffmpeg",
		}}},
		name: "media_worker",
	}
	runtime.client = runtimeMediaWorkerClient{runtime: runtime}
	transcodeResult, err := runtime.Client().TranscodeMedia(context.Background(), "workspace/demo.mov", "workspace/demo.mp4", "mp4")
	if err != nil {
		t.Fatalf("TranscodeMedia returned error: %v", err)
	}
	if transcodeResult.OutputPath != "workspace/demo.mp4" {
		t.Fatalf("unexpected transcode result: %+v", transcodeResult)
	}
	runtime.invoker = &stubWorkerInvoker{err: sidecarTransportError{err: errors.New("worker crashed")}}
	_, err = runtime.Client().NormalizeRecording(context.Background(), "workspace/demo.mov", "workspace/demo.mp4")
	if !errors.Is(err, tools.ErrMediaWorkerFailed) {
		t.Fatalf("expected ErrMediaWorkerFailed, got %v", err)
	}
}

func TestMediaWorkerRuntimeRequestFailureKeepsReadyState(t *testing.T) {
	runtime := &MediaWorkerRuntime{
		os:        platform.NewLocalOSCapabilityAdapter(),
		ready:     true,
		available: true,
		invoker:   &stubWorkerInvoker{err: sidecarRequestError{code: "media_request_failed", message: "bad input"}},
		name:      "media_worker",
	}
	runtime.client = runtimeMediaWorkerClient{runtime: runtime}
	_, err := runtime.Client().TranscodeMedia(context.Background(), "workspace/demo.mov", "workspace/demo.mp4", "mp4")
	if !errors.Is(err, tools.ErrMediaWorkerFailed) {
		t.Fatalf("expected ErrMediaWorkerFailed, got %v", err)
	}
	if !runtime.Ready() {
		t.Fatal("expected request failure to keep media runtime ready")
	}
}

func TestMediaWorkerRuntimeStartFailsHealthCheck(t *testing.T) {
	runtime := &MediaWorkerRuntime{
		os:        platform.NewLocalOSCapabilityAdapter(),
		ready:     false,
		available: true,
		invoker:   &stubWorkerInvoker{err: sidecarTransportError{err: errors.New("health failed")}},
		name:      "media_worker",
	}
	err := runtime.Start()
	if !errors.Is(err, tools.ErrMediaWorkerFailed) {
		t.Fatalf("expected ErrMediaWorkerFailed, got %v", err)
	}
}

func TestMediaToolsExecuteAndValidate(t *testing.T) {
	mediaClient := stubMediaWorkerClient{
		transcodeResult: tools.MediaTranscodeResult{InputPath: "workspace/demo.mov", OutputPath: "workspace/demo.mp4", Format: "mp4", Source: "media_worker_ffmpeg"},
		framesResult:    tools.MediaFrameExtractResult{InputPath: "workspace/demo.mov", OutputDir: "workspace/frames", FramePaths: []string{"workspace/frames/frame-001.jpg"}, FrameCount: 1, Source: "media_worker_frames"},
	}
	transcodeTool := NewTranscodeMediaTool()
	if err := transcodeTool.Validate(map[string]any{"path": "workspace/demo.mov", "output_path": "workspace/demo.mp4"}); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	transcodeResult, err := transcodeTool.Execute(context.Background(), &tools.ToolExecuteContext{Media: mediaClient}, map[string]any{"path": "workspace/demo.mov", "output_path": "workspace/demo.mp4", "format": "mp4"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(transcodeResult.Artifacts) != 1 {
		t.Fatalf("expected artifact from media transcode, got %+v", transcodeResult)
	}
	framesTool := NewExtractFramesTool()
	framesResult, err := framesTool.Execute(context.Background(), &tools.ToolExecuteContext{Media: mediaClient}, map[string]any{"path": "workspace/demo.mov", "output_dir": "workspace/frames", "limit": 1.0})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if framesResult.RawOutput["frame_count"] != 1 {
		t.Fatalf("unexpected frame extraction output: %+v", framesResult.RawOutput)
	}
}

type stubMediaWorkerClient struct {
	transcodeResult tools.MediaTranscodeResult
	framesResult    tools.MediaFrameExtractResult
}

func (s stubMediaWorkerClient) TranscodeMedia(_ context.Context, _, _, _ string) (tools.MediaTranscodeResult, error) {
	return s.transcodeResult, nil
}

func (s stubMediaWorkerClient) NormalizeRecording(_ context.Context, _, _ string) (tools.MediaTranscodeResult, error) {
	return s.transcodeResult, nil
}

func (s stubMediaWorkerClient) ExtractFrames(_ context.Context, _, _ string, _ float64, _ int) (tools.MediaFrameExtractResult, error) {
	return s.framesResult, nil
}

func TestRegisterMediaTools(t *testing.T) {
	registry := tools.NewRegistry()
	if err := RegisterMediaTools(registry); err != nil {
		t.Fatalf("RegisterMediaTools returned error: %v", err)
	}
	for _, toolName := range []string{"transcode_media", "extract_frames", "normalize_recording"} {
		if _, err := registry.Get(toolName); err != nil {
			t.Fatalf("expected %s to be registered, got %v", toolName, err)
		}
	}
}

func TestMediaNoopUnavailableAndValidationBranches(t *testing.T) {
	client := NewNoopMediaWorkerClient()
	if _, err := client.NormalizeRecording(context.Background(), "workspace/demo.mov", "workspace/demo.mp4"); !errors.Is(err, tools.ErrMediaWorkerFailed) {
		t.Fatalf("expected noop media failure, got %v", err)
	}
	runtime := NewUnavailableMediaWorkerRuntime(plugin.NewService(), platform.NewLocalOSCapabilityAdapter())
	if runtime.Available() {
		t.Fatal("expected unavailable media runtime")
	}
	if runtime.Name() != "media_worker" {
		t.Fatalf("unexpected runtime name: %q", runtime.Name())
	}
	if err := NewNormalizeRecordingTool().Validate(map[string]any{"path": "workspace/demo.mov", "output_path": "workspace/demo.mp4"}); err != nil {
		t.Fatalf("expected normalize_recording validate to pass, got %v", err)
	}
	if err := NewExtractFramesTool().Validate(map[string]any{"path": "workspace/demo.mov", "output_dir": "workspace/frames"}); err != nil {
		t.Fatalf("expected extract_frames validate to pass, got %v", err)
	}
}
