// Package context captures and normalizes task-facing input snapshots.
package context

import "strings"

// TaskContextSnapshot aggregates the normalized request context that the main
// task pipeline uses for intent inference and orchestration.
type TaskContextSnapshot struct {
	Source         string
	Trigger        string
	InputType      string
	InputMode      string
	Text           string
	SelectionText  string
	ErrorText      string
	Files          []string
	PageTitle      string
	PageURL        string
	AppName        string
	BrowserKind    string
	ProcessPath    string
	ProcessID      int
	WindowTitle    string
	VisibleText    string
	ScreenSummary  string
	ClipboardText  string
	HoverTarget    string
	LastAction     string
	DwellMillis    int
	CopyCount      int
	WindowSwitches int
	PageSwitches   int
}

// Service folds JSON-RPC request params into a stable task context object.
// It does not make intent or execution decisions.
type Service struct{}

// NewService constructs a context capture service.
func NewService() *Service {
	return &Service{}
}

// Snapshot returns a minimal service descriptor for bootstrap and debug views.
func (s *Service) Snapshot() map[string]string {
	return map[string]string{"source": "desktop"}
}

// Capture extracts task context from an RPC payload.
// It merges both input.* and context.* fields so downstream services can rely
// on one normalized snapshot shape.
func (s *Service) Capture(params map[string]any) TaskContextSnapshot {
	input := mapValue(params, "input")
	contextValue := mapValue(params, "context")
	selection := mapValue(contextValue, "selection")
	pageContext := mapValue(input, "page_context")
	pageFallback := mapValue(contextValue, "page")
	errorValue := mapValue(contextValue, "error")
	clipboard := mapValue(contextValue, "clipboard")
	screen := mapValue(contextValue, "screen")
	behavior := mapValue(contextValue, "behavior")

	selectionText := firstNonEmpty(
		stringValue(selection, "text"),
		stringValue(contextValue, "selection_text"),
	)
	text := firstNonEmpty(
		stringValue(input, "text"),
		stringValue(contextValue, "text"),
	)
	errorText := firstNonEmpty(
		stringValue(input, "error_message"),
		stringValue(errorValue, "message"),
	)

	files := dedupeStrings(append(
		append(stringSliceValue(input["files"]), stringSliceValue(contextValue["files"])...),
		stringSliceValue(contextValue["file_paths"])...,
	))

	inputType := firstNonEmpty(stringValue(input, "type"), inferInputType(text, selectionText, errorText, files))
	if inputType == "text_selection" && text == "" {
		text = selectionText
	}
	if inputType == "error" && text == "" {
		text = errorText
	}

	return TaskContextSnapshot{
		Source:         stringValue(params, "source"),
		Trigger:        firstNonEmpty(stringValue(params, "trigger"), inferTrigger(inputType, selectionText, errorText, files)),
		InputType:      inputType,
		InputMode:      firstNonEmpty(stringValue(input, "input_mode"), inferInputMode(text)),
		Text:           text,
		SelectionText:  selectionText,
		ErrorText:      errorText,
		Files:          files,
		PageTitle:      firstNonEmpty(stringValue(pageContext, "title"), stringValue(pageFallback, "title")),
		PageURL:        firstNonEmpty(stringValue(pageContext, "url"), stringValue(pageFallback, "url")),
		AppName:        firstNonEmpty(stringValue(pageContext, "app_name"), stringValue(pageFallback, "app_name")),
		BrowserKind:    firstNonEmpty(stringValue(pageContext, "browser_kind"), stringValue(pageFallback, "browser_kind")),
		ProcessPath:    firstNonEmpty(stringValue(pageContext, "process_path"), stringValue(pageFallback, "process_path")),
		ProcessID:      intValue(pageContext, "process_id", intValue(pageFallback, "process_id", 0)),
		WindowTitle:    firstNonEmpty(stringValue(pageContext, "window_title"), stringValue(pageFallback, "window_title"), stringValue(screen, "window_title")),
		VisibleText:    firstNonEmpty(stringValue(pageContext, "visible_text"), stringValue(pageFallback, "visible_text"), stringValue(screen, "visible_text")),
		ScreenSummary:  firstNonEmpty(stringValue(contextValue, "screen_summary"), stringValue(screen, "summary"), stringValue(screen, "screen_summary")),
		ClipboardText:  firstNonEmpty(stringValue(contextValue, "clipboard_text"), stringValue(clipboard, "text")),
		HoverTarget:    firstNonEmpty(stringValue(contextValue, "hover_target"), stringValue(pageContext, "hover_target"), stringValue(pageFallback, "hover_target"), stringValue(screen, "hover_target")),
		LastAction:     firstNonEmpty(stringValue(contextValue, "last_action"), stringValue(behavior, "last_action")),
		DwellMillis:    intValue(contextValue, "dwell_millis", intValue(behavior, "dwell_millis", 0)),
		CopyCount:      intValue(contextValue, "copy_count", intValue(behavior, "copy_count", 0)),
		WindowSwitches: intValue(contextValue, "window_switch_count", intValue(behavior, "window_switch_count", 0)),
		PageSwitches:   intValue(contextValue, "page_switch_count", intValue(behavior, "page_switch_count", 0)),
	}
}

// mapValue safely reads a nested object without leaking repeated assertions to
// callers.
func mapValue(values map[string]any, key string) map[string]any {
	rawValue, ok := values[key]
	if !ok {
		return map[string]any{}
	}

	value, ok := rawValue.(map[string]any)
	if !ok {
		return map[string]any{}
	}

	return value
}

// stringValue safely reads a string field and trims surrounding whitespace.
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

// stringSliceValue converts JSON-decoded string collections into a trimmed,
// deduplicated []string.
func stringSliceValue(rawValue any) []string {
	if values, ok := rawValue.([]string); ok {
		return dedupeStrings(values)
	}

	values, ok := rawValue.([]any)
	if ok {
		result := make([]string, 0, len(values))
		for _, rawItem := range values {
			item, ok := rawItem.(string)
			if ok && strings.TrimSpace(item) != "" {
				result = append(result, strings.TrimSpace(item))
			}
		}
		return dedupeStrings(result)
	}

	if value, ok := rawValue.(string); ok {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return nil
		}
		return []string{trimmed}
	}

	return nil
}

func inferInputType(text, selectionText, errorText string, files []string) string {
	switch {
	case len(files) > 0:
		return "file"
	case strings.TrimSpace(errorText) != "":
		return "error"
	case strings.TrimSpace(selectionText) != "":
		return "text_selection"
	case strings.TrimSpace(text) != "":
		return "text"
	default:
		return ""
	}
}

func inferTrigger(inputType, selectionText, errorText string, files []string) string {
	switch {
	case len(files) > 0:
		return "file_drop"
	case strings.TrimSpace(errorText) != "" || inputType == "error":
		return "error_detected"
	case strings.TrimSpace(selectionText) != "" || inputType == "text_selection":
		return "text_selected_click"
	case inputType == "text":
		return "hover_text_input"
	default:
		return ""
	}
}

func inferInputMode(text string) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	return "text"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func intValue(values map[string]any, key string, fallback int) int {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}
	switch typed := rawValue.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return fallback
	}
}
