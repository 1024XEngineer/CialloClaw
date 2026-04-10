package taskinspector

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

func TestServiceRunAggregatesWorkspaceNotepadAndRuntimeState(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("NewLocalPathPolicy returned error: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "todos"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "inbox.md"), []byte("- [ ] review report\n- [x] archive note\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "later.md"), []byte("- [ ] follow up\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }

	result := service.Run(RunInput{
		Config: map[string]any{
			"task_sources":           []string{"workspace/todos"},
			"inspection_interval":    map[string]any{"unit": "minute", "value": 15},
			"inspect_on_startup":     true,
			"inspect_on_file_change": true,
		},
		UnfinishedTasks: []runengine.TaskRecord{
			{
				TaskID:    "task_001",
				Title:     "stale task",
				Status:    "processing",
				UpdatedAt: time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC),
			},
		},
		NotepadItems: []map[string]any{
			{"item_id": "todo_001", "title": "today item", "status": "due_today"},
			{"item_id": "todo_002", "title": "overdue item", "status": "overdue"},
			{"item_id": "todo_003", "title": "later item", "status": "normal"},
			{"item_id": "todo_004", "title": "done item", "status": "completed"},
		},
	})

	summary := result.Summary
	if summary["parsed_files"] != 2 {
		t.Fatalf("expected parsed_files 2, got %+v", summary)
	}
	if summary["identified_items"] != 6 {
		t.Fatalf("expected identified_items 6, got %+v", summary)
	}
	if summary["due_today"] != 1 || summary["overdue"] != 1 {
		t.Fatalf("expected due bucket counts to be aggregated, got %+v", summary)
	}
	if summary["stale"] != 1 {
		t.Fatalf("expected stale count 1, got %+v", summary)
	}
	if len(result.Suggestions) < 3 {
		t.Fatalf("expected runtime suggestions, got %+v", result.Suggestions)
	}
}

func TestServiceRunHonorsTargetSourcesAndHandlesMissingFiles(t *testing.T) {
	service := NewService(nil)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC) }

	result := service.Run(RunInput{
		TargetSources: []string{"workspace/missing"},
		Config: map[string]any{
			"task_sources":        []string{"workspace/todos"},
			"inspection_interval": map[string]any{"unit": "hour", "value": 1},
		},
	})

	if result.Summary["parsed_files"] != 0 {
		t.Fatalf("expected no parsed files without file system, got %+v", result.Summary)
	}
	if len(result.Suggestions) == 0 || result.Suggestions[0] == "" {
		t.Fatalf("expected fallback suggestion, got %+v", result.Suggestions)
	}
	if sourceToFSPath("workspace/missing") != "missing" {
		t.Fatalf("expected target source to use workspace-relative fs path")
	}
}
