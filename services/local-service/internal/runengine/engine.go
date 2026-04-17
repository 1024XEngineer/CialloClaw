// 该文件负责内存态 task/run 运行时与状态流转。
package runengine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

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

// TaskRecord 描述当前模块记录。

// TaskRecord 是 runengine 在内存中维护的任务主记录。
// 它同时承载对外 task 语义和对内 run 执行态之间的映射结果，是 4 号主链路最核心的状态对象。
type TaskRecord struct {
	TaskID            string
	SessionID         string
	RunID             string
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
	CurrentStepStatus string
}

// TaskStepRecord 描述当前模块记录。

// TaskStepRecord 表示 task 视角下的时间线步骤。
// orchestrator 会根据它构造 timeline，用于 dashboard、task detail 和运行态追踪。
type TaskStepRecord struct {
	StepID        string
	TaskID        string
	Name          string
	Status        string
	OrderIndex    int
	InputSummary  string
	OutputSummary string
}

// NotificationRecord 描述当前模块记录。

// NotificationRecord 保存尚未被前端消费的通知事件。
// RPC 层会在响应之后继续回放这些通知，形成 task.updated / delivery.ready 等事件流。
type NotificationRecord struct {
	Method    string
	Params    map[string]any
	CreatedAt time.Time
}

// CreateTaskInput 定义当前模块的数据结构。

// CreateTaskInput 描述创建任务时 runengine 需要的一揽子初始化参数。
// orchestrator 会在这里一次性传入标题、状态、intent、timeline 和初始展示信息。
type CreateTaskInput struct {
	SessionID         string
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
	MirrorReferences  []map[string]any
	Snapshot          contextsvc.TaskContextSnapshot
}

// InspectorConfig 描述当前模块配置。

// InspectorConfig 保存任务巡检模块的当前配置快照。
type InspectorConfig struct {
	TaskSources          []string
	InspectionInterval   map[string]any
	InspectOnFileChange  bool
	InspectOnStartup     bool
	RemindBeforeDeadline bool
	RemindWhenStale      bool
}

// Engine 维护当前模块的运行状态。

// Engine 是主链路运行态的内存状态机。
// 它负责维护 task/run 映射、时间线推进、授权等待、交付计划、通知缓存以及设置项快照。
type Engine struct {
	mu            sync.RWMutex
	nextID        uint64
	now           func() time.Time
	taskStore     storage.TaskRunStore
	todoStore     storage.TodoStore
	tasks         map[string]*TaskRecord
	taskOrder     []string
	sessionOrder  []string
	inspector     InspectorConfig
	settings      map[string]any
	notepadItems  []map[string]any
	notepadClaims map[string]struct{}
}

// NewEngine 创建并返回Engine。

// NewEngine 创建一套新的内存态引擎，并填充主链路需要的默认设置和巡检配置。
func NewEngine() *Engine {
	engine, _ := newEngine(nil)
	return engine
}

// NewEngineWithStore 创建带有 task/run 持久化存储的引擎实例。
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

func newEngine(taskStore storage.TaskRunStore) (*Engine, error) {
	engine := &Engine{
		now:           time.Now,
		taskStore:     taskStore,
		tasks:         map[string]*TaskRecord{},
		taskOrder:     []string{},
		sessionOrder:  []string{},
		notepadClaims: map[string]struct{}{},
		inspector: InspectorConfig{
			TaskSources:          []string{defaultTaskSourcePath},
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

// CurrentState 处理当前模块的相关逻辑。

// CurrentState 返回兼容层的 run_status。
// 对外产品态仍以 task_status 为主，这里只保留 processing/completed 两态兼容。
func (e *Engine) CurrentState() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.taskOrder) == 0 {
		return "processing"
	}

	return e.tasks[e.taskOrder[0]].runStatus()
}

// CurrentTaskStatus 处理当前模块的相关逻辑。

// CurrentTaskStatus 返回当前主任务的 task_status。
func (e *Engine) CurrentTaskStatus() string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.taskOrder) == 0 {
		return "confirming_intent"
	}

	return e.tasks[e.taskOrder[0]].Status
}

