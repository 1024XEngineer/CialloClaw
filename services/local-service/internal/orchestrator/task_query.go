package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/runengine"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

// TaskList returns protocol-facing task items with stable paging semantics.
// It merges runtime and storage-backed views so callers do not need to know
// whether the latest state is still in memory or already persisted.
func (s *Service) TaskList(params map[string]any) (map[string]any, error) {
	group := stringValue(params, "group", "unfinished")
	// Clamp paging params at the RPC boundary so runtime and storage-backed
	// list flows expose the same contract to dashboard consumers.
	limit := clampListLimit(intValue(params, "limit", 20))
	offset := clampListOffset(intValue(params, "offset", 0))
	sortBy := stringValue(params, "sort_by", "updated_at")
	sortOrder := stringValue(params, "sort_order", "desc")
	tasks, total := s.taskListRecords(group, sortBy, sortOrder, limit, offset)

	items := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		items = append(items, taskMap(task))
	}

	return map[string]any{
		"items": items,
		"page":  pageMap(limit, offset, total),
	}, nil
}

// TaskDetailGet returns the task detail payload for `agent.task.detail.get`.
// It keeps structured storage authoritative for formal evidence while allowing
// live runtime state to fill task status fields that have not persisted yet.
func (s *Service) TaskDetailGet(request TaskDetailGetRequest) (TaskDetailGetResponse, error) {
	return s.taskDetailGet(request)
}

// TaskDetailGetFromParams adapts the RPC-decoded request map once, then keeps
// task-detail assembly typed through the orchestrator response boundary.
func (s *Service) TaskDetailGetFromParams(params map[string]any) (TaskDetailGetResponse, error) {
	return s.TaskDetailGet(TaskDetailGetRequestFromParams(params))
}

