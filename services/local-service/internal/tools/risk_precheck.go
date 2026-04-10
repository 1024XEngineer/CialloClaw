package tools

import (
	"context"
	"path/filepath"
	"strings"
)

const (
	RiskLevelGreen  = "green"
	RiskLevelYellow = "yellow"
	RiskLevelRed    = "red"
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
type DefaultRiskPrechecker struct{}

// Precheck implements RiskPrechecker.
func (DefaultRiskPrechecker) Precheck(_ context.Context, input RiskPrecheckInput) (RiskPrecheckResult, error) {
	result := RiskPrecheckResult{RiskLevel: RiskLevelGreen}

	switch input.ToolName {
	case "read_file":
		return result, nil
	case "write_file":
		return evaluateWriteFileRisk(input), nil
	case "exec_command":
		return evaluateExecCommandRisk(input), nil
	default:
		if input.Metadata.RiskHint == RiskLevelRed {
			result.RiskLevel = RiskLevelRed
			result.ApprovalRequired = true
		}
		return result, nil
	}
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

func evaluateWriteFileRisk(input RiskPrecheckInput) RiskPrecheckResult {
	result := RiskPrecheckResult{
		RiskLevel:          RiskLevelYellow,
		CheckpointRequired: true,
	}

	if input.Workspace.TargetPath == "" {
		result.ApprovalRequired = true
		result.DenyReason = "write target path is missing"
		return result
	}

	if input.Workspace.Within == nil {
		result.ApprovalRequired = true
		result.DenyReason = "workspace boundary is unknown"
		return result
	}

	if !*input.Workspace.Within {
		result.RiskLevel = RiskLevelRed
		result.Deny = true
		result.DenyReason = "write target is outside workspace boundary"
	}

	return result
}

func evaluateExecCommandRisk(input RiskPrecheckInput) RiskPrecheckResult {
	command := normalizeCommandString(input.Input)
	if command == "" {
		return RiskPrecheckResult{
			RiskLevel:        RiskLevelYellow,
			ApprovalRequired: true,
			DenyReason:       "command content is missing",
		}
	}

	if matchesDangerousCommand(command) {
		return RiskPrecheckResult{
			RiskLevel:  RiskLevelRed,
			Deny:       true,
			DenyReason: "command is blocked by local risk policy",
		}
	}

	if matchesApprovalCommand(command) {
		return RiskPrecheckResult{
			RiskLevel:        RiskLevelRed,
			ApprovalRequired: true,
			DenyReason:       "command requires approval before execution",
		}
	}

	return RiskPrecheckResult{RiskLevel: RiskLevelYellow}
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

func matchesDangerousCommand(command string) bool {
	patterns := []string{
		"rm -rf",
		"del /f",
		"rd /s /q",
		"format ",
		"mkfs",
		"shutdown",
		"reboot",
		"diskpart",
	}
	return matchesAnyPattern(command, patterns)
}

func matchesApprovalCommand(command string) bool {
	patterns := []string{
		"curl ",
		"wget ",
		"powershell",
		"chmod ",
		"chown ",
		"git clean",
	}
	return matchesAnyPattern(command, patterns)
}

func matchesAnyPattern(command string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(command, pattern) {
			return true
		}
	}
	return false
}

func boolPtr(v bool) *bool {
	return &v
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
