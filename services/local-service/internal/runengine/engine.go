// Package runengine owns the in-memory task/run runtime state machine.
package runengine

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	serviceconfig "github.com/cialloclaw/cialloclaw/services/local-service/internal/config"
	contextsvc "github.com/cialloclaw/cialloclaw/services/local-service/internal/context"
	"github.com/cialloclaw/cialloclaw/services/local-service/internal/storage"
)

const (
	defaultWorkspaceRoot   = "workspace"
	defaultTaskSourcePath  = "workspace/todos"
	defaultRecoveryPathObj = "workspace/temp.md"
)

var (
	ErrTaskNotFound        = errors.New("task not found")
	ErrTaskStatusInvalid   = errors.New("task status invalid")
	ErrTaskAlreadyFinished = errors.New("task already finished")
)

func defaultSettingsWorkspaceRoot() string {
	return filepath.ToSlash(filepath.Clean(serviceconfig.DefaultWorkspaceRoot()))
}

func defaultSettingsTaskSourcePath() string {
	return filepath.ToSlash(filepath.Join(serviceconfig.DefaultWorkspaceRoot(), "todos"))
}

// TaskRecord is the canonical in-memory record that bridges external task
// semantics with internal run execution state.
type TaskRecord struct {
	TaskID            string
	SessionID         string
	RunID             string
	PrimaryRunID      string
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
	Timeline          []TaskStepRecord
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
	Notifications     []NotificationRecord
	LatestEvent       map[string]any
	LatestToolCall    map[string]any
	LoopStopReason    string
	SteeringMessages  []string
	CurrentStepStatus string
}

// TaskStepRecord represents one task-facing timeline step.
type TaskStepRecord struct {
	StepID        string
	TaskID        string
	Name          string
	Status        string
	OrderIndex    int
	InputSummary  string
	OutputSummary string
}

// NotificationRecord stores a buffered notification that the transport will
// replay after the main RPC response is sent.
type NotificationRecord struct {
	Method    string
	Params    map[string]any
	CreatedAt time.Time
}

func taskUpdatedNotificationParams(record *TaskRecord) map[string]any {
	return map[string]any{
		"task_id":    record.TaskID,
		"session_id": taskSessionValue(record.SessionID),
		"status":     record.Status,
	}
}

func taskSessionValue(sessionID string) any {
	if strings.TrimSpace(sessionID) == "" {
		return nil
	}
	return strings.TrimSpace(sessionID)
}

// CreateTaskInput contains the runtime initialization payload for a new task.
type CreateTaskInput struct {
	SessionID         string
	RequestSource     string
	RequestTrigger    string
	Title             string
	SourceType        string
	Status            string
	Intent            map[string]any
	PreferredDelivery string
	FallbackDelivery  string
	CurrentStep       string
	RiskLevel         string
	Timeline          []TaskStepRecord
	BubbleMessage     map[string]any
	DeliveryResult    map[string]any
	Artifacts         []map[string]any
	Citations         []map[string]any
	MirrorReferences  []map[string]any
	Snapshot          contextsvc.TaskContextSnapshot
}

// ContinuationUpdate captures the minimum runtime state changes required when a
// later desktop input should stay on the same task instead of opening a new one.
type ContinuationUpdate struct {
	Snapshot        contextsvc.TaskContextSnapshot
	Title           string
	Intent          map[string]any
	Status          string
	CurrentStep     string
	BubbleMessage   map[string]any
	SteeringMessage string
}

// InspectorConfig stores the current task-inspector runtime settings.
type InspectorConfig struct {
	TaskSources          []string
	InspectionInterval   map[string]any
	InspectOnFileChange  bool
	InspectOnStartup     bool
	RemindBeforeDeadline bool
	RemindWhenStale      bool
}

// Engine is the in-memory runtime state machine for the main task pipeline.
type Engine struct {
	mu            sync.RWMutex
	nextID        uint64
	now           func() time.Time
	taskStore     storage.TaskRunStore
	todoStore     storage.TodoStore
	settingsStore storage.SettingsStore
	sessionStore  storage.SessionStore
	tasks         map[string]*TaskRecord
	taskOrder     []string
	sessionOrder  []string
	inspector     InspectorConfig
	settings      map[string]any
	notepadItems  []map[string]any
	notepadClaims map[string]struct{}
}

// NewEngine constructs a runtime engine with default settings and inspector
// configuration.
func NewEngine() *Engine {
	engine, _ := newEngine(nil)
	return engine
}

// NewEngineWithStore constructs a runtime engine backed by persisted task/run
// storage.
func NewEngineWithStore(taskStore storage.TaskRunStore) (*Engine, error) {
	return newEngine(taskStore)
}

// WithTodoStore attaches todo persistence and hydrates notes state from storage
// when durable records are available.
func (e *Engine) WithTodoStore(todoStore storage.TodoStore) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.todoStore = todoStore
	if todoStore == nil {
		return nil
	}

	items, rules, err := todoStore.LoadTodoState(context.Background())
	if err != nil {
		return err
	}
	loaded := restoreNotepadItemsFromStore(items, rules)
	e.notepadItems = loaded
	return nil
}

// WithSettingsStore attaches ordinary settings persistence and hydrates the
// in-memory settings snapshot from durable storage when records exist.
func (e *Engine) WithSettingsStore(settingsStore storage.SettingsStore) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.settingsStore = settingsStore
	if settingsStore == nil {
		return nil
	}
	snapshot, err := settingsStore.LoadSettingsSnapshot(context.Background())
	if err != nil {
		return err
	}
	snapshot = normalizeSettingsPatch(snapshot)
	if len(snapshot) == 0 {
		return nil
	}
	mergeMaps(e.settings, snapshot)
	return nil
}

// WithSessionStore attaches first-class session persistence used to keep the
// `session -> task -> run` mapping durable alongside task snapshot updates.
func (e *Engine) WithSessionStore(sessionStore storage.SessionStore) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessionStore = sessionStore
	for _, taskID := range e.taskOrder {
		record := e.tasks[taskID]
		e.persistSessionLocked(record)
	}
	return nil
}

func newEngine(taskStore storage.TaskRunStore) (*Engine, error) {
	engine := &Engine{
		now:           time.Now,
		taskStore:     taskStore,
		tasks:         map[string]*TaskRecord{},
		taskOrder:     []string{},
		sessionOrder:  []string{},
		notepadClaims: map[string]struct{}{},
		inspector: InspectorConfig{
			TaskSources:          []string{defaultSettingsTaskSourcePath()},
			InspectionInterval:   map[string]any{"unit": "minute", "value": 15},
			InspectOnFileChange:  true,
			InspectOnStartup:     true,
			RemindBeforeDeadline: true,
			RemindWhenStale:      false,
		},
		settings:     buildDefaultSettings(),
		notepadItems: buildDefaultNotepadItems(time.Now()),
	}

	if err := engine.loadPersistedTaskRuns(context.Background()); err != nil {
		return nil, err
	}

	return engine, nil
}

// CurrentState returns the compatibility-layer run_status for the current lead
// task.
func (e *Engine) CurrentState() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.taskOrder) == 0 {
		return "processing"
	}

	return e.tasks[e.taskOrder[0]].runStatus()
}

// CurrentTaskStatus returns the task_status of the current lead task.
func (e *Engine) CurrentTaskStatus() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.taskOrder) == 0 {
		return "confirming_intent"
	}

	return e.tasks[e.taskOrder[0]].Status
}

// CreateTask creates the task/run mapping and seeds initial timeline,
// presentation, and security state.
func (e *Engine) CreateTask(input CreateTaskInput) TaskRecord {
	e.mu.Lock()
	defer e.mu.Unlock()

	createdAt := e.now()
	taskID := e.nextIdentifier("task")
	runID := e.nextIdentifier("run")
	stepTimeline := cloneTimeline(input.Timeline)
	for index := range stepTimeline {
		if stepTimeline[index].StepID == "" {
			stepTimeline[index].StepID = e.nextIdentifier("step")
		}
		stepTimeline[index].TaskID = taskID
	}

	record := &TaskRecord{
		TaskID:            taskID,
		SessionID:         firstNonEmpty(input.SessionID, e.nextIdentifier("sess")),
		RunID:             runID,
		PrimaryRunID:      runID,
		RequestSource:     strings.TrimSpace(input.RequestSource),
		RequestTrigger:    strings.TrimSpace(input.RequestTrigger),
		ExecutionAttempt:  1,
		Title:             input.Title,
		SourceType:        input.SourceType,
		Status:            input.Status,
		Intent:            cloneMap(input.Intent),
		PreferredDelivery: input.PreferredDelivery,
		FallbackDelivery:  input.FallbackDelivery,
		CurrentStep:       input.CurrentStep,
		RiskLevel:         input.RiskLevel,
		StartedAt:         createdAt,
		UpdatedAt:         createdAt,
		Timeline:          stepTimeline,
		BubbleMessage:     cloneMap(input.BubbleMessage),
		DeliveryResult:    cloneMap(input.DeliveryResult),
		Artifacts:         cloneMapSlice(input.Artifacts),
		Citations:         cloneMapSlice(input.Citations),
		MirrorReferences:  cloneMapSlice(input.MirrorReferences),
		Snapshot:          cloneContextSnapshot(input.Snapshot),
		SecuritySummary:   buildSecuritySummary(input.RiskLevel, nil),
		CurrentStepStatus: currentTimelineStatus(stepTimeline),
	}

	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))

	e.tasks[taskID] = record
	e.taskOrder = append([]string{taskID}, e.taskOrder...)
	e.trackSessionLocked(record.SessionID)
	e.persistTaskLocked(record)

	return record.clone()
}

// DeleteTask removes a task from runtime state and the backing task store.
// It is used for compensating rollback paths where task creation succeeded but
// the surrounding workflow failed before the task became a valid external
// object.
func (e *Engine) DeleteTask(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return ErrTaskNotFound
	}

	record, ok := e.tasks[taskID]
	if !ok || record == nil {
		return ErrTaskNotFound
	}
	if e.taskStore != nil {
		if err := e.taskStore.DeleteTaskRun(context.Background(), taskID); err != nil {
			return fmt.Errorf("delete task run %s: %w", taskID, err)
		}
	}

	delete(e.tasks, taskID)
	e.taskOrder = removeStringValue(e.taskOrder, taskID)
	e.untrackSessionLocked(record.SessionID)
	e.persistSessionByIDLocked(record.SessionID)
	return nil
}

// GetTask returns a defensive copy of the task snapshot for the given task_id.
func (e *Engine) GetTask(taskID string) (TaskRecord, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	return record.clone(), true
}

// ActiveSessionTask returns the current task that is holding execution for the
// given session. Only runtime-active states participate in the session queue.
func (e *Engine) ActiveSessionTask(sessionID, excludeTaskID string) (TaskRecord, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return TaskRecord{}, false
	}

	for _, taskID := range e.taskOrder {
		if taskID == excludeTaskID {
			continue
		}
		record := e.tasks[taskID]
		if record == nil || record.SessionID != sessionID {
			continue
		}
		if isSessionBusyTask(record) {
			return record.clone(), true
		}
	}

	return TaskRecord{}, false
}

// HydrateTaskFromStorage reloads a persisted task snapshot into runtime memory
// so governance and query flows can survive restarts.
func (e *Engine) HydrateTaskFromStorage(record TaskRecord) TaskRecord {
	e.mu.Lock()
	defer e.mu.Unlock()
	cloned := record.clone()
	if existing, ok := e.tasks[cloned.TaskID]; ok {
		*existing = cloned
		e.persistTaskLocked(existing)
		return existing.clone()
	}
	stored := cloned
	e.tasks[stored.TaskID] = &stored
	e.taskOrder = append([]string{stored.TaskID}, e.taskOrder...)
	if stored.SessionID != "" {
		seen := false
		for _, sessionID := range e.sessionOrder {
			if sessionID == stored.SessionID {
				seen = true
				break
			}
		}
		if !seen {
			e.sessionOrder = append(e.sessionOrder, stored.SessionID)
		}
	}
	e.persistTaskLocked(&stored)
	return stored.clone()
}

// ListTasks returns unfinished or finished tasks with the shared runtime sort
// order applied before paging.
func (e *Engine) ListTasks(group, sortBy, sortOrder string, limit, offset int) ([]TaskRecord, int) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	filtered := make([]TaskRecord, 0, len(e.taskOrder))
	for _, taskID := range e.taskOrder {
		record := e.tasks[taskID]
		if group == "finished" {
			if !record.isFinished() {
				continue
			}
		} else if record.isFinished() {
			continue
		}
		filtered = append(filtered, record.clone())
	}
	sortTaskRecords(filtered, sortBy, sortOrder)

	total := len(filtered)
	if offset >= total {
		return []TaskRecord{}, total
	}

	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}

	return filtered[offset:end], total
}

// ConfirmTask advances a task from confirming_intent to processing and updates
// the task-facing title, intent, bubble, and timeline state.
func (e *Engine) ConfirmTask(taskID, title string, intent map[string]any, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.Title = firstNonEmpty(title, record.Title)
	record.Intent = cloneMap(intent)
	record.Status = "processing"
	record.CurrentStep = "generate_output"
	record.UpdatedAt = e.now()
	record.BubbleMessage = cloneMap(bubbleMessage)
	record.Timeline = advanceTimeline(record.Timeline, "generate_output", "running", "生成输出开始")
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	e.persistTaskLocked(record)

	return record.clone(), true
}

