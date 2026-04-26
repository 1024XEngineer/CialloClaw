package storage

import "testing"

func TestNormalizeExtensionAssetReferencesFiltersUnsupportedBoundaryEntries(t *testing.T) {
	refs := NormalizeExtensionAssetReferences([]ExtensionAssetReference{
		{
			AssetKind: ExtensionAssetKindSkillManifest,
			AssetID:   "skill_builtin_default_agent_loop",
			Name:      " default_agent_loop_skill ",
			Version:   " builtin-v1 ",
			Source:    " builtin ",
			Entry:     "ignored://skill",
		},
		{
			AssetKind: ExtensionAssetKindSkillManifest,
			AssetID:   "community_skill",
			Name:      "community",
			Version:   "v1",
			Source:    extensionAssetSourceGitHub,
		},
		{
			AssetKind: "perception_pack",
			AssetID:   "pack_001",
			Version:   "v1",
			Source:    extensionAssetSourceBuiltin,
		},
		{
			AssetKind:    ExtensionAssetKindModelProviderRoute,
			AssetID:      "openai_responses",
			Name:         "OpenAI Responses",
			Version:      "builtin-v1",
			Source:       extensionAssetSourceBuiltin,
			Entry:        "builtin://model-provider/openai_responses",
			Capabilities: []string{"generate_text", " generate_text ", "generate_tool_calls"},
			Permissions:  []string{"network:model_api", "network:model_api", "secret:model_api_key"},
			RuntimeNames: []string{"ignored_runtime"},
		},
		{
			AssetKind:    ExtensionAssetKindPerceptionPackage,
			AssetID:      "desktop_context_core",
			Name:         "Desktop Context Core",
			Version:      "builtin-v1",
			Source:       extensionAssetSourceBuiltin,
			Entry:        "builtin://perception-package/desktop_context_core",
			Capabilities: []string{"screen_context", "screen_context", "clipboard_context"},
			Permissions:  []string{"screen:read", "clipboard:read", "clipboard:read"},
		},
		{
			AssetKind: ExtensionAssetKindPerceptionPackage,
			AssetID:   "desktop_context_core",
			Version:   "builtin-v1",
			Source:    extensionAssetSourceGitHub,
		},
		{
			AssetKind: ExtensionAssetKindPluginManifest,
			AssetID:   "plugin_ocr",
			Name:      "OCR",
			Version:   "v1",
			Source:    extensionAssetSourceMarketplace,
			Capabilities: []string{
				"ocr_image",
				" ocr_image ",
				"",
			},
			Permissions: []string{"artifact_read", "artifact_read"},
			RuntimeNames: []string{
				"ocr_worker",
				" ocr_worker ",
			},
		},
		{
			AssetKind: ExtensionAssetKindPluginManifest,
			AssetID:   "plugin_ocr",
			Name:      "OCR",
			Version:   "v1",
			Source:    extensionAssetSourceMarketplace,
		},
		{
			AssetKind: ExtensionAssetKindPromptTemplateVersion,
			AssetID:   "prompt_missing_version",
			Version:   "",
			Source:    extensionAssetSourceBuiltin,
		},
	})

	if len(refs) != 4 {
		t.Fatalf("expected builtin config assets, provider/perception boundaries, and supported plugin refs, got %+v", refs)
	}
	if refs[0].AssetKind != ExtensionAssetKindSkillManifest || refs[0].Source != extensionAssetSourceBuiltin {
		t.Fatalf("expected builtin skill manifest to survive normalization, got %+v", refs[0])
	}
	if refs[0].Entry != "" || len(refs[0].Capabilities) != 0 || len(refs[0].Permissions) != 0 || len(refs[0].RuntimeNames) != 0 {
		t.Fatalf("expected non-plugin asset fields to be cleared, got %+v", refs[0])
	}
	if refs[1].AssetKind != ExtensionAssetKindModelProviderRoute || refs[1].Source != extensionAssetSourceBuiltin {
		t.Fatalf("expected builtin provider route to survive normalization, got %+v", refs[1])
	}
	if len(refs[1].Capabilities) != 2 || refs[1].Capabilities[0] != "generate_text" {
		t.Fatalf("expected provider capabilities to be trimmed and deduplicated, got %+v", refs[1])
	}
	if len(refs[1].Permissions) != 2 || refs[1].Permissions[0] != "network:model_api" {
		t.Fatalf("expected provider permissions to be trimmed and deduplicated, got %+v", refs[1])
	}
	if len(refs[1].RuntimeNames) != 0 {
		t.Fatalf("expected provider runtime names to be cleared, got %+v", refs[1])
	}
	if refs[2].AssetKind != ExtensionAssetKindPerceptionPackage || refs[2].Source != extensionAssetSourceBuiltin {
		t.Fatalf("expected builtin perception package to survive normalization, got %+v", refs[2])
	}
	if len(refs[2].Capabilities) != 2 || refs[2].Capabilities[0] != "screen_context" {
		t.Fatalf("expected perception capabilities to be trimmed and deduplicated, got %+v", refs[2])
	}
	if len(refs[2].Permissions) != 2 || refs[2].Permissions[0] != "screen:read" {
		t.Fatalf("expected perception permissions to be trimmed and deduplicated, got %+v", refs[2])
	}
	if refs[3].AssetKind != ExtensionAssetKindPluginManifest || refs[3].Source != extensionAssetSourceMarketplace {
		t.Fatalf("expected marketplace plugin manifest to survive normalization, got %+v", refs[3])
	}
	if len(refs[3].Capabilities) != 1 || refs[3].Capabilities[0] != "ocr_image" {
		t.Fatalf("expected plugin capabilities to be trimmed and deduplicated, got %+v", refs[3])
	}
	if len(refs[3].Permissions) != 1 || refs[3].Permissions[0] != "artifact_read" {
		t.Fatalf("expected plugin permissions to be trimmed and deduplicated, got %+v", refs[3])
	}
	if len(refs[3].RuntimeNames) != 1 || refs[3].RuntimeNames[0] != "ocr_worker" {
		t.Fatalf("expected plugin runtime names to be trimmed and deduplicated, got %+v", refs[3])
	}
}

func TestNormalizeExtensionAssetReferencesRejectsInvalidPluginSources(t *testing.T) {
	refs := NormalizeExtensionAssetReferences([]ExtensionAssetReference{{
		AssetKind: ExtensionAssetKindPluginManifest,
		AssetID:   "plugin_unknown",
		Version:   "v1",
		Source:    "zip_file",
	}})
	if refs != nil {
		t.Fatalf("expected unsupported plugin source to be dropped, got %+v", refs)
	}
}
