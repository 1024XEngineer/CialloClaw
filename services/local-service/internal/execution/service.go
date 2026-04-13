// 该文件负责主链路最小真实执行链路：收集输入、生成内容、写入 workspace 并返回交付结果。
package execution

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
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
	execution  tools.ExecutionCapability
	playwright tools.PlaywrightSidecarClient
	model      *model.Service
	audit      *audit.Service
	checkpoint *checkpoint.Service
	delivery   *delivery.Service
	tools      *tools.Registry
	executor   *tools.ToolExecutor
	plugin     *plugin.Service
	workspace  string
}

// Request 描述一次任务执行所需的最小输入。
type Request struct {
	TaskID          string
	RunID           string
	Title           string
	Intent          map[string]any
	Snapshot        contextsvc.TaskContextSnapshot
	DeliveryType    string
	ResultTitle     string
	ApprovalGranted bool
}

// Result 描述执行完成后需要回填给 orchestrator 的交付与痕迹。
type Result struct {
	Content         string
	DeliveryResult  map[string]any
	Artifacts       []map[string]any
	BubbleText      string
	RecoveryPoint   map[string]any
	ModelInvocation map[string]any
	AuditRecord     map[string]any
	ToolCalls       []tools.ToolCallRecord
	ToolName        string
	ToolInput       map[string]any
	ToolOutput      map[string]any
	DurationMS      int64
}

// GovernanceAssessment 描述一次潜在高风险动作的预执行治理判断结果。
type GovernanceAssessment struct {
	OperationName      string
	TargetObject       string
	RiskLevel          string
	ApprovalRequired   bool
	CheckpointRequired bool
	Deny               bool
	Reason             string
	ImpactScope        map[string]any
}

// ErrRecoveryPointPrepareFailed 表示执行前的恢复点准备失败。
var ErrRecoveryPointPrepareFailed = errors.New("execution: recovery point prepare failed")

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
	executionBackend tools.ExecutionCapability,
	playwrightClient tools.PlaywrightSidecarClient,
	modelService *model.Service,
	auditService *audit.Service,
	checkpointService *checkpoint.Service,
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
		execution:  executionBackend,
		playwright: playwrightClient,
		model:      modelService,
		audit:      auditService,
		checkpoint: checkpointService,
		delivery:   deliveryService,
		tools:      toolRegistry,
		executor:   toolExecutor,
		plugin:     pluginService,
		workspace:  resolveWorkspaceRoot(fileSystem),
	}
}

// AssessGovernance 在真正执行前，基于将要落地的工具调用做一次统一风险判断。
func (s *Service) AssessGovernance(ctx context.Context, request Request) (GovernanceAssessment, bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	toolName, toolInput, execCtx, ok, err := s.resolveGovernanceToolExecution(request)
	if err != nil {
		return GovernanceAssessment{}, false, err
	}
	if !ok || s.tools == nil {
		return GovernanceAssessment{}, false, nil
	}
	_, precheck, err := s.executor.PrecheckToolWithContext(ctx, execCtx, toolName, toolInput)
	if err != nil {
		return GovernanceAssessment{}, false, err
	}
	if precheck == nil {
		return GovernanceAssessment{}, false, nil
	}
	reason := precheck.Reason
	if reason == "" {
		reason = precheck.DenyReason
	}
	if requireAuthorizationFlag(request.Intent) && !precheck.Deny {
		precheck.ApprovalRequired = true
		if precheck.RiskLevel == "" || precheck.RiskLevel == tools.RiskLevelGreen {
			precheck.RiskLevel = tools.RiskLevelYellow
		}
		if reason == "" {
			reason = "policy_requires_authorization"
		}
	}
	return GovernanceAssessment{
		OperationName:      toolName,
		TargetObject:       governanceTargetObject(toolName, toolInput, execCtx),
		RiskLevel:          precheck.RiskLevel,
		ApprovalRequired:   precheck.ApprovalRequired,
		CheckpointRequired: precheck.CheckpointRequired,
		Deny:               precheck.Deny,
		Reason:             reason,
		ImpactScope:        cloneMap(precheck.ImpactScope),
	}, true, nil
}