// BeginExecution moves a task from suggestion or approval states into the real
// execution step. It must advance timeline state and emit task.updated together
// so task-centric readers and notification consumers see the same step/status
// transition without having to query compatibility-layer events separately.
func (e *Engine) BeginExecution(taskID, stepName, outputSummary string) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.Status = "processing"
	record.CurrentStep = firstNonEmpty(stepName, "generate_output")
	record.UpdatedAt = e.now()
	record.Timeline = advanceTimeline(record.Timeline, record.CurrentStep, "running", outputSummary)
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	e.persistTaskLocked(record)

	return record.clone(), true
}

// BeginPreparedExecution atomically replaces the live runtime record with a
// prepared restart attempt and then starts execution. This keeps restart
// visibility aligned with the first real processing state instead of exposing a
// transient preflight snapshot to concurrent readers.
func (e *Engine) BeginPreparedExecution(task TaskRecord, stepName, outputSummary string) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.commitPreparedTaskLocked(task)
	if !ok {
		return TaskRecord{}, false
	}

	record.Status = "processing"
	record.CurrentStep = firstNonEmpty(stepName, "generate_output")
	record.UpdatedAt = e.now()
	record.Timeline = advanceTimeline(record.Timeline, record.CurrentStep, "running", outputSummary)
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	e.persistTaskLocked(record)

	return record.clone(), true
}

// UpdateIntent replaces the effective intent and title without changing task
// identity.
func (e *Engine) UpdateIntent(taskID, title string, intent map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.Title = firstNonEmpty(title, record.Title)
	record.Intent = cloneMap(intent)
	record.UpdatedAt = e.now()
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	e.persistTaskLocked(record)

	return record.clone(), true
}

// ReopenIntentConfirmation moves a reviewed task back into confirming_intent
// after human intervention. The reset must clear delivery, approval, memory,
// and persistence plans so stale post-confirmation state cannot leak into the
// next plan the user is about to confirm.
func (e *Engine) ReopenIntentConfirmation(taskID, title string, intent map[string]any, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.Title = firstNonEmpty(title, record.Title)
	record.Intent = cloneMap(intent)
	record.Status = "confirming_intent"
	record.CurrentStep = "confirming_intent"
	record.UpdatedAt = e.now()
	record.FinishedAt = nil
	record.DeliveryResult = nil
	record.Artifacts = nil
	record.BubbleMessage = cloneMap(bubbleMessage)
	record.PendingExecution = nil
	record.ApprovalRequest = nil
	record.Authorization = nil
	record.ImpactScope = nil
	record.StorageWritePlan = nil
	record.ArtifactPlans = nil
	record.MemoryReadPlans = nil
	record.MemoryWritePlans = nil
	record.MirrorReferences = nil
	record.SecuritySummary = mergePreservedSecuritySummary(
		buildSecuritySummary(record.RiskLevel, latestRestorePointFromSummary(record.SecuritySummary)),
		record.SecuritySummary,
	)
	record.Timeline = advanceTimeline(record.Timeline, "confirming_intent", "pending", "等待人工复核后的新方案确认")
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	e.persistTaskLocked(record)

	return record.clone(), true
}

// ReopenWaitingInput keeps a task open when execution determined that more user
// input is required. The reset clears completed delivery state so follow-up
// submissions continue the same task instead of creating a fake finished result.
func (e *Engine) ReopenWaitingInput(taskID, title string, intent map[string]any, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.Title = firstNonEmpty(title, record.Title)
	record.Intent = cloneMap(intent)
	record.Status = "waiting_input"
	record.CurrentStep = "collect_input"
	record.UpdatedAt = e.now()
	record.FinishedAt = nil
	record.DeliveryResult = nil
	record.Artifacts = nil
	record.BubbleMessage = cloneMap(bubbleMessage)
	record.PendingExecution = nil
	record.ApprovalRequest = nil
	record.Authorization = nil
	record.ImpactScope = nil
	record.StorageWritePlan = nil
	record.ArtifactPlans = nil
	record.MemoryReadPlans = nil
	record.MemoryWritePlans = nil
	record.MirrorReferences = nil
	record.SecuritySummary = mergePreservedSecuritySummary(
		buildSecuritySummary(record.RiskLevel, latestRestorePointFromSummary(record.SecuritySummary)),
		record.SecuritySummary,
	)
	record.Timeline = advanceTimeline(record.Timeline, "collect_input", "pending", "等待用户补充输入")
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	e.persistTaskLocked(record)

	return record.clone(), true
}

// SetPresentation updates task-facing presentation fields without changing the
// state-machine conclusion.
func (e *Engine) SetPresentation(taskID string, bubbleMessage map[string]any, deliveryResult map[string]any, artifacts []map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.UpdatedAt = e.now()
	if bubbleMessage != nil {
		record.BubbleMessage = cloneMap(bubbleMessage)
	}
	if deliveryResult != nil {
		record.DeliveryResult = cloneMap(deliveryResult)
	}
	if artifacts != nil {
		record.Artifacts = cloneMapSlice(artifacts)
	}
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	if deliveryResult != nil {
		record.queueNotification("delivery.ready", map[string]any{
			"task_id":         record.TaskID,
			"delivery_result": cloneMap(record.DeliveryResult),
		})
	}
	e.persistTaskLocked(record)

	return record.clone(), true
}

// SetCitations stores protocol-facing citation objects separately from delivery
// and artifacts so task detail can expose evidence chains without overloading
// either delivery_result or artifact snapshots.
func (e *Engine) SetCitations(taskID string, citations []map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.UpdatedAt = e.now()
	record.Citations = cloneMapSlice(citations)
	e.persistTaskLocked(record)

	return record.clone(), true
}

// RecordToolCall records the latest completed compatibility-layer tool_call for
// the task.
func (e *Engine) RecordToolCall(taskID, toolName string, input, output map[string]any, durationMS int64) (TaskRecord, bool) {
	return e.RecordToolCallLifecycle(taskID, toolName, "succeeded", input, output, durationMS, nil)
}

// RecordToolCallLifecycle captures the latest compatibility-layer tool_call and
// emits the paired task/tool notifications together. Keeping both writes in one
// locked transition avoids a split view where task.updated lands without the
// corresponding tool_call.completed payload, or vice versa.
func (e *Engine) RecordToolCallLifecycle(taskID, toolName, status string, input, output map[string]any, durationMS int64, errorCode any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.UpdatedAt = e.now()
	record.LatestToolCall = e.buildToolCallRecord(record, toolName, status, input, output, durationMS, errorCode)
	record.LatestEvent = e.buildEventWithPayload(record, "tool_call.completed", toolCallEventPayload(record, toolName, status, input, output, errorCode))
	record.queueNotification("tool_call.completed", map[string]any{
		"task_id":     record.TaskID,
		"tool_call":   cloneMap(record.LatestToolCall),
		"event":       cloneMap(record.LatestEvent),
		"tool_name":   toolName,
		"tool_status": firstNonEmpty(status, "succeeded"),
	})
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	e.persistTaskLocked(record)

	return record.clone(), true
}

// RecordLoopLifecycle records one structured agent loop lifecycle event so
// task-centric consumers can inspect round transitions without querying the
// normalized compatibility tables directly.
func (e *Engine) RecordLoopLifecycle(taskID, eventType, stopReason string, payload map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.UpdatedAt = e.now()
	record.LoopStopReason = firstNonEmpty(stopReason, record.LoopStopReason)
	record.LatestEvent = e.buildEventWithPayload(record, eventType, payload)
	record.queueNotification(eventType, map[string]any{
		"task_id":     record.TaskID,
		"event":       cloneMap(record.LatestEvent),
		"stop_reason": firstNonEmpty(stopReason, record.LoopStopReason),
	})
	params := taskUpdatedNotificationParams(record)
	params["stop_reason"] = firstNonEmpty(stopReason, record.LoopStopReason)
	record.queueNotification("task.updated", params)
	e.persistTaskLocked(record)

	return record.clone(), true
}

// EmitRuntimeNotification appends a formal runtime notification for task-level
// consumers while also keeping LatestEvent in sync for query surfaces. The
// runtime path stores stop reasons here so later task queries can still explain
// why an agent loop stopped even after transports have drained the notification.
func (e *Engine) EmitRuntimeNotification(taskID, method string, payload map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}
	record.UpdatedAt = e.now()
	record.LoopStopReason = firstNonEmpty(runtimeStopReasonFromPayload(payload), record.LoopStopReason)
	record.LatestEvent = e.buildEventWithPayload(record, method, payload)
	record.queueNotification(method, map[string]any{
		"task_id":     taskID,
		"event":       cloneMap(record.LatestEvent),
		"stop_reason": firstNonEmpty(runtimeStopReasonFromPayload(payload), record.LoopStopReason),
	})
	e.persistTaskLocked(record)
	return record.clone(), true
}

// AppendSteeringMessage stores one follow-up instruction for a non-terminal task
// so future execution or resume paths can fold it into the loop planner input.
func (e *Engine) AppendSteeringMessage(taskID, message string, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok || record.isFinished() {
		return TaskRecord{}, false
	}
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return TaskRecord{}, false
	}
	record.UpdatedAt = e.now()
	record.SteeringMessages = append(record.SteeringMessages, trimmed)
	record.BubbleMessage = cloneMap(bubbleMessage)
	record.LatestEvent = e.buildEventWithPayload(record, "task.steered", map[string]any{
		"status":  record.Status,
		"message": trimmed,
	})
	record.queueNotification("task.steered", map[string]any{
		"task_id": record.TaskID,
		"message": trimmed,
	})
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	e.persistTaskLocked(record)
	return record.clone(), true
}

// ContinueTask merges a later desktop input into an existing non-terminal task.
// It preserves task identity while allowing orchestrator to refresh snapshot,
// title, intent, and optional steering data before execution resumes.
func (e *Engine) ContinueTask(taskID string, update ContinuationUpdate) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok || record.isFinished() {
		return TaskRecord{}, false
	}

	record.UpdatedAt = e.now()
	record.Snapshot = mergeTaskSnapshot(record.Snapshot, update.Snapshot)
	record.BubbleMessage = cloneMap(update.BubbleMessage)

	if strings.TrimSpace(update.Title) != "" {
		record.Title = strings.TrimSpace(update.Title)
	}
	if len(update.Intent) > 0 {
		record.Intent = cloneMap(update.Intent)
	}
	if strings.TrimSpace(update.Status) != "" {
		record.Status = strings.TrimSpace(update.Status)
	}
	if strings.TrimSpace(update.CurrentStep) != "" {
		record.CurrentStep = strings.TrimSpace(update.CurrentStep)
	}
	trimmedSteering := strings.TrimSpace(update.SteeringMessage)
	if nextStep := strings.TrimSpace(update.CurrentStep); nextStep != "" {
		record.Timeline = advanceTimeline(record.Timeline, nextStep, timelineStatusForTaskStatus(record.Status), continuationOutputSummary(update.BubbleMessage, trimmedSteering))
		record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	}

	if trimmedSteering != "" {
		record.SteeringMessages = append(record.SteeringMessages, trimmedSteering)
		record.LatestEvent = e.buildEventWithPayload(record, "task.steered", map[string]any{
			"status":  record.Status,
			"message": trimmedSteering,
		})
		record.queueNotification("task.steered", map[string]any{
			"task_id": record.TaskID,
			"message": trimmedSteering,
		})
	} else {
		record.LatestEvent = e.buildEvent(record, "task.updated")
	}

	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	e.persistTaskLocked(record)
	return record.clone(), true
}

// DrainSteeringMessages returns and clears queued steering messages for the
// active task so an in-flight loop can absorb new follow-up guidance.
func (e *Engine) DrainSteeringMessages(taskID string) ([]string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return nil, false
	}
	if len(record.SteeringMessages) == 0 {
		return nil, true
	}
	messages := append([]string(nil), record.SteeringMessages...)
	record.SteeringMessages = nil
	e.persistTaskLocked(record)
	return messages, true
}

// FailTaskExecution collapses a task into failed for execution failures and
// recovery-point preparation failures. It clears pending execution state and
// seals FinishedAt because callers treat failed as terminal and may immediately
// persist audit, recovery, or delivery fallback records against that outcome.
func (e *Engine) FailTaskExecution(taskID, stepName, securityStatus, outputSummary string, impactScope map[string]any, bubbleMessage map[string]any, latestRestorePoint ...map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	now := e.now()
	record.Status = "failed"
	record.CurrentStep = firstNonEmpty(stepName, "execution_failed")
	record.UpdatedAt = now
	record.FinishedAt = &now
	record.PendingExecution = nil
	record.BubbleMessage = cloneMap(bubbleMessage)
	record.ImpactScope = cloneMap(impactScope)
	restorePoint := latestRestorePointFromSummary(record.SecuritySummary)
	if len(latestRestorePoint) > 0 && len(latestRestorePoint[0]) > 0 {
		restorePoint = cloneMap(latestRestorePoint[0])
	}
	record.SecuritySummary = mergePreservedSecuritySummary(map[string]any{
		"security_status":        firstNonEmpty(securityStatus, "execution_error"),
		"risk_level":             record.RiskLevel,
		"pending_authorizations": 0,
		"latest_restore_point":   restorePoint,
	}, record.SecuritySummary)
	record.Timeline = advanceTimeline(record.Timeline, record.CurrentStep, "failed", firstNonEmpty(outputSummary, "执行失败"))
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	e.persistTaskLocked(record)

	return record.clone(), true
}

