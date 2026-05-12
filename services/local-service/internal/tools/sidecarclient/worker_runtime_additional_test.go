package sidecarclient

import (
	"context"
	"errors"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestNoopWorkerClientsCoverAllMethods(t *testing.T) {
	playwrightClient := NewNoopPlaywrightSidecarClient()
	if _, err := playwrightClient.SearchPage(context.Background(), "https://example.com", "broken", 1); !errors.Is(err, tools.ErrPlaywrightSidecarFailed) {
		t.Fatalf("expected noop playwright search to fail, got %v", err)
	}
	ocrClient := NewNoopOCRWorkerClient()
	if _, err := ocrClient.OCRImage(context.Background(), "workspace/demo.png", "eng"); !errors.Is(err, tools.ErrOCRWorkerFailed) {
		t.Fatalf("expected noop OCR image to fail, got %v", err)
	}
	if _, err := ocrClient.OCRPDF(context.Background(), "workspace/demo.pdf", "eng"); !errors.Is(err, tools.ErrOCRWorkerFailed) {
		t.Fatalf("expected noop OCR pdf to fail, got %v", err)
	}

	mediaClient := NewNoopMediaWorkerClient()
	if _, err := mediaClient.TranscodeMedia(context.Background(), "workspace/demo.mov", "workspace/demo.mp4", "mp4"); !errors.Is(err, tools.ErrMediaWorkerFailed) {
		t.Fatalf("expected noop media transcode to fail, got %v", err)
	}
	if _, err := mediaClient.ExtractFrames(context.Background(), "workspace/demo.mov", "workspace/frames", 1, 1); !errors.Is(err, tools.ErrMediaWorkerFailed) {
		t.Fatalf("expected noop media frame extraction to fail, got %v", err)
	}
}

func TestWorkerRuntimeConstructorsExposeAvailableCapabilities(t *testing.T) {
	ocrRuntime, err := NewOCRWorkerRuntime(plugin.NewService(), platform.NewLocalOSCapabilityAdapter())
	if err != nil {
		t.Fatalf("NewOCRWorkerRuntime returned error: %v", err)
	}
	if !ocrRuntime.Available() || ocrRuntime.Name() != "ocr_worker" || ocrRuntime.PipeName() == "" {
		t.Fatalf("expected constructed OCR runtime metadata, got %+v", ocrRuntime)
	}

	mediaRuntime, err := NewMediaWorkerRuntime(plugin.NewService(), platform.NewLocalOSCapabilityAdapter())
	if err != nil {
		t.Fatalf("NewMediaWorkerRuntime returned error: %v", err)
	}
	if !mediaRuntime.Available() || mediaRuntime.Name() != "media_worker" || mediaRuntime.PipeName() == "" {
		t.Fatalf("expected constructed media runtime metadata, got %+v", mediaRuntime)
	}
}

func TestWorkerRuntimeUnavailableAndNilOSBranches(t *testing.T) {
	ocrUnavailable := NewUnavailableOCRWorkerRuntime(plugin.NewService(), platform.NewLocalOSCapabilityAdapter())
	if err := ocrUnavailable.Start(); err != nil {
		t.Fatalf("expected unavailable OCR runtime start to noop, got %v", err)
	}
	if err := ocrUnavailable.Stop(); err != nil {
		t.Fatalf("expected unavailable OCR runtime stop to noop, got %v", err)
	}

	mediaUnavailable := NewUnavailableMediaWorkerRuntime(plugin.NewService(), platform.NewLocalOSCapabilityAdapter())
	if err := mediaUnavailable.Start(); err != nil {
		t.Fatalf("expected unavailable media runtime start to noop, got %v", err)
	}
	if err := mediaUnavailable.Stop(); err != nil {
		t.Fatalf("expected unavailable media runtime stop to noop, got %v", err)
	}

	ocrRuntime := &OCRWorkerRuntime{available: true, name: "ocr_worker"}
	if err := ocrRuntime.Start(); err == nil || err.Error() != "os capability adapter is required" {
		t.Fatalf("expected OCR runtime without os capability to fail, got %v", err)
	}

	mediaRuntime := &MediaWorkerRuntime{available: true, name: "media_worker"}
	if err := mediaRuntime.Start(); err == nil || err.Error() != "os capability adapter is required" {
		t.Fatalf("expected media runtime without os capability to fail, got %v", err)
	}
}
