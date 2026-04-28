// Package taskinspector aggregates the minimal runtime state used by task
// inspection flows.
package taskinspector

import (
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/textdecode"
)

const defaultStaleInterval = 15 * time.Minute

const (
	notepadBucketClosed        = "closed"
	notepadBucketLater         = "later"
	notepadBucketRecurringRule = "recurring_rule"
	notepadBucketUpcoming      = "upcoming"
)

// Service builds inspection results from workspace, notepad, and runtime task state.
type Service struct {
	fileSystem platform.FileSystemAdapter
	now        func() time.Time
}

// RunInput carries runtime state required by one inspection pass.
type RunInput struct {
	Reason          string
	TargetSources   []string
	Config          map[string]any
	UnfinishedTasks []runengine.TaskRecord
	FinishedTasks   []runengine.TaskRecord
	NotepadItems    []map[string]any
}

// RunResult carries the protocol-compatible result of one inspection pass.
type RunResult struct {
	InspectionID string
	Summary      map[string]any
	Suggestions  []string
	NotepadItems []map[string]any
	SourceSynced bool
}

var (
	// ErrInspectionFileSystemUnavailable reports that inspection sources were
	// provided but no workspace-bound filesystem is available to read them.
	ErrInspectionFileSystemUnavailable = errors.New("task inspection file system unavailable")
	// ErrInspectionSourceOutsideWorkspace reports that one configured source tries
	// to escape the current workspace boundary.
	ErrInspectionSourceOutsideWorkspace = errors.New("task inspection source outside workspace")
	// ErrInspectionSourceNotFound reports that the configured source path does not
	// exist under the current runtime workspace root.
	ErrInspectionSourceNotFound = errors.New("task inspection source not found")
	// ErrInspectionSourceUnreadable reports that the source exists but runtime
	// traversal or file reads failed.
	ErrInspectionSourceUnreadable = errors.New("task inspection source unreadable")
)

// NewService creates a task inspector service.
func NewService(fileSystem platform.FileSystemAdapter) *Service {
	return &Service{
		fileSystem: fileSystem,
		now:        time.Now,
	}
}

// Run executes one inspection pass and fails explicitly when configured sources
// cannot be read from the current runtime workspace.
func (s *Service) Run(input RunInput) (RunResult, error) {
	sources := resolveSources(input.TargetSources, input.Config)
	if len(sources) > 0 && s.fileSystem == nil {
		return RunResult{}, ErrInspectionFileSystemUnavailable
	}
	sourceSynced := len(sources) > 0 && s.fileSystem != nil
	parsedFiles, parsedNotepadItems, err := s.inspectSources(sources)
	if err != nil {
		return RunResult{}, err
	}
	resolvedNotepadItems := cloneMapSlice(input.NotepadItems)
	if sourceSynced {
		resolvedNotepadItems = cloneMapSlice(parsedNotepadItems)
	}
	fileItems := countOpenNotepadItems(parsedNotepadItems)
	dueToday, overdue := countDueBuckets(resolvedNotepadItems, s.now())
	staleCount := countStaleTasks(input.UnfinishedTasks, inspectionDuration(input.Config), s.now())
	identifiedItems := countOpenNotepadItems(resolvedNotepadItems)

	return RunResult{
		InspectionID: fmt.Sprintf("insp_%d", s.now().UnixNano()),
		Summary: map[string]any{
			"parsed_files":     parsedFiles,
			"identified_items": identifiedItems,
			"due_today":        dueToday,
			"overdue":          overdue,
			"stale":            staleCount,
		},
		Suggestions:  buildSuggestions(resolvedNotepadItems, input.UnfinishedTasks, sources, parsedFiles, dueToday, overdue, staleCount, fileItems),
		NotepadItems: cloneMapSlice(resolvedNotepadItems),
		SourceSynced: sourceSynced,
	}, nil
}