// BlockTaskByPolicy collapses a policy-intercepted task into cancelled. This is
// intentionally terminal because a denied governance decision is not a pause:
// downstream code must see the task as ended unless a new user action creates a
// different approval path.
func (e *Engine) BlockTaskByPolicy(taskID, riskLevel, outputSummary string, impactScope map[string]any, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	now := e.now()
	record.Status = "cancelled"
	record.CurrentStep = "risk_blocked"
	record.UpdatedAt = now
	record.FinishedAt = &now
	record.PendingExecution = nil
	record.ApprovalRequest = nil
	record.BubbleMessage = cloneMap(bubbleMessage)
	record.ImpactScope = cloneMap(impactScope)
	if riskLevel != "" {
		record.RiskLevel = riskLevel
	}
	record.SecuritySummary = mergePreservedSecuritySummary(map[string]any{
		"security_status":        "intercepted",
		"risk_level":             record.RiskLevel,
		"pending_authorizations": 0,
		"latest_restore_point":   latestRestorePointFromSummary(record.SecuritySummary),
	}, record.SecuritySummary)
	record.Timeline = advanceTimeline(record.Timeline, "risk_blocked", "cancelled", firstNonEmpty(outputSummary, "高风险操作已被策略拦截"))
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	e.persistTaskLocked(record)

	return record.clone(), true
}

