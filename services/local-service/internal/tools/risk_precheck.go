package tools

import (
	"context"
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
	RiskLevel          string `json:"risk_level"`
	ApprovalRequired   bool   `json:"approval_required"`
	CheckpointRequired bool   `json:"checkpoint_required"`
	Deny               bool   `json:"deny"`
	DenyReason         string `json:"deny_reason,omitempty"`
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
		DenyReason:         assessment.Reason,
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

	targetPath, ok := extractTargetPath(input)
	if !ok {
		return precheckInput
	}

	precheckInput.Workspace.TargetPath = targetPath
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

	assessment := risksvc.AssessmentInput{
		OperationName:       input.ToolName,
		TargetObject:        input.Workspace.TargetPath,
		CapabilityAvailable: true,
		WorkspaceKnown:      workspaceKnown,
		CommandPreview:      normalizeCommandString(input.Input),
		ImpactScope: risksvc.ImpactScope{
			Files:          filesFromTarget(input.Workspace.TargetPath),
			OutOfWorkspace: outOfWorkspace,
		},
	}

	if input.ToolName == "write_file" {
		assessment.ImpactScope.OverwriteOrDeleteRisk = workspaceKnown && !outOfWorkspace
	}

	return assessment
}

func extractTargetPath(input map[string]any) (string, bool) {
	for _, key := range []string{"path", "target_path", "file_path"} {
		value, ok := input[key].(string)
		if ok && strings.TrimSpace(value) != "" {
			return value, true
		}
	}
	return "", false
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