// CreateTask 创建Task。

// CreateTask 创建 task/run 映射，并把初始时间线、展示信息和安全摘要写入内存态。
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
		MirrorReferences:  cloneMapSlice(input.MirrorReferences),
		Snapshot:          cloneContextSnapshot(input.Snapshot),
		SecuritySummary:   buildSecuritySummary(input.RiskLevel, nil),
		CurrentStepStatus: currentTimelineStatus(stepTimeline),
	}

	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", map[string]any{
		"task_id": taskID,
		"status":  record.Status,
	})

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
	return nil
}

// GetTask 获取Task。

// GetTask 根据 task_id 读取一份防御性复制后的任务快照。
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

// HydrateTaskFromStorage 将持久化快照重新装载回运行时内存，用于恢复重启后的治理动作。
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

// ListTasks 列出Tasks。

// ListTasks 按未完成/已完成分组列出任务，并在分页前应用统一排序规则。
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

// ConfirmTask 确认Task。

// ConfirmTask 把处于 confirming_intent 的任务推进到 processing。
// 这里会更新标题、intent、当前步骤和气泡展示，并推进 timeline。
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
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// BeginExecution 把任务推进到真实执行步骤，并刷新 timeline 与事件。
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
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// UpdateIntent 更新Task当前生效意图。

// UpdateIntent 在不改变整体任务身份的前提下覆盖当前生效意图与标题。
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
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// SetPresentation 设置Presentation。

// SetPresentation 只更新任务的展示层信息，不改变主状态机结论。
// 它常用于确认态、等待输入态或仅更新气泡的场景。
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
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})
	if deliveryResult != nil {
		record.queueNotification("delivery.ready", map[string]any{
			"task_id":         record.TaskID,
			"delivery_result": cloneMap(record.DeliveryResult),
		})
	}
	e.persistTaskLocked(record)

	return record.clone(), true
}

// RecordToolCall 记录主链路最近一次完成的 tool_call 兼容层快照。
func (e *Engine) RecordToolCall(taskID, toolName string, input, output map[string]any, durationMS int64) (TaskRecord, bool) {
	return e.RecordToolCallLifecycle(taskID, toolName, "succeeded", input, output, durationMS, nil)
}