func (s *Service) taskDetailGet(request TaskDetailGetRequest) (TaskDetailGetResponse, error) {
	taskID := request.TaskID
	task, ok := s.taskDetailFromStorage(taskID)
	if runtimeTask, runtimeOK := s.runEngine.TaskDetail(taskID); runtimeOK {
		if ok {
			task = mergeRuntimeTaskDetail(task, runtimeTask)
		} else {
			task = runtimeTask
			ok = true
		}
	}
	if !ok {
		return TaskDetailGetResponse{}, ErrTaskNotFound
	}

	securitySummary := cloneMap(task.SecuritySummary)
	if securitySummary == nil {
		securitySummary = map[string]any{}
	}
	approvalRequest := s.pendingApprovalRequestFromStorage(task.TaskID, task.RiskLevel)
	if approvalRequest == nil {
		approvalRequest = activeTaskDetailApprovalRequest(task)
	}
	if task.Status != "waiting_auth" {
		approvalRequest = nil
	}
	approvalRequestValue := any(nil)
	if approvalRequest != nil {
		approvalRequestValue = approvalRequest
	}
	storageAuthorizationRecord := s.latestAttemptAuthorizationRecordFromStorage(task)
	authorizationRecord := selectTaskDetailAuthorizationRecord(task.TaskID, task.Authorization, storageAuthorizationRecord)
	authorizationRecordValue := any(nil)
	if authorizationRecord != nil {
		authorizationRecordValue = authorizationRecord
	}
	storageAuditRecords := s.loadAttemptAuditRecordsFromStorage(task, 0, 0)
	auditRecord := selectTaskDetailAuditRecord(task, task.AuditRecords, storageAuditRecords)
	auditRecordValue := any(nil)
	if auditRecord != nil {
		auditRecordValue = auditRecord
	}
	securitySummary["pending_authorizations"] = 0
	if approvalRequest != nil {
		securitySummary["pending_authorizations"] = 1
	}
	if strings.TrimSpace(stringValue(securitySummary, "security_status", "")) == "" {
		if approvalRequest != nil {
			securitySummary["security_status"] = "pending_confirmation"
		} else {
			securitySummary["security_status"] = "normal"
		}
	}
	if strings.TrimSpace(stringValue(securitySummary, "risk_level", "")) == "" {
		securitySummary["risk_level"] = firstNonEmptyString(
			stringValue(approvalRequest, "risk_level", ""),
			firstNonEmptyString(task.RiskLevel, s.risk.DefaultLevel()),
		)
	}
	latestRestorePoint := s.normalizeTaskDetailRestorePoint(task.TaskID, securitySummary)
	if latestRestorePoint == nil {
		securitySummary["latest_restore_point"] = nil
	} else {
		securitySummary["latest_restore_point"] = latestRestorePoint
	}
	runtimeSummary := s.buildTaskRuntimeSummary(task)
	deliveryResultValue := any(nil)
	deliveryResult := s.resolveFormalTaskDeliveryResult(task)
	normalizedDelivery := normalizeTaskDetailDeliveryResult(task.TaskID, deliveryResult)
	if len(normalizedDelivery) > 0 {
		deliveryResultValue = normalizedDelivery
	}

	deliveryResultDTO, err := deliveryResultDTOPointerFromValue(deliveryResultValue)
	if err != nil {
		return TaskDetailGetResponse{}, fmt.Errorf("delivery_result: %w", err)
	}
	artifacts, err := artifactDTOListFromValues(s.artifactsForTask(task, task.Artifacts))
	if err != nil {
		return TaskDetailGetResponse{}, err
	}
	citations, err := citationDTOListFromValues(s.citationsForTask(task, task.Citations))
	if err != nil {
		return TaskDetailGetResponse{}, err
	}
	mirrorReferences, err := mirrorReferenceDTOListFromValues(task.MirrorReferences)
	if err != nil {
		return TaskDetailGetResponse{}, err
	}
	approvalRequestDTO, err := approvalRequestDTOPointerFromValue(approvalRequestValue)
	if err != nil {
		return TaskDetailGetResponse{}, fmt.Errorf("approval_request: %w", err)
	}
	authorizationRecordDTO, err := authorizationRecordDTOPointerFromValue(authorizationRecordValue)
	if err != nil {
		return TaskDetailGetResponse{}, fmt.Errorf("authorization_record: %w", err)
	}
	auditRecordDTO, err := auditRecordDTOPointerFromValue(auditRecordValue)
	if err != nil {
		return TaskDetailGetResponse{}, fmt.Errorf("audit_record: %w", err)
	}
	securitySummaryDTO, err := securitySummaryDTOFromMap(securitySummary)
	if err != nil {
		return TaskDetailGetResponse{}, fmt.Errorf("security_summary: %w", err)
	}
	runtimeSummaryDTO, err := runtimeSummaryDTOFromMap(runtimeSummary)
	if err != nil {
		return TaskDetailGetResponse{}, fmt.Errorf("runtime_summary: %w", err)
	}

	return TaskDetailGetResponse{
		Task:                taskDTOFromRecord(task),
		Timeline:            taskStepDTOListFromRecords(task.Timeline),
		DeliveryResult:      deliveryResultDTO,
		Artifacts:           artifacts,
		Citations:           citations,
		MirrorReferences:    mirrorReferences,
		ApprovalRequest:     approvalRequestDTO,
		AuthorizationRecord: authorizationRecordDTO,
		AuditRecord:         auditRecordDTO,
		SecuritySummary:     securitySummaryDTO,
		RuntimeSummary:      runtimeSummaryDTO,
	}, nil
}

func taskDTOFromRecord(record runengine.TaskRecord) TaskDTO {
	return TaskDTO{
		TaskID:         record.TaskID,
		SessionID:      trimmedStringPointer(record.SessionID),
		Title:          record.Title,
		SourceType:     record.SourceType,
		Status:         record.Status,
		Intent:         intentPayloadFromTaskIntent(record.Intent),
		CurrentStep:    record.CurrentStep,
		RiskLevel:      record.RiskLevel,
		LoopStopReason: trimmedStringPointer(record.LoopStopReason),
		StartedAt:      stringPointer(record.StartedAt.Format(dateTimeLayout)),
		UpdatedAt:      record.UpdatedAt.Format(dateTimeLayout),
		FinishedAt:     timePointerString(record.FinishedAt),
	}
}

