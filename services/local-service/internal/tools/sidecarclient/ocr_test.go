package sidecarclient

import (
	"context"
	"errors"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestOCRWorkerRuntimeClientExtractText(t *testing.T) {
	runtime := &OCRWorkerRuntime{
		os:        platform.NewLocalOSCapabilityAdapter(),
		ready:     true,
		available: true,
		invoker: &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{
			"path":       "workspace/demo.txt",
			"text":       "hello ocr",
			"language":   "plain_text",
			"page_count": 1,
			"source":     "ocr_worker_text",
		}}},
		name: "ocr_worker",
	}
	runtime.client = runtimeOCRWorkerClient{runtime: runtime}
	result, err := runtime.Client().ExtractText(context.Background(), "workspace/demo.txt")
	if err != nil {
		t.Fatalf("ExtractText returned error: %v", err)
	}
	if result.Text != "hello ocr" || result.Source != "ocr_worker_text" {
		t.Fatalf("unexpected extract text result: %+v", result)
	}
}

func TestOCRWorkerRuntimeLifecycle(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewOCRWorkerRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewOCRWorkerRuntime returned error: %v", err)
	}
	runtime.invoker = &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{"status": "ok"}}}
	if err := runtime.Start(); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if !runtime.Ready() || !osCapability.HasNamedPipe(runtime.PipeName()) {
		t.Fatalf("expected runtime ready after start")
	}
	if err := runtime.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}
	if runtime.Ready() {
		t.Fatal("expected runtime not ready after stop")
	}
}

func TestOCRWorkerRuntimeImageAndPDFActions(t *testing.T) {
	runtime := &OCRWorkerRuntime{
		os:        platform.NewLocalOSCapabilityAdapter(),
		ready:     true,
		available: true,
		invoker: &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{
			"path":       "workspace/demo.png",
			"text":       "image text",
			"language":   "eng",
			"page_count": 1,
			"source":     "ocr_worker_tesseract",
		}}},
		name: "ocr_worker",
	}
	runtime.client = runtimeOCRWorkerClient{runtime: runtime}
	imageResult, err := runtime.Client().OCRImage(context.Background(), "workspace/demo.png", "eng")
	if err != nil {
		t.Fatalf("OCRImage returned error: %v", err)
	}
	if imageResult.Text != "image text" {
		t.Fatalf("unexpected image OCR result: %+v", imageResult)
	}
	runtime.invoker = &stubWorkerInvoker{response: sidecarResponse{OK: true, Result: map[string]any{
		"path":       "workspace/demo.pdf",
		"text":       "pdf text",
		"language":   "pdf_text",
		"page_count": 3,
		"source":     "ocr_worker_pdf",
	}}}
	pdfResult, err := runtime.Client().OCRPDF(context.Background(), "workspace/demo.pdf", "eng")
	if err != nil {
		t.Fatalf("OCRPDF returned error: %v", err)
	}
	if pdfResult.PageCount != 3 || pdfResult.Source != "ocr_worker_pdf" {
		t.Fatalf("unexpected pdf OCR result: %+v", pdfResult)
	}
}

func TestOCRWorkerRuntimeTransportFailureMarksRuntimeUnavailable(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime := &OCRWorkerRuntime{
		os:        osCapability,
		ready:     true,
		available: true,
		invoker:   &stubWorkerInvoker{err: sidecarTransportError{err: errors.New("worker crashed")}},
		name:      "ocr_worker",
	}
	runtime.client = runtimeOCRWorkerClient{runtime: runtime}
	if err := osCapability.EnsureNamedPipe(runtime.PipeName()); err != nil {
		t.Fatalf("ensure pipe: %v", err)
	}
	_, err := runtime.Client().ExtractText(context.Background(), "workspace/demo.txt")
	if !errors.Is(err, tools.ErrOCRWorkerFailed) {
		t.Fatalf("expected ErrOCRWorkerFailed, got %v", err)
	}
	if runtime.Ready() {
		t.Fatal("expected runtime not ready after transport failure")
	}
}

