package processors

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

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

type ContextProcessor struct {
	Bus         eventbus.Bus
	SessionRepo session.Repository
	MemoryRepo  memory.Repository
	LogRepo     logstate.Repository
	SessionID   string
}

func (p *ContextProcessor) ID() string {
	return "cognition.context_processor"
}

func (p *ContextProcessor) SubscribeTypes() []string {
	return []string{protocol.EventTypeUserInputReceived, protocol.EventTypeClipboardChanged, protocol.EventTypeTodoScanCompleted}
}

func (p *ContextProcessor) Handle(ctx context.Context, event protocol.Event) error {
	wm, err := ensureWorkingMemory(ctx, p.MemoryRepo, p.SessionID)
	if err != nil {
		return err
	}

	wm.RecentEventIDs = appendRecentEvent(wm.RecentEventIDs, event.ID)
	wm.UpdatedAt = time.Now()

	switch payload := event.Payload.(type) {
	case protocol.UserInputPayload:
		wm.CurrentFocus = ptr(payload.Text)
		wm.ContextSummary = summarizeContext(payload.Text)
	case protocol.ClipboardChangedPayload:
		wm.RecentClipboard = ptr(payload.Text)
		wm.ClipboardKind = payload.Kind
		wm.CurrentFocus = ptr("clipboard")
		wm.ContextSummary = summarizeContext(payload.Text)
		wm.Suggestions = buildClipboardSuggestions(payload)
		if payload.IsVideoURL {
			videoEvent := protocol.DeriveEvent(event, protocol.EventTypeVideoLinkDetected, "cognition.context_processor", protocol.PriorityHigh, protocol.VideoLinkDetectedPayload{URL: payload.Text})
			if err := p.Bus.Publish(ctx, videoEvent); err != nil {
				return err
			}
		}
	case protocol.TodoScanCompletedPayload:
		wm.CurrentFocus = ptr("todo")
		wm.ContextSummary = fmt.Sprintf("最近一次待办扫描：%d 个未完成，%d 个已完成。", payload.PendingCount, payload.CompletedCount)
	}

	if err := p.MemoryRepo.SaveWorkingSnapshot(ctx, wm); err != nil {
		return err
	}
	_ = appendLog(ctx, p.LogRepo, event, logstate.LevelInfo, "context", "工作记忆已更新", nil)

	ctxEvent := protocol.DeriveEvent(event, protocol.EventTypeContextUpdated, "cognition.context_processor", protocol.PriorityNormal, protocol.ContextUpdatedPayload{
		Summary:       wm.ContextSummary,
		Clipboard:     wm.RecentClipboard,
		ClipboardKind: wm.ClipboardKind,
		RecentEventID: wm.RecentEventIDs,
	})
	return p.Bus.Publish(ctx, ctxEvent)
}

type IntentClassifier struct {
	Bus       eventbus.Bus
	LogRepo   logstate.Repository
	SessionID string
}

func (p *IntentClassifier) ID() string {
	return "cognition.intent_classifier"
}

func (p *IntentClassifier) SubscribeTypes() []string {
	return []string{protocol.EventTypeUserInputReceived}
}

func (p *IntentClassifier) Handle(ctx context.Context, event protocol.Event) error {
	payload, ok := event.Payload.(protocol.UserInputPayload)
	if !ok {
		return nil
	}

	intent, entities := classifyIntent(payload)
	intentEvent := protocol.DeriveEvent(event, protocol.EventTypeIntentRecognized, "cognition.intent_classifier", protocol.PriorityHigh, protocol.IntentRecognizedPayload{
		Intent:          intent,
		Confidence:      0.82,
		Entities:        entities,
		RawInput:        payload.Text,
		RequestedAction: payload.RequestedAction,
	})
	_ = appendLog(ctx, p.LogRepo, event, logstate.LevelInfo, "intent", "已识别意图", map[string]any{"intent": intent})
	return p.Bus.Publish(ctx, intentEvent)
}

type TaskPlanner struct {
	Bus         eventbus.Bus
	SessionRepo session.Repository
	TaskRepo    task.Repository
	MemoryRepo  memory.Repository
	LogRepo     logstate.Repository
	SessionID   string
}

func (p *TaskPlanner) ID() string {
	return "cognition.task_planner"
}

func (p *TaskPlanner) SubscribeTypes() []string {
	return []string{protocol.EventTypeIntentRecognized}
}