func intentPayloadFromTaskIntent(intent map[string]any) *IntentPayload {
	if len(intent) == 0 {
		return nil
	}
	name := stringValue(intent, "name", "")
	arguments := cloneMap(mapValue(intent, "arguments"))
	if strings.TrimSpace(name) == "" && len(arguments) == 0 {
		return nil
	}
	return &IntentPayload{Name: name, Arguments: arguments}
}

func taskStepDTOListFromRecords(timeline []runengine.TaskStepRecord) []TaskStepDTO {
	if len(timeline) == 0 {
		return []TaskStepDTO{}
	}
	result := make([]TaskStepDTO, 0, len(timeline))
	for _, step := range timeline {
		result = append(result, TaskStepDTO{
			StepID:        step.StepID,
			TaskID:        step.TaskID,
			Name:          step.Name,
			Status:        step.Status,
			OrderIndex:    step.OrderIndex,
			InputSummary:  step.InputSummary,
			OutputSummary: step.OutputSummary,
		})
	}
	return result
}

func deliveryResultDTOPointerFromValue(value any) (*DeliveryResultDTO, error) {
	if value == nil {
		return nil, nil
	}
	deliveryResult, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("must be object, got %T", value)
	}
	if len(deliveryResult) == 0 {
		return nil, nil
	}
	dto, err := deliveryResultDTOFromMap(deliveryResult)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

func artifactDTOListFromValues(artifacts []map[string]any) ([]ArtifactDTO, error) {
	normalized := protocolArtifactList(artifacts)
	result := make([]ArtifactDTO, 0, len(normalized))
	for index, artifact := range normalized {
		dto, err := artifactDTOFromMap(artifact)
		if err != nil {
			return nil, fmt.Errorf("artifacts[%d]: %w", index, err)
		}
		result = append(result, dto)
	}
	return result, nil
}

func citationDTOListFromValues(citations []map[string]any) ([]CitationDTO, error) {
	normalized := protocolCitationList(citations)
	result := make([]CitationDTO, 0, len(normalized))
	for index, citation := range normalized {
		dto, err := citationDTOFromMap(citation)
		if err != nil {
			return nil, fmt.Errorf("citations[%d]: %w", index, err)
		}
		result = append(result, dto)
	}
	return result, nil
}

func mirrorReferenceDTOListFromValues(references []map[string]any) ([]MirrorReferenceDTO, error) {
	normalized := protocolMirrorReferenceList(references)
	result := make([]MirrorReferenceDTO, 0, len(normalized))
	for index, reference := range normalized {
		dto, err := mirrorReferenceDTOFromMap(reference)
		if err != nil {
			return nil, fmt.Errorf("mirror_references[%d]: %w", index, err)
		}
		result = append(result, dto)
	}
	return result, nil
}

func approvalRequestDTOPointerFromValue(value any) (*ApprovalRequestDTO, error) {
	if value == nil {
		return nil, nil
	}
	approvalRequest, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("must be object, got %T", value)
	}
	if len(approvalRequest) == 0 {
		return nil, nil
	}
	dto, err := approvalRequestDTOFromMap(approvalRequest)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

func authorizationRecordDTOPointerFromValue(value any) (*AuthorizationRecordDTO, error) {
	if value == nil {
		return nil, nil
	}
	authorizationRecord, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("must be object, got %T", value)
	}
	if len(authorizationRecord) == 0 {
		return nil, nil
	}
	dto, err := authorizationRecordDTOFromMap(authorizationRecord)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

func auditRecordDTOPointerFromValue(value any) (*AuditRecordDTO, error) {
	if value == nil {
		return nil, nil
	}
	auditRecord, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("must be object, got %T", value)
	}
	if len(auditRecord) == 0 {
		return nil, nil
	}
	dto, err := auditRecordDTOFromMap(auditRecord)
	if err != nil {
		return nil, err
	}
	return &dto, nil
}

