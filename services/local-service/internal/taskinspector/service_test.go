package taskinspector

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/titlegen"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

type stubModelClient struct {
	output string
	calls  *atomic.Int32
}

func (s stubModelClient) GenerateText(_ context.Context, _ model.GenerateTextRequest) (model.GenerateTextResponse, error) {
	if s.calls != nil {
		s.calls.Add(1)
	}
	return model.GenerateTextResponse{OutputText: s.output}, nil
}

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

	service := NewService(fileSystem).WithTitleGenerator(titlegen.NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"每周复盘阻塞项"}`,
	})))
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }

	result, err := service.Run(RunInput{
		AllowGeneratedTitles: true,
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

	service := NewService(fileSystem).WithTitleGenerator(titlegen.NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"每周复盘阻塞项"}`,
	})))
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }
	result, err := service.Run(RunInput{
		AllowGeneratedTitles: true,
		Config:               map[string]any{"task_sources": []string{"workspace/todos"}},
	})
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
	if retro["title"] != "每周复盘阻塞项" {
		t.Fatalf("expected note body to use generated notepad title, got %+v", retro)
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

func TestServiceRunCachesGeneratedNoteTitlesUntilContentChanges(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("NewLocalPathPolicy returned error: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "todos"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	notePath := filepath.Join(workspaceRoot, "todos", "weekly.md")
	if err := os.WriteFile(notePath, []byte("- [ ] Weekly retro\n  note: review blockers and next steps\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	callCount := &atomic.Int32{}
	service := NewService(fileSystem).WithTitleGenerator(titlegen.NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"每周复盘阻塞项"}`,
		calls:  callCount,
	})))
	service.now = func() time.Time { return time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC) }

	firstResult, err := service.Run(RunInput{
		AllowGeneratedTitles: true,
		Config:               map[string]any{"task_sources": []string{"workspace/todos"}},
	})
	if err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}
	if len(firstResult.NotepadItems) != 1 || firstResult.NotepadItems[0]["title"] != "每周复盘阻塞项" {
		t.Fatalf("expected generated note title on first run, got %+v", firstResult.NotepadItems)
	}
	if got := callCount.Load(); got != 1 {
		t.Fatalf("expected one title generation call on first run, got %d", got)
	}

	secondResult, err := service.Run(RunInput{
		AllowGeneratedTitles: true,
		Config:               map[string]any{"task_sources": []string{"workspace/todos"}},
	})
	if err != nil {
		t.Fatalf("second Run returned error: %v", err)
	}
	if len(secondResult.NotepadItems) != 1 || secondResult.NotepadItems[0]["title"] != "每周复盘阻塞项" {
		t.Fatalf("expected cached note title on second run, got %+v", secondResult.NotepadItems)
	}
	if got := callCount.Load(); got != 1 {
		t.Fatalf("expected unchanged note content to reuse cached title, got %d calls", got)
	}

	if err := os.WriteFile(notePath, []byte("- [ ] Weekly retro\n  note: review blockers, risks, and owners\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if _, err := service.Run(RunInput{
		AllowGeneratedTitles: true,
		Config:               map[string]any{"task_sources": []string{"workspace/todos"}},
	}); err != nil {
		t.Fatalf("third Run returned error: %v", err)
	}
	if got := callCount.Load(); got != 2 {
		t.Fatalf("expected changed note content to refresh generated title, got %d calls", got)
	}
}

func TestServiceRunLimitsGeneratedNoteTitlesPerManualPass(t *testing.T) {
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
		"  note: review blockers and next steps",
		"- [ ] Release checklist",
		"  note: verify owners and rollback steps",
		"- [ ] Hiring sync",
		"  note: gather open questions and decisions",
		"- [ ] Infra backlog",
		"  note: clean stale alerts and ticket links",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "weekly.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	callCount := &atomic.Int32{}
	service := NewService(fileSystem).WithTitleGenerator(titlegen.NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"模型标题"}`,
		calls:  callCount,
	})))

	result, err := service.Run(RunInput{
		AllowGeneratedTitles: true,
		Config:               map[string]any{"task_sources": []string{"workspace/todos"}},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := callCount.Load(); got != defaultGeneratedTitleLimit {
		t.Fatalf("expected manual generation budget %d, got %d", defaultGeneratedTitleLimit, got)
	}
	if len(result.NotepadItems) != 4 {
		t.Fatalf("expected four parsed notes, got %+v", result.NotepadItems)
	}
	if result.NotepadItems[3]["title"] == "模型标题" {
		t.Fatalf("expected notes beyond the generation budget to keep local fallback titles, got %+v", result.NotepadItems[3])
	}
}

func TestServiceRunDoesNotSpendGenerationBudgetOnFallbackOnlyNotes(t *testing.T) {
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
		"- [ ] plain checklist one",
		"- [ ] plain checklist two",
		"- [ ] plain checklist three",
		"- [ ] Weekly retro",
		"  note: review blockers and next steps",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "mixed.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	callCount := &atomic.Int32{}
	service := NewService(fileSystem).WithTitleGenerator(titlegen.NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"每周复盘阻塞项"}`,
		calls:  callCount,
	})))

	result, err := service.Run(RunInput{
		AllowGeneratedTitles: true,
		Config:               map[string]any{"task_sources": []string{"workspace/todos"}},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := callCount.Load(); got != 1 {
		t.Fatalf("expected only the rich-context note to consume generation budget, got %d calls", got)
	}
	if len(result.NotepadItems) != 4 {
		t.Fatalf("expected four parsed notes, got %+v", result.NotepadItems)
	}
	if result.NotepadItems[3]["title"] != "每周复盘阻塞项" {
		t.Fatalf("expected later rich-context note to still receive generated title, got %+v", result.NotepadItems[3])
	}
}

func TestServiceRunDoesNotSpendGenerationBudgetOnCachedNoteTitles(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("NewLocalPathPolicy returned error: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "todos"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	notePath := filepath.Join(workspaceRoot, "todos", "weekly.md")
	initialContent := strings.Join([]string{
		"- [ ] Weekly retro",
		"  note: review blockers and next steps",
		"- [ ] Release checklist",
		"  note: verify owners and rollback steps",
		"- [ ] Hiring sync",
		"  note: gather open questions and decisions",
	}, "\n")
	if err := os.WriteFile(notePath, []byte(initialContent), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	callCount := &atomic.Int32{}
	service := NewService(fileSystem).WithTitleGenerator(titlegen.NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"模型标题"}`,
		calls:  callCount,
	})))

	if _, err := service.Run(RunInput{
		AllowGeneratedTitles: true,
		Config:               map[string]any{"task_sources": []string{"workspace/todos"}},
	}); err != nil {
		t.Fatalf("first Run returned error: %v", err)
	}
	if got := callCount.Load(); got != defaultGeneratedTitleLimit {
		t.Fatalf("expected first pass to consume budget on the first %d notes, got %d", defaultGeneratedTitleLimit, got)
	}

	updatedContent := initialContent + "\n- [ ] Infra backlog\n  note: clean stale alerts and ticket links\n"
	if err := os.WriteFile(notePath, []byte(updatedContent), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	result, err := service.Run(RunInput{
		AllowGeneratedTitles: true,
		Config:               map[string]any{"task_sources": []string{"workspace/todos"}},
	})
	if err != nil {
		t.Fatalf("second Run returned error: %v", err)
	}
	if got := callCount.Load(); got != defaultGeneratedTitleLimit+1 {
		t.Fatalf("expected cached titles to preserve budget for one new note, got %d calls", got)
	}
	if len(result.NotepadItems) != 4 {
		t.Fatalf("expected four parsed notes, got %+v", result.NotepadItems)
	}
	if result.NotepadItems[3]["title"] != "模型标题" {
		t.Fatalf("expected later uncached note to still receive generated title, got %+v", result.NotepadItems[3])
	}
}

