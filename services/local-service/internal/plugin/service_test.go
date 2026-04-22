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
	catalog := service.CatalogEntries()
	if len(catalog) != 3 || catalog[0].PluginID != "playwright" || catalog[1].PluginID != "ocr" || catalog[2].PluginID != "media" {
		t.Fatalf("expected stable builtin catalog order, got %+v", catalog)
	}
	if len(catalog[0].RuntimeRefs) != 2 || catalog[1].DisplayName != "OCR Worker" {
		t.Fatalf("expected catalog to join static metadata and runtime refs, got %+v", catalog)
	}
	ocrSnapshot, ok := service.CatalogSnapshot("ocr")
	if !ok {
		t.Fatal("expected OCR catalog snapshot to resolve")
	}
	if ocrSnapshot.Manifest.PluginID != "ocr" || len(ocrSnapshot.Runtimes) != 1 || len(ocrSnapshot.Metrics) != 1 {
		t.Fatalf("expected catalog snapshot to join manifest/runtime/metrics, got %+v", ocrSnapshot)
	}
	if len(ocrSnapshot.RecentEvents) == 0 || ocrSnapshot.RecentEvents[0].Name != "ocr_worker" {
		t.Fatalf("expected catalog snapshot to include runtime events for matching plugin, got %+v", ocrSnapshot)
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

func TestCatalogEntriesAndSnapshotsAreCloned(t *testing.T) {
	service := NewService()
	entries := service.CatalogEntries()
	entries[0].DisplayName = "mutated"
	entries[0].RuntimeRefs[0].Name = "mutated_runtime"
	freshEntries := service.CatalogEntries()
	if freshEntries[0].DisplayName == "mutated" || freshEntries[0].RuntimeRefs[0].Name == "mutated_runtime" {
		t.Fatalf("expected catalog entries to be cloned, got %+v", freshEntries)
	}
	snapshot, ok := service.CatalogSnapshot("playwright")
	if !ok {
		t.Fatal("expected playwright catalog snapshot")
	}
	snapshot.Catalog.DisplayName = "mutated"
	snapshot.Manifest.Name = "mutated"
	snapshot.Runtimes[0].Name = "mutated"
	freshSnapshot, ok := service.CatalogSnapshot("playwright")
	if !ok {
		t.Fatal("expected fresh playwright catalog snapshot")
	}
	if freshSnapshot.Catalog.DisplayName == "mutated" || freshSnapshot.Manifest.Name == "mutated" || freshSnapshot.Runtimes[0].Name == "mutated" {
		t.Fatalf("expected catalog snapshots to be cloned, got %+v", freshSnapshot)
	}
}

func TestNilServiceCatalogSnapshotsStillExposeBuiltinStaticView(t *testing.T) {
	var service *Service
	entries := service.CatalogEntries()
	if len(entries) != 3 || entries[0].PluginID != "playwright" {
		t.Fatalf("expected nil service to expose builtin catalog entries, got %+v", entries)
	}
	snapshot, ok := service.CatalogSnapshot("ocr")
	if !ok || snapshot.Manifest.PluginID != "ocr" || len(snapshot.Runtimes) != 0 {
		t.Fatalf("expected nil service catalog snapshot to expose static manifest only, got snapshot=%+v ok=%v", snapshot, ok)
	}
}

func TestCatalogEntriesIncludeNonBuiltinPluginRuntimeSources(t *testing.T) {
	service := NewService()
	extraManifest := Manifest{
		PluginID:     "community_skill_runtime",
		Name:         "Community Skill Runtime",
		Summary:      "Loads verified community skills through a managed runtime.",
		Version:      "github-v1",
		Entry:        "github://community-skill/runtime",
		Source:       "github",
		Capabilities: []string{"community_skill_run"},
		Permissions:  []string{"workspace_read"},
	}
	service.declareRuntime(RuntimeState{
		Name:         "community_skill_worker",
		Kind:         RuntimeKindWorker,
		Status:       RuntimeStatusDeclared,
		Transport:    "named_pipe",
		Health:       RuntimeHealthUnknown,
		Manifest:     &extraManifest,
		Capabilities: []string{"community_skill_run"},
	})
	service.MarkRuntimeHealthy(RuntimeKindWorker, "community_skill_worker")

	entries := service.CatalogEntries()
	if len(entries) != 4 {
		t.Fatalf("expected dynamic plugin runtime to extend catalog entries, got %+v", entries)
	}
	extraEntry := entries[3]
	if extraEntry.PluginID != "community_skill_runtime" || extraEntry.DisplayName != "Community Skill Runtime" || len(extraEntry.RuntimeRefs) != 1 {
		t.Fatalf("expected dynamic plugin runtime entry to be visible in catalog, got %+v", extraEntry)
	}
	snapshot, ok := service.CatalogSnapshot("community_skill_runtime")
	if !ok {
		t.Fatal("expected dynamic plugin runtime snapshot to resolve")
	}
	if snapshot.Manifest.PluginID != "community_skill_runtime" || len(snapshot.Runtimes) != 1 || len(snapshot.Metrics) != 1 {
		t.Fatalf("expected dynamic plugin runtime snapshot to join runtime state, got %+v", snapshot)
	}
	if len(snapshot.RecentEvents) == 0 || snapshot.RecentEvents[0].Name != "community_skill_worker" {
		t.Fatalf("expected dynamic plugin runtime snapshot to include runtime events, got %+v", snapshot)
	}
}

func assertError(message string) error { return testError(message) }

type testError string

func (e testError) Error() string { return string(e) }
