// Package titlegen generates concise user-facing titles from full task or note
// context while keeping the final task/run contracts deterministic.
package titlegen

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/taskcontext"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/textutil"
)

const (
	defaultTitleLengthLimit    = 24
	defaultNoteTitleCacheLimit = 256
	taskPromptPrimaryLimit     = 320
	taskPromptSecondaryLimit   = 160
	notePromptPrimaryLimit     = 320
	notePromptSecondaryLimit   = 160
)

var defaultGenerationTimeout = 3 * time.Second

// Service keeps title generation behind one model-backed boundary with a small
// deterministic fallback when the model is unavailable.
type Service struct {
	modelMu        sync.RWMutex
	model          *model.Service
	timeout        time.Duration
	maxTitle       int
	noteTitleMu    sync.RWMutex
	noteTitleCache map[string]string
}

// GenerationOwner carries the formal task/run ownership that a title
// generation call must preserve for provider attribution and downstream audit.
type GenerationOwner struct {
	TaskID string
	RunID  string
}

// TaskSubjectResult captures the user-facing title plus the model invocation
// metadata needed when callers must audit or account for the generation cost.
type TaskSubjectResult struct {
	Title      string
	Generated  bool
	Invocation *model.InvocationRecord
}

// NoteTitleResult mirrors TaskSubjectResult for note-title generation so
// caller-controlled flows can persist ownership, audit, and token accounting.
type NoteTitleResult struct {
	Title      string
	Generated  bool
	Invocation *model.InvocationRecord
}

// NewService creates a title generator around the current runtime model.
func NewService(modelService *model.Service) *Service {
	return &Service{
		model:          modelService,
		timeout:        defaultGenerationTimeout,
		maxTitle:       defaultTitleLengthLimit,
		noteTitleCache: map[string]string{},
	}
}

// ReplaceModel keeps title generation aligned with runtime model changes.
func (s *Service) ReplaceModel(modelService *model.Service) {
	if s == nil {
		return
	}
	s.modelMu.Lock()
	s.model = modelService
	s.modelMu.Unlock()
	s.clearNoteTitleCache()
}

func (s *Service) currentModel() *model.Service {
	if s == nil {
		return nil
	}
	s.modelMu.RLock()
	defer s.modelMu.RUnlock()
	return s.model
}

// GenerateTaskSubject summarizes the full task snapshot into a short final task
// title.
func (s *Service) GenerateTaskSubject(ctx context.Context, owner GenerationOwner, snapshot taskcontext.TaskContextSnapshot, intentName string, fallback string) string {
	return s.GenerateTaskSubjectResult(ctx, owner, snapshot, intentName, fallback).Title
}

// GenerateTaskSubjectResult exposes the task-title model invocation so the
// caller can project the spend back into formal task audit/token accounting.
func (s *Service) GenerateTaskSubjectResult(ctx context.Context, owner GenerationOwner, snapshot taskcontext.TaskContextSnapshot, intentName string, fallback string) TaskSubjectResult {
	prompt := buildTaskSubjectPrompt(snapshot, intentName, s.maxTitle)
	title, generated, invocation := s.generate(ctx, owner, prompt, fallback)
	return TaskSubjectResult{
		Title:      title,
		Generated:  generated,
		Invocation: invocation,
	}
}

// CompactTaskFallback keeps the first task-facing title deterministic when the
// runtime model is slow, unavailable, or intentionally disabled.
func CompactTaskFallback(raw string) string {
	return compactFallbackTitle(raw, defaultTitleLengthLimit, true)
}

// CompactNoteFallback keeps note titles bounded and readable without requiring
// a model round-trip in the synchronous inspector path.
func CompactNoteFallback(raw string) string {
	return compactFallbackTitle(raw, defaultTitleLengthLimit, false)
}

// GenerateNoteTitle summarizes note body context into one short dashboard
// label.
func (s *Service) GenerateNoteTitle(ctx context.Context, owner GenerationOwner, item map[string]any, fallback string) string {
	return s.GenerateNoteTitleResult(ctx, owner, item, fallback).Title
}

// GenerateNoteTitleResult preserves the model invocation for callers that must
// account for manual note-title generation outside the task execution loop.
func (s *Service) GenerateNoteTitleResult(ctx context.Context, owner GenerationOwner, item map[string]any, fallback string) NoteTitleResult {
	prompt := buildNoteTitlePrompt(item, s.maxTitle)
	cacheKey := noteTitleCacheKey(prompt, fallback, s.maxTitle)
	if title, ok := s.cachedNoteTitle(cacheKey); ok {
		return NoteTitleResult{Title: title}
	}
	title, generated, invocation := s.generate(ctx, owner, prompt, fallback)
	if generated {
		s.storeNoteTitle(cacheKey, title)
	}
	return NoteTitleResult{
		Title:      title,
		Generated:  generated,
		Invocation: invocation,
	}
}

