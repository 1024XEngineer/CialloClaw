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
	if len(definitions) != 6 {
		t.Fatalf("expected six planner-visible tools without browser hints, got %+v", definitions)
	}

	wantNames := []string{"read_file", "list_dir", "extract_text", "page_read", "page_search", "web_search"}
	for index, want := range wantNames {
		if definitions[index].Name != want {
			t.Fatalf("unexpected tool definition order at %d: got %q want %q", index, definitions[index].Name, want)
		}
		if !service.isAllowedAgentLoopTool(definitions[index].Name) {
			t.Fatalf("expected planner-visible tool %q to stay executable", definitions[index].Name)
		}
		if !strings.Contains(definitions[index].Description, "适用场景") || !strings.Contains(definitions[index].Description, "不适用场景") || !strings.Contains(definitions[index].Description, "约束") {
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
	definitions := service.agentLoopToolDefinitionsForSnapshot(taskcontext.TaskContextSnapshot{
		BrowserKind: "chrome",
		PageURL:     "https://example.com",
		WindowTitle: "Example",
	})
	if len(definitions) != 11 {
		t.Fatalf("expected browser-capable snapshot to expose eleven planner-visible tools, got %+v", definitions)
	}

	wantNames := []string{"read_file", "list_dir", "extract_text", "browser_attach_current", "browser_snapshot", "browser_tabs_list", "page_read", "page_search", "browser_navigate", "browser_tab_focus", "web_search"}
	names := make([]string, 0, len(definitions))
	browserAttachIndex := -1
	for index, want := range wantNames {
		names = append(names, definitions[index].Name)
		if definitions[index].Name != want {
			t.Fatalf("unexpected browser-aware tool definition order at %d: got %q want %q", index, definitions[index].Name, want)
		}
		if definitions[index].Name == "browser_attach_current" {
			browserAttachIndex = index
		}
	}
	for _, want := range []string{"read_file", "list_dir", "extract_text", "browser_attach_current", "browser_snapshot", "page_read", "page_search", "web_search"} {
		if !containsString(names, want) {
			t.Fatalf("expected browser-capable snapshot to expose %q, got %+v", want, names)
		}
	}
	if browserAttachIndex < 0 {
		t.Fatalf("expected browser_attach_current to stay visible, got %+v", definitions)
	}
	browserAttachDefinition := definitions[browserAttachIndex]
	if !strings.Contains(browserAttachDefinition.Description, "Chrome/Edge") {
		t.Fatalf("expected browser_attach_current description to explain attach boundary, got %q", browserAttachDefinition.Description)
	}
	properties, ok := browserAttachDefinition.InputSchema["properties"].(map[string]any)
	if !ok || properties == nil {
		t.Fatalf("expected browser_attach_current schema properties to stay an empty object, got %+v", browserAttachDefinition.InputSchema)
	}
	if len(properties) != 0 {
		t.Fatalf("expected browser_attach_current schema properties to stay empty, got %+v", properties)
	}
}

func TestAgentLoopToolDefinitionsAllowSparseBrowserContextForDiscoveryTools(t *testing.T) {
	service := newAgentLoopCapabilityTestService(t, true)
	definitions := service.agentLoopToolDefinitionsForSnapshot(taskcontext.TaskContextSnapshot{BrowserKind: "edge"})
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.Name)
	}
	if !containsString(names, "read_file") || !containsString(names, "list_dir") || !containsString(names, "extract_text") || !containsString(names, "page_read") || !containsString(names, "page_search") || !containsString(names, "web_search") {
		t.Fatalf("expected sparse browser context to preserve non-browser planner tools, got %+v", names)
	}
	if containsString(names, "browser_attach_current") || containsString(names, "browser_snapshot") || containsString(names, "browser_navigate") {
		t.Fatalf("expected sparse browser context to keep current-page browser tools hidden, got %+v", names)
	}
	if !containsString(names, "browser_tabs_list") || !containsString(names, "browser_tab_focus") {
		t.Fatalf("expected sparse browser context to expose browser discovery tools, got %+v", names)
	}
}

func TestJoinCapabilityConstraintsSkipsBlankEntries(t *testing.T) {
	joined := joinCapabilityConstraints([]string{" workspace files only ", "", "prefer list_dir before read_file when the path is uncertain"})
	if joined != "workspace files only, prefer list_dir before read_file when the path is uncertain" {
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
	if !builtinOnly.isAllowedAgentLoopTool("extract_text") {
		t.Fatal("expected registered worker catalog tool to be allowed")
	}
	if builtinOnly.isAllowedAgentLoopTool("browser_snapshot") {
		t.Fatal("expected missing browser registry entry to stay disallowed")
	}
	if builtinOnly.isAllowedAgentLoopTool("page_search") {
		t.Fatal("expected missing registry entry to stay disallowed")
	}
	if builtinOnly.isAllowedAgentLoopTool("web_search") {
		t.Fatal("expected missing registry entry to stay disallowed")
	}

	withPlaywright := newAgentLoopCapabilityTestService(t, true)
	if withPlaywright.isAllowedAgentLoopTool("browser_attach_current") {
		t.Fatal("expected browser_attach_current to stay hidden without attach-capable snapshot hints")
	}
	if !withPlaywright.isAllowedAgentLoopToolForSnapshot("browser_attach_current", taskcontext.TaskContextSnapshot{BrowserKind: "edge", PageURL: "https://example.com", WindowTitle: "Example"}) {
		t.Fatal("expected browser_attach_current to be allowed when the snapshot exposes an attach-capable browser")
	}
	if withPlaywright.isAllowedAgentLoopToolForSnapshot("browser_attach_current", taskcontext.TaskContextSnapshot{BrowserKind: "chrome"}) {
		t.Fatal("expected browser_attach_current to stay hidden for sparse browser context without target hints")
	}
	if !withPlaywright.isAllowedAgentLoopToolForSnapshot("browser_tabs_list", taskcontext.TaskContextSnapshot{BrowserKind: "chrome"}) {
		t.Fatal("expected browser_tabs_list to be allowed for sparse browser context")
	}
	if !withPlaywright.isAllowedAgentLoopToolForSnapshot("browser_navigate", taskcontext.TaskContextSnapshot{BrowserKind: "edge", PageURL: "https://example.com", WindowTitle: "Example"}) {
		t.Fatal("expected browser_navigate to be allowed when the snapshot exposes an attach-capable page")
	}
	if !withPlaywright.isAllowedAgentLoopToolForSnapshot("browser_tab_focus", taskcontext.TaskContextSnapshot{BrowserKind: "chrome"}) {
		t.Fatal("expected browser_tab_focus to be allowed for sparse browser context")
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
	if err := sidecarclient.RegisterOCRTools(toolRegistry); err != nil {
		t.Fatalf("register ocr tools: %v", err)
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
