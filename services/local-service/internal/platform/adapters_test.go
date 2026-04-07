package platform

import (
	"path/filepath"
	"testing"
)

func TestEnsureWithinWorkspace(t *testing.T) {
	workspaceRoot := t.TempDir()
	policy, err := NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}

	insidePath := filepath.Join(workspaceRoot, "notes", "demo.md")
	if _, err := policy.EnsureWithinWorkspace(insidePath); err != nil {
		t.Fatalf("expected inside path to pass: %v", err)
	}

	outsidePath := filepath.Join(workspaceRoot, "..", "outside.md")
	if _, err := policy.EnsureWithinWorkspace(outsidePath); err == nil {
		t.Fatal("expected outside path to fail")
	}
}
