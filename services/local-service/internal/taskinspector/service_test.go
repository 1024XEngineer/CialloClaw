package taskinspector

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type readFileErrorAdapter struct {
	platform.FileSystemAdapter
	failPath string
}

func (a readFileErrorAdapter) ReadFile(name string) ([]byte, error) {
	if filepath.ToSlash(name) == filepath.ToSlash(a.failPath) {
		return nil, fs.ErrPermission
	}
	return a.FileSystemAdapter.ReadFile(name)
}

type relErrorAdapter struct {
	platform.FileSystemAdapter
	failEnsureRoot bool
}

func (a relErrorAdapter) EnsureWithinWorkspace(path string) (string, error) {
	if a.failEnsureRoot && path == "." {
		return "", errors.New("workspace root unavailable")
	}
	return a.FileSystemAdapter.EnsureWithinWorkspace(path)
}

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

	result, err := service.Run(RunInput{
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
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

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
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
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

func TestServiceRunDefaultsGenericSourceChecklistToUpcoming(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("NewLocalPathPolicy returned error: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "todos"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "tasks.md"), []byte("- [ ] review report\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.NotepadItems) != 1 {
		t.Fatalf("expected parsed checklist item, got %+v", result.NotepadItems)
	}
	if result.NotepadItems[0]["bucket"] != notepadBucketUpcoming {
		t.Fatalf("expected generic source checklist to default to upcoming, got %+v", result.NotepadItems[0])
	}
}

func TestServiceRunPreservesMetadataShapedNaturalNotes(t *testing.T) {
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
		"due: tomorrow",
		"note: remember rollback",
		"status: blocked by QA",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "metadata-shaped.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.NotepadItems) != 1 {
		t.Fatalf("expected metadata-shaped natural note to be preserved, got %+v", result.NotepadItems)
	}
	item := result.NotepadItems[0]
	if item["title"] != "due: tomorrow" || item["due_at"] == nil || item["bucket"] != notepadBucketUpcoming {
		t.Fatalf("expected metadata-shaped title to remain natural with due hint, got %+v", item)
	}
	if item["note_text"] != "note: remember rollback\nstatus: blocked by QA" {
		t.Fatalf("expected metadata-shaped body lines to remain natural, got %+v", item)
	}
}

func TestServiceRunParsesManagedTimestampMetadataFromChecklistNotes(t *testing.T) {
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
		"- [x] Release prep",
		"  created_at: 2026-04-09T08:00:00.000Z",
		"  updated_at: 2026-04-10T09:30:00.000Z",
		"  ended_at: 2026-04-11T10:00:00.000Z",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "timestamps.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 12, 9, 30, 0, 0, time.UTC) }
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.NotepadItems) != 1 {
		t.Fatalf("expected one parsed note, got %+v", result.NotepadItems)
	}
	item := result.NotepadItems[0]
	if item["created_at"] != "2026-04-09T08:00:00Z" {
		t.Fatalf("expected created_at metadata to be preserved, got %+v", item)
	}
	if item["updated_at"] != "2026-04-10T09:30:00Z" {
		t.Fatalf("expected updated_at metadata to be preserved, got %+v", item)
	}
	if item["ended_at"] != "2026-04-11T10:00:00Z" {
		t.Fatalf("expected ended_at metadata to be preserved, got %+v", item)
	}
	if item["note_text"] != "Release prep" {
		t.Fatalf("expected managed timestamps to stay out of note_text, got %+v", item)
	}
}

func TestServiceRunKeepsRecurringBucketWithoutInventingRepeatRule(t *testing.T) {
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
		"  bucket: recurring_rule",
		"  due: 2026-04-18",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "weekly.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.NotepadItems) != 1 {
		t.Fatalf("expected one parsed note, got %+v", result.NotepadItems)
	}
	item := result.NotepadItems[0]
	if item["bucket"] != notepadBucketRecurringRule || item["type"] != "recurring" {
		t.Fatalf("expected explicit recurring bucket to be preserved, got %+v", item)
	}
	if item["repeat_rule_text"] != nil {
		t.Fatalf("expected recurring item without repeat metadata to stay unspecified, got %+v", item)
	}
	if item["next_occurrence_at"] != nil {
		t.Fatalf("expected recurring item without repeat metadata to skip derived next occurrence, got %+v", item)
	}
}

