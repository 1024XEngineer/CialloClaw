// Package tools provides the unified tool execution facade.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
)

// DefaultTimeoutSec is the default tool execution timeout in seconds.
const DefaultTimeoutSec = 30

// ToolExecutorOption customizes ToolExecutor behavior.
type ToolExecutorOption func(*ToolExecutor)

// WithToolCallRecorder injects a lifecycle recorder.
func WithToolCallRecorder(recorder *ToolCallRecorder) ToolExecutorOption {
	return func(e *ToolExecutor) {
		if recorder != nil {
			e.recorder = recorder
		}
	}
}

// WithToolErrorMapper injects a custom error mapper.
func WithToolErrorMapper(mapper ToolErrorMapper) ToolExecutorOption {
	return func(e *ToolExecutor) {
		if mapper != nil {
			e.errorMapper = mapper
		}
	}
}

// WithRiskPrechecker injects a custom precheck implementation.
func WithRiskPrechecker(prechecker RiskPrechecker) ToolExecutorOption {
	return func(e *ToolExecutor) {
		if prechecker != nil {
			e.prechecker = prechecker
		}
	}
}

// ToolExecutor is the unified tool execution entrypoint.
type ToolExecutor struct {
	registry    *Registry
	recorder    *ToolCallRecorder
	errorMapper ToolErrorMapper
	prechecker  RiskPrechecker
}

// NewToolExecutor creates a ToolExecutor with safe defaults.
func NewToolExecutor(registry *Registry, opts ...ToolExecutorOption) *ToolExecutor {
	exec := &ToolExecutor{
		registry:    registry,
		recorder:    NewToolCallRecorder(nil),
		errorMapper: DefaultToolErrorMapper{},
		prechecker:  DefaultRiskPrechecker{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(exec)
		}
	}
	return exec
}

// ResolveTool returns a registered tool by name.
func (e *ToolExecutor) ResolveTool(name string) (Tool, error) {
	if e.registry == nil {
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, name)
	}
	tool, err := e.registry.Get(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, name)
	}
	return tool, nil
}

// ExecuteTool executes a tool without an execution context.
func (e *ToolExecutor) ExecuteTool(ctx context.Context, name string, input map[string]any) (*ToolExecutionResult, error) {
	return e.ExecuteToolWithContext(ctx, nil, name, input)
}

// PrecheckToolWithContext runs validation and risk precheck without executing the tool.
func (e *ToolExecutor) PrecheckToolWithContext(ctx context.Context, execCtx *ToolExecuteContext, name string, input map[string]any) (ToolMetadata, *RiskPrecheckResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if execCtx == nil {
		execCtx = &ToolExecuteContext{}
	}
	tool, err := e.ResolveTool(name)
	if err != nil {
		return ToolMetadata{}, nil, err
	}
	metadata := tool.Metadata()
	if err := tool.Validate(input); err != nil {
		return metadata, nil, fmt.Errorf("%w: %v", ErrToolValidationFailed, err)
	}
	precheckResult, err := e.precheck(ctx, BuildRiskPrecheckInput(metadata, name, execCtx, input))
	if err != nil {
		return metadata, nil, err
	}
	return metadata, precheckResult, nil
}

// ExecuteToolWithContext executes a tool with a ToolExecuteContext.
func (e *ToolExecutor) ExecuteToolWithContext(ctx context.Context, execCtx *ToolExecuteContext, name string, input map[string]any) (*ToolExecutionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if execCtx == nil {
		execCtx = &ToolExecuteContext{}
	}

	record := e.recorder.Start(ctx, execCtx, name, input)

	tool, err := e.ResolveTool(name)
	if err != nil {
		record = e.recorder.Finish(ctx, record, ToolCallStatusFailed, nil, 0, mapToolErrorCode(e.errorMapper, err))
		return &ToolExecutionResult{ToolCall: record}, err
	}

	metadata := tool.Metadata()
	if err := tool.Validate(input); err != nil {
		wrapped := fmt.Errorf("%w: %v", ErrToolValidationFailed, err)
		record = e.recorder.Finish(ctx, record, ToolCallStatusFailed, nil, 0, mapToolErrorCode(e.errorMapper, wrapped))
		return &ToolExecutionResult{Metadata: metadata, ToolCall: record}, wrapped
	}

	precheckInput := BuildRiskPrecheckInput(metadata, name, execCtx, input)
	precheckResult, err := e.precheck(ctx, precheckInput)
	if err != nil {
		record = e.recorder.Finish(ctx, record, ToolCallStatusFailed, nil, 0, mapToolErrorCode(e.errorMapper, err))
		return &ToolExecutionResult{Metadata: metadata, ToolCall: record}, err
	}
	approvalGranted := approvalBypassAllowed(execCtx, name, precheckInput)
	if precheckResult != nil && (precheckResult.Deny || (precheckResult.ApprovalRequired && !approvalGranted)) {
		blockedErr := e.precheckBlockedError(*precheckResult)
		result := e.buildPrecheckBlockedResult(ctx, metadata, record, *precheckResult, blockedErr)
		return result, blockedErr
	}

	timeout := e.resolveTimeout(tool)
	execCtx.Timeout = timeout

	execCtx.Cancel = nil
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	execCtx.Cancel = cancel

	start := time.Now()
	toolResult, execErr := tool.Execute(callCtx, execCtx, input)
	duration := normalizeDuration(time.Since(start))

	if execErr != nil || errors.Is(callCtx.Err(), context.DeadlineExceeded) {
		status, finalErr := e.normalizeExecutionError(callCtx, execErr)
		result := e.buildErrorExecutionResult(ctx, metadata, name, toolResult, duration, record, status, finalErr)
		return result, finalErr
	}

	if toolResult == nil {
		toolResult = &ToolResult{}
	}
	toolResult.ToolName = name
	toolResult.Duration = duration

	outputSummary := firstNonEmptyMap(toolResult.SummaryOutput, toolResult.Output, toolResult.RawOutput)
	record = e.recorder.Finish(ctx, record, ToolCallStatusSucceeded, outputSummary, duration, nil)

	return &ToolExecutionResult{
		Metadata:      metadata,
		Precheck:      precheckResult,
		RawOutput:     toolResult.RawOutput,
		SummaryOutput: summarizeResultOutput(toolResult),
		Artifacts:     toolResult.Artifacts,
		Error:         toolResult.Error,
		Duration:      duration,
		ToolCall:      record,
	}, nil
}

