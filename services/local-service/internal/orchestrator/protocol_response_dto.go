package orchestrator

import (
	"encoding/json"
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
	RunID                 string `json:"run_id,omitempty"`
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

// StartTaskRequestFromParams adapts RPC-decoded params to the typed
// orchestrator request. The map is accepted only at the RPC adapter boundary,
// then normalized back through the typed DTO before orchestration continues.
func StartTaskRequestFromParams(params map[string]any) StartTaskRequest {
	var request StartTaskRequest
	decodeProtocolMap(params, &request)
	if intent := mapValue(params, "intent"); len(intent) > 0 {
		request.Intent = cloneMap(intent)
	}
	return request
}

// SubmitInputRequestFromParams adapts RPC-decoded params to the typed
// orchestrator request. The map is accepted only at the RPC adapter boundary,
// then normalized back through the typed DTO before orchestration continues.
func SubmitInputRequestFromParams(params map[string]any) SubmitInputRequest {
	var request SubmitInputRequest
	decodeProtocolMap(params, &request)
	return request
}

// TaskDetailGetRequestFromParams adapts RPC-decoded params to the typed
// orchestrator request. The map is accepted only at the RPC adapter boundary,
// then normalized back through the typed DTO before orchestration continues.
func TaskDetailGetRequestFromParams(params map[string]any) TaskDetailGetRequest {
	var request TaskDetailGetRequest
	decodeProtocolMap(params, &request)
	return request
}

func (r StartTaskRequest) paramsMap() map[string]any {
	params := structToProtocolMap(r)
	if len(r.Intent) > 0 {
		params["intent"] = cloneMap(r.Intent)
	}
	return params
}

func (r SubmitInputRequest) paramsMap() map[string]any {
	return structToProtocolMap(r)
}

func (r TaskDetailGetRequest) paramsMap() map[string]any {
	return structToProtocolMap(r)
}

func newTaskEntryResponse(payload map[string]any) TaskEntryResponse {
	var response TaskEntryResponse
	decodeProtocolMap(payload, &response)
	return response
}

func newTaskDetailGetResponse(payload map[string]any) TaskDetailGetResponse {
	var response TaskDetailGetResponse
	decodeProtocolMap(payload, &response)
	return response
}

// Map returns the protocol payload as a map for package tests that assert
// individual fields. Production callers should consume the typed DTO directly.
func (r TaskEntryResponse) Map() map[string]any {
	return responseDTOToProtocolMap(r)
}

// Map returns the protocol payload as a map for package tests that assert
// individual fields. Production callers should consume the typed DTO directly.
func (r TaskDetailGetResponse) Map() map[string]any {
	return responseDTOToProtocolMap(r)
}

func decodeProtocolMap(values map[string]any, target any) {
	if len(values) == 0 {
		return
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return
	}
	_ = json.Unmarshal(payload, target)
}

func structToProtocolMap(value any) map[string]any {
	payload, err := json.Marshal(value)
	if err != nil {
		return map[string]any{}
	}
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		return map[string]any{}
	}
	return result
}

func responseDTOToProtocolMap(value any) map[string]any {
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
