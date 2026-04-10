// 该文件负责主链路最小真实执行链路：收集输入、生成内容、写入 workspace 并返回交付结果。
package execution

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/delivery"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/model"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/platform"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/plugin"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

// Service 负责在当前仓库代码范围内完成一条可运行的最小执行链路。
type Service struct {
	fileSystem platform.FileSystemAdapter
	model      *model.Service
	audit      *audit.Service
	delivery   *delivery.Service
	tools      *tools.Registry
	executor   *tools.ToolExecutor
	plugin     *plugin.Service
	workspace  string
}

// Request 描述一次任务执行所需的最小输入。
type Request struct {
	TaskID       string
	RunID        string
	Title        string
	Intent       map[string]any
	Snapshot     contextsvc.TaskContextSnapshot
	DeliveryType string
	ResultTitle  string
}

// Result 描述执行完成后需要回填给 orchestrator 的交付与痕迹。
type Result struct {
	Content         string
	DeliveryResult  map[string]any
	Artifacts       []map[string]any
	BubbleText      string
	ModelInvocation map[string]any
	AuditRecord     map[string]any
	ToolCalls       []tools.ToolCallRecord
	ToolName        string
	ToolInput       map[string]any
	ToolOutput      map[string]any
	DurationMS      int64
}

type generationTrace struct {
	OutputText       string
	ToolCalls        []tools.ToolCallRecord
	ModelInvocation  map[string]any
	AuditRecord      map[string]any
	GenerationOutput map[string]any
}

// NewService 创建执行服务。
func NewService(
	fileSystem platform.FileSystemAdapter,
	modelService *model.Service,
	auditService *audit.Service,
	deliveryService *delivery.Service,
	toolRegistry *tools.Registry,
	toolExecutor *tools.ToolExecutor,
	pluginService *plugin.Service,
) *Service {
	if toolExecutor == nil {
		toolExecutor = tools.NewToolExecutor(toolRegistry)
	}

	return &Service{
		fileSystem: fileSystem,
		model:      modelService,
		audit:      auditService,
		delivery:   deliveryService,
		tools:      toolRegistry,
		executor:   toolExecutor,
		plugin:     pluginService,
		workspace:  resolveWorkspaceRoot(fileSystem),
	}
}

// Execute 执行当前任务的最小内容生成与落盘链路。
func (s *Service) Execute(ctx context.Context, request Request) (Result, error) {
	startedAt := time.Now()
	inputText := s.buildExecutionInput(request.Snapshot)
	trace, err := s.generateOutput(ctx, request, inputText)
	if err != nil {
		return Result{}, err
	}

	deliveryType := firstNonEmpty(request.DeliveryType, "workspace_document")
	targetPath := targetPathFromIntent(request.Intent)
	previewText := previewTextForOutput(trace.OutputText, deliveryType)
	deliveryResult := s.delivery.BuildDeliveryResultWithTargetPath(request.TaskID, deliveryType, request.ResultTitle, previewText, targetPath)

	result := Result{
		Content:         trace.OutputText,
		DeliveryResult:  deliveryResult,
		DurationMS:      time.Since(startedAt).Milliseconds(),
		ModelInvocation: cloneMap(trace.ModelInvocation),
		AuditRecord:     cloneMap(trace.AuditRecord),
		ToolCalls:       append([]tools.ToolCallRecord(nil), trace.ToolCalls...),
	}

	if deliveryType == "workspace_document" {
		documentContent := workspaceDocumentContent(request.ResultTitle, trace.OutputText)
		targetPath = deliveryPayloadPath(deliveryResult)
		if targetPath == "" {
			return Result{}, fmt.Errorf("workspace delivery requires payload path")
		}
		if workspaceFSPath(targetPath) == "" {
			return Result{}, fmt.Errorf("workspace delivery requires writable workspace path")
		}

		writeResult, err := s.executeTool(ctx, request, "write_file", map[string]any{
			"path":    targetPath,
			"content": documentContent,
		})
		if err != nil {
			return Result{}, fmt.Errorf("write workspace output: %w", err)
		}

		result.ToolCalls = append(result.ToolCalls, writeResult.ToolCall)
		result.Content = documentContent
		result.Artifacts = s.delivery.BuildArtifact(request.TaskID, request.ResultTitle, deliveryResult)
		result.BubbleText = fmt.Sprintf("结果已写入 %s，可直接查看。", targetPath)
		assignLatestToolTrace(&result, writeResult.ToolCall)
		enrichToolTrace(&result, map[string]any{
			"path":             targetPath,
			"artifact_count":   len(result.Artifacts),
			"content_bytes":    len(documentContent),
			"model_invocation": cloneMap(result.ModelInvocation),
			"audit_record":     cloneMap(result.AuditRecord),
		})
		enrichLatestToolCall(&result, map[string]any{
			"path":             targetPath,
			"artifact_count":   len(result.Artifacts),
			"content_bytes":    len(documentContent),
			"model_invocation": cloneMap(result.ModelInvocation),
			"audit_record":     cloneMap(result.AuditRecord),
		})
		return result, nil
	}

	result.BubbleText = truncateBubbleText(trace.OutputText)
	assignLatestToolTrace(&result, latestToolCall(result.ToolCalls))
	enrichToolTrace(&result, map[string]any{
		"preview_text":     previewText,
		"content_size":     len(trace.OutputText),
		"model_invocation": cloneMap(result.ModelInvocation),
		"audit_record":     cloneMap(result.AuditRecord),
	})
	enrichLatestToolCall(&result, map[string]any{
		"preview_text":     previewText,
		"content_size":     len(trace.OutputText),
		"model_invocation": cloneMap(result.ModelInvocation),
		"audit_record":     cloneMap(result.AuditRecord),
	})
	return result, nil
}

