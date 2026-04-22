package traceeval

import (
	"encoding/json"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

func TestServiceCaptureNormalizesExtensionAssetBoundary(t *testing.T) {
	service := NewService(nil, nil)
	result, err := service.Capture(CaptureInput{
		TaskID: "task_boundary",
		RunID:  "run_boundary",
		ExtensionAssets: []storage.ExtensionAssetReference{
			{
				AssetKind: storage.ExtensionAssetKindSkillManifest,
				AssetID:   "skill_builtin_default_agent_loop",
				Name:      "default_agent_loop_skill",
				Version:   "builtin-v1",
				Source:    "builtin",
			},
			{
				AssetKind: storage.ExtensionAssetKindSkillManifest,
				AssetID:   "community_skill",
				Name:      "community_skill",
				Version:   "v1",
				Source:    "github",
			},
			{
				AssetKind: storage.ExtensionAssetKindPluginManifest,
				AssetID:   "playwright",
				Name:      "Playwright Automation",
				Version:   "builtin-v1",
				Source:    "marketplace",
			},
			{
				AssetKind:    storage.ExtensionAssetKindModelProviderRoute,
				AssetID:      "openai_responses",
				Name:         "OpenAI Responses",
				Version:      "builtin-v1",
				Source:       "builtin",
				Entry:        "builtin://model-provider/openai_responses",
				Capabilities: []string{"generate_text"},
				Permissions:  []string{"secret:model_api_key"},
			},
			{
				AssetKind:    storage.ExtensionAssetKindPerceptionPackage,
				AssetID:      "desktop_context_core",
				Name:         "Desktop Context Core",
				Version:      "builtin-v1",
				Source:       "builtin",
				Capabilities: []string{"screen_context"},
				Permissions:  []string{"screen:read"},
			},
			{
				AssetKind: storage.ExtensionAssetKindPromptTemplateVersion,
				AssetID:   "prompt_missing_version",
				Source:    "builtin",
			},
		},
	})
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	var refs []storage.ExtensionAssetReference
	if err := json.Unmarshal([]byte(result.TraceRecord.AssetRefsJSON), &refs); err != nil {
		t.Fatalf("unmarshal trace asset refs: %v", err)
	}
	if len(refs) != 4 {
		t.Fatalf("expected only supported extension asset refs to be recorded, got %+v", refs)
	}
	if result.Metrics["extension_asset_count"] != 4 {
		t.Fatalf("expected extension asset metrics to count normalized refs, got %+v", result.Metrics)
	}
	if result.Metrics[storage.ExtensionAssetKindSkillManifest+"_count"] != 1 {
		t.Fatalf("expected one builtin skill manifest ref after normalization, got %+v", result.Metrics)
	}
	if result.Metrics[storage.ExtensionAssetKindPluginManifest+"_count"] != 1 {
		t.Fatalf("expected one plugin manifest ref after normalization, got %+v", result.Metrics)
	}
	if result.Metrics[storage.ExtensionAssetKindModelProviderRoute+"_count"] != 1 {
		t.Fatalf("expected one model provider route ref after normalization, got %+v", result.Metrics)
	}
	if result.Metrics[storage.ExtensionAssetKindPerceptionPackage+"_count"] != 1 {
		t.Fatalf("expected one perception package ref after normalization, got %+v", result.Metrics)
	}
}
