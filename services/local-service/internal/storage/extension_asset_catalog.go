package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	// ExtensionAssetKindSkillManifest marks one skill_manifests usage reference.
	ExtensionAssetKindSkillManifest = "skill_manifest"
	// ExtensionAssetKindBlueprintDefinition marks one blueprint_definitions usage reference.
	ExtensionAssetKindBlueprintDefinition = "blueprint_definition"
	// ExtensionAssetKindPromptTemplateVersion marks one prompt_template_versions usage reference.
	ExtensionAssetKindPromptTemplateVersion = "prompt_template_version"
	// ExtensionAssetKindPluginManifest marks one plugin_manifests usage reference.
	ExtensionAssetKindPluginManifest = "plugin_manifest"
	// ExtensionAssetKindModelProviderRoute marks one attributed model-provider route.
	ExtensionAssetKindModelProviderRoute = "model_provider_route"
	// ExtensionAssetKindPerceptionPackage marks one attributed perception-package boundary.
	ExtensionAssetKindPerceptionPackage = "perception_package"
)

// ExtensionAssetCatalog is the smallest owner-5 boundary that execution can use
// to attribute versioned config assets and plugin manifests to one task/run.
type ExtensionAssetCatalog interface {
	CurrentExecutionAssets(ctx context.Context) ([]ExtensionAssetReference, error)
	PluginAssetsForCapabilities(ctx context.Context, capabilities []string) ([]ExtensionAssetReference, error)
}

type skillManifestSourceLookup interface {
	latestSkillManifestBySource(ctx context.Context, source string) (SkillManifestRecord, bool, error)
}

type blueprintDefinitionSourceLookup interface {
	latestBlueprintDefinitionBySource(ctx context.Context, source string) (BlueprintDefinitionRecord, bool, error)
}

type promptTemplateVersionSourceLookup interface {
	latestPromptTemplateVersionBySource(ctx context.Context, source string) (PromptTemplateVersionRecord, bool, error)
}

// CurrentExecutionAssets returns the current built-in skill/blueprint/prompt
// asset selection that the main execution path should attribute to one run.
func (s *Service) CurrentExecutionAssets(ctx context.Context) ([]ExtensionAssetReference, error) {
	if s == nil {
		return nil, nil
	}
	refs := make([]ExtensionAssetReference, 0, 3)
	if ref, ok, err := latestSkillManifestRef(ctx, s.skillManifestStore); err != nil {
		return nil, err
	} else if ok {
		refs = append(refs, ref)
	}
	if ref, ok, err := latestBlueprintDefinitionRef(ctx, s.blueprintDefinitionStore); err != nil {
		return nil, err
	} else if ok {
		refs = append(refs, ref)
	}
	if ref, ok, err := latestPromptTemplateVersionRef(ctx, s.promptTemplateStore); err != nil {
		return nil, err
	} else if ok {
		refs = append(refs, ref)
	}
	return NormalizeExtensionAssetReferences(refs), nil
}

// PluginAssetsForCapabilities resolves plugin manifest references for the given
// capability names without turning on install/marketplace flows.
func (s *Service) PluginAssetsForCapabilities(ctx context.Context, capabilities []string) ([]ExtensionAssetReference, error) {
	if s == nil || s.pluginManifestStore == nil || len(capabilities) == 0 {
		return nil, nil
	}
	items, _, err := s.pluginManifestStore.ListPluginManifests(ctx, 0, 0)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	capabilitySet := make(map[string]struct{}, len(capabilities))
	for _, capability := range capabilities {
		trimmed := strings.TrimSpace(capability)
		if trimmed == "" {
			continue
		}
		capabilitySet[trimmed] = struct{}{}
	}
	if len(capabilitySet) == 0 {
		return nil, nil
	}
	refs := make([]ExtensionAssetReference, 0)
	for _, item := range items {
		pluginCapabilities := decodeStringList(item.CapabilitiesJSON)
		matched := false
		for _, capability := range pluginCapabilities {
			if _, ok := capabilitySet[capability]; ok {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		refs = append(refs, pluginManifestReference(item))
	}
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].Name == refs[j].Name {
			return refs[i].AssetID < refs[j].AssetID
		}
		return refs[i].Name < refs[j].Name
	})
	return NormalizeExtensionAssetReferences(refs), nil
}