func (s *Service) inspectSources(sources []string) (int, []map[string]any, error) {
	if s.fileSystem == nil || len(sources) == 0 {
		return 0, nil, nil
	}

	parsedFiles := 0
	identifiedItems := make([]map[string]any, 0)
	seenFiles := map[string]struct{}{}

	for _, source := range sources {
		root, err := sourceToFSPath(s.fileSystem, source)
		if err != nil {
			return 0, nil, fmt.Errorf("%w: %s", err, strings.TrimSpace(source))
		}
		if _, err := fs.Stat(s.fileSystem, root); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return 0, nil, fmt.Errorf("%w: %s", ErrInspectionSourceNotFound, strings.TrimSpace(source))
			}
			return 0, nil, fmt.Errorf("%w: %s: %v", ErrInspectionSourceUnreadable, strings.TrimSpace(source), err)
		}

		if err := fs.WalkDir(s.fileSystem, root, func(currentPath string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry == nil || entry.IsDir() {
				if walkErr != nil {
					return walkErr
				}
				return nil
			}
			if _, seen := seenFiles[currentPath]; seen {
				return nil
			}
			seenFiles[currentPath] = struct{}{}
			if shouldSkipTaskSourceAttachment(currentPath) {
				return nil
			}
			if !isSupportedTextTaskSourceFile(currentPath) {
				return nil
			}
			parsedFiles++

			content, err := fs.ReadFile(s.fileSystem, currentPath)
			if err != nil {
				return err
			}
			decoded, err := textdecode.Decode(content)
			if err != nil {
				if shouldSkipUnreadableTaskSourceFile(currentPath) {
					return nil
				}
				return fmt.Errorf("decode task source file %s: %w", currentPath, err)
			}
			identifiedItems = append(identifiedItems, parseNotepadItemsFromMarkdown(sourcePathFromFSPath(currentPath), decoded.Text, s.now())...)
			return nil
		}); err != nil {
			return 0, nil, fmt.Errorf("%w: %s: %v", ErrInspectionSourceUnreadable, strings.TrimSpace(source), err)
		}
	}

	return parsedFiles, identifiedItems, nil
}

func shouldSkipTaskSourceAttachment(currentPath string) bool {
	switch strings.ToLower(path.Ext(currentPath)) {
	case ".7z", ".avi", ".bin", ".bmp", ".class", ".dll", ".doc", ".docx", ".dylib", ".exe", ".gif", ".gz", ".ico", ".jar", ".jpeg", ".jpg", ".mov", ".mp3", ".mp4", ".pdf", ".png", ".ppt", ".pptx", ".rar", ".so", ".tar", ".wav", ".webp", ".xls", ".xlsx", ".zip":
		return true
	default:
		return false
	}
}

func shouldSkipUnreadableTaskSourceFile(currentPath string) bool {
	switch strings.ToLower(path.Ext(currentPath)) {
	case "", ".markdown", ".md", ".text", ".txt":
		return false
	default:
		return true
	}
}