func TestServiceRunNormalizesExplicitClosedBucketToCompleted(t *testing.T) {
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
		"- [ ] Archive release prep",
		"  bucket: closed",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "closed.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 12, 9, 30, 0, 0, time.UTC) }
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.NotepadItems) != 1 {
		t.Fatalf("expected one parsed note, got %+v", result.NotepadItems)
	}
	item := result.NotepadItems[0]
	if item["bucket"] != notepadBucketClosed || item["status"] != "completed" {
		t.Fatalf("expected explicit closed bucket to normalize into a completed note, got %+v", item)
	}
	if item["ended_at"] != "2026-04-12T09:30:00Z" {
		t.Fatalf("expected normalized closed note to receive ended_at, got %+v", item)
	}
}

func TestServiceRunDropsEndedAtMetadataForActiveNotes(t *testing.T) {
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
		"- [ ] Reopen note",
		"  bucket: later",
		"  ended_at: 2026-04-11T10:00:00.000Z",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "reopen.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 12, 9, 30, 0, 0, time.UTC) }
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.NotepadItems) != 1 {
		t.Fatalf("expected one parsed note, got %+v", result.NotepadItems)
	}
	item := result.NotepadItems[0]
	if item["bucket"] != notepadBucketLater || item["status"] != "normal" {
		t.Fatalf("expected reopened note to stay active, got %+v", item)
	}
	if item["ended_at"] != nil {
		t.Fatalf("expected active note to drop ended_at metadata, got %+v", item)
	}
}

func TestServiceRunCombinesNoteMetadataAndChecklistBody(t *testing.T) {
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
		"- [ ] Release prep",
		"  note: collect rollout context",
		"",
		"  keep the rollback checklist nearby",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "notes.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 12, 9, 30, 0, 0, time.UTC) }
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.NotepadItems) != 1 {
		t.Fatalf("expected one parsed note, got %+v", result.NotepadItems)
	}
	if result.NotepadItems[0]["note_text"] != "collect rollout context\n\nkeep the rollback checklist nearby" {
		t.Fatalf("expected note metadata and body to stay combined, got %+v", result.NotepadItems[0])
	}
}

func TestServiceRunSplitsTopLevelChecklistAwayFromNaturalNote(t *testing.T) {
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
		"# Release prep",
		"first paragraph",
		"",
		"second paragraph",
		"- item A",
		"- [ ] verify changelog",
		"- [ ] update docs",
		"due: keep visible",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "release.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.NotepadItems) != 3 {
		t.Fatalf("expected one natural note plus two top-level checklist items, got %+v", result.NotepadItems)
	}
	natural := result.NotepadItems[0]
	if natural["title"] != "Release prep" {
		t.Fatalf("expected heading to remain the natural note title, got %+v", natural)
	}
	if natural["note_text"] != "first paragraph\n\nsecond paragraph\n- item A" {
		t.Fatalf("expected natural note body to stop before top-level checklist items, got %+v", natural)
	}
	if result.NotepadItems[1]["title"] != "verify changelog" || result.NotepadItems[1]["note_text"] != "verify changelog" {
		t.Fatalf("expected first top-level checklist to become its own item, got %+v", result.NotepadItems[1])
	}
	if result.NotepadItems[2]["title"] != "update docs" || result.NotepadItems[2]["note_text"] != "update docs" {
		t.Fatalf("expected final checklist row to remain its own note, got %+v", result.NotepadItems[2])
	}
	if result.NotepadItems[2]["due_at"] != "keep visible" || result.NotepadItems[2]["planned_at"] != "keep visible" {
		t.Fatalf("expected following metadata lines to stay attached to the checklist item they belong to, got %+v", result.NotepadItems[2])
	}
}

