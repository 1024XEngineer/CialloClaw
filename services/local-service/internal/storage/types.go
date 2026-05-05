// Package storage defines shared storage contracts and persisted record shapes.
package storage

import (
	"context"
	"errors"
	"time"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/audit"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/checkpoint"
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/tools"
)

// CapabilitySnapshot describes the currently available storage capabilities.
type CapabilitySnapshot struct {
	Backend                string
	Configured             bool
	SupportsStructuredData bool
	SupportsMemoryStore    bool
	SupportsToolCallSink   bool
	SupportsRetrievalHits  bool
	SupportsFTS5           bool
	SupportsSQLiteVecStub  bool
	SupportsArtifactStore  bool
	SupportsLoopRuntime    bool
	SupportsSecretStore    bool
	MemoryStoreBackend     string
	ToolCallStoreBackend   string
	ArtifactStoreBackend   string
	SecretStoreBackend     string
	MemoryRetrievalBackend string
	FallbackActive         bool
}

var (
	// ErrSecretNotFound reports that the requested secret key does not exist.
	ErrSecretNotFound = errors.New("secret not found")
	// ErrSecretStoreAccessFailed reports that the stronghold-compatible secret store could not be accessed.
	ErrSecretStoreAccessFailed = errors.New("secret store access failed")
)

// MemorySummaryRecord describes one persisted memory summary row.
type MemorySummaryRecord struct {
	MemorySummaryID string
	TaskID          string
	RunID           string
	Summary         string
	CreatedAt       string
}

// MemoryRetrievalRecord describes one persisted memory retrieval-hit row.
type MemoryRetrievalRecord struct {
	RetrievalHitID string
	TaskID         string
	RunID          string
	MemoryID       string
	Score          float64
	Source         string
	Summary        string
	CreatedAt      string
}

// MemoryStore defines the persistence contract for memory summaries and hits.
type MemoryStore interface {
	SaveSummary(ctx context.Context, summary MemorySummaryRecord) error
	SaveRetrievalHits(ctx context.Context, hits []MemoryRetrievalRecord) error
	SearchSummaries(ctx context.Context, taskID, runID, query string, limit int) ([]MemoryRetrievalRecord, error)
	ListRecentSummaries(ctx context.Context, limit int) ([]MemorySummaryRecord, error)
}

// ArtifactRecord describes one persisted artifact snapshot.
type ArtifactRecord struct {
	ArtifactID          string
	TaskID              string
	RunID               string
	ArtifactType        string
	Title               string
	Path                string
	MimeType            string
	DeliveryType        string
	DeliveryPayloadJSON string
	CreatedAt           string
}

// ArtifactStore defines artifact persistence and lookup behavior.
type ArtifactStore interface {
	SaveArtifacts(ctx context.Context, records []ArtifactRecord) error
	ListArtifacts(ctx context.Context, taskID, runID string, limit, offset int) ([]ArtifactRecord, int, error)
}

// TodoItemRecord describes one persisted notes/todo snapshot.
type TodoItemRecord struct {
	ItemID               string
	Title                string
	Bucket               string
	Status               string
	SourcePath           string
	SourceLine           int
	SourceBucket         string
	DueAt                string
	TagsJSON             string
	AgentSuggestion      string
	NoteText             string
	Prerequisite         string
	PlannedAt            string
	PreviousBucket       string
	PreviousDueAt        string
	PreviousStatus       string
	EndedAt              string
	RelatedResourcesJSON string
	LinkedTaskID         string
	CreatedAt            string
	UpdatedAt            string
}

// RecurringRuleRecord describes one persisted recurring-rule snapshot.
type RecurringRuleRecord struct {
	RuleID               string
	ItemID               string
	RuleType             string
	CronExpr             string
	IntervalValue        int
	IntervalUnit         string
	ReminderStrategy     string
	Enabled              bool
	RepeatRuleText       string
	NextOccurrenceAt     string
	RecentInstanceStatus string
	EffectiveScope       string
	CreatedAt            string
	UpdatedAt            string
}