func trimmedStringPointer(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func stringPointer(value string) *string {
	return &value
}

func timePointerString(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(dateTimeLayout)
	return &formatted
}

// mergeRuntimeTaskDetail keeps first-class structured evidence authoritative but
// lets the live runtime state win for task status fields when persistence is
// temporarily stale.
func mergeRuntimeTaskDetail(structuredTask, runtimeTask runengine.TaskRecord) runengine.TaskRecord {
	merged := mergeStructuredTaskDetailCompatibility(structuredTask, runtimeTask)
	if taskUsesAttemptScopedFormalReads(runtimeTask) {
		merged.DeliveryResult = cloneMap(runtimeTask.DeliveryResult)
		merged.Artifacts = cloneMapSlice(runtimeTask.Artifacts)
		merged.Citations = cloneMapSlice(runtimeTask.Citations)
		merged.ApprovalRequest = cloneMap(runtimeTask.ApprovalRequest)
		merged.Authorization = cloneMap(runtimeTask.Authorization)
		merged.ImpactScope = cloneMap(runtimeTask.ImpactScope)
		merged.PendingExecution = cloneMap(runtimeTask.PendingExecution)
		merged.AuditRecords = cloneMapSlice(runtimeTask.AuditRecords)
		merged.LatestToolCall = cloneMap(runtimeTask.LatestToolCall)
		merged.LoopStopReason = runtimeTask.LoopStopReason
	}
	if runtimeTask.RunID != "" {
		merged.RunID = runtimeTask.RunID
	}
	if runtimeTask.PrimaryRunID != "" {
		merged.PrimaryRunID = runtimeTask.PrimaryRunID
	}
	if runtimeTask.ExecutionAttempt > 0 {
		merged.ExecutionAttempt = runtimeTask.ExecutionAttempt
	}
	if runtimeTask.Status != "" {
		merged.Status = runtimeTask.Status
	}
	if runtimeTask.CurrentStep != "" {
		merged.CurrentStep = runtimeTask.CurrentStep
	}
	if runtimeTask.CurrentStepStatus != "" {
		merged.CurrentStepStatus = runtimeTask.CurrentStepStatus
	}
	if runtimeTask.UpdatedAt.After(merged.UpdatedAt) {
		merged.UpdatedAt = runtimeTask.UpdatedAt
	}
	if runtimeTask.FinishedAt != nil {
		if merged.FinishedAt == nil || runtimeTask.FinishedAt.After(*merged.FinishedAt) {
			merged.FinishedAt = cloneTimePointer(runtimeTask.FinishedAt)
		}
	}
	if runtimeTask.LoopStopReason != "" {
		merged.LoopStopReason = runtimeTask.LoopStopReason
	}
	if len(runtimeTask.BubbleMessage) > 0 {
		merged.BubbleMessage = cloneMap(runtimeTask.BubbleMessage)
	}
	if len(runtimeTask.PendingExecution) > 0 {
		merged.PendingExecution = cloneMap(runtimeTask.PendingExecution)
	}
	if len(runtimeTask.TokenUsage) > 0 {
		merged.TokenUsage = cloneMap(runtimeTask.TokenUsage)
	}
	if len(runtimeTask.LatestEvent) > 0 {
		merged.LatestEvent = cloneMap(runtimeTask.LatestEvent)
	}
	if len(runtimeTask.LatestToolCall) > 0 {
		merged.LatestToolCall = cloneMap(runtimeTask.LatestToolCall)
	}
	if len(runtimeTask.SteeringMessages) > 0 {
		merged.SteeringMessages = append([]string(nil), runtimeTask.SteeringMessages...)
	}
	if !isEmptySnapshot(runtimeTask.Snapshot) {
		merged.Snapshot = cloneTaskSnapshot(runtimeTask.Snapshot)
	}
	return merged
}

func (s *Service) buildTaskRuntimeSummary(task runengine.TaskRecord) map[string]any {
	summary := map[string]any{
		"loop_stop_reason":        nil,
		"events_count":            0,
		"latest_event_type":       nil,
		"active_steering_count":   len(task.SteeringMessages),
		"latest_failure_code":     nil,
		"latest_failure_category": nil,
		"latest_failure_summary":  nil,
		"observation_signals":     []string{},
	}
	if strings.TrimSpace(task.LoopStopReason) != "" {
		summary["loop_stop_reason"] = task.LoopStopReason
	}
	if failureCode, failureCategory, failureSummary := latestTaskFailure(task); failureCode != "" || failureSummary != "" {
		if failureCode != "" {
			summary["latest_failure_code"] = failureCode
		}
		if failureCategory != "" {
			summary["latest_failure_category"] = failureCategory
		}
		if failureSummary != "" {
			summary["latest_failure_summary"] = failureSummary
		}
	}
	if observationSignals := taskObservationSignals(task); len(observationSignals) > 0 {
		summary["observation_signals"] = observationSignals
	}
	if s.storage == nil || s.storage.LoopRuntimeStore() == nil {
		return summary
	}
	runIDFilter := ""
	if taskUsesAttemptScopedFormalReads(task) {
		runIDFilter = task.RunID
	}
	// Keep latest_event_type scoped to normalized runtime events so task-level
	// notifications such as task.updated or task.steered do not leak into the
	// runtime summary contract when no runtime events have been persisted yet.
	records, total, err := s.storage.LoopRuntimeStore().ListEvents(context.Background(), task.TaskID, runIDFilter, "", "", "", 1, 0)
	if err == nil {
		summary["events_count"] = total
		if len(records) > 0 && strings.TrimSpace(records[0].Type) != "" {
			summary["latest_event_type"] = records[0].Type
		}
	}
	return summary
}

func latestTaskFailure(task runengine.TaskRecord) (string, string, string) {
	var fallbackCode string
	var fallbackCategory string
	var fallbackSummary string
	for index := len(task.AuditRecords) - 1; index >= 0; index-- {
		record := task.AuditRecords[index]
		if stringValue(record, "result", "") != "failed" {
			continue
		}
		metadata := mapValue(record, "metadata")
		failureCode := strings.TrimSpace(stringValue(metadata, "failure_code", ""))
		failureCategory := strings.TrimSpace(stringValue(metadata, "failure_category", ""))
		failureSummary := firstNonEmptyString(stringValue(record, "summary", ""), stringValue(record, "reason", ""))
		if failureCode != "" || failureCategory != "" {
			return firstNonEmptyString(failureCode, stringValue(record, "action", "")), firstNonEmptyString(failureCategory, firstNonEmptyString(stringValue(record, "type", ""), stringValue(record, "category", ""))), failureSummary
		}
		if fallbackCode == "" && fallbackCategory == "" && fallbackSummary == "" {
			fallbackCode = firstNonEmptyString(stringValue(record, "action", ""), firstNonEmptyString(stringValue(record, "type", ""), stringValue(record, "category", "")))
			fallbackCategory = firstNonEmptyString(stringValue(record, "type", ""), stringValue(record, "category", ""))
			fallbackSummary = failureSummary
		}
	}
	if fallbackCode != "" || fallbackCategory != "" || fallbackSummary != "" {
		return fallbackCode, fallbackCategory, fallbackSummary
	}
	if task.Status == "failed" {
		return firstNonEmptyString(task.CurrentStep, "execution_failed"), "task_execution", firstNonEmptyString(stringValue(task.BubbleMessage, "text", ""), "任务执行失败")
	}
	return "", "", ""
}

func taskObservationSignals(task runengine.TaskRecord) []string {
	result := make([]string, 0, 4)
	observationSources := []struct {
		signal string
		value  string
	}{
		{signal: "screen_summary", value: task.Snapshot.ScreenSummary},
		{signal: "visible_text", value: task.Snapshot.VisibleText},
		{signal: "page_title", value: task.Snapshot.PageTitle},
		{signal: "window_title", value: task.Snapshot.WindowTitle},
	}
	for _, item := range observationSources {
		if strings.TrimSpace(item.value) == "" {
			continue
		}
		result = append(result, item.signal)
	}
	return uniqueTrimmedStrings(result)
}

// TaskEventsList handles agent.task.events.list and exposes normalized runtime
// events without leaking storage-specific row shapes across the RPC boundary.
func (s *Service) TaskEventsList(params map[string]any) (map[string]any, error) {
	limit := clampListLimit(intValue(params, "limit", 20))
	offset := clampListOffset(intValue(params, "offset", 0))
	taskID := stringValue(params, "task_id", "")
	runID := stringValue(params, "run_id", "")
	eventType := stringValue(params, "type", "")
	createdAtFrom, err := normalizeEventTimeFilter(stringValue(params, "created_at_from", ""))
	if err != nil {
		return nil, fmt.Errorf("created_at_from must be RFC3339: %w", err)
	}
	createdAtTo, err := normalizeEventTimeFilter(stringValue(params, "created_at_to", ""))
	if err != nil {
		return nil, fmt.Errorf("created_at_to must be RFC3339: %w", err)
	}
	if strings.TrimSpace(taskID) == "" {
		return nil, errors.New("task_id is required")
	}
	if createdAtFrom != "" && createdAtTo != "" && parseEventTimeFilter(createdAtFrom).After(parseEventTimeFilter(createdAtTo)) {
		return nil, errors.New("created_at_from must be earlier than or equal to created_at_to")
	}
	if s.storage == nil || s.storage.LoopRuntimeStore() == nil {
		return map[string]any{"items": []map[string]any{}, "page": pageMap(limit, offset, 0)}, nil
	}
	records, total, err := s.storage.LoopRuntimeStore().ListEvents(context.Background(), taskID, runID, eventType, createdAtFrom, createdAtTo, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
	}
	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		items = append(items, map[string]any{
			"event_id":     record.EventID,
			"run_id":       record.RunID,
			"task_id":      record.TaskID,
			"step_id":      record.StepID,
			"type":         record.Type,
			"level":        record.Level,
			"payload_json": record.PayloadJSON,
			"created_at":   record.CreatedAt,
		})
	}
	return map[string]any{
		"items": items,
		"page":  pageMap(limit, offset, total),
	}, nil
}