// RecordToolCallLifecycle 根据工具执行状态记录最近一次 tool_call 快照。
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
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})
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
	record.queueNotification("task.updated", map[string]any{
		"task_id":     record.TaskID,
		"status":      record.Status,
		"stop_reason": firstNonEmpty(stopReason, record.LoopStopReason),
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// FailTaskExecution 将任务收敛到 failed，用于执行失败或恢复点准备失败场景。
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
	record.SecuritySummary = map[string]any{
		"security_status":        firstNonEmpty(securityStatus, "execution_error"),
		"risk_level":             record.RiskLevel,
		"pending_authorizations": 0,
		"latest_restore_point":   restorePoint,
	}
	record.Timeline = advanceTimeline(record.Timeline, record.CurrentStep, "failed", firstNonEmpty(outputSummary, "执行失败"))
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// BlockTaskByPolicy 将被治理策略拦截的任务收敛到 cancelled。
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
	record.SecuritySummary = map[string]any{
		"security_status":        "intercepted",
		"risk_level":             record.RiskLevel,
		"pending_authorizations": 0,
		"latest_restore_point":   latestRestorePointFromSummary(record.SecuritySummary),
	}
	record.Timeline = advanceTimeline(record.Timeline, "risk_blocked", "cancelled", firstNonEmpty(outputSummary, "高风险操作已被策略拦截"))
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// CompleteTask 完成Task。

// CompleteTask 把任务收敛到 completed，并写入正式交付结果、artifact 和恢复点摘要。
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
	record.Timeline = advanceTimeline(record.Timeline, "return_result", "completed", "结果已正式交付")
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	restorePoint := buildRecoveryPoint(record.TaskID, now)
	if len(latestRestorePoint) > 0 && len(latestRestorePoint[0]) > 0 {
		restorePoint = cloneMap(latestRestorePoint[0])
	}
	record.SecuritySummary = buildSecuritySummary(record.RiskLevel, restorePoint)
	record.LatestEvent = e.buildEvent(record, "delivery.ready")
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})
	record.queueNotification("delivery.ready", map[string]any{
		"task_id":         record.TaskID,
		"delivery_result": cloneMap(record.DeliveryResult),
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// ApplyRecoveryOutcome 在恢复点应用后刷新任务的状态、安全摘要与通知回写。
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
	record.SecuritySummary = map[string]any{
		"security_status":        firstNonEmpty(securityStatus, "recovered"),
		"risk_level":             record.RiskLevel,
		"pending_authorizations": 0,
		"latest_restore_point":   cloneMap(recoveryPoint),
	}
	eventType := "recovery.failed"
	if securityStatus == "recovered" {
		eventType = "recovery.applied"
	}
	record.LatestEvent = e.buildEventWithPayload(record, eventType, map[string]any{
		"status":            record.Status,
		"security_status":   firstNonEmpty(securityStatus, "recovered"),
		"recovery_point_id": stringValue(cloneMap(recoveryPoint), "recovery_point_id", ""),
	})
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// ControlTask 控制Task。

// ControlTask 处理 pause/resume/cancel/restart 等用户控制动作。
func (e *Engine) ControlTask(taskID, action string, bubbleMessage map[string]any) (TaskRecord, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, ErrTaskNotFound
	}

	now := e.now()
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
		if record.Status != "paused" {
			return TaskRecord{}, ErrTaskStatusInvalid
		}
		record.Status = "processing"
	case "cancel":
		if record.isFinished() {
			return TaskRecord{}, ErrTaskAlreadyFinished
		}
		record.Status = "cancelled"
		record.FinishedAt = &now
		record.ApprovalRequest = nil
		record.PendingExecution = nil
		record.SecuritySummary = buildSecuritySummary(record.RiskLevel, latestRestorePointFromSummary(record.SecuritySummary))
		record.Timeline = advanceTimeline(record.Timeline, "task_cancelled", "cancelled", "任务已取消")
		record.CurrentStep = "task_cancelled"
	case "restart":
		if !record.isFinished() {
			return TaskRecord{}, ErrTaskStatusInvalid
		}
		record.Status = "processing"
		record.FinishedAt = nil
		record.CurrentStep = "generate_output"
		record.DeliveryResult = nil
		record.Artifacts = nil
		record.BubbleMessage = nil
		record.ApprovalRequest = nil
		record.PendingExecution = nil
		record.Authorization = nil
		record.ImpactScope = nil
		record.StorageWritePlan = nil
		record.ArtifactPlans = nil
		record.MemoryReadPlans = nil
		record.MemoryWritePlans = nil
		record.MirrorReferences = nil
		record.SecuritySummary = buildSecuritySummary(record.RiskLevel, latestRestorePointFromSummary(record.SecuritySummary))
		record.Timeline = advanceTimeline(record.Timeline, "generate_output", "running", "任务已重新开始")
	default:
		return TaskRecord{}, ErrTaskStatusInvalid
	}

	record.UpdatedAt = now
	record.BubbleMessage = cloneMap(bubbleMessage)
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})
	e.persistTaskLocked(record)

	return record.clone(), nil
}

// MarkWaitingApproval 处理当前模块的相关逻辑。

// MarkWaitingApproval 是等待授权态的简化入口。
func (e *Engine) MarkWaitingApproval(taskID string, approvalRequest map[string]any, bubbleMessage map[string]any) (TaskRecord, bool) {
	return e.MarkWaitingApprovalWithPlan(taskID, approvalRequest, nil, bubbleMessage)
}

// MarkWaitingApprovalWithPlan 将任务切换为等待授权，并附带待恢复执行计划。