func isSupportedTextTaskSourceFile(currentPath string) bool {
	switch strings.ToLower(path.Ext(currentPath)) {
	case "", ".markdown", ".md", ".text", ".txt":
		return true
	default:
		return false
	}
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

// sourceToFSPath accepts both workspace-virtual paths and legacy absolute paths
// that still point inside the current workspace root.
func sourceToFSPath(fileSystem platform.FileSystemAdapter, source string) (string, error) {
	source = normalizeTaskSourceInput(source)
	if source == "" {
		return "", nil
	}
	trimmedVirtual := strings.TrimPrefix(source, "/")
	switch trimmedVirtual {
	case "", "workspace", ".":
		return ".", nil
	}
	if strings.HasPrefix(trimmedVirtual, "workspace/") {
		normalized := path.Clean(strings.TrimPrefix(trimmedVirtual, "workspace/"))
		if normalized == "." {
			return ".", nil
		}
		if strings.HasPrefix(normalized, "../") || normalized == ".." {
			return "", ErrInspectionSourceOutsideWorkspace
		}
		return normalized, nil
	}
	// The workspace-virtual /workspace/... form has already been handled above.
	// A single-leading-slash path without a bound workspace filesystem is treated
	// as a legacy unix-style absolute path and rejected explicitly instead of
	// flowing into the generic absolute-path branch.
	if fileSystem == nil && strings.HasPrefix(source, "/") && !strings.HasPrefix(source, "//") {
		return "", ErrInspectionSourceOutsideWorkspace
	}
	if strings.HasPrefix(source, "/") && !filepath.IsAbs(source) {
		return "", ErrInspectionSourceOutsideWorkspace
	}

	if isAbsoluteTaskSourcePath(source) {
		if fileSystem == nil {
			return "", ErrInspectionFileSystemUnavailable
		}
		workspaceRoot, err := fileSystem.EnsureWithinWorkspace(".")
		if err != nil {
			return "", ErrInspectionSourceOutsideWorkspace
		}
		relPath, ok := relativeAbsoluteTaskSourcePath(workspaceRoot, source)
		if !ok {
			return "", ErrInspectionSourceOutsideWorkspace
		}
		if filepath.Clean(relPath) == "." {
			return ".", nil
		}
		return relPath, nil
	}

	normalized := path.Clean(strings.TrimPrefix(source, "/"))
	if normalized == "." {
		return ".", nil
	}
	if strings.HasPrefix(normalized, "../") || normalized == ".." {
		return "", ErrInspectionSourceOutsideWorkspace
	}
	return normalized, nil
}

// isAbsoluteTaskSourcePath recognizes both native absolute paths for the
// current GOOS and Windows drive-letter paths so non-Windows tests can still
// validate Windows-style inspection sources such as D:/todos.
func isAbsoluteTaskSourcePath(source string) bool {
	if filepath.IsAbs(source) {
		return true
	}
	if strings.HasPrefix(source, "//") {
		return true
	}
	if len(source) < 3 || source[1] != ':' || source[2] != '/' {
		return false
	}
	driveLetter := source[0]
	return (driveLetter >= 'a' && driveLetter <= 'z') || (driveLetter >= 'A' && driveLetter <= 'Z')
}

func normalizeAbsoluteTaskSourcePath(source string) string {
	if strings.HasPrefix(source, "//") {
		trimmed := strings.TrimPrefix(source, "//")
		cleaned := path.Clean("/" + trimmed)
		return "//" + strings.TrimPrefix(cleaned, "/")
	}
	if filepath.IsAbs(source) {
		return filepath.ToSlash(filepath.Clean(source))
	}
	return path.Clean(source)
}

// normalizeTaskSourceInput keeps the policy in forward-slash form regardless of
// the host GOOS. filepath.ToSlash only rewrites the native separator, so a
// second pass is still required for injected Windows literals on Unix CI.
func normalizeTaskSourceInput(source string) string {
	return strings.ReplaceAll(filepath.ToSlash(strings.TrimSpace(source)), "\\", "/")
}

func relativeAbsoluteTaskSourcePath(workspaceRoot string, source string) (string, bool) {
	normalizedWorkspaceRoot := normalizeAbsoluteTaskSourcePath(normalizeTaskSourceInput(workspaceRoot))
	normalizedSource := normalizeAbsoluteTaskSourcePath(normalizeTaskSourceInput(source))
	if normalizedWorkspaceRoot == "" || normalizedSource == "" {
		return "", false
	}
	caseInsensitive := strings.HasPrefix(normalizedWorkspaceRoot, "//") || (len(normalizedWorkspaceRoot) >= 2 && normalizedWorkspaceRoot[1] == ':')
	if pathsEqualForTaskSource(normalizedSource, normalizedWorkspaceRoot, caseInsensitive) {
		return ".", true
	}
	workspacePrefix := normalizedWorkspaceRoot
	if !strings.HasSuffix(workspacePrefix, "/") {
		workspacePrefix += "/"
	}
	if !pathHasPrefixForTaskSource(normalizedSource, workspacePrefix, caseInsensitive) {
		return "", false
	}
	return strings.TrimPrefix(normalizedSource, workspacePrefix), true
}

func pathsEqualForTaskSource(left string, right string, caseInsensitive bool) bool {
	if caseInsensitive {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func pathHasPrefixForTaskSource(value string, prefix string, caseInsensitive bool) bool {
	if caseInsensitive {
		return strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix))
	}
	return strings.HasPrefix(value, prefix)
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

func parseNotepadItemsFromMarkdown(sourcePath, content string, now time.Time) []map[string]any {
	lines := strings.Split(content, "\n")
	items := make([]map[string]any, 0)
	var current map[string]any
	noteLines := make([]string, 0)
	naturalLines := make([]string, 0)
	naturalStartLine := 0
	pendingNoteSpacer := false
	flushCurrent := func() {
		if current == nil {
			return
		}
		trimmedNoteLines := trimNaturalNotepadBlankLines(noteLines)
		if len(trimmedNoteLines) > 0 && stringValue(current, "note_text") == "" {
			current["note_text"] = strings.Join(trimmedNoteLines, "\n")
		}
		items = append(items, normalizeParsedNotepadItem(current, sourcePath, now))
		current = nil
		noteLines = noteLines[:0]
		pendingNoteSpacer = false
	}
	flushNatural := func() {
		if !hasNaturalNotepadContent(naturalLines) {
			return
		}
		title, noteText := splitNaturalNoteContent(naturalLines)
		if title != "" {
			item := buildSourceBackedNotepadItem(sourcePath, naturalStartLine, title, false, now)
			if noteText != "" {
				item["note_text"] = noteText
			}
			applyNaturalNotepadHints(item, strings.Join(naturalLines, "\n"), now)
			items = append(items, normalizeParsedNotepadItem(item, sourcePath, now))
		}
		naturalLines = naturalLines[:0]
		naturalStartLine = 0
	}

	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Only top-level checklist rows create structured notepad items; indented
		// rows belong to the current body so editor-saved notes round-trip.
		if current != nil || !hasNaturalNotepadContent(naturalLines) {
			checked, title, ok := parseChecklistLine(line)
			if ok {
				flushNatural()
				flushCurrent()
				current = buildSourceBackedNotepadItem(sourcePath, index+1, title, checked, now)
				pendingNoteSpacer = false
				continue
			}
		}
		if current == nil {
			if isNaturalNotepadHeadingLine(line) && hasNaturalNotepadContent(naturalLines) {
				flushNatural()
			}
			naturalLine := normalizeNaturalNotepadLine(line)
			if strings.TrimSpace(naturalLine) == "" {
				if hasNaturalNotepadContent(naturalLines) {
					naturalLines = append(naturalLines, "")
				}
				continue
			}
			if !hasNaturalNotepadContent(naturalLines) {
				naturalStartLine = index + 1
			}
			naturalLines = append(naturalLines, naturalLine)
			continue
		}
		if trimmed == "" {
			if len(noteLines) > 0 {
				noteLines = append(noteLines, "")
			} else {
				// Source notes commonly keep one spacer before metadata. Keep it
				// out of note_text unless a real body line follows.
				pendingNoteSpacer = true
			}
			continue
		}
		if len(noteLines) == 0 && (!pendingNoteSpacer || !isIndentedMarkdownLine(line)) {
			if handled := applyNotepadMetadataLine(current, trimmed, now); handled {
				pendingNoteSpacer = false
				continue
			}
		}
		pendingNoteSpacer = false
		noteLines = append(noteLines, normalizeNotepadChecklistBodyLine(line))
	}
	flushCurrent()
	flushNatural()
	return items
}

func buildSourceBackedNotepadItem(sourcePath string, line int, title string, checked bool, now time.Time) map[string]any {
	return map[string]any{
		"item_id":     buildSourceBackedNotepadID(sourcePath, line, title),
		"title":       title,
		"bucket":      bucketFromSourcePath(sourcePath, checked, false),
		"status":      statusFromChecklist(checked),
		"type":        todoTypeFromChecklist(checked, false),
		"source_path": sourcePath,
		"source_line": line,
		"created_at":  now.UTC().Format(time.RFC3339),
		"updated_at":  now.UTC().Format(time.RFC3339),
	}
}

func parseChecklistLine(line string) (bool, string, bool) {
	trimmed := strings.TrimRight(line, " \t\r")
	if strings.HasPrefix(trimmed, " ") || strings.HasPrefix(trimmed, "\t") {
		return false, "", false
	}
	switch {
	case strings.HasPrefix(trimmed, "- [ ] "), strings.HasPrefix(trimmed, "* [ ] "):
		return false, strings.TrimSpace(trimmed[6:]), true
	case strings.HasPrefix(trimmed, "- [x] "), strings.HasPrefix(trimmed, "* [x] "), strings.HasPrefix(trimmed, "- [X] "), strings.HasPrefix(trimmed, "* [X] "):
		return true, strings.TrimSpace(trimmed[6:]), true
	default:
		return false, "", false
	}
}

func isIndentedMarkdownLine(line string) bool {
	return strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
}

func normalizeNotepadChecklistBodyLine(line string) string {
	trimmedRight := strings.TrimRight(line, " \t\r")
	if strings.HasPrefix(trimmedRight, "  ") {
		return strings.TrimPrefix(trimmedRight, "  ")
	}
	if strings.HasPrefix(trimmedRight, "\t") {
		return strings.TrimPrefix(trimmedRight, "\t")
	}
	return trimmedRight
}

func applyNotepadMetadataLine(item map[string]any, line string, now time.Time) bool {
	key, value, ok := splitMetadataLine(line)
	if !ok {
		return false
	}
	switch key {
	case "due":
		if dueAt := normalizeMetadataTime(value, now); dueAt != "" {
			item["due_at"] = dueAt
			item["planned_at"] = dueAt
		}
	case "bucket":
		item["bucket"] = normalizeBucketValue(value, stringValue(item, "bucket"))
	case "prerequisite":
		item["prerequisite"] = value
	case "suggest", "agent":
		item["agent_suggestion"] = value
	case "repeat":
		item["repeat_rule_text"] = value
		item["bucket"] = notepadBucketRecurringRule
		item["type"] = "recurring"
	case "next":
		if nextOccurrence := normalizeMetadataTime(value, now); nextOccurrence != "" {
			item["next_occurrence_at"] = nextOccurrence
			if stringValue(item, "due_at") == "" {
				item["due_at"] = nextOccurrence
			}
		}
	case "scope":
		item["effective_scope"] = value
	case "status":
		item["recent_instance_status"] = value
	case "resource":
		resources := cloneMapSlice(resourceListValue(item["related_resources"]))
		resources = append(resources, buildSourceResource(item, value))
		item["related_resources"] = resources
	case "tags":
		item["tags"] = splitTagList(value)
	case "note":
		item["note_text"] = value
	case "reminder":
		item["reminder_strategy"] = value
	default:
		return false
	}
	return true
}

func splitMetadataLine(line string) (string, string, bool) {
	parts := strings.SplitN(line, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	key := strings.ToLower(strings.TrimSpace(parts[0]))
	value := strings.TrimSpace(parts[1])
	if key == "" || value == "" {
		return "", "", false
	}
	return key, value, true
}

func normalizeNaturalNotepadLine(line string) string {
	trimmedRight := strings.TrimRight(line, " \t\r")
	trimmed := strings.TrimSpace(trimmedRight)
	if strings.HasPrefix(trimmed, "#") {
		return strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
	}
	return trimmedRight
}

func isNaturalNotepadHeadingLine(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "#") && normalizeNaturalNotepadLine(line) != ""
}

func hasNaturalNotepadContent(lines []string) bool {
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			return true
		}
	}
	return false
}

