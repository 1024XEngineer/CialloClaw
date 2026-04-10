package risk

// RiskLevel 定义风险等级。
//
// 取值严格对齐已冻结枚举：green / yellow / red。
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
	ReasonWorkspaceUnknown  = "workspace_unknown"
	ReasonNormal            = "normal"
)

// ImpactScope 是 risk 模块内部使用的最小影响面结构。
//
// 字段语义对齐 protocol 中已定义的 ImpactScope，
// 但本类型只在后端 risk 模块内部使用，不直接替代协议真源。
type ImpactScope struct {
	Files                 []string
	Webpages              []string
	Apps                  []string
	OutOfWorkspace        bool
	OverwriteOrDeleteRisk bool
}

// AssessmentInput 描述一次风险评估的最小输入。
//
// 本阶段只保留 risk 模块真正需要的判断信息，避免侵入主链路状态机。
type AssessmentInput struct {
	OperationName       string
	TargetObject        string
	CapabilityAvailable bool
	WorkspaceKnown      bool
	CommandPreview      string
	ImpactScope         ImpactScope
}

// AssessmentResult 描述一次风险评估的最小结果。
//
// approval_required 表示应交由上层进入人工确认流；
// deny 表示当前模块建议直接拦截，不进入执行；
// reason 用于上层构造统一错误或审批原因，但不直接等同完整协议对象。
type AssessmentResult struct {
	RiskLevel          RiskLevel
	ApprovalRequired   bool
	CheckpointRequired bool
	Deny               bool
	Reason             string
	ImpactScope        ImpactScope
}