// TodoStore defines persistence for notes/todo items and recurring rules.
type TodoStore interface {
	ReplaceTodoState(ctx context.Context, items []TodoItemRecord, rules []RecurringRuleRecord) error
	LoadTodoState(ctx context.Context) ([]TodoItemRecord, []RecurringRuleRecord, error)
}

// SettingsStore persists the ordinary non-secret settings snapshot used by
// `agent.settings.get / agent.settings.update`.
type SettingsStore interface {
	SaveSettingsSnapshot(ctx context.Context, snapshot map[string]any) error
	LoadSettingsSnapshot(ctx context.Context) (map[string]any, error)
}

// TraceRecord describes one persisted execution trace snapshot.
type TraceRecord struct {
	TraceID          string
	TaskID           string
	RunID            string
	LoopRound        int
	LLMInputSummary  string
	LLMOutputSummary string
	LatencyMS        int64
	Cost             float64
	AssetRefsJSON    string
	RuleHitsJSON     string
	ReviewResult     string
	CreatedAt        string
}

// EvalSnapshotRecord describes one persisted evaluation snapshot.
type EvalSnapshotRecord struct {
	EvalSnapshotID string
	TraceID        string
	TaskID         string
	Status         string
	AssetRefsJSON  string
	MetricsJSON    string
	CreatedAt      string
}

// TraceStore defines persistence for trace records.
type TraceStore interface {
	WriteTraceRecord(ctx context.Context, record TraceRecord) error
	DeleteTraceRecord(ctx context.Context, traceID string) error
	ListTraceRecords(ctx context.Context, taskID string, limit, offset int) ([]TraceRecord, int, error)
}

// EvalStore defines persistence for evaluation snapshots.
type EvalStore interface {
	WriteEvalSnapshot(ctx context.Context, record EvalSnapshotRecord) error
	ListEvalSnapshots(ctx context.Context, taskID string, limit, offset int) ([]EvalSnapshotRecord, int, error)
}

// SkillManifestRecord reserves the formal skill_manifests table without turning
// on any marketplace or installation flow ahead of the current roadmap.
type SkillManifestRecord struct {
	SkillManifestID string
	Name            string
	Version         string
	Source          string
	Summary         string
	ManifestJSON    string
	CreatedAt       string
	UpdatedAt       string
}

// BlueprintDefinitionRecord reserves the formal blueprint_definitions table for
// future execution assets without expanding blueprint product behavior yet.
type BlueprintDefinitionRecord struct {
	BlueprintDefinitionID string
	Name                  string
	Version               string
	Source                string
	Summary               string
	DefinitionJSON        string
	CreatedAt             string
	UpdatedAt             string
}

// PromptTemplateVersionRecord reserves the formal prompt_template_versions
// table so future prompt assets can attach to traceable versioned records.
type PromptTemplateVersionRecord struct {
	PromptTemplateVersionID string
	TemplateName            string
	Version                 string
	Source                  string
	Summary                 string
	TemplateBody            string
	VariablesJSON           string
	CreatedAt               string
	UpdatedAt               string
}

// SkillManifestStore persists versioned skill manifest assets.
type SkillManifestStore interface {
	WriteSkillManifest(ctx context.Context, record SkillManifestRecord) error
	GetSkillManifest(ctx context.Context, skillManifestID string) (SkillManifestRecord, error)
	ListSkillManifests(ctx context.Context, limit, offset int) ([]SkillManifestRecord, int, error)
}

// BlueprintDefinitionStore persists versioned blueprint definition assets.
type BlueprintDefinitionStore interface {
	WriteBlueprintDefinition(ctx context.Context, record BlueprintDefinitionRecord) error
	GetBlueprintDefinition(ctx context.Context, blueprintDefinitionID string) (BlueprintDefinitionRecord, error)
	ListBlueprintDefinitions(ctx context.Context, limit, offset int) ([]BlueprintDefinitionRecord, int, error)
}