func splitNaturalNoteContent(lines []string) (string, string) {
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		normalized = append(normalized, normalizeNaturalNotepadLine(line))
	}
	normalized = trimNaturalNotepadBlankLines(normalized)
	if len(normalized) == 0 {
		return "", ""
	}
	title := strings.TrimSpace(normalized[0])
	if len(normalized) == 1 {
		return title, ""
	}
	bodyLines := trimNaturalNotepadBlankLines(normalized[1:])
	return title, strings.Join(bodyLines, "\n")
}

// trimNaturalNotepadBlankLines preserves paragraph gaps inside a natural note
// while removing only the leading and trailing blank boundaries.
func trimNaturalNotepadBlankLines(lines []string) []string {
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines)
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return lines[start:end]
}

// applyNaturalNotepadHints keeps hand-written notes useful without requiring
// users to learn the markdown metadata keys maintained by the compatibility
// layer.
func applyNaturalNotepadHints(item map[string]any, text string, now time.Time) {
	lower := strings.ToLower(text)
	if hasNaturalRepeatHint(lower) {
		item["bucket"] = notepadBucketRecurringRule
		item["type"] = "recurring"
		item["repeat_rule_text"] = strings.TrimSpace(text)
	}
	if hasNaturalLaterHint(lower) && stringValue(item, "bucket") != notepadBucketRecurringRule {
		item["bucket"] = notepadBucketLater
	}
	if dueAt := inferNaturalDueTime(lower, now); dueAt != "" {
		item["due_at"] = dueAt
		item["planned_at"] = dueAt
		if stringValue(item, "bucket") == notepadBucketLater {
			item["bucket"] = notepadBucketUpcoming
		}
	}
}