// TaskToolCallsList handles agent.task.tool_calls.list and exposes persisted
// tool_call records through one task-centric query surface.
func (s *Service) TaskToolCallsList(params map[string]any) (map[string]any, error) {
	limit := clampListLimit(intValue(params, "limit", 20))
	offset := clampListOffset(intValue(params, "offset", 0))
	taskID := stringValue(params, "task_id", "")
	runID := stringValue(params, "run_id", "")
	if strings.TrimSpace(taskID) == "" {
		return nil, errors.New("task_id is required")
	}
	if s.storage == nil || s.storage.ToolCallStore() == nil {
		compatibilityItems := compatibilityTaskToolCalls(s, taskID, runID)
		return map[string]any{
			"items": paginateTaskToolCallItems(compatibilityItems, limit, offset),
			"page":  pageMap(limit, offset, len(compatibilityItems)),
		}, nil
	}
	items, total, err := s.storage.ToolCallStore().ListToolCalls(context.Background(), taskID, runID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrStorageQueryFailed, err)
	}
	if total == 0 {
		compatibilityItems := compatibilityTaskToolCalls(s, taskID, runID)
		return map[string]any{
			"items": paginateTaskToolCallItems(compatibilityItems, limit, offset),
			"page":  pageMap(limit, offset, len(compatibilityItems)),
		}, nil
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, taskToolCallMap(item))
	}
	return map[string]any{
		"items": result,
		"page":  pageMap(limit, offset, total),
	}, nil
}

