package execution

import (
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/builtin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/sidecarclient"
)

func TestAgentLoopToolDefinitionsUseSharedCatalog(t *testing.T) {
	service := newAgentLoopCapabilityTestService(t, true)
	definitions := service.agentLoopToolDefinitions()
	if len(definitions) != 4 {
		t.Fatalf("expected four planner-visible agent loop tools, got %+v", definitions)
	}

	wantNames := []string{"read_file", "list_dir", "page_read", "page_search"}
	for index, want := range wantNames {
		if definitions[index].Name != want {
			t.Fatalf("unexpected tool definition order at %d: got %q want %q", index, definitions[index].Name, want)
		}
		if !service.isAllowedAgentLoopTool(definitions[index].Name) {
			t.Fatalf("expected planner-visible tool %q to stay executable", definitions[index].Name)
		}
	}

	mutated := definitions[0].InputSchema
	mutatedProperties, ok := mutated["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected read_file schema properties, got %+v", mutated)
	}
	delete(mutatedProperties, "path")

	refreshed := service.agentLoopToolDefinitions()
	refreshedProperties, ok := refreshed[0].InputSchema["properties"].(map[string]any)
	if !ok || refreshedProperties["path"] == nil {
		t.Fatalf("expected shared catalog schemas to be cloned per call, got %+v", refreshed[0].InputSchema)
	}
}

func TestAgentLoopToolAllowlistRequiresCatalogMembershipAndRegistryPresence(t *testing.T) {
	builtinOnly := newAgentLoopCapabilityTestService(t, false)
	if !builtinOnly.isAllowedAgentLoopTool("read_file") {
		t.Fatal("expected registered catalog tool to be allowed")
	}
	if builtinOnly.isAllowedAgentLoopTool("page_search") {
		t.Fatal("expected missing registry entry to stay disallowed")
	}

	withPlaywright := newAgentLoopCapabilityTestService(t, true)
	if withPlaywright.isAllowedAgentLoopTool("structured_dom") {
		t.Fatal("expected non-catalog tool to stay disallowed even when registered")
	}
	if withPlaywright.isAllowedAgentLoopTool("unknown_tool") {
		t.Fatal("expected unknown tool to stay disallowed")
	}
}

func newAgentLoopCapabilityTestService(t *testing.T, registerPlaywright bool) *Service {
	t.Helper()

	toolRegistry := tools.NewRegistry()
	if err := builtin.RegisterBuiltinTools(toolRegistry); err != nil {
		t.Fatalf("register builtin tools: %v", err)
	}
	if registerPlaywright {
		if err := sidecarclient.RegisterPlaywrightTools(toolRegistry); err != nil {
			t.Fatalf("register playwright tools: %v", err)
		}
	}

	return NewService(
		platform.NewLocalFileSystemAdapter(mustPathPolicy(t)),
		stubExecutionCapability{},
		sidecarclient.NewNoopPlaywrightSidecarClient(),
		sidecarclient.NewNoopOCRWorkerClient(),
		sidecarclient.NewNoopMediaWorkerClient(),
		sidecarclient.NewNoopScreenCaptureClient(),
		nil,
		audit.NewService(),
		checkpoint.NewService(),
		delivery.NewService(),
		toolRegistry,
		nil,
		plugin.NewService(),
	)
}
