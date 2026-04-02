package executor

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cialloclaw/internal/action/tooling"
	"cialloclaw/internal/cognition/agents"
	"cialloclaw/internal/protocol"
	"cialloclaw/internal/runtime/eventbus"
	"cialloclaw/internal/runtime/id"
	"cialloclaw/internal/state/approval"
	logstate "cialloclaw/internal/state/log"
	"cialloclaw/internal/state/memory"
	"cialloclaw/internal/state/session"
	"cialloclaw/internal/state/task"
)

type AssignmentExecutor struct {
	Bus         eventbus.Bus
	TaskRepo    task.Repository
	SessionRepo session.Repository
	MemoryRepo  memory.Repository
	LogRepo     logstate.Repository
	Agents      *agents.Registry
	Tools       *tooling.Registry
}

func (e *AssignmentExecutor) ID() string {
	return "action.assignment_executor"
}

func (e *AssignmentExecutor) SubscribeTypes() []string {
	return []string{protocol.EventTypeTaskAssigned}
}

func (e *AssignmentExecutor) Handle(ctx context.Context, event protocol.Event) error {
	if event.TaskID == nil {
		return nil
	}
	item, err := e.TaskRepo.GetByID(ctx, *event.TaskID)
	if err != nil || item == nil {
		return err
	}
	if item.AssigneeAgent == nil {
		return errors.New("task has no assigned agent")
	}
	agent, ok := e.Agents.Get(*item.AssigneeAgent)
	if !ok {
		return fmt.Errorf("agent %s not found", *item.AssigneeAgent)
	}
	wm, err := ensureWorkingMemory(ctx, e.MemoryRepo, item.SessionID)
	if err != nil {
		return err
	}
	plan, err := agent.Execute(ctx, *item, *wm)
	if err != nil {
		return err
	}
	now := time.Now()
	item.Status = task.StatusRunning
	item.StartedAt = optionalTime(item.StartedAt, now)
	item.UpdatedAt = now
	if len(item.Steps) > 0 {
		item.Steps[0].Status = task.StepRunning
		item.Steps[0].ToolName = &plan.ToolName
		item.Steps[0].StartedAt = optionalTime(item.Steps[0].StartedAt, now)
	}
	if err := e.TaskRepo.Update(ctx, item); err != nil {
		return err
	}

	startedEvent := protocol.DeriveEvent(event, protocol.EventTypeTaskExecutionStarted, "action.assignment_executor", protocol.PriorityHigh, map[string]any{
		"task_id": item.ID,
	})
	startedEvent.TaskID = &item.ID
	if len(item.Steps) > 0 {
		startedEvent.TaskStepID = &item.Steps[0].ID
	}
	_ = appendLog(ctx, e.LogRepo, startedEvent, logstate.LevelInfo, "execution", "任务执行已开始", map[string]any{"task_id": item.ID, "tool": plan.ToolName})
	if err := e.Bus.Publish(ctx, startedEvent); err != nil {
		return err
	}

	metaTool, ok := e.Tools.Get(plan.ToolName)
	if !ok {
		return fmt.Errorf("tool %s not found", plan.ToolName)
	}
	meta := metaTool.Metadata()
	requested := protocol.DeriveEvent(event, protocol.EventTypeToolCallRequested, "action.assignment_executor", protocol.PriorityHigh, protocol.ToolCallRequestedPayload{
		ToolName:        plan.ToolName,
		Args:            plan.ToolArgs,
		RiskLevel:       string(meta.Risk),
		RequireApproval: meta.RequiresApproval,
	})
	requested.TaskID = &item.ID
	if len(item.Steps) > 0 {
		requested.TaskStepID = &item.Steps[0].ID
	}
	return e.Bus.Publish(ctx, requested)
}

type ToolExecutor struct {
	Bus      eventbus.Bus
	Tools    *tooling.Registry
	LogRepo  logstate.Repository
	TaskRepo task.Repository
}

func (e *ToolExecutor) ID() string {
	return "action.tool_executor"
}

func (e *ToolExecutor) SubscribeTypes() []string {
	return []string{protocol.EventTypeToolCallRequested}
}

