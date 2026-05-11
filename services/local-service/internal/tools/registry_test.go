package tools

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

type registryTestTool struct {
	meta ToolMetadata
}

func (t *registryTestTool) Metadata() ToolMetadata {
	return t.meta
}

func (t *registryTestTool) Validate(_ map[string]any) error {
	return nil
}

func (t *registryTestTool) Execute(_ context.Context, _ *ToolExecuteContext, _ map[string]any) (*ToolResult, error) {
	return &ToolResult{}, nil
}

func makeRegistryTool(name, displayName string, source ToolSource) *registryTestTool {
	return &registryTestTool{meta: ToolMetadata{Name: name, DisplayName: displayName, Source: source}}
}

func TestToolRegistryRegister(t *testing.T) {
	tests := []struct {
		name    string
		tool    Tool
		wantErr error
	}{
		{name: "nil_tool", tool: nil, wantErr: ErrToolValidationFailed},
		{name: "empty_name", tool: &registryTestTool{meta: ToolMetadata{DisplayName: "无名称", Source: ToolSourceBuiltin}}, wantErr: ErrToolNameRequired},
		{name: "non_snake_case_name", tool: &registryTestTool{meta: ToolMetadata{Name: "readFile", DisplayName: "驼峰", Source: ToolSourceBuiltin}}, wantErr: ErrToolNameInvalid},
		{name: "empty_source", tool: &registryTestTool{meta: ToolMetadata{Name: "no_source", DisplayName: "无来源"}}, wantErr: ErrToolSourceRequired},
		{name: "invalid_source", tool: &registryTestTool{meta: ToolMetadata{Name: "bad_source", DisplayName: "非法来源", Source: ToolSource("cloud")}}, wantErr: ErrToolSourceInvalid},
		{name: "empty_display_name", tool: &registryTestTool{meta: ToolMetadata{Name: "no_display", Source: ToolSourceBuiltin}}, wantErr: ErrToolDisplayNameRequired},
		{name: "valid_builtin", tool: makeRegistryTool("read_file", "读取文件", ToolSourceBuiltin), wantErr: nil},
		{name: "valid_worker", tool: makeRegistryTool("ocr_scan", "OCR扫描", ToolSourceWorker), wantErr: nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reg := NewRegistry()
			err := reg.Register(tc.tool)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestToolRegistryRejectsDuplicateName(t *testing.T) {
	reg := NewRegistry()
	tool1 := makeRegistryTool("dup_tool", "重复", ToolSourceBuiltin)
	tool2 := makeRegistryTool("dup_tool", "重复2", ToolSourceWorker)

	if err := reg.Register(tool1); err != nil {
		t.Fatalf("first register should succeed, got %v", err)
	}
	if err := reg.Register(tool2); !errors.Is(err, ErrToolDuplicateName) {
		t.Fatalf("expected ErrToolDuplicateName, got %v", err)
	}
}

func TestToolRegistryMustRegister(t *testing.T) {
	t.Run("panic_on_error", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected panic, got nil")
			}
		}()

		reg := NewRegistry()
		reg.MustRegister(nil)
	})

	t.Run("success", func(t *testing.T) {
		reg := NewRegistry()
		reg.MustRegister(makeRegistryTool("ok_tool", "正常", ToolSourceBuiltin))
		if reg.Count() != 1 {
			t.Fatalf("expected 1 tool, got %d", reg.Count())
		}
	})
}