// CompleteTask collapses a task into completed and records its formal delivery,
// artifacts, and recovery-point summary. The completion write also emits
// delivery.ready so transports and dashboard queries observe the same formal
// delivery boundary instead of inferring completion from task status alone.
func (e *Engine) CompleteTask(taskID string, deliveryResult map[string]any, bubbleMessage map[string]any, artifacts []map[string]any, latestRestorePoint ...map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	now := e.now()
	record.Status = "completed"
	record.CurrentStep = "return_result"
	record.UpdatedAt = now
	record.FinishedAt = &now
	record.DeliveryResult = cloneMap(deliveryResult)
	record.BubbleMessage = cloneMap(bubbleMessage)
	record.Artifacts = cloneMapSlice(artifacts)
	record.PendingExecution = nil
	record.ApprovalRequest = nil
	record.Authorization = nil
	record.Timeline = advanceTimeline(record.Timeline, "return_result", "completed", "结果已正式交付")
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	restorePoint := buildRecoveryPoint(record.TaskID, now)
	if len(latestRestorePoint) > 0 && len(latestRestorePoint[0]) > 0 {
		restorePoint = cloneMap(latestRestorePoint[0])
	}
	record.SecuritySummary = mergePreservedSecuritySummary(
		buildSecuritySummary(record.RiskLevel, restorePoint),
		record.SecuritySummary,
	)
	record.LatestEvent = e.buildEvent(record, "delivery.ready")
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	record.queueNotification("delivery.ready", map[string]any{
		"task_id":         record.TaskID,
		"delivery_result": cloneMap(record.DeliveryResult),
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// ApplyRecoveryOutcome finalizes restore-point execution and rewrites the task
// security summary around the outcome. It clears pending approval state because
// a restore decision is terminal for that authorization path, then emits one
// recovery-specific event so audit and task views can distinguish success from
// execution failure.
func (e *Engine) ApplyRecoveryOutcome(taskID, taskStatus, securityStatus string, recoveryPoint map[string]any, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.UpdatedAt = e.now()
	if strings.TrimSpace(taskStatus) != "" {
		record.Status = taskStatus
		if taskStatus == "completed" || taskStatus == "failed" || taskStatus == "cancelled" {
			now := e.now()
			record.FinishedAt = &now
		} else {
			record.FinishedAt = nil
		}
	}
	record.BubbleMessage = cloneMap(bubbleMessage)
	record.PendingExecution = nil
	record.ApprovalRequest = nil
	record.SecuritySummary = mergePreservedSecuritySummary(map[string]any{
		"security_status":        firstNonEmpty(securityStatus, "recovered"),
		"risk_level":             record.RiskLevel,
		"pending_authorizations": 0,
		"latest_restore_point":   cloneMap(recoveryPoint),
	}, record.SecuritySummary)
	eventType := "recovery.failed"
	if securityStatus == "recovered" {
		eventType = "recovery.applied"
	}
	record.LatestEvent = e.buildEventWithPayload(record, eventType, map[string]any{
		"status":            record.Status,
		"security_status":   firstNonEmpty(securityStatus, "recovered"),
		"recovery_point_id": stringValue(cloneMap(recoveryPoint), "recovery_point_id", ""),
	})
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	e.persistTaskLocked(record)

	return record.clone(), true
}

// ControlTask applies user-driven runtime controls while preserving the task
// to run mapping. Each branch enforces state guards so invalid resumes or
// restarts fail early instead of silently mutating timeline, approval, or
// delivery state into a combination the orchestrator cannot reason about.
func (e *Engine) ControlTask(taskID, action string, bubbleMessage map[string]any) (TaskRecord, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, ErrTaskNotFound
	}

	now := e.now()
	wasHumanLoop := record.isBlockedHumanLoop()
	switch action {
	case "pause":
		if record.isFinished() {
			return TaskRecord{}, ErrTaskAlreadyFinished
		}
		if record.Status != "processing" {
			return TaskRecord{}, ErrTaskStatusInvalid
		}
		record.Status = "paused"
	case "resume":
		if record.isFinished() {
			return TaskRecord{}, ErrTaskAlreadyFinished
		}
		if record.Status != "paused" && !record.isBlockedHumanLoop() {
			return TaskRecord{}, ErrTaskStatusInvalid
		}
		record.Status = "processing"
		record.CurrentStep = firstNonEmpty(resumeStepForTask(record), record.CurrentStep)
		if !wasHumanLoop {
			record.PendingExecution = nil
		}
	case "cancel":
		if record.isFinished() {
			return TaskRecord{}, ErrTaskAlreadyFinished
		}
		record.Status = "cancelled"
		record.FinishedAt = &now
		record.ApprovalRequest = nil
		record.PendingExecution = nil
		record.SecuritySummary = mergePreservedSecuritySummary(
			buildSecuritySummary(record.RiskLevel, latestRestorePointFromSummary(record.SecuritySummary)),
			record.SecuritySummary,
		)
		record.Timeline = advanceTimeline(record.Timeline, "task_cancelled", "cancelled", "任务已取消")
		record.CurrentStep = "task_cancelled"
	case "restart":
		if !record.isFinished() {
			return TaskRecord{}, ErrTaskStatusInvalid
		}
		// The generic engine path still reopens restart directly into processing.
		// The orchestrator uses PrepareRestart to stage the fresh run in memory
		// until queue/governance preflight decides the first persisted state.
		e.prepareRestartRecordLocked(record, now, bubbleMessage)
		record.Status = "processing"
		record.Timeline = advanceTimeline(record.Timeline, "generate_output", "running", "任务已重新开始")
	default:
		return TaskRecord{}, ErrTaskStatusInvalid
	}

	record.UpdatedAt = now
	record.BubbleMessage = cloneMap(bubbleMessage)
	if action == "resume" && wasHumanLoop {
		record.Timeline = advanceTimeline(record.Timeline, record.CurrentStep, "running", "人工介入后恢复执行")
	} else if action == "resume" {
		record.Timeline = advanceTimeline(record.Timeline, record.CurrentStep, "running", "任务已恢复执行")
	}
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	e.persistTaskLocked(record)

	return record.clone(), nil
}

// PrepareRestart builds a fresh restart attempt snapshot without mutating the
// live runtime record yet. The orchestrator runs session queue and governance
// preflight against this copy first so readers never observe a half-prepared
// run_id before the first durable restart state is known.
func (e *Engine) PrepareRestart(taskID string, bubbleMessage map[string]any) (TaskRecord, TaskRecord, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, TaskRecord{}, ErrTaskNotFound
	}
	if !record.isFinished() {
		return TaskRecord{}, TaskRecord{}, ErrTaskStatusInvalid
	}

	previous := record.clone()
	prepared := previous.clone()
	e.prepareRestartRecordLocked(&prepared, e.now(), bubbleMessage)
	return previous, prepared, nil
}

func (e *Engine) prepareRestartRecordLocked(record *TaskRecord, now time.Time, bubbleMessage map[string]any) {
	// Restart begins a fresh execution attempt for the same task, so it must
	// allocate a new run identifier before any loop/runtime rows are emitted.
	record.RunID = e.nextIdentifier("run")
	record.ExecutionAttempt++
	record.FinishedAt = nil
	record.CurrentStep = "generate_output"
	record.UpdatedAt = now
	record.DeliveryResult = nil
	record.Artifacts = nil
	record.Citations = nil
	record.AuditRecords = nil
	record.BubbleMessage = cloneMap(bubbleMessage)
	record.ApprovalRequest = nil
	record.PendingExecution = nil
	record.Authorization = nil
	record.ImpactScope = nil
	record.StorageWritePlan = nil
	record.ArtifactPlans = nil
	record.MemoryReadPlans = nil
	record.MemoryWritePlans = nil
	record.MirrorReferences = nil
	record.LatestToolCall = nil
	record.LoopStopReason = ""
	record.SecuritySummary = mergePreservedSecuritySummary(
		buildSecuritySummary(record.RiskLevel, latestRestorePointFromSummary(record.SecuritySummary)),
		record.SecuritySummary,
	)
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = nil
}

func (e *Engine) commitPreparedTaskLocked(task TaskRecord) (*TaskRecord, bool) {
	record, ok := e.tasks[task.TaskID]
	if !ok {
		return nil, false
	}
	prepared := task.clone()
	*record = prepared
	return record, true
}

// MarkWaitingApproval is the shorthand entrypoint for moving a task into the
// waiting_auth state.
func (e *Engine) MarkWaitingApproval(taskID string, approvalRequest map[string]any, bubbleMessage map[string]any) (TaskRecord, bool) {
	return e.MarkWaitingApprovalWithPlan(taskID, approvalRequest, nil, bubbleMessage)
}

// MarkWaitingApprovalWithPlan moves a task into waiting_auth and stores the
// execution plan required to resume after approval. The buffered plan must be
// saved before approval.pending is emitted because transports and later resume
// requests rely on this persisted shape instead of reconstructing intent from a
// potentially stale caller payload.
func (e *Engine) MarkWaitingApprovalWithPlan(taskID string, approvalRequest map[string]any, pendingExecution map[string]any, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	now := e.now()
	record.Status = "waiting_auth"
	record.CurrentStep = "waiting_authorization"
	record.UpdatedAt = now
	record.ApprovalRequest = cloneMap(approvalRequest)
	record.PendingExecution = cloneMap(pendingExecution)
	if riskLevel, ok := approvalRequest["risk_level"].(string); ok && riskLevel != "" {
		record.RiskLevel = riskLevel
	}
	record.BubbleMessage = cloneMap(bubbleMessage)
	latestRestorePoint := latestRestorePointFromSummary(record.SecuritySummary)
	record.SecuritySummary = mergePreservedSecuritySummary(map[string]any{
		"security_status":        "pending_confirmation",
		"risk_level":             record.RiskLevel,
		"pending_authorizations": 1,
		"latest_restore_point":   latestRestorePoint,
	}, record.SecuritySummary)
	record.Timeline = advanceTimeline(record.Timeline, "waiting_authorization", "running", "等待用户授权")
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "approval.pending")
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	record.queueNotification("approval.pending", map[string]any{
		"task_id":          record.TaskID,
		"approval_request": cloneMap(record.ApprovalRequest),
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// MarkPreparedTaskWaitingApprovalWithPlan atomically publishes a prepared
// restart attempt as waiting_auth after governance preflight decides that the
// new run must stop at authorization before any execution can begin.
func (e *Engine) MarkPreparedTaskWaitingApprovalWithPlan(task TaskRecord, approvalRequest map[string]any, pendingExecution map[string]any, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.commitPreparedTaskLocked(task)
	if !ok {
		return TaskRecord{}, false
	}

	now := e.now()
	record.Status = "waiting_auth"
	record.CurrentStep = "waiting_authorization"
	record.UpdatedAt = now
	record.ApprovalRequest = cloneMap(approvalRequest)
	record.PendingExecution = cloneMap(pendingExecution)
	if riskLevel, ok := approvalRequest["risk_level"].(string); ok && riskLevel != "" {
		record.RiskLevel = riskLevel
	}
	record.BubbleMessage = cloneMap(bubbleMessage)
	latestRestorePoint := latestRestorePointFromSummary(record.SecuritySummary)
	record.SecuritySummary = mergePreservedSecuritySummary(map[string]any{
		"security_status":        "pending_confirmation",
		"risk_level":             record.RiskLevel,
		"pending_authorizations": 1,
		"latest_restore_point":   latestRestorePoint,
	}, record.SecuritySummary)
	record.Timeline = advanceTimeline(record.Timeline, "waiting_authorization", "running", "等待用户授权")
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "approval.pending")
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	record.queueNotification("approval.pending", map[string]any{
		"task_id":          record.TaskID,
		"approval_request": cloneMap(record.ApprovalRequest),
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// ResolveAuthorization records the final authorization outcome and clears the
// pending approval state once the orchestrator has already finalized the task
// outcome. This separation lets allow-once flows keep authorization metadata
// while executing, then normalize the security summary after delivery is known.
func (e *Engine) ResolveAuthorization(taskID string, authorization map[string]any, impactScope map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.Authorization = cloneMap(authorization)
	record.ImpactScope = cloneMap(impactScope)
	record.ApprovalRequest = nil
	record.PendingExecution = nil
	latestRestorePoint := map[string]any(nil)
	if existingRestorePoint, ok := record.SecuritySummary["latest_restore_point"].(map[string]any); ok {
		latestRestorePoint = cloneMap(existingRestorePoint)
	}
	record.SecuritySummary = mergePreservedSecuritySummary(
		buildSecuritySummary(record.RiskLevel, latestRestorePoint),
		record.SecuritySummary,
	)
	e.persistTaskLocked(record)
	return record.clone(), true
}

// ResumeAfterApproval returns an approved task from waiting_auth to processing
// while preserving the follow-up execution plan. The plan is intentionally not
// cleared here because execution still needs it to finish delivery or recovery
// work after the user approves the risky action.
func (e *Engine) ResumeAfterApproval(taskID string, authorization map[string]any, impactScope map[string]any, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}
	if record.Status != "waiting_auth" || record.CurrentStep != "waiting_authorization" || len(record.ApprovalRequest) == 0 {
		return TaskRecord{}, false
	}

	now := e.now()
	record.Status = "processing"
	record.CurrentStep = "authorized_execution"
	record.UpdatedAt = now
	record.Authorization = cloneMap(authorization)
	record.ImpactScope = cloneMap(impactScope)
	record.ApprovalRequest = nil
	record.BubbleMessage = cloneMap(bubbleMessage)
	record.SecuritySummary = mergePreservedSecuritySummary(
		buildSecuritySummary(record.RiskLevel, latestRestorePointFromSummary(record.SecuritySummary)),
		record.SecuritySummary,
	)
	record.Timeline = advanceTimeline(record.Timeline, "authorized_execution", "running", "授权通过，继续执行")
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	e.persistTaskLocked(record)

	return record.clone(), true
}

// DenyAfterApproval terminates a task after the user rejects authorization and
// keeps the final authorization and impact summary attached.
func (e *Engine) DenyAfterApproval(taskID string, authorization map[string]any, impactScope map[string]any, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}
	if record.Status != "waiting_auth" || record.CurrentStep != "waiting_authorization" || len(record.ApprovalRequest) == 0 {
		return TaskRecord{}, false
	}

	now := e.now()
	record.Status = "cancelled"
	record.CurrentStep = "authorization_denied"
	record.UpdatedAt = now
	record.FinishedAt = &now
	record.Authorization = cloneMap(authorization)
	record.ImpactScope = cloneMap(impactScope)
	record.ApprovalRequest = nil
	record.PendingExecution = nil
	record.BubbleMessage = cloneMap(bubbleMessage)
	record.SecuritySummary = mergePreservedSecuritySummary(map[string]any{
		"security_status":        "intercepted",
		"risk_level":             record.RiskLevel,
		"pending_authorizations": 0,
		"latest_restore_point":   latestRestorePointFromSummary(record.SecuritySummary),
	}, record.SecuritySummary)
	record.Timeline = advanceTimeline(record.Timeline, "authorization_denied", "cancelled", "用户拒绝授权，任务已结束")
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	e.persistTaskLocked(record)

	return record.clone(), true
}

// PendingExecutionPlan returns the buffered resume plan for a waiting_auth
// task. Callers use this instead of rebuilding delivery intent so authorization
// resumes remain deterministic even if the original request context is gone.
func (e *Engine) PendingExecutionPlan(taskID string) (map[string]any, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	record, ok := e.tasks[taskID]
	if !ok || len(record.PendingExecution) == 0 {
		return nil, false
	}

	return cloneMap(record.PendingExecution), true
}

// QueueTaskForSession blocks a task behind another active task in the same
// session so the session-level agent loop remains serial. This avoids two tasks
// in one conversational lane racing to mutate shared context, notifications, or
// follow-up decisions at the same time.
func (e *Engine) QueueTaskForSession(taskID, blockingTaskID string, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.Status = "blocked"
	record.CurrentStep = "session_queue"
	record.UpdatedAt = e.now()
	record.BubbleMessage = cloneMap(bubbleMessage)
	record.Timeline = advanceTimeline(record.Timeline, "session_queue", "pending", "等待同一会话中的前序任务完成")
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEventWithPayload(record, "task.session_queued", map[string]any{
		"status":           record.Status,
		"blocking_task_id": blockingTaskID,
	})
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	record.queueNotification("task.session_queued", map[string]any{
		"task_id":          record.TaskID,
		"blocking_task_id": blockingTaskID,
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// QueuePreparedTaskForSession atomically publishes a prepared restart attempt
// as the queued runtime state so query readers never observe the intermediate
// restart snapshot before the session gate decides its first visible status.
func (e *Engine) QueuePreparedTaskForSession(task TaskRecord, blockingTaskID string, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.commitPreparedTaskLocked(task)
	if !ok {
		return TaskRecord{}, false
	}

	record.Status = "blocked"
	record.CurrentStep = "session_queue"
	record.UpdatedAt = e.now()
	record.BubbleMessage = cloneMap(bubbleMessage)
	record.Timeline = advanceTimeline(record.Timeline, "session_queue", "pending", "等待同一会话中的前序任务完成")
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEventWithPayload(record, "task.session_queued", map[string]any{
		"status":           record.Status,
		"blocking_task_id": blockingTaskID,
	})
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	record.queueNotification("task.session_queued", map[string]any{
		"task_id":          record.TaskID,
		"blocking_task_id": blockingTaskID,
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// EscalateHumanLoop blocks one task for structured human review while keeping
// the pending escalation payload available for later resume/cancel handling.
// The escalation payload stays in PendingExecution because resume decisions must
// know what review artifact or override data triggered the block.
func (e *Engine) EscalateHumanLoop(taskID string, escalation map[string]any, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.Status = "blocked"
	record.CurrentStep = "human_in_loop"
	record.PendingExecution = map[string]any{
		"kind":       "human_in_loop",
		"escalation": cloneMap(escalation),
	}
	record.UpdatedAt = e.now()
	record.BubbleMessage = cloneMap(bubbleMessage)
	record.Timeline = advanceTimeline(record.Timeline, "human_in_loop", "pending", "等待人工介入处理当前任务")
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEventWithPayload(record, "task.updated", map[string]any{
		"status":       record.Status,
		"current_step": record.CurrentStep,
	})
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	e.persistTaskLocked(record)

	return record.clone(), true
}

func (r TaskRecord) isBlockedHumanLoop() bool {
	if r.Status != "blocked" || r.CurrentStep != "human_in_loop" {
		return false
	}
	return stringValue(r.PendingExecution, "kind", "") == "human_in_loop"
}

func resumeStepForTask(record *TaskRecord) string {
	if record == nil {
		return "generate_output"
	}
	if currentStep := strings.TrimSpace(record.CurrentStep); currentStep != "" && currentStep != "human_in_loop" {
		// Pause/resume must preserve the step that was actually running. Intent
		// alone can overstate agent-loop capability after prompt fallback.
		return currentStep
	}
	if stringValue(record.Intent, "name", "") == "agent_loop" {
		return "agent_loop"
	}
	return "generate_output"
}

// NextQueuedTaskForSession returns the earliest queued task that is waiting for
// the same session lane to become available.
func (e *Engine) NextQueuedTaskForSession(sessionID string) (TaskRecord, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return TaskRecord{}, false
	}

	var selected *TaskRecord
	for _, taskID := range e.taskOrder {
		record := e.tasks[taskID]
		if record == nil || record.SessionID != sessionID {
			continue
		}
		if record.Status != "blocked" || record.CurrentStep != "session_queue" {
			continue
		}
		if selected == nil || record.StartedAt.Before(selected.StartedAt) {
			selected = record
		}
	}
	if selected == nil {
		return TaskRecord{}, false
	}
	return selected.clone(), true
}

// ResumeQueuedTask returns a queued session task to processing once the session
// lane becomes available again.
func (e *Engine) ResumeQueuedTask(taskID, stepName string, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.Status = "processing"
	record.CurrentStep = firstNonEmpty(stepName, "generate_output")
	record.UpdatedAt = e.now()
	record.BubbleMessage = cloneMap(bubbleMessage)
	record.Timeline = advanceTimeline(record.Timeline, record.CurrentStep, "running", "前序任务完成，当前会话任务开始执行")
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEventWithPayload(record, "task.session_resumed", map[string]any{
		"status": record.Status,
	})
	record.queueNotification("task.updated", taskUpdatedNotificationParams(record))
	record.queueNotification("task.session_resumed", map[string]any{
		"task_id": record.TaskID,
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// SetMemoryPlans stores memory read/write plans for later orchestration
// handoffs and observability.
func (e *Engine) SetMemoryPlans(taskID string, readPlans []map[string]any, writePlans []map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	if readPlans != nil {
		record.MemoryReadPlans = cloneMapSlice(readPlans)
	}
	if writePlans != nil {
		record.MemoryWritePlans = cloneMapSlice(writePlans)
	}
	e.persistTaskLocked(record)
	return record.clone(), true
}

// SetMirrorReferences stores the mirror-reference snapshot attached to the
// task.
func (e *Engine) SetMirrorReferences(taskID string, mirrorReferences []map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.MirrorReferences = cloneMapSlice(mirrorReferences)
	e.persistTaskLocked(record)
	return record.clone(), true
}

// SetDeliveryPlans stores the workspace write plan and artifact persistence
// plans.
func (e *Engine) SetDeliveryPlans(taskID string, storageWritePlan map[string]any, artifactPlans []map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.StorageWritePlan = cloneMap(storageWritePlan)
	record.ArtifactPlans = cloneMapSlice(artifactPlans)
	e.persistTaskLocked(record)
	return record.clone(), true
}

// PendingNotifications returns the notification snapshot that has not yet been
// consumed.
func (e *Engine) PendingNotifications(taskID string) ([]NotificationRecord, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return nil, false
	}

	return cloneNotifications(record.Notifications), true
}

// DrainNotifications returns and clears the buffered notification queue for a
// task.
func (e *Engine) DrainNotifications(taskID string) ([]NotificationRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return nil, false
	}

	notifications := cloneNotifications(record.Notifications)
	record.Notifications = nil
	e.persistTaskLocked(record)
	return notifications, true
}

// PendingApprovalRequests enumerates the currently pending approval requests.
func (e *Engine) PendingApprovalRequests(limit, offset int) ([]map[string]any, int) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	items := make([]map[string]any, 0)
	for _, taskID := range e.taskOrder {
		record := e.tasks[taskID]
		if len(record.ApprovalRequest) == 0 {
			continue
		}
		items = append(items, cloneMap(record.ApprovalRequest))
	}

	total := len(items)
	if offset >= total {
		return []map[string]any{}, total
	}

	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}

	return items[offset:end], total
}

// TaskDetail returns the full task snapshot used by the task detail view.
func (e *Engine) TaskDetail(taskID string) (TaskRecord, bool) {
	return e.GetTask(taskID)
}

// InspectorConfig returns the current effective task-inspector config.
func (e *Engine) InspectorConfig() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return map[string]any{
		"task_sources":           append([]string(nil), e.inspector.TaskSources...),
		"inspection_interval":    cloneMap(e.inspector.InspectionInterval),
		"inspect_on_file_change": e.inspector.InspectOnFileChange,
		"inspect_on_startup":     e.inspector.InspectOnStartup,
		"remind_before_deadline": e.inspector.RemindBeforeDeadline,
		"remind_when_stale":      e.inspector.RemindWhenStale,
	}
}

// UpdateInspectorConfig patches inspector settings and returns the full updated
// snapshot.
func (e *Engine) UpdateInspectorConfig(values map[string]any) map[string]any {
	e.mu.Lock()
	defer e.mu.Unlock()

	if sources := stringSlice(values["task_sources"]); len(sources) > 0 {
		e.inspector.TaskSources = sources
	}
	if interval, ok := values["inspection_interval"].(map[string]any); ok {
		e.inspector.InspectionInterval = cloneMap(interval)
	}
	if value, ok := values["inspect_on_file_change"].(bool); ok {
		e.inspector.InspectOnFileChange = value
	}
	if value, ok := values["inspect_on_startup"].(bool); ok {
		e.inspector.InspectOnStartup = value
	}
	if value, ok := values["remind_before_deadline"].(bool); ok {
		e.inspector.RemindBeforeDeadline = value
	}
	if value, ok := values["remind_when_stale"].(bool); ok {
		e.inspector.RemindWhenStale = value
	}

	return map[string]any{
		"task_sources":           append([]string(nil), e.inspector.TaskSources...),
		"inspection_interval":    cloneMap(e.inspector.InspectionInterval),
		"inspect_on_file_change": e.inspector.InspectOnFileChange,
		"inspect_on_startup":     e.inspector.InspectOnStartup,
		"remind_before_deadline": e.inspector.RemindBeforeDeadline,
		"remind_when_stale":      e.inspector.RemindWhenStale,
	}
}

// Settings returns the current in-memory settings snapshot.
func (e *Engine) Settings() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return normalizeSettingsPatch(cloneMap(e.settings))
}

// UpdateSettings merges a settings patch, persists the ordinary snapshot when a
// settings store is attached, and reports affected fields, apply mode, and
// restart requirements.
func (e *Engine) UpdateSettings(values map[string]any) (map[string]any, []string, string, bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	values = normalizeSettingsPatch(values)

	nextSettings := cloneMap(e.settings)
	if nextSettings == nil {
		nextSettings = buildDefaultSettings()
	}
	updatedKeys := make([]string, 0)
	effectiveSettings := map[string]any{}
	applyMode := "immediate"
	needRestart := false
	modelSettingsChanged := false

	for _, section := range []string{"general", "floating_ball", "memory", "task_automation", "models"} {
		sectionPatch, ok := values[section].(map[string]any)
		if !ok || len(sectionPatch) == 0 {
			continue
		}

		currentSection := cloneMap(mapValue(nextSettings, section))
		if currentSection == nil {
			currentSection = map[string]any{}
		}
		previousSection := cloneMap(currentSection)

		mergeMaps(currentSection, sectionPatch)
		nextSettings[section] = currentSection
		effectiveSettings[section] = cloneMap(sectionPatch)

		sectionUpdatedKeys := settingsPatchPaths(section, sectionPatch)
		updatedKeys = append(updatedKeys, sectionUpdatedKeys...)

		if section == "general" {
			if nextLanguage, ok := sectionPatch["language"]; ok {
				currentLanguage, hasCurrentLanguage := previousSection["language"]
				if !hasCurrentLanguage || !reflect.DeepEqual(currentLanguage, nextLanguage) {
					applyMode = "restart_required"
					needRestart = true
				}
			}
			if downloadPatch, ok := sectionPatch["download"].(map[string]any); ok {
				currentDownload := cloneMap(mapValue(previousSection, "download"))
				if nextWorkspacePath, ok := downloadPatch["workspace_path"]; ok {
					currentWorkspacePath, hasCurrentWorkspacePath := currentDownload["workspace_path"]
					if !hasCurrentWorkspacePath || !reflect.DeepEqual(currentWorkspacePath, nextWorkspacePath) {
						applyMode = "restart_required"
						needRestart = true
					}
				}
			}
		}
		if section == "models" && modelRouteSettingsChanged(sectionUpdatedKeys) {
			modelSettingsChanged = true
		}
	}
	if modelSettingsChanged && !needRestart {
		applyMode = "next_task_effective"
	}
	if e.settingsStore != nil {
		if err := e.settingsStore.SaveSettingsSnapshot(context.Background(), normalizeSettingsPatch(cloneMap(nextSettings))); err != nil {
			return nil, nil, "", false, err
		}
	}
	e.settings = nextSettings

	return effectiveSettings, updatedKeys, applyMode, needRestart, nil
}

// NotepadItems returns the current notepad bucket view using the frozen TodoItem
// contract, even when the internal owner-5 foundation carries richer metadata.
func (e *Engine) NotepadItems(group string, limit, offset int) ([]map[string]any, int) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	filtered := make([]map[string]any, 0, len(e.notepadItems))
	for _, item := range e.notepadItems {
		normalized := protocolNotepadItemMap(item, e.now())
		if group != "" {
			if bucket, ok := normalized["bucket"].(string); !ok || bucket != group {
				continue
			}
		}
		filtered = append(filtered, normalized)
	}
	sortNotepadItems(filtered)

	total := len(filtered)
	if offset >= total {
		return []map[string]any{}, total
	}

	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}

	return filtered[offset:end], total
}

func (e *Engine) NotepadItem(itemID string) (map[string]any, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	item, _, ok := e.findNotepadItem(itemID)
	if !ok {
		return nil, false
	}

	return normalizeNotepadItem(item, e.now()), true
}

func (e *Engine) ReplaceNotepadItems(items []map[string]any) {
	e.mu.Lock()
	defer e.mu.Unlock()

	_ = e.replaceNotepadItemsLocked(items)
}

func (e *Engine) CompleteNotepadItem(itemID string) (map[string]any, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	updated, index, ok := e.updatedNotepadItem(itemID)
	if !ok {
		return nil, false
	}

	closeNotepadItem(updated, "completed", e.now())
	items := cloneMapSlice(e.notepadItems)
	items[index] = updated
	if err := e.replaceNotepadItemsLocked(items); err != nil {
		return nil, false
	}
	return normalizeNotepadItem(updated, e.now()), true
}

func (e *Engine) ClaimNotepadItemTask(itemID string) (map[string]any, bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	item, _, ok := e.findNotepadItem(itemID)
	if !ok {
		return nil, false, nil
	}

	status := stringValue(item, "status", "normal")
	if status == "completed" || status == "cancelled" {
		return nil, true, fmt.Errorf("notepad item is already closed: %s", itemID)
	}

	linkedTaskID := stringValue(item, "linked_task_id", "")
	if linkedTaskID != "" {
		return nil, true, fmt.Errorf("notepad item is already linked to task: %s", linkedTaskID)
	}
	if _, claimed := e.notepadClaims[itemID]; claimed {
		return nil, true, fmt.Errorf("notepad item is already being converted: %s", itemID)
	}

	e.notepadClaims[itemID] = struct{}{}
	return normalizeNotepadItem(item, e.now()), true, nil
}

func (e *Engine) ReleaseNotepadItemClaim(itemID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.notepadClaims, strings.TrimSpace(itemID))
}

func (e *Engine) LinkNotepadItemTask(itemID, taskID string) (map[string]any, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	item, index, ok := e.findNotepadItem(itemID)
	if !ok {
		return nil, false
	}

	linkedTaskID := stringValue(item, "linked_task_id", "")
	if linkedTaskID != "" && linkedTaskID != strings.TrimSpace(taskID) {
		return nil, false
	}
	if _, claimed := e.notepadClaims[itemID]; !claimed {
		return nil, false
	}

	updated := cloneMap(item)
	updated["linked_task_id"] = strings.TrimSpace(taskID)
	items := cloneMapSlice(e.notepadItems)
	items[index] = updated
	if err := e.replaceNotepadItemsLocked(items); err != nil {
		return nil, false
	}
	delete(e.notepadClaims, strings.TrimSpace(itemID))
	return normalizeNotepadItem(updated, e.now()), true
}

func (e *Engine) UpdateNotepadItem(itemID, action string) (map[string]any, []string, string, bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	item, index, ok := e.findNotepadItem(itemID)
	if !ok {
		return nil, nil, "", false, nil
	}

	now := e.now()
	currentBucket := stringValue(item, "bucket", "upcoming")
	refreshGroups := []string{currentBucket}
	updated := cloneMap(item)

	switch strings.TrimSpace(action) {
	case "complete":
		if currentBucket == "closed" {
			return nil, nil, "", true, fmt.Errorf("notepad item is already closed: %s", itemID)
		}
		closeNotepadItem(updated, "completed", now)
		refreshGroups = append(refreshGroups, "closed")
	case "cancel":
		if currentBucket == "closed" {
			return nil, nil, "", true, fmt.Errorf("notepad item is already closed: %s", itemID)
		}
		closeNotepadItem(updated, "cancelled", now)
		refreshGroups = append(refreshGroups, "closed")
	case "move_upcoming":
		if currentBucket != "later" {
			return nil, nil, "", true, fmt.Errorf("notepad action move_upcoming requires later bucket: %s", itemID)
		}
		updated["bucket"] = "upcoming"
		updated["status"] = "normal"
		refreshGroups = append(refreshGroups, "upcoming")
	case "toggle_recurring":
		if currentBucket != "recurring_rule" {
			return nil, nil, "", true, fmt.Errorf("notepad action toggle_recurring requires recurring_rule bucket: %s", itemID)
		}
		currentEnabled := boolValue(updated["recurring_enabled"], true)
		nextEnabled := !currentEnabled
		updated["recurring_enabled"] = nextEnabled
		if nextEnabled {
			updated["status"] = "normal"
			updated["recent_instance_status"] = "重复规则已恢复"
		} else {
			updated["status"] = "cancelled"
			updated["recent_instance_status"] = "重复规则已暂停"
		}
	case "cancel_recurring":
		if currentBucket != "recurring_rule" {
			return nil, nil, "", true, fmt.Errorf("notepad action cancel_recurring requires recurring_rule bucket: %s", itemID)
		}
		updated["recurring_enabled"] = false
		closeNotepadItem(updated, "cancelled", now)
		refreshGroups = append(refreshGroups, "closed")
	case "restore":
		if currentBucket != "closed" {
			return nil, nil, "", true, fmt.Errorf("notepad action restore requires closed bucket: %s", itemID)
		}
		restoreNotepadItem(updated, now)
		refreshGroups = append(refreshGroups, stringValue(updated, "bucket", currentBucket))
	case "delete":
		if currentBucket != "closed" {
			return nil, nil, "", true, fmt.Errorf("notepad action delete requires closed bucket: %s", itemID)
		}
		items := cloneMapSlice(e.notepadItems)
		items = append(items[:index], items[index+1:]...)
		if err := e.replaceNotepadItemsLocked(items); err != nil {
			return nil, nil, "", true, err
		}
		return nil, dedupeStrings(refreshGroups), itemID, true, nil
	default:
		return nil, nil, "", true, fmt.Errorf("unsupported notepad action: %s", action)
	}

	items := cloneMapSlice(e.notepadItems)
	items[index] = updated
	if err := e.replaceNotepadItemsLocked(items); err != nil {
		return nil, nil, "", true, err
	}
	return normalizeNotepadItem(updated, now), dedupeStrings(refreshGroups), "", true, nil
}

func (e *Engine) AppendAuditData(taskID string, auditRecords []map[string]any, tokenUsage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	if len(auditRecords) > 0 {
		record.AuditRecords = append(record.AuditRecords, cloneMapSlice(auditRecords)...)
	}
	if len(tokenUsage) > 0 {
		record.TokenUsage = cloneMap(tokenUsage)
	}
	record.UpdatedAt = e.now()
	e.persistTaskLocked(record)
	return record.clone(), true
}

// UpdateSecuritySummary writes task-facing governance metadata back into the
// runtime record before later state transitions rebuild the base summary.
func (e *Engine) UpdateSecuritySummary(taskID string, securitySummary map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.UpdatedAt = e.now()
	record.SecuritySummary = cloneMap(securitySummary)
	e.persistTaskLocked(record)
	return record.clone(), true
}

// buildEvent creates one compatibility-layer event record for the task.
func (e *Engine) buildEvent(record *TaskRecord, eventType string) map[string]any {
	return e.buildEventWithPayload(record, eventType, map[string]any{"status": record.Status})
}

func (e *Engine) buildEventWithPayload(record *TaskRecord, eventType string, payload map[string]any) map[string]any {
	return map[string]any{
		"event_id":   e.nextIdentifier("evt"),
		"run_id":     record.RunID,
		"task_id":    record.TaskID,
		"step_id":    timelineCurrentStepID(record.Timeline),
		"type":       eventType,
		"level":      "info",
		"payload":    cloneMap(payload),
		"created_at": e.now().Format(time.RFC3339),
	}
}

// buildToolCall creates one compatibility-layer tool_call record for the task.
func (e *Engine) buildToolCall(record *TaskRecord, toolName string) map[string]any {
	return e.buildToolCallRecord(record, toolName, "succeeded", map[string]any{}, map[string]any{}, 120, nil)
}

func (e *Engine) buildToolCallRecord(record *TaskRecord, toolName, status string, input, output map[string]any, durationMS int64, errorCode any) map[string]any {
	if durationMS <= 0 {
		durationMS = 1
	}

	return map[string]any{
		"tool_call_id": e.nextIdentifier("tool"),
		"run_id":       record.RunID,
		"task_id":      record.TaskID,
		"step_id":      timelineCurrentStepID(record.Timeline),
		"tool_name":    toolName,
		"status":       firstNonEmpty(status, "succeeded"),
		"input":        cloneMap(input),
		"output":       cloneMap(output),
		"error_code":   errorCode,
		"duration_ms":  durationMS,
	}
}

func toolCallEventPayload(record *TaskRecord, toolName, status string, input, output map[string]any, errorCode any) map[string]any {
	payload := map[string]any{
		"status":      record.Status,
		"tool_name":   toolName,
		"tool_status": firstNonEmpty(status, "succeeded"),
	}
	if errorCode != nil {
		payload["error_code"] = errorCode
	}
	for _, key := range []string{"source", "execution_backend", "path", "url", "output_path", "output_dir", "actions_applied", "page_count", "frame_count"} {
		if value, ok := output[key]; ok {
			payload[key] = value
		}
	}
	for _, key := range []string{"path", "url", "output_path", "output_dir"} {
		if _, exists := payload[key]; exists {
			continue
		}
		if value, ok := input[key]; ok {
			payload[key] = value
		}
	}
	if summaryOutput, ok := output["summary_output"].(map[string]any); ok && len(summaryOutput) > 0 {
		payload["summary_output"] = cloneMap(summaryOutput)
	}
	return payload
}

// nextIdentifier allocates a prefixed identifier and prefers persistent
// storage-backed allocation when available. That keeps task/run/event/tool ids
// stable across process restarts instead of relying on an in-memory counter.
func (e *Engine) nextIdentifier(prefix string) string {
	if e.taskStore != nil {
		identifier, err := e.taskStore.AllocateIdentifier(context.Background(), prefix)
		if err == nil {
			return identifier
		}
	}

	e.nextID++
	return fmt.Sprintf("%s_%03d", prefix, e.nextID)
}

func (e *Engine) loadPersistedTaskRuns(ctx context.Context) error {
	if e.taskStore == nil {
		return nil
	}

	records, err := e.taskStore.LoadTaskRuns(ctx)
	if err != nil {
		return fmt.Errorf("load persisted task runs: %w", err)
	}

	if len(records) == 0 {
		return nil
	}

	seenSessions := make(map[string]struct{}, len(records))
	for _, persisted := range records {
		record := taskRecordFromStorage(persisted)
		e.tasks[record.TaskID] = &record
		e.taskOrder = append(e.taskOrder, record.TaskID)
		if _, seen := seenSessions[record.SessionID]; !seen {
			e.sessionOrder = append(e.sessionOrder, record.SessionID)
			seenSessions[record.SessionID] = struct{}{}
		}
	}

	return nil
}

func (e *Engine) persistTaskLocked(record *TaskRecord) {
	if record == nil {
		return
	}
	if e.taskStore != nil {
		_ = e.taskStore.SaveTaskRun(context.Background(), taskRecordToStorage(record.clone()))
	}
	e.persistSessionLocked(record)
}

func (e *Engine) persistSessionLocked(record *TaskRecord) {
	if record == nil {
		return
	}
	e.persistSessionByIDLocked(record.SessionID)
}

func (e *Engine) persistSessionByIDLocked(sessionID string) {
	if e.sessionStore == nil || strings.TrimSpace(sessionID) == "" {
		return
	}
	record, ok := e.sessionRecordLocked(sessionID)
	if !ok {
		_ = e.sessionStore.DeleteSession(context.Background(), sessionID)
		return
	}
	_ = e.sessionStore.WriteSession(context.Background(), record)
}

func (e *Engine) sessionRecordLocked(sessionID string) (storage.SessionRecord, bool) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return storage.SessionRecord{}, false
	}
	var latest *TaskRecord
	for _, taskID := range e.taskOrder {
		record := e.tasks[taskID]
		if record == nil || record.SessionID != sessionID {
			continue
		}
		if latest == nil || record.UpdatedAt.After(latest.UpdatedAt) {
			latest = record
		}
	}
	if latest == nil {
		return storage.SessionRecord{}, false
	}
	status := "idle"
	for _, taskID := range e.taskOrder {
		record := e.tasks[taskID]
		if record == nil || record.SessionID != sessionID {
			continue
		}
		if !record.isFinished() {
			status = "active"
			break
		}
	}
	createdAt := latest.StartedAt
	for _, taskID := range e.taskOrder {
		record := e.tasks[taskID]
		if record == nil || record.SessionID != sessionID {
			continue
		}
		if record.StartedAt.Before(createdAt) {
			createdAt = record.StartedAt
		}
	}
	return storage.SessionRecord{
		SessionID: sessionID,
		Title:     latest.Title,
		Status:    status,
		CreatedAt: createdAt.Format(time.RFC3339Nano),
		UpdatedAt: latest.UpdatedAt.Format(time.RFC3339Nano),
	}, true
}

