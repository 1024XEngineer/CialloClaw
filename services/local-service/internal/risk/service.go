// Package risk implements the minimal governance assessment layer.
package risk

import (
	"net"
	"net/url"
	"strings"
)

// Service evaluates tool risk without mutating orchestrator state.
type Service struct{}

// NewService constructs a minimal risk assessment service.
func NewService() *Service {
	return &Service{}
}

// DefaultLevel returns the default risk level used by callers that need a
// stable fallback before any assessment runs.
func (s *Service) DefaultLevel() string {
	return string(RiskLevelGreen)
}

// Assess returns a stable, testable governance decision for one tool request.
// It does not allocate approval records or mutate any task state.
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

	if isLowRiskBrowserObservationOperation(input.OperationName) {
		return result
	}

	if requiresApprovalForSensitiveWebTarget(input.OperationName, input.TargetObject) {
		result.RiskLevel = RiskLevelYellow
		result.ApprovalRequired = true
		result.Reason = ReasonWebpageApproval
		return result
	}

	if isApprovalBrowserOperation(input.OperationName) {
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

func isApprovalBrowserOperation(operationName string) bool {
	switch strings.TrimSpace(operationName) {
	case "page_interact", "structured_dom", "browser_navigate", "browser_tabs_list", "browser_tab_focus", "browser_interact":
		return true
	default:
		return false
	}
}

func isLowRiskBrowserObservationOperation(operationName string) bool {
	switch strings.TrimSpace(operationName) {
	case "browser_attach_current", "browser_snapshot":
		return true
	default:
		return false
	}
}

// requiresApprovalForSensitiveWebTarget keeps read-only web tools low risk for
// ordinary public pages while restoring authorization for local, loopback, and
// private-network targets that could expose host-only services through the
// browser sidecar path.
func requiresApprovalForSensitiveWebTarget(operationName, targetObject string) bool {
	switch strings.TrimSpace(operationName) {
	case "page_read", "page_search":
		return isSensitiveWebTarget(targetObject)
	default:
		return false
	}
}

func isSensitiveWebTarget(targetObject string) bool {
	parsed, err := url.Parse(strings.TrimSpace(targetObject))
	if err != nil {
		return true
	}
	if scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme)); scheme != "http" && scheme != "https" {
		return true
	}
	hostname := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if hostname == "" {
		return true
	}
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") {
		return true
	}
	ip := net.ParseIP(hostname)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}

func isWorkspaceWriteOperation(operationName string) bool {
	switch strings.TrimSpace(operationName) {
	case "write_file", "transcode_media", "normalize_recording", "extract_frames":
		return true
	default:
		return false
	}
}
