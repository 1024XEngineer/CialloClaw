package plugin

import "strings"

// RuntimeRef keeps one static runtime declaration attached to the plugin catalog
// entry so later query layers can join static assets and live runtime caches
// without duplicating builtin wiring tables.
type RuntimeRef struct {
	Name      string
	Kind      RuntimeKind
	Transport string
}

// CatalogEntry is the owner-5 plugin read model that combines stable manifest
// metadata with the declared runtime edges consumed by dashboard/detail views.
type CatalogEntry struct {
	PluginID     string
	Name         string
	DisplayName  string
	Summary      string
	Version      string
	Source       string
	Entry        string
	Enabled      bool
	Capabilities []string
	Permissions  []string
	RuntimeRefs  []RuntimeRef
}

// CatalogSnapshot joins one static catalog entry with current runtime, metric,
// and event state so later consumers can query one backend-owned read model.
type CatalogSnapshot struct {
	Catalog      CatalogEntry
	Manifest     Manifest
	Runtimes     []RuntimeState
	Metrics      []MetricSnapshot
	RecentEvents []RuntimeEvent
}

// BuiltinCatalogEntries returns the stable builtin plugin catalog in declaration order.
func BuiltinCatalogEntries() []CatalogEntry {
	items := builtinCatalogEntries()
	result := make([]CatalogEntry, 0, len(items))
	for _, item := range items {
		result = append(result, cloneCatalogEntry(item))
	}
	return result
}

// CatalogEntries returns the backend-owned plugin read model with live runtime
// refs reattached to the current manifest catalog.
func (s *Service) CatalogEntries() []CatalogEntry {
	entries := BuiltinCatalogEntries()
	if s == nil {
		return entries
	}
	entryIndex := make(map[string]int, len(entries))
	for index := range entries {
		entryIndex[strings.TrimSpace(entries[index].PluginID)] = index
	}
	manifestIndex := make(map[string]Manifest)
	for _, manifest := range s.Manifests() {
		manifestIndex[strings.TrimSpace(manifest.PluginID)] = manifest
	}
	runtimeRefsByPluginID := make(map[string][]RuntimeRef)
	for _, runtime := range s.RuntimeStates() {
		if runtime.Manifest == nil || strings.TrimSpace(runtime.Manifest.PluginID) == "" {
			continue
		}
		pluginID := strings.TrimSpace(runtime.Manifest.PluginID)
		runtimeRefsByPluginID[pluginID] = append(runtimeRefsByPluginID[pluginID], RuntimeRef{
			Name:      runtime.Name,
			Kind:      runtime.Kind,
			Transport: runtime.Transport,
		})
	}
	for index := range entries {
		if manifest, ok := manifestIndex[entries[index].PluginID]; ok {
			entries[index].DisplayName = firstNonEmptyCatalog(strings.TrimSpace(manifest.Name), entries[index].DisplayName)
			entries[index].Summary = firstNonEmptyCatalog(strings.TrimSpace(manifest.Summary), entries[index].Summary)
			entries[index].Version = firstNonEmptyCatalog(strings.TrimSpace(manifest.Version), entries[index].Version)
			entries[index].Source = firstNonEmptyCatalog(strings.TrimSpace(manifest.Source), entries[index].Source)
			entries[index].Entry = firstNonEmptyCatalog(strings.TrimSpace(manifest.Entry), entries[index].Entry)
			entries[index].Capabilities = append([]string(nil), manifest.Capabilities...)
			entries[index].Permissions = append([]string(nil), manifest.Permissions...)
		}
		if refs, ok := runtimeRefsByPluginID[entries[index].PluginID]; ok {
			entries[index].RuntimeRefs = cloneRuntimeRefs(refs)
			entries[index].Enabled = len(refs) > 0
		}
	}
	for _, pluginID := range appendMissingCatalogPluginIDs(s, entryIndex, manifestIndex, runtimeRefsByPluginID) {
		entry := catalogEntryFromDynamicSources(pluginID, manifestIndex[pluginID], runtimeRefsByPluginID[pluginID])
		entries = append(entries, entry)
		entryIndex[pluginID] = len(entries) - 1
	}
	return entries
}