// MarkWaitingApprovalWithPlan 把任务切换到 waiting_auth，并附带后续恢复执行所需的计划。
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
	record.SecuritySummary = map[string]any{
		"security_status":        "pending_confirmation",
		"risk_level":             record.RiskLevel,
		"pending_authorizations": 1,
		"latest_restore_point":   latestRestorePoint,
	}
	record.Timeline = advanceTimeline(record.Timeline, "waiting_authorization", "running", "等待用户授权")
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "approval.pending")
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})
	record.queueNotification("approval.pending", map[string]any{
		"task_id":          record.TaskID,
		"approval_request": cloneMap(record.ApprovalRequest),
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// ResolveAuthorization 处理Authorization。

// ResolveAuthorization 记录本次授权结果，并清理挂起中的审批请求与待执行计划。
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
	record.SecuritySummary = buildSecuritySummary(record.RiskLevel, latestRestorePoint)
	e.persistTaskLocked(record)
	return record.clone(), true
}

// ResumeAfterApproval 将已授权任务恢复到处理中状态，并保留后续执行计划。

// ResumeAfterApproval 在用户允许后把任务从 waiting_auth 恢复到 processing。
func (e *Engine) ResumeAfterApproval(taskID string, authorization map[string]any, impactScope map[string]any, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
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
	record.SecuritySummary = buildSecuritySummary(record.RiskLevel, latestRestorePointFromSummary(record.SecuritySummary))
	record.Timeline = advanceTimeline(record.Timeline, "authorized_execution", "running", "授权通过，继续执行")
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// DenyAfterApproval 将已拒绝授权的任务收敛到结束状态。

// DenyAfterApproval 在用户拒绝授权时终止任务，并保留授权记录与影响范围摘要。
func (e *Engine) DenyAfterApproval(taskID string, authorization map[string]any, impactScope map[string]any, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
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
	record.SecuritySummary = map[string]any{
		"security_status":        "intercepted",
		"risk_level":             record.RiskLevel,
		"pending_authorizations": 0,
		"latest_restore_point":   latestRestorePointFromSummary(record.SecuritySummary),
	}
	record.Timeline = advanceTimeline(record.Timeline, "authorization_denied", "cancelled", "用户拒绝授权，任务已结束")
	record.CurrentStepStatus = currentTimelineStatus(record.Timeline)
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// PendingExecutionPlan 返回等待授权任务保存的执行计划。

// PendingExecutionPlan 返回任务在等待授权期间缓存的恢复执行计划。
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
// session so the session-level agent loop remains serial.
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
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})
	record.queueNotification("task.session_queued", map[string]any{
		"task_id":          record.TaskID,
		"blocking_task_id": blockingTaskID,
	})
	e.persistTaskLocked(record)

	return record.clone(), true
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
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})
	record.queueNotification("task.session_resumed", map[string]any{
		"task_id": record.TaskID,
	})
	e.persistTaskLocked(record)

	return record.clone(), true
}

// SetMemoryPlans 设置MemoryPlans。

// SetMemoryPlans 记录 memory 读取/写入计划，供主链路后续交接和观测使用。
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

// SetDeliveryPlans 设置DeliveryPlans。

// SetDeliveryPlans 记录 workspace 写入计划和 artifact 持久化计划。
// SetMirrorReferences 记录任务挂接后的镜像引用快照。
// SetMirrorReferences 记录任务挂接后的镜像引用快照。
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

// SetDeliveryPlans 记录 workspace 写入计划和 artifact 持久化计划。
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

// PendingNotifications 返回待处理的Notifications。

// PendingNotifications 返回当前尚未被消费的通知快照。
func (e *Engine) PendingNotifications(taskID string) ([]NotificationRecord, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return nil, false
	}

	return cloneNotifications(record.Notifications), true
}

// DrainNotifications 取出并清空Notifications。

// DrainNotifications 取出并清空某个任务的通知队列。
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

// PendingApprovalRequests 返回待处理的ApprovalRequests。

// PendingApprovalRequests 枚举当前所有待处理的审批请求。
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

// TaskDetail 处理当前模块的相关逻辑。

// TaskDetail 返回任务详情视图所需的完整任务快照。
func (e *Engine) TaskDetail(taskID string) (TaskRecord, bool) {
	return e.GetTask(taskID)
}

// InspectorConfig 处理当前模块的相关逻辑。