func (p *TaskPlanner) Handle(ctx context.Context, event protocol.Event) error {
	payload, ok := event.Payload.(protocol.IntentRecognizedPayload)
	if !ok {
		return nil
	}

	taskID := id.New("task")
	planID := id.New("plan")
	stepID := id.New("step")
	now := time.Now()
	newTask := &task.Task{
		ID:        taskID,
		SessionID: p.SessionID,
		Kind:      taskKindForIntent(payload.Intent),
		Title:     taskTitleForIntent(payload.Intent, payload.RawInput),
		Goal:      taskGoalForIntent(payload.Intent, payload.RawInput),
		Status:    task.StatusPlanned,
		Priority:  1,
		Planner:   "rule_based",
		Input:     cloneMap(payload.Entities),
		Output:    map[string]any{},
		CreatedAt: now,
		UpdatedAt: now,
		TraceID:   event.TraceID,
		PlanID:    &planID,
		MaxRetry:  2,
		Metadata:  map[string]string{"source_event": event.ID, "intent": payload.Intent},
		Steps: []task.TaskStep{
			{
				ID:          stepID,
				Index:       0,
				Name:        "prepare_execution",
				Description: "prepare tool call",
				Status:      task.StepPending,
				Input:       cloneMap(payload.Entities),
			},
		},
	}
	newTask.Input["action"] = payload.Intent

	if err := p.TaskRepo.Create(ctx, newTask); err != nil {
		return err
	}
	if err := touchSessionTask(ctx, p.SessionRepo, p.MemoryRepo, p.SessionID, taskID, "add"); err != nil {
		return err
	}
	_ = appendLog(ctx, p.LogRepo, event, logstate.LevelInfo, "task", "已创建任务", map[string]any{"task_id": taskID, "title": newTask.Title})

	createdEvent := protocol.DeriveEvent(event, protocol.EventTypeTaskCreated, "cognition.task_planner", protocol.PriorityHigh, protocol.TaskCreatedPayload{
		Kind:  string(newTask.Kind),
		Title: newTask.Title,
		Goal:  newTask.Goal,
	})
	createdEvent.TaskID = &taskID

	plannedEvent := protocol.DeriveEvent(event, protocol.EventTypeTaskPlanned, "cognition.task_planner", protocol.PriorityHigh, protocol.TaskPlannedPayload{
		PlanID: planID,
		Steps: []protocol.PlannedStep{
			{Name: "prepare_execution", Description: "prepare tool call"},
		},
	})
	plannedEvent.TaskID = &taskID

	if err := p.Bus.Publish(ctx, createdEvent); err != nil {
		return err
	}
	return p.Bus.Publish(ctx, plannedEvent)
}

type Director struct {
	Bus      eventbus.Bus
	TaskRepo task.Repository
	Agents   *agents.Registry
	LogRepo  logstate.Repository
}

func (p *Director) ID() string {
	return "cognition.director"
}

func (p *Director) SubscribeTypes() []string {
	return []string{protocol.EventTypeTaskPlanned}
}

func (p *Director) Handle(ctx context.Context, event protocol.Event) error {
	if event.TaskID == nil {
		return nil
	}
	item, err := p.TaskRepo.GetByID(ctx, *event.TaskID)
	if err != nil || item == nil {
		return err
	}
	matches := p.Agents.Match(*item)
	if len(matches) == 0 {
		return fmt.Errorf("no agent can handle task %s", item.ID)
	}
	agentName := matches[0].Name()
	item.AssigneeAgent = &agentName
	item.UpdatedAt = time.Now()
	if len(item.Steps) > 0 {
		item.Steps[0].AgentName = &agentName
	}
	if err := p.TaskRepo.Update(ctx, item); err != nil {
		return err
	}
	_ = appendLog(ctx, p.LogRepo, event, logstate.LevelInfo, "director", "任务已分配", map[string]any{"task_id": item.ID, "agent": agentName})

	assignedEvent := protocol.DeriveEvent(event, protocol.EventTypeTaskAssigned, "cognition.director", protocol.PriorityHigh, protocol.TaskAssignedPayload{
		AgentName: agentName,
	})
	assignedEvent.TaskID = &item.ID
	return p.Bus.Publish(ctx, assignedEvent)
}

