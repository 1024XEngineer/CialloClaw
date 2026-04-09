// 该文件负责任务巡检模块的最小运行态聚合逻辑。
package taskinspector

import (
	"fmt"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
)

const defaultStaleInterval = 15 * time.Minute

// Service 负责根据 workspace、notepad 和 runtime task 状态生成巡检结果。
type Service struct {
	fileSystem platform.FileSystemAdapter
	now        func() time.Time
}

// RunInput 描述一次巡检执行所需的运行态输入。
type RunInput struct {
	Reason          string
	TargetSources   []string
	Config          map[string]any
	UnfinishedTasks []runengine.TaskRecord
	FinishedTasks   []runengine.TaskRecord
	NotepadItems    []map[string]any
}

// RunResult 描述一次巡检执行输出的协议兼容结果。
type RunResult struct {
	InspectionID string
	Summary      map[string]any
	Suggestions  []string
}

// NewService 创建并返回 task inspector 服务。
func NewService(fileSystem platform.FileSystemAdapter) *Service {
	return &Service{
		fileSystem: fileSystem,
		now:        time.Now,
	}
}

// Run 执行一次最小真实巡检。
func (s *Service) Run(input RunInput) RunResult {
	sources := resolveSources(input.TargetSources, input.Config)
	parsedFiles, fileItems := s.inspectSources(sources)
	dueToday, overdue := countDueBuckets(input.NotepadItems)
	staleCount := countStaleTasks(input.UnfinishedTasks, inspectionDuration(input.Config), s.now())
	identifiedItems := fileItems + countOpenNotepadItems(input.NotepadItems)

	return RunResult{
		InspectionID: fmt.Sprintf("insp_%d", s.now().UnixNano()),
		Summary: map[string]any{
			"parsed_files":     parsedFiles,
			"identified_items": identifiedItems,
			"due_today":        dueToday,
			"overdue":          overdue,
			"stale":            staleCount,
		},
		Suggestions: buildSuggestions(input.NotepadItems, input.UnfinishedTasks, sources, parsedFiles, dueToday, overdue, staleCount, fileItems),
	}
}

func (s *Service) inspectSources(sources []string) (int, int) {
	if s.fileSystem == nil || len(sources) == 0 {
		return 0, 0
	}

	parsedFiles := 0
	identifiedItems := 0
	seenFiles := map[string]struct{}{}

	for _, source := range sources {
		root := sourceToFSPath(source)
		if root == "" {
			continue
		}

		_ = fs.WalkDir(s.fileSystem, root, func(currentPath string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry == nil || entry.IsDir() {
				return nil
			}
			if _, seen := seenFiles[currentPath]; seen {
				return nil
			}
			seenFiles[currentPath] = struct{}{}
			parsedFiles++

			content, err := fs.ReadFile(s.fileSystem, currentPath)
			if err != nil {
				return nil
			}
			identifiedItems += countChecklistItems(string(content))
			return nil
		})
	}

	return parsedFiles, identifiedItems
}

func resolveSources(targetSources []string, config map[string]any) []string {
	if len(targetSources) > 0 {
		return dedupeNonEmptyStrings(targetSources)
	}

	rawSources, ok := config["task_sources"].([]string)
	if ok {
		return dedupeNonEmptyStrings(rawSources)
	}

	anySources, ok := config["task_sources"].([]any)
	if !ok {
		return nil
	}

	sources := make([]string, 0, len(anySources))
	for _, rawSource := range anySources {
		source, ok := rawSource.(string)
		if ok && strings.TrimSpace(source) != "" {
			sources = append(sources, source)
		}
	}

	return dedupeNonEmptyStrings(sources)
}

func dedupeNonEmptyStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func sourceToFSPath(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}

	source = strings.TrimPrefix(source, "workspace/")
	if source == "workspace" || source == "." || source == "/" {
		return "."
	}

	normalized := path.Clean(strings.TrimPrefix(source, "/"))
	if normalized == "." {
		return "."
	}
	if strings.HasPrefix(normalized, "../") || normalized == ".." {
		return ""
	}
	return normalized
}

func countChecklistItems(content string) int {
	lines := strings.Split(content, "\n")
	count := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "- [ ]"),
			strings.HasPrefix(trimmed, "* [ ]"),
			strings.HasPrefix(trimmed, "- [x]"),
			strings.HasPrefix(trimmed, "* [x]"),
			strings.HasPrefix(trimmed, "- [X]"),
			strings.HasPrefix(trimmed, "* [X]"):
			count++
		}
	}
	return count
}

