// This file contains the minimal risk assessment service skeleton.
package risk

import "strings"

// Service provides risk assessment capabilities for this module.
type Service struct{}

// NewService creates a Service instance.
func NewService() *Service {
	return &Service{}
}

// DefaultLevel returns the default risk level for ordinary operations.
func (s *Service) DefaultLevel() string {
	return string(RiskLevelGreen)
}

// Assess performs the minimal risk evaluation for a tool or operation request.
//
// The current rules stay conservative:
// 1. Capability unavailable => red + deny
// 2. Denied command matched => red + deny
// 3. Approval command matched => red + approval_required
// 4. Out of workspace => red + deny
// 5. Workspace write with unknown workspace facts => yellow + approval_required
// 6. Webpage and attached browser operations => explicit webpage policy
// 7. Overwrite/delete risk => yellow + checkpoint_required
// 8. Everything else => green
//
// Notes:
// - This service does not create ApprovalRequest records directly;
// - This service does not advance state machines;
// - This service only returns a stable, testable risk decision for callers.
func (s *Service) Assess(input AssessmentInput) AssessmentResult {
	result := AssessmentResult{
		RiskLevel:   RiskLevelGreen,
		Reason:      ReasonNormal,
		ImpactScope: input.ImpactScope,
	}

	if !input.CapabilityAvailable {
		result.RiskLevel = RiskLevelRed
		result.Deny = true
		result.Reason = ReasonCapabilityDenied
		return result
	}

	if isDeniedCommand(input.CommandPreview) {
		result.RiskLevel = RiskLevelRed
		result.Deny = true
		result.Reason = ReasonCommandNotAllowed
		return result
	}

	if isApprovalCommand(input.CommandPreview) {
		result.RiskLevel = RiskLevelRed
		result.ApprovalRequired = true
		result.CheckpointRequired = input.OperationName == "exec_command"
		result.Reason = ReasonCommandApproval
		return result
	}

	if input.ImpactScope.OutOfWorkspace {
		result.RiskLevel = RiskLevelRed
		result.Deny = true
		result.Reason = ReasonOutOfWorkspace
		return result
	}

	if isWorkspaceWriteOperation(input.OperationName) && (!input.WorkspaceKnown || strings.TrimSpace(input.TargetObject) == "") {
		result.RiskLevel = RiskLevelYellow
		result.ApprovalRequired = true
		result.Reason = ReasonWorkspaceUnknown
		return result
	}

	if isWebpageOperation(input.OperationName) {
		result.RiskLevel = RiskLevelYellow
		result.ApprovalRequired = true
		result.Reason = ReasonWebpageApproval
		return result
	}

	if input.OperationName == "exec_command" {
		result.RiskLevel = RiskLevelYellow
		result.ApprovalRequired = true
		result.CheckpointRequired = len(input.ImpactScope.Files) > 0
		result.Reason = ReasonCommandApproval
		return result
	}

	if input.ImpactScope.OverwriteOrDeleteRisk {
		result.RiskLevel = RiskLevelYellow
		result.ApprovalRequired = true
		result.CheckpointRequired = true
		result.Reason = ReasonOverwriteOrDelete
		return result
	}

	return result
}

func isDeniedCommand(commandPreview string) bool {
	preview := strings.ToLower(strings.TrimSpace(commandPreview))
	if preview == "" {
		return false
	}

	deniedPatterns := []string{
		"rm -rf",
		"del /f",
		"format ",
		"shutdown ",
		"powershell remove-item",
	}

	for _, pattern := range deniedPatterns {
		if strings.Contains(preview, pattern) {
			return true
		}
	}

	return false
}

func isApprovalCommand(commandPreview string) bool {
	preview := strings.ToLower(strings.TrimSpace(commandPreview))
	if preview == "" {
		return false
	}

	approvalPatterns := []string{
		"curl ",
		"wget ",
		"powershell",
		"chmod ",
		"chown ",
		"git clean",
	}

	for _, pattern := range approvalPatterns {
		if strings.Contains(preview, pattern) {
			return true
		}
	}

	return false
}

func isWebpageOperation(operationName string) bool {
	switch strings.TrimSpace(operationName) {
	case "page_read", "page_search", "page_interact", "structured_dom":
		return true
	case "browser_attach_current", "browser_snapshot", "browser_tabs_list", "browser_tab_focus":
		return true
	case "browser_navigate", "browser_interact":
		return true
	default:
		return false
	}
}

func isWorkspaceWriteOperation(operationName string) bool {
	switch strings.TrimSpace(operationName) {
	case "write_file", "transcode_media", "normalize_recording", "extract_frames":
		return true
	default:
		return false
	}
}