func (e *ToolExecutor) Handle(ctx context.Context, event protocol.Event) error {
	payload, ok := event.Payload.(protocol.ToolCallRequestedPayload)
	if !ok {
		return nil
	}
	tool, ok := e.Tools.Get(payload.ToolName)
	if !ok {
		return fmt.Errorf("tool %s not found", payload.ToolName)
	}
	meta := tool.Metadata()
	runCtx, cancel := context.WithTimeout(ctx, meta.Timeout)
	defer cancel()

	startedAt := time.Now()
	result, err := tool.Execute(runCtx, tooling.ToolCall{
		SessionID: event.SessionID,
		TaskID:    event.TaskID,
		StepID:    event.TaskStepID,
		Args:      payload.Args,
	})
	duration := time.Since(startedAt).Milliseconds()
	if err != nil && result.Error == nil {
		text := err.Error()
		result.Error = &text
	}

	completed := protocol.DeriveEvent(event, protocol.EventTypeToolCallCompleted, "action.tool_executor", protocol.PriorityHigh, protocol.ToolCallCompletedPayload{
		ToolName:   payload.ToolName,
		Success:    err == nil && result.Success,
		Result:     result.Data,
		Error:      result.Error,
		DurationMS: duration,
	})
	completed.TaskID = event.TaskID
	completed.TaskStepID = event.TaskStepID
	_ = appendLog(ctx, e.LogRepo, completed, logstate.LevelInfo, "tool", "工具调用已完成", map[string]any{"tool": payload.ToolName, "success": err == nil && result.Success})
	return e.Bus.Publish(ctx, completed)
}

type ResultHandler struct {
	Bus         eventbus.Bus
	TaskRepo    task.Repository
	SessionRepo session.Repository
	MemoryRepo  memory.Repository
	LogRepo     logstate.Repository
}

func (e *ResultHandler) ID() string {
	return "action.result_handler"
}

func (e *ResultHandler) SubscribeTypes() []string {
	return []string{protocol.EventTypeToolCallCompleted}
}

func (e *ResultHandler) Handle(ctx context.Context, event protocol.Event) error {
	if event.TaskID == nil {
		return nil
	}
	payload, ok := event.Payload.(protocol.ToolCallCompletedPayload)
	if !ok {
		return nil
	}
	item, err := e.TaskRepo.GetByID(ctx, *event.TaskID)
	if err != nil || item == nil {
		return err
	}

	now := time.Now()
	if payload.Success {
		item.Status = task.StatusSucceeded
		item.Output = payload.Result
		item.CompletedAt = &now
		item.Summary = contentSummary(payload.Result)
		if len(item.Steps) > 0 {
			item.Steps[0].Status = task.StepDone
			item.Steps[0].Output = payload.Result
			item.Steps[0].EndedAt = &now
		}
	} else {
		item.Status = task.StatusFailed
		item.CompletedAt = &now
		item.Error = payload.Error
		if len(item.Steps) > 0 {
			item.Steps[0].Status = task.StepFailed
			item.Steps[0].Error = payload.Error
			item.Steps[0].EndedAt = &now
		}
	}
	item.UpdatedAt = now
	if err := e.TaskRepo.Update(ctx, item); err != nil {
		return err
	}
	if err := touchTask(ctx, e.SessionRepo, e.MemoryRepo, item.SessionID, item.ID, "remove"); err != nil {
		return err
	}

	level := logstate.LevelInfo
	resultType := protocol.EventTypeTaskExecutionComplete
	message := "任务执行已完成"
	if !payload.Success {
		level = logstate.LevelError
		resultType = protocol.EventTypeTaskExecutionFailed
		message = "任务执行失败"
	}
	followup := protocol.DeriveEvent(event, resultType, "action.result_handler", protocol.PriorityHigh, map[string]any{
		"task_id": item.ID,
		"summary": item.Summary,
	})
	followup.TaskID = &item.ID
	if len(item.Steps) > 0 {
		followup.TaskStepID = &item.Steps[0].ID
	}
	_ = appendLog(ctx, e.LogRepo, followup, level, "task", message, map[string]any{"task_id": item.ID, "status": item.Status})
	return e.Bus.Publish(ctx, followup)
}

type ApprovalResponder struct {
	Bus          eventbus.Bus
	ApprovalRepo approval.Repository
	SessionRepo  session.Repository
	MemoryRepo   memory.Repository
	LogRepo      logstate.Repository
}

func (e *ApprovalResponder) ID() string {
	return "action.approval_responder"
}

func (e *ApprovalResponder) SubscribeTypes() []string {
	return []string{protocol.EventTypeApprovalResponded}
}