// InspectorConfig 返回任务巡检配置的当前有效值。
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

// UpdateInspectorConfig 更新InspectorConfig。

// UpdateInspectorConfig 用补丁方式更新巡检配置，并返回更新后的完整快照。
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

	return e.InspectorConfig()
}

// Settings 设置tings。

// Settings 返回当前内存中的设置快照。
func (e *Engine) Settings() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return cloneMap(e.settings)
}

// UpdateSettings 更新Settings。

// UpdateSettings 合并设置补丁，并计算受影响字段、应用模式和是否需要重启。
func (e *Engine) UpdateSettings(values map[string]any) (map[string]any, []string, string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	updatedKeys := make([]string, 0)
	effectiveSettings := map[string]any{}
	applyMode := "immediate"
	needRestart := false

	for _, section := range []string{"general", "floating_ball", "memory", "task_automation", "data_log"} {
		sectionPatch, ok := values[section].(map[string]any)
		if !ok || len(sectionPatch) == 0 {
			continue
		}

		currentSection, ok := e.settings[section].(map[string]any)
		if !ok {
			currentSection = map[string]any{}
		}

		mergeMaps(currentSection, sectionPatch)
		e.settings[section] = currentSection
		effectiveSettings[section] = cloneMap(sectionPatch)

		keys := make([]string, 0, len(sectionPatch))
		for key := range sectionPatch {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			updatedKeys = append(updatedKeys, fmt.Sprintf("%s.%s", section, key))
		}

		if section == "general" {
			if _, ok := sectionPatch["language"]; ok {
				applyMode = "restart_required"
				needRestart = true
			}
		}
	}

	return effectiveSettings, updatedKeys, applyMode, needRestart
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

// buildEvent 处理当前模块的相关逻辑。

// buildEvent 为当前任务生成一条兼容层 Event 记录。
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

// buildToolCall 处理当前模块的相关逻辑。

// buildToolCall 为当前任务生成一条兼容层 ToolCall 记录。
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

// nextIdentifier 处理当前模块的相关逻辑。
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
	if e.taskStore == nil || record == nil {
		return
	}

	_ = e.taskStore.SaveTaskRun(context.Background(), taskRecordToStorage(record.clone()))
}

// clone 处理当前模块的相关逻辑。

// clone 返回 TaskRecord 的深拷贝，避免外部持有内部状态引用。
func (r TaskRecord) clone() TaskRecord {
	clone := r
	clone.Intent = cloneMap(r.Intent)
	clone.Timeline = cloneTimeline(r.Timeline)
	clone.BubbleMessage = cloneMap(r.BubbleMessage)
	clone.DeliveryResult = cloneMap(r.DeliveryResult)
	clone.Artifacts = cloneMapSlice(r.Artifacts)
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
	if r.FinishedAt != nil {
		finishedAt := *r.FinishedAt
		clone.FinishedAt = &finishedAt
	}
	return clone
}

// queueNotification 处理当前模块的相关逻辑。

// queueNotification 把一条通知追加到任务的待发送队列。
func (r *TaskRecord) queueNotification(method string, params map[string]any) {
	r.Notifications = append(r.Notifications, NotificationRecord{
		Method:    method,
		Params:    cloneMap(params),
		CreatedAt: time.Now(),
	})
}

// isFinished 处理当前模块的相关逻辑。
func (r TaskRecord) isFinished() bool {
	switch r.Status {
	case "completed", "cancelled", "ended_unfinished", "failed":
		return true
	default:
		return false
	}
}

// runStatus 处理当前模块的相关逻辑。
func (r TaskRecord) runStatus() string {
	if r.Status == "completed" {
		return "completed"
	}
	return "processing"
}

// cloneTimeline 处理当前模块的相关逻辑。
func cloneTimeline(timeline []TaskStepRecord) []TaskStepRecord {
	if len(timeline) == 0 {
		return nil
	}

	result := make([]TaskStepRecord, len(timeline))
	copy(result, timeline)
	return result
}

// cloneMap 处理当前模块的相关逻辑。
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