func TestServiceRunUsesNoteTextAsFallbackWhenGeneratorUnavailable(t *testing.T) {
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
		"  note: 补齐风险项、责任人和发布时间",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "weekly.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	service := NewService(fileSystem)
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(result.NotepadItems) != 1 {
		t.Fatalf("expected one parsed note, got %+v", result.NotepadItems)
	}
	if got := result.NotepadItems[0]["title"]; got != "补齐风险项、责任人和发布时间" {
		t.Fatalf("expected fallback title to prefer note body context, got %+v", got)
	}
}

func TestServiceRunKeepsFallbackTitlesUnlessGenerationIsExplicitlyAllowed(t *testing.T) {
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
		"  note: review blockers",
	}, "\n")
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "weekly.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	callCount := &atomic.Int32{}
	service := NewService(fileSystem).WithTitleGenerator(titlegen.NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"每周复盘阻塞项"}`,
		calls:  callCount,
	})))

	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(result.NotepadItems) != 1 || result.NotepadItems[0]["title"] != "review blockers" {
		t.Fatalf("expected local fallback title on default inspection path, got %+v", result.NotepadItems)
	}
	if got := callCount.Load(); got != 0 {
		t.Fatalf("expected default inspection path to skip title model calls, got %d", got)
	}
}

func TestServiceRunSkipsModelForPlainChecklistTitles(t *testing.T) {
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	pathPolicy, err := platform.NewLocalPathPolicy(workspaceRoot)
	if err != nil {
		t.Fatalf("NewLocalPathPolicy returned error: %v", err)
	}
	fileSystem := platform.NewLocalFileSystemAdapter(pathPolicy)
	if err := os.MkdirAll(filepath.Join(workspaceRoot, "todos"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "todos", "simple.md"), []byte("- [ ] review report\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	callCount := &atomic.Int32{}
	service := NewService(fileSystem).WithTitleGenerator(titlegen.NewService(model.NewService(serviceconfig.ModelConfig{}, stubModelClient{
		output: `{"title":"报告复盘"}`,
		calls:  callCount,
	})))
	result, err := service.Run(RunInput{Config: map[string]any{"task_sources": []string{"workspace/todos"}}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(result.NotepadItems) != 1 || result.NotepadItems[0]["title"] != "review report" {
		t.Fatalf("expected plain checklist item to keep direct fallback title, got %+v", result.NotepadItems)
	}
	if got := callCount.Load(); got != 0 {
		t.Fatalf("expected plain checklist item to skip title model call, got %d calls", got)
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