func (s *Service) buildExecutionInput(snapshot contextsvc.TaskContextSnapshot) string {
	sections := make([]string, 0, 6)
	if snapshot.SelectionText != "" {
		sections = append(sections, "选中文本:\n"+strings.TrimSpace(snapshot.SelectionText))
	}
	if snapshot.Text != "" {
		sections = append(sections, "输入文本:\n"+strings.TrimSpace(snapshot.Text))
	}
	if snapshot.ErrorText != "" {
		sections = append(sections, "错误信息:\n"+strings.TrimSpace(snapshot.ErrorText))
	}
	if len(snapshot.Files) > 0 {
		for _, filePath := range snapshot.Files {
			sections = append(sections, s.fileSection(filePath))
		}
	}
	if snapshot.PageTitle != "" || snapshot.PageURL != "" || snapshot.AppName != "" {
		sections = append(sections, fmt.Sprintf(
			"页面上下文:\n标题: %s\nURL: %s\n应用: %s",
			strings.TrimSpace(snapshot.PageTitle),
			strings.TrimSpace(snapshot.PageURL),
			strings.TrimSpace(snapshot.AppName),
		))
	}
	if len(sections) == 0 {
		return "无可用输入"
	}
	return strings.Join(sections, "\n\n")
}

func (s *Service) fileSection(filePath string) string {
	trimmedPath := strings.TrimSpace(filePath)
	if trimmedPath == "" {
		return "文件: <empty>"
	}
	if s.fileSystem == nil {
		return fmt.Sprintf("文件: %s", trimmedPath)
	}

	workspacePath := workspaceFSPath(trimmedPath)
	if workspacePath == "" {
		return fmt.Sprintf("文件: %s", trimmedPath)
	}

	content, err := s.fileSystem.ReadFile(workspacePath)
	if err != nil {
		return fmt.Sprintf("文件: %s\n读取失败: %v", trimmedPath, err)
	}

	return fmt.Sprintf("文件 %s 内容:\n%s", trimmedPath, truncateText(string(content), 1600))
}

