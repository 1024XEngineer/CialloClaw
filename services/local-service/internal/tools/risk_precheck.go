package tools

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	risksvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/risk"
)

const (
	RiskLevelGreen  = string(risksvc.RiskLevelGreen)
	RiskLevelYellow = string(risksvc.RiskLevelYellow)
	RiskLevelRed    = string(risksvc.RiskLevelRed)
)

// WorkspaceBoundaryInfo 描述当前工具调用涉及的工作区边界信息。
type WorkspaceBoundaryInfo struct {
	WorkspacePath string `json:"workspace_path,omitempty"`
	TargetPath    string `json:"target_path,omitempty"`
	Within        *bool  `json:"within_workspace,omitempty"`
	Exists        *bool  `json:"exists,omitempty"`
}

// PlatformCapabilityInfo 预留平台能力信息，后续可继续扩展审批/检查点能力接线。
type PlatformCapabilityInfo struct {
	Available                 bool `json:"available"`
	SupportsWorkspaceBoundary bool `json:"supports_workspace_boundary"`
}

// RiskPrecheckInput 是风险预检查的最小输入。
type RiskPrecheckInput struct {
	Metadata  ToolMetadata           `json:"metadata"`
	ToolName  string                 `json:"tool_name"`
	Input     map[string]any         `json:"input,omitempty"`
	Workspace WorkspaceBoundaryInfo  `json:"workspace"`
	Platform  PlatformCapabilityInfo `json:"platform"`
}

// RiskPrecheckResult 是风险预检查的最小输出。
type RiskPrecheckResult struct {
	RiskLevel          string         `json:"risk_level"`
	ApprovalRequired   bool           `json:"approval_required"`
	CheckpointRequired bool           `json:"checkpoint_required"`
	Deny               bool           `json:"deny"`
	Reason             string         `json:"reason,omitempty"`
	DenyReason         string         `json:"deny_reason,omitempty"`
	ImpactScope        map[string]any `json:"impact_scope,omitempty"`
}

// RiskPrechecker 在执行前完成本地风险判定，不直接触发工具执行。
type RiskPrechecker interface {
	Precheck(ctx context.Context, input RiskPrecheckInput) (RiskPrecheckResult, error)
}

// DefaultRiskPrechecker 提供最小可用的默认策略。
type DefaultRiskPrechecker struct {
	service *risksvc.Service
}

func NewDefaultRiskPrechecker(service *risksvc.Service) DefaultRiskPrechecker {
	return DefaultRiskPrechecker{service: service}
}

func (p DefaultRiskPrechecker) riskService() *risksvc.Service {
	if p.service != nil {
		return p.service
	}
	return risksvc.NewService()
}

// Precheck implements RiskPrechecker.
func (p DefaultRiskPrechecker) Precheck(_ context.Context, input RiskPrecheckInput) (RiskPrecheckResult, error) {
	assessment := p.riskService().Assess(buildAssessmentInput(input))
	return RiskPrecheckResult{
		RiskLevel:          string(assessment.RiskLevel),
		ApprovalRequired:   assessment.ApprovalRequired,
		CheckpointRequired: assessment.CheckpointRequired,
		Deny:               assessment.Deny,
		Reason:             assessment.Reason,
		DenyReason:         assessment.Reason,
		ImpactScope:        impactScopeMap(assessment.ImpactScope),
	}, nil
}

// BuildRiskPrecheckInput 从执行上下文中提取风险判定所需的最小信息。
func BuildRiskPrecheckInput(metadata ToolMetadata, toolName string, execCtx *ToolExecuteContext, input map[string]any) RiskPrecheckInput {
	precheckInput := RiskPrecheckInput{
		Metadata: metadata,
		ToolName: toolName,
		Input:    input,
	}

	if execCtx == nil {
		return precheckInput
	}

	precheckInput.Workspace.WorkspacePath = execCtx.WorkspacePath
	precheckInput.Platform = PlatformCapabilityInfo{
		Available:                 execCtx.Platform != nil,
		SupportsWorkspaceBoundary: execCtx.Platform != nil,
	}

	targetPath, ok := extractTargetPath(precheckInput.ToolName, precheckInput.Input)
	if !ok {
		return precheckInput
	}

	precheckInput.Workspace.TargetPath = targetPath
	if isWebpageTool(precheckInput.ToolName) {
		return precheckInput
	}
	if execCtx.Platform == nil {
		precheckInput.Workspace.Within = withinWorkspacePath(execCtx.WorkspacePath, targetPath)
		return precheckInput
	}

	safePath, ensureErr := execCtx.Platform.EnsureWithinWorkspace(targetPath)
	within := ensureErr == nil
	precheckInput.Workspace.Within = boolPtr(within)
	if ensureErr == nil {
		precheckInput.Workspace.TargetPath = safePath
		if absPath, err := execCtx.Platform.Abs(safePath); err == nil {
			precheckInput.Workspace.TargetPath = absPath
		}
		if _, statErr := execCtx.Platform.Stat(safePath); statErr == nil {
			precheckInput.Workspace.Exists = boolPtr(true)
		} else if errors.Is(statErr, fs.ErrNotExist) {
			precheckInput.Workspace.Exists = boolPtr(false)
		}
	}
	return precheckInput
}

