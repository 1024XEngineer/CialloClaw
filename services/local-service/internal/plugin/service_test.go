package plugin

import "testing"

func TestServiceRuntimeLifecycleAndSnapshots(t *testing.T) {
	service := NewService()
	if len(service.Workers()) != 3 || len(service.Sidecars()) != 1 {
		t.Fatalf("expected declared workers and sidecars, got workers=%+v sidecars=%+v", service.Workers(), service.Sidecars())
	}
	if got := service.Workers(); got[0] != "playwright_worker" || got[1] != "ocr_worker" || got[2] != "media_worker" {
		t.Fatalf("expected worker order to stay stable, got %+v", got)
	}
	if got := service.Sidecars(); got[0] != "playwright_sidecar" {
		t.Fatalf("expected sidecar order to stay stable, got %+v", got)
	}
	if !service.HasSidecar("playwright_sidecar") {
		t.Fatal("expected declared sidecar to be discoverable")
	}
	if _, ok := service.SidecarSpec("playwright_sidecar"); !ok {
		t.Fatal("expected sidecar spec to resolve")
	}
	service.MarkRuntimeStarting(RuntimeKindWorker, "ocr_worker")
	service.MarkRuntimeHealthy(RuntimeKindWorker, "ocr_worker")
	service.MarkRuntimeFailed(RuntimeKindSidecar, "playwright_sidecar", assertError("transport lost"))
	service.MarkRuntimeUnavailable(RuntimeKindWorker, "media_worker", "binary missing")
	service.MarkRuntimeStopped(RuntimeKindWorker, "media_worker")

	runtime, ok := service.RuntimeState(RuntimeKindWorker, "ocr_worker")
	if !ok || runtime.Status != RuntimeStatusRunning || runtime.Health != RuntimeHealthHealthy {
		t.Fatalf("expected runtime state to reflect healthy worker, got %+v ok=%v", runtime, ok)
	}
	if runtime.Manifest == nil || runtime.Manifest.PluginID != "ocr" || runtime.Manifest.Source != "builtin" {
		t.Fatalf("expected runtime to expose manifest linkage, got %+v", runtime)
	}
	failedRuntime, ok := service.RuntimeState(RuntimeKindSidecar, "playwright_sidecar")
	if !ok || failedRuntime.Health != RuntimeHealthFailed || failedRuntime.LastError == "" {
		t.Fatalf("expected sidecar failure state, got %+v ok=%v", failedRuntime, ok)
	}
	ocrRuntime, ok := service.RuntimeState(RuntimeKindWorker, "ocr_worker")
	if !ok || ocrRuntime.Transport != "named_pipe" {
		t.Fatalf("expected ocr worker transport to reflect named pipe runtime, got %+v ok=%v", ocrRuntime, ok)
	}
	mediaRuntime, ok := service.RuntimeState(RuntimeKindWorker, "media_worker")
	if !ok || mediaRuntime.Transport != "named_pipe" {
		t.Fatalf("expected media worker transport to reflect named pipe runtime, got %+v ok=%v", mediaRuntime, ok)
	}
	metrics := service.MetricSnapshots()
	if len(metrics) == 0 {
		t.Fatal("expected metric snapshots to be available")
	}
	manifests := service.Manifests()
	if len(manifests) != 3 {
		t.Fatalf("expected one manifest per declared plugin, got %+v", manifests)
	}
	if manifests[0].Summary == "" || manifests[1].Summary == "" || manifests[2].Summary == "" {
		t.Fatalf("expected built-in manifests to expose summaries, got %+v", manifests)
	}
	if metrics[0].Name != "playwright_worker" || metrics[1].Name != "ocr_worker" || metrics[2].Name != "media_worker" {
		t.Fatalf("expected metric snapshots to follow declaration order, got %+v", metrics)
	}
	events := service.RuntimeEvents()
	if len(events) < 4 {
		t.Fatalf("expected runtime events to be buffered, got %+v", events)
	}
}

func TestServiceEventPayloadsAreCloned(t *testing.T) {
	service := NewService()
	service.MarkRuntimeUnavailable(RuntimeKindWorker, "ocr_worker", "binary missing")
	events := service.RuntimeEvents()
	events[0].Payload["error"] = "mutated"
	freshEvents := service.RuntimeEvents()
	if freshEvents[0].Payload["error"] != "binary missing" {
		t.Fatalf("expected runtime events to return cloned payloads, got %+v", freshEvents)
	}
}

func TestServiceRuntimeEventsStayBounded(t *testing.T) {
	service := NewService()
	for index := 0; index < maxRuntimeEvents+10; index++ {
		service.MarkRuntimeFailed(RuntimeKindWorker, "ocr_worker", testError("failure"))
	}
	events := service.RuntimeEvents()
	if len(events) != maxRuntimeEvents {
		t.Fatalf("expected runtime events to stay bounded at %d, got %d", maxRuntimeEvents, len(events))
	}
}

func assertError(message string) error { return testError(message) }

type testError string

func (e testError) Error() string { return string(e) }