func TestOCRWorkerRuntimeRequestFailureKeepsReadyState(t *testing.T) {
	runtime := &OCRWorkerRuntime{
		os:        platform.NewLocalOSCapabilityAdapter(),
		ready:     true,
		available: true,
		invoker:   &stubWorkerInvoker{err: sidecarRequestError{code: "ocr_request_failed", message: "bad input"}},
		name:      "ocr_worker",
	}
	runtime.client = runtimeOCRWorkerClient{runtime: runtime}
	_, err := runtime.Client().ExtractText(context.Background(), "workspace/demo.txt")
	if !errors.Is(err, tools.ErrOCRWorkerFailed) {
		t.Fatalf("expected ErrOCRWorkerFailed, got %v", err)
	}
	if !runtime.Ready() {
		t.Fatal("expected request failure to keep runtime ready")
	}
}

func TestOCRWorkerRuntimeStartFailsHealthCheck(t *testing.T) {
	runtime := &OCRWorkerRuntime{
		os:        platform.NewLocalOSCapabilityAdapter(),
		ready:     false,
		available: true,
		invoker:   &stubWorkerInvoker{err: sidecarTransportError{err: errors.New("health failed")}},
		name:      "ocr_worker",
	}
	err := runtime.Start()
	if !errors.Is(err, tools.ErrOCRWorkerFailed) {
		t.Fatalf("expected ErrOCRWorkerFailed, got %v", err)
	}
}

func TestOCRToolsExecuteAndValidate(t *testing.T) {
	ocrClient := stubOCRWorkerClient{result: tools.OCRTextResult{Path: "workspace/demo.txt", Text: "hello ocr", Language: "plain_text", PageCount: 1, Source: "ocr_worker_text"}}
	for _, tool := range []tools.Tool{NewExtractTextTool(), NewOCRImageTool(), NewOCRPDFTool()} {
		if err := tool.Validate(map[string]any{"path": "workspace/demo.txt"}); err != nil {
			t.Fatalf("Validate returned error: %v", err)
		}
		result, err := tool.Execute(context.Background(), &tools.ToolExecuteContext{OCR: ocrClient}, map[string]any{"path": "workspace/demo.txt", "language": "eng"})
		if err != nil {
			t.Fatalf("Execute returned error: %v", err)
		}
		if result.SummaryOutput["content_preview"] == "" {
			t.Fatalf("expected content preview, got %+v", result.SummaryOutput)
		}
	}
}

type stubOCRWorkerClient struct{ result tools.OCRTextResult }

func (s stubOCRWorkerClient) ExtractText(_ context.Context, _ string) (tools.OCRTextResult, error) {
	return s.result, nil
}

func (s stubOCRWorkerClient) OCRImage(_ context.Context, _ string, _ string) (tools.OCRTextResult, error) {
	return s.result, nil
}

func (s stubOCRWorkerClient) OCRPDF(_ context.Context, _ string, _ string) (tools.OCRTextResult, error) {
	return s.result, nil
}

func TestRegisterOCRTools(t *testing.T) {
	registry := tools.NewRegistry()
	if err := RegisterOCRTools(registry); err != nil {
		t.Fatalf("RegisterOCRTools returned error: %v", err)
	}
	for _, toolName := range []string{"extract_text", "ocr_image", "ocr_pdf"} {
		if _, err := registry.Get(toolName); err != nil {
			t.Fatalf("expected %s to be registered, got %v", toolName, err)
		}
	}
}

func TestOCRNoopAndUnavailableRuntime(t *testing.T) {
	client := NewNoopOCRWorkerClient()
	if _, err := client.ExtractText(context.Background(), "workspace/demo.txt"); !errors.Is(err, tools.ErrOCRWorkerFailed) {
		t.Fatalf("expected noop OCR failure, got %v", err)
	}
	runtime := NewUnavailableOCRWorkerRuntime(plugin.NewService(), platform.NewLocalOSCapabilityAdapter())
	if runtime.Available() {
		t.Fatal("expected unavailable OCR runtime")
	}
	if runtime.Name() != "ocr_worker" {
		t.Fatalf("unexpected runtime name: %q", runtime.Name())
	}
}
