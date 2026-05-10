package orchestrator

// screenAnalysisApprovalState keeps the controlled screen authorization bundle
// typed until the runtime engine or storage boundary needs legacy map payloads.
type screenAnalysisApprovalState struct {
	ApprovalRequest  ApprovalRequestDTO
	PendingExecution screenAnalysisPendingExecution
	BubbleMessage    BubbleMessageDTO
}

type screenAnalysisPendingExecution struct {
	Kind          string                    `json:"kind"`
	OperationName string                    `json:"operation_name"`
	SourcePath    string                    `json:"source_path"`
	CaptureMode   string                    `json:"capture_mode"`
	Source        string                    `json:"source"`
	TargetObject  string                    `json:"target_object"`
	Language      string                    `json:"language"`
	EvidenceRole  string                    `json:"evidence_role"`
	DeliveryType  string                    `json:"delivery_type"`
	ResultTitle   string                    `json:"result_title"`
	PreviewText   string                    `json:"preview_text"`
	ImpactScope   screenAnalysisImpactScope `json:"impact_scope"`
}

type screenAnalysisImpactScope struct {
	Files                 []string `json:"files"`
	Webpages              []string `json:"webpages"`
	Apps                  []string `json:"apps"`
	OutOfWorkspace        bool     `json:"out_of_workspace"`
	OverwriteOrDeleteRisk bool     `json:"overwrite_or_delete_risk"`
}

func newScreenAnalysisApprovalState(approvalRequest map[string]any, pendingExecution screenAnalysisPendingExecution, bubble map[string]any) (screenAnalysisApprovalState, error) {
	approval, err := approvalRequestDTOFromMap(approvalRequest)
	if err != nil {
		return screenAnalysisApprovalState{}, err
	}
	bubbleMessage, err := bubbleMessageDTOFromMap(bubble)
	if err != nil {
		return screenAnalysisApprovalState{}, err
	}
	return screenAnalysisApprovalState{
		ApprovalRequest:  approval,
		PendingExecution: pendingExecution,
		BubbleMessage:    bubbleMessage,
	}, nil
}

func (state screenAnalysisApprovalState) approvalRequestMap() map[string]any {
	return protocolMapFromDTO(state.ApprovalRequest)
}

func (state screenAnalysisApprovalState) pendingExecutionMap() map[string]any {
	return protocolMapFromDTO(state.PendingExecution)
}

func (state screenAnalysisApprovalState) bubbleMessageMap() map[string]any {
	return protocolMapFromDTO(state.BubbleMessage)
}

func (scope screenAnalysisImpactScope) mapValue() map[string]any {
	return protocolMapFromDTO(screenAnalysisImpactScope{
		Files:                 cloneScreenAnalysisStrings(scope.Files),
		Webpages:              cloneScreenAnalysisStrings(scope.Webpages),
		Apps:                  cloneScreenAnalysisStrings(scope.Apps),
		OutOfWorkspace:        scope.OutOfWorkspace,
		OverwriteOrDeleteRisk: scope.OverwriteOrDeleteRisk,
	})
}

func cloneScreenAnalysisStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}