func (s *Service) generateOutput(ctx context.Context, request Request, inputText string) (generationTrace, error) {
	prompt := buildPrompt(request, inputText)
	toolResult, err := s.executeTool(ctx, request, "generate_text", map[string]any{
		"prompt":        prompt,
		"fallback_text": fallbackOutput(request, inputText),
		"intent_name":   stringValue(request.Intent, "name", "summarize"),
	})
	if err != nil {
		return generationTrace{}, fmt.Errorf("generate text: %w", err)
	}

	outputText, ok := toolResult.RawOutput["content"].(string)
	if !ok || strings.TrimSpace(outputText) == "" {
		return generationTrace{}, fmt.Errorf("%w: generate_text content is missing", tools.ErrToolOutputInvalid)
	}

	invocation := invocationRecordFromToolResult(request, toolResult)
	auditRecord, err := s.buildModelAuditRecord(ctx, request, invocation)
	if err != nil {
		return generationTrace{}, err
	}

	return generationTrace{
		OutputText:       strings.TrimSpace(outputText),
		ToolCalls:        []tools.ToolCallRecord{toolResult.ToolCall},
		ModelInvocation:  invocationRecordMap(invocation),
		AuditRecord:      auditRecord,
		GenerationOutput: cloneMap(toolResult.RawOutput),
	}, nil
}

func (s *Service) buildModelAuditRecord(ctx context.Context, request Request, invocation *model.InvocationRecord) (map[string]any, error) {
	if s.audit == nil || invocation == nil {
		return nil, nil
	}

	target := strings.TrimSpace(invocation.Provider + ":" + invocation.ModelID)
	if target == ":" {
		target = stringValue(request.Intent, "name", "main_flow")
	}

	record, err := s.audit.Write(ctx, audit.RecordInput{
		TaskID:  request.TaskID,
		Type:    "model",
		Action:  "generate_text",
		Summary: "model invocation completed",
		Target:  target,
		Result:  "success",
	})
	if err != nil {
		return nil, fmt.Errorf("write model audit record: %w", err)
	}
	return record.Map(), nil
}

func invocationRecordFromToolResult(request Request, toolResult *tools.ToolExecutionResult) *model.InvocationRecord {
	if toolResult == nil || len(toolResult.RawOutput) == 0 {
		return nil
	}

	if boolValue(toolResult.RawOutput, "fallback") {
		return nil
	}

	provider := stringValue(toolResult.RawOutput, "provider", "")
	modelID := stringValue(toolResult.RawOutput, "model_id", "")
	if provider == "" || provider == "local_fallback" || modelID == "" {
		return nil
	}

	return &model.InvocationRecord{
		TaskID:    request.TaskID,
		RunID:     request.RunID,
		RequestID: stringValue(toolResult.RawOutput, "request_id", ""),
		Provider:  provider,
		ModelID:   modelID,
		Usage: model.TokenUsage{
			InputTokens:  intValue(mapValue(toolResult.RawOutput, "token_usage"), "input_tokens"),
			OutputTokens: intValue(mapValue(toolResult.RawOutput, "token_usage"), "output_tokens"),
			TotalTokens:  intValue(mapValue(toolResult.RawOutput, "token_usage"), "total_tokens"),
		},
		LatencyMS: int64Value(toolResult.RawOutput, "latency_ms"),
	}
}

func invocationRecordMap(record *model.InvocationRecord) map[string]any {
	if record == nil {
		return nil
	}
	return record.Map()
}

func buildPrompt(request Request, inputText string) string {
	intentName := stringValue(request.Intent, "name", "summarize")
	targetLanguage := stringValue(mapValue(request.Intent, "arguments"), "target_language", "中文")

	instruction := "请整理以下内容并给出结果。"
	switch intentName {
	case "rewrite":
		instruction = "请保留原意并以更清晰、可直接使用的中文改写以下内容。"
	case "translate":
		instruction = fmt.Sprintf("请将以下内容翻译成%s，并直接输出翻译结果。", targetLanguage)
	case "explain":
		instruction = "请用简洁中文解释以下内容，突出重点和结论。"
	case "write_file":
		instruction = "请根据以下输入生成一份可直接保存为文档的中文内容，使用清晰标题和小节。"
	case "summarize":
		instruction = "请总结以下内容，输出结构清晰的中文摘要。"
	}

	return strings.TrimSpace(instruction) + "\n\n输入内容:\n" + strings.TrimSpace(inputText)
}

