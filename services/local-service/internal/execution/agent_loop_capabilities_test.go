package execution

import (
	"strings"
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/builtin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools/sidecarclient"
)

func TestAgentLoopToolDefinitionsUseSharedCatalog(t *testing.T) {
	service := newAgentLoopCapabilityTestService(t, true)
	definitions := service.agentLoopToolDefinitions()
	if len(definitions) != 4 {
		t.Fatalf("expected four planner-visible tools without browser hints, got %+v", definitions)
	}

	wantNames := []string{"read_file", "list_dir", "page_read", "page_search"}
	for index, want := range wantNames {
		if definitions[index].Name != want {
			t.Fatalf("unexpected tool definition order at %d: got %q want %q", index, definitions[index].Name, want)
		}
		if !service.isAllowedAgentLoopTool(definitions[index].Name) {
			t.Fatalf("expected planner-visible tool %q to stay executable", definitions[index].Name)
		}
		if !strings.Contains(definitions[index].Description, "适用场景：") || !strings.Contains(definitions[index].Description, "不适用场景：") || !strings.Contains(definitions[index].Description, "约束：") {
			t.Fatalf("expected planner-visible tool %q to include guidance text, got %q", definitions[index].Name, definitions[index].Description)
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

func TestAgentLoopToolDefinitionsExposeBrowserToolsWhenSnapshotSupportsAttach(t *testing.T) {
	service := newAgentLoopCapabilityTestService(t, true)
	definitions := service.agentLoopToolDefinitionsForSnapshot(taskcontext.TaskContextSnapshot{BrowserKind: "chrome", PageURL: "https://example.com", WindowTitle: "Example"})
	if len(definitions) != 6 {
		t.Fatalf("expected browser-capable snapshot to expose six planner-visible tools, got %+v", definitions)
	}
	wantNames := []string{"read_file", "list_dir", "browser_attach_current", "browser_snapshot", "page_read", "page_search"}
	for index, want := range wantNames {
		if definitions[index].Name != want {
			t.Fatalf("unexpected browser-aware tool definition order at %d: got %q want %q", index, definitions[index].Name, want)
		}
	}
	if !strings.Contains(definitions[2].Description, "不会隐式导航或交互页面") {
		t.Fatalf("expected browser_attach_current description to explain attach boundary, got %q", definitions[2].Description)
	}
}

func TestAgentLoopToolDefinitionsAllowSparseBrowserContextForDiscoveryTools(t *testing.T) {
	service := newAgentLoopCapabilityTestService(t, true)
	definitions := service.agentLoopToolDefinitionsForSnapshot(taskcontext.TaskContextSnapshot{BrowserKind: "edge"})
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.Name)
	}
	if !containsString(names, "read_file") || !containsString(names, "list_dir") || !containsString(names, "page_read") || !containsString(names, "page_search") {
		t.Fatalf("expected sparse browser context to preserve non-browser planner tools, got %+v", names)
	}
	if containsString(names, "browser_attach_current") || containsString(names, "browser_snapshot") || containsString(names, "browser_tabs_list") || containsString(names, "browser_navigate") || containsString(names, "browser_tab_focus") {
		t.Fatalf("expected sparse browser context to keep target-dependent browser tools hidden, got %+v", names)
	}
}

func TestJoinCapabilityConstraintsSkipsBlankEntries(t *testing.T) {
	joined := joinCapabilityConstraints([]string{" 仅限工作区文件 ", "", "路径不确定时先用 list_dir"})
	if joined != "仅限工作区文件, 路径不确定时先用 list_dir" {
		t.Fatalf("unexpected joined constraints: %q", joined)
	}
	if joined := joinCapabilityConstraints(nil); joined != "" {
		t.Fatalf("expected nil constraints to stay empty, got %q", joined)
	}
}

func TestAgentLoopToolAllowlistRequiresCatalogMembershipAndRegistryPresence(t *testing.T) {
	builtinOnly := newAgentLoopCapabilityTestService(t, false)
	if !builtinOnly.isAllowedAgentLoopTool("read_file") {
		t.Fatal("expected registered catalog tool to be allowed")
	}
	if builtinOnly.isAllowedAgentLoopTool("browser_snapshot") {
		t.Fatal("expected missing browser registry entry to stay disallowed")
	}
	if builtinOnly.isAllowedAgentLoopTool("page_search") {
		t.Fatal("expected missing registry entry to stay disallowed")
	}

	withPlaywright := newAgentLoopCapabilityTestService(t, true)
	if withPlaywright.isAllowedAgentLoopTool("structured_dom") {
		t.Fatal("expected structured_dom to stay outside the default planner catalog")
	}
	if withPlaywright.isAllowedAgentLoopTool("browser_attach_current") {
		t.Fatal("expected browser_attach_current to stay hidden without attach-capable snapshot hints")
	}
	if !withPlaywright.isAllowedAgentLoopToolForSnapshot("browser_attach_current", taskcontext.TaskContextSnapshot{BrowserKind: "edge", PageURL: "https://example.com", WindowTitle: "Example"}) {
		t.Fatal("expected browser_attach_current to be allowed when the snapshot exposes an attach-capable browser")
	}
	if withPlaywright.isAllowedAgentLoopToolForSnapshot("browser_attach_current", taskcontext.TaskContextSnapshot{BrowserKind: "chrome"}) {
		t.Fatal("expected browser_attach_current to stay hidden for sparse browser context without target hints")
	}
	if withPlaywright.isAllowedAgentLoopToolForSnapshot("browser_tabs_list", taskcontext.TaskContextSnapshot{BrowserKind: "chrome"}) {
		t.Fatal("expected browser_tabs_list to stay hidden until loop governance can pause for approval")
	}
	if withPlaywright.isAllowedAgentLoopToolForSnapshot("browser_navigate", taskcontext.TaskContextSnapshot{BrowserKind: "edge", PageURL: "https://example.com", WindowTitle: "Example"}) {
		t.Fatal("expected browser_navigate to stay hidden until loop governance can pause for approval")
	}
	if withPlaywright.isAllowedAgentLoopTool("browser_interact") {
		t.Fatal("expected browser_interact to stay disallowed until the planner catalog opts in")
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