func TestServiceRunPreservesSerializedChecklistBodyMarkers(t *testing.T) {
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
		"- [ ] Release prep",
		"",
		"  first paragraph",
		"",
		"  second paragraph",
		"  - item A",
		"  - [ ] verify changelog",
		"  - [ ] update docs",
		"    code block",
		"    - nested item",
		"  due: keep visible",
		"- [ ] Separate top-level",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "release.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.NotepadItems) != 2 {
		t.Fatalf("expected serialized body checkboxes plus one top-level item, got %+v", result.NotepadItems)
	}
	first := result.NotepadItems[0]
	if first["title"] != "Release prep" {
		t.Fatalf("expected first serialized block to keep its title, got %+v", first)
	}
	expectedNoteText := "first paragraph\n\nsecond paragraph\n- item A\n- [ ] verify changelog\n- [ ] update docs\n  code block\n  - nested item\ndue: keep visible"
	if first["note_text"] != expectedNoteText {
		t.Fatalf("expected editor-saved checklist body markers to remain body text, got %+v", first)
	}
	if result.NotepadItems[1]["title"] != "Separate top-level" {
		t.Fatalf("expected unindented checklist row to remain a separate item, got %+v", result.NotepadItems[1])
	}
}

func TestServiceRunPreservesChecklistMetadataAfterSpacer(t *testing.T) {
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
		"- [ ] Release prep",
		"",
		"due: 2026-04-18",
		"bucket: later",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "release.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.NotepadItems) != 1 {
		t.Fatalf("expected one checklist item with spacer metadata, got %+v", result.NotepadItems)
	}
	item := result.NotepadItems[0]
	if item["title"] != "Release prep" || item["bucket"] != notepadBucketLater {
		t.Fatalf("expected title and bucket metadata to be preserved, got %+v", item)
	}
	if item["due_at"] != "2026-04-18T09:30:00Z" || item["planned_at"] != "2026-04-18T09:30:00Z" {
		t.Fatalf("expected due metadata to be preserved after spacer, got %+v", item)
	}
}

func TestServiceRunAcceptsTopLevelChecklistWithThreeLeadingSpaces(t *testing.T) {
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
		"   - [ ] Release prep",
		"    - [ ] nested body line",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "indented.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.NotepadItems) != 1 {
		t.Fatalf("expected one parsed checklist item, got %+v", result.NotepadItems)
	}
	item := result.NotepadItems[0]
	if item["title"] != "Release prep" {
		t.Fatalf("expected indented checklist title to be parsed, got %+v", item)
	}
	if item["note_text"] != "  - [ ] nested body line" {
		t.Fatalf("expected four-space line to stay in body text, got %+v", item)
	}
}

func TestServiceRunParsesNaturalMarkdownNotesWithoutMetadata(t *testing.T) {
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
		"# 明天整理发布说明",
		"补充影响范围和回滚说明",
		"",
		"## 每周一同步巡检报告",
		"### 后天检查巡检结果",
		"",
		"## 以后研究插件市场入口",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "notes.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.NotepadItems) != 4 {
		t.Fatalf("expected natural notes to become items, got %+v", result.NotepadItems)
	}
	first := result.NotepadItems[0]
	if first["title"] != "明天整理发布说明" || first["bucket"] != notepadBucketUpcoming || first["due_at"] == nil {
		t.Fatalf("expected tomorrow natural note to infer upcoming due date, got %+v", first)
	}
	if first["note_text"] != "补充影响范围和回滚说明" {
		t.Fatalf("expected following paragraph line to become note text, got %+v", first)
	}
	recurring := result.NotepadItems[1]
	if recurring["bucket"] != notepadBucketRecurringRule || recurring["type"] != "recurring" {
		t.Fatalf("expected natural repeat hint to infer recurring rule, got %+v", recurring)
	}
	third := result.NotepadItems[2]
	if third["title"] != "后天检查巡检结果" || third["bucket"] != notepadBucketUpcoming || third["due_at"] == nil {
		t.Fatalf("expected adjacent heading to start a separate upcoming note, got %+v", third)
	}
	later := result.NotepadItems[3]
	if later["bucket"] != notepadBucketLater {
		t.Fatalf("expected natural later hint to infer later bucket, got %+v", later)
	}
}