func fallbackOutput(request Request, inputText string) string {
	intentName := stringValue(request.Intent, "name", "summarize")
	normalized := normalizeWhitespace(inputText)
	if normalized == "" {
		normalized = "无可用输入"
	}

	switch intentName {
	case "rewrite":
		return "改写结果：\n" + normalized
	case "translate":
		targetLanguage := stringValue(mapValue(request.Intent, "arguments"), "target_language", "中文")
		return fmt.Sprintf("翻译结果（回退模式，目标语言：%s）：\n%s", targetLanguage, normalized)
	case "explain":
		return "解释结果：\n" + firstNonEmpty(firstSentence(normalized), normalized)
	case "write_file":
		fallthrough
	case "summarize":
		highlights := extractHighlights(normalized, 3)
		if len(highlights) == 0 {
			return "总结结果：\n- 暂无可总结内容"
		}

		lines := []string{"总结结果："}
		for _, highlight := range highlights {
			lines = append(lines, "- "+highlight)
		}
		return strings.Join(lines, "\n")
	default:
		return normalized
	}
}

func workspaceDocumentContent(title, outputText string) string {
	trimmed := strings.TrimSpace(outputText)
	if trimmed == "" {
		trimmed = "暂无内容"
	}
	if strings.HasPrefix(trimmed, "#") {
		return trimmed + "\n"
	}
	return fmt.Sprintf("# %s\n\n%s\n", firstNonEmpty(strings.TrimSpace(title), "处理结果"), trimmed)
}

func previewTextForOutput(outputText, deliveryType string) string {
	preview := truncateText(normalizeWhitespace(outputText), 96)
	if preview == "" {
		preview = "结果已生成"
	}
	if deliveryType == "workspace_document" {
		return "已生成正式文档：" + preview
	}
	return preview
}

func truncateBubbleText(outputText string) string {
	trimmed := strings.TrimSpace(outputText)
	if trimmed == "" {
		return "结果已生成。"
	}
	return truncateText(trimmed, 480)
}

func deliveryPayloadPath(deliveryResult map[string]any) string {
	payload, ok := deliveryResult["payload"].(map[string]any)
	if !ok {
		return ""
	}
	return stringValue(payload, "path", "")
}

func targetPathFromIntent(taskIntent map[string]any) string {
	targetPath := stringValue(mapValue(taskIntent, "arguments"), "target_path", "")
	switch targetPath {
	case "", "workspace_document", "bubble", "result_page", "task_detail", "open_file", "reveal_in_folder":
		return ""
	default:
		return targetPath
	}
}

func workspaceFSPath(filePath string) string {
	normalized := strings.TrimSpace(strings.ReplaceAll(filePath, "\\", "/"))
	if normalized == "" {
		return ""
	}
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimPrefix(normalized, "/")
	if normalized == "workspace" {
		return "."
	}
	if strings.HasPrefix(normalized, "workspace/") {
		normalized = strings.TrimPrefix(normalized, "workspace/")
	}
	cleaned := path.Clean(normalized)
	if cleaned == "." {
		return "."
	}
	if strings.HasPrefix(cleaned, "../") {
		return ""
	}
	return cleaned
}

func extractHighlights(inputText string, limit int) []string {
	fields := strings.FieldsFunc(inputText, func(r rune) bool {
		switch r {
		case '\n', '\r', '。', '！', '？', '.', '!', '?', ';', '；':
			return true
		default:
			return false
		}
	})

	highlights := make([]string, 0, limit)
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed == "" {
			continue
		}
		highlights = append(highlights, truncateText(trimmed, 80))
		if len(highlights) == limit {
			break
		}
	}
	return highlights
}

func firstSentence(inputText string) string {
	highlights := extractHighlights(inputText, 1)
	if len(highlights) == 0 {
		return ""
	}
	return highlights[0]
}

