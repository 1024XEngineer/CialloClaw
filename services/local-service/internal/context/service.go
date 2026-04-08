// 该文件负责主链路输入上下文的采集与归一化。
package context

type TaskContextSnapshot struct {
	Source        string
	Trigger       string
	InputType     string
	InputMode     string
	Text          string
	SelectionText string
	ErrorText     string
	Files         []string
	PageTitle     string
	PageURL       string
	AppName       string
}

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Snapshot() map[string]string {
	return map[string]string{"source": "desktop"}
}

func (s *Service) Capture(params map[string]any) TaskContextSnapshot {
	input := mapValue(params, "input")
	contextValue := mapValue(params, "context")
	selection := mapValue(contextValue, "selection")
	page := mapValue(input, "page_context")
	if len(page) == 0 {
		page = mapValue(contextValue, "page")
	}

	selectionText := stringValue(selection, "text")
	if selectionText == "" {
		selectionText = stringValue(contextValue, "selection_text")
	}

	files := stringSliceValue(input["files"])
	if len(files) == 0 {
		files = stringSliceValue(contextValue["files"])
	}

	return TaskContextSnapshot{
		Source:        stringValue(params, "source"),
		Trigger:       stringValue(params, "trigger"),
		InputType:     stringValue(input, "type"),
		InputMode:     stringValue(input, "input_mode"),
		Text:          stringValue(input, "text"),
		SelectionText: selectionText,
		ErrorText:     stringValue(input, "error_message"),
		Files:         files,
		PageTitle:     stringValue(page, "title"),
		PageURL:       stringValue(page, "url"),
		AppName:       stringValue(page, "app_name"),
	}
}

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

func stringValue(values map[string]any, key string) string {
	rawValue, ok := values[key]
	if !ok {
		return ""
	}

	value, ok := rawValue.(string)
	if !ok {
		return ""
	}

	return value
}

func stringSliceValue(rawValue any) []string {
	values, ok := rawValue.([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(values))
	for _, rawItem := range values {
		item, ok := rawItem.(string)
		if ok && item != "" {
			result = append(result, item)
		}
	}

	return result
}