type VideoSuggestionProcessor struct {
	Bus          eventbus.Bus
	ApprovalRepo approval.Repository
	SessionRepo  session.Repository
	MemoryRepo   memory.Repository
	LogRepo      logstate.Repository
	SessionID    string
}

func (p *VideoSuggestionProcessor) ID() string {
	return "cognition.video_suggestion_processor"
}

func (p *VideoSuggestionProcessor) SubscribeTypes() []string {
	return []string{protocol.EventTypeVideoLinkDetected}
}

func (p *VideoSuggestionProcessor) Handle(ctx context.Context, event protocol.Event) error {
	payload, ok := event.Payload.(protocol.VideoLinkDetectedPayload)
	if !ok {
		return nil
	}

	pending, err := p.ApprovalRepo.ListPendingBySession(ctx, p.SessionID)
	if err != nil {
		return err
	}
	for _, existing := range pending {
		if existing.ActionType == "video_summary" && existing.ProposedArgs["url"] == payload.URL {
			return nil
		}
	}

	approvalID := id.New("approval")
	timeout := time.Now().Add(15 * time.Minute)
	item := &approval.Approval{
		ID:            approvalID,
		SessionID:     p.SessionID,
		Status:        approval.StatusPending,
		Risk:          approval.RiskMedium,
		ActionType:    "video_summary",
		ActionSummary: "检测到视频链接，是否生成视频总结？",
		ProposedArgs:  map[string]any{"url": payload.URL, "action": "video_summary"},
		RequestedBy:   "perception.clipboard",
		RequestedAt:   time.Now(),
		TimeoutAt:     &timeout,
		TraceID:       event.TraceID,
	}
	if err := p.ApprovalRepo.Create(ctx, item); err != nil {
		return err
	}
	if err := touchApproval(ctx, p.SessionRepo, p.MemoryRepo, p.SessionID, approvalID, "add"); err != nil {
		return err
	}
	_ = appendLog(ctx, p.LogRepo, event, logstate.LevelAudit, "approval", "已请求视频总结审批", map[string]any{"approval_id": approvalID, "url": payload.URL})

	out := protocol.DeriveEvent(event, protocol.EventTypeApprovalRequested, "cognition.video_suggestion_processor", protocol.PriorityHigh, protocol.ApprovalRequestedPayload{
		ActionType:    item.ActionType,
		ActionSummary: item.ActionSummary,
		ProposedArgs:  item.ProposedArgs,
		RiskLevel:     string(item.Risk),
		TimeoutAt:     item.TimeoutAt,
	})
	out.ApprovalID = &approvalID
	return p.Bus.Publish(ctx, out)
}

func classifyIntent(payload protocol.UserInputPayload) (string, map[string]any) {
	entities := cloneMap(payload.Metadata)
	if payload.RequestedAction != "" {
		if payload.Text != "" {
			entities["text"] = payload.Text
		}
		return payload.RequestedAction, entities
	}

	text := strings.TrimSpace(payload.Text)
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "翻译") || strings.Contains(lower, "translate"):
		entities["text"] = text
		return "translate", entities
	case strings.Contains(lower, "总结") || strings.Contains(lower, "summary"):
		entities["text"] = text
		return "summary", entities
	case strings.Contains(lower, "解释") || strings.Contains(lower, "explain"):
		entities["text"] = text
		return "explain", entities
	case strings.Contains(lower, "下一步") || strings.Contains(lower, "next"):
		entities["text"] = text
		return "next_steps", entities
	case strings.Contains(lower, "待办") || strings.Contains(lower, "todo"):
		return "todo_scan", entities
	default:
		entities["text"] = text
		return "explain", entities
	}
}

func taskKindForIntent(intent string) task.Kind {
	switch intent {
	case "video_summary":
		return task.KindResearch
	case "todo_scan":
		return task.KindWorkflow
	case "translate", "summary", "explain", "next_steps":
		return task.KindWriting
	default:
		return task.KindQuestionAnswer
	}
}

func taskTitleForIntent(intent, raw string) string {
	switch intent {
	case "summary":
		return "总结内容"
	case "translate":
		return "翻译内容"
	case "explain":
		return "解释内容"
	case "next_steps":
		return "生成下一步建议"
	case "todo_scan":
		return "扫描 Markdown 待办"
	case "video_summary":
		return "生成视频总结"
	default:
		if raw == "" {
			return "处理输入"
		}
		return "处理输入: " + trimForTitle(raw)
	}
}

