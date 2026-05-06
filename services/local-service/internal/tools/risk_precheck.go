package tools

import (
	"context"
	"errors"
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

// WorkspaceBoundaryInfo describes the workspace boundary touched by a tool call.
type WorkspaceBoundaryInfo struct {
	WorkspacePath string `json:"workspace_path,omitempty"`
	TargetPath    string `json:"target_path,omitempty"`
	Within        *bool  `json:"within_workspace,omitempty"`
	Exists        *bool  `json:"exists,omitempty"`
}

// PlatformCapabilityInfo keeps platform capability facts for approval and checkpoint wiring.
type PlatformCapabilityInfo struct {
	Available                 bool `json:"available"`
	SupportsWorkspaceBoundary bool `json:"supports_workspace_boundary"`
}

// RiskPrecheckInput is the minimal input for a local risk precheck.
type RiskPrecheckInput struct {
	Metadata  ToolMetadata           `json:"metadata"`
	ToolName  string                 `json:"tool_name"`
	Input     map[string]any         `json:"input,omitempty"`
	Workspace WorkspaceBoundaryInfo  `json:"workspace"`
	Platform  PlatformCapabilityInfo `json:"platform"`
}

// RiskPrecheckResult is the minimal output from a local risk precheck.
type RiskPrecheckResult struct {
	RiskLevel          string         `json:"risk_level"`
	ApprovalRequired   bool           `json:"approval_required"`
	CheckpointRequired bool           `json:"checkpoint_required"`
	Deny               bool           `json:"deny"`
	Reason             string         `json:"reason,omitempty"`
	DenyReason         string         `json:"deny_reason,omitempty"`
	ImpactScope        map[string]any `json:"impact_scope,omitempty"`
}

// RiskPrechecker evaluates local risk before execution without invoking the tool.
type RiskPrechecker interface {
	Precheck(ctx context.Context, input RiskPrecheckInput) (RiskPrecheckResult, error)
}

// DefaultRiskPrechecker provides the default minimum viable policy.
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

// BuildRiskPrecheckInput extracts the minimal risk facts from the execution context.
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
			return value, true
		}
	}
	if isAttachedBrowserTool(toolName) {
		if value, ok := browserAttachTarget(input); ok {
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

func browserAttachTarget(input map[string]any) (string, bool) {
	attach, ok := input["attach"].(map[string]any)
	if !ok {
		return "", false
	}
	target, ok := attach["target"].(map[string]any)
	if !ok {
		return "", false
	}
	for _, key := range []string{"url", "title_contains"} {
		if value, ok := target[key].(string); ok && strings.TrimSpace(value) != "" {
			return value, true
		}
	}
	return "", false
}

func isWebpageTool(toolName string) bool {
	return isLegacyWebpageTool(toolName) || isAttachedBrowserTool(toolName)
}

func isLegacyWebpageTool(toolName string) bool {
	switch strings.TrimSpace(toolName) {
	case "page_read", "page_search", "page_interact", "structured_dom":
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