// CatalogSnapshots returns one joined read model per plugin for downstream task
// detail/debug consumers without forcing them to manually correlate caches.
func (s *Service) CatalogSnapshots() []CatalogSnapshot {
	entries := s.CatalogEntries()
	if len(entries) == 0 {
		return nil
	}
	if s == nil {
		result := make([]CatalogSnapshot, 0, len(entries))
		for _, entry := range entries {
			result = append(result, CatalogSnapshot{Catalog: cloneCatalogEntry(entry), Manifest: manifestFromCatalogEntry(entry)})
		}
		return result
	}
	manifestIndex := make(map[string]Manifest)
	for _, manifest := range s.Manifests() {
		manifestIndex[strings.TrimSpace(manifest.PluginID)] = manifest
	}
	runtimeIndex := make(map[string]RuntimeState)
	for _, runtime := range s.RuntimeStates() {
		runtimeIndex[runtimeKey(runtime.Kind, runtime.Name)] = runtime
	}
	metricIndex := make(map[string]MetricSnapshot)
	for _, metric := range s.MetricSnapshots() {
		metricIndex[runtimeKey(metric.Kind, metric.Name)] = metric
	}
	events := s.RuntimeEvents()
	result := make([]CatalogSnapshot, 0, len(entries))
	for _, entry := range entries {
		snapshot := CatalogSnapshot{
			Catalog:  cloneCatalogEntry(entry),
			Manifest: manifestFromCatalogEntry(entry),
		}
		if manifest, ok := manifestIndex[entry.PluginID]; ok {
			snapshot.Manifest = manifest
		}
		for _, ref := range entry.RuntimeRefs {
			if runtime, ok := runtimeIndex[runtimeKey(ref.Kind, ref.Name)]; ok {
				snapshot.Runtimes = append(snapshot.Runtimes, runtime)
			}
			if metric, ok := metricIndex[runtimeKey(ref.Kind, ref.Name)]; ok {
				snapshot.Metrics = append(snapshot.Metrics, metric)
			}
		}
		snapshot.RecentEvents = filterRuntimeEventsForRefs(events, entry.RuntimeRefs)
		result = append(result, snapshot)
	}
	return result
}

// CatalogSnapshot returns one joined plugin read model by plugin_id.
func (s *Service) CatalogSnapshot(pluginID string) (CatalogSnapshot, bool) {
	needle := strings.TrimSpace(pluginID)
	if needle == "" {
		return CatalogSnapshot{}, false
	}
	for _, snapshot := range s.CatalogSnapshots() {
		if snapshot.Catalog.PluginID == needle {
			return cloneCatalogSnapshot(snapshot), true
		}
	}
	return CatalogSnapshot{}, false
}

func builtinCatalogEntries() []CatalogEntry {
	return []CatalogEntry{
		{
			PluginID:     "playwright",
			Name:         "playwright",
			DisplayName:  "Playwright Browser Automation",
			Summary:      "Read, search, and interact with web pages through the controlled Playwright runtime.",
			Version:      "builtin-v1",
			Source:       "builtin",
			Entry:        "builtin://plugin/playwright",
			Enabled:      true,
			Capabilities: []string{"page_read", "page_search", "page_interact", "structured_dom"},
			Permissions:  []string{"webpage_read", "webpage_interact"},
			RuntimeRefs: []RuntimeRef{
				{Name: "playwright_worker", Kind: RuntimeKindWorker, Transport: "worker_process"},
				{Name: "playwright_sidecar", Kind: RuntimeKindSidecar, Transport: "named_pipe"},
			},
		},
		{
			PluginID:     "ocr",
			Name:         "ocr",
			DisplayName:  "OCR Worker",
			Summary:      "Extract text from files, images, and PDFs through the managed OCR worker.",
			Version:      "builtin-v1",
			Source:       "builtin",
			Entry:        "builtin://plugin/ocr",
			Enabled:      true,
			Capabilities: []string{"extract_text", "ocr_image", "ocr_pdf"},
			Permissions:  []string{"workspace_read", "artifact_read"},
			RuntimeRefs: []RuntimeRef{
				{Name: "ocr_worker", Kind: RuntimeKindWorker, Transport: "named_pipe"},
			},
		},
		{
			PluginID:     "media",
			Name:         "media",
			DisplayName:  "Media Worker",
			Summary:      "Normalize recordings, transcode media, and extract representative frames.",
			Version:      "builtin-v1",
			Source:       "builtin",
			Entry:        "builtin://plugin/media",
			Enabled:      true,
			Capabilities: []string{"transcode_media", "normalize_recording", "extract_frames"},
			Permissions:  []string{"workspace_read", "workspace_write", "artifact_write"},
			RuntimeRefs: []RuntimeRef{
				{Name: "media_worker", Kind: RuntimeKindWorker, Transport: "named_pipe"},
			},
		},
	}
}

func appendMissingCatalogPluginIDs(service *Service, entryIndex map[string]int, manifestIndex map[string]Manifest, runtimeRefsByPluginID map[string][]RuntimeRef) []string {
	if service == nil {
		return nil
	}
	ordered := make([]string, 0)
	appendIfMissing := func(pluginID string) {
		trimmed := strings.TrimSpace(pluginID)
		if trimmed == "" {
			return
		}
		if _, exists := entryIndex[trimmed]; exists {
			return
		}
		for _, existing := range ordered {
			if existing == trimmed {
				return
			}
		}
		ordered = append(ordered, trimmed)
	}
	for _, manifest := range service.Manifests() {
		appendIfMissing(manifest.PluginID)
	}
	for _, runtime := range service.RuntimeStates() {
		if runtime.Manifest != nil {
			appendIfMissing(runtime.Manifest.PluginID)
		}
	}
	for pluginID := range manifestIndex {
		appendIfMissing(pluginID)
	}
	for pluginID := range runtimeRefsByPluginID {
		appendIfMissing(pluginID)
	}
	return ordered
}