// Execute keeps the legacy executor API for existing callers.
func (e *ToolExecutor) Execute(ctx context.Context, toolName string, execCtx *ToolExecuteContext, input map[string]any) (*ToolResult, error) {
	result, err := e.ExecuteToolWithContext(ctx, execCtx, toolName, input)
	if result == nil {
		return nil, err
	}
	return &ToolResult{
		ToolName:      toolName,
		RawOutput:     result.RawOutput,
		SummaryOutput: result.SummaryOutput,
		Artifacts:     result.Artifacts,
		Error:         result.Error,
		Duration:      result.Duration,
	}, err
}

// DryRun executes a tool in dry-run mode.
func (e *ToolExecutor) DryRun(ctx context.Context, toolName string, execCtx *ToolExecuteContext, input map[string]any) (*ToolResult, error) {
	tool, err := e.ResolveTool(toolName)
	if err != nil {
		return nil, err
	}

	dryRunTool, ok := tool.(DryRunTool)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrToolDryRunNotSupported, toolName)
	}

	if err := tool.Validate(input); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrToolValidationFailed, err)
	}

	return dryRunTool.DryRun(ctx, execCtx, input)
}

func (e *ToolExecutor) resolveTimeout(tool Tool) time.Duration {
	meta := tool.Metadata()
	if meta.TimeoutSec > 0 {
		return time.Duration(meta.TimeoutSec) * time.Second
	}
	return time.Duration(DefaultTimeoutSec) * time.Second
}

func (e *ToolExecutor) normalizeExecutionError(ctx context.Context, execErr error) (ToolCallStatus, error) {
	if errors.Is(execErr, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ToolCallStatusTimeout, fmt.Errorf("%w: %v", ErrToolExecutionTimeout, context.DeadlineExceeded)
	}
	if execErr == nil {
		execErr = ErrToolExecutionFailed
	}
	return ToolCallStatusFailed, errors.Join(ErrToolExecutionFailed, execErr)
}

func (e *ToolExecutor) buildErrorExecutionResult(ctx context.Context, metadata ToolMetadata, name string, toolResult *ToolResult, duration time.Duration, record ToolCallRecord, status ToolCallStatus, err error) *ToolExecutionResult {
	if toolResult == nil {
		toolResult = &ToolResult{}
	}
	toolResult.ToolName = name
	toolResult.Duration = duration
	if toolResult.Error == nil {
		toolResult.Error = &ToolResultError{Message: err.Error()}
	}
	toolResult.Error.Code = derefInt(mapToolErrorCode(e.errorMapper, err))

	outputSummary := firstNonEmptyMap(toolResult.SummaryOutput, toolResult.Output, toolResult.RawOutput)
	record = e.recorder.Finish(ctx, record, status, outputSummary, duration, mapToolErrorCode(e.errorMapper, err))

	return &ToolExecutionResult{
		Metadata:      metadata,
		Precheck:      nil,
		RawOutput:     toolResult.RawOutput,
		SummaryOutput: summarizeResultOutput(toolResult),
		Artifacts:     toolResult.Artifacts,
		Error:         toolResult.Error,
		Duration:      duration,
		ToolCall:      record,
	}
}

