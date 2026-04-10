package builtin

import (
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

func TestDefaultToolsReturnsFourCoreTools(t *testing.T) {
	items := DefaultTools()
	if len(items) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(items))
	}

	names := []string{
		items[0].Metadata().Name,
		items[1].Metadata().Name,
		items[2].Metadata().Name,
		items[3].Metadata().Name,
	}
	expected := []string{"read_file", "write_file", "list_dir", "exec_command"}
	for index, want := range expected {
		if names[index] != want {
			t.Fatalf("expected tool %q at index %d, got %q", want, index, names[index])
		}
	}
}

func TestRegisterBuiltinTools(t *testing.T) {
	registry := tools.NewRegistry()

	if err := RegisterBuiltinTools(registry); err != nil {
		t.Fatalf("RegisterBuiltinTools returned error: %v", err)
	}

	items := registry.List()
	if len(items) != 4 {
		t.Fatalf("expected 4 registered tools, got %d", len(items))
	}
	for _, item := range items {
		if item.Source != tools.ToolSourceBuiltin {
			t.Fatalf("expected builtin source, got %+v", item)
		}
	}
}
