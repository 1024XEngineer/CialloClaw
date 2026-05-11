package risk

// RiskLevel is the canonical risk level used by backend risk decisions.
// Values must stay aligned with the frozen green/yellow/red protocol semantics.
type RiskLevel string

const (
	RiskLevelGreen  RiskLevel = "green"
	RiskLevelYellow RiskLevel = "yellow"
	RiskLevelRed    RiskLevel = "red"
)

const (
	ReasonOutOfWorkspace    = "out_of_workspace"
	ReasonOverwriteOrDelete = "overwrite_or_delete_risk"
	ReasonCommandNotAllowed = "command_not_allowed"
	ReasonCommandApproval   = "command_requires_approval"
	ReasonCapabilityDenied  = "capability_denied"
	ReasonWebpageApproval   = "webpage_requires_approval"
	ReasonWorkspaceUnknown  = "workspace_unknown"
	ReasonNormal            = "normal"
)

// ImpactScope is the module-local impact shape used before protocol mapping.
// It mirrors the protocol ImpactScope fields without replacing the source of truth.
type ImpactScope struct {
	Files                 []string
	Webpages              []string
	Apps                  []string
	OutOfWorkspace        bool
	OverwriteOrDeleteRisk bool
}

// AssessmentInput is the minimal data needed for one risk evaluation.
// It avoids pulling task state-machine details into the risk package.
type AssessmentInput struct {
	OperationName       string
	TargetObject        string
	CapabilityAvailable bool
	WorkspaceKnown      bool
	CommandPreview      string
	ImpactScope         ImpactScope
}

// AssessmentResult is the minimal risk decision returned to orchestrator.
// ApprovalRequired asks the caller to enter authorization, Deny blocks execution,
// and Reason stays a local explanation rather than a full protocol object.
type AssessmentResult struct {
	RiskLevel          RiskLevel
	ApprovalRequired   bool
	CheckpointRequired bool
	Deny               bool
	Reason             string
	ImpactScope        ImpactScope
}