func TestServiceRunKeepsNaturalRepeatRuleToHintLine(t *testing.T) {
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
		"# Every Monday sync",
		"Discuss launch details and blockers.",
		"Keep the agenda in this note body.",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "notes.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.NotepadItems) != 1 {
		t.Fatalf("expected one natural recurring note, got %+v", result.NotepadItems)
	}
	item := result.NotepadItems[0]
	if item["bucket"] != notepadBucketRecurringRule || item["type"] != "recurring" {
		t.Fatalf("expected natural repeat hint to infer a recurring item, got %+v", item)
	}
	if item["repeat_rule_text"] != "Every Monday sync" {
		t.Fatalf("expected repeat rule to stay on the hint line only, got %+v", item)
	}
	if item["note_text"] != "Discuss launch details and blockers.\nKeep the agenda in this note body." {
		t.Fatalf("expected natural note body to stay outside repeat metadata, got %+v", item)
	}
}

func TestServiceRunIgnoresNaturalHintsThatAppearOnlyInBodyText(t *testing.T) {
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
		"# Release prep",
		"Repeat the rollout steps tomorrow.",
		"Keep the checklist nearby.",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "notes.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.NotepadItems) != 1 {
		t.Fatalf("expected one natural note, got %+v", result.NotepadItems)
	}
	item := result.NotepadItems[0]
	if item["bucket"] != notepadBucketUpcoming {
		t.Fatalf("expected natural note to keep default upcoming bucket, got %+v", item)
	}
	if item["repeat_rule_text"] != nil {
		t.Fatalf("expected body-only repeat wording to stay out of repeat_rule_text, got %+v", item)
	}
	if item["due_at"] != nil {
		t.Fatalf("expected body-only date wording to stay out of due_at, got %+v", item)
	}
}

func TestServiceRunSplitsHeadinglessNaturalParagraphsIntoSeparateNotes(t *testing.T) {
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
		"Task A",
		"",
		"Task B",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "paragraphs.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(result.NotepadItems) != 2 {
		t.Fatalf("expected heading-less natural paragraphs to become separate notes, got %+v", result.NotepadItems)
	}
	if result.NotepadItems[0]["title"] != "Task A" || result.NotepadItems[0]["note_text"] != "Task A" {
		t.Fatalf("expected first paragraph to stay isolated, got %+v", result.NotepadItems[0])
	}
	if result.NotepadItems[1]["title"] != "Task B" || result.NotepadItems[1]["note_text"] != "Task B" {
		t.Fatalf("expected second paragraph to stay isolated, got %+v", result.NotepadItems[1])
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
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if result.Summary["parsed_files"] != 1 || len(result.NotepadItems) != 1 {
		t.Fatalf("expected legacy markdown source to be parsed, got summary=%+v items=%+v", result.Summary, result.NotepadItems)
	}
	if result.NotepadItems[0]["title"] != "修复巡检乱码" {
		t.Fatalf("expected decoded notepad title, got %+v", result.NotepadItems[0])
	}
}