func taskGoalForIntent(intent, raw string) string {
	switch intent {
	case "summary":
		return "为当前文本生成简明摘要"
	case "translate":
		return "将当前文本翻译成目标语言"
	case "explain":
		return "解释当前内容并提炼关键点"
	case "next_steps":
		return "给出清晰可执行的下一步建议"
	case "todo_scan":
		return "扫描配置目录中的 Markdown 待办项"
	case "video_summary":
		return "为复制到剪贴板的视频链接生成结构化总结"
	default:
		return "处理用户输入：" + trimForTitle(raw)
	}
}

func buildClipboardSuggestions(payload protocol.ClipboardChangedPayload) []memory.Suggestion {
	suggestions := []memory.Suggestion{
		{
			ID:          id.New("sug"),
			Kind:        "quick_action",
			Title:       "总结复制内容",
			Description: "快速提炼重点",
			Action:      "summary",
			CreatedAt:   time.Now(),
		},
		{
			ID:          id.New("sug"),
			Kind:        "quick_action",
			Title:       "翻译复制内容",
			Description: "生成翻译结果",
			Action:      "translate",
			CreatedAt:   time.Now(),
		},
		{
			ID:          id.New("sug"),
			Kind:        "quick_action",
			Title:       "解释复制内容",
			Description: "解释当前片段",
			Action:      "explain",
			CreatedAt:   time.Now(),
		},
		{
			ID:          id.New("sug"),
			Kind:        "quick_action",
			Title:       "问下一步做什么",
			Description: "输出行动建议",
			Action:      "next_steps",
			CreatedAt:   time.Now(),
		},
	}
	if payload.IsVideoURL {
		suggestions = append([]memory.Suggestion{{
			ID:          id.New("sug"),
			Kind:        "approval_hint",
			Title:       "视频链接已识别",
			Description: "审批后可生成视频总结",
			Action:      "video_summary",
			CreatedAt:   time.Now(),
			Payload:     map[string]string{"url": payload.Text},
		}}, suggestions...)
	}
	return suggestions
}

func summarizeContext(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "当前没有可用上下文。"
	}
	if len(text) > 120 {
		text = text[:120] + "..."
	}
	return "当前焦点：" + text
}

func ensureWorkingMemory(ctx context.Context, repo memory.Repository, sessionID string) (*memory.WorkingMemory, error) {
	wm, err := repo.GetWorkingSnapshot(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if wm != nil {
		return wm, nil
	}
	now := time.Now()
	return &memory.WorkingMemory{
		ID:        id.New("wm"),
		SessionID: sessionID,
		UpdatedAt: now,
	}, nil
}

func appendRecentEvent(events []string, eventID string) []string {
	events = append(events, eventID)
	if len(events) > 20 {
		events = events[len(events)-20:]
	}
	return events
}

func ptr[T any](value T) *T {
	return &value
}

func touchSessionTask(ctx context.Context, sessionRepo session.Repository, memoryRepo memory.Repository, sessionID, taskID, mode string) error {
	item, err := sessionRepo.GetByID(ctx, sessionID)
	if err != nil || item == nil {
		return err
	}
	switch mode {
	case "add":
		item.ActiveTaskIDs = appendUnique(item.ActiveTaskIDs, taskID)
		item.CurrentTaskID = &taskID
	case "remove":
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
	if mode == "add" {
		wm.ActiveTaskIDs = appendUnique(wm.ActiveTaskIDs, taskID)
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
	switch mode {
	case "add":
		item.PendingApprovalIDs = appendUnique(item.PendingApprovalIDs, approvalID)
	case "remove":
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
	if mode == "add" {
		wm.PendingApprovalIDs = appendUnique(wm.PendingApprovalIDs, approvalID)
	}
	if mode == "remove" {
		wm.PendingApprovalIDs = removeValue(wm.PendingApprovalIDs, approvalID)
	}
	wm.UpdatedAt = time.Now()
	return memoryRepo.SaveWorkingSnapshot(ctx, wm)
}

func appendUnique(values []string, candidate string) []string {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
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

func trimForTitle(text string) string {
	text = strings.TrimSpace(text)
	if len(text) > 32 {
		return text[:32] + "..."
	}
	return text
}

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
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

func SortedPendingSuggestions(items []memory.Suggestion) []memory.Suggestion {
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items
}
