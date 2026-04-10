// 该文件负责风险评估层的最小骨架。
package risk

import "strings"

// Service 提供当前模块的服务能力。
type Service struct{}

// NewService 创建并返回Service。
func NewService() *Service {
	return &Service{}
}

// DefaultLevel 处理当前模块的相关逻辑。
func (s *Service) DefaultLevel() string {
	return string(RiskLevelGreen)
}

// Assess 对一次工具或操作请求进行最小风险评估。
//
// 当前规则保持保守：
// 1. 能力不可用 => red + deny
// 2. 命中危险命令 => red + deny
// 3. 超出工作区 => red + approval_required
// 4. 存在覆盖/删除风险 => yellow + approval_required
// 5. 其他 => green
//
// 注意：
// - 这里不直接生成 ApprovalRequest；
// - 这里不推进状态机；
// - 这里只给上层一个稳定、可测试的风险判断结果。
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

	if input.ImpactScope.OutOfWorkspace {
		result.RiskLevel = RiskLevelRed
		result.ApprovalRequired = true
		result.Reason = ReasonOutOfWorkspace
		return result
	}

	if input.ImpactScope.OverwriteOrDeleteRisk {
		result.RiskLevel = RiskLevelYellow
		result.ApprovalRequired = true
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
