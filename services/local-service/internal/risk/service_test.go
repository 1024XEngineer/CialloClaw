package risk

import "testing"

func TestServiceDefaultLevel(t *testing.T) {
	service := NewService()

	if service.DefaultLevel() != "green" {
		t.Fatalf("expected green default level, got %q", service.DefaultLevel())
	}
}

func TestServiceAssess(t *testing.T) {
	service := NewService()

	tests := []struct {
		name  string
		input AssessmentInput
		want  AssessmentResult
	}{
		{
			name: "normal_operation_green",
			input: AssessmentInput{
				OperationName:       "read_file",
				TargetObject:        "D:/workspace/notes/demo.txt",
				CapabilityAvailable: true,
				ImpactScope: ImpactScope{
					Files: []string{"D:/workspace/notes/demo.txt"},
				},
			},
			want: AssessmentResult{
				RiskLevel:   RiskLevelGreen,
				Reason:      ReasonNormal,
				ImpactScope: ImpactScope{Files: []string{"D:/workspace/notes/demo.txt"}},
			},
		},
		{
			name: "capability_denied_red",
			input: AssessmentInput{
				OperationName:       "write_file",
				TargetObject:        "D:/workspace/report.md",
				CapabilityAvailable: false,
			},
			want: AssessmentResult{
				RiskLevel: RiskLevelRed,
				Deny:      true,
				Reason:    ReasonCapabilityDenied,
			},
		},
		{
			name: "command_not_allowed_red",
			input: AssessmentInput{
				OperationName:       "exec_command",
				CapabilityAvailable: true,
				CommandPreview:      "rm -rf /tmp/demo",
			},
			want: AssessmentResult{
				RiskLevel: RiskLevelRed,
				Deny:      true,
				Reason:    ReasonCommandNotAllowed,
			},
		},
		{
			name: "command_requires_approval_red",
			input: AssessmentInput{
				OperationName:       "exec_command",
				CapabilityAvailable: true,
				CommandPreview:      "powershell Get-Process",
			},
			want: AssessmentResult{
				RiskLevel:        RiskLevelRed,
				ApprovalRequired: true,
				Reason:           ReasonCommandApproval,
			},
		},
		{
			name: "out_of_workspace_denied",
			input: AssessmentInput{
				OperationName:       "write_file",
				TargetObject:        "D:/outside/report.md",
				CapabilityAvailable: true,
				WorkspaceKnown:      true,
				ImpactScope: ImpactScope{
					Files:          []string{"D:/outside/report.md"},
					OutOfWorkspace: true,
				},
			},
			want: AssessmentResult{
				RiskLevel: RiskLevelRed,
				Deny:      true,
				Reason:    ReasonOutOfWorkspace,
				ImpactScope: ImpactScope{
					Files:          []string{"D:/outside/report.md"},
					OutOfWorkspace: true,
				},
			},
		},
		{
			name: "write_file_unknown_workspace_requires_approval",
			input: AssessmentInput{
				OperationName:       "write_file",
				TargetObject:        "",
				CapabilityAvailable: true,
				WorkspaceKnown:      false,
			},
			want: AssessmentResult{
				RiskLevel:        RiskLevelYellow,
				ApprovalRequired: true,
				Reason:           ReasonWorkspaceUnknown,
			},
		},
		{
			name: "overwrite_requires_checkpoint",
			input: AssessmentInput{
				OperationName:       "write_file",
				TargetObject:        "D:/workspace/report.md",
				CapabilityAvailable: true,
				WorkspaceKnown:      true,
				ImpactScope: ImpactScope{
					Files:                 []string{"D:/workspace/report.md"},
					OverwriteOrDeleteRisk: true,
				},
			},
			want: AssessmentResult{
				RiskLevel:          RiskLevelYellow,
				CheckpointRequired: true,
				Reason:             ReasonOverwriteOrDelete,
				ImpactScope: ImpactScope{
					Files:                 []string{"D:/workspace/report.md"},
					OverwriteOrDeleteRisk: true,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := service.Assess(tc.input)

			if got.RiskLevel != tc.want.RiskLevel {
				t.Fatalf("expected risk level %q, got %q", tc.want.RiskLevel, got.RiskLevel)
			}
			if got.ApprovalRequired != tc.want.ApprovalRequired {
				t.Fatalf("expected approval_required %v, got %v", tc.want.ApprovalRequired, got.ApprovalRequired)
			}
			if got.CheckpointRequired != tc.want.CheckpointRequired {
				t.Fatalf("expected checkpoint_required %v, got %v", tc.want.CheckpointRequired, got.CheckpointRequired)
			}
			if got.Deny != tc.want.Deny {
				t.Fatalf("expected deny %v, got %v", tc.want.Deny, got.Deny)
			}
			if got.Reason != tc.want.Reason {
				t.Fatalf("expected reason %q, got %q", tc.want.Reason, got.Reason)
			}
			if got.ImpactScope.OutOfWorkspace != tc.want.ImpactScope.OutOfWorkspace {
				t.Fatalf("expected out_of_workspace %v, got %v", tc.want.ImpactScope.OutOfWorkspace, got.ImpactScope.OutOfWorkspace)
			}
			if got.ImpactScope.OverwriteOrDeleteRisk != tc.want.ImpactScope.OverwriteOrDeleteRisk {
				t.Fatalf("expected overwrite_or_delete_risk %v, got %v", tc.want.ImpactScope.OverwriteOrDeleteRisk, got.ImpactScope.OverwriteOrDeleteRisk)
			}
		})
	}
}