// clone returns a deep copy of TaskRecord so callers cannot retain internal
// state references. The engine stores mutable maps and slices in memory, so
// every outward read must copy nested state to keep locks and later mutations
// from leaking through shared references.
func (r TaskRecord) clone() TaskRecord {
	clone := r
	clone.Intent = cloneMap(r.Intent)
	clone.Timeline = cloneTimeline(r.Timeline)
	clone.BubbleMessage = cloneMap(r.BubbleMessage)
	clone.DeliveryResult = cloneMap(r.DeliveryResult)
	clone.Artifacts = cloneMapSlice(r.Artifacts)
	clone.Citations = cloneMapSlice(r.Citations)
	clone.AuditRecords = cloneMapSlice(r.AuditRecords)
	clone.MirrorReferences = cloneMapSlice(r.MirrorReferences)
	clone.SecuritySummary = cloneMap(r.SecuritySummary)
	clone.ApprovalRequest = cloneMap(r.ApprovalRequest)
	clone.PendingExecution = cloneMap(r.PendingExecution)
	clone.Authorization = cloneMap(r.Authorization)
	clone.ImpactScope = cloneMap(r.ImpactScope)
	clone.TokenUsage = cloneMap(r.TokenUsage)
	clone.MemoryReadPlans = cloneMapSlice(r.MemoryReadPlans)
	clone.MemoryWritePlans = cloneMapSlice(r.MemoryWritePlans)
	clone.StorageWritePlan = cloneMap(r.StorageWritePlan)
	clone.ArtifactPlans = cloneMapSlice(r.ArtifactPlans)
	clone.Notifications = cloneNotifications(r.Notifications)
	clone.LatestEvent = cloneMap(r.LatestEvent)
	clone.LatestToolCall = cloneMap(r.LatestToolCall)
	clone.SteeringMessages = append([]string(nil), r.SteeringMessages...)
	if r.FinishedAt != nil {
		finishedAt := *r.FinishedAt
		clone.FinishedAt = &finishedAt
	}
	return clone
}

