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
	return map[string]any{
		"approval_id":    state.ApprovalRequest.ApprovalID,
		"task_id":        state.ApprovalRequest.TaskID,
		"operation_name": state.ApprovalRequest.OperationName,
		"risk_level":     state.ApprovalRequest.RiskLevel,
		"target_object":  state.ApprovalRequest.TargetObject,
		"reason":         state.ApprovalRequest.Reason,
		"status":         state.ApprovalRequest.Status,
		"created_at":     state.ApprovalRequest.CreatedAt,
	}
}

func (state screenAnalysisApprovalState) pendingExecutionMap() map[string]any {
	return map[string]any{
		"kind":           state.PendingExecution.Kind,
		"operation_name": state.PendingExecution.OperationName,
		"source_path":    state.PendingExecution.SourcePath,
		"capture_mode":   state.PendingExecution.CaptureMode,
		"source":         state.PendingExecution.Source,
		"target_object":  state.PendingExecution.TargetObject,
		"language":       state.PendingExecution.Language,
		"evidence_role":  state.PendingExecution.EvidenceRole,
		"delivery_type":  state.PendingExecution.DeliveryType,
		"result_title":   state.PendingExecution.ResultTitle,
		"preview_text":   state.PendingExecution.PreviewText,
		"impact_scope":   state.PendingExecution.ImpactScope.mapValue(),
	}
}

func (state screenAnalysisApprovalState) bubbleMessageMap() map[string]any {
	return map[string]any{
		"bubble_id":  state.BubbleMessage.BubbleID,
		"task_id":    state.BubbleMessage.TaskID,
		"type":       state.BubbleMessage.Type,
		"text":       state.BubbleMessage.Text,
		"pinned":     state.BubbleMessage.Pinned,
		"hidden":     state.BubbleMessage.Hidden,
		"created_at": state.BubbleMessage.CreatedAt,
	}
}

func (scope screenAnalysisImpactScope) mapValue() map[string]any {
	return map[string]any{
		"files":                    cloneScreenAnalysisStrings(scope.Files),
		"webpages":                 cloneScreenAnalysisStrings(scope.Webpages),
		"apps":                     cloneScreenAnalysisStrings(scope.Apps),
		"out_of_workspace":         scope.OutOfWorkspace,
		"overwrite_or_delete_risk": scope.OverwriteOrDeleteRisk,
	}
}

func cloneScreenAnalysisStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}