func compatibilityTaskToolCalls(s *Service, taskID, runID string) []map[string]any {
	if s == nil {
		return nil
	}
	task, ok := s.taskDetailFromStorage(taskID)
	if runtimeTask, runtimeOK := s.runEngine.TaskDetail(taskID); runtimeOK {
		if ok {
			task = mergeRuntimeTaskDetail(task, runtimeTask)
		} else {
			task = runtimeTask
			ok = true
		}
	}
	if !ok || len(task.LatestToolCall) == 0 {
		return nil
	}
	if strings.TrimSpace(runID) != "" && stringValue(task.LatestToolCall, "run_id", "") != runID {
		return nil
	}
	return []map[string]any{normalizeTaskToolCallMap(task.LatestToolCall)}
}

func paginateTaskToolCallItems(items []map[string]any, limit, offset int) []map[string]any {
	if len(items) == 0 || offset >= len(items) {
		return []map[string]any{}
	}
	end := len(items)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return cloneMapSlice(items[offset:end])
}

func normalizeTaskToolCallMap(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	stepID := any(nil)
	if candidate := stringValue(value, "step_id", ""); strings.TrimSpace(candidate) != "" {
		stepID = candidate
	}
	createdAt := any(nil)
	if candidate := stringValue(value, "created_at", ""); strings.TrimSpace(candidate) != "" {
		createdAt = candidate
	}
	errorCode := value["error_code"]
	return map[string]any{
		"tool_call_id": stringValue(value, "tool_call_id", ""),
		"run_id":       stringValue(value, "run_id", ""),
		"task_id":      stringValue(value, "task_id", ""),
		"step_id":      stepID,
		"created_at":   createdAt,
		"tool_name":    stringValue(value, "tool_name", ""),
		"status":       outwardToolCallStatus(stringValue(value, "status", "pending")),
		"input":        cloneMapOrEmpty(mapValue(value, "input")),
		"output":       cloneMapOrEmpty(mapValue(value, "output")),
		"error_code":   errorCode,
		"duration_ms":  intValue(value, "duration_ms", 0),
	}
}

