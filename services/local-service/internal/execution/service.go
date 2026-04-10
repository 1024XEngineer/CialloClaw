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
	plugin     *plugin.Service
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
	ToolName        string
	ToolInput       map[string]any
	ToolOutput      map[string]any
	DurationMS      int64
}

// NewService 创建执行服务。
func NewService(
	fileSystem platform.FileSystemAdapter,
	modelService *model.Service,
	auditService *audit.Service,
	deliveryService *delivery.Service,
	toolRegistry *tools.Registry,
	pluginService *plugin.Service,
) *Service {
	return &Service{
		fileSystem: fileSystem,
		model:      modelService,
		audit:      auditService,
		delivery:   deliveryService,
		tools:      toolRegistry,
		plugin:     pluginService,
	}
}

// Execute 执行当前任务的最小内容生成与落盘链路。
func (s *Service) Execute(ctx context.Context, request Request) (Result, error) {
	startedAt := time.Now()
	inputText := s.buildExecutionInput(request.Snapshot)
	outputText, invocationRecord, err := s.generateOutput(ctx, request, inputText)
	if err != nil {
		return Result{}, err
	}
	deliveryType := firstNonEmpty(request.DeliveryType, "workspace_document")
	targetPath := targetPathFromIntent(request.Intent)
	previewText := previewTextForOutput(outputText, deliveryType)
	deliveryResult := s.delivery.BuildDeliveryResultWithTargetPath(request.TaskID, deliveryType, request.ResultTitle, previewText, targetPath)
	auditRecord, err := s.buildModelAuditRecord(ctx, request, invocationRecord)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Content:         outputText,
		DeliveryResult:  deliveryResult,
		DurationMS:      time.Since(startedAt).Milliseconds(),
		ModelInvocation: invocationRecordMap(invocationRecord),
		AuditRecord:     auditRecord,
		ToolInput: map[string]any{
			"intent_name":     stringValue(request.Intent, "name", "summarize"),
			"delivery_type":   deliveryType,
			"input_preview":   truncateText(inputText, 96),
			"available_tools": s.availableToolNames(),
			"workers":         s.availableWorkers(),
		},
	}

	if deliveryType == "workspace_document" {
		documentContent := workspaceDocumentContent(request.ResultTitle, outputText)
		targetPath = deliveryPayloadPath(deliveryResult)
		if targetPath == "" {
			return Result{}, fmt.Errorf("workspace delivery requires payload path")
		}
		writePath := workspaceFSPath(targetPath)
		if writePath == "" {
			return Result{}, fmt.Errorf("workspace delivery requires writable workspace path")
		}
		if s.fileSystem == nil {
			return Result{}, fmt.Errorf("workspace delivery requires file system adapter")
		}
		if err := s.fileSystem.WriteFile(writePath, []byte(documentContent)); err != nil {
			return Result{}, fmt.Errorf("write workspace output: %w", err)
		}

		result.Content = documentContent
		result.Artifacts = s.delivery.BuildArtifact(request.TaskID, request.ResultTitle, deliveryResult)
		result.BubbleText = fmt.Sprintf("结果已写入 %s，可直接查看。", targetPath)
		result.ToolName = "write_file"
		result.ToolOutput = map[string]any{
			"path":             targetPath,
			"artifact_count":   len(result.Artifacts),
			"content_bytes":    len(documentContent),
			"model_invocation": result.ModelInvocation,
			"audit_record":     result.AuditRecord,
		}
		return result, nil
	}

	result.BubbleText = truncateBubbleText(outputText)
	result.ToolName = "generate_text"
	result.ToolOutput = map[string]any{
		"preview_text":     previewText,
		"content_size":     len(outputText),
		"model_invocation": result.ModelInvocation,
		"audit_record":     result.AuditRecord,
	}
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

func (s *Service) generateOutput(ctx context.Context, request Request, inputText string) (string, *model.InvocationRecord, error) {
	prompt := buildPrompt(request, inputText)
	if s.model != nil {
		response, err := s.model.GenerateText(ctx, model.GenerateTextRequest{
			TaskID: request.TaskID,
			RunID:  request.RunID,
			Input:  prompt,
		})
		if err == nil {
			if outputText := strings.TrimSpace(response.OutputText); outputText != "" {
				record := response.InvocationRecord()
				return outputText, &record, nil
			}
			return "", nil, fmt.Errorf("model returned empty output")
		}
		return "", nil, fmt.Errorf("model generate text: %w", err)
	}

	return fallbackOutput(request, inputText), nil, nil
}

func (s *Service) buildModelAuditRecord(ctx context.Context, request Request, invocation *model.InvocationRecord) (map[string]any, error) {
	if s.audit == nil || invocation == nil {
		return nil, nil
	}

	record, err := s.audit.Write(ctx, audit.RecordInput{
		TaskID:  request.TaskID,
		Type:    "model",
		Action:  "generate_text",
		Summary: "model invocation completed",
		Target:  invocation.Provider + ":" + invocation.ModelID,
		Result:  "success",
	})
	if err != nil {
		return nil, fmt.Errorf("write model audit record: %w", err)
	}
	return record.Map(), nil
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