func TestToolRegistryGet(t *testing.T) {
	reg := NewRegistry()
	reg.MustRegister(makeRegistryTool("find_me", "查找", ToolSourceBuiltin))
	reg.MustRegister(makeRegistryTool("other_tool", "其他", ToolSourceWorker))

	tests := []struct {
		name     string
		toolName string
		wantErr  error
	}{
		{name: "exists", toolName: "find_me", wantErr: nil},
		{name: "not_exists", toolName: "missing", wantErr: ErrToolNotFound},
		{name: "empty_name", toolName: "", wantErr: ErrToolNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tool, err := reg.Get(tc.toolName)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if tool.Metadata().Name != tc.toolName {
					t.Fatalf("expected tool name %q, got %q", tc.toolName, tool.Metadata().Name)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestToolRegistryList(t *testing.T) {
	t.Run("sorted_metadata", func(t *testing.T) {
		reg := NewRegistry()
		reg.MustRegister(makeRegistryTool("z_tool", "Z", ToolSourceBuiltin))
		reg.MustRegister(makeRegistryTool("a_tool", "A", ToolSourceBuiltin))
		reg.MustRegister(makeRegistryTool("m_tool", "M", ToolSourceWorker))

		list := reg.List()
		if len(list) != 3 {
			t.Fatalf("expected 3 items, got %d", len(list))
		}
		if list[0].Name != "a_tool" || list[1].Name != "m_tool" || list[2].Name != "z_tool" {
			t.Fatalf("expected sorted order, got %+v", list)
		}
	})

	t.Run("empty_registry", func(t *testing.T) {
		reg := NewRegistry()
		list := reg.List()
		if len(list) != 0 {
			t.Fatalf("expected empty list, got %d items", len(list))
		}
	})
}

func TestToolRegistryListBySource(t *testing.T) {
	reg := NewRegistry()
	reg.MustRegister(makeRegistryTool("b1", "B1", ToolSourceBuiltin))
	reg.MustRegister(makeRegistryTool("b2", "B2", ToolSourceBuiltin))
	reg.MustRegister(makeRegistryTool("w1", "W1", ToolSourceWorker))
	reg.MustRegister(makeRegistryTool("s1", "S1", ToolSourceSidecar))

	tests := []struct {
		name      string
		source    ToolSource
		wantCount int
		wantFirst string
	}{
		{name: "builtin", source: ToolSourceBuiltin, wantCount: 2, wantFirst: "b1"},
		{name: "worker", source: ToolSourceWorker, wantCount: 1, wantFirst: "w1"},
		{name: "sidecar", source: ToolSourceSidecar, wantCount: 1, wantFirst: "s1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			items := reg.ListBySource(tc.source)
			if len(items) != tc.wantCount {
				t.Fatalf("expected %d items, got %d", tc.wantCount, len(items))
			}
			if items[0].Name != tc.wantFirst {
				t.Fatalf("expected first item %q, got %q", tc.wantFirst, items[0].Name)
			}
		})
	}
}

func TestToolRegistryNamesAndCount(t *testing.T) {
	reg := NewRegistry()
	if reg.Count() != 0 {
		t.Fatalf("expected 0, got %d", reg.Count())
	}

	reg.MustRegister(makeRegistryTool("z_name", "Z", ToolSourceBuiltin))
	reg.MustRegister(makeRegistryTool("a_name", "A", ToolSourceBuiltin))

	names := reg.Names()
	if len(names) != 2 || names[0] != "a_name" || names[1] != "z_name" {
		t.Fatalf("unexpected names: %+v", names)
	}
	if reg.Count() != 2 {
		t.Fatalf("expected 2, got %d", reg.Count())
	}
}

func TestNewRegistryWithInitialTools(t *testing.T) {
	reg := NewRegistry(
		makeRegistryTool("init_a", "A", ToolSourceBuiltin),
		makeRegistryTool("init_b", "B", ToolSourceWorker),
	)
	if reg.Count() != 2 {
		t.Fatalf("expected 2 initial tools, got %d", reg.Count())
	}
}

func TestNewRegistryPanicsOnBadInitialTool(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for bad initial tool")
		}
	}()

	NewRegistry(&registryTestTool{meta: ToolMetadata{Name: "BadName", DisplayName: "非法", Source: ToolSourceBuiltin}})
}

func ExampleRegistry() {
	reg := NewRegistry()

	reg.MustRegister(makeRegistryTool("read_file", "Read file", ToolSourceBuiltin))
	reg.MustRegister(makeRegistryTool("ocr_scan", "OCR扫描", ToolSourceWorker))

	tool, err := reg.Get("read_file")
	if err != nil {
		panic(err)
	}

	fmt.Println(tool.Metadata().DisplayName)
	fmt.Println(reg.Count())

	builtins := reg.ListBySource(ToolSourceBuiltin)
	fmt.Println(len(builtins))

	// Output:
	// Read file
	// 2
	// 1
}