// EnsureBuiltinExecutionAssets keeps one minimal built-in asset selection in the
// formal config-asset stores so execution/trace/eval can attribute one concrete
// version without expanding any install UI or external catalog flow.
func (s *Service) EnsureBuiltinExecutionAssets(ctx context.Context) error {
	if s == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if s.skillManifestStore != nil {
		if err := s.skillManifestStore.WriteSkillManifest(ctx, builtinSkillManifestRecord(now)); err != nil {
			return fmt.Errorf("write builtin skill manifest: %w", err)
		}
	}
	if s.blueprintDefinitionStore != nil {
		if err := s.blueprintDefinitionStore.WriteBlueprintDefinition(ctx, builtinBlueprintDefinitionRecord(now)); err != nil {
			return fmt.Errorf("write builtin blueprint definition: %w", err)
		}
	}
	if s.promptTemplateStore != nil {
		if err := s.promptTemplateStore.WritePromptTemplateVersion(ctx, builtinPromptTemplateVersionRecord(now)); err != nil {
			return fmt.Errorf("write builtin prompt template version: %w", err)
		}
	}
	return nil
}

func latestSkillManifestRef(ctx context.Context, store SkillManifestStore) (ExtensionAssetReference, bool, error) {
	if store == nil {
		return ExtensionAssetReference{}, false, nil
	}
	if lookup, ok := store.(skillManifestSourceLookup); ok {
		item, found, err := lookup.latestSkillManifestBySource(ctx, extensionAssetSourceBuiltin)
		if err != nil || !found {
			return ExtensionAssetReference{}, found, err
		}
		ref, ok := normalizeSingleExtensionAssetReference(skillManifestReference(item))
		return ref, ok, nil
	}
	return latestSkillManifestRefByPaging(ctx, store)
}

func latestBlueprintDefinitionRef(ctx context.Context, store BlueprintDefinitionStore) (ExtensionAssetReference, bool, error) {
	if store == nil {
		return ExtensionAssetReference{}, false, nil
	}
	if lookup, ok := store.(blueprintDefinitionSourceLookup); ok {
		item, found, err := lookup.latestBlueprintDefinitionBySource(ctx, extensionAssetSourceBuiltin)
		if err != nil || !found {
			return ExtensionAssetReference{}, found, err
		}
		ref, ok := normalizeSingleExtensionAssetReference(blueprintDefinitionReference(item))
		return ref, ok, nil
	}
	return latestBlueprintDefinitionRefByPaging(ctx, store)
}

func latestPromptTemplateVersionRef(ctx context.Context, store PromptTemplateVersionStore) (ExtensionAssetReference, bool, error) {
	if store == nil {
		return ExtensionAssetReference{}, false, nil
	}
	if lookup, ok := store.(promptTemplateVersionSourceLookup); ok {
		item, found, err := lookup.latestPromptTemplateVersionBySource(ctx, extensionAssetSourceBuiltin)
		if err != nil || !found {
			return ExtensionAssetReference{}, found, err
		}
		ref, ok := normalizeSingleExtensionAssetReference(promptTemplateVersionReference(item))
		return ref, ok, nil
	}
	return latestPromptTemplateVersionRefByPaging(ctx, store)
}

// The execution finalization path needs one current built-in config asset per
// kind without pulling full history tables into memory. Concrete stores expose a
// source-filtered lookup; this paged fallback keeps custom test doubles bounded
// to one row per query while preserving the same boundary checks.
func latestSkillManifestRefByPaging(ctx context.Context, store SkillManifestStore) (ExtensionAssetReference, bool, error) {
	for offset := 0; ; {
		items, total, err := store.ListSkillManifests(ctx, 1, offset)
		if err != nil {
			return ExtensionAssetReference{}, false, err
		}
		if len(items) == 0 {
			return ExtensionAssetReference{}, false, nil
		}
		if ref, ok := normalizeSingleExtensionAssetReference(skillManifestReference(items[0])); ok {
			return ref, true, nil
		}
		if total > 0 && offset+len(items) >= total {
			return ExtensionAssetReference{}, false, nil
		}
		offset += len(items)
	}
}

func latestBlueprintDefinitionRefByPaging(ctx context.Context, store BlueprintDefinitionStore) (ExtensionAssetReference, bool, error) {
	for offset := 0; ; {
		items, total, err := store.ListBlueprintDefinitions(ctx, 1, offset)
		if err != nil {
			return ExtensionAssetReference{}, false, err
		}
		if len(items) == 0 {
			return ExtensionAssetReference{}, false, nil
		}
		if ref, ok := normalizeSingleExtensionAssetReference(blueprintDefinitionReference(items[0])); ok {
			return ref, true, nil
		}
		if total > 0 && offset+len(items) >= total {
			return ExtensionAssetReference{}, false, nil
		}
		offset += len(items)
	}
}

