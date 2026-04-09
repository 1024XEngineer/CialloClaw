// 该文件负责内存态 task/run 运行时与状态流转。
package runengine

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

const (
	defaultWorkspaceRoot   = "workspace"
	defaultTaskSourcePath  = "workspace/todos"
	defaultRecoveryPathObj = "workspace/temp.md"
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
	MirrorReferences  []map[string]any
	SecuritySummary   map[string]any
	ApprovalRequest   map[string]any
	PendingExecution  map[string]any
	Authorization     map[string]any
	ImpactScope       map[string]any
	MemoryReadPlans   []map[string]any
	MemoryWritePlans  []map[string]any
	StorageWritePlan  map[string]any
	ArtifactPlans     []map[string]any
	Notifications     []NotificationRecord
	LatestEvent       map[string]any
	LatestToolCall    map[string]any
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
	mu           sync.RWMutex
	nextID       uint64
	now          func() time.Time
	tasks        map[string]*TaskRecord
	taskOrder    []string
	sessionOrder []string
	inspector    InspectorConfig
	settings     map[string]any
	notepadItems []map[string]any
}

// NewEngine 创建并返回Engine。

// NewEngine 创建一套新的内存态引擎，并填充主链路需要的默认设置和巡检配置。
func NewEngine() *Engine {
	engine := &Engine{
		now:          time.Now,
		tasks:        map[string]*TaskRecord{},
		taskOrder:    []string{},
		sessionOrder: []string{},
		inspector: InspectorConfig{
			TaskSources:          []string{defaultTaskSourcePath},
			InspectionInterval:   map[string]any{"unit": "minute", "value": 15},
			InspectOnFileChange:  true,
			InspectOnStartup:     true,
			RemindBeforeDeadline: true,
			RemindWhenStale:      false,
		},
		settings: buildDefaultSettings(),
		notepadItems: []map[string]any{
			{
				"item_id":          "todo_001",
				"title":            "整理 Q3 复盘要点",
				"bucket":           "upcoming",
				"status":           "due_today",
				"type":             "one_time",
				"due_at":           "2026-04-07T18:00:00+08:00",
				"agent_suggestion": "先生成一个 3 点摘要",
			},
		},
	}

	return engine
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
		SecuritySummary:   buildSecuritySummary(input.RiskLevel, nil),
		CurrentStepStatus: currentTimelineStatus(stepTimeline),
	}

	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.LatestToolCall = e.buildToolCall(record, "read_file")
	record.queueNotification("task.updated", map[string]any{
		"task_id": taskID,
		"status":  record.Status,
	})

	e.tasks[taskID] = record
	e.taskOrder = append([]string{taskID}, e.taskOrder...)

	return record.clone()
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
// 这里会更新 intent、当前步骤和气泡展示，并推进 timeline。
func (e *Engine) ConfirmTask(taskID string, intent map[string]any, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

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

	return record.clone(), true
}

// UpdateIntent 更新Task当前生效意图。

// UpdateIntent 在不改变整体任务身份的前提下覆盖当前生效意图。
func (e *Engine) UpdateIntent(taskID string, intent map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	record.Intent = cloneMap(intent)
	record.UpdatedAt = e.now()
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})

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

	return record.clone(), true
}

// CompleteTask 完成Task。

// CompleteTask 把任务收敛到 completed，并写入正式交付结果、artifact 和恢复点摘要。
func (e *Engine) CompleteTask(taskID string, deliveryResult map[string]any, bubbleMessage map[string]any, artifacts []map[string]any) (TaskRecord, bool) {
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
	record.SecuritySummary = buildSecuritySummary(record.RiskLevel, buildRecoveryPoint(record.TaskID, now))
	record.LatestEvent = e.buildEvent(record, "delivery.ready")
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})
	record.queueNotification("delivery.ready", map[string]any{
		"task_id":         record.TaskID,
		"delivery_result": cloneMap(record.DeliveryResult),
	})

	return record.clone(), true
}

// ControlTask 控制Task。

// ControlTask 处理 pause/resume/cancel/restart 等用户控制动作。
func (e *Engine) ControlTask(taskID, action string, bubbleMessage map[string]any) (TaskRecord, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record, ok := e.tasks[taskID]
	if !ok {
		return TaskRecord{}, false
	}

	now := e.now()
	switch action {
	case "pause":
		record.Status = "paused"
	case "resume":
		record.Status = "processing"
	case "cancel":
		record.Status = "cancelled"
		record.FinishedAt = &now
	case "restart":
		record.Status = "processing"
		record.FinishedAt = nil
	}

	record.UpdatedAt = now
	record.BubbleMessage = cloneMap(bubbleMessage)
	record.LatestEvent = e.buildEvent(record, "task.updated")
	record.queueNotification("task.updated", map[string]any{
		"task_id": record.TaskID,
		"status":  record.Status,
	})

	return record.clone(), true
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
	record.SecuritySummary = map[string]any{
		"security_status":        "pending_confirmation",
		"risk_level":             record.RiskLevel,
		"pending_authorizations": 1,
		"latest_restore_point":   nil,
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
	return record.clone(), true
}

// SetDeliveryPlans 设置DeliveryPlans。

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

// NotepadItems 处理当前模块的相关逻辑。

// NotepadItems 返回便签模块在当前内存态中的示例数据。
func (e *Engine) NotepadItems(group string, limit, offset int) ([]map[string]any, int) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	filtered := make([]map[string]any, 0, len(e.notepadItems))
	for _, item := range e.notepadItems {
		if group != "" {
			if bucket, ok := item["bucket"].(string); !ok || bucket != group {
				continue
			}
		}
		filtered = append(filtered, cloneMap(item))
	}

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

// buildEvent 处理当前模块的相关逻辑。

// buildEvent 为当前任务生成一条兼容层 Event 记录。
func (e *Engine) buildEvent(record *TaskRecord, eventType string) map[string]any {
	return map[string]any{
		"event_id":   e.nextIdentifier("evt"),
		"run_id":     record.RunID,
		"task_id":    record.TaskID,
		"step_id":    timelineCurrentStepID(record.Timeline),
		"type":       eventType,
		"level":      "info",
		"payload":    map[string]any{"status": record.Status},
		"created_at": e.now().Format(time.RFC3339),
	}
}

// buildToolCall 处理当前模块的相关逻辑。

// buildToolCall 为当前任务生成一条兼容层 ToolCall 记录。
func (e *Engine) buildToolCall(record *TaskRecord, toolName string) map[string]any {
	return map[string]any{
		"tool_call_id": e.nextIdentifier("tool"),
		"run_id":       record.RunID,
		"task_id":      record.TaskID,
		"step_id":      timelineCurrentStepID(record.Timeline),
		"tool_name":    toolName,
		"status":       "succeeded",
		"input":        map[string]any{},
		"output":       map[string]any{},
		"error_code":   nil,
		"duration_ms":  120,
	}
}

// nextIdentifier 处理当前模块的相关逻辑。
func (e *Engine) nextIdentifier(prefix string) string {
	e.nextID++
	return fmt.Sprintf("%s_%03d", prefix, e.nextID)
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
	clone.MirrorReferences = cloneMapSlice(r.MirrorReferences)
	clone.SecuritySummary = cloneMap(r.SecuritySummary)
	clone.ApprovalRequest = cloneMap(r.ApprovalRequest)
	clone.PendingExecution = cloneMap(r.PendingExecution)
	clone.Authorization = cloneMap(r.Authorization)
	clone.ImpactScope = cloneMap(r.ImpactScope)
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
