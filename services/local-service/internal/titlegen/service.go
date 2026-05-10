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
	taskTitleRequestID         = "task_title_generator"
	noteTitleRequestID         = "note_title_generator"
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
func (s *Service) GenerateTaskSubject(ctx context.Context, snapshot taskcontext.TaskContextSnapshot, intentName string, fallback string) string {
	prompt := buildTaskSubjectPrompt(snapshot, intentName, s.maxTitle)
	title, _ := s.generate(ctx, taskTitleRequestID, prompt, fallback)
	return title
}

// GenerateNoteTitle summarizes note body context into one short dashboard
// label.
func (s *Service) GenerateNoteTitle(ctx context.Context, item map[string]any, fallback string) string {
	prompt := buildNoteTitlePrompt(item, s.maxTitle)
	cacheKey := noteTitleCacheKey(prompt, fallback, s.maxTitle)
	if title, ok := s.cachedNoteTitle(cacheKey); ok {
		return title
	}
	title, generated := s.generate(ctx, noteTitleRequestID, prompt, fallback)
	if generated {
		s.storeNoteTitle(cacheKey, title)
	}
	return title
}

func (s *Service) generate(ctx context.Context, requestID string, prompt string, fallback string) (string, bool) {
	fallback = normalizeTitle(fallback, s.maxTitle)
	if strings.TrimSpace(prompt) == "" {
		return fallback, false
	}
	modelService := s.currentModel()
	if modelService == nil {
		return fallback, false
	}
	generationCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	response, err := modelService.GenerateText(generationCtx, model.GenerateTextRequest{
		TaskID: requestID,
		RunID:  requestID,
		Input:  prompt,
	})
	if err != nil {
		return fallback, false
	}
	if title := parseGeneratedTitle(response.OutputText, s.maxTitle); title != "" {
		return title, true
	}
	return fallback, false
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
		taskSnapshotSummary(snapshot),
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

func taskSnapshotSummary(snapshot taskcontext.TaskContextSnapshot) string {
	lines := make([]string, 0, 12)
	appendLine := func(label string, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			lines = append(lines, label+": "+value)
		}
	}
	appendLine("input_type", snapshot.InputType)
	appendLine("text", snapshot.Text)
	appendLine("selection_text", snapshot.SelectionText)
	appendLine("error_text", snapshot.ErrorText)
	if len(snapshot.Files) > 0 {
		lines = append(lines, "files: "+strings.Join(snapshot.Files, ", "))
	}
	appendLine("page_title", snapshot.PageTitle)
	appendLine("window_title", snapshot.WindowTitle)
	appendLine("screen_summary", snapshot.ScreenSummary)
	appendLine("visible_text", snapshot.VisibleText)
	appendLine("hover_target", snapshot.HoverTarget)
	return strings.Join(lines, "\n")
}

func notepadItemSummary(item map[string]any) string {
	lines := make([]string, 0, 6)
	appendLine := func(label string, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			lines = append(lines, label+": "+value)
		}
	}
	appendLine("title", stringValue(item, "title"))
	appendLine("note_text", stringValue(item, "note_text"))
	appendLine("agent_suggestion", stringValue(item, "agent_suggestion"))
	appendLine("prerequisite", stringValue(item, "prerequisite"))
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