func latestPromptTemplateVersionRefByPaging(ctx context.Context, store PromptTemplateVersionStore) (ExtensionAssetReference, bool, error) {
	for offset := 0; ; {
		items, total, err := store.ListPromptTemplateVersions(ctx, 1, offset)
		if err != nil {
			return ExtensionAssetReference{}, false, err
		}
		if len(items) == 0 {
			return ExtensionAssetReference{}, false, nil
		}
		if ref, ok := normalizeSingleExtensionAssetReference(promptTemplateVersionReference(items[0])); ok {
			return ref, true, nil
		}
		if total > 0 && offset+len(items) >= total {
			return ExtensionAssetReference{}, false, nil
		}
		offset += len(items)
	}
}

func skillManifestReference(item SkillManifestRecord) ExtensionAssetReference {
	return ExtensionAssetReference{
		AssetKind: ExtensionAssetKindSkillManifest,
		AssetID:   item.SkillManifestID,
		Name:      item.Name,
		Version:   item.Version,
		Source:    item.Source,
		Summary:   item.Summary,
	}
}

func blueprintDefinitionReference(item BlueprintDefinitionRecord) ExtensionAssetReference {
	return ExtensionAssetReference{
		AssetKind: ExtensionAssetKindBlueprintDefinition,
		AssetID:   item.BlueprintDefinitionID,
		Name:      item.Name,
		Version:   item.Version,
		Source:    item.Source,
		Summary:   item.Summary,
	}
}

func promptTemplateVersionReference(item PromptTemplateVersionRecord) ExtensionAssetReference {
	return ExtensionAssetReference{
		AssetKind: ExtensionAssetKindPromptTemplateVersion,
		AssetID:   item.PromptTemplateVersionID,
		Name:      item.TemplateName,
		Version:   item.Version,
		Source:    item.Source,
		Summary:   item.Summary,
	}
}

func normalizeSingleExtensionAssetReference(ref ExtensionAssetReference) (ExtensionAssetReference, bool) {
	refs := NormalizeExtensionAssetReferences([]ExtensionAssetReference{ref})
	if len(refs) != 1 {
		return ExtensionAssetReference{}, false
	}
	return refs[0], true
}

func pluginManifestReference(item PluginManifestRecord) ExtensionAssetReference {
	return ExtensionAssetReference{
		AssetKind:    ExtensionAssetKindPluginManifest,
		AssetID:      item.PluginID,
		Name:         item.Name,
		Version:      item.Version,
		Source:       item.Source,
		Summary:      item.Summary,
		Entry:        item.Entry,
		Capabilities: decodeStringList(item.CapabilitiesJSON),
		Permissions:  decodeStringList(item.PermissionsJSON),
		RuntimeNames: decodeStringList(item.RuntimeNamesJSON),
	}
}

func builtinSkillManifestRecord(timestamp string) SkillManifestRecord {
	return SkillManifestRecord{
		SkillManifestID: "skill_builtin_default_agent_loop",
		Name:            "default_agent_loop_skill",
		Version:         "builtin-v1",
		Source:          "builtin",
		Summary:         "Built-in owner-5 default skill boundary for the agent loop.",
		ManifestJSON:    `{"entry":"builtin://skill/default_agent_loop","mode":"read_only"}`,
		CreatedAt:       timestamp,
		UpdatedAt:       timestamp,
	}
}

func builtinBlueprintDefinitionRecord(timestamp string) BlueprintDefinitionRecord {
	return BlueprintDefinitionRecord{
		BlueprintDefinitionID: "blueprint_builtin_default_execution",
		Name:                  "default_execution_blueprint",
		Version:               "builtin-v1",
		Source:                "builtin",
		Summary:               "Built-in owner-5 execution blueprint boundary.",
		DefinitionJSON:        `{"steps":["context_prepare","agent_loop","delivery_publish"]}`,
		CreatedAt:             timestamp,
		UpdatedAt:             timestamp,
	}
}

func builtinPromptTemplateVersionRecord(timestamp string) PromptTemplateVersionRecord {
	return PromptTemplateVersionRecord{
		PromptTemplateVersionID: "prompt_builtin_default_execution_planner",
		TemplateName:            "default_execution_planner",
		Version:                 "builtin-v1",
		Source:                  "builtin",
		Summary:                 "Built-in owner-5 prompt template for execution planning.",
		TemplateBody:            "You are the default CialloClaw execution planner. Prefer safe task-centric delivery and preserve tool, trace, and governance boundaries.",
		VariablesJSON:           `["task_id","intent_name","delivery_type"]`,
		CreatedAt:               timestamp,
		UpdatedAt:               timestamp,
	}
}

func decodeStringList(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(trimmed), &values); err != nil {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if candidate := strings.TrimSpace(value); candidate != "" {
			result = append(result, candidate)
		}
	}
	return result
}