func TestServiceRunReturnsErrorWhenSourceDecodeFails(t *testing.T) {
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
	_, err = service.Run(RunInput{
		Config: map[string]any{"task_sources": []string{"workspace/todos"}},
		NotepadItems: []map[string]any{
			{"item_id": "todo_existing", "title": "preserve me", "status": "normal"},
		},
	})
	if !errors.Is(err, ErrInspectionSourceUnreadable) {
		t.Fatalf("expected failed task-source decode to map to unreadable source error, got %v", err)
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
	result, err := service.Run(RunInput{
		Config: map[string]any{"task_sources": []string{"workspace/todos"}},
		NotepadItems: []map[string]any{
			{"item_id": "todo_existing", "title": "old snapshot", "status": "normal"},
		},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

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
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

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
	resolvedStrings := resolveSources(nil, map[string]any{"task_sources": []string{"workspace/todos", " ", "workspace/todos"}})
	if len(resolvedStrings) != 1 || resolvedStrings[0] != "workspace/todos" {
		t.Fatalf("expected resolveSources to accept []string settings payloads, got %+v", resolvedStrings)
	}
	emptyPath, err := sourceToFSPath(nil, " ")
	if err != nil || emptyPath != "" {
		t.Fatalf("expected blank source to normalize to empty path, got path=%q err=%v", emptyPath, err)
	}
	fsPath, err := sourceToFSPath(nil, "/workspace/notes")
	if err != nil || fsPath != "notes" {
		t.Fatalf("expected sourceToFSPath to normalize workspace prefix")
	}
	rootPath, err := sourceToFSPath(nil, "/")
	if err != nil || rootPath != "." {
		t.Fatalf("expected root slash to normalize to dot, got path=%q err=%v", rootPath, err)
	}
	drivePath, err := sourceToFSPath(nil, `D:/workspace/notes`)
	if !errors.Is(err, ErrInspectionFileSystemUnavailable) {
		t.Fatalf("expected drive-letter source without file system to require workspace binding, got path=%q err=%v", drivePath, err)
	}
	driveBackslashPath, err := sourceToFSPath(nil, `D:\workspace\notes`)
	if !errors.Is(err, ErrInspectionFileSystemUnavailable) {
		t.Fatalf("expected backslash drive-letter source without file system to require workspace binding, got path=%q err=%v", driveBackslashPath, err)
	}
	_, err = sourceToFSPath(nil, "../../etc")
	if !errors.Is(err, ErrInspectionSourceOutsideWorkspace) {
		t.Fatalf("expected sourceToFSPath to reject outside-workspace paths, got %v", err)
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

	_, err := service.Run(RunInput{
		TargetSources: []string{"workspace/missing"},
		Config: map[string]any{
			"task_sources":        []string{"workspace/todos"},
			"inspection_interval": map[string]any{"unit": "hour", "value": 1},
		},
	})
	if !errors.Is(err, ErrInspectionFileSystemUnavailable) {
		t.Fatalf("expected missing filesystem error, got %v", err)
	}
	fsPath, err := sourceToFSPath(nil, "workspace/missing")
	if err != nil || fsPath != "missing" {
		t.Fatalf("expected target source to use workspace-relative fs path")
	}
}

func TestServiceRunWithoutSourcesKeepsRuntimeNotepadItems(t *testing.T) {
	service := NewService(nil)
	service.now = func() time.Time { return time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC) }

	result, err := service.Run(RunInput{
		NotepadItems: []map[string]any{{
			"item_id": "todo_runtime_only",
			"title":   "keep runtime notes",
			"status":  "normal",
		}},
	})
	if err != nil {
		t.Fatalf("expected no-source run to succeed, got %v", err)
	}
	if result.SourceSynced {
		t.Fatal("expected no-source run to avoid source sync")
	}
	if len(result.NotepadItems) != 1 || result.NotepadItems[0]["item_id"] != "todo_runtime_only" {
		t.Fatalf("expected runtime items to survive without sources, got %+v", result.NotepadItems)
	}
}

func TestServiceRunReturnsExplicitErrorForMissingSource(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("NewLocalPathPolicy returned error: %v", err)
	}
	service := NewService(platform.NewLocalFileSystemAdapter(pathPolicy))
	service.now = func() time.Time { return time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC) }

	_, err = service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/missing"}}})
	if !errors.Is(err, ErrInspectionSourceNotFound) {
		t.Fatalf("expected source not found error, got %v", err)
	}
}

func TestServiceRunReturnsExplicitErrorForUnreadableSource(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("NewLocalPathPolicy returned error: %v", err)
	}
	baseFileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "todos"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "blocked.md"), []byte("- [ ] blocked\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	service := NewService(readFileErrorAdapter{FileSystemAdapter: baseFileSystem, failPath: "todos/blocked.md"})

	_, err = service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if !errors.Is(err, ErrInspectionSourceUnreadable) {
		t.Fatalf("expected source unreadable error, got %v", err)
	}
}