// queueNotification appends one buffered outbound notification. RPC transports
// drain this queue after the main response so state changes and delivery events
// can be replayed in order without requiring the runtime engine to know the
// active transport.
func (r *TaskRecord) queueNotification(method string, params map[string]any) {
	r.Notifications = append(r.Notifications, NotificationRecord{
		Method:    method,
		Params:    cloneMap(params),
		CreatedAt: time.Now(),
	})
}

// isFinished reports whether the task is already in a terminal status.
func (r TaskRecord) isFinished() bool {
	switch r.Status {
	case "completed", "cancelled", "ended_unfinished", "failed":
		return true
	default:
		return false
	}
}

// runStatus maps the task status to the reduced compatibility-layer run state.
func (r TaskRecord) runStatus() string {
	if r.Status == "completed" {
		return "completed"
	}
	return "processing"
}

// cloneTimeline returns a shallow copy of the task timeline slice.
func cloneTimeline(timeline []TaskStepRecord) []TaskStepRecord {
	if len(timeline) == 0 {
		return nil
	}

	result := make([]TaskStepRecord, len(timeline))
	copy(result, timeline)
	return result
}

func mergeTaskSnapshot(base, update contextsvc.TaskContextSnapshot) contextsvc.TaskContextSnapshot {
	merged := base
	merged.Source = pickLastNonEmpty(base.Source, update.Source)
	merged.Trigger = pickLastNonEmpty(base.Trigger, update.Trigger)
	merged.InputType = pickLastNonEmpty(base.InputType, update.InputType)
	merged.InputMode = pickLastNonEmpty(base.InputMode, update.InputMode)
	merged.Text = mergeSnapshotText(base.Text, update.Text)
	merged.SelectionText = mergeSnapshotText(base.SelectionText, update.SelectionText)
	merged.ErrorText = mergeSnapshotText(base.ErrorText, update.ErrorText)
	merged.Files = dedupeAppendedStrings(base.Files, update.Files)
	merged.PageTitle = pickLastNonEmpty(base.PageTitle, update.PageTitle)
	merged.PageURL = pickLastNonEmpty(base.PageURL, update.PageURL)
	merged.AppName = pickLastNonEmpty(base.AppName, update.AppName)
	merged.WindowTitle = pickLastNonEmpty(base.WindowTitle, update.WindowTitle)
	merged.VisibleText = mergeSnapshotText(base.VisibleText, update.VisibleText)
	merged.ScreenSummary = mergeSnapshotText(base.ScreenSummary, update.ScreenSummary)
	merged.ClipboardText = mergeSnapshotText(base.ClipboardText, update.ClipboardText)
	merged.HoverTarget = pickLastNonEmpty(base.HoverTarget, update.HoverTarget)
	merged.LastAction = pickLastNonEmpty(base.LastAction, update.LastAction)
	if update.DwellMillis > 0 {
		merged.DwellMillis = update.DwellMillis
	}
	if update.CopyCount > 0 {
		merged.CopyCount = update.CopyCount
	}
	if update.WindowSwitches > 0 {
		merged.WindowSwitches = update.WindowSwitches
	}
	if update.PageSwitches > 0 {
		merged.PageSwitches = update.PageSwitches
	}
	return merged
}

func pickLastNonEmpty(base, update string) string {
	if strings.TrimSpace(update) != "" {
		return strings.TrimSpace(update)
	}
	return strings.TrimSpace(base)
}

func mergeSnapshotText(base, update string) string {
	base = strings.TrimSpace(base)
	update = strings.TrimSpace(update)
	switch {
	case update == "":
		return base
	case base == "":
		return update
	case base == update:
		return base
	default:
		return base + "\n\n" + update
	}
}

func dedupeAppendedStrings(base, update []string) []string {
	if len(base) == 0 && len(update) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(base)+len(update))
	result := make([]string, 0, len(base)+len(update))
	for _, value := range append(append([]string{}, base...), update...) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func timelineStatusForTaskStatus(status string) string {
	switch status {
	case "waiting_input", "waiting_auth":
		return "pending"
	case "failed":
		return "failed"
	case "completed":
		return "completed"
	case "cancelled":
		return "cancelled"
	default:
		return "running"
	}
}

func continuationOutputSummary(bubbleMessage map[string]any, steeringMessage string) string {
	if strings.TrimSpace(steeringMessage) != "" {
		return "Follow-up continuation received"
	}
	if len(bubbleMessage) == 0 {
		return "Task continuation updated"
	}
	text, _ := bubbleMessage["text"].(string)
	if strings.TrimSpace(text) == "" {
		return "Task continuation updated"
	}
	return strings.TrimSpace(text)
}

// cloneMap recursively copies a map[string]any payload.
func cloneMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}

	result := make(map[string]any, len(values))
	for key, value := range values {
		switch typed := value.(type) {
		case map[string]any:
			result[key] = cloneMap(typed)
		case []map[string]any:
			result[key] = cloneMapSlice(typed)
		case []string:
			copied := append([]string(nil), typed...)
			result[key] = copied
		default:
			result[key] = value
		}
	}
	return result
}

// cloneMapSlice recursively copies a []map[string]any payload.
func cloneMapSlice(values []map[string]any) []map[string]any {
	if len(values) == 0 {
		return nil
	}

	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		result = append(result, cloneMap(value))
	}
	return result
}

// cloneNotifications copies a notification slice and its nested params maps.
func cloneNotifications(values []NotificationRecord) []NotificationRecord {
	if len(values) == 0 {
		return nil
	}

	result := make([]NotificationRecord, len(values))
	for index, value := range values {
		result[index] = NotificationRecord{
			Method:    value.Method,
			Params:    cloneMap(value.Params),
			CreatedAt: value.CreatedAt,
		}
	}

	return result
}

// currentTimelineStatus returns the status of the last timeline step because
// the engine treats the tail entry as the single authoritative current step.
func currentTimelineStatus(timeline []TaskStepRecord) string {
	if len(timeline) == 0 {
		return "pending"
	}

	return timeline[len(timeline)-1].Status
}

func isSessionBusyTask(record *TaskRecord) bool {
	if record == nil {
		return false
	}
	switch record.Status {
	case "processing", "waiting_auth", "paused":
		return true
	default:
		return false
	}
}

// advanceTimeline moves the task timeline forward, completing the previous step
// before appending a new one when the step name changes.
func advanceTimeline(timeline []TaskStepRecord, stepName, status, outputSummary string) []TaskStepRecord {
	if len(timeline) == 0 {
		return []TaskStepRecord{{
			StepID:        timelineStepID("", stepName, 1),
			Name:          stepName,
			Status:        status,
			OrderIndex:    1,
			InputSummary:  "",
			OutputSummary: outputSummary,
		}}
	}

	updated := cloneTimeline(timeline)
	lastIndex := len(updated) - 1
	if updated[lastIndex].Name != stepName {
		updated[lastIndex].Status = "completed"
		nextOrderIndex := updated[lastIndex].OrderIndex + 1
		updated = append(updated, TaskStepRecord{
			StepID:        timelineStepID(updated[lastIndex].TaskID, stepName, nextOrderIndex),
			TaskID:        updated[lastIndex].TaskID,
			Name:          stepName,
			Status:        status,
			OrderIndex:    nextOrderIndex,
			InputSummary:  updated[lastIndex].OutputSummary,
			OutputSummary: outputSummary,
		})
		return updated
	}

	updated[lastIndex].Status = status
	updated[lastIndex].OutputSummary = outputSummary
	return updated
}

func timelineStepID(taskID, stepName string, orderIndex int) string {
	orderIndex = maxInt(orderIndex, 1)
	if strings.TrimSpace(taskID) != "" {
		return fmt.Sprintf("%s_step_%03d_%s", taskID, orderIndex, stepName)
	}
	return fmt.Sprintf("step_%03d_%s", orderIndex, stepName)
}

// buildSecuritySummary creates the minimal security summary shown in task
// detail views.
func buildSecuritySummary(riskLevel string, latestRestorePoint map[string]any) map[string]any {
	return map[string]any{
		"security_status":        "normal",
		"risk_level":             riskLevel,
		"pending_authorizations": 0,
		"latest_restore_point":   latestRestorePoint,
	}
}

// mergePreservedSecuritySummary keeps governance/runtime metadata that the
// orchestrator attaches before state-machine transitions rebuild the base
// security summary for completion, failure, cancellation, or recovery.
func mergePreservedSecuritySummary(base map[string]any, current map[string]any) map[string]any {
	merged := cloneMap(base)
	if merged == nil {
		merged = map[string]any{}
	}
	for key, value := range current {
		if strings.HasPrefix(key, "budget_auto_downgrade_") {
			merged[key] = value
		}
	}
	return merged
}

func latestRestorePointFromSummary(summary map[string]any) map[string]any {
	if summary == nil {
		return nil
	}
	latestRestorePoint, ok := summary["latest_restore_point"].(map[string]any)
	if !ok {
		return nil
	}
	return cloneMap(latestRestorePoint)
}

// buildRecoveryPoint creates the recovery-point metadata attached when a task
// completes.
func buildRecoveryPoint(taskID string, createdAt time.Time) map[string]any {
	return map[string]any{
		"recovery_point_id": fmt.Sprintf("rp_%d", createdAt.UnixNano()),
		"task_id":           taskID,
		"summary":           "工具执行前恢复点",
		"created_at":        createdAt.Format(time.RFC3339),
		"objects":           []string{defaultRecoveryPathObj},
	}
}

func (e *Engine) trackSessionLocked(sessionID string) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	for _, existing := range e.sessionOrder {
		if existing == sessionID {
			return
		}
	}
	e.sessionOrder = append(e.sessionOrder, sessionID)
}

func (e *Engine) untrackSessionLocked(sessionID string) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return
	}
	for _, task := range e.tasks {
		if task != nil && task.SessionID == sessionID {
			return
		}
	}
	e.sessionOrder = removeStringValue(e.sessionOrder, sessionID)
}

func removeStringValue(values []string, target string) []string {
	if len(values) == 0 {
		return nil
	}
	filtered := values[:0]
	for _, value := range values {
		if value == target {
			continue
		}
		filtered = append(filtered, value)
	}
	return append([]string(nil), filtered...)
}

// timelineCurrentStepID returns the step_id of the latest timeline entry.
func timelineCurrentStepID(timeline []TaskStepRecord) any {
	if len(timeline) == 0 {
		return nil
	}

	return timeline[len(timeline)-1].StepID
}

