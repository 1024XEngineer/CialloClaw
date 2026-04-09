// 该文件负责主链路输入上下文的采集与归一化。
package context

// TaskContextSnapshot 定义当前模块的数据结构。

// TaskContextSnapshot 汇总了一次请求中与主链路调度相关的上下文快照。
// 它的职责是把前端输入、页面环境、选区、文件列表等来源差异统一收口成一个稳定结构，
// 让 intent、orchestrator、memory 等后续模块都只依赖这一份归一化结果。
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

// Service 提供当前模块的服务能力。

// Service 负责把 JSON-RPC 请求里的输入参数折叠成主链路可消费的上下文对象。
// 这一层不做意图判断和任务决策，只负责“收集”和“归一化”。
type Service struct{}

// NewService 创建并返回Service。

// NewService 创建上下文采集服务。
func NewService() *Service {
	return &Service{}
}

// Snapshot 处理当前模块的相关逻辑。

// Snapshot 返回当前上下文服务的最小运行态快照。
// 目前这里主要用于 bootstrap 和调试接口暴露服务来源信息。
func (s *Service) Snapshot() map[string]string {
	return map[string]string{"source": "desktop"}
}

// Capture 处理当前模块的相关逻辑。

// Capture 从一次 RPC 调用参数中提取任务相关上下文。
// 这里会兼容 input/context 两层字段来源，并把选中文本、页面信息、文件列表等数据合并到统一快照中。
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

// mapValue 处理当前模块的相关逻辑。

// mapValue 安全读取嵌套对象，避免上层到处重复类型断言。
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

// stringValue 处理当前模块的相关逻辑。

// stringValue 安全读取字符串字段，缺失或类型不匹配时返回空字符串。
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

// stringSliceValue 处理当前模块的相关逻辑。

// stringSliceValue 把来自 JSON 的 []any 转成 []string，并过滤空值。
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