func hasNaturalRepeatHint(lowerText string) bool {
	for _, hint := range []string{"every ", "daily", "weekly", "monthly", "repeat", "recurring", "每天", "每日", "每周", "每月", "工作日", "定期", "重复"} {
		if strings.Contains(lowerText, hint) {
			return true
		}
	}
	return false
}

func hasNaturalLaterHint(lowerText string) bool {
	for _, hint := range []string{"later", "someday", "backlog", "以后", "稍后", "有空", "之后"} {
		if strings.Contains(lowerText, hint) {
			return true
		}
	}
	return false
}

func inferNaturalDueTime(lowerText string, now time.Time) string {
	switch {
	case strings.Contains(lowerText, "今天") || strings.Contains(lowerText, "today"):
		return naturalDateTime(now, 0, 0, 0)
	case strings.Contains(lowerText, "明天") || strings.Contains(lowerText, "tomorrow"):
		return naturalDateTime(now, 0, 0, 1)
	case strings.Contains(lowerText, "后天"):
		return naturalDateTime(now, 0, 0, 2)
	case strings.Contains(lowerText, "下周") || strings.Contains(lowerText, "next week"):
		return naturalDateTime(now, 0, 0, 7)
	case strings.Contains(lowerText, "下个月") || strings.Contains(lowerText, "next month"):
		return naturalDateTime(now, 0, 1, 0)
	default:
		return ""
	}
}