func countOpenNotepadItems(items []map[string]any) int {
	count := 0
	for _, item := range items {
		status := stringValue(item, "status")
		if status == "completed" || status == "cancelled" {
			continue
		}
		count++
	}
	return count
}

func countDueBuckets(items []map[string]any) (int, int) {
	dueToday := 0
	overdue := 0
	for _, item := range items {
		switch stringValue(item, "status") {
		case "due_today":
			dueToday++
		case "overdue":
			overdue++
		}
	}
	return dueToday, overdue
}

func countStaleTasks(tasks []runengine.TaskRecord, interval time.Duration, now time.Time) int {
	count := 0
	for _, task := range tasks {
		if task.Status == "waiting_auth" || task.Status == "paused" || task.Status == "processing" || task.Status == "waiting_input" || task.Status == "blocked" || task.Status == "confirming_intent" {
			if now.Sub(task.UpdatedAt) >= interval {
				count++
			}
		}
	}
	return count
}

func inspectionDuration(config map[string]any) time.Duration {
	interval, ok := config["inspection_interval"].(map[string]any)
	if !ok {
		return defaultStaleInterval
	}

	value, ok := interval["value"].(int)
	if !ok {
		floatValue, ok := interval["value"].(float64)
		if !ok || floatValue <= 0 {
			return defaultStaleInterval
		}
		value = int(floatValue)
	}
	if value <= 0 {
		return defaultStaleInterval
	}

	switch unit := stringValue(interval, "unit"); unit {
	case "minute":
		return time.Duration(value) * time.Minute
	case "hour":
		return time.Duration(value) * time.Hour
	case "day":
		return time.Duration(value) * 24 * time.Hour
	case "week":
		return time.Duration(value) * 7 * 24 * time.Hour
	default:
		return defaultStaleInterval
	}
}

func buildSuggestions(notepadItems []map[string]any, unfinishedTasks []runengine.TaskRecord, sources []string, parsedFiles, dueToday, overdue, staleCount, fileItems int) []string {
	suggestions := make([]string, 0, 4)

	if overdueTitle := firstNotepadTitleByStatus(notepadItems, "overdue"); overdueTitle != "" {
		suggestions = append(suggestions, fmt.Sprintf("优先处理逾期待办：%s", overdueTitle))
	}
	if dueToday > 0 {
		title := firstNotepadTitleByStatus(notepadItems, "due_today")
		if title != "" {
			suggestions = append(suggestions, fmt.Sprintf("今天到期的待办建议先推进：%s", title))
		}
	}
	if staleCount > 0 {
		title := firstStaleTaskTitle(unfinishedTasks)
		if title != "" {
			suggestions = append(suggestions, fmt.Sprintf("有任务超过巡检窗口未更新，建议先复查：%s", title))
		} else {
			suggestions = append(suggestions, fmt.Sprintf("有 %d 个任务超过巡检窗口未更新，建议优先复查。", staleCount))
		}
	}
	if parsedFiles == 0 && len(sources) > 0 {
		suggestions = append(suggestions, fmt.Sprintf("当前巡检源未发现可解析文件，建议检查 %s。", sources[0]))
	}
	if fileItems > 0 {
		suggestions = append(suggestions, fmt.Sprintf("已从巡检源识别 %d 条候选事项，可继续转为任务。", fileItems))
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "当前未发现高优先级异常，建议继续保持定期巡检。")
	}

	return suggestions
}

func firstNotepadTitleByStatus(items []map[string]any, status string) string {
	for _, item := range items {
		if stringValue(item, "status") == status {
			return stringValue(item, "title")
		}
	}
	return ""
}

func firstStaleTaskTitle(tasks []runengine.TaskRecord) string {
	if len(tasks) == 0 {
		return ""
	}

	staleTitle := ""
	staleUpdatedAt := time.Time{}
	for _, task := range tasks {
		if task.UpdatedAt.IsZero() {
			continue
		}
		if staleTitle == "" || task.UpdatedAt.Before(staleUpdatedAt) {
			staleTitle = task.Title
			staleUpdatedAt = task.UpdatedAt
		}
	}
	return staleTitle
}

func stringValue(values map[string]any, key string) string {
	rawValue, ok := values[key]
	if !ok {
		return ""
	}
	value, ok := rawValue.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}