// CachedNoteTitle exposes note-title cache hits so callers can keep manual
// generation budgets aligned with actual model invocations.
func (s *Service) CachedNoteTitle(item map[string]any, fallback string) (string, bool) {
	if s == nil {
		return "", false
	}
	prompt := buildNoteTitlePrompt(item, s.maxTitle)
	cacheKey := noteTitleCacheKey(prompt, fallback, s.maxTitle)
	return s.cachedNoteTitle(cacheKey)
}

func (s *Service) generate(ctx context.Context, owner GenerationOwner, prompt string, fallback string) (string, bool, *model.InvocationRecord) {
	fallback = normalizeTitle(fallback, s.maxTitle)
	if strings.TrimSpace(prompt) == "" {
		return fallback, false, nil
	}
	modelService := s.currentModel()
	if modelService == nil {
		return fallback, false, nil
	}
	generationCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	owner = normalizedGenerationOwner(owner)
	response, err := modelService.GenerateText(generationCtx, model.GenerateTextRequest{
		TaskID: owner.TaskID,
		RunID:  owner.RunID,
		Input:  prompt,
	})
	if err != nil {
		return fallback, false, nil
	}
	invocation := response.InvocationRecord()
	if title := parseGeneratedTitle(response.OutputText, s.maxTitle); title != "" {
		return title, true, &invocation
	}
	return fallback, false, &invocation
}

func normalizedGenerationOwner(owner GenerationOwner) GenerationOwner {
	taskID := strings.TrimSpace(owner.TaskID)
	runID := strings.TrimSpace(owner.RunID)
	if taskID == "" && runID == "" {
		return GenerationOwner{
			TaskID: "title_generation",
			RunID:  "title_generation",
		}
	}
	if taskID == "" {
		taskID = runID
	}
	if runID == "" {
		runID = taskID
	}
	return GenerationOwner{
		TaskID: taskID,
		RunID:  runID,
	}
}

func buildTaskSubjectPrompt(snapshot taskcontext.TaskContextSnapshot, intentName string, maxLength int) string {
	lines := []string{
		"You generate one compact task title subject for a desktop agent task.",
		"Use the full context, not just the first sentence.",
		"Return JSON only.",
		`Schema: {"title":"..."}`,
		"Rules:",
		"- Keep the title natural, specific, and under the visible character limit.",
		"- Return the final title text directly. Do not add labels like 处理：, 翻译：, 总结：, explain:, or todo:.",
		"- Do not copy filler like 请帮我, 帮我, 我想, summarize, translate, review, todo, note.",
		"- Do not invent goals that are not present in the input.",
		"- Prefer the real object, deliverable, or topic the user wants handled.",
		"",
		"Title body max visible characters:",
		strconv.Itoa(maxLength),
		"",
		"Intent:",
		firstNonEmpty(intentName, "agent_loop"),
		"",
		"Context:",
		taskSnapshotSummary(snapshot, intentName),
	}
	return strings.Join(lines, "\n")
}

func buildNoteTitlePrompt(item map[string]any, maxLength int) string {
	lines := []string{
		"You generate one compact note title for a desktop dashboard item.",
		"Use the full note context, not only the first checklist line.",
		"Return JSON only.",
		`Schema: {"title":"..."}`,
		"Rules:",
		"- Keep the title natural, specific, and under the visible character limit.",
		"- Prefer the actual work item or topic over generic wrappers.",
		"- Do not output markdown bullets, prefixes, or surrounding quotes.",
		"",
		"Title max visible characters:",
		strconv.Itoa(maxLength),
		"",
		"Note context:",
		notepadItemSummary(item),
	}
	return strings.Join(lines, "\n")
}

func taskSnapshotSummary(snapshot taskcontext.TaskContextSnapshot, intentName string) string {
	lines := make([]string, 0, 12)
	appendLine := func(label string, value string, maxLength int) {
		if value = budgetPromptValue(value, maxLength); value != "" {
			lines = append(lines, label+": "+value)
		}
	}
	appendLine("input_type", snapshot.InputType, taskPromptSecondaryLimit)
	appendLine("text", snapshot.Text, taskPromptPrimaryLimit)
	appendLine("selection_text", snapshot.SelectionText, taskPromptPrimaryLimit)
	appendLine("error_text", snapshot.ErrorText, taskPromptPrimaryLimit)
	if len(snapshot.Files) > 0 {
		appendLine("files", strings.Join(snapshot.Files, ", "), taskPromptSecondaryLimit)
	}
	appendLine("page_title", snapshot.PageTitle, taskPromptSecondaryLimit)
	appendLine("window_title", snapshot.WindowTitle, taskPromptSecondaryLimit)
	if strings.TrimSpace(intentName) == "screen_analyze" {
		// Ambient OCR/page text only belongs to explicit screen-analysis flows.
		appendLine("screen_summary", snapshot.ScreenSummary, taskPromptSecondaryLimit)
		appendLine("visible_text", snapshot.VisibleText, taskPromptPrimaryLimit)
	}
	appendLine("hover_target", snapshot.HoverTarget, taskPromptSecondaryLimit)
	return strings.Join(lines, "\n")
}