func naturalDateTime(now time.Time, years, months, days int) string {
	target := now.AddDate(years, months, days)
	return time.Date(target.Year(), target.Month(), target.Day(), now.Hour(), now.Minute(), 0, 0, now.Location()).Format(time.RFC3339)
}

func normalizeParsedNotepadItem(item map[string]any, sourcePath string, now time.Time) map[string]any {
	if stringValue(item, "bucket") == notepadBucketRecurringRule {
		item["type"] = "recurring"
		if stringValue(item, "repeat_rule_text") == "" {
			item["repeat_rule_text"] = "每周重复一次"
		}
		if _, ok := item["recurring_enabled"]; !ok {
			item["recurring_enabled"] = true
		}
		if stringValue(item, "next_occurrence_at") == "" {
			if nextOccurrence := deriveParsedRecurringNextOccurrence(item); nextOccurrence != "" {
				item["next_occurrence_at"] = nextOccurrence
			}
		}
	}
	resources := resourceListValue(item["related_resources"])
	if !hasResourcePath(resources, sourcePath) {
		resources = append(resources, buildSourcePathResource(sourcePath))
	}
	if len(resources) > 0 {
		item["related_resources"] = resources
	}
	if stringValue(item, "note_text") == "" {
		item["note_text"] = stringValue(item, "title")
	}
	if stringValue(item, "planned_at") == "" {
		item["planned_at"] = stringValue(item, "due_at")
	}
	if stringValue(item, "status") == "completed" {
		item["bucket"] = notepadBucketClosed
		if stringValue(item, "ended_at") == "" {
			item["ended_at"] = now.UTC().Format(time.RFC3339)
		}
	}
	return item
}

func buildSourceBackedNotepadID(sourcePath string, line int, title string) string {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(sourcePath))
	_, _ = hasher.Write([]byte("|"))
	_, _ = hasher.Write([]byte(fmt.Sprintf("%d", line)))
	_, _ = hasher.Write([]byte("|"))
	_, _ = hasher.Write([]byte(strings.TrimSpace(title)))
	return fmt.Sprintf("todo_%08x", hasher.Sum32())
}

func sourcePathFromFSPath(fsPath string) string {
	clean := path.Clean(strings.TrimPrefix(fsPath, "./"))
	if clean == "." {
		return "workspace"
	}
	return path.Join("workspace", clean)
}

func bucketFromSourcePath(sourcePath string, closed bool, recurring bool) string {
	if closed {
		return notepadBucketClosed
	}
	normalized := strings.ToLower(sourcePath)
	if recurring || strings.Contains(normalized, "recurring") || strings.Contains(normalized, "weekly") || strings.Contains(normalized, "repeat") {
		return notepadBucketRecurringRule
	}
	if strings.Contains(normalized, "later") || strings.Contains(normalized, "backlog") {
		return notepadBucketLater
	}
	if strings.Contains(normalized, "inbox") || strings.Contains(normalized, "upcoming") || strings.Contains(normalized, "today") || strings.Contains(normalized, "urgent") {
		return notepadBucketUpcoming
	}
	return notepadBucketUpcoming
}

func statusFromChecklist(checked bool) string {
	if checked {
		return "completed"
	}
	return "normal"
}

func todoTypeFromChecklist(_ bool, recurring bool) string {
	if recurring {
		return "recurring"
	}
	return "one_time"
}

