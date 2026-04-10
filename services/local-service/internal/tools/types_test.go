package tools

import (
	"testing"
)

// ---------------------------------------------------------------------------
// isSnakeCase
// ---------------------------------------------------------------------------

func TestIsSnakeCaseAcceptsValidNames(t *testing.T) {
	valid := []string{"read_file", "write_file", "command_preview", "a1_b2_c3"}
	for _, name := range valid {
		if !isSnakeCase(name) {
			t.Fatalf("expected %q to be valid snake_case", name)
		}
	}
}

func TestIsSnakeCaseRejectsInvalidNames(t *testing.T) {
	invalid := []string{"readFile", "ReadFile", "read-file", "read file", "1read", "_leading", "trailing_", ""}
	for _, name := range invalid {
		if isSnakeCase(name) {
			t.Fatalf("expected %q to be invalid snake_case", name)
		}
	}
}

// ---------------------------------------------------------------------------
// ToolMetadata.Validate
// ---------------------------------------------------------------------------

func TestToolMetadataValidateRejectsEmptyName(t *testing.T) {
	meta := ToolMetadata{DisplayName: "工具", Source: ToolSourceBuiltin}
	if err := meta.Validate(); err != ErrToolNameRequired {
		t.Fatalf("expected ErrToolNameRequired, got %v", err)
	}
}

func TestToolMetadataValidateRejectsNonSnakeCaseName(t *testing.T) {
	meta := ToolMetadata{Name: "readFile", DisplayName: "工具", Source: ToolSourceBuiltin}
	if err := meta.Validate(); err == nil {
		t.Fatal("expected error for non-snake_case name")
	}
}

func TestToolMetadataValidateRejectsEmptySource(t *testing.T) {
	meta := ToolMetadata{Name: "read_file", DisplayName: "工具"}
	if err := meta.Validate(); err != ErrToolSourceRequired {
		t.Fatalf("expected ErrToolSourceRequired, got %v", err)
	}
}

func TestToolMetadataValidateRejectsInvalidSource(t *testing.T) {
	meta := ToolMetadata{Name: "read_file", DisplayName: "工具", Source: "cloud"}
	if err := meta.Validate(); err == nil {
		t.Fatal("expected error for invalid source")
	}
}

func TestToolMetadataValidateRejectsEmptyDisplayName(t *testing.T) {
	meta := ToolMetadata{Name: "read_file", Source: ToolSourceBuiltin}
	if err := meta.Validate(); err != ErrToolDisplayNameRequired {
		t.Fatalf("expected ErrToolDisplayNameRequired, got %v", err)
	}
}

func TestToolMetadataValidatePassesForValidMetadata(t *testing.T) {
	meta := ToolMetadata{
		Name:        "read_file",
		DisplayName: "读取文件",
		Source:      ToolSourceBuiltin,
		RiskHint:    "green",
		TimeoutSec:  10,
	}
	if err := meta.Validate(); err != nil {
		t.Fatalf("expected valid metadata, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// ToolResult
// ---------------------------------------------------------------------------

func TestToolResultCarriesError(t *testing.T) {
	result := &ToolResult{
		ToolName: "read_file",
		Error: &ToolResultError{
			Code:    1003002,
			Message: "read failed",
		},
	}
	if result.Error == nil || result.Error.Code != 1003002 {
		t.Fatalf("unexpected error: %+v", result.Error)
	}
}

func TestToolResultCarriesArtifacts(t *testing.T) {
	result := &ToolResult{
		ToolName: "write_file",
		Artifacts: []ArtifactRef{
			{ArtifactType: "generated_doc", Title: "report.md", Path: "/workspace/report.md", MimeType: "text/markdown"},
		},
	}
	if len(result.Artifacts) != 1 || result.Artifacts[0].Title != "report.md" {
		t.Fatalf("unexpected artifacts: %+v", result.Artifacts)
	}
}

// ---------------------------------------------------------------------------
// ToolSource
// ---------------------------------------------------------------------------

func TestToolSourceValues(t *testing.T) {
	if ToolSourceBuiltin != "builtin" {
		t.Fatalf("expected builtin, got %q", ToolSourceBuiltin)
	}
	if ToolSourceWorker != "worker" {
		t.Fatalf("expected worker, got %q", ToolSourceWorker)
	}
	if ToolSourceSidecar != "sidecar" {
		t.Fatalf("expected sidecar, got %q", ToolSourceSidecar)
	}
}