func (e *ApprovalResponder) Handle(ctx context.Context, event protocol.Event) error {
	if event.ApprovalID == nil {
		return nil
	}
	payload, ok := event.Payload.(protocol.ApprovalRespondedPayload)
	if !ok {
		return nil
	}
	item, err := e.ApprovalRepo.GetByID(ctx, *event.ApprovalID)
	if err != nil || item == nil {
		return err
	}
	if item.Status != approval.StatusPending {
		return nil
	}
	now := time.Now()
	item.DecidedAt = &now
	item.DecidedBy = ptr("user")
	item.Reason = payload.Reason
	switch payload.Decision {
	case "approved":
		item.Status = approval.StatusApproved
	case "rejected":
		item.Status = approval.StatusRejected
	case "expired":
		item.Status = approval.StatusExpired
	default:
		item.Status = approval.StatusCanceled
	}
	if err := e.ApprovalRepo.Update(ctx, item); err != nil {
		return err
	}
	if err := touchApproval(ctx, e.SessionRepo, e.MemoryRepo, item.SessionID, item.ID, "remove"); err != nil {
		return err
	}
	_ = appendLog(ctx, e.LogRepo, event, logstate.LevelAudit, "approval", "审批结果已记录", map[string]any{"approval_id": item.ID, "decision": item.Status})

	if item.Status == approval.StatusApproved && item.ActionType == "video_summary" {
		intentEvent := protocol.DeriveEvent(event, protocol.EventTypeIntentRecognized, "action.approval_responder", protocol.PriorityHigh, protocol.IntentRecognizedPayload{
			Intent:     "video_summary",
			Confidence: 1,
			Entities: map[string]any{
				"url": item.ProposedArgs["url"],
			},
			RawInput: fmt.Sprint(item.ProposedArgs["url"]),
		})
		return e.Bus.Publish(ctx, intentEvent)
	}
	return nil
}

func ensureWorkingMemory(ctx context.Context, repo memory.Repository, sessionID string) (*memory.WorkingMemory, error) {
	wm, err := repo.GetWorkingSnapshot(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if wm != nil {
		return wm, nil
	}
	return &memory.WorkingMemory{
		ID:        id.New("wm"),
		SessionID: sessionID,
		UpdatedAt: time.Now(),
	}, nil
}

func touchTask(ctx context.Context, sessionRepo session.Repository, memoryRepo memory.Repository, sessionID, taskID, mode string) error {
	item, err := sessionRepo.GetByID(ctx, sessionID)
	if err != nil || item == nil {
		return err
	}
	if mode == "remove" {
		item.ActiveTaskIDs = removeValue(item.ActiveTaskIDs, taskID)
		if item.CurrentTaskID != nil && *item.CurrentTaskID == taskID {
			item.CurrentTaskID = nil
		}
	}
	item.LastActiveAt = time.Now()
	if err := sessionRepo.Update(ctx, item); err != nil {
		return err
	}
	wm, err := ensureWorkingMemory(ctx, memoryRepo, sessionID)
	if err != nil {
		return err
	}
	if mode == "remove" {
		wm.ActiveTaskIDs = removeValue(wm.ActiveTaskIDs, taskID)
	}
	wm.UpdatedAt = time.Now()
	return memoryRepo.SaveWorkingSnapshot(ctx, wm)
}

func touchApproval(ctx context.Context, sessionRepo session.Repository, memoryRepo memory.Repository, sessionID, approvalID, mode string) error {
	item, err := sessionRepo.GetByID(ctx, sessionID)
	if err != nil || item == nil {
		return err
	}
	if mode == "remove" {
		item.PendingApprovalIDs = removeValue(item.PendingApprovalIDs, approvalID)
	}
	item.LastActiveAt = time.Now()
	if err := sessionRepo.Update(ctx, item); err != nil {
		return err
	}
	wm, err := ensureWorkingMemory(ctx, memoryRepo, sessionID)
	if err != nil {
		return err
	}
	if mode == "remove" {
		wm.PendingApprovalIDs = removeValue(wm.PendingApprovalIDs, approvalID)
	}
	wm.UpdatedAt = time.Now()
	return memoryRepo.SaveWorkingSnapshot(ctx, wm)
}

func appendLog(ctx context.Context, repo logstate.Repository, event protocol.Event, level logstate.Level, category, message string, payload map[string]any) error {
	sessionID := event.SessionID
	entry := &logstate.Entry{
		ID:           id.New("log"),
		Timestamp:    time.Now(),
		Level:        level,
		SessionID:    &sessionID,
		TaskID:       event.TaskID,
		TaskStepID:   event.TaskStepID,
		LoopID:       event.LoopID,
		ApprovalID:   event.ApprovalID,
		EventID:      &event.ID,
		TraceID:      event.TraceID,
		SpanID:       id.New("span"),
		ParentSpanID: &event.SpanID,
		Category:     category,
		Message:      message,
		Payload:      payload,
	}
	return repo.Append(ctx, entry)
}

func optionalTime(current *time.Time, fallback time.Time) *time.Time {
	if current != nil {
		return current
	}
	return &fallback
}

func contentSummary(result map[string]any) string {
	if result == nil {
		return ""
	}
	if content, ok := result["content"].(string); ok {
		if len(content) > 160 {
			return content[:160] + "..."
		}
		return content
	}
	return fmt.Sprint(result)
}

func removeValue(values []string, candidate string) []string {
	var out []string
	for _, value := range values {
		if value != candidate {
			out = append(out, value)
		}
	}
	return out
}

func ptr[T any](value T) *T {
	return &value
}