func notepadItemSummary(item map[string]any) string {
	lines := make([]string, 0, 6)
	appendLine := func(label string, value string, maxLength int) {
		if value = budgetPromptValue(value, maxLength); value != "" {
			lines = append(lines, label+": "+value)
		}
	}
	appendLine("title", stringValue(item, "title"), notePromptSecondaryLimit)
	appendLine("note_text", stringValue(item, "note_text"), notePromptPrimaryLimit)
	appendLine("agent_suggestion", stringValue(item, "agent_suggestion"), notePromptSecondaryLimit)
	appendLine("prerequisite", stringValue(item, "prerequisite"), notePromptSecondaryLimit)
	return strings.Join(lines, "\n")
}

func parseGeneratedTitle(raw string, maxLength int) string {
	payload := extractJSONObject(raw)
	if payload != "" {
		var decoded struct {
			Title string `json:"title"`
		}
		if err := json.Unmarshal([]byte(payload), &decoded); err == nil {
			return normalizeTitle(decoded.Title, maxLength)
		}
	}
	return normalizeTitle(raw, maxLength)
}

func extractJSONObject(raw string) string {
	trimmed := strings.TrimSpace(raw)
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end < start {
		return ""
	}
	return trimmed[start : end+1]
}

func normalizeTitle(value string, maxLength int) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "```json")
	value = strings.TrimPrefix(value, "```")
	value = strings.TrimSuffix(value, "```")
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"'`")
	value = strings.Join(strings.Fields(value), " ")
	return textutil.TruncateGraphemes(value, maxLength)
}

func compactFallbackTitle(raw string, maxLength int, trimLeadIn bool) string {
	segments := titleSegments(raw, trimLeadIn)
	if len(segments) == 0 {
		return normalizeTitle(raw, maxLength)
	}
	candidate := strings.Join(segments[:minInt(len(segments), 2)], " ")
	return normalizeTitle(candidate, maxLength)
}

func titleSegments(raw string, trimLeadIn bool) []string {
	replacer := strings.NewReplacer("\r\n", "\n", "\r", "\n", "。", "\n", "！", "\n", "？", "\n", "；", "\n", ";", "\n", "\t", " ")
	parts := strings.Split(replacer.Replace(raw), "\n")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		segment := strings.TrimSpace(part)
		if segment == "" {
			continue
		}
		segment = stripChecklistPrefix(segment)
		if trimLeadIn {
			segment = trimTaskLeadIn(segment)
		}
		segment = strings.Join(strings.Fields(segment), " ")
		if segment == "" {
			continue
		}
		if len(segments) > 0 && segments[len(segments)-1] == segment {
			continue
		}
		segments = append(segments, segment)
	}
	return segments
}

func stripChecklistPrefix(value string) string {
	value = strings.TrimSpace(value)
	for _, prefix := range []string{"- [ ] ", "- [x] ", "- [X] ", "* [ ] ", "* [x] ", "* [X] ", "- ", "* "} {
		if strings.HasPrefix(value, prefix) {
			return strings.TrimSpace(value[len(prefix):])
		}
	}
	return value
}

func trimTaskLeadIn(value string) string {
	trimmed := strings.TrimSpace(value)
	for _, prefix := range []string{
		"请你帮我", "请帮我", "帮我", "麻烦你", "麻烦", "请你", "请", "我想让你", "我想请你", "我需要你",
		"please help me", "help me", "can you", "please",
	} {
		if strings.HasPrefix(strings.ToLower(trimmed), prefix) {
			trimmed = strings.TrimSpace(trimmed[len(prefix):])
			break
		}
	}
	return trimmed
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func budgetPromptValue(value string, maxLength int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if value == "" {
		return ""
	}
	if maxLength <= 0 {
		return value
	}
	return textutil.TruncateGraphemes(value, maxLength)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stringValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func noteTitleCacheKey(prompt string, fallback string, maxLength int) string {
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(prompt))
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write([]byte(fallback))
	_, _ = hasher.Write([]byte{0})
	_, _ = hasher.Write([]byte(strconv.Itoa(maxLength)))
	return strconv.FormatUint(hasher.Sum64(), 16)
}

func (s *Service) cachedNoteTitle(cacheKey string) (string, bool) {
	if s == nil || cacheKey == "" {
		return "", false
	}
	s.noteTitleMu.RLock()
	defer s.noteTitleMu.RUnlock()
	title, ok := s.noteTitleCache[cacheKey]
	return title, ok
}

func (s *Service) storeNoteTitle(cacheKey string, title string) {
	if s == nil || cacheKey == "" || title == "" {
		return
	}
	s.noteTitleMu.Lock()
	defer s.noteTitleMu.Unlock()
	if len(s.noteTitleCache) >= defaultNoteTitleCacheLimit {
		// Note titles are regenerated from source-of-truth note content, so
		// clearing the bounded memo is safer than allowing unbounded growth during
		// long-lived inspection sessions.
		s.noteTitleCache = map[string]string{}
	}
	s.noteTitleCache[cacheKey] = title
}

func (s *Service) clearNoteTitleCache() {
	if s == nil {
		return
	}
	s.noteTitleMu.Lock()
	s.noteTitleCache = map[string]string{}
	s.noteTitleMu.Unlock()
}
