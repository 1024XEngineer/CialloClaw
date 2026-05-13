package risk

import (
	"strings"
	"testing"
)

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
				ImpactScope:         ImpactScope{Files: []string{"D:/workspace/notes/demo.txt"}},
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
			want: AssessmentResult{RiskLevel: RiskLevelRed, Deny: true, Reason: ReasonCapabilityDenied},
		},
		{
			name: "command_not_allowed_red",
			input: AssessmentInput{
				OperationName:       "exec_command",
				CapabilityAvailable: true,
				CommandPreview:      "rm -rf /tmp/demo",
			},
			want: AssessmentResult{RiskLevel: RiskLevelRed, Deny: true, Reason: ReasonCommandNotAllowed},
		},
		{
			name: "command_requires_approval_red",
			input: AssessmentInput{
				OperationName:       "exec_command",
				CapabilityAvailable: true,
				CommandPreview:      "powershell Get-Process",
			},
			want: AssessmentResult{RiskLevel: RiskLevelRed, ApprovalRequired: true, CheckpointRequired: true, Reason: ReasonCommandApproval},
		},
		{
			name: "safe_command_still_requires_approval",
			input: AssessmentInput{
				OperationName:       "exec_command",
				TargetObject:        "D:/workspace",
				CapabilityAvailable: true,
				WorkspaceKnown:      true,
				ImpactScope:         ImpactScope{Files: []string{"D:/workspace"}},
			},
			want: AssessmentResult{
				RiskLevel:          RiskLevelYellow,
				ApprovalRequired:   true,
				CheckpointRequired: true,
				Reason:             ReasonCommandApproval,
				ImpactScope:        ImpactScope{Files: []string{"D:/workspace"}},
			},
		},
		{
			name: "out_of_workspace_denied",
			input: AssessmentInput{
				OperationName:       "write_file",
				TargetObject:        "D:/outside/report.md",
				CapabilityAvailable: true,
				WorkspaceKnown:      true,
				ImpactScope:         ImpactScope{Files: []string{"D:/outside/report.md"}, OutOfWorkspace: true},
			},
			want: AssessmentResult{
				RiskLevel:   RiskLevelRed,
				Deny:        true,
				Reason:      ReasonOutOfWorkspace,
				ImpactScope: ImpactScope{Files: []string{"D:/outside/report.md"}, OutOfWorkspace: true},
			},
		},
		{
			name: "webpage_read_stays_low_risk",
			input: AssessmentInput{
				OperationName:       "page_read",
				TargetObject:        "https://example.com/demo",
				CapabilityAvailable: true,
				ImpactScope:         ImpactScope{Webpages: []string{"https://example.com/demo"}},
			},
			want: AssessmentResult{RiskLevel: RiskLevelGreen, Reason: ReasonNormal, ImpactScope: ImpactScope{Webpages: []string{"https://example.com/demo"}}},
		},
		{
			name: "webpage_read_loopback_requires_approval",
			input: AssessmentInput{
				OperationName:       "page_read",
				TargetObject:        "http://127.0.0.1:8080/admin",
				CapabilityAvailable: true,
				ImpactScope:         ImpactScope{Webpages: []string{"http://127.0.0.1:8080/admin"}},
			},
			want: AssessmentResult{RiskLevel: RiskLevelYellow, ApprovalRequired: true, Reason: ReasonWebpageApproval, ImpactScope: ImpactScope{Webpages: []string{"http://127.0.0.1:8080/admin"}}},
		},
		{
			name: "webpage_search_private_network_requires_approval",
			input: AssessmentInput{
				OperationName:       "page_search",
				TargetObject:        "http://192.168.1.20/admin",
				CapabilityAvailable: true,
				ImpactScope:         ImpactScope{Webpages: []string{"http://192.168.1.20/admin"}},
			},
			want: AssessmentResult{RiskLevel: RiskLevelYellow, ApprovalRequired: true, Reason: ReasonWebpageApproval, ImpactScope: ImpactScope{Webpages: []string{"http://192.168.1.20/admin"}}},
		},
		{
			name: "browser_snapshot_is_low_risk_observation",
			input: AssessmentInput{
				OperationName:       "browser_snapshot",
				TargetObject:        "https://example.com/demo",
				CapabilityAvailable: true,
				ImpactScope:         ImpactScope{Webpages: []string{"https://example.com/demo"}, Apps: []string{"chrome"}},
			},
			want: AssessmentResult{RiskLevel: RiskLevelGreen, Reason: ReasonNormal, ImpactScope: ImpactScope{Webpages: []string{"https://example.com/demo"}, Apps: []string{"chrome"}}},
		},
		{
			name: "browser_tabs_list_stays_low_risk",
			input: AssessmentInput{
				OperationName:       "browser_tabs_list",
				TargetObject:        "browser_tab:0",
				CapabilityAvailable: true,
				ImpactScope:         ImpactScope{Apps: []string{"chrome"}},
			},
			want: AssessmentResult{RiskLevel: RiskLevelGreen, Reason: ReasonNormal, ImpactScope: ImpactScope{Apps: []string{"chrome"}}},
		},
		{
			name: "browser_navigate_public_target_stays_low_risk",
			input: AssessmentInput{
				OperationName:       "browser_navigate",
				TargetObject:        "https://example.com/next",
				CapabilityAvailable: true,
				ImpactScope:         ImpactScope{Webpages: []string{"https://example.com/next"}, Apps: []string{"edge"}},
			},
			want: AssessmentResult{RiskLevel: RiskLevelGreen, Reason: ReasonNormal, ImpactScope: ImpactScope{Webpages: []string{"https://example.com/next"}, Apps: []string{"edge"}}},
		},
		{
			name: "browser_tab_focus_stays_low_risk",
			input: AssessmentInput{
				OperationName:       "browser_tab_focus",
				TargetObject:        "browser_tab:2",
				CapabilityAvailable: true,
				ImpactScope:         ImpactScope{Webpages: []string{"browser_tab:2"}, Apps: []string{"chrome"}},
			},
			want: AssessmentResult{RiskLevel: RiskLevelGreen, Reason: ReasonNormal, ImpactScope: ImpactScope{Webpages: []string{"browser_tab:2"}, Apps: []string{"chrome"}}},
		},
		{
			name: "browser_interact_requires_approval",
			input: AssessmentInput{
				OperationName:       "browser_interact",
				TargetObject:        "Example Docs",
				CapabilityAvailable: true,
				ImpactScope:         ImpactScope{Webpages: []string{"Example Docs"}},
			},
			want: AssessmentResult{RiskLevel: RiskLevelYellow, ApprovalRequired: true, Reason: ReasonWebpageApproval, ImpactScope: ImpactScope{Webpages: []string{"Example Docs"}}},
		},
		{
			name: "browser_snapshot_is_low_risk_observation",
			input: AssessmentInput{
				OperationName:       "browser_snapshot",
				TargetObject:        "https://example.com/demo",
				CapabilityAvailable: true,
				ImpactScope: ImpactScope{
					Webpages: []string{"https://example.com/demo"},
					Apps:     []string{"chrome"},
				},
			},
			want: AssessmentResult{
				RiskLevel: RiskLevelGreen,
				Reason:    ReasonNormal,
				ImpactScope: ImpactScope{
					Webpages: []string{"https://example.com/demo"},
					Apps:     []string{"chrome"},
				},
			},
		},
		{
			name: "browser_navigate_public_target_stays_low_risk_duplicate",
			input: AssessmentInput{
				OperationName:       "browser_navigate",
				TargetObject:        "https://example.com/next",
				CapabilityAvailable: true,
				ImpactScope: ImpactScope{
					Webpages: []string{"https://example.com/next"},
					Apps:     []string{"edge"},
				},
			},
			want: AssessmentResult{
				RiskLevel: RiskLevelGreen,
				Reason:    ReasonNormal,
				ImpactScope: ImpactScope{
					Webpages: []string{"https://example.com/next"},
					Apps:     []string{"edge"},
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
			want: AssessmentResult{RiskLevel: RiskLevelYellow, ApprovalRequired: true, Reason: ReasonWorkspaceUnknown},
		},
		{
			name: "transcode_media_unknown_workspace_requires_approval",
			input: AssessmentInput{
				OperationName:       "transcode_media",
				TargetObject:        "",
				CapabilityAvailable: true,
				WorkspaceKnown:      false,
			},
			want: AssessmentResult{RiskLevel: RiskLevelYellow, ApprovalRequired: true, Reason: ReasonWorkspaceUnknown},
		},
		{
			name: "overwrite_requires_checkpoint",
			input: AssessmentInput{
				OperationName:       "write_file",
				TargetObject:        "D:/workspace/report.md",
				CapabilityAvailable: true,
				WorkspaceKnown:      true,
				ImpactScope:         ImpactScope{Files: []string{"D:/workspace/report.md"}, OverwriteOrDeleteRisk: true},
			},
			want: AssessmentResult{
				RiskLevel:          RiskLevelYellow,
				ApprovalRequired:   true,
				CheckpointRequired: true,
				Reason:             ReasonOverwriteOrDelete,
				ImpactScope:        ImpactScope{Files: []string{"D:/workspace/report.md"}, OverwriteOrDeleteRisk: true},
			},
		},
		{
			name: "extract_frames_existing_output_requires_checkpoint",
			input: AssessmentInput{
				OperationName:       "extract_frames",
				TargetObject:        "D:/workspace/frames",
				CapabilityAvailable: true,
				WorkspaceKnown:      true,
				ImpactScope:         ImpactScope{Files: []string{"D:/workspace/frames"}, OverwriteOrDeleteRisk: true},
			},
			want: AssessmentResult{
				RiskLevel:          RiskLevelYellow,
				ApprovalRequired:   true,
				CheckpointRequired: true,
				Reason:             ReasonOverwriteOrDelete,
				ImpactScope:        ImpactScope{Files: []string{"D:/workspace/frames"}, OverwriteOrDeleteRisk: true},
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
			if strings.Join(got.ImpactScope.Webpages, "\n") != strings.Join(tc.want.ImpactScope.Webpages, "\n") {
				t.Fatalf("expected webpages %+v, got %+v", tc.want.ImpactScope.Webpages, got.ImpactScope.Webpages)
			}
		})
	}
}