// cloneMapSlice 处理当前模块的相关逻辑。
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

// cloneNotifications 处理当前模块的相关逻辑。
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

// currentTimelineStatus 处理当前模块的相关逻辑。

// currentTimelineStatus 返回当前时间线最后一个步骤的状态。
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

// advanceTimeline 处理当前模块的相关逻辑。

// advanceTimeline 推进 task timeline。
// 如果步骤名发生变化，会先完成上一个步骤，再追加一个新的步骤记录。
func advanceTimeline(timeline []TaskStepRecord, stepName, status, outputSummary string) []TaskStepRecord {
	if len(timeline) == 0 {
		return []TaskStepRecord{{
			StepID:        fmt.Sprintf("step_%s", stepName),
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
		updated = append(updated, TaskStepRecord{
			StepID:        fmt.Sprintf("step_%s", stepName),
			TaskID:        updated[lastIndex].TaskID,
			Name:          stepName,
			Status:        status,
			OrderIndex:    updated[lastIndex].OrderIndex + 1,
			InputSummary:  updated[lastIndex].OutputSummary,
			OutputSummary: outputSummary,
		})
		return updated
	}

	updated[lastIndex].Status = status
	updated[lastIndex].OutputSummary = outputSummary
	return updated
}

// buildSecuritySummary 处理当前模块的相关逻辑。

// buildSecuritySummary 生成任务详情里展示的最小安全摘要。
func buildSecuritySummary(riskLevel string, latestRestorePoint map[string]any) map[string]any {
	return map[string]any{
		"security_status":        "normal",
		"risk_level":             riskLevel,
		"pending_authorizations": 0,
		"latest_restore_point":   latestRestorePoint,
	}
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

// buildRecoveryPoint 处理当前模块的相关逻辑。

// buildRecoveryPoint 生成任务完成时附带的恢复点元数据。
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

// timelineCurrentStepID 处理当前模块的相关逻辑。
func timelineCurrentStepID(timeline []TaskStepRecord) any {
	if len(timeline) == 0 {
		return nil
	}

	return timeline[len(timeline)-1].StepID
}

// firstNonEmpty 处理当前模块的相关逻辑。
func firstNonEmpty(primary, fallback string) string {
	if primary != "" {
		return primary
	}
	return fallback
}

// sortTaskRecords 按协议约定的排序字段和方向整理任务列表。
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

// stringSlice 处理当前模块的相关逻辑。
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

// mergeMaps 处理当前模块的相关逻辑。
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

// buildDefaultSettings 处理当前模块的相关逻辑。

// buildDefaultSettings 构造主链路和工作台使用的默认设置快照。
func buildDefaultSettings() map[string]any {
	return map[string]any{
		"general": map[string]any{
			"language":                   "zh-CN",
			"auto_launch":                true,
			"theme_mode":                 "follow_system",
			"voice_notification_enabled": true,
			"voice_type":                 "default_female",
			"download": map[string]any{
				"workspace_path":            defaultWorkspaceRoot,
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
			"task_sources":           []string{defaultTaskSourcePath},
			"remind_before_deadline": true,
			"remind_when_stale":      false,
		},
		"data_log": map[string]any{
			"provider":              "openai",
			"budget_auto_downgrade": true,
		},
	}
}

func taskRecordToStorage(record TaskRecord) storage.TaskRunRecord {
	return storage.TaskRunRecord{
		TaskID:            record.TaskID,
		SessionID:         record.SessionID,
		RunID:             record.RunID,
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
		CurrentStepStatus: record.CurrentStepStatus,
	}
}

func taskRecordFromStorage(record storage.TaskRunRecord) TaskRecord {
	return TaskRecord{
		TaskID:            record.TaskID,
		SessionID:         record.SessionID,
		RunID:             record.RunID,
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

func cloneContextSnapshot(snapshot contextsvc.TaskContextSnapshot) contextsvc.TaskContextSnapshot {
	cloned := snapshot
	if len(snapshot.Files) > 0 {
		cloned.Files = append([]string(nil), snapshot.Files...)
	}
	return cloned
}
