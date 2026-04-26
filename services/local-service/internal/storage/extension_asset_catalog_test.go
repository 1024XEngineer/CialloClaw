package storage

import (
	"context"
	"errors"
	"testing"
)

func TestServiceCurrentExecutionAssetsAndPluginResolution(t *testing.T) {
	service := NewService(nil)
	if err := service.EnsureBuiltinExecutionAssets(context.Background()); err != nil {
		t.Fatalf("ensure builtin execution assets: %v", err)
	}
	if err := service.PluginManifestStore().WritePluginManifest(context.Background(), PluginManifestRecord{
		PluginID:         "ocr",
		Name:             "OCR Worker",
		Version:          "builtin-v1",
		Entry:            "builtin://plugin/ocr",
		Source:           "builtin",
		Summary:          "OCR runtime manifest",
		CapabilitiesJSON: `["ocr_image","ocr_pdf"]`,
		PermissionsJSON:  `["artifact_read"]`,
		RuntimeNamesJSON: `["ocr_worker"]`,
	}); err != nil {
		t.Fatalf("write plugin manifest: %v", err)
	}

	refs, err := service.CurrentExecutionAssets(context.Background())
	if err != nil {
		t.Fatalf("current execution assets: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("expected built-in skill/blueprint/prompt refs, got %+v", refs)
	}

	pluginRefs, err := service.PluginAssetsForCapabilities(context.Background(), []string{"ocr_image"})
	if err != nil {
		t.Fatalf("plugin assets for capabilities: %v", err)
	}
	if len(pluginRefs) != 1 || pluginRefs[0].AssetKind != ExtensionAssetKindPluginManifest || pluginRefs[0].AssetID != "ocr" {
		t.Fatalf("expected OCR plugin manifest ref, got %+v", pluginRefs)
	}
}

func TestExtensionAssetCatalogHandlesEmptyAndMalformedPluginData(t *testing.T) {
	service := NewService(nil)
	refs, err := service.CurrentExecutionAssets(context.Background())
	if err != nil {
		t.Fatalf("CurrentExecutionAssets returned error: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("expected nil refs before built-in asset seeding, got %+v", refs)
	}
	if pluginRefs, err := service.PluginAssetsForCapabilities(context.Background(), []string{"   "}); err != nil || pluginRefs != nil {
		t.Fatalf("expected blank capability request to be ignored, refs=%+v err=%v", pluginRefs, err)
	}
	if err := service.PluginManifestStore().WritePluginManifest(context.Background(), PluginManifestRecord{
		PluginID:         "broken",
		Name:             "Broken Plugin",
		Version:          "v1",
		Entry:            "builtin://plugin/broken",
		Source:           "builtin",
		Summary:          "broken manifest",
		CapabilitiesJSON: `not-json`,
		PermissionsJSON:  `[]`,
		RuntimeNamesJSON: `[]`,
	}); err != nil {
		t.Fatalf("write malformed plugin manifest: %v", err)
	}
	if pluginRefs, err := service.PluginAssetsForCapabilities(context.Background(), []string{"ocr_image"}); err != nil || len(pluginRefs) != 0 {
		t.Fatalf("expected malformed plugin manifest capabilities to be ignored, refs=%+v err=%v", pluginRefs, err)
	}
}

func TestServiceCurrentExecutionAssetsSkipsNewerUnsupportedStoreRows(t *testing.T) {
	service := NewService(nil)
	ctx := context.Background()
	builtinSkill := builtinSkillManifestRecord("2026-04-22T10:00:00Z")
	builtinBlueprint := builtinBlueprintDefinitionRecord("2026-04-22T10:00:00Z")
	builtinPrompt := builtinPromptTemplateVersionRecord("2026-04-22T10:00:00Z")
	if err := service.SkillManifestStore().WriteSkillManifest(ctx, builtinSkill); err != nil {
		t.Fatalf("write builtin skill manifest: %v", err)
	}
	if err := service.BlueprintDefinitionStore().WriteBlueprintDefinition(ctx, builtinBlueprint); err != nil {
		t.Fatalf("write builtin blueprint definition: %v", err)
	}
	if err := service.PromptTemplateVersionStore().WritePromptTemplateVersion(ctx, builtinPrompt); err != nil {
		t.Fatalf("write builtin prompt template version: %v", err)
	}
	if err := service.SkillManifestStore().WriteSkillManifest(ctx, SkillManifestRecord{
		SkillManifestID: "skill_community_latest",
		Name:            "community_skill",
		Version:         "v2",
		Source:          extensionAssetSourceGitHub,
		Summary:         "community skill should stay outside the current execution boundary",
		ManifestJSON:    `{}`,
		CreatedAt:       "2026-04-22T11:00:00Z",
		UpdatedAt:       "2026-04-22T11:00:00Z",
	}); err != nil {
		t.Fatalf("write community skill manifest: %v", err)
	}

	refs, err := service.CurrentExecutionAssets(ctx)
	if err != nil {
		t.Fatalf("current execution assets: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("expected fallback to supported builtin execution assets, got %+v", refs)
	}
	if refs[0].AssetKind != ExtensionAssetKindSkillManifest || refs[0].AssetID != builtinSkill.SkillManifestID {
		t.Fatalf("expected builtin skill manifest to remain the selected execution asset, got %+v", refs[0])
	}
}

func TestLatestExecutionAssetRefsFallbackToPagedLookup(t *testing.T) {
	skillStore := &pagingSkillManifestStore{items: []SkillManifestRecord{
		{SkillManifestID: "skill_community_latest", Name: "community", Version: "v2", Source: extensionAssetSourceGitHub, Summary: "summary"},
		{SkillManifestID: "skill_builtin_next", Name: "builtin", Version: "v1", Source: extensionAssetSourceBuiltin, Summary: "summary"},
	}}
	ref, found, err := latestSkillManifestRef(context.Background(), skillStore)
	if err != nil || !found || ref.AssetID != "skill_builtin_next" {
		t.Fatalf("expected paged skill manifest fallback to find builtin ref, ref=%+v found=%v err=%v", ref, found, err)
	}
	if len(skillStore.offsets) != 2 || skillStore.offsets[0] != 0 || skillStore.offsets[1] != 1 {
		t.Fatalf("expected paged skill manifest lookup offsets [0 1], got %+v", skillStore.offsets)
	}
	emptySkillStore := &pagingSkillManifestStore{items: []SkillManifestRecord{{SkillManifestID: "skill_community_only", Name: "community", Version: "v3", Source: extensionAssetSourceGitHub, Summary: "summary"}}}
	if _, found, err := latestSkillManifestRef(context.Background(), emptySkillStore); err != nil || found {
		t.Fatalf("expected paged skill manifest fallback to report no builtin ref, found=%v err=%v", found, err)
	}

	blueprintStore := &pagingBlueprintDefinitionStore{items: []BlueprintDefinitionRecord{
		{BlueprintDefinitionID: "blueprint_community_latest", Name: "community", Version: "v2", Source: extensionAssetSourceGitHub, Summary: "summary"},
		{BlueprintDefinitionID: "blueprint_builtin_next", Name: "builtin", Version: "v1", Source: extensionAssetSourceBuiltin, Summary: "summary"},
	}}
	blueprintRef, found, err := latestBlueprintDefinitionRef(context.Background(), blueprintStore)
	if err != nil || !found || blueprintRef.AssetID != "blueprint_builtin_next" {
		t.Fatalf("expected paged blueprint fallback to find builtin ref, ref=%+v found=%v err=%v", blueprintRef, found, err)
	}
	if len(blueprintStore.offsets) != 2 || blueprintStore.offsets[0] != 0 || blueprintStore.offsets[1] != 1 {
		t.Fatalf("expected paged blueprint lookup offsets [0 1], got %+v", blueprintStore.offsets)
	}
	noBuiltinBlueprintStore := &pagingBlueprintDefinitionStore{items: []BlueprintDefinitionRecord{
		{BlueprintDefinitionID: "blueprint_community", Name: "community", Version: "v2", Source: extensionAssetSourceGitHub, Summary: "summary"},
	}}
	if _, found, err := latestBlueprintDefinitionRef(context.Background(), noBuiltinBlueprintStore); err != nil || found {
		t.Fatalf("expected paged blueprint fallback to report no builtin ref, found=%v err=%v", found, err)
	}

	promptSuccessStore := &pagingPromptTemplateVersionStore{items: []PromptTemplateVersionRecord{
		{PromptTemplateVersionID: "prompt_community_latest", TemplateName: "community", Version: "v2", Source: extensionAssetSourceGitHub, Summary: "summary"},
		{PromptTemplateVersionID: "prompt_builtin_next", TemplateName: "builtin", Version: "v1", Source: extensionAssetSourceBuiltin, Summary: "summary"},
	}}
	promptRef, found, err := latestPromptTemplateVersionRef(context.Background(), promptSuccessStore)
	if err != nil || !found || promptRef.AssetID != "prompt_builtin_next" {
		t.Fatalf("expected paged prompt fallback to find builtin ref, ref=%+v found=%v err=%v", promptRef, found, err)
	}
	if len(promptSuccessStore.offsets) != 2 || promptSuccessStore.offsets[0] != 0 || promptSuccessStore.offsets[1] != 1 {
		t.Fatalf("expected paged prompt lookup offsets [0 1], got %+v", promptSuccessStore.offsets)
	}
	noBuiltinPromptStore := &pagingPromptTemplateVersionStore{items: []PromptTemplateVersionRecord{{PromptTemplateVersionID: "prompt_community_only", TemplateName: "community", Version: "v3", Source: extensionAssetSourceGitHub, Summary: "summary"}}}
	if _, found, err := latestPromptTemplateVersionRef(context.Background(), noBuiltinPromptStore); err != nil || found {
		t.Fatalf("expected paged prompt fallback to report no builtin ref, found=%v err=%v", found, err)
	}

	promptStore := &pagingPromptTemplateVersionStore{err: errors.New("prompt lookup failed")}
	if _, found, err := latestPromptTemplateVersionRef(context.Background(), promptStore); err == nil || found {
		t.Fatalf("expected paged prompt fallback to return error, found=%v err=%v", found, err)
	}
}

type pagingSkillManifestStore struct {
	items   []SkillManifestRecord
	err     error
	offsets []int
}

func (s *pagingSkillManifestStore) WriteSkillManifest(context.Context, SkillManifestRecord) error {
	return errors.New("unused")
}

func (s *pagingSkillManifestStore) GetSkillManifest(context.Context, string) (SkillManifestRecord, error) {
	return SkillManifestRecord{}, errors.New("unused")
}

func (s *pagingSkillManifestStore) ListSkillManifests(_ context.Context, limit, offset int) ([]SkillManifestRecord, int, error) {
	s.offsets = append(s.offsets, offset)
	if s.err != nil {
		return nil, 0, s.err
	}
	if offset >= len(s.items) {
		return nil, len(s.items), nil
	}
	end := offset + limit
	if limit <= 0 || end > len(s.items) {
		end = len(s.items)
	}
	return s.items[offset:end], len(s.items), nil
}

type pagingBlueprintDefinitionStore struct {
	items   []BlueprintDefinitionRecord
	err     error
	offsets []int
}

func (s *pagingBlueprintDefinitionStore) WriteBlueprintDefinition(context.Context, BlueprintDefinitionRecord) error {
	return errors.New("unused")
}

func (s *pagingBlueprintDefinitionStore) GetBlueprintDefinition(context.Context, string) (BlueprintDefinitionRecord, error) {
	return BlueprintDefinitionRecord{}, errors.New("unused")
}

func (s *pagingBlueprintDefinitionStore) ListBlueprintDefinitions(_ context.Context, limit, offset int) ([]BlueprintDefinitionRecord, int, error) {
	s.offsets = append(s.offsets, offset)
	if s.err != nil {
		return nil, 0, s.err
	}
	if offset >= len(s.items) {
		return nil, len(s.items), nil
	}
	end := offset + limit
	if limit <= 0 || end > len(s.items) {
		end = len(s.items)
	}
	return s.items[offset:end], len(s.items), nil
}

type pagingPromptTemplateVersionStore struct {
	items   []PromptTemplateVersionRecord
	err     error
	offsets []int
}

func (s *pagingPromptTemplateVersionStore) WritePromptTemplateVersion(context.Context, PromptTemplateVersionRecord) error {
	return errors.New("unused")
}

func (s *pagingPromptTemplateVersionStore) GetPromptTemplateVersion(context.Context, string) (PromptTemplateVersionRecord, error) {
	return PromptTemplateVersionRecord{}, errors.New("unused")
}

func (s *pagingPromptTemplateVersionStore) ListPromptTemplateVersions(_ context.Context, limit, offset int) ([]PromptTemplateVersionRecord, int, error) {
	s.offsets = append(s.offsets, offset)
	if s.err != nil {
		return nil, 0, s.err
	}
	if offset >= len(s.items) {
		return nil, len(s.items), nil
	}
	end := offset + limit
	if limit <= 0 || end > len(s.items) {
		end = len(s.items)
	}
	return s.items[offset:end], len(s.items), nil
}