func TestSourceToFSPathAcceptsWorkspaceAbsolutePaths(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("NewLocalPathPolicy returned error: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	absoluteSource := filepath.Join(workspaceRoot, "todos")

	fsPath, err := sourceToFSPath(fileSystem, absoluteSource)
	if err != nil {
		t.Fatalf("expected absolute workspace source to be accepted, got %v", err)
	}
	if fsPath != "todos" {
		t.Fatalf("expected absolute workspace source to stay addressable, got %q", fsPath)
	}

	rootPath, err := sourceToFSPath(fileSystem, workspaceRoot)
	if err != nil || rootPath != "." {
		t.Fatalf("expected workspace root path to normalize to dot, got path=%q err=%v", rootPath, err)
	}

	absWithoutFileSystem, err := sourceToFSPath(nil, absoluteSource)
	if runtime.GOOS == "windows" {
		if !errors.Is(err, ErrInspectionFileSystemUnavailable) {
			t.Fatalf("expected absolute source without file system to require workspace binding on windows, path=%q err=%v", absWithoutFileSystem, err)
		}
	} else {
		if !errors.Is(err, ErrInspectionSourceOutsideWorkspace) {
			t.Fatalf("expected absolute source without file system to stay outside workspace on non-windows hosts, path=%q err=%v", absWithoutFileSystem, err)
		}
	}

	_, err = sourceToFSPath(fileSystem, `D:/workspace/notes`)
	if !errors.Is(err, ErrInspectionSourceOutsideWorkspace) {
		t.Fatalf("expected foreign drive source to stay outside the workspace boundary, got %v", err)
	}

	_, err = sourceToFSPath(nil, "/workspace/../outside")
	if !errors.Is(err, ErrInspectionSourceOutsideWorkspace) {
		t.Fatalf("expected workspace-relative escape path to be rejected, got %v", err)
	}

	_, err = sourceToFSPath(nil, "/tmp/workspace/notes")
	if !errors.Is(err, ErrInspectionSourceOutsideWorkspace) {
		t.Fatalf("expected legacy unix-style absolute path without filesystem binding to be rejected, got %v", err)
	}

	_, err = sourceToFSPath(relErrorAdapter{FileSystemAdapter: fileSystem, failEnsureRoot: true}, absoluteSource)
	if !errors.Is(err, ErrInspectionSourceOutsideWorkspace) {
		t.Fatalf("expected workspace-root resolution failure to map to boundary error, got %v", err)
	}

	_, err = sourceToFSPath(fileSystem, filepath.Join(t.TempDir(), "outside"))
	if !errors.Is(err, ErrInspectionSourceOutsideWorkspace) {
		t.Fatalf("expected outside absolute source to be rejected, got %v", err)
	}

	_, err = sourceToFSPath(fileSystem, `..\evil`)
	if !errors.Is(err, ErrInspectionSourceOutsideWorkspace) {
		t.Fatalf("expected backslash parent traversal to be rejected, got %v", err)
	}

	unsafePath, err := sourceToFSPath(fileSystem, `sub\a.md`)
	if err != nil || unsafePath != "sub/a.md" {
		t.Fatalf("expected relative windows path to normalize to slash form, path=%q err=%v", unsafePath, err)
	}
}
