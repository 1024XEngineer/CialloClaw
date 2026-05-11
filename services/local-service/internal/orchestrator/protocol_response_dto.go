package orchestrator

import (
	"fmt"
	"reflect"
	"strings"
)

// IntentPayload keeps intent arguments dynamic while making the outer protocol
// object explicit.
type IntentPayload struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// TaskDTO is the protocol-facing task object returned through stable RPCs.
type TaskDTO struct {
	TaskID         string         `json:"task_id"`
	SessionID      *string        `json:"session_id"`
	Title          string         `json:"title"`
	SourceType     string         `json:"source_type"`
	Status         string         `json:"status"`
	Intent         *IntentPayload `json:"intent"`
	CurrentStep    string         `json:"current_step"`
	RiskLevel      string         `json:"risk_level"`
	LoopStopReason *string        `json:"loop_stop_reason,omitempty"`
	StartedAt      *string        `json:"started_at"`
	UpdatedAt      string         `json:"updated_at"`
	FinishedAt     *string        `json:"finished_at"`
}

// BubbleMessageDTO is the stable bubble_message response object.
type BubbleMessageDTO struct {
	BubbleID  string `json:"bubble_id"`
	TaskID    string `json:"task_id"`
	Type      string `json:"type"`
	Text      string `json:"text"`
	Pinned    bool   `json:"pinned"`
	Hidden    bool   `json:"hidden"`
	CreatedAt string `json:"created_at"`
}

// DeliveryPayloadDTO is the stable payload nested under delivery_result.
type DeliveryPayloadDTO struct {
	Path   *string `json:"path"`
	URL    *string `json:"url"`
	TaskID *string `json:"task_id"`
}

// DeliveryResultDTO is the formal delivery_result object.
type DeliveryResultDTO struct {
	Type        string             `json:"type"`
	Title       string             `json:"title"`
	Payload     DeliveryPayloadDTO `json:"payload"`
	PreviewText string             `json:"preview_text"`
}