// Execute 执行当前任务的最小内容生成与落盘链路。
func (s *Service) Execute(ctx context.Context, request Request) (Result, error) {
	startedAt := time.Now()
	if result, ok, err := s.executeDirectBuiltinTool(ctx, request); err != nil {
		return result, err
	} else if ok {
		result.DurationMS = time.Since(startedAt).Milliseconds()
		return result, nil
	}

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
		ToolInput: map[string]any{
			"intent_name":     effectiveIntentName(request.Intent),
			"delivery_type":   deliveryType,
			"input_preview":   truncateText(inputText, 96),
			"available_tools": s.availableToolNames(),
			"workers":         s.availableWorkers(),
		},
	}

	if toolResult, ok, err := s.executeThroughToolExecutor(ctx, request, deliveryResult, trace.OutputText); err != nil {
		return toolResult, err
	} else if ok {
		toolResult.ToolCalls = append(append([]tools.ToolCallRecord(nil), result.ToolCalls...), toolResult.ToolCalls...)
		// 当请求已走 ToolExecutor 路径时，保留真实工具输入，
		// 仅把 execution 层的通用上下文附加到 execution_context，
		// 避免后续 ToolCall 记录丢失实际工具参数。
		toolResult.ToolInput = mergeToolInputs(toolResult.ToolInput, result.ToolInput)
		toolResult.DurationMS = time.Since(startedAt).Milliseconds()
		return toolResult, nil
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

		writeResult, recoveryPoint, err := s.executeTool(ctx, request, workspacePathFromDeliveryResult(deliveryResult), "write_file", map[string]any{
			"path":    targetPath,
			"content": documentContent,
		})
		if err != nil {
			failedResult := result
			failedResult.RecoveryPoint = cloneMap(recoveryPoint)
			if writeResult != nil {
				failedResult.ToolCalls = append(failedResult.ToolCalls, writeResult.ToolCall)
				failedResult.ToolName = writeResult.ToolCall.ToolName
				failedResult.ToolInput = cloneMap(writeResult.ToolCall.Input)
				failedResult.ToolOutput = cloneMap(writeResult.ToolCall.Output)
			}
			return failedResult, fmt.Errorf("write workspace output: %w", err)
		}

		result.ToolCalls = append(result.ToolCalls, writeResult.ToolCall)
		result.RecoveryPoint = cloneMap(recoveryPoint)
		result.Content = documentContent
		result.Artifacts = s.delivery.BuildArtifact(request.TaskID, request.ResultTitle, deliveryResult)
		result.BubbleText = fmt.Sprintf("结果已写入 %s，可直接查看。", targetPath)
		assignLatestToolTrace(&result, writeResult.ToolCall)
		if len(recoveryPoint) > 0 {
			enrichToolTrace(&result, map[string]any{"recovery_point": cloneMap(recoveryPoint)})
			enrichLatestToolCall(&result, map[string]any{"recovery_point": cloneMap(recoveryPoint)})
		}
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

func (s *Service) executeDirectBuiltinTool(ctx context.Context, request Request) (Result, bool, error) {
	intentName := stringValue(request.Intent, "name", "")
	if intentName == "" || intentName == "write_file" {
		return Result{}, false, nil
	}
	if s.executor == nil || s.tools == nil {
		return Result{}, false, nil
	}
	if _, err := s.tools.Get(intentName); err != nil {
		return Result{}, false, nil
	}
	args := mapValue(request.Intent, "arguments")
	toolResult, recoveryPoint, err := s.executeTool(ctx, request, s.workspace, intentName, args)
	if err != nil {
		failedResult := Result{
			RecoveryPoint: cloneMap(recoveryPoint),
		}
		if toolResult != nil {
			failedResult.ToolCalls = []tools.ToolCallRecord{normalizeFilesystemToolCall(toolResult.ToolCall, map[string]any{"path": stringValue(args, "path", "")})}
			failedResult.ToolName = intentName
			failedResult.ToolInput = mergeToolInputs(args, map[string]any{
				"intent_name":     intentName,
				"delivery_type":   "bubble",
				"available_tools": s.availableToolNames(),
				"workers":         s.availableWorkers(),
			})
			failedResult.ToolOutput = normalizeFilesystemToolOutput(intentName, mergeToolOutputs(toolResult.RawOutput, toolResult.SummaryOutput), args)
		}
		return failedResult, false, fmt.Errorf("execute builtin tool %s: %w", intentName, err)
	}
	bubbleText := toolBubbleText(intentName, toolResult)
	return Result{
		Content:        bubbleText,
		DeliveryResult: s.delivery.BuildDeliveryResultWithTargetPath(request.TaskID, "bubble", request.ResultTitle, bubbleText, ""),
		Artifacts:      toolArtifactsFromResult(request.TaskID, toolResult),
		BubbleText:     bubbleText,
		RecoveryPoint:  cloneMap(recoveryPoint),
		ToolCalls:      []tools.ToolCallRecord{normalizeFilesystemToolCall(toolResult.ToolCall, map[string]any{"path": stringValue(args, "path", "")})},
		ToolName:       intentName,
		ToolInput: mergeToolInputs(args, map[string]any{
			"intent_name":     intentName,
			"delivery_type":   "bubble",
			"available_tools": s.availableToolNames(),
			"workers":         s.availableWorkers(),
		}),
		ToolOutput: normalizeFilesystemToolOutput(intentName, mergeToolOutputs(toolResult.RawOutput, toolResult.SummaryOutput), args),
	}, true, nil
}

func (s *Service) executeThroughToolExecutor(ctx context.Context, request Request, deliveryResult map[string]any, outputText string) (Result, bool, error) {
	toolName, toolInput, ok := s.resolveToolExecution(request, deliveryResult, outputText)
	if !ok || s.executor == nil {
		return Result{}, false, nil
	}

	workspacePath := workspacePathFromDeliveryResult(deliveryResult)
	toolResult, recoveryPoint, err := s.executeTool(ctx, request, workspacePath, toolName, toolInput)
	if err != nil {
		failedResult := Result{
			Content:        outputText,
			DeliveryResult: deliveryResult,
			RecoveryPoint:  cloneMap(recoveryPoint),
			ToolName:       toolName,
			ToolInput:      toolInput,
		}
		if toolResult != nil {
			failedResult.ToolCalls = []tools.ToolCallRecord{normalizeFilesystemToolCall(toolResult.ToolCall, toolInput)}
			failedResult.ToolOutput = normalizeFilesystemToolOutput(toolName, mergeToolOutputs(toolResult.RawOutput, toolResult.SummaryOutput), toolInput)
		}
		return failedResult, false, fmt.Errorf("execute tool %s: %w", toolName, err)
	}

	result := Result{
		Content:        outputText,
		DeliveryResult: deliveryResult,
		Artifacts:      toolArtifactsFromResult(request.TaskID, toolResult),
		RecoveryPoint:  cloneMap(recoveryPoint),
		ToolCalls:      []tools.ToolCallRecord{normalizeFilesystemToolCall(toolResult.ToolCall, toolInput)},
		ToolName:       toolName,
		ToolInput:      toolInput,
		ToolOutput:     normalizeFilesystemToolOutput(toolName, mergeToolOutputs(toolResult.RawOutput, toolResult.SummaryOutput), toolInput),
	}
	if toolName == "write_file" {
		result.Artifacts = s.delivery.BuildArtifact(request.TaskID, request.ResultTitle, deliveryResult)
		result.BubbleText = fmt.Sprintf("结果已写入 %s，可直接查看。", deliveryPayloadPath(deliveryResult))
		if content, ok := toolResult.RawOutput["content"].(string); ok && strings.TrimSpace(content) != "" {
			result.Content = content
		}
		consumedOutput, consumedArtifact, err := s.consumeWriteFileCandidates(ctx, request.TaskID, toolResult.RawOutput)
		if err != nil {
			return Result{}, false, err
		}
		if consumedOutput != nil {
			result.ToolOutput = normalizeFilesystemToolOutput(toolName, mergeToolOutputs(consumedOutput, toolResult.SummaryOutput), toolInput)
		}
		if consumedArtifact != nil {
			if len(result.Artifacts) == 0 {
				result.Artifacts = append(result.Artifacts, consumedArtifact)
			}
		}
	} else {
		bubbleText := toolBubbleText(toolName, toolResult)
		result.BubbleText = bubbleText
		result.Content = bubbleText
		result.DeliveryResult = s.delivery.BuildDeliveryResultWithTargetPath(request.TaskID, "bubble", request.ResultTitle, bubbleText, "")
	}

	return result, true, nil
}

func (s *Service) resolveToolExecution(request Request, deliveryResult map[string]any, outputText string) (string, map[string]any, bool) {
	intentName := stringValue(request.Intent, "name", "")
	args := mapValue(request.Intent, "arguments")

	if intentName == "write_file" || request.DeliveryType == "workspace_document" {
		targetPath := firstNonEmpty(targetPathFromIntent(request.Intent), deliveryPayloadPath(deliveryResult))
		writePath := workspaceFSPath(targetPath)
		if writePath == "" {
			return "", nil, false
		}
		content := workspaceDocumentContent(request.ResultTitle, outputText)
		return "write_file", map[string]any{
			"path":    writePath,
			"content": content,
		}, true
	}

	if s.tools == nil || intentName == "" {
		return "", nil, false
	}
	if _, err := s.tools.Get(intentName); err != nil {
		return "", nil, false
	}

	switch intentName {
	case "read_file":
		pathValue := stringValue(args, "path", stringValue(args, "target_path", ""))
		if pathValue == "" {
			return "", nil, false
		}
		return intentName, map[string]any{"path": pathValue}, true
	case "list_dir":
		pathValue := stringValue(args, "path", stringValue(args, "target_path", ""))
		if pathValue == "" {
			return "", nil, false
		}
		input := map[string]any{"path": pathValue}
		if limit, ok := args["limit"]; ok {
			input["limit"] = limit
		}
		return intentName, input, true
	case "exec_command":
		input := map[string]any{}
		for _, key := range []string{"command", "args", "working_dir"} {
			if value, ok := args[key]; ok {
				input[key] = value
			}
		}
		if len(input) == 0 {
			return "", nil, false
		}
		return intentName, input, true
	default:
		return "", nil, false
	}
}

func mergeToolOutputs(rawOutput, summaryOutput map[string]any) map[string]any {
	if len(rawOutput) == 0 && len(summaryOutput) == 0 {
		return nil
	}
	merged := map[string]any{}
	for key, value := range rawOutput {
		merged[key] = value
	}
	if len(summaryOutput) > 0 {
		merged["summary_output"] = summaryOutput
	}
	return merged
}

func mergeToolInputs(toolInput, executionContext map[string]any) map[string]any {
	if len(toolInput) == 0 && len(executionContext) == 0 {
		return nil
	}
	merged := map[string]any{}
	for key, value := range toolInput {
		merged[key] = value
	}
	if len(executionContext) > 0 {
		merged["execution_context"] = executionContext
	}
	return merged
}

func normalizeFilesystemToolOutput(toolName string, output map[string]any, toolInput map[string]any) map[string]any {
	if len(output) == 0 {
		return nil
	}
	normalized := cloneOutput(output)
	pathValue := stringValue(toolInput, "path", "")
	if pathValue == "" {
		return normalized
	}
	switch toolName {
	case "read_file", "write_file", "list_dir":
		normalized["path"] = pathValue
	}
	return normalized
}

func normalizeFilesystemToolCall(record tools.ToolCallRecord, toolInput map[string]any) tools.ToolCallRecord {
	pathValue := stringValue(toolInput, "path", "")
	if pathValue == "" {
		return record
	}
	if record.Input == nil {
		record.Input = map[string]any{}
	}
	record.Input["path"] = pathValue
	if record.Output == nil {
		record.Output = map[string]any{}
	}
	record.Output["path"] = pathValue
	return record
}

func toolArtifactsFromResult(taskID string, result *tools.ToolExecutionResult) []map[string]any {
	if result == nil || len(result.Artifacts) == 0 {
		return nil
	}
	artifacts := make([]map[string]any, 0, len(result.Artifacts))
	for _, artifact := range result.Artifacts {
		artifacts = append(artifacts, map[string]any{
			"artifact_id":   "",
			"task_id":       taskID,
			"artifact_type": artifact.ArtifactType,
			"title":         artifact.Title,
			"path":          artifact.Path,
			"mime_type":     artifact.MimeType,
		})
	}
	return artifacts
}

func (s *Service) consumeWriteFileCandidates(ctx context.Context, taskID string, rawOutput map[string]any) (map[string]any, map[string]any, error) {
	if len(rawOutput) == 0 {
		return nil, nil, nil
	}

	merged := cloneOutput(rawOutput)
	var artifact map[string]any
	if auditCandidate, ok := rawOutput["audit_candidate"].(map[string]any); ok && s.audit != nil {
		recordInput, err := audit.BuildRecordInputFromCandidate(taskID, auditCandidate)
		if err != nil {
			return nil, nil, fmt.Errorf("build audit record from candidate: %w", err)
		}
		if record, err := s.audit.Write(ctx, recordInput); err != nil {
			return nil, nil, fmt.Errorf("write audit record from candidate: %w", err)
		} else {
			merged["audit_record"] = record.Map()
		}
	}

	if checkpointCandidate, ok := rawOutput["checkpoint_candidate"].(map[string]any); ok && s.checkpoint != nil {
		if _, hasRecoveryPoint := merged["recovery_point"].(map[string]any); !hasRecoveryPoint {
			createInput, shouldCreate, err := checkpoint.BuildCreateInputFromCandidate(taskID, checkpointCandidate)
			if err != nil {
				return nil, nil, fmt.Errorf("build checkpoint input from candidate: %w", err)
			}
			if shouldCreate {
				point, err := s.checkpoint.Create(ctx, createInput)
				if err != nil {
					return nil, nil, fmt.Errorf("create recovery point from candidate: %w", err)
				}
				merged["recovery_point"] = recoveryPointMap(point)
			}
		}
	}

	if artifactCandidate, ok := rawOutput["artifact_candidate"].(map[string]any); ok {
		artifact = map[string]any{
			"artifact_id":   "",
			"task_id":       taskID,
			"artifact_type": artifactCandidate["artifact_type"],
			"title":         artifactCandidate["title"],
			"path":          artifactCandidate["path"],
			"mime_type":     artifactCandidate["mime_type"],
		}
	}

	return merged, artifact, nil
}

func cloneOutput(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func workspacePathFromDeliveryResult(deliveryResult map[string]any) string {
	pathValue := deliveryPayloadPath(deliveryResult)
	if pathValue == "" {
		return "workspace"
	}
	if normalized := workspaceFSPath(pathValue); normalized != "" {
		return normalized
	}
	return "workspace"
}

func toolBubbleText(toolName string, result *tools.ToolExecutionResult) string {
	if result == nil {
		return fmt.Sprintf("%s 执行完成。", toolName)
	}
	if preview := stringValue(result.SummaryOutput, "content_preview", ""); preview != "" {
		return preview
	}
	if preview := stringValue(result.SummaryOutput, "stdout_preview", ""); preview != "" {
		return preview
	}
	if count, ok := result.SummaryOutput["entry_count"]; ok {
		return fmt.Sprintf("%s 执行完成，当前目录条目数：%v。", toolName, count)
	}
	return fmt.Sprintf("%s 执行完成。", toolName)
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
	toolResult, _, err := s.executeTool(ctx, request, s.workspace, "generate_text", map[string]any{
		"prompt":        prompt,
		"fallback_text": fallbackOutput(request, inputText),
		"intent_name":   effectiveIntentName(request.Intent),
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
	intentName := effectiveIntentName(request.Intent)
	targetLanguage := stringValue(mapValue(request.Intent, "arguments"), "target_language", "中文")

	instruction := "请先根据输入判断用户想要什么帮助；如果目标不明确，请明确指出需要用户补充处理方式，不要把内容误当成总结任务。"
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
	intentName := effectiveIntentName(request.Intent)
	normalized := normalizeWhitespace(inputText)
	if normalized == "" {
		normalized = "无可用输入"
	}

	switch intentName {
	case "":
		return "我还不确定你希望我怎么处理这段内容，请补充你的目标，例如解释、翻译、改写或总结。"
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

func effectiveIntentName(taskIntent map[string]any) string {
	return strings.TrimSpace(stringValue(taskIntent, "name", ""))
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

func previewTextForDeliveryType(deliveryType string) string {
	if deliveryType == "workspace_document" {
		return "已为你写入文档并打开"
	}
	return "结果已通过气泡返回"
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
	if filepath.IsAbs(normalized) || isWindowsAbsolutePath(normalized) {
		return path.Clean(normalized)
	}
	normalized = strings.TrimPrefix(normalized, "./")
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

func isWindowsAbsolutePath(pathValue string) bool {
	return len(pathValue) >= 3 && pathValue[1] == ':' && (pathValue[2] == '/' || pathValue[2] == '\\')
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

func (s *Service) executeTool(ctx context.Context, request Request, workspacePath, toolName string, input map[string]any) (*tools.ToolExecutionResult, map[string]any, error) {
	if s.executor == nil {
		return nil, nil, fmt.Errorf("tool executor is required")
	}
	execCtx := s.toolExecutionContext(workspacePath, request)
	recoveryPoint, err := s.prepareGovernanceRecoveryPoint(ctx, request, workspacePath, toolName, input)
	if err != nil {
		return nil, cloneMap(recoveryPoint), err
	}
	toolResult, err := s.executor.ExecuteToolWithContext(ctx, execCtx, toolName, input)
	if toolResult != nil && len(recoveryPoint) > 0 {
		if toolResult.RawOutput == nil {
			toolResult.RawOutput = map[string]any{}
		}
		toolResult.RawOutput["recovery_point"] = cloneMap(recoveryPoint)
	}
	return toolResult, cloneMap(recoveryPoint), err
}

func (s *Service) resolveGovernanceToolExecution(request Request) (string, map[string]any, *tools.ToolExecuteContext, bool, error) {
	intentName := stringValue(request.Intent, "name", "")
	args := mapValue(request.Intent, "arguments")
	deliveryType := firstNonEmpty(strings.TrimSpace(request.DeliveryType), "workspace_document")
	previewText := previewTextForDeliveryType(deliveryType)
	deliveryResult := s.delivery.BuildDeliveryResultWithTargetPath(
		request.TaskID,
		deliveryType,
		firstNonEmpty(strings.TrimSpace(request.ResultTitle), "处理结果"),
		previewText,
		targetPathFromIntent(request.Intent),
	)
	if s.tools != nil && intentName != "" && intentName != "write_file" {
		if _, err := s.tools.Get(intentName); err == nil {
			switch intentName {
			case "read_file":
				pathValue := stringValue(args, "path", stringValue(args, "target_path", ""))
				if pathValue != "" {
					return intentName, map[string]any{"path": pathValue}, s.toolExecutionContext(s.workspace, request), true, nil
				}
			case "list_dir":
				pathValue := stringValue(args, "path", stringValue(args, "target_path", ""))
				if pathValue != "" {
					input := map[string]any{"path": pathValue}
					if limit, ok := args["limit"]; ok {
						input["limit"] = limit
					}
					return intentName, input, s.toolExecutionContext(s.workspace, request), true, nil
				}
			case "exec_command":
				input := map[string]any{}
				for _, key := range []string{"command", "args", "working_dir"} {
					if value, ok := args[key]; ok {
						input[key] = value
					}
				}
				if len(input) > 0 {
					return intentName, input, s.toolExecutionContext(s.workspace, request), true, nil
				}
			}
		}
	}
	rawTargetPath := firstNonEmpty(targetPathFromIntent(request.Intent), deliveryPayloadPath(deliveryResult))
	writePath := workspaceFSPath(rawTargetPath)
	if writePath == "" {
		writePath = strings.TrimSpace(strings.ReplaceAll(rawTargetPath, "\\", "/"))
	}
	if writePath == "" {
		return "", nil, nil, false, nil
	}
	toolName, toolInput := "write_file", map[string]any{"path": writePath, "content": ""}
	workspacePath := s.workspace
	if toolName == "write_file" {
		workspacePath = workspacePathFromDeliveryResult(deliveryResult)
	}
	return toolName, toolInput, s.toolExecutionContext(workspacePath, request), true, nil
}

func (s *Service) toolExecutionContext(workspacePath string, request Request) *tools.ToolExecuteContext {
	workspacePath = firstNonEmpty(strings.TrimSpace(workspacePath), s.workspace)
	return &tools.ToolExecuteContext{
		TaskID:               request.TaskID,
		RunID:                request.RunID,
		WorkspacePath:        workspacePath,
		ApprovalGranted:      request.ApprovalGranted,
		ApprovedOperation:    stringValue(request.Intent, "name", ""),
		ApprovedTargetObject: approvedTargetObject(request.Intent, workspacePath),
		Platform:             s.fileSystem,
		Execution:            s.execution,
		Playwright:           s.playwright,
		Model:                s.model,
	}
}

func (s *Service) prepareGovernanceRecoveryPoint(ctx context.Context, request Request, workspacePath, toolName string, input map[string]any) (map[string]any, error) {
	if s.checkpoint == nil {
		return nil, nil
	}
	switch toolName {
	case "write_file":
		targetPath := stringValue(input, "path", "")
		if targetPath == "" {
			return nil, nil
		}
		if _, err := s.fileSystem.Stat(targetPath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil, nil
			}
			return nil, fmt.Errorf("%w: inspect target path %s: %v", ErrRecoveryPointPrepareFailed, targetPath, err)
		}
		point, err := s.checkpoint.Create(ctx, checkpoint.CreateInput{
			TaskID:  request.TaskID,
			Summary: "write_file_before_change",
			Objects: []string{targetPath},
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRecoveryPointPrepareFailed, err)
		}
		return recoveryPointMap(point), nil
	case "exec_command":
		targetObject := stringValue(input, "working_dir", "")
		if strings.TrimSpace(targetObject) == "" {
			targetObject = firstNonEmpty(strings.TrimSpace(workspacePath), s.workspace)
		}
		if strings.TrimSpace(targetObject) == "" {
			return nil, nil
		}
		point, err := s.checkpoint.Create(ctx, checkpoint.CreateInput{
			TaskID:  request.TaskID,
			Summary: "exec_command_before_change",
			Objects: []string{targetObject},
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrRecoveryPointPrepareFailed, err)
		}
		return recoveryPointMap(point), nil
	default:
		return nil, nil
	}
}

func recoveryPointMap(point checkpoint.RecoveryPoint) map[string]any {
	return map[string]any{
		"recovery_point_id": point.RecoveryPointID,
		"task_id":           point.TaskID,
		"summary":           point.Summary,
		"created_at":        point.CreatedAt,
		"objects":           append([]string(nil), point.Objects...),
	}
}

func governanceTargetObject(toolName string, toolInput map[string]any, execCtx *tools.ToolExecuteContext) string {
	switch toolName {
	case "write_file":
		return stringValue(toolInput, "path", "")
	case "exec_command":
		return firstNonEmpty(stringValue(toolInput, "working_dir", ""), execCtx.WorkspacePath)
	default:
		return stringValue(toolInput, "path", "")
	}
}

func approvedTargetObject(intent map[string]any, workspacePath string) string {
	arguments := mapValue(intent, "arguments")
	for _, key := range []string{"target_path", "path", "working_dir"} {
		if value := strings.TrimSpace(stringValue(arguments, key, "")); value != "" {
			normalized := strings.ReplaceAll(value, "\\", "/")
			if key != "working_dir" {
				if candidate := workspaceFSPath(normalized); candidate != "" {
					normalized = candidate
				}
			}
			workspaceRoot := strings.ReplaceAll(strings.TrimSpace(workspacePath), "\\", "/")
			if workspaceRoot != "" && !path.IsAbs(normalized) && !isWindowsAbsolutePath(normalized) {
				return path.Join(workspaceRoot, normalized)
			}
			return normalized
		}
	}
	if stringValue(intent, "name", "") == "exec_command" {
		return workspacePath
	}
	return ""
}

func requireAuthorizationFlag(intent map[string]any) bool {
	return boolValue(mapValue(intent, "arguments"), "require_authorization")
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
