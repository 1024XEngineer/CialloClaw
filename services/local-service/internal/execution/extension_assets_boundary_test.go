package execution

import (
	"context"
	"testing"

	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
)

func TestExecuteAttachesModelProviderAndPerceptionBoundaryAssets(t *testing.T) {
	service, _ := newTestExecutionServiceWithConfig(t, serviceconfig.ModelConfig{
		Provider: model.OpenAIResponsesProvider,
		ModelID:  "gpt-5.4",
		Endpoint: "https://api.openai.com/v1/responses",
	}, "Boundary-aware summary")

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_boundary_assets",
		RunID:        "run_boundary_assets",
		Title:        "Boundary asset test",
		Intent:       map[string]any{"name": "explain", "arguments": map[string]any{}},
		Snapshot:     taskcontext.TaskContextSnapshot{InputType: "text_selection", SelectionText: "release note warning", WindowTitle: "Browser - Release", VisibleText: "Warning: release notes incomplete.", ClipboardText: "copied release summary"},
		DeliveryType: "bubble",
		ResultTitle:  "Boundary asset result",
	})
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	if len(result.ExtensionAssets) < 5 {
		t.Fatalf("expected builtin execution assets plus provider/perception boundaries, got %+v", result.ExtensionAssets)
	}
	foundProvider := false
	foundPerception := false
	for _, asset := range result.ExtensionAssets {
		if asset["asset_kind"] == storage.ExtensionAssetKindModelProviderRoute && asset["asset_id"] == model.OpenAIResponsesProvider {
			foundProvider = true
		}
		if asset["asset_kind"] == storage.ExtensionAssetKindPerceptionPackage && asset["asset_id"] == "desktop_context_core" {
			foundPerception = true
		}
	}
	if !foundProvider || !foundPerception {
		t.Fatalf("expected execution to attach provider and perception boundary assets, got %+v", result.ExtensionAssets)
	}
	refs, ok := result.ModelInvocation["extension_asset_refs"].([]map[string]any)
	if !ok || len(refs) != len(result.ExtensionAssets) {
		t.Fatalf("expected model invocation to mirror boundary asset refs, got %+v", result.ModelInvocation)
	}
}

func TestSupplementalExecutionBoundaryAssetsSkipsUnusedBoundaries(t *testing.T) {
	modelService := model.NewService(serviceconfig.ModelConfig{Provider: model.OpenAIResponsesProvider, ModelID: "gpt-5.4", Endpoint: "https://api.openai.com/v1/responses"}, &stubModelClient{output: "unused"})
	refs := supplementalExecutionBoundaryAssets(Request{Snapshot: taskcontext.TaskContextSnapshot{}}, Result{}, modelService)
	if len(refs) != 0 {
		t.Fatalf("expected no supplemental boundary assets when model and perception boundaries were unused, got %+v", refs)
	}
	refs = supplementalExecutionBoundaryAssets(Request{}, Result{ModelInvocation: map[string]any{"provider": "budget_downgrade_fallback"}}, modelService)
	if len(refs) != 0 {
		t.Fatalf("expected synthetic fallback provider markers not to resolve provider route assets, got %+v", refs)
	}
	if snapshotUsesPerceptionBoundary(taskcontext.TaskContextSnapshot{}) {
		t.Fatal("expected empty snapshot not to require perception package attribution")
	}
	if snapshotUsesPerceptionBoundary(taskcontext.TaskContextSnapshot{SelectionText: "selected text only"}) {
		t.Fatal("expected pure selection text not to trigger perception package attribution")
	}
	if snapshotUsesPerceptionBoundary(taskcontext.TaskContextSnapshot{ErrorText: "runtime error text only"}) {
		t.Fatal("expected pure error text not to trigger perception package attribution")
	}
}

func TestExecuteDoesNotAttachPerceptionPackageForSelectionOnlyInput(t *testing.T) {
	service, _ := newTestExecutionServiceWithConfig(t, serviceconfig.ModelConfig{
		Provider: model.OpenAIResponsesProvider,
		ModelID:  "gpt-5.4",
		Endpoint: "https://api.openai.com/v1/responses",
	}, "Selection-only summary")

	result, err := service.Execute(context.Background(), Request{
		TaskID:       "task_selection_only_boundary",
		RunID:        "run_selection_only_boundary",
		Title:        "Selection-only boundary test",
		Intent:       map[string]any{"name": "explain", "arguments": map[string]any{}},
		Snapshot:     taskcontext.TaskContextSnapshot{InputType: "text_selection", SelectionText: "selected sentence only"},
		DeliveryType: "bubble",
		ResultTitle:  "Selection-only boundary result",
	})
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}
	for _, asset := range result.ExtensionAssets {
		if asset["asset_kind"] == storage.ExtensionAssetKindPerceptionPackage {
			t.Fatalf("expected selection-only execution not to attach perception package asset, got %+v", result.ExtensionAssets)
		}
	}
}
