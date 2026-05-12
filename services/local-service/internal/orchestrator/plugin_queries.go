package orchestrator

import (
	"errors"
	"sort"
	"strings"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

type pluginCapabilitySummary struct {
	ToolName    string
	DisplayName string
	Description string
	Source      string
	RiskHint    string
}

type pluginContractField struct {
	Name        string
	Type        string
	Required    bool
	Description string
	Example     string
}

type pluginDataContract struct {
	SchemaRef  string
	SchemaJSON map[string]any
	Fields     []pluginContractField
}

type pluginDeliveryMapping struct {
	EmitsToolCall       bool
	ArtifactTypes       []string
	DeliveryTypes       []string
	CitationSourceTypes []string
}

type pluginToolContract struct {
	ToolName       string
	DisplayName    string
	Description    string
	Source         string
	RiskHint       string
	TimeoutSec     int
	SupportsDryRun bool
	InputContract  pluginDataContract
	OutputContract pluginDataContract
	DeliveryMap    pluginDeliveryMapping
}

// PluginList returns the task-adjacent plugin catalog view documented by the
// protocol without exposing raw runtime caches as the primary list object.
func (s *Service) PluginList(params map[string]any) (map[string]any, error) {
	query := strings.TrimSpace(stringValue(params, "query", ""))
	pageParams := mapValue(params, "page")
	limit := clampListLimit(intValue(pageParams, "limit", 20))
	offset := clampListOffset(intValue(pageParams, "offset", 0))
	kinds, err := normalizePluginFilterValues(params["kinds"], validPluginRuntimeKind)
	if err != nil {
		return nil, err
	}
	health, err := normalizePluginFilterValues(params["health"], validPluginHealth)
	if err != nil {
		return nil, err
	}

	runtimeIndex := pluginRuntimeIndex(s.plugin)
	toolIndex := pluginToolMetadataIndex(s.tools)
	items := make([]map[string]any, 0)
	for _, entry := range pluginCatalogEntries(s.plugin) {
		runtimes := pluginRuntimesForCatalogEntry(entry, runtimeIndex)
		if !matchesPluginListQuery(entry, runtimes, query, kinds, health) {
			continue
		}
		items = append(items, pluginListItem(entry, runtimes, toolIndex))
	}

	total := len(items)
	if offset >= total {
		return map[string]any{
			"items": []map[string]any{},
			"page":  pageMap(limit, offset, total),
		}, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return map[string]any{
		"items": items[offset:end],
		"page":  pageMap(limit, offset, total),
	}, nil
}

// PluginDetailGet exposes one plugin-centric detail payload with optional
// runtime, metric, and event slices so later UI work does not need to infer the
// plugin catalog from worker declarations.
func (s *Service) PluginDetailGet(params map[string]any) (map[string]any, error) {
	pluginID := strings.TrimSpace(stringValue(params, "plugin_id", ""))
	if pluginID == "" {
		return nil, errors.New("plugin_id is required")
	}
	snapshot, ok := pluginCatalogSnapshot(s.plugin, pluginID)
	if !ok {
		return nil, errors.New("plugin_id is invalid")
	}

	includeRuntime := boolValue(params, "include_runtime", true)
	includeMetrics := boolValue(params, "include_metrics", true)
	includeEvents := boolValue(params, "include_events", true)
	toolIndex := pluginToolMetadataIndex(s.tools)

	result := map[string]any{
		"plugin":        pluginManifestItemFromToolIndex(snapshot.Catalog, pluginToolMetadataForCatalogEntry(snapshot.Catalog, snapshot.Runtimes, toolIndex)),
		"runtimes":      []map[string]any{},
		"metrics":       []map[string]any{},
		"recent_events": []map[string]any{},
		"tools":         pluginToolItems(pluginToolContractsForCatalogEntry(snapshot.Catalog, snapshot.Runtimes, s.tools)),
	}
	if includeRuntime {
		result["runtimes"] = pluginRuntimeItems(snapshot.Runtimes)
	}
	if includeMetrics {
		result["metrics"] = pluginMetricItems(snapshot.Metrics)
	}
	if includeEvents {
		result["recent_events"] = pluginEventItems(snapshot.RecentEvents)
	}
	return result, nil
}

func pluginCatalogEntries(service *plugin.Service) []plugin.CatalogEntry {
	if service == nil {
		return plugin.BuiltinCatalogEntries()
	}
	return service.CatalogEntries()
}

func pluginCatalogSnapshot(service *plugin.Service, pluginID string) (plugin.CatalogSnapshot, bool) {
	if service == nil {
		var nilService *plugin.Service
		return nilService.CatalogSnapshot(pluginID)
	}
	return service.CatalogSnapshot(pluginID)
}

func pluginCatalogSnapshots(service *plugin.Service) []plugin.CatalogSnapshot {
	if service == nil {
		var nilService *plugin.Service
		return nilService.CatalogSnapshots()
	}
	return service.CatalogSnapshots()
}

func pluginSnapshotRuntimes(items []plugin.CatalogSnapshot) []plugin.RuntimeState {
	result := make([]plugin.RuntimeState, 0)
	for _, item := range items {
		result = append(result, item.Runtimes...)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func pluginSnapshotMetrics(items []plugin.CatalogSnapshot) []plugin.MetricSnapshot {
	result := make([]plugin.MetricSnapshot, 0)
	for _, item := range items {
		result = append(result, item.Metrics...)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func pluginSnapshotEvents(items []plugin.CatalogSnapshot) []plugin.RuntimeEvent {
	result := make([]plugin.RuntimeEvent, 0)
	for _, item := range items {
		result = append(result, item.RecentEvents...)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func pluginRuntimeIndex(service *plugin.Service) map[string]plugin.RuntimeState {
	if service == nil {
		return map[string]plugin.RuntimeState{}
	}
	index := make(map[string]plugin.RuntimeState)
	for _, item := range service.RuntimeStates() {
		index[pluginRefKey(item.Kind, item.Name)] = item
	}
	return index
}

func pluginRuntimesForCatalogEntry(entry plugin.CatalogEntry, runtimeIndex map[string]plugin.RuntimeState) []plugin.RuntimeState {
	result := make([]plugin.RuntimeState, 0, len(entry.RuntimeRefs))
	for _, ref := range entry.RuntimeRefs {
		if runtime, ok := runtimeIndex[pluginRefKey(ref.Kind, ref.Name)]; ok {
			result = append(result, runtime)
		}
	}
	return result
}

func matchesPluginListQuery(entry plugin.CatalogEntry, runtimes []plugin.RuntimeState, query string, kinds []string, health []string) bool {
	if query != "" {
		haystack := strings.ToLower(strings.Join([]string{entry.PluginID, entry.Name, entry.DisplayName, entry.Summary}, " "))
		if !strings.Contains(haystack, strings.ToLower(query)) {
			return false
		}
	}
	if len(kinds) > 0 {
		foundKind := false
		for _, runtime := range runtimes {
			if containsPluginFilterValue(kinds, string(runtime.Kind)) {
				foundKind = true
				break
			}
		}
		if !foundKind {
			return false
		}
	}
	if len(health) > 0 {
		foundHealth := false
		for _, runtime := range runtimes {
			if containsPluginFilterValue(health, string(runtime.Health)) {
				foundHealth = true
				break
			}
		}
		if !foundHealth {
			return false
		}
	}
	return true
}

func normalizePluginFilterValues(value any, validate func(string) bool) ([]string, error) {
	if value == nil {
		return nil, nil
	}
	items, ok := normalizeStringSlice(value)
	if !ok {
		return nil, errors.New("plugin filter values must be string arrays")
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		normalized := strings.TrimSpace(item)
		if normalized == "" || !validate(normalized) {
			return nil, errors.New("plugin filter values contain unsupported entries")
		}
		result = append(result, normalized)
	}
	return result, nil
}

func validPluginRuntimeKind(value string) bool {
	switch value {
	case string(plugin.RuntimeKindWorker), string(plugin.RuntimeKindSidecar):
		return true
	default:
		return false
	}
}

func validPluginHealth(value string) bool {
	switch value {
	case string(plugin.RuntimeHealthUnknown), string(plugin.RuntimeHealthHealthy), string(plugin.RuntimeHealthDegraded), string(plugin.RuntimeHealthFailed), string(plugin.RuntimeHealthStopped), string(plugin.RuntimeHealthUnavailable):
		return true
	default:
		return false
	}
}

func containsPluginFilterValue(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func pluginRefKey(kind plugin.RuntimeKind, name string) string {
	return string(kind) + "::" + strings.TrimSpace(name)
}

func pluginListItem(entry plugin.CatalogEntry, runtimes []plugin.RuntimeState, toolIndex map[string]tools.ToolMetadata) map[string]any {
	result := pluginManifestItemFromToolIndex(entry, pluginToolMetadataForCatalogEntry(entry, runtimes, toolIndex))
	result["runtimes"] = pluginRuntimeItems(runtimes)
	return result
}

func pluginManifestItemFromToolIndex(entry plugin.CatalogEntry, metadata []tools.ToolMetadata) map[string]any {
	return map[string]any{
		"plugin_id":    entry.PluginID,
		"name":         entry.Name,
		"display_name": entry.DisplayName,
		"summary":      entry.Summary,
		"version":      entry.Version,
		"source":       entry.Source,
		"entry":        entry.Entry,
		"enabled":      entry.Enabled,
		"permissions":  append([]string(nil), entry.Permissions...),
		"capabilities": pluginCapabilityItems(pluginCapabilitySummaries(metadata)),
	}
}

func pluginToolMetadataIndex(registry *tools.Registry) map[string]tools.ToolMetadata {
	if registry == nil {
		return map[string]tools.ToolMetadata{}
	}
	index := make(map[string]tools.ToolMetadata)
	for _, item := range registry.List() {
		index[item.Name] = item
	}
	return index
}

// pluginToolMetadataForEntry resolves one plugin's declared runtime capabilities
// back to the registered tool metadata so query surfaces stay aligned with the
// real execution registry.
func pluginToolMetadataForCatalogEntry(entry plugin.CatalogEntry, runtimes []plugin.RuntimeState, toolIndex map[string]tools.ToolMetadata) []tools.ToolMetadata {
	names := make(map[string]struct{})
	for _, runtime := range runtimes {
		for _, capability := range runtime.Capabilities {
			name := strings.TrimSpace(capability)
			if name != "" {
				names[name] = struct{}{}
			}
		}
	}
	if len(names) == 0 {
		for _, capability := range entry.Capabilities {
			name := strings.TrimSpace(capability)
			if name != "" {
				names[name] = struct{}{}
			}
		}
	}

	result := make([]tools.ToolMetadata, 0, len(names))
	for name := range names {
		if metadata, ok := toolIndex[name]; ok {
			result = append(result, metadata)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func pluginCapabilitySummaries(items []tools.ToolMetadata) []pluginCapabilitySummary {
	result := make([]pluginCapabilitySummary, 0, len(items))
	for _, item := range items {
		result = append(result, pluginCapabilitySummary{
			ToolName:    item.Name,
			DisplayName: item.DisplayName,
			Description: item.Description,
			Source:      string(item.Source),
			RiskHint:    item.RiskHint,
		})
	}
	return result
}

func pluginCapabilityItems(items []pluginCapabilitySummary) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"tool_name":    item.ToolName,
			"display_name": item.DisplayName,
			"description":  item.Description,
			"source":       item.Source,
			"risk_hint":    item.RiskHint,
		})
	}
	return result
}

func pluginToolContractsForCatalogEntry(entry plugin.CatalogEntry, runtimes []plugin.RuntimeState, registry *tools.Registry) []pluginToolContract {
	metadata := pluginToolMetadataForCatalogEntry(entry, runtimes, pluginToolMetadataIndex(registry))
	result := make([]pluginToolContract, 0, len(metadata))
	for _, item := range metadata {
		result = append(result, pluginToolContract{
			ToolName:       item.Name,
			DisplayName:    item.DisplayName,
			Description:    item.Description,
			Source:         string(item.Source),
			RiskHint:       item.RiskHint,
			TimeoutSec:     item.TimeoutSec,
			SupportsDryRun: item.SupportsDryRun,
			InputContract:  pluginSchemaRefContract(item.InputSchemaRef),
			OutputContract: pluginSchemaRefContract(item.OutputSchemaRef),
			DeliveryMap:    pluginDeliveryMappingForMetadata(item),
		})
	}
	return result
}

func pluginSchemaRefContract(schemaRef string) pluginDataContract {
	return pluginDataContract{
		SchemaRef:  strings.TrimSpace(schemaRef),
		SchemaJSON: nil,
		Fields:     nil,
	}
}

// pluginDeliveryMappingForMetadata keeps the detail query payload honest about
// the shared task/tool chain while limiting any non-registry assumptions to the
// delivery surface that is not modeled in ToolMetadata yet.
func pluginDeliveryMappingForMetadata(metadata tools.ToolMetadata) pluginDeliveryMapping {
	mapping := pluginDeliveryMapping{
		EmitsToolCall:       true,
		ArtifactTypes:       []string{},
		DeliveryTypes:       []string{"task_detail"},
		CitationSourceTypes: []string{},
	}
	switch metadata.Name {
	case "page_read", "page_search", "web_search", "page_interact", "structured_dom", "browser_attach_current", "browser_snapshot", "browser_navigate", "browser_tabs_list", "browser_tab_focus", "browser_interact":
		mapping.CitationSourceTypes = []string{"web"}
	case "extract_text", "ocr_image", "ocr_pdf":
		mapping.CitationSourceTypes = []string{"file"}
	case "transcode_media", "normalize_recording":
		mapping.ArtifactTypes = []string{"generated_file"}
		mapping.CitationSourceTypes = []string{"file"}
	case "extract_frames":
		mapping.ArtifactTypes = []string{"image"}
		mapping.CitationSourceTypes = []string{"file"}
	}
	return mapping
}

func pluginToolItems(items []pluginToolContract) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{
			"tool_name":        item.ToolName,
			"display_name":     item.DisplayName,
			"description":      item.Description,
			"source":           item.Source,
			"risk_hint":        item.RiskHint,
			"timeout_sec":      item.TimeoutSec,
			"supports_dry_run": item.SupportsDryRun,
			"input_contract":   pluginDataContractItem(item.InputContract),
			"output_contract":  pluginDataContractItem(item.OutputContract),
			"delivery_mapping": pluginDeliveryMappingItem(item.DeliveryMap),
		})
	}
	return result
}

func pluginDataContractItem(contract pluginDataContract) map[string]any {
	return map[string]any{
		"schema_ref":  contract.SchemaRef,
		"schema_json": cloneMap(contract.SchemaJSON),
		"fields":      pluginContractFieldItems(contract.Fields),
	}
}

func pluginContractFieldItems(items []pluginContractField) []map[string]any {
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		field := map[string]any{
			"name":        item.Name,
			"type":        item.Type,
			"required":    item.Required,
			"description": item.Description,
		}
		if strings.TrimSpace(item.Example) != "" {
			field["example"] = item.Example
		}
		result = append(result, field)
	}
	return result
}

func pluginDeliveryMappingItem(mapping pluginDeliveryMapping) map[string]any {
	return map[string]any{
		"emits_tool_call":       mapping.EmitsToolCall,
		"artifact_types":        append([]string(nil), mapping.ArtifactTypes...),
		"delivery_types":        append([]string(nil), mapping.DeliveryTypes...),
		"citation_source_types": append([]string(nil), mapping.CitationSourceTypes...),
	}
}
