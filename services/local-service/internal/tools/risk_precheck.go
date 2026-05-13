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

// WorkspaceBoundaryInfo captures the workspace boundary state for one tool call.
type WorkspaceBoundaryInfo struct {
	WorkspacePath string `json:"workspace_path,omitempty"`
	TargetPath    string `json:"target_path,omitempty"`
	Within        *bool  `json:"within_workspace,omitempty"`
	Exists        *bool  `json:"exists,omitempty"`
}

// PlatformCapabilityInfo records the platform capabilities available to the
// precheck stage.
type PlatformCapabilityInfo struct {
	Available                 bool `json:"available"`
	SupportsWorkspaceBoundary bool `json:"supports_workspace_boundary"`
}

// RiskPrecheckInput is the minimal input required for one local governance
// precheck.
type RiskPrecheckInput struct {
	Metadata  ToolMetadata           `json:"metadata"`
	ToolName  string                 `json:"tool_name"`
	Input     map[string]any         `json:"input,omitempty"`
	Workspace WorkspaceBoundaryInfo  `json:"workspace"`
	Platform  PlatformCapabilityInfo `json:"platform"`
}

// RiskPrecheckResult is the minimal output produced by one local governance
// precheck.
type RiskPrecheckResult struct {
	RiskLevel          string         `json:"risk_level"`
	ApprovalRequired   bool           `json:"approval_required"`
	CheckpointRequired bool           `json:"checkpoint_required"`
	Deny               bool           `json:"deny"`
	Reason             string         `json:"reason,omitempty"`
	DenyReason         string         `json:"deny_reason,omitempty"`
	ImpactScope        map[string]any `json:"impact_scope,omitempty"`
}

// RiskPrechecker performs local governance classification before any tool
// execution starts.
type RiskPrechecker interface {
	Precheck(ctx context.Context, input RiskPrecheckInput) (RiskPrecheckResult, error)
}

// DefaultRiskPrechecker provides the default bounded local governance policy.
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
	result := RiskPrecheckResult{
		RiskLevel:          string(assessment.RiskLevel),
		ApprovalRequired:   assessment.ApprovalRequired,
		CheckpointRequired: assessment.CheckpointRequired,
		Deny:               assessment.Deny,
		Reason:             assessment.Reason,
		DenyReason:         assessment.Reason,
		ImpactScope:        impactScopeMap(assessment.ImpactScope),
	}
	if requiresBrowserEndpointApproval(input.ToolName, input.Input) && !result.Deny {
		result.ApprovalRequired = true
		if result.RiskLevel == "" || result.RiskLevel == RiskLevelGreen {
			result.RiskLevel = RiskLevelYellow
		}
		if strings.TrimSpace(result.Reason) == "" || result.Reason == risksvc.ReasonNormal {
			result.Reason = risksvc.ReasonWebpageApproval
		}
		result.DenyReason = result.Reason
	}
	return result, nil
}

// BuildRiskPrecheckInput extracts the minimum governance context from one tool
// execution request.
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
	if ensureErr == nil && isWorkspaceWriteOperation(precheckInput.ToolName) {
		if workspaceWithin := withinWorkspacePath(execCtx.WorkspacePath, safePath); workspaceWithin != nil {
			within = *workspaceWithin
		}
	}
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

	impactScope := risksvc.ImpactScope{OutOfWorkspace: outOfWorkspace}
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
	if isAttachedBrowserTool(toolName) {
		if value := browserAttachTargetForTool(toolName, input); value != "" {
			return value, true
		}
	}
	if isLegacyWebpageTool(toolName) {
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
	return []string{trimmed}
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
	if pageIndex, ok := browserAttachPageIndexValue(target["page_index"]); ok {
		return fmt.Sprintf("browser_tab:%d", pageIndex)
	}
	if value, ok := target["title_contains"].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return ""
}

func browserAttachTargetForTool(toolName string, input map[string]any) string {
	target := browserAttachTarget(input)
	if target == "" {
		return ""
	}
	if requiresStableBrowserTarget(toolName) && !isStableBrowserTarget(target) {
		return ""
	}
	return target
}

func browserAttachKind(input map[string]any) string {
	attach, ok := input["attach"].(map[string]any)
	if !ok {
		return ""
	}
	value, _ := attach["browser_kind"].(string)
	return strings.TrimSpace(value)
}

func requiresBrowserEndpointApproval(toolName string, input map[string]any) bool {
	switch strings.TrimSpace(toolName) {
	case "browser_attach_current", "browser_snapshot", "browser_navigate", "browser_tabs_list", "browser_tab_focus", "browser_interact":
		attach, ok := input["attach"].(map[string]any)
		if !ok {
			return false
		}
		endpointURL, _ := attach["endpoint_url"].(string)
		return strings.TrimSpace(endpointURL) != ""
	default:
		return false
	}
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

func isWebpageTool(toolName string) bool {
	return isLegacyWebpageTool(toolName) || isAttachedBrowserTool(toolName)
}

func isLegacyWebpageTool(toolName string) bool {
	switch strings.TrimSpace(toolName) {
	case "page_read", "page_search", "web_search", "page_interact":
		return true
	default:
		return false
	}
}

func isAttachedBrowserTool(toolName string) bool {
	switch strings.TrimSpace(toolName) {
	case "browser_attach_current", "browser_snapshot", "browser_navigate", "browser_tabs_list", "browser_tab_focus", "browser_interact":
		return true
	default:
		return false
	}
}

func requiresStableBrowserTarget(toolName string) bool {
	switch strings.TrimSpace(toolName) {
	case "browser_navigate", "browser_tab_focus", "browser_interact":
		return true
	default:
		return false
	}
}

func isStableBrowserTarget(target string) bool {
	trimmed := strings.TrimSpace(target)
	return strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") || strings.HasPrefix(trimmed, "browser_tab:")
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