func taskToolCallMap(record tools.ToolCallRecord) map[string]any {
	stepID := any(nil)
	if strings.TrimSpace(record.StepID) != "" {
		stepID = record.StepID
	}
	createdAt := any(nil)
	if strings.TrimSpace(record.CreatedAt) != "" {
		createdAt = record.CreatedAt
	}
	errorCode := any(nil)
	if record.ErrorCode != nil {
		errorCode = *record.ErrorCode
	}
	return map[string]any{
		"tool_call_id": record.ToolCallID,
		"run_id":       record.RunID,
		"task_id":      record.TaskID,
		"step_id":      stepID,
		"created_at":   createdAt,
		"tool_name":    record.ToolName,
		"status":       outwardToolCallStatus(string(record.Status)),
		"input":        cloneMapOrEmpty(record.Input),
		"output":       cloneMapOrEmpty(record.Output),
		"error_code":   errorCode,
		"duration_ms":  record.DurationMS,
	}
}

func cloneMapOrEmpty(values map[string]any) map[string]any {
	if cloned := cloneMap(values); cloned != nil {
		return cloned
	}
	return map[string]any{}
}

func outwardToolCallStatus(status string) string {
	switch strings.TrimSpace(status) {
	case "started":
		return "running"
	case "succeeded":
		return "succeeded"
	case "failed", "timeout":
		return "failed"
	default:
		return "pending"
	}
}

func normalizeEventTimeFilter(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", nil
	}
	parsed := parseEventTimeFilter(trimmed)
	if parsed.IsZero() {
		return "", fmt.Errorf("invalid time %q", trimmed)
	}
	// Preserve the caller's timestamp precision after normalizing to UTC so the
	// SQLite runtime-event store can compare the actual instant rather than a
	// second-truncated string representation.
	return parsed.UTC().Format(time.RFC3339Nano), nil
}

func parseEventTimeFilter(value string) time.Time {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed
	}
	return time.Time{}
}