// TaskStepDTO is the protocol-facing task_step timeline item.
type TaskStepDTO struct {
	StepID        string `json:"step_id"`
	TaskID        string `json:"task_id"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	OrderIndex    int    `json:"order_index"`
	InputSummary  string `json:"input_summary"`
	OutputSummary string `json:"output_summary"`
}

// ArtifactDTO is the formal artifact response object.
type ArtifactDTO struct {
	ArtifactID   string `json:"artifact_id"`
	TaskID       string `json:"task_id"`
	ArtifactType string `json:"artifact_type"`
	Title        string `json:"title"`
	Path         string `json:"path"`
	MimeType     string `json:"mime_type"`
}

// CitationDTO is the formal citation response object.
type CitationDTO struct {
	CitationID      string `json:"citation_id"`
	TaskID          string `json:"task_id"`
	RunID           string `json:"run_id"`
	SourceType      string `json:"source_type"`
	SourceRef       string `json:"source_ref"`
	Label           string `json:"label"`
	ArtifactID      string `json:"artifact_id,omitempty"`
	ArtifactType    string `json:"artifact_type,omitempty"`
	EvidenceRole    string `json:"evidence_role,omitempty"`
	ExcerptText     string `json:"excerpt_text,omitempty"`
	ScreenSessionID string `json:"screen_session_id,omitempty"`
}

// MirrorReferenceDTO is the mirror reference shape surfaced in task detail.
type MirrorReferenceDTO struct {
	MemoryID string `json:"memory_id"`
	Reason   string `json:"reason"`
	Summary  string `json:"summary"`
}

// ApprovalRequestDTO is the formal approval_request object.
type ApprovalRequestDTO struct {
	ApprovalID    string `json:"approval_id"`
	TaskID        string `json:"task_id"`
	OperationName string `json:"operation_name"`
	RiskLevel     string `json:"risk_level"`
	TargetObject  string `json:"target_object"`
	Reason        string `json:"reason"`
	Status        string `json:"status"`
	CreatedAt     string `json:"created_at"`
}

// AuthorizationRecordDTO is the formal authorization_record object.
type AuthorizationRecordDTO struct {
	AuthorizationRecordID string `json:"authorization_record_id"`
	TaskID                string `json:"task_id"`
	ApprovalID            string `json:"approval_id"`
	Decision              string `json:"decision"`
	RememberRule          bool   `json:"remember_rule"`
	Operator              string `json:"operator"`
	CreatedAt             string `json:"created_at"`
}

// AuditRecordDTO is the task-detail view of one formal audit record.
type AuditRecordDTO struct {
	AuditID   string `json:"audit_id"`
	TaskID    string `json:"task_id"`
	Type      string `json:"type"`
	Action    string `json:"action"`
	Summary   string `json:"summary"`
	Target    string `json:"target"`
	Result    string `json:"result"`
	CreatedAt string `json:"created_at"`
}

// RecoveryPointDTO is the task-detail security restore point object.
type RecoveryPointDTO struct {
	RecoveryPointID string   `json:"recovery_point_id"`
	TaskID          string   `json:"task_id"`
	Summary         string   `json:"summary"`
	CreatedAt       string   `json:"created_at"`
	Objects         []string `json:"objects"`
}

// SecuritySummaryDTO is the task-detail security summary projection.
type SecuritySummaryDTO struct {
	SecurityStatus        string            `json:"security_status,omitempty"`
	RiskLevel             string            `json:"risk_level,omitempty"`
	PendingAuthorizations int               `json:"pending_authorizations"`
	LatestRestorePoint    *RecoveryPointDTO `json:"latest_restore_point"`
}

// TaskRuntimeSummaryDTO is the stable runtime summary returned from task detail.
type TaskRuntimeSummaryDTO struct {
	LoopStopReason        *string  `json:"loop_stop_reason"`
	EventsCount           int      `json:"events_count"`
	LatestEventType       *string  `json:"latest_event_type"`
	ActiveSteeringCount   int      `json:"active_steering_count"`
	LatestFailureCode     *string  `json:"latest_failure_code"`
	LatestFailureCategory *string  `json:"latest_failure_category"`
	LatestFailureSummary  *string  `json:"latest_failure_summary"`
	ObservationSignals    []string `json:"observation_signals"`
}

// TaskEntryResponse is shared by agent.input.submit and agent.task.start.
type TaskEntryResponse struct {
	Task           *TaskDTO           `json:"task"`
	BubbleMessage  *BubbleMessageDTO  `json:"bubble_message"`
	DeliveryResult *DeliveryResultDTO `json:"delivery_result"`
}

// TaskDetailGetResponse is the typed result for agent.task.detail.get.
type TaskDetailGetResponse struct {
	Task                TaskDTO                 `json:"task"`
	Timeline            []TaskStepDTO           `json:"timeline"`
	DeliveryResult      *DeliveryResultDTO      `json:"delivery_result"`
	Artifacts           []ArtifactDTO           `json:"artifacts"`
	Citations           []CitationDTO           `json:"citations"`
	MirrorReferences    []MirrorReferenceDTO    `json:"mirror_references"`
	ApprovalRequest     *ApprovalRequestDTO     `json:"approval_request"`
	AuthorizationRecord *AuthorizationRecordDTO `json:"authorization_record"`
	AuditRecord         *AuditRecordDTO         `json:"audit_record"`
	SecuritySummary     SecuritySummaryDTO      `json:"security_summary"`
	RuntimeSummary      TaskRuntimeSummaryDTO   `json:"runtime_summary"`
}

type startTaskProtocolParams struct {
	RequestMeta RequestMeta         `json:"request_meta"`
	SessionID   string              `json:"session_id,omitempty"`
	Source      string              `json:"source,omitempty"`
	Trigger     string              `json:"trigger,omitempty"`
	Input       TaskStartInput      `json:"input"`
	Context     *InputContext       `json:"context,omitempty"`
	Delivery    *DeliveryPreference `json:"delivery,omitempty"`
	Options     *TaskStartOptions   `json:"options,omitempty"`
	Intent      map[string]any      `json:"intent,omitempty"`
}

type submitInputProtocolParams struct {
	RequestMeta RequestMeta         `json:"request_meta"`
	SessionID   string              `json:"session_id,omitempty"`
	Source      string              `json:"source,omitempty"`
	Trigger     string              `json:"trigger,omitempty"`
	Input       InputSubmitInput    `json:"input"`
	Context     *InputContext       `json:"context,omitempty"`
	VoiceMeta   *VoiceMeta          `json:"voice_meta,omitempty"`
	Options     *InputSubmitOptions `json:"options,omitempty"`
}

type taskDetailGetProtocolParams struct {
	RequestMeta RequestMeta `json:"request_meta"`
	TaskID      string      `json:"task_id"`
}

// StartTaskRequestFromParams adapts RPC-decoded params to the typed
// orchestrator request. The adapter stays manual so hot RPC entrypoints avoid
// extra JSON round-trips after the boundary has already validated the payload.
func StartTaskRequestFromParams(params map[string]any) StartTaskRequest {
	request := StartTaskRequest{
		RequestMeta: requestMetaFromMap(mapValue(params, "request_meta")),
		SessionID:   stringValue(params, "session_id", ""),
		Source:      stringValue(params, "source", ""),
		Trigger:     stringValue(params, "trigger", ""),
		Input:       taskStartInputFromMap(mapValue(params, "input")),
		Context:     inputContextPointerFromMap(mapValue(params, "context")),
		Delivery:    deliveryPreferencePointerFromMap(mapValue(params, "delivery")),
		Options:     taskStartOptionsPointerFromMap(mapValue(params, "options")),
	}
	if intent := mapValue(params, "intent"); len(intent) > 0 {
		request.Intent = cloneMap(intent)
	}
	return request
}

// SubmitInputRequestFromParams adapts RPC-decoded params to the typed
// orchestrator request. The adapter stays manual so hot RPC entrypoints avoid
// extra JSON round-trips after the boundary has already validated the payload.
func SubmitInputRequestFromParams(params map[string]any) SubmitInputRequest {
	return SubmitInputRequest{
		RequestMeta: requestMetaFromMap(mapValue(params, "request_meta")),
		SessionID:   stringValue(params, "session_id", ""),
		Source:      stringValue(params, "source", ""),
		Trigger:     stringValue(params, "trigger", ""),
		Input:       inputSubmitInputFromMap(mapValue(params, "input")),
		Context:     inputContextPointerFromMap(mapValue(params, "context")),
		VoiceMeta:   voiceMetaPointerFromMap(mapValue(params, "voice_meta")),
		Options:     inputSubmitOptionsPointerFromMap(mapValue(params, "options")),
	}
}

// TaskDetailGetRequestFromParams adapts RPC-decoded params to the typed
// orchestrator request. The adapter stays manual so hot RPC entrypoints avoid
// extra JSON round-trips after the boundary has already validated the payload.
func TaskDetailGetRequestFromParams(params map[string]any) TaskDetailGetRequest {
	return TaskDetailGetRequest{
		RequestMeta: requestMetaFromMap(mapValue(params, "request_meta")),
		TaskID:      stringValue(params, "task_id", ""),
	}
}

// ProtocolParamsMap exports the normalized protocol payload for RPC adapters
// that have already validated the transport envelope.
func (r StartTaskRequest) ProtocolParamsMap() map[string]any {
	return protocolMapFromDTO(startTaskProtocolParams{
		RequestMeta: r.RequestMeta,
		SessionID:   strings.TrimSpace(r.SessionID),
		Source:      strings.TrimSpace(r.Source),
		Trigger:     strings.TrimSpace(r.Trigger),
		Input:       r.Input,
		Context:     r.Context,
		Delivery:    r.Delivery,
		Options:     r.Options,
		Intent:      cloneMap(r.Intent),
	})
}

// ProtocolParamsMap exports the normalized protocol payload for RPC adapters
// that have already validated the transport envelope.
func (r SubmitInputRequest) ProtocolParamsMap() map[string]any {
	return protocolMapFromDTO(submitInputProtocolParams{
		RequestMeta: r.RequestMeta,
		SessionID:   strings.TrimSpace(r.SessionID),
		Source:      strings.TrimSpace(r.Source),
		Trigger:     strings.TrimSpace(r.Trigger),
		Input:       r.Input,
		Context:     r.Context,
		VoiceMeta:   r.VoiceMeta,
		Options:     r.Options,
	})
}

// ProtocolParamsMap exports the normalized protocol payload for RPC adapters
// that have already validated the transport envelope.
func (r TaskDetailGetRequest) ProtocolParamsMap() map[string]any {
	return protocolMapFromDTO(taskDetailGetProtocolParams{
		RequestMeta: r.RequestMeta,
		TaskID:      strings.TrimSpace(r.TaskID),
	})
}

func newTaskEntryResponse(payload map[string]any) (TaskEntryResponse, error) {
	task, err := taskDTOPointerFromMap(payload, "task")
	if err != nil {
		return TaskEntryResponse{}, err
	}
	bubbleMessage, err := bubbleMessageDTOPointerFromMap(payload, "bubble_message")
	if err != nil {
		return TaskEntryResponse{}, err
	}
	deliveryResult, err := deliveryResultDTOPointerFromMap(payload, "delivery_result")
	if err != nil {
		return TaskEntryResponse{}, err
	}
	return TaskEntryResponse{
		Task:           task,
		BubbleMessage:  bubbleMessage,
		DeliveryResult: deliveryResult,
	}, nil
}

// Map returns the protocol payload as a map for package tests that assert
// individual fields. Production callers should consume the typed DTO directly.
func (r TaskEntryResponse) Map() map[string]any {
	return protocolMapFromDTO(r)
}

// Map returns the protocol payload as a map for package tests that assert
// individual fields. Production callers should consume the typed DTO directly.
func (r TaskDetailGetResponse) Map() map[string]any {
	return protocolMapFromDTO(r)
}

func taskDTOPointerFromMap(values map[string]any, key string) (*TaskDTO, error) {
	payload, ok, err := protocolMapField(values, key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	task, err := taskDTOFromMap(payload)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	return &task, nil
}

func taskDTOFromMap(values map[string]any) (TaskDTO, error) {
	taskID, err := requireProtocolStringField(values, "task_id")
	if err != nil {
		return TaskDTO{}, err
	}
	sessionID, err := protocolStringPointerField(values, "session_id")
	if err != nil {
		return TaskDTO{}, err
	}
	title, err := requireProtocolStringField(values, "title")
	if err != nil {
		return TaskDTO{}, err
	}
	sourceType, err := requireProtocolStringField(values, "source_type")
	if err != nil {
		return TaskDTO{}, err
	}
	status, err := requireProtocolStringField(values, "status")
	if err != nil {
		return TaskDTO{}, err
	}
	intent, err := intentPayloadPointerFromMap(values, "intent")
	if err != nil {
		return TaskDTO{}, err
	}
	currentStep, err := requireProtocolStringField(values, "current_step")
	if err != nil {
		return TaskDTO{}, err
	}
	riskLevel, err := requireProtocolStringField(values, "risk_level")
	if err != nil {
		return TaskDTO{}, err
	}
	loopStopReason, err := protocolStringPointerField(values, "loop_stop_reason")
	if err != nil {
		return TaskDTO{}, err
	}
	startedAt, err := protocolStringPointerField(values, "started_at")
	if err != nil {
		return TaskDTO{}, err
	}
	updatedAt, err := requireProtocolStringField(values, "updated_at")
	if err != nil {
		return TaskDTO{}, err
	}
	finishedAt, err := protocolStringPointerField(values, "finished_at")
	if err != nil {
		return TaskDTO{}, err
	}
	return TaskDTO{
		TaskID:         taskID,
		SessionID:      sessionID,
		Title:          title,
		SourceType:     sourceType,
		Status:         status,
		Intent:         intent,
		CurrentStep:    currentStep,
		RiskLevel:      riskLevel,
		LoopStopReason: loopStopReason,
		StartedAt:      startedAt,
		UpdatedAt:      updatedAt,
		FinishedAt:     finishedAt,
	}, nil
}

func intentPayloadPointerFromMap(values map[string]any, key string) (*IntentPayload, error) {
	payload, ok, err := protocolMapField(values, key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	name, err := protocolStringField(payload, "name")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	arguments, ok, err := protocolMapField(payload, "arguments")
	if err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	if !ok {
		arguments = map[string]any{}
	} else {
		arguments = cloneProtocolMap(arguments)
	}
	if strings.TrimSpace(name) == "" && len(arguments) == 0 {
		return nil, nil
	}
	return &IntentPayload{Name: name, Arguments: arguments}, nil
}

func bubbleMessageDTOPointerFromMap(values map[string]any, key string) (*BubbleMessageDTO, error) {
	payload, ok, err := protocolMapField(values, key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	bubbleMessage, err := bubbleMessageDTOFromMap(payload)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	return &bubbleMessage, nil
}

func bubbleMessageDTOFromMap(values map[string]any) (BubbleMessageDTO, error) {
	bubbleID, err := requireProtocolStringField(values, "bubble_id")
	if err != nil {
		return BubbleMessageDTO{}, err
	}
	taskID, err := requireProtocolStringField(values, "task_id")
	if err != nil {
		return BubbleMessageDTO{}, err
	}
	messageType, err := requireProtocolStringField(values, "type")
	if err != nil {
		return BubbleMessageDTO{}, err
	}
	text, err := requireProtocolStringField(values, "text")
	if err != nil {
		return BubbleMessageDTO{}, err
	}
	pinned, err := requireProtocolBoolField(values, "pinned")
	if err != nil {
		return BubbleMessageDTO{}, err
	}
	hidden, err := requireProtocolBoolField(values, "hidden")
	if err != nil {
		return BubbleMessageDTO{}, err
	}
	createdAt, err := requireProtocolStringField(values, "created_at")
	if err != nil {
		return BubbleMessageDTO{}, err
	}
	return BubbleMessageDTO{
		BubbleID:  bubbleID,
		TaskID:    taskID,
		Type:      messageType,
		Text:      text,
		Pinned:    pinned,
		Hidden:    hidden,
		CreatedAt: createdAt,
	}, nil
}

func deliveryResultDTOPointerFromMap(values map[string]any, key string) (*DeliveryResultDTO, error) {
	payload, ok, err := protocolMapField(values, key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	deliveryResult, err := deliveryResultDTOFromMap(payload)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	return &deliveryResult, nil
}

func deliveryResultDTOFromMap(values map[string]any) (DeliveryResultDTO, error) {
	resultType, err := requireProtocolStringField(values, "type")
	if err != nil {
		return DeliveryResultDTO{}, err
	}
	title, err := requireProtocolStringField(values, "title")
	if err != nil {
		return DeliveryResultDTO{}, err
	}
	payloadMap, err := requireProtocolMapField(values, "payload")
	if err != nil {
		return DeliveryResultDTO{}, err
	}
	payload, err := deliveryPayloadDTOFromMap(payloadMap)
	if err != nil {
		return DeliveryResultDTO{}, fmt.Errorf("payload: %w", err)
	}
	previewText, err := requireProtocolStringField(values, "preview_text")
	if err != nil {
		return DeliveryResultDTO{}, err
	}
	return DeliveryResultDTO{
		Type:        resultType,
		Title:       title,
		Payload:     payload,
		PreviewText: previewText,
	}, nil
}

func deliveryPayloadDTOFromMap(values map[string]any) (DeliveryPayloadDTO, error) {
	path, err := protocolStringPointerField(values, "path")
	if err != nil {
		return DeliveryPayloadDTO{}, err
	}
	url, err := protocolStringPointerField(values, "url")
	if err != nil {
		return DeliveryPayloadDTO{}, err
	}
	taskID, err := protocolStringPointerField(values, "task_id")
	if err != nil {
		return DeliveryPayloadDTO{}, err
	}
	return DeliveryPayloadDTO{Path: path, URL: url, TaskID: taskID}, nil
}

func artifactDTOFromMap(values map[string]any) (ArtifactDTO, error) {
	artifactID, err := requireProtocolStringField(values, "artifact_id")
	if err != nil {
		return ArtifactDTO{}, err
	}
	taskID, err := requireProtocolStringField(values, "task_id")
	if err != nil {
		return ArtifactDTO{}, err
	}
	artifactType, err := requireProtocolStringField(values, "artifact_type")
	if err != nil {
		return ArtifactDTO{}, err
	}
	title, err := requireProtocolStringField(values, "title")
	if err != nil {
		return ArtifactDTO{}, err
	}
	path, err := requireProtocolStringField(values, "path")
	if err != nil {
		return ArtifactDTO{}, err
	}
	mimeType, err := requireProtocolStringField(values, "mime_type")
	if err != nil {
		return ArtifactDTO{}, err
	}
	return ArtifactDTO{
		ArtifactID:   artifactID,
		TaskID:       taskID,
		ArtifactType: artifactType,
		Title:        title,
		Path:         path,
		MimeType:     mimeType,
	}, nil
}

func citationDTOFromMap(values map[string]any) (CitationDTO, error) {
	citationID, err := requireProtocolStringField(values, "citation_id")
	if err != nil {
		return CitationDTO{}, err
	}
	taskID, err := requireProtocolStringField(values, "task_id")
	if err != nil {
		return CitationDTO{}, err
	}
	runID, err := requireProtocolStringField(values, "run_id")
	if err != nil {
		return CitationDTO{}, err
	}
	sourceType, err := requireProtocolStringField(values, "source_type")
	if err != nil {
		return CitationDTO{}, err
	}
	sourceRef, err := requireProtocolStringField(values, "source_ref")
	if err != nil {
		return CitationDTO{}, err
	}
	label, err := requireProtocolStringField(values, "label")
	if err != nil {
		return CitationDTO{}, err
	}
	artifactID, err := protocolStringField(values, "artifact_id")
	if err != nil {
		return CitationDTO{}, err
	}
	artifactType, err := protocolStringField(values, "artifact_type")
	if err != nil {
		return CitationDTO{}, err
	}
	evidenceRole, err := protocolStringField(values, "evidence_role")
	if err != nil {
		return CitationDTO{}, err
	}
	excerptText, err := protocolStringField(values, "excerpt_text")
	if err != nil {
		return CitationDTO{}, err
	}
	screenSessionID, err := protocolStringField(values, "screen_session_id")
	if err != nil {
		return CitationDTO{}, err
	}
	return CitationDTO{
		CitationID:      citationID,
		TaskID:          taskID,
		RunID:           runID,
		SourceType:      sourceType,
		SourceRef:       sourceRef,
		Label:           label,
		ArtifactID:      artifactID,
		ArtifactType:    artifactType,
		EvidenceRole:    evidenceRole,
		ExcerptText:     excerptText,
		ScreenSessionID: screenSessionID,
	}, nil
}

func mirrorReferenceDTOFromMap(values map[string]any) (MirrorReferenceDTO, error) {
	memoryID, err := requireProtocolStringField(values, "memory_id")
	if err != nil {
		return MirrorReferenceDTO{}, err
	}
	reason, err := requireProtocolStringField(values, "reason")
	if err != nil {
		return MirrorReferenceDTO{}, err
	}
	summary, err := requireProtocolStringField(values, "summary")
	if err != nil {
		return MirrorReferenceDTO{}, err
	}
	return MirrorReferenceDTO{MemoryID: memoryID, Reason: reason, Summary: summary}, nil
}

func approvalRequestDTOFromMap(values map[string]any) (ApprovalRequestDTO, error) {
	approvalID, err := requireProtocolStringField(values, "approval_id")
	if err != nil {
		return ApprovalRequestDTO{}, err
	}
	taskID, err := requireProtocolStringField(values, "task_id")
	if err != nil {
		return ApprovalRequestDTO{}, err
	}
	operationName, err := requireProtocolStringField(values, "operation_name")
	if err != nil {
		return ApprovalRequestDTO{}, err
	}
	riskLevel, err := requireProtocolStringField(values, "risk_level")
	if err != nil {
		return ApprovalRequestDTO{}, err
	}
	targetObject, err := requireProtocolStringField(values, "target_object")
	if err != nil {
		return ApprovalRequestDTO{}, err
	}
	reason, err := requireProtocolStringField(values, "reason")
	if err != nil {
		return ApprovalRequestDTO{}, err
	}
	status, err := requireProtocolStringField(values, "status")
	if err != nil {
		return ApprovalRequestDTO{}, err
	}
	createdAt, err := requireProtocolStringField(values, "created_at")
	if err != nil {
		return ApprovalRequestDTO{}, err
	}
	return ApprovalRequestDTO{
		ApprovalID:    approvalID,
		TaskID:        taskID,
		OperationName: operationName,
		RiskLevel:     riskLevel,
		TargetObject:  targetObject,
		Reason:        reason,
		Status:        status,
		CreatedAt:     createdAt,
	}, nil
}

func authorizationRecordDTOFromMap(values map[string]any) (AuthorizationRecordDTO, error) {
	recordID, err := requireProtocolStringField(values, "authorization_record_id")
	if err != nil {
		return AuthorizationRecordDTO{}, err
	}
	taskID, err := requireProtocolStringField(values, "task_id")
	if err != nil {
		return AuthorizationRecordDTO{}, err
	}
	approvalID, err := requireProtocolStringField(values, "approval_id")
	if err != nil {
		return AuthorizationRecordDTO{}, err
	}
	decision, err := requireProtocolStringField(values, "decision")
	if err != nil {
		return AuthorizationRecordDTO{}, err
	}
	rememberRule, err := requireProtocolBoolField(values, "remember_rule")
	if err != nil {
		return AuthorizationRecordDTO{}, err
	}
	operator, err := requireProtocolStringField(values, "operator")
	if err != nil {
		return AuthorizationRecordDTO{}, err
	}
	createdAt, err := requireProtocolStringField(values, "created_at")
	if err != nil {
		return AuthorizationRecordDTO{}, err
	}
	return AuthorizationRecordDTO{
		AuthorizationRecordID: recordID,
		TaskID:                taskID,
		ApprovalID:            approvalID,
		Decision:              decision,
		RememberRule:          rememberRule,
		Operator:              operator,
		CreatedAt:             createdAt,
	}, nil
}

func auditRecordDTOFromMap(values map[string]any) (AuditRecordDTO, error) {
	auditID, err := requireProtocolStringField(values, "audit_id")
	if err != nil {
		return AuditRecordDTO{}, err
	}
	taskID, err := requireProtocolStringField(values, "task_id")
	if err != nil {
		return AuditRecordDTO{}, err
	}
	recordType, err := requireProtocolStringField(values, "type")
	if err != nil {
		return AuditRecordDTO{}, err
	}
	action, err := requireProtocolStringField(values, "action")
	if err != nil {
		return AuditRecordDTO{}, err
	}
	summary, err := requireProtocolStringField(values, "summary")
	if err != nil {
		return AuditRecordDTO{}, err
	}
	target, err := requireProtocolStringField(values, "target")
	if err != nil {
		return AuditRecordDTO{}, err
	}
	result, err := requireProtocolStringField(values, "result")
	if err != nil {
		return AuditRecordDTO{}, err
	}
	createdAt, err := requireProtocolStringField(values, "created_at")
	if err != nil {
		return AuditRecordDTO{}, err
	}
	return AuditRecordDTO{
		AuditID:   auditID,
		TaskID:    taskID,
		Type:      recordType,
		Action:    action,
		Summary:   summary,
		Target:    target,
		Result:    result,
		CreatedAt: createdAt,
	}, nil
}

func recoveryPointDTOPointerFromMap(values map[string]any, key string) (*RecoveryPointDTO, error) {
	payload, ok, err := protocolMapField(values, key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	recoveryPoint, err := recoveryPointDTOFromMap(payload)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	return &recoveryPoint, nil
}

func recoveryPointDTOFromMap(values map[string]any) (RecoveryPointDTO, error) {
	recoveryPointID, err := requireProtocolStringField(values, "recovery_point_id")
	if err != nil {
		return RecoveryPointDTO{}, err
	}
	taskID, err := requireProtocolStringField(values, "task_id")
	if err != nil {
		return RecoveryPointDTO{}, err
	}
	summary, err := requireProtocolStringField(values, "summary")
	if err != nil {
		return RecoveryPointDTO{}, err
	}
	createdAt, err := requireProtocolStringField(values, "created_at")
	if err != nil {
		return RecoveryPointDTO{}, err
	}
	objects, err := requireProtocolStringSliceField(values, "objects")
	if err != nil {
		return RecoveryPointDTO{}, err
	}
	return RecoveryPointDTO{
		RecoveryPointID: recoveryPointID,
		TaskID:          taskID,
		Summary:         summary,
		CreatedAt:       createdAt,
		Objects:         objects,
	}, nil
}

func securitySummaryDTOFromMap(values map[string]any) (SecuritySummaryDTO, error) {
	securityStatus, err := requireProtocolStringField(values, "security_status")
	if err != nil {
		return SecuritySummaryDTO{}, err
	}
	riskLevel, err := requireProtocolStringField(values, "risk_level")
	if err != nil {
		return SecuritySummaryDTO{}, err
	}
	pendingAuthorizations, err := requireProtocolIntField(values, "pending_authorizations")
	if err != nil {
		return SecuritySummaryDTO{}, err
	}
	latestRestorePoint, err := recoveryPointDTOPointerFromMap(values, "latest_restore_point")
	if err != nil {
		return SecuritySummaryDTO{}, err
	}
	return SecuritySummaryDTO{
		SecurityStatus:        securityStatus,
		RiskLevel:             riskLevel,
		PendingAuthorizations: pendingAuthorizations,
		LatestRestorePoint:    latestRestorePoint,
	}, nil
}

func runtimeSummaryDTOFromMap(values map[string]any) (TaskRuntimeSummaryDTO, error) {
	loopStopReason, err := protocolStringPointerField(values, "loop_stop_reason")
	if err != nil {
		return TaskRuntimeSummaryDTO{}, err
	}
	eventsCount, err := requireProtocolIntField(values, "events_count")
	if err != nil {
		return TaskRuntimeSummaryDTO{}, err
	}
	latestEventType, err := protocolStringPointerField(values, "latest_event_type")
	if err != nil {
		return TaskRuntimeSummaryDTO{}, err
	}
	activeSteeringCount, err := requireProtocolIntField(values, "active_steering_count")
	if err != nil {
		return TaskRuntimeSummaryDTO{}, err
	}
	latestFailureCode, err := protocolStringPointerField(values, "latest_failure_code")
	if err != nil {
		return TaskRuntimeSummaryDTO{}, err
	}
	latestFailureCategory, err := protocolStringPointerField(values, "latest_failure_category")
	if err != nil {
		return TaskRuntimeSummaryDTO{}, err
	}
	latestFailureSummary, err := protocolStringPointerField(values, "latest_failure_summary")
	if err != nil {
		return TaskRuntimeSummaryDTO{}, err
	}
	observationSignals, err := requireProtocolStringSliceField(values, "observation_signals")
	if err != nil {
		return TaskRuntimeSummaryDTO{}, err
	}
	return TaskRuntimeSummaryDTO{
		LoopStopReason:        loopStopReason,
		EventsCount:           eventsCount,
		LatestEventType:       latestEventType,
		ActiveSteeringCount:   activeSteeringCount,
		LatestFailureCode:     latestFailureCode,
		LatestFailureCategory: latestFailureCategory,
		LatestFailureSummary:  latestFailureSummary,
		ObservationSignals:    observationSignals,
	}, nil
}

func protocolMapField(values map[string]any, key string) (map[string]any, bool, error) {
	rawValue, ok := values[key]
	if !ok || rawValue == nil {
		return nil, false, nil
	}
	value, ok := rawValue.(map[string]any)
	if !ok {
		return nil, false, protocolTypeError(key, "object", rawValue)
	}
	return value, true, nil
}

func requireProtocolMapField(values map[string]any, key string) (map[string]any, error) {
	value, ok, err := protocolMapField(values, key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%s must be object", key)
	}
	return value, nil
}

func protocolStringField(values map[string]any, key string) (string, error) {
	rawValue, ok := values[key]
	if !ok || rawValue == nil {
		return "", nil
	}
	value, ok := rawValue.(string)
	if !ok {
		return "", protocolTypeError(key, "string", rawValue)
	}
	return value, nil
}

func requireProtocolStringField(values map[string]any, key string) (string, error) {
	rawValue, ok := values[key]
	if !ok || rawValue == nil {
		return "", fmt.Errorf("%s must be string", key)
	}
	value, ok := rawValue.(string)
	if !ok {
		return "", protocolTypeError(key, "string", rawValue)
	}
	return value, nil
}

func protocolStringPointerField(values map[string]any, key string) (*string, error) {
	rawValue, ok := values[key]
	if !ok || rawValue == nil {
		return nil, nil
	}
	value, ok := rawValue.(string)
	if !ok {
		return nil, protocolTypeError(key, "string or null", rawValue)
	}
	return &value, nil
}

func protocolBoolField(values map[string]any, key string) (bool, error) {
	rawValue, ok := values[key]
	if !ok || rawValue == nil {
		return false, nil
	}
	value, ok := rawValue.(bool)
	if !ok {
		return false, protocolTypeError(key, "boolean", rawValue)
	}
	return value, nil
}

func requireProtocolBoolField(values map[string]any, key string) (bool, error) {
	rawValue, ok := values[key]
	if !ok || rawValue == nil {
		return false, fmt.Errorf("%s must be boolean", key)
	}
	return protocolBoolField(values, key)
}

func protocolIntField(values map[string]any, key string) (int, error) {
	rawValue, ok := values[key]
	if !ok || rawValue == nil {
		return 0, nil
	}
	switch value := rawValue.(type) {
	case int:
		return value, nil
	case int32:
		return int(value), nil
	case int64:
		return int(value), nil
	case float32:
		return int(value), nil
	case float64:
		return int(value), nil
	default:
		return 0, protocolTypeError(key, "number", rawValue)
	}
}

func requireProtocolIntField(values map[string]any, key string) (int, error) {
	rawValue, ok := values[key]
	if !ok || rawValue == nil {
		return 0, fmt.Errorf("%s must be number", key)
	}
	return protocolIntField(values, key)
}

func protocolStringSliceField(values map[string]any, key string) ([]string, error) {
	rawValue, ok := values[key]
	if !ok || rawValue == nil {
		return nil, nil
	}
	switch value := rawValue.(type) {
	case []string:
		return append([]string(nil), value...), nil
	case []any:
		result := make([]string, 0, len(value))
		for index, rawItem := range value {
			item, ok := rawItem.(string)
			if !ok {
				return nil, protocolIndexedTypeError(key, index, "string", rawItem)
			}
			result = append(result, item)
		}
		return result, nil
	default:
		return nil, protocolTypeError(key, "array of strings", rawValue)
	}
}

func requireProtocolStringSliceField(values map[string]any, key string) ([]string, error) {
	rawValue, ok := values[key]
	if !ok || rawValue == nil {
		return nil, fmt.Errorf("%s must be array of strings", key)
	}
	return protocolStringSliceField(values, key)
}

func cloneProtocolMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	cloned := cloneMap(values)
	if cloned == nil {
		return map[string]any{}
	}
	return cloned
}

func protocolTypeError(key, expected string, rawValue any) error {
	return fmt.Errorf("%s must be %s, got %T", key, expected, rawValue)
}

func protocolIndexedTypeError(key string, index int, expected string, rawValue any) error {
	return fmt.Errorf("%s[%d] must be %s, got %T", key, index, expected, rawValue)
}

func requestMetaFromMap(values map[string]any) RequestMeta {
	return RequestMeta{
		TraceID:    stringValue(values, "trace_id", ""),
		ClientTime: stringValue(values, "client_time", ""),
	}
}

func pageContextPointerFromMap(values map[string]any) *PageContext {
	if len(values) == 0 {
		return nil
	}
	return &PageContext{
		Title:       stringValue(values, "title", ""),
		AppName:     stringValue(values, "app_name", ""),
		URL:         stringValue(values, "url", ""),
		BrowserKind: stringValue(values, "browser_kind", ""),
		ProcessPath: stringValue(values, "process_path", ""),
		ProcessID:   intValue(values, "process_id", 0),
		WindowTitle: stringValue(values, "window_title", ""),
		VisibleText: stringValue(values, "visible_text", ""),
		HoverTarget: stringValue(values, "hover_target", ""),
	}
}

func screenContextPointerFromMap(values map[string]any) *ScreenContext {
	if len(values) == 0 {
		return nil
	}
	return &ScreenContext{
		Summary:       stringValue(values, "summary", ""),
		ScreenSummary: stringValue(values, "screen_summary", ""),
		VisibleText:   stringValue(values, "visible_text", ""),
		WindowTitle:   stringValue(values, "window_title", ""),
		HoverTarget:   stringValue(values, "hover_target", ""),
	}
}

func behaviorContextPointerFromMap(values map[string]any) *BehaviorContext {
	if len(values) == 0 {
		return nil
	}
	return &BehaviorContext{
		LastAction:        stringValue(values, "last_action", ""),
		DwellMillis:       intValue(values, "dwell_millis", 0),
		CopyCount:         intValue(values, "copy_count", 0),
		WindowSwitchCount: intValue(values, "window_switch_count", 0),
		PageSwitchCount:   intValue(values, "page_switch_count", 0),
	}
}

func selectionContextPointerFromMap(values map[string]any) *SelectionContext {
	if len(values) == 0 {
		return nil
	}
	return &SelectionContext{Text: stringValue(values, "text", "")}
}

func errorContextPointerFromMap(values map[string]any) *ErrorContext {
	if len(values) == 0 {
		return nil
	}
	return &ErrorContext{Message: stringValue(values, "message", "")}
}

func clipboardContextPointerFromMap(values map[string]any) *ClipboardContext {
	if len(values) == 0 {
		return nil
	}
	return &ClipboardContext{Text: stringValue(values, "text", "")}
}

func inputContextPointerFromMap(values map[string]any) *InputContext {
	if len(values) == 0 {
		return nil
	}
	return &InputContext{
		Page:              pageContextPointerFromMap(mapValue(values, "page")),
		Screen:            screenContextPointerFromMap(mapValue(values, "screen")),
		Behavior:          behaviorContextPointerFromMap(mapValue(values, "behavior")),
		Selection:         selectionContextPointerFromMap(mapValue(values, "selection")),
		Error:             errorContextPointerFromMap(mapValue(values, "error")),
		Clipboard:         clipboardContextPointerFromMap(mapValue(values, "clipboard")),
		Text:              stringValue(values, "text", ""),
		SelectionText:     stringValue(values, "selection_text", ""),
		Files:             stringSliceValue(values["files"]),
		FilePaths:         stringSliceValue(values["file_paths"]),
		ScreenSummary:     stringValue(values, "screen_summary", ""),
		ClipboardText:     stringValue(values, "clipboard_text", ""),
		HoverTarget:       stringValue(values, "hover_target", ""),
		LastAction:        stringValue(values, "last_action", ""),
		DwellMillis:       intValue(values, "dwell_millis", 0),
		CopyCount:         intValue(values, "copy_count", 0),
		WindowSwitchCount: intValue(values, "window_switch_count", 0),
		PageSwitchCount:   intValue(values, "page_switch_count", 0),
	}
}

func voiceMetaPointerFromMap(values map[string]any) *VoiceMeta {
	if len(values) == 0 {
		return nil
	}
	return &VoiceMeta{
		VoiceSessionID:  stringValue(values, "voice_session_id", ""),
		IsLockedSession: boolValue(values, "is_locked_session", false),
		ASRConfidence:   floatValue(values, "asr_confidence", 0),
		SegmentID:       stringValue(values, "segment_id", ""),
	}
}

func inputSubmitInputFromMap(values map[string]any) InputSubmitInput {
	return InputSubmitInput{
		Type:      stringValue(values, "type", ""),
		Text:      stringValue(values, "text", ""),
		InputMode: stringValue(values, "input_mode", ""),
	}
}

func inputSubmitOptionsPointerFromMap(values map[string]any) *InputSubmitOptions {
	if len(values) == 0 {
		return nil
	}
	return &InputSubmitOptions{
		ConfirmRequired:   boolValue(values, "confirm_required", false),
		PreferredDelivery: stringValue(values, "preferred_delivery", ""),
	}
}

func taskStartInputFromMap(values map[string]any) TaskStartInput {
	return TaskStartInput{
		Type:         stringValue(values, "type", ""),
		Text:         stringValue(values, "text", ""),
		Files:        stringSliceValue(values["files"]),
		PageContext:  pageContextPointerFromMap(mapValue(values, "page_context")),
		ErrorMessage: stringValue(values, "error_message", ""),
	}
}

func deliveryPreferencePointerFromMap(values map[string]any) *DeliveryPreference {
	if len(values) == 0 {
		return nil
	}
	return &DeliveryPreference{
		Preferred: stringValue(values, "preferred", ""),
		Fallback:  stringValue(values, "fallback", ""),
	}
}

func taskStartOptionsPointerFromMap(values map[string]any) *TaskStartOptions {
	if len(values) == 0 {
		return nil
	}
	return &TaskStartOptions{
		ConfirmRequired: boolValue(values, "confirm_required", false),
	}
}

func floatValue(values map[string]any, key string, fallback float64) float64 {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}
	switch value := rawValue.(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int32:
		return float64(value)
	case int64:
		return float64(value)
	default:
		return fallback
	}
}

func protocolMapFromDTO(value any) map[string]any {
	result, ok := protocolValueFromReflect(reflect.ValueOf(value)).(map[string]any)
	if !ok || result == nil {
		return map[string]any{}
	}
	return result
}

func protocolValueFromReflect(value reflect.Value) any {
	if !value.IsValid() {
		return nil
	}
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}

	switch value.Kind() {
	case reflect.Struct:
		result := map[string]any{}
		valueType := value.Type()
		for index := 0; index < value.NumField(); index++ {
			field := valueType.Field(index)
			if !field.IsExported() {
				continue
			}
			name, omitEmpty := jsonFieldName(field)
			if name == "" {
				continue
			}
			fieldValue := value.Field(index)
			if omitEmpty && isJSONEmptyValue(fieldValue) {
				continue
			}
			result[name] = protocolValueFromReflect(fieldValue)
		}
		return result
	case reflect.Slice, reflect.Array:
		return protocolSliceValue(value)
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String {
			return nil
		}
		if value.IsNil() {
			return map[string]any(nil)
		}
		result := make(map[string]any, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			result[iter.Key().String()] = protocolValueFromReflect(iter.Value())
		}
		return result
	default:
		return value.Interface()
	}
}

func protocolSliceValue(value reflect.Value) any {
	length := value.Len()
	elemKind := value.Type().Elem().Kind()
	switch elemKind {
	case reflect.Struct, reflect.Map:
		result := make([]map[string]any, 0, length)
		for index := 0; index < length; index++ {
			item, ok := protocolValueFromReflect(value.Index(index)).(map[string]any)
			if !ok {
				return protocolSliceFallback(value)
			}
			result = append(result, item)
		}
		return result
	case reflect.String:
		result := make([]string, 0, length)
		for index := 0; index < length; index++ {
			result = append(result, value.Index(index).String())
		}
		return result
	case reflect.Bool:
		result := make([]bool, 0, length)
		for index := 0; index < length; index++ {
			result = append(result, value.Index(index).Bool())
		}
		return result
	default:
		return protocolSliceFallback(value)
	}
}

func protocolSliceFallback(value reflect.Value) []any {
	result := make([]any, 0, value.Len())
	for index := 0; index < value.Len(); index++ {
		result = append(result, protocolValueFromReflect(value.Index(index)))
	}
	return result
}

func jsonFieldName(field reflect.StructField) (string, bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", false
	}
	if tag == "" {
		return field.Name, false
	}
	parts := strings.Split(tag, ",")
	name := parts[0]
	if name == "" {
		name = field.Name
	}
	for _, option := range parts[1:] {
		if option == "omitempty" {
			return name, true
		}
	}
	return name, false
}

func isJSONEmptyValue(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return value.Len() == 0
	case reflect.Bool:
		return !value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return value.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return value.IsNil()
	}
	return false
}