func (e *ToolExecutor) precheck(ctx context.Context, input RiskPrecheckInput) (*RiskPrecheckResult, error) {
	if e.prechecker == nil {
		return nil, nil
	}
	result, err := e.prechecker.Precheck(ctx, input)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (e *ToolExecutor) precheckBlockedError(result RiskPrecheckResult) error {
	if result.Deny {
		reason := strings.ToLower(result.DenyReason)
		switch {
		case strings.Contains(reason, "workspace"):
			return fmt.Errorf("%w: %s", ErrWorkspaceBoundaryDenied, result.DenyReason)
		case strings.Contains(reason, "command"):
			return fmt.Errorf("%w: %s", ErrCommandNotAllowed, result.DenyReason)
		default:
			return fmt.Errorf("%w: %s", ErrCapabilityDenied, result.DenyReason)
		}
	}
	return fmt.Errorf("%w: %s", ErrApprovalRequired, result.DenyReason)
}

func (e *ToolExecutor) buildPrecheckBlockedResult(ctx context.Context, metadata ToolMetadata, record ToolCallRecord, precheck RiskPrecheckResult, err error) *ToolExecutionResult {
	output := map[string]any{
		"risk_level":          precheck.RiskLevel,
		"approval_required":   precheck.ApprovalRequired,
		"checkpoint_required": precheck.CheckpointRequired,
		"deny":                precheck.Deny,
	}
	if precheck.Reason != "" {
		output["reason"] = precheck.Reason
	}
	if precheck.DenyReason != "" {
		output["deny_reason"] = precheck.DenyReason
	}
	if len(precheck.ImpactScope) > 0 {
		output["impact_scope"] = precheck.ImpactScope
	}

	record = e.recorder.Finish(ctx, record, ToolCallStatusFailed, output, time.Nanosecond, mapToolErrorCode(e.errorMapper, err))

	return &ToolExecutionResult{
		Metadata: metadata,
		Precheck: &precheck,
		Error: &ToolResultError{
			Code:    derefInt(mapToolErrorCode(e.errorMapper, err)),
			Message: err.Error(),
		},
		Duration: time.Nanosecond,
		ToolCall: record,
	}
}

func approvalBypassAllowed(execCtx *ToolExecuteContext, toolName string, precheckInput RiskPrecheckInput) bool {
	if execCtx == nil || !execCtx.ApprovalGranted {
		return false
	}
	if strings.TrimSpace(execCtx.ApprovedOperation) != "" && execCtx.ApprovedOperation != toolName {
		return false
	}
	if strings.TrimSpace(execCtx.ApprovedTargetObject) == "" {
		return true
	}
	target := strings.TrimSpace(precheckInput.Workspace.TargetPath)
	if target == "" {
		if toolName == "browser_tabs_list" {
			target = browserAttachKind(precheckInput.Input)
		}
	}
	if target == "" {
		target = strings.TrimSpace(precheckInput.Workspace.WorkspacePath)
	}
	if target == "" {
		return false
	}
	workspaceRoot := strings.TrimSpace(precheckInput.Workspace.WorkspacePath)
	if normalizeApprovalTarget(execCtx.ApprovedTargetObject, workspaceRoot) != normalizeApprovalTarget(target, workspaceRoot) {
		return false
	}
	if len(execCtx.ApprovedToolInput) == 0 {
		return true
	}
	return sameApprovedToolInput(execCtx.ApprovedToolInput, precheckInput.Input)
}

func normalizeApprovalTarget(target, workspaceRoot string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(target), "\\", "/")
	workspaceRoot = strings.Trim(strings.ReplaceAll(strings.TrimSpace(workspaceRoot), "\\", "/"), "/")
	normalized = strings.Trim(normalized, "/")
	if workspaceRoot != "" {
		if normalized == workspaceRoot {
			return "."
		}
		if strings.HasPrefix(normalized, workspaceRoot+"/") {
			normalized = strings.TrimPrefix(normalized, workspaceRoot+"/")
		}
	}
	normalized = strings.TrimPrefix(normalized, "workspace/")
	normalized = strings.TrimPrefix(normalized, "./")
	if normalized == "" {
		return "."
	}
	return strings.Trim(path.Clean(normalized), "/")
}

// sameApprovedToolInput compares the exact approved payload with the replayed
// tool input. Runtime agent-loop approvals persist the blocked tool input so a
// later allow_once decision only bypasses governance for the same payload.
func sameApprovedToolInput(approved, current map[string]any) bool {
	approvedJSON, approvedErr := json.Marshal(approved)
	currentJSON, currentErr := json.Marshal(current)
	if approvedErr != nil || currentErr != nil {
		return false
	}
	return string(approvedJSON) == string(currentJSON)
}

func normalizeDuration(duration time.Duration) time.Duration {
	if duration <= 0 {
		return time.Nanosecond
	}
	return duration
}

func summarizeResultOutput(result *ToolResult) map[string]any {
	if result == nil {
		return nil
	}
	return summarizeMap(firstNonEmptyMap(result.SummaryOutput, result.Output, result.RawOutput))
}

func firstNonEmptyMap(items ...map[string]any) map[string]any {
	for _, item := range items {
		if len(item) > 0 {
			return item
		}
	}
	return nil
}

func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
