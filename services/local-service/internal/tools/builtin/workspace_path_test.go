package builtin

import "testing"

func TestNormalizeWorkspaceToolPathPreservesAbsoluteUnixPaths(t *testing.T) {
	pathValue := normalizeWorkspaceToolPath("/tmp/out.md")
	if pathValue != "/tmp/out.md" {
		t.Fatalf("expected absolute path to stay absolute, got %q", pathValue)
	}
}

func TestNormalizeWorkspaceToolPathTrimsWorkspaceAlias(t *testing.T) {
	pathValue := normalizeWorkspaceToolPath("workspace/docs/out.md")
	if pathValue != "docs/out.md" {
		t.Fatalf("expected workspace alias to become relative path, got %q", pathValue)
	}
}

func TestNormalizeWorkspaceToolPathRejectsEscapingRelativePath(t *testing.T) {
	pathValue := normalizeWorkspaceToolPath("../outside.md")
	if pathValue != "" {
		t.Fatalf("expected escaping path to be rejected, got %q", pathValue)
	}
}
