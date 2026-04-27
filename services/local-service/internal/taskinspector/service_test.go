package taskinspector

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
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

func TestServiceRunDecodesLegacyMarkdownSources(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("NewLocalPathPolicy returned error: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "todos"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	content, _, err := transform.Bytes(simplifiedchinese.GB18030.NewEncoder(), []byte("- [ ] 修复巡检乱码\n"))
	if err != nil {
		t.Fatalf("GB18030 encode failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "legacy.md"), content, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }
	result := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})

	if result.Summary["parsed_files"] != 1 || len(result.NotepadItems) != 1 {
		t.Fatalf("expected legacy markdown source to be parsed, got summary=%+v items=%+v", result.Summary, result.NotepadItems)
	}
	if result.NotepadItems[0]["title"] != "修复巡检乱码" {
		t.Fatalf("expected decoded notepad title, got %+v", result.NotepadItems[0])
	}
}

func TestServiceRunPreservesNotepadWhenSourceDecodeFails(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("NewLocalPathPolicy returned error: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "todos"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "good.md"), []byte("- [ ] source item\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "bad.md"), []byte{0x00, 0x01, 0x02, 0xff}, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }
	result := service.Run(RunInput{
		Config: map[string]any{"task_sources": []string{"workspace/todos"}},
		NotepadItems: []map[string]any{
			{"item_id": "todo_existing", "title": "preserve me", "status": "normal"},
		},
	})

	if result.SourceSynced {
		t.Fatalf("expected failed source decode to block source sync")
	}
	if result.Summary["parsed_files"] != 1 {
		t.Fatalf("expected readable source files to still be counted, got %+v", result.Summary)
	}
	if len(result.NotepadItems) != 1 || result.NotepadItems[0]["title"] != "preserve me" {
		t.Fatalf("expected existing notepad items to be preserved, got %+v", result.NotepadItems)
	}
}

func TestServiceRunSkipsBinaryAttachmentsAndKeepsTextSources(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("NewLocalPathPolicy returned error: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "todos"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "good.md"), []byte("- [ ] source item\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "notes.txt"), []byte("- [ ] txt item\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "checklist"), []byte("- [ ] extensionless item\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "attachment.bin"), []byte{0x00, 0x01, 0x02, 0xff}, 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }
	result := service.Run(RunInput{
		Config: map[string]any{"task_sources": []string{"workspace/todos"}},
		NotepadItems: []map[string]any{
			{"item_id": "todo_existing", "title": "old snapshot", "status": "normal"},
		},
	})

	if !result.SourceSynced {
		t.Fatalf("expected binary attachments to be skipped without blocking source sync")
	}
	if result.Summary["parsed_files"] != 3 {
		t.Fatalf("expected text source files to be counted, got %+v", result.Summary)
	}
	if len(result.NotepadItems) != 3 {
		t.Fatalf("expected readable text sources to replace old snapshot, got %+v", result.NotepadItems)
	}
	titles := map[string]bool{}
	for _, item := range result.NotepadItems {
		titles[stringValue(item, "title")] = true
	}
	for _, title := range []string{"source item", "txt item", "extensionless item"} {
		if !titles[title] {
			t.Fatalf("expected parsed title %q in %+v", title, result.NotepadItems)
		}
	}
}

func TestServiceRunIgnoresUnsupportedTextTaskSourceFiles(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("NewLocalPathPolicy returned error: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "todos"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "good.md"), []byte("- [ ] markdown item\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "notes.txt"), []byte("- [ ] text item\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "config.json"), []byte("{\n  \"checklist\": [\"- [ ] should stay ignored\"]\n}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }
	result := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})

	if !result.SourceSynced {
		t.Fatalf("expected supported task source files to sync cleanly, got %+v", result)
	}
	if result.Summary["parsed_files"] != 2 {
		t.Fatalf("expected only markdown and txt task sources to be counted, got %+v", result.Summary)
	}
	if len(result.NotepadItems) != 2 {
		t.Fatalf("expected unsupported text files to stay ignored, got %+v", result.NotepadItems)
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
	for _, path := range []string{"todos/inbox.md", "todos/inbox.markdown", "todos/notes.txt", "todos/checklist"} {
		if shouldSkipTaskSourceAttachment(path) || shouldSkipUnreadableTaskSourceFile(path) {
			t.Fatalf("expected text task source file %q to be accepted", path)
		}
	}
	if !shouldSkipTaskSourceAttachment("todos/attachment.bin") {
		t.Fatal("expected binary attachment to be skipped")
	}
	if !isSupportedTextTaskSourceFile("todos/notes.txt") || !isSupportedTextTaskSourceFile("todos/checklist") {
		t.Fatal("expected supported text source helper to preserve text compatibility")
	}
	if isSupportedTextTaskSourceFile("todos/config.json") {
		t.Fatal("expected unsupported text source helper to reject non-task file types")
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