func normalizeWhitespace(inputText string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(inputText)), " ")
}

func truncateText(inputText string, maxLength int) string {
	if maxLength <= 0 || len(inputText) <= maxLength {
		return inputText
	}
	return inputText[:maxLength] + "..."
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

func stringValue(values map[string]any, key, fallback string) string {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}
	value, ok := rawValue.(string)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func boolValue(values map[string]any, key string) bool {
	rawValue, ok := values[key]
	if !ok {
		return false
	}
	value, ok := rawValue.(bool)
	return ok && value
}

func intValue(values map[string]any, key string) int {
	rawValue, ok := values[key]
	if !ok {
		return 0
	}
	switch value := rawValue.(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float32:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func int64Value(values map[string]any, key string) int64 {
	rawValue, ok := values[key]
	if !ok {
		return 0
	}
	switch value := rawValue.(type) {
	case int:
		return int64(value)
	case int32:
		return int64(value)
	case int64:
		return value
	case float32:
		return int64(value)
	case float64:
		return int64(value)
	default:
		return 0
	}
}

func firstNonEmpty(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

func (s *Service) availableToolNames() []string {
	if s.tools == nil {
		return nil
	}
	return s.tools.Names()
}

func (s *Service) availableWorkers() []string {
	if s.plugin == nil {
		return nil
	}
	return s.plugin.Workers()
}

func (s *Service) executeTool(ctx context.Context, request Request, toolName string, input map[string]any) (*tools.ToolExecutionResult, error) {
	if s.executor == nil {
		return nil, fmt.Errorf("tool executor is required")
	}

	return s.executor.ExecuteToolWithContext(ctx, &tools.ToolExecuteContext{
		TaskID:        request.TaskID,
		RunID:         request.RunID,
		WorkspacePath: s.workspace,
		Platform:      s.fileSystem,
		Model:         s.model,
	}, toolName, input)
}

func resolveWorkspaceRoot(fileSystem platform.FileSystemAdapter) string {
	if fileSystem == nil {
		return ""
	}

	workspaceRoot, err := fileSystem.EnsureWithinWorkspace(".")
	if err != nil {
		return ""
	}
	return workspaceRoot
}

func latestToolCall(toolCalls []tools.ToolCallRecord) tools.ToolCallRecord {
	if len(toolCalls) == 0 {
		return tools.ToolCallRecord{}
	}
	return toolCalls[len(toolCalls)-1]
}

func assignLatestToolTrace(result *Result, toolCall tools.ToolCallRecord) {
	if result == nil || toolCall.ToolName == "" {
		return
	}
	result.ToolName = toolCall.ToolName
	result.ToolInput = cloneMap(toolCall.Input)
	result.ToolOutput = cloneMap(toolCall.Output)
}

func enrichToolTrace(result *Result, extras map[string]any) {
	if result == nil || len(extras) == 0 {
		return
	}
	if result.ToolOutput == nil {
		result.ToolOutput = map[string]any{}
	}
	for key, value := range extras {
		result.ToolOutput[key] = value
	}
}

func enrichLatestToolCall(result *Result, extras map[string]any) {
	if result == nil || len(result.ToolCalls) == 0 || len(extras) == 0 {
		return
	}

	lastIndex := len(result.ToolCalls) - 1
	if result.ToolCalls[lastIndex].Output == nil {
		result.ToolCalls[lastIndex].Output = map[string]any{}
	}
	for key, value := range extras {
		result.ToolCalls[lastIndex].Output[key] = value
	}
}

func cloneMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}

	cloned := make(map[string]any, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			cloned[key] = cloneMap(typed)
		case []map[string]any:
			cloned[key] = cloneMapSlice(typed)
		case []string:
			cloned[key] = append([]string(nil), typed...)
		default:
			cloned[key] = value
		}
	}
	return cloned
}

func cloneMapSlice(values []map[string]any) []map[string]any {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]map[string]any, 0, len(values))
	for _, value := range values {
		cloned = append(cloned, cloneMap(value))
	}
	return cloned
}