func catalogEntryFromDynamicSources(pluginID string, manifest Manifest, runtimeRefs []RuntimeRef) CatalogEntry {
	trimmedPluginID := strings.TrimSpace(pluginID)
	entry := CatalogEntry{
		PluginID:     trimmedPluginID,
		Name:         firstNonEmptyCatalog(strings.TrimSpace(manifest.PluginID), trimmedPluginID),
		DisplayName:  firstNonEmptyCatalog(strings.TrimSpace(manifest.Name), trimmedPluginID),
		Summary:      strings.TrimSpace(manifest.Summary),
		Version:      firstNonEmptyCatalog(strings.TrimSpace(manifest.Version), "runtime-unversioned"),
		Source:       firstNonEmptyCatalog(strings.TrimSpace(manifest.Source), "runtime"),
		Entry:        strings.TrimSpace(manifest.Entry),
		Enabled:      len(runtimeRefs) > 0,
		Capabilities: append([]string(nil), manifest.Capabilities...),
		Permissions:  append([]string(nil), manifest.Permissions...),
		RuntimeRefs:  cloneRuntimeRefs(runtimeRefs),
	}
	if entry.Name == "" {
		entry.Name = trimmedPluginID
	}
	if entry.DisplayName == "" {
		entry.DisplayName = trimmedPluginID
	}
	return entry
}

func manifestFromCatalogEntry(entry CatalogEntry) Manifest {
	return Manifest{
		PluginID:     entry.PluginID,
		Name:         firstNonEmptyCatalog(entry.DisplayName, entry.Name),
		Summary:      entry.Summary,
		Version:      entry.Version,
		Entry:        entry.Entry,
		Source:       entry.Source,
		Capabilities: append([]string(nil), entry.Capabilities...),
		Permissions:  append([]string(nil), entry.Permissions...),
	}
}

func filterRuntimeEventsForRefs(events []RuntimeEvent, refs []RuntimeRef) []RuntimeEvent {
	if len(events) == 0 || len(refs) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		allowed[runtimeKey(ref.Kind, ref.Name)] = struct{}{}
	}
	result := make([]RuntimeEvent, 0, len(events))
	for _, event := range events {
		if _, ok := allowed[runtimeKey(event.Kind, event.Name)]; ok {
			result = append(result, cloneRuntimeEvent(event))
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func cloneCatalogEntry(entry CatalogEntry) CatalogEntry {
	return CatalogEntry{
		PluginID:     entry.PluginID,
		Name:         entry.Name,
		DisplayName:  entry.DisplayName,
		Summary:      entry.Summary,
		Version:      entry.Version,
		Source:       entry.Source,
		Entry:        entry.Entry,
		Enabled:      entry.Enabled,
		Capabilities: append([]string(nil), entry.Capabilities...),
		Permissions:  append([]string(nil), entry.Permissions...),
		RuntimeRefs:  cloneRuntimeRefs(entry.RuntimeRefs),
	}
}

func cloneCatalogSnapshot(snapshot CatalogSnapshot) CatalogSnapshot {
	return CatalogSnapshot{
		Catalog:      cloneCatalogEntry(snapshot.Catalog),
		Manifest:     cloneManifest(&snapshot.Manifest),
		Runtimes:     cloneRuntimeStates(snapshot.Runtimes),
		Metrics:      cloneMetricSnapshots(snapshot.Metrics),
		RecentEvents: cloneRuntimeEvents(snapshot.RecentEvents),
	}
}

func cloneRuntimeRefs(items []RuntimeRef) []RuntimeRef {
	if len(items) == 0 {
		return nil
	}
	result := make([]RuntimeRef, 0, len(items))
	for _, item := range items {
		result = append(result, RuntimeRef{Name: item.Name, Kind: item.Kind, Transport: item.Transport})
	}
	return result
}

func cloneRuntimeStates(items []RuntimeState) []RuntimeState {
	if len(items) == 0 {
		return nil
	}
	result := make([]RuntimeState, 0, len(items))
	for _, item := range items {
		result = append(result, cloneRuntimeState(item))
	}
	return result
}

func cloneMetricSnapshots(items []MetricSnapshot) []MetricSnapshot {
	if len(items) == 0 {
		return nil
	}
	result := make([]MetricSnapshot, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	return result
}

func cloneRuntimeEvents(items []RuntimeEvent) []RuntimeEvent {
	if len(items) == 0 {
		return nil
	}
	result := make([]RuntimeEvent, 0, len(items))
	for _, item := range items {
		result = append(result, cloneRuntimeEvent(item))
	}
	return result
}

func cloneRuntimeEvent(item RuntimeEvent) RuntimeEvent {
	return RuntimeEvent{
		Name:      item.Name,
		Kind:      item.Kind,
		EventType: item.EventType,
		Payload:   cloneMap(item.Payload),
		CreatedAt: item.CreatedAt,
	}
}

func firstNonEmptyCatalog(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