func buildAssessmentInput(input RiskPrecheckInput) risksvc.AssessmentInput {
	outOfWorkspace := false
	workspaceKnown := false
	if input.Workspace.Within != nil {
		workspaceKnown = true
		outOfWorkspace = !*input.Workspace.Within
	}

	impactScope := risksvc.ImpactScope{
		OutOfWorkspace: outOfWorkspace,
	}
	targetObject := input.Workspace.TargetPath
	if isWebpageTool(input.ToolName) {
		impactScope.Webpages = webpagesFromTarget(input.Workspace.TargetPath)
		if browserKind := browserAttachKind(input.Input); browserKind != "" {
			impactScope.Apps = []string{browserKind}
		}
	} else {
		impactScope.Files = filesFromTarget(firstNonEmptyTarget(input.Workspace.TargetPath, input.Workspace.WorkspacePath))
	}

	assessment := risksvc.AssessmentInput{
		OperationName:       input.ToolName,
		TargetObject:        targetObject,
		CapabilityAvailable: true,
		WorkspaceKnown:      workspaceKnown,
		CommandPreview:      normalizeCommandString(input.Input),
		ImpactScope:         impactScope,
	}

	if isWorkspaceWriteOperation(input.ToolName) {
		exists := input.Workspace.Exists != nil && *input.Workspace.Exists
		assessment.ImpactScope.OverwriteOrDeleteRisk = workspaceKnown && !outOfWorkspace && exists
	}

	return assessment
}

func extractTargetPath(toolName string, input map[string]any) (string, bool) {
	if toolName == "exec_command" {
		if value, ok := input["working_dir"].(string); ok && strings.TrimSpace(value) != "" {
			return value, true
		}
	}
	if toolName == "browser_navigate" {
		if value, ok := input["url"].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value), true
		}
	}
	if isBrowserTool(toolName) {
		if value := browserAttachTarget(input); value != "" {
			return value, true
		}
	}
	if isWebpageTool(toolName) {
		if value, ok := input["url"].(string); ok && strings.TrimSpace(value) != "" {
			return value, true
		}
	}
	for _, key := range targetKeysForTool(toolName) {
		value, ok := input[key].(string)
		if ok && strings.TrimSpace(value) != "" {
			return value, true
		}
	}
	return "", false
}

func targetKeysForTool(toolName string) []string {
	switch strings.TrimSpace(toolName) {
	case "transcode_media", "normalize_recording":
		return []string{"output_path", "path", "target_path", "file_path"}
	case "extract_frames":
		return []string{"output_dir", "path", "target_path", "file_path"}
	default:
		return []string{"path", "target_path", "file_path"}
	}
}

func isWorkspaceWriteOperation(toolName string) bool {
	switch strings.TrimSpace(toolName) {
	case "write_file", "transcode_media", "normalize_recording", "extract_frames":
		return true
	default:
		return false
	}
}

func firstNonEmptyTarget(primary, fallback string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return fallback
}

func impactScopeMap(scope risksvc.ImpactScope) map[string]any {
	return map[string]any{
		"files":                    append([]string(nil), scope.Files...),
		"webpages":                 append([]string(nil), scope.Webpages...),
		"apps":                     append([]string(nil), scope.Apps...),
		"out_of_workspace":         scope.OutOfWorkspace,
		"overwrite_or_delete_risk": scope.OverwriteOrDeleteRisk,
	}
}

func normalizeCommandString(input map[string]any) string {
	for _, key := range []string{"command", "cmd"} {
		value, ok := input[key].(string)
		if ok {
			value = strings.TrimSpace(strings.ToLower(value))
			if value != "" {
				return value
			}
		}
	}
	return ""
}

func boolPtr(v bool) *bool {
	return &v
}

func filesFromTarget(target string) []string {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return nil
	}
	return []string{trimmed}
}

func webpagesFromTarget(target string) []string {
	trimmed := strings.TrimSpace(target)
	if trimmed == "" {
		return nil
	}
	if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
		return nil
	}
	return []string{trimmed}
}

func isWebpageTool(toolName string) bool {
	switch toolName {
	case "page_read", "page_search", "page_interact", "structured_dom", "browser_attach_current", "browser_snapshot", "browser_navigate", "browser_tabs_list", "browser_tab_focus", "browser_interact":
		return true
	default:
		return false
	}
}

func isBrowserTool(toolName string) bool {
	switch toolName {
	case "browser_attach_current", "browser_snapshot", "browser_navigate", "browser_tabs_list", "browser_tab_focus", "browser_interact":
		return true
	default:
		return false
	}
}

func browserAttachTarget(input map[string]any) string {
	attach, ok := input["attach"].(map[string]any)
	if !ok {
		return ""
	}
	target, ok := attach["target"].(map[string]any)
	if !ok {
		return ""
	}
	if value, ok := target["url"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	if value, ok := target["title_contains"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	if pageIndex, ok := browserAttachPageIndexValue(target["page_index"]); ok {
		return fmt.Sprintf("browser_tab:%d", pageIndex)
	}
	return ""
}

func browserAttachKind(input map[string]any) string {
	attach, ok := input["attach"].(map[string]any)
	if !ok {
		return ""
	}
	value, _ := attach["browser_kind"].(string)
	return strings.TrimSpace(value)
}

func browserAttachPageIndexValue(rawValue any) (int, bool) {
	switch typed := rawValue.(type) {
	case int:
		if typed >= 0 {
			return typed, true
		}
	case float64:
		if typed >= 0 && typed == float64(int(typed)) {
			return int(typed), true
		}
	}
	return 0, false
}

func withinWorkspacePath(workspacePath, targetPath string) *bool {
	if strings.TrimSpace(workspacePath) == "" || strings.TrimSpace(targetPath) == "" {
		return nil
	}

	workspacePath = filepath.Clean(workspacePath)
	targetPath = filepath.Clean(targetPath)
	rel, err := filepath.Rel(workspacePath, targetPath)
	if err != nil {
		return nil
	}
	within := rel == "." || (!strings.HasPrefix(rel, "..") && rel != "")
	return &within
}
