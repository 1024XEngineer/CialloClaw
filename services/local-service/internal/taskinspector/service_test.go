package taskinspector

import (
	"os"
	"path/filepath"
	"strings"
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
	if summary["identified_items"] != 2 {
		t.Fatalf("expected identified_items 2 after source-backed sync, got %+v", summary)
	}
	if summary["due_today"] != 0 || summary["overdue"] != 0 {
		t.Fatalf("expected due bucket counts to be aggregated, got %+v", summary)
	}
	if summary["stale"] != 1 {
		t.Fatalf("expected stale count 1, got %+v", summary)
	}
	if len(result.NotepadItems) != 3 {
		t.Fatalf("expected parsed notepad items to be returned, got %+v", result.NotepadItems)
	}
	if result.NotepadItems[0]["source_path"] == nil {
		t.Fatalf("expected source-backed notepad metadata, got %+v", result.NotepadItems[0])
	}
	if len(result.Suggestions) < 2 {
		t.Fatalf("expected runtime suggestions, got %+v", result.Suggestions)
	}
}

func TestServiceRunParsesMarkdownIntoRichNotepadFoundation(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("NewLocalPathPolicy returned error: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "todos"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	content := strings.Join([]string{
		"- [ ] Weekly retro",
		"  due: 2026-04-18",
		"  repeat: every 2 weeks",
		"  prerequisite: collect status updates",
		"  resource: workspace/templates/retro.md",
		"  scope: Project A",
		"  note: review blockers and next steps",
		"- [ ] Later review packet",
		"  bucket: later",
		"  resource: https://example.com/review",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "weekly.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }
	result := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if len(result.NotepadItems) != 2 {
		t.Fatalf("expected parsed notes from markdown, got %+v", result.NotepadItems)
	}
	retro := result.NotepadItems[0]
	if retro["bucket"] != notepadBucketRecurringRule || retro["type"] != "recurring" {
		t.Fatalf("expected weekly retro to become recurring rule item, got %+v", retro)
	}
	if retro["repeat_rule_text"] != "every 2 weeks" || retro["prerequisite"] != "collect status updates" {
		t.Fatalf("expected recurring metadata to be parsed, got %+v", retro)
	}
	resources, ok := retro["related_resources"].([]map[string]any)
	if !ok || len(resources) < 2 {
		t.Fatalf("expected parsed resources plus source path fallback, got %+v", retro["related_resources"])
	}
	if retro["next_occurrence_at"] == nil {
		t.Fatalf("expected next occurrence to be derived, got %+v", retro)
	}
	later := result.NotepadItems[1]
	if later["bucket"] != notepadBucketLater {
		t.Fatalf("expected explicit bucket metadata to win, got %+v", later)
	}
}

func TestTaskInspectorHelperFunctions(t *testing.T) {
	if countChecklistItems("- [ ] one\n* [x] two\nplain text") != 2 {
		t.Fatal("expected checklist counter to include open and closed items")
	}
	resolved := resolveSources(nil, map[string]any{"task_sources": []any{"workspace/todos", "workspace/todos", "workspace/later"}})
	if len(resolved) != 2 || resolved[0] != "workspace/todos" {
		t.Fatalf("expected resolveSources to dedupe non-empty values, got %+v", resolved)
	}
	if sourceToFSPath("/workspace/notes") != "workspace/notes" {
		t.Fatalf("expected sourceToFSPath to normalize workspace prefix")
	}
	if sourceToFSPath("../../etc") != "" {
		t.Fatalf("expected sourceToFSPath to reject outside-workspace paths")
	}
	tags := splitTagList("urgent, weekly, notes")
	if len(tags) != 3 || tags[1] != "weekly" {
		t.Fatalf("expected splitTagList to trim comma-separated values, got %+v", tags)
	}
	resources := resourceListValue([]any{map[string]any{"path": "workspace/todos/inbox.md"}})
	if len(resources) != 1 || !hasResourcePath(resources, "workspace/todos/inbox.md") {
		t.Fatalf("expected resourceListValue and hasResourcePath to cooperate, got %+v", resources)
	}
	if buildSourceResource(map[string]any{"item_id": "todo_001"}, "https://example.com")["target_kind"] != "url" {
		t.Fatal("expected url resource to be marked as url")
	}
	if deriveParsedRecurringNextOccurrence(map[string]any{"planned_at": "2026-04-18T09:30:00Z", "repeat_rule_text": "every month"}) != "2026-05-18T09:30:00Z" {
		t.Fatal("expected parsed recurring helper to support monthly rules")
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