func normalizeMetadataTime(value string, now time.Time) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04", "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			if layout == "2006-01-02" {
				parsed = time.Date(parsed.Year(), parsed.Month(), parsed.Day(), now.Hour(), now.Minute(), 0, 0, now.Location())
			}
			return parsed.Format(time.RFC3339)
		}
	}
	return value
}

func normalizeBucketValue(value string, fallback string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case notepadBucketUpcoming, notepadBucketLater, notepadBucketRecurringRule, notepadBucketClosed:
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return fallback
	}
}

func buildSourceResource(item map[string]any, target string) map[string]any {
	target = strings.TrimSpace(target)
	resourceType := "file"
	targetKind := "file"
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		resourceType = "url"
		targetKind = "url"
	} else if !strings.Contains(path.Base(target), ".") || strings.HasSuffix(target, "/") {
		resourceType = "directory"
		targetKind = "folder"
	}
	return map[string]any{
		"id":          stringValue(item, "item_id") + fmt.Sprintf("_resource_%08x", fnvHash(target)),
		"label":       path.Base(strings.TrimSuffix(target, "/")),
		"path":        target,
		"type":        resourceType,
		"target_kind": targetKind,
	}
}

func buildSourcePathResource(sourcePath string) map[string]any {
	return map[string]any{
		"id":          fmt.Sprintf("source_%08x", fnvHash(sourcePath)),
		"label":       path.Base(sourcePath),
		"path":        sourcePath,
		"type":        "file",
		"target_kind": "file",
	}
}

func splitTagList(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func resourceListValue(rawValue any) []map[string]any {
	resources, ok := rawValue.([]map[string]any)
	if ok {
		return resources
	}
	anyResources, ok := rawValue.([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]any, 0, len(anyResources))
	for _, rawResource := range anyResources {
		resource, ok := rawResource.(map[string]any)
		if ok {
			result = append(result, resource)
		}
	}
	return result
}

func hasResourcePath(resources []map[string]any, targetPath string) bool {
	for _, resource := range resources {
		if stringValue(resource, "path") == targetPath {
			return true
		}
	}
	return false
}

func deriveParsedRecurringNextOccurrence(item map[string]any) string {
	base := stringValue(item, "planned_at")
	if base == "" {
		base = stringValue(item, "due_at")
	}
	if base == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, base)
	if err != nil {
		return base
	}
	ruleText := strings.ToLower(stringValue(item, "repeat_rule_text"))
	switch {
	case strings.Contains(ruleText, "2 week"), strings.Contains(ruleText, "两周"):
		parsed = parsed.AddDate(0, 0, 14)
	case strings.Contains(ruleText, "month"), strings.Contains(ruleText, "每月"):
		parsed = parsed.AddDate(0, 1, 0)
	case strings.Contains(ruleText, "day"), strings.Contains(ruleText, "每天"):
		parsed = parsed.AddDate(0, 0, 1)
	default:
		parsed = parsed.AddDate(0, 0, 7)
	}
	return parsed.Format(time.RFC3339)
}

func cloneMapSlice(values []map[string]any) []map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		result = append(result, cloneMap(value))
	}
	return result
}

func cloneMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			result[key] = cloneMap(typed)
		case []map[string]any:
			result[key] = cloneMapSlice(typed)
		case []string:
			result[key] = append([]string(nil), typed...)
		default:
			result[key] = value
		}
	}
	return result
}

func fnvHash(value string) uint32 {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(value))
	return hasher.Sum32()
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

func countDueBuckets(items []map[string]any, now time.Time) (int, int) {
	dueToday := 0
	overdue := 0
	for _, item := range items {
		status := stringValue(item, "status")
		if status == "normal" {
			status = normalizeTodoStatus(stringValue(item, "due_at"), now)
		}
		switch status {
		case "due_today":
			dueToday++
		case "overdue":
			overdue++
		}
	}
	return dueToday, overdue
}

func normalizeTodoStatus(dueAt string, now time.Time) string {
	if strings.TrimSpace(dueAt) == "" {
		return "normal"
	}
	parsed, err := time.Parse(time.RFC3339, dueAt)
	if err != nil {
		return "normal"
	}
	if parsed.Before(now) {
		return "overdue"
	}
	yearNow, monthNow, dayNow := now.Date()
	yearDue, monthDue, dayDue := parsed.Date()
	if yearNow == yearDue && monthNow == monthDue && dayNow == dayDue {
		return "due_today"
	}
	return "normal"
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
