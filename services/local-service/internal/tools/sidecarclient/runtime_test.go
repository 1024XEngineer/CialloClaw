package sidecarclient

import (
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestPlaywrightSidecarRuntimeLifecycle(t *testing.T) {
	osCapability := platform.NewLocalOSCapabilityAdapter()
	runtime, err := NewPlaywrightSidecarRuntime(plugin.NewService(), osCapability)
	if err != nil {
		t.Fatalf("NewPlaywrightSidecarRuntime returned error: %v", err)
	}
	if runtime.Name() != "playwright_sidecar" {
		t.Fatalf("unexpected runtime name: %q", runtime.Name())
	}
	if runtime.Ready() {
		t.Fatal("expected runtime to start as not ready")
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