// firstNonEmpty returns primary when present, otherwise fallback.
func firstNonEmpty(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

func runtimeStopReasonFromPayload(payload map[string]any) string {
	if len(payload) == 0 {
		return ""
	}
	if stopReason, ok := payload["stop_reason"].(string); ok && strings.TrimSpace(stopReason) != "" {
		return strings.TrimSpace(stopReason)
	}
	if stopReason, ok := payload["loop_stop_reason"].(string); ok && strings.TrimSpace(stopReason) != "" {
		return strings.TrimSpace(stopReason)
	}
	return ""
}

// sortTaskRecords orders task lists according to the protocol-level sort fields
// and direction.
func sortTaskRecords(records []TaskRecord, sortBy, sortOrder string) {
	if len(records) <= 1 {
		return
	}

	sortBy = normalizeTaskSortBy(sortBy)
	sortOrder = normalizeTaskSortOrder(sortOrder)

	sort.SliceStable(records, func(i, j int) bool {
		left, right := taskSortValue(records[i], sortBy), taskSortValue(records[j], sortBy)
		if left.Equal(right) {
			return records[i].UpdatedAt.After(records[j].UpdatedAt)
		}
		if sortOrder == "asc" {
			return left.Before(right)
		}
		return left.After(right)
	})
}

func taskSortValue(record TaskRecord, sortBy string) time.Time {
	switch sortBy {
	case "started_at":
		return record.StartedAt
	case "finished_at":
		if record.FinishedAt != nil {
			return *record.FinishedAt
		}
		return time.Time{}
	default:
		return record.UpdatedAt
	}
}

func normalizeTaskSortBy(sortBy string) string {
	switch sortBy {
	case "started_at", "finished_at":
		return sortBy
	default:
		return "updated_at"
	}
}

func normalizeTaskSortOrder(sortOrder string) string {
	if sortOrder == "asc" {
		return sortOrder
	}
	return "desc"
}

// stringSlice converts a JSON-decoded value into a trimmed []string.
func stringSlice(rawValue any) []string {
	values, ok := rawValue.([]string)
	if ok {
		return append([]string(nil), values...)
	}

	anyValues, ok := rawValue.([]any)
	if !ok {
		return nil
	}

	result := make([]string, 0, len(anyValues))
	for _, rawItem := range anyValues {
		item, ok := rawItem.(string)
		if ok && item != "" {
			result = append(result, item)
		}
	}
	return result
}

// mergeMaps recursively overlays source values into destination.
func mergeMaps(target map[string]any, patch map[string]any) {
	for key, value := range patch {
		patchMap, ok := value.(map[string]any)
		if ok {
			currentMap, currentOk := target[key].(map[string]any)
			if !currentOk {
				currentMap = map[string]any{}
			}
			mergeMaps(currentMap, patchMap)
			target[key] = currentMap
			continue
		}
		target[key] = value
	}
}

func buildDefaultNotepadItems(now time.Time) []map[string]any {
	base := now.In(time.FixedZone("CST", 8*60*60))
	dueToday := time.Date(base.Year(), base.Month(), base.Day(), 18, 0, 0, 0, base.Location())
	if dueToday.Before(base) {
		dueToday = base.Add(2 * time.Hour)
	}
	later := dueToday.Add(48 * time.Hour)
	recurring := dueToday.Add(7 * 24 * time.Hour)
	completedAt := dueToday.Add(-24 * time.Hour)

	return []map[string]any{
		{
			"item_id":                "todo_001",
			"title":                  "整理本周会议纪要",
			"bucket":                 "upcoming",
			"status":                 "normal",
			"type":                   "one_time",
			"due_at":                 dueToday.Format(time.RFC3339),
			"agent_suggestion":       "先生成一个结构化摘要",
			"recurring_enabled":      nil,
			"note_text":              "把这周会议里的共识、待确认事项和风险点整理成一页结构化纪要，方便后续同步给项目组。",
			"prerequisite":           "先确认会议录音、群聊结论和白板截图都已经归档。",
			"repeat_rule_text":       "",
			"next_occurrence_at":     nil,
			"recent_instance_status": nil,
			"effective_scope":        nil,
			"ended_at":               nil,
			"related_resources": []map[string]any{{
				"resource_id":   "todo_001_minutes",
				"label":         "会议纪要目录",
				"path":          "workspace/meetings",
				"resource_type": "folder",
				"open_action":   "reveal_in_folder",
				"open_payload": map[string]any{
					"path":    "workspace/meetings",
					"task_id": nil,
					"url":     nil,
				},
			}},
		},
		{
			"item_id":                "todo_002",
			"title":                  "补齐下周评审材料",
			"bucket":                 "later",
			"status":                 "normal",
			"type":                   "one_time",
			"due_at":                 later.Format(time.RFC3339),
			"agent_suggestion":       "可以先整理提纲再扩写成文档",
			"recurring_enabled":      nil,
			"note_text":              "这份材料暂时不急着执行，但需要提前把背景、目标和评审关注点补齐，否则下周会上无法直接过稿。",
			"prerequisite":           "等本周结论稳定后再整理，避免材料重复返工。",
			"repeat_rule_text":       "",
			"next_occurrence_at":     nil,
			"recent_instance_status": nil,
			"effective_scope":        nil,
			"ended_at":               nil,
			"related_resources": []map[string]any{{
				"resource_id":   "todo_002_review",
				"label":         "评审材料草稿",
				"path":          "workspace/reviews/next-week.md",
				"resource_type": "file",
				"open_action":   "open_file",
				"open_payload": map[string]any{
					"path":    "workspace/reviews/next-week.md",
					"task_id": nil,
					"url":     nil,
				},
			}},
		},
		{
			"item_id":                "todo_003",
			"title":                  "每周项目复盘",
			"bucket":                 "recurring_rule",
			"status":                 "normal",
			"type":                   "recurring",
			"due_at":                 recurring.Format(time.RFC3339),
			"agent_suggestion":       "建议生成固定模板后重复复用",
			"recurring_enabled":      true,
			"note_text":              "每周固定回看目标完成情况、风险变化和下周重点，持续沉淀团队可复用的复盘节奏。",
			"prerequisite":           "先把本周新增任务和已完成交付汇总齐全。",
			"repeat_rule_text":       "每周五 18:00",
			"next_occurrence_at":     recurring.Format(time.RFC3339),
			"recent_instance_status": "上次复盘已完成并生成摘要",
			"effective_scope":        "仅对当前项目工作周生效",
			"ended_at":               nil,
			"related_resources": []map[string]any{{
				"resource_id":   "todo_003_template",
				"label":         "复盘模板",
				"path":          "workspace/templates/weekly-retro.md",
				"resource_type": "file",
				"open_action":   "open_file",
				"open_payload": map[string]any{
					"path":    "workspace/templates/weekly-retro.md",
					"task_id": nil,
					"url":     nil,
				},
			}},
		},
		{
			"item_id":                "todo_004",
			"title":                  "已归档的日报整理",
			"bucket":                 "closed",
			"status":                 "completed",
			"type":                   "one_time",
			"due_at":                 completedAt.Format(time.RFC3339),
			"agent_suggestion":       nil,
			"recurring_enabled":      nil,
			"note_text":              "这条事项已经处理完成并归档，用来保留来源记录和后续追溯入口。",
			"prerequisite":           nil,
			"repeat_rule_text":       "",
			"next_occurrence_at":     nil,
			"recent_instance_status": nil,
			"effective_scope":        nil,
			"ended_at":               completedAt.Format(time.RFC3339),
			"related_resources": []map[string]any{{
				"resource_id":   "todo_004_archive",
				"label":         "归档日报",
				"path":          "workspace/archive/daily-summary.md",
				"resource_type": "file",
				"open_action":   "open_file",
				"open_payload": map[string]any{
					"path":    "workspace/archive/daily-summary.md",
					"task_id": nil,
					"url":     nil,
				},
			}},
		},
	}
}

func (e *Engine) findNotepadItem(itemID string) (map[string]any, int, bool) {
	for index, item := range e.notepadItems {
		if stringValue(item, "item_id", "") == itemID {
			return item, index, true
		}
	}
	return nil, -1, false
}

func deriveNotepadStatus(item map[string]any, now time.Time) string {
	status := stringValue(item, "status", "normal")
	if status == "completed" || status == "cancelled" {
		return status
	}
	if stringValue(item, "bucket", "") == "closed" {
		return "completed"
	}

	dueAt, ok := parseNotepadDueTime(item)
	if !ok {
		return "normal"
	}
	nowAtDueZone := now.In(dueAt.Location())
	if dueAt.Before(nowAtDueZone) {
		return "overdue"
	}
	if sameDay(dueAt, nowAtDueZone) {
		return "due_today"
	}
	return "normal"
}

func parseNotepadDueTime(item map[string]any) (time.Time, bool) {
	dueAt := stringValue(item, "due_at", "")
	if dueAt == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, dueAt)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func sameDay(left, right time.Time) bool {
	return left.Year() == right.Year() && left.YearDay() == right.YearDay()
}

func sortNotepadItems(items []map[string]any) {
	sort.Slice(items, func(i, j int) bool {
		leftBucket := stringValue(items[i], "bucket", "")
		rightBucket := stringValue(items[j], "bucket", "")
		if leftBucket != rightBucket {
			return todoBucketRank(leftBucket) < todoBucketRank(rightBucket)
		}

		leftDue, leftOK := parseNotepadDueTime(items[i])
		rightDue, rightOK := parseNotepadDueTime(items[j])
		switch {
		case leftOK && rightOK && !leftDue.Equal(rightDue):
			return leftDue.Before(rightDue)
		case leftOK != rightOK:
			return leftOK
		}

		return stringValue(items[i], "title", "") < stringValue(items[j], "title", "")
	})
}

func todoBucketRank(bucket string) int {
	switch bucket {
	case "upcoming":
		return 0
	case "later":
		return 1
	case "recurring_rule":
		return 2
	case "closed":
		return 3
	default:
		return 4
	}
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func boolValue(rawValue any, fallback bool) bool {
	if rawValue == nil {
		return fallback
	}
	value, ok := rawValue.(bool)
	if !ok {
		return fallback
	}
	return value
}

func stringValue(values map[string]any, key, fallback string) string {
	rawValue, ok := values[key]
	if !ok {
		return fallback
	}

	value, ok := rawValue.(string)
	if !ok || value == "" {
		return fallback
	}

	return value
}

// buildDefaultSettings constructs the default settings snapshot used by the
// main pipeline and dashboard surfaces. Defaults intentionally stay workspace-
// relative so the runtime does not leak machine-specific absolute paths into
// persisted task, recovery, or automation configuration.
func buildDefaultSettings() map[string]any {
	return map[string]any{
		"general": map[string]any{
			"language":                   "zh-CN",
			"auto_launch":                true,
			"theme_mode":                 "follow_system",
			"voice_notification_enabled": true,
			"voice_type":                 "default_female",
			"download": map[string]any{
				"workspace_path":            defaultSettingsWorkspaceRoot(),
				"ask_before_save_each_file": true,
			},
		},
		"floating_ball": map[string]any{
			"auto_snap":        true,
			"idle_translucent": true,
			"position_mode":    "draggable",
			"size":             "medium",
		},
		"memory": map[string]any{
			"enabled":                  true,
			"lifecycle":                "30d",
			"work_summary_interval":    map[string]any{"unit": "day", "value": 7},
			"profile_refresh_interval": map[string]any{"unit": "week", "value": 2},
		},
		"task_automation": map[string]any{
			"inspect_on_startup":     true,
			"inspect_on_file_change": true,
			"inspection_interval":    map[string]any{"unit": "minute", "value": 15},
			"task_sources":           []string{defaultSettingsTaskSourcePath()},
			"remind_before_deadline": true,
			"remind_when_stale":      false,
		},
		"models": map[string]any{
			"provider": "openai",
			"credentials": map[string]any{
				"budget_auto_downgrade": true,
				"base_url":              "",
				"model":                 "",
				"budget_policy": map[string]any{
					"planner_retry_budget":      1,
					"failure_signal_window":     2,
					"token_pressure_threshold":  64,
					"cost_pressure_threshold":   0.05,
					"expensive_tool_categories": []string{"command", "browser_mutation", "media_heavy"},
				},
			},
		},
	}
}

func modelRouteSettingsChanged(updatedKeys []string) bool {
	for _, key := range updatedKeys {
		switch strings.TrimSpace(key) {
		case "models.provider",
			"models.base_url",
			"models.model",
			"models.api_key",
			"models.delete_api_key",
			"models.credentials.provider",
			"models.credentials.base_url",
			"models.credentials.model":
			return true
		}
	}
	return false
}

func normalizeSettingsPatch(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	normalized := cloneMap(values)
	if normalized == nil {
		normalized = map[string]any{}
	}
	if dataLog, ok := normalized["data_log"].(map[string]any); ok && len(dataLog) > 0 {
		models := cloneMap(mapValue(normalized, "models"))
		if models == nil {
			models = map[string]any{}
		}
		credentials := cloneMap(mapValue(models, "credentials"))
		if credentials == nil {
			credentials = map[string]any{}
		}
		for key, value := range dataLog {
			switch key {
			case "provider":
				models[key] = value
			default:
				credentials[key] = value
			}
		}
		if len(credentials) > 0 {
			models["credentials"] = credentials
		}
		normalized["models"] = models
		delete(normalized, "data_log")
	}
	if models, ok := normalized["models"].(map[string]any); ok && len(models) > 0 {
		credentials := cloneMap(mapValue(models, "credentials"))
		if credentials == nil {
			credentials = map[string]any{}
		}
		delete(models, "provider_api_key_configured")
		delete(models, "stronghold")
		delete(credentials, "provider_api_key_configured")
		delete(credentials, "stronghold")
		for _, key := range []string{"budget_auto_downgrade", "base_url", "model", "budget_policy"} {
			if value, exists := models[key]; exists {
				credentials[key] = value
				delete(models, key)
			}
		}
		models["credentials"] = credentials
		normalized["models"] = models
	}
	normalized = migrateLegacyWorkspaceSettings(normalized)
	return normalized
}

func migrateLegacyWorkspaceSettings(values map[string]any) map[string]any {
	if len(values) == 0 {
		return values
	}
	general := cloneMap(mapValue(values, "general"))
	if len(general) > 0 {
		download := cloneMap(mapValue(general, "download"))
		if len(download) > 0 {
			if migrated, changed := migrateWorkspaceRootSetting(download["workspace_path"]); changed {
				download["workspace_path"] = migrated
				general["download"] = download
				values["general"] = general
			}
		}
	}
	taskAutomation := cloneMap(mapValue(values, "task_automation"))
	if len(taskAutomation) > 0 {
		if migrated, changed := migrateTaskSourceSettings(taskAutomation["task_sources"]); changed {
			taskAutomation["task_sources"] = migrated
			values["task_automation"] = taskAutomation
		}
	}
	return values
}

func migrateWorkspaceRootSetting(rawValue any) (string, bool) {
	value, ok := rawValue.(string)
	if !ok {
		return "", false
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	// Windows drive-relative inputs (for example `C:workspace`) are not stable
	// runtime roots, so migration resets them the same way as any other unsafe
	// legacy-relative workspace placeholder.
	if hasWindowsDriveLetterPrefix(trimmed) && !isWindowsStyleAbsolutePath(trimmed) {
		return defaultSettingsWorkspaceRoot(), true
	}
	if isRuntimeAbsolutePathLike(trimmed) {
		cleaned := filepath.ToSlash(filepath.Clean(trimmed))
		return cleaned, cleaned != trimmed
	}
	normalized := path.Clean(strings.ReplaceAll(trimmed, "\\", "/"))
	if normalized == "." || normalized == defaultWorkspaceRoot {
		return defaultSettingsWorkspaceRoot(), true
	}
	if !isSafeRuntimeRelativePath(normalized) {
		return defaultSettingsWorkspaceRoot(), true
	}
	return filepath.ToSlash(filepath.Join(serviceconfig.DefaultRuntimeRoot(), filepath.FromSlash(normalized))), true
}

func migrateTaskSourceSettings(rawValue any) ([]string, bool) {
	_, recognizedString := rawValue.([]string)
	_, recognizedAny := rawValue.([]any)
	if !recognizedString && !recognizedAny {
		return nil, false
	}
	sources := stringSlice(rawValue)
	result := make([]string, 0, len(sources))
	changed := false
	for _, source := range sources {
		migrated, migratedChanged := migrateTaskSourceSetting(source)
		if strings.TrimSpace(migrated) == "" {
			continue
		}
		result = append(result, migrated)
		changed = changed || migratedChanged
	}
	return dedupeStrings(result), changed
}

func migrateTaskSourceSetting(source string) (string, bool) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return "", false
	}
	// Drive-relative task sources must be dropped during migration because they do
	// not bind to a stable runtime workspace across platforms or launches.
	if hasWindowsDriveLetterPrefix(trimmed) && !isWindowsStyleAbsolutePath(trimmed) {
		return "", true
	}
	if isRuntimeAbsolutePathLike(trimmed) {
		cleaned := filepath.ToSlash(filepath.Clean(trimmed))
		return cleaned, cleaned != trimmed
	}
	normalized := path.Clean(strings.ReplaceAll(trimmed, "\\", "/"))
	if normalized == "." || normalized == defaultWorkspaceRoot {
		return defaultSettingsWorkspaceRoot(), true
	}
	if strings.HasPrefix(normalized, defaultWorkspaceRoot+"/") {
		return filepath.ToSlash(filepath.Join(serviceconfig.DefaultWorkspaceRoot(), filepath.FromSlash(strings.TrimPrefix(normalized, defaultWorkspaceRoot+"/")))), true
	}
	if !isSafeRuntimeRelativePath(normalized) {
		return "", true
	}
	return filepath.ToSlash(filepath.Join(serviceconfig.DefaultRuntimeRoot(), filepath.FromSlash(normalized))), true
}

func isSafeRuntimeRelativePath(normalized string) bool {
	if normalized == ".." || strings.HasPrefix(normalized, "../") {
		return false
	}
	if hasWindowsDriveLetterPrefix(normalized) || filepath.VolumeName(filepath.FromSlash(normalized)) != "" {
		return false
	}
	return !strings.HasPrefix(normalized, "/")
}

// isRuntimeAbsolutePathLike keeps migration semantics host-independent so a
// Windows-style absolute path remains absolute even when the snapshot is loaded
// on a Unix host during tests or cross-platform maintenance flows.
func isRuntimeAbsolutePathLike(value string) bool {
	trimmed := strings.TrimSpace(value)
	if hasWindowsDriveLetterPrefix(trimmed) {
		return isWindowsStyleAbsolutePath(trimmed)
	}
	return filepath.IsAbs(trimmed)
}

func hasWindowsDriveLetterPrefix(value string) bool {
	if len(value) < 2 {
		return false
	}
	letter := value[0]
	return ((letter >= 'A' && letter <= 'Z') || (letter >= 'a' && letter <= 'z')) && value[1] == ':'
}

func isWindowsStyleAbsolutePath(value string) bool {
	return hasWindowsDriveLetterPrefix(value) && len(value) >= 3 && (value[2] == '\\' || value[2] == '/')
}

func mapValue(values map[string]any, path ...string) map[string]any {
	current := values
	for _, key := range path {
		rawValue, ok := current[key]
		if !ok {
			return nil
		}
		next, ok := rawValue.(map[string]any)
		if !ok {
			return nil
		}
		current = next
	}
	return current
}

func settingsPatchPaths(prefix string, patch map[string]any) []string {
	if len(patch) == 0 {
		return nil
	}
	keys := make([]string, 0, len(patch))
	for key := range patch {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	paths := make([]string, 0, len(keys))
	for _, key := range keys {
		nextPrefix := prefix + "." + key
		if nested, ok := patch[key].(map[string]any); ok && len(nested) > 0 {
			paths = append(paths, settingsPatchPaths(nextPrefix, nested)...)
			continue
		}
		paths = append(paths, nextPrefix)
	}
	return paths
}

func taskRecordToStorage(record TaskRecord) storage.TaskRunRecord {
	return storage.TaskRunRecord{
		TaskID:            record.TaskID,
		SessionID:         record.SessionID,
		RunID:             record.RunID,
		RequestSource:     record.RequestSource,
		RequestTrigger:    record.RequestTrigger,
		ExecutionAttempt:  record.ExecutionAttempt,
		Title:             record.Title,
		SourceType:        record.SourceType,
		Status:            record.Status,
		Intent:            cloneMap(record.Intent),
		PreferredDelivery: record.PreferredDelivery,
		FallbackDelivery:  record.FallbackDelivery,
		CurrentStep:       record.CurrentStep,
		RiskLevel:         record.RiskLevel,
		StartedAt:         record.StartedAt,
		UpdatedAt:         record.UpdatedAt,
		FinishedAt:        cloneTimePointer(record.FinishedAt),
		Timeline:          timelineToStorage(record.Timeline),
		BubbleMessage:     cloneMap(record.BubbleMessage),
		DeliveryResult:    cloneMap(record.DeliveryResult),
		Artifacts:         cloneMapSlice(record.Artifacts),
		Citations:         cloneMapSlice(record.Citations),
		AuditRecords:      cloneMapSlice(record.AuditRecords),
		MirrorReferences:  cloneMapSlice(record.MirrorReferences),
		Snapshot:          cloneContextSnapshot(record.Snapshot),
		SecuritySummary:   cloneMap(record.SecuritySummary),
		ApprovalRequest:   cloneMap(record.ApprovalRequest),
		PendingExecution:  cloneMap(record.PendingExecution),
		Authorization:     cloneMap(record.Authorization),
		ImpactScope:       cloneMap(record.ImpactScope),
		TokenUsage:        cloneMap(record.TokenUsage),
		MemoryReadPlans:   cloneMapSlice(record.MemoryReadPlans),
		MemoryWritePlans:  cloneMapSlice(record.MemoryWritePlans),
		StorageWritePlan:  cloneMap(record.StorageWritePlan),
		ArtifactPlans:     cloneMapSlice(record.ArtifactPlans),
		Notifications:     notificationsToStorage(record.Notifications),
		LatestEvent:       cloneMap(record.LatestEvent),
		LatestToolCall:    cloneMap(record.LatestToolCall),
		LoopStopReason:    record.LoopStopReason,
		SteeringMessages:  append([]string(nil), record.SteeringMessages...),
		CurrentStepStatus: record.CurrentStepStatus,
	}
}

func taskRecordFromStorage(record storage.TaskRunRecord) TaskRecord {
	return TaskRecord{
		TaskID:            record.TaskID,
		SessionID:         record.SessionID,
		RunID:             record.RunID,
		PrimaryRunID:      record.RunID,
		RequestSource:     firstNonEmpty(record.RequestSource, record.Snapshot.Source),
		RequestTrigger:    firstNonEmpty(record.RequestTrigger, record.Snapshot.Trigger),
		ExecutionAttempt:  maxInt(record.ExecutionAttempt, 1),
		Title:             record.Title,
		SourceType:        record.SourceType,
		Status:            record.Status,
		Intent:            cloneMap(record.Intent),
		PreferredDelivery: record.PreferredDelivery,
		FallbackDelivery:  record.FallbackDelivery,
		CurrentStep:       record.CurrentStep,
		RiskLevel:         record.RiskLevel,
		StartedAt:         record.StartedAt,
		UpdatedAt:         record.UpdatedAt,
		FinishedAt:        cloneTimePointer(record.FinishedAt),
		Timeline:          timelineFromStorage(record.Timeline),
		BubbleMessage:     cloneMap(record.BubbleMessage),
		DeliveryResult:    cloneMap(record.DeliveryResult),
		Artifacts:         cloneMapSlice(record.Artifacts),
		Citations:         cloneMapSlice(record.Citations),
		AuditRecords:      cloneMapSlice(record.AuditRecords),
		MirrorReferences:  cloneMapSlice(record.MirrorReferences),
		Snapshot:          cloneContextSnapshot(record.Snapshot),
		SecuritySummary:   cloneMap(record.SecuritySummary),
		ApprovalRequest:   cloneMap(record.ApprovalRequest),
		PendingExecution:  cloneMap(record.PendingExecution),
		Authorization:     cloneMap(record.Authorization),
		ImpactScope:       cloneMap(record.ImpactScope),
		TokenUsage:        cloneMap(record.TokenUsage),
		MemoryReadPlans:   cloneMapSlice(record.MemoryReadPlans),
		MemoryWritePlans:  cloneMapSlice(record.MemoryWritePlans),
		StorageWritePlan:  cloneMap(record.StorageWritePlan),
		ArtifactPlans:     cloneMapSlice(record.ArtifactPlans),
		Notifications:     notificationsFromStorage(record.Notifications),
		LatestEvent:       cloneMap(record.LatestEvent),
		LatestToolCall:    cloneMap(record.LatestToolCall),
		LoopStopReason:    record.LoopStopReason,
		SteeringMessages:  append([]string(nil), record.SteeringMessages...),
		CurrentStepStatus: record.CurrentStepStatus,
	}
}

func timelineToStorage(timeline []TaskStepRecord) []storage.TaskStepSnapshot {
	if len(timeline) == 0 {
		return nil
	}

	result := make([]storage.TaskStepSnapshot, len(timeline))
	for index, step := range timeline {
		result[index] = storage.TaskStepSnapshot{
			StepID:        step.StepID,
			TaskID:        step.TaskID,
			Name:          step.Name,
			Status:        step.Status,
			OrderIndex:    step.OrderIndex,
			InputSummary:  step.InputSummary,
			OutputSummary: step.OutputSummary,
		}
	}

	return result
}

func timelineFromStorage(timeline []storage.TaskStepSnapshot) []TaskStepRecord {
	if len(timeline) == 0 {
		return nil
	}

	result := make([]TaskStepRecord, len(timeline))
	for index, step := range timeline {
		result[index] = TaskStepRecord{
			StepID:        step.StepID,
			TaskID:        step.TaskID,
			Name:          step.Name,
			Status:        step.Status,
			OrderIndex:    step.OrderIndex,
			InputSummary:  step.InputSummary,
			OutputSummary: step.OutputSummary,
		}
	}

	return result
}

func notificationsToStorage(values []NotificationRecord) []storage.NotificationSnapshot {
	if len(values) == 0 {
		return nil
	}

	result := make([]storage.NotificationSnapshot, len(values))
	for index, value := range values {
		result[index] = storage.NotificationSnapshot{
			Method:    value.Method,
			Params:    cloneMap(value.Params),
			CreatedAt: value.CreatedAt,
		}
	}

	return result
}

func notificationsFromStorage(values []storage.NotificationSnapshot) []NotificationRecord {
	if len(values) == 0 {
		return nil
	}

	result := make([]NotificationRecord, len(values))
	for index, value := range values {
		result[index] = NotificationRecord{
			Method:    value.Method,
			Params:    cloneMap(value.Params),
			CreatedAt: value.CreatedAt,
		}
	}

	return result
}

func cloneTimePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}

func maxInt(primary, fallback int) int {
	if primary > 0 {
		return primary
	}
	return fallback
}

func cloneContextSnapshot(snapshot contextsvc.TaskContextSnapshot) contextsvc.TaskContextSnapshot {
	cloned := snapshot
	if len(snapshot.Files) > 0 {
		cloned.Files = append([]string(nil), snapshot.Files...)
	}
	return cloned
}