func TestServiceSensitiveWebTargetClassification(t *testing.T) {
	service := NewService()

	tests := []struct {
		name   string
		target string
		want   bool
	}{
		{name: "public_hostname_is_low_risk", target: "https://public.example.com/docs", want: false},
		{name: "multi_label_hostname_stays_low_risk_without_dns_lookup", target: "https://blog.csdn.net/csdnnews/article/details/160669079", want: false},
		{name: "benchmark_range_ip_stays_low_risk", target: "http://198.18.0.58/demo", want: false},
		{name: "public_ipv6_literal_is_low_risk", target: "https://[2606:2800:220:1:248:1893:25c8:1946]/docs", want: false},
		{name: "single_label_hostname_requires_approval", target: "http://intranet", want: true},
		{name: "local_suffix_requires_approval", target: "http://printer.local/status", want: true},
		{name: "docker_internal_suffix_requires_approval", target: "http://host.docker.internal", want: true},
		{name: "carrier_grade_nat_literal_requires_approval", target: "http://100.64.1.10/demo", want: true},
		{name: "trailing_dot_hostname_normalizes_without_lookup", target: "https://trailing-dot.example.com./docs", want: false},
		{name: "mixed_case_hostname_normalizes_without_lookup", target: "https://MIXED-CASE.EXAMPLE.COM/docs", want: false},
		{name: "unsupported_scheme_requires_approval", target: "ftp://public.example.com/archive", want: true},
		{name: "invalid_url_requires_approval", target: "://bad", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := service.isSensitiveWebTarget(tc.target); got != tc.want {
				t.Fatalf("expected sensitive=%v, got %v for %q", tc.want, got, tc.target)
			}
		})
	}
}