// PromptTemplateVersionStore persists versioned prompt template assets.
type PromptTemplateVersionStore interface {
	WritePromptTemplateVersion(ctx context.Context, record PromptTemplateVersionRecord) error
	GetPromptTemplateVersion(ctx context.Context, promptTemplateVersionID string) (PromptTemplateVersionRecord, error)
	ListPromptTemplateVersions(ctx context.Context, limit, offset int) ([]PromptTemplateVersionRecord, int, error)
}

// PluginManifestRecord persists one formal plugin_manifests row without turning
// on plugin marketplace or install flows before the roadmap reaches them.
type PluginManifestRecord struct {
	PluginID         string
	Name             string
	Version          string
	Entry            string
	Source           string
	Summary          string
	CapabilitiesJSON string
	PermissionsJSON  string
	RuntimeNamesJSON string
	CreatedAt        string
	UpdatedAt        string
}

// PluginManifestStore persists versioned plugin manifest assets.
type PluginManifestStore interface {
	WritePluginManifest(ctx context.Context, record PluginManifestRecord) error
	GetPluginManifest(ctx context.Context, pluginID string) (PluginManifestRecord, error)
	ListPluginManifests(ctx context.Context, limit, offset int) ([]PluginManifestRecord, int, error)
}

// ExtensionAssetReference records one concrete versioned asset that execution,
// trace, and eval can attribute to a task/run without exposing any marketplace
// or installation workflow.
type ExtensionAssetReference struct {
	AssetKind    string   `json:"asset_kind"`
	AssetID      string   `json:"asset_id"`
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Source       string   `json:"source"`
	Summary      string   `json:"summary,omitempty"`
	Entry        string   `json:"entry,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Permissions  []string `json:"permissions,omitempty"`
	RuntimeNames []string `json:"runtime_names,omitempty"`
}

// SecretRecord captures one secret value persisted outside the normal settings path.
type SecretRecord struct {
	Namespace string
	Key       string
	Value     string
	UpdatedAt string
}

// SecretStore defines Stronghold-compatible secret storage behavior.
type SecretStore interface {
	PutSecret(ctx context.Context, record SecretRecord) error
	GetSecret(ctx context.Context, namespace, key string) (SecretRecord, error)
	DeleteSecret(ctx context.Context, namespace, key string) error
}

// StrongholdProvider defines the formal Stronghold lifecycle boundary. The
// backend can bind a real Stronghold runtime here while keeping SQLite as a
// development fallback instead of pretending it is the formal secret source.
type StrongholdProvider interface {
	Open(ctx context.Context) (SecretStore, error)
	Descriptor() StrongholdDescriptor
}

// StrongholdDescriptor exposes the current Stronghold lifecycle status without
// leaking secrets into settings snapshots or normal task state payloads.
type StrongholdDescriptor struct {
	Backend     string
	Available   bool
	Fallback    bool
	Initialized bool
}

// TaskStepSnapshot describes the storage snapshot for one task timeline entry.
type TaskStepSnapshot struct {
	StepID        string
	TaskID        string
	Name          string
	Status        string
	OrderIndex    int
	InputSummary  string
	OutputSummary string
}

// NotificationSnapshot describes one pending notification snapshot in storage.
type NotificationSnapshot struct {
	Method    string
	Params    map[string]any
	CreatedAt time.Time
}

// TaskRunRecord captures the full task/run snapshot persisted by storage.
type TaskRunRecord struct {
	TaskID            string
	SessionID         string
	RunID             string
	RequestSource     string
	RequestTrigger    string
	ExecutionAttempt  int
	Title             string
	SourceType        string
	Status            string
	Intent            map[string]any
	PreferredDelivery string
	FallbackDelivery  string
	CurrentStep       string
	RiskLevel         string
	StartedAt         time.Time
	UpdatedAt         time.Time
	FinishedAt        *time.Time
	Timeline          []TaskStepSnapshot
	BubbleMessage     map[string]any
	DeliveryResult    map[string]any
	Artifacts         []map[string]any
	Citations         []map[string]any
	AuditRecords      []map[string]any
	MirrorReferences  []map[string]any
	Snapshot          contextsvc.TaskContextSnapshot
	SecuritySummary   map[string]any
	ApprovalRequest   map[string]any
	PendingExecution  map[string]any
	Authorization     map[string]any
	ImpactScope       map[string]any
	TokenUsage        map[string]any
	MemoryReadPlans   []map[string]any
	MemoryWritePlans  []map[string]any
	StorageWritePlan  map[string]any
	ArtifactPlans     []map[string]any
	Notifications     []NotificationSnapshot
	LatestEvent       map[string]any
	LatestToolCall    map[string]any
	LoopStopReason    string
	SteeringMessages  []string
	CurrentStepStatus string
}

// TaskRunStore defines persistence for the task/run primary state snapshot.
type TaskRunStore interface {
	AllocateIdentifier(ctx context.Context, prefix string) (string, error)
	DeleteTaskRun(ctx context.Context, taskID string) error
	SaveTaskRun(ctx context.Context, record TaskRunRecord) error
	LoadTaskRuns(ctx context.Context) ([]TaskRunRecord, error)
	GetTaskRun(ctx context.Context, taskID string) (TaskRunRecord, error)
	LoadLegacyTaskRuns(ctx context.Context, structuredTaskIDs []string) ([]TaskRunRecord, error)
}

// TaskRecord describes one first-class tasks row aligned with the product layer.
type TaskRecord struct {
	TaskID              string
	SessionID           string
	RunID               string
	PrimaryRunID        string
	Title               string
	SourceType          string
	Status              string
	IntentName          string
	IntentArgumentsJSON string
	PreferredDelivery   string
	FallbackDelivery    string
	CurrentStep         string
	CurrentStepStatus   string
	RiskLevel           string
	RequestSource       string
	RequestTrigger      string
	StartedAt           string
	UpdatedAt           string
	FinishedAt          string
	SnapshotJSON        string
}

// SessionRecord describes one first-class sessions row aligned with the product layer.
type SessionRecord struct {
	SessionID string
	Title     string
	Status    string
	CreatedAt string
	UpdatedAt string
}

// SessionStore persists first-class sessions rows.
type SessionStore interface {
	WriteSession(ctx context.Context, record SessionRecord) error
	DeleteSession(ctx context.Context, sessionID string) error
	GetSession(ctx context.Context, sessionID string) (SessionRecord, error)
	ListSessions(ctx context.Context, limit, offset int) ([]SessionRecord, int, error)
}

// TaskStepRecord describes one first-class task_steps row for user-facing timelines.
type TaskStepRecord struct {
	StepID        string
	TaskID        string
	Name          string
	Status        string
	OrderIndex    int
	InputSummary  string
	OutputSummary string
	CreatedAt     string
	UpdatedAt     string
}

// TaskStore persists first-class tasks rows alongside task_runs snapshots.
type TaskStore interface {
	WriteTask(ctx context.Context, record TaskRecord) error
	DeleteTask(ctx context.Context, taskID string) error
	GetTask(ctx context.Context, taskID string) (TaskRecord, error)
	ListTasks(ctx context.Context, limit, offset int) ([]TaskRecord, int, error)
	ListTasksBySession(ctx context.Context, sessionID string, limit, offset int) ([]TaskRecord, int, error)
}

// TaskStepStore persists first-class task_steps rows for task-facing timelines.
type TaskStepStore interface {
	ReplaceTaskSteps(ctx context.Context, taskID string, records []TaskStepRecord) error
	ListTaskSteps(ctx context.Context, taskID string, limit, offset int) ([]TaskStepRecord, int, error)
}

// LoopRuntimeStore defines normalized run/step/event/delivery_result persistence
// plus the task-facing citation snapshots that structured task detail can
// rehydrate without relying on task_run compatibility rows.
type LoopRuntimeStore interface {
	SaveRun(ctx context.Context, record RunRecord) error
	SaveSteps(ctx context.Context, records []StepRecord) error
	SaveEvents(ctx context.Context, records []EventRecord) error
	SaveDeliveryResult(ctx context.Context, record DeliveryResultRecord) error
	GetRun(ctx context.Context, runID string) (RunRecord, error)
	ListDeliveryResults(ctx context.Context, taskID, runID string, limit, offset int) ([]DeliveryResultRecord, int, error)
	ReplaceTaskCitations(ctx context.Context, taskID string, records []CitationRecord) error
	GetLatestDeliveryResult(ctx context.Context, taskID, runID string) (DeliveryResultRecord, bool, error)
	ListTaskCitations(ctx context.Context, taskID, runID string) ([]CitationRecord, error)
	ListEvents(ctx context.Context, taskID, runID, eventType, createdAtFrom, createdAtTo string, limit, offset int) ([]EventRecord, int, error)
}

// ToolCallStore defines persistence for tool_call records.
type ToolCallStore interface {
	SaveToolCall(ctx context.Context, record tools.ToolCallRecord) error
	ListToolCalls(ctx context.Context, taskID, runID string, limit, offset int) ([]tools.ToolCallRecord, int, error)
}

// AuditStore defines persistence for audit records.
type AuditStore interface {
	WriteAuditRecord(ctx context.Context, record audit.Record) error
	ListAuditRecords(ctx context.Context, taskID, runID string, limit, offset int) ([]audit.Record, int, error)
}

// RecoveryPointStore defines persistence for recovery points.
type RecoveryPointStore interface {
	WriteRecoveryPoint(ctx context.Context, point checkpoint.RecoveryPoint) error
	ListRecoveryPoints(ctx context.Context, taskID string, limit, offset int) ([]checkpoint.RecoveryPoint, int, error)
	GetRecoveryPoint(ctx context.Context, recoveryPointID string) (checkpoint.RecoveryPoint, error)
}

// ApprovalRequestRecord describes one persisted approval_requests snapshot.
type ApprovalRequestRecord struct {
	ApprovalID      string
	TaskID          string
	OperationName   string
	RiskLevel       string
	TargetObject    string
	Reason          string
	Status          string
	ImpactScopeJSON string
	CreatedAt       string
	UpdatedAt       string
}

// AuthorizationRecordRecord describes one persisted authorization_records snapshot.
type AuthorizationRecordRecord struct {
	AuthorizationRecordID string
	TaskID                string
	RunID                 string
	ApprovalID            string
	Decision              string
	Operator              string
	RememberRule          bool
	CreatedAt             string
}

// ApprovalRequestStore persists formal approval_requests records.
type ApprovalRequestStore interface {
	WriteApprovalRequest(ctx context.Context, record ApprovalRequestRecord) error
	UpdateApprovalRequestStatus(ctx context.Context, approvalID string, status string, updatedAt string) error
	ListApprovalRequests(ctx context.Context, taskID string, limit, offset int) ([]ApprovalRequestRecord, int, error)
	ListPendingApprovalRequests(ctx context.Context, limit, offset int) ([]ApprovalRequestRecord, int, error)
}

// AuthorizationRecordStore persists formal authorization_records records.
type AuthorizationRecordStore interface {
	WriteAuthorizationRecord(ctx context.Context, record AuthorizationRecordRecord) error
	// WriteAuthorizationDecision persists one authorization_records row and its
	// linked approval_requests status transition inside a single storage boundary.
	WriteAuthorizationDecision(ctx context.Context, record AuthorizationRecordRecord, approvalStatus string, updatedAt string) error
	ListAuthorizationRecords(ctx context.Context, taskID, runID string, limit, offset int) ([]AuthorizationRecordRecord, int, error)
}
