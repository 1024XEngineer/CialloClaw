package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type PermissionBoundary struct {
	GreenActions  []string `json:"green_actions"`
	YellowActions []string `json:"yellow_actions"`
	RedActions    []string `json:"red_actions"`
}

type Agent struct {
	ID                 string             `json:"id"`
	Name               string             `json:"name"`
	Style              string             `json:"style"`
	MemoryBoundary     string             `json:"memory_boundary"`
	RouteBoundary      string             `json:"route_boundary"`
	PermissionBoundary PermissionBoundary `json:"permission_boundary"`
	Default            bool               `json:"default"`
}

type QuickTask struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	ShortLabel  string `json:"short_label"`
	Description string `json:"description"`
	Verb        string `json:"verb"`
	Level       int    `json:"level"`
}

type AppSettings struct {
	Language              string `json:"language"`
	AutoStart             bool   `json:"auto_start"`
	MinimizeToTray        bool   `json:"minimize_to_tray"`
	KeepRunningOnClose    bool   `json:"keep_running_on_close"`
	SystemNotifyEnabled   bool   `json:"system_notify_enabled"`
	SoundReminderEnabled  bool   `json:"sound_reminder_enabled"`
	ShowFloatingBall      bool   `json:"show_floating_ball"`
	AutoDock              bool   `json:"auto_dock"`
	IdleHalfTransparent   bool   `json:"idle_half_transparent"`
	BallSize              int    `json:"ball_size"`
	EnableTaskRing        bool   `json:"enable_task_ring"`
	ActiveAssistEnabled   bool   `json:"active_assist_enabled"`
	AssistLevelDefault    int    `json:"assist_level_default"`
	NudgeMinIntervalSec   int    `json:"nudge_min_interval_sec"`
	NudgeMaxPerHour       int    `json:"nudge_max_per_hour"`
	MemoryLifecycle       string `json:"memory_lifecycle"`
	InspectFrequency      string `json:"inspect_frequency"`
	InspectOnStartup      bool   `json:"inspect_on_startup"`
	InspectOnFileChanged  bool   `json:"inspect_on_file_changed"`
	DueReminderEnabled    bool   `json:"due_reminder_enabled"`
	LongPendingAlert      bool   `json:"long_pending_alert"`
	CurrentModel          string `json:"current_model"`
	CurrentProvider       string `json:"current_provider"`
	BudgetAutoDowngrade   bool   `json:"budget_auto_downgrade"`
	WorkspacePath         string `json:"workspace_path"`
	ProactiveOnlySafeMode bool   `json:"proactive_only_safe_mode"`
}

type NoteTask struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Status        string    `json:"status"`
	Priority      string    `json:"priority"`
	DueAt         time.Time `json:"due_at"`
	Tags          []string  `json:"tags"`
	SourceFile    string    `json:"source_file"`
	PendingDays   int       `json:"pending_days"`
	SuggestedNext string    `json:"suggested_next"`
}

type ChatRequest struct {
	SessionID string `json:"session_id"`
	AgentID   string `json:"agent_id"`
	Text      string `json:"text"`
}

type ChatResponse struct {
	Accepted bool   `json:"accepted"`
	RunID    string `json:"run_id"`
}

type DashboardSnapshot struct {
	BrowserFeed  []map[string]any `json:"browser_feed"`
	TerminalFeed []map[string]any `json:"terminal_feed"`
	FileFeed     []map[string]any `json:"file_feed"`
	DecisionFeed []map[string]any `json:"decision_feed"`
}

type SkillPackage struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Source      string `json:"source"`
	Version     string `json:"version"`
	Installed   bool   `json:"installed"`
	Description string `json:"description"`
	Scope       string `json:"scope"`
}

type SSEEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id,omitempty"`
	AgentID   string    `json:"agent_id,omitempty"`
	RunID     string    `json:"run_id,omitempty"`
	Level     int       `json:"level,omitempty"`
	Data      any       `json:"data,omitempty"`
}

type Broker struct {
	mu          sync.RWMutex
	subscribers map[chan SSEEvent]struct{}
}

func NewBroker() *Broker {
	return &Broker{subscribers: make(map[chan SSEEvent]struct{})}
}

func (b *Broker) Subscribe() chan SSEEvent {
	ch := make(chan SSEEvent, 64)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Broker) Unsubscribe(ch chan SSEEvent) {
	b.mu.Lock()
	if _, ok := b.subscribers[ch]; ok {
		delete(b.subscribers, ch)
		close(ch)
	}
	b.mu.Unlock()
}

func (b *Broker) Publish(ev SSEEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subscribers {
		select {
		case ch <- ev:
		default:
		}
	}
}

type Server struct {
	broker      *Broker
	agents      []Agent
	quickTasks  map[string][]QuickTask
	skills      []SkillPackage
	noteTasks   []NoteTask
	settings    AppSettings
	agentStates map[string]string
	mu          sync.RWMutex
	rnd         *rand.Rand
}

func NewServer() *Server {
	now := time.Now()
	return &Server{
		broker: NewBroker(),
		agents: []Agent{
			{
				ID:             "life-agent",
				Name:           "生活助手",
				Style:          "陪伴优先，轻提醒",
				MemoryBoundary: "personal-memory",
				RouteBoundary:  "生活场景",
				PermissionBoundary: PermissionBoundary{
					GreenActions:  []string{"总结", "翻译", "生成清单"},
					YellowActions: []string{"打开网站", "读取工作区文件"},
					RedActions:    []string{"执行命令", "修改敏感文件", "发送外部消息"},
				},
				Default: true,
			},
			{
				ID:             "work-agent",
				Name:           "工作助手",
				Style:          "效率优先，任务巡检",
				MemoryBoundary: "work-memory",
				RouteBoundary:  "开发与办公场景",
				PermissionBoundary: PermissionBoundary{
					GreenActions:  []string{"任务巡检", "每日摘要", "读取日志"},
					YellowActions: []string{"浏览网页", "写入普通文件"},
					RedActions:    []string{"删除文件", "执行脚本", "网络上传"},
				},
			},
		},
		quickTasks: map[string][]QuickTask{
			"life-agent": {
				{ID: "summary", Label: "总结当前内容", ShortLabel: "总结", Description: "提炼当前选中文本的关键点", Verb: "总结", Level: 1},
				{ID: "translate", Label: "翻译选中内容", ShortLabel: "翻译", Description: "将选中内容翻译为目标语言", Verb: "翻译", Level: 1},
				{ID: "explain", Label: "解释这段内容", ShortLabel: "解释", Description: "提供简洁解释与示例", Verb: "解释", Level: 1},
				{ID: "todo", Label: "记录为待办", ShortLabel: "待办", Description: "加入任务巡检队列", Verb: "待办", Level: 2},
				{ID: "next", Label: "建议下一步", ShortLabel: "下一步", Description: "给出下一步建议", Verb: "下一步", Level: 1},
				{ID: "more", Label: "更多动作", ShortLabel: "更多", Description: "打开动作库", Verb: "更多", Level: 0},
			},
			"work-agent": {
				{ID: "inspect", Label: "巡检任务便签", ShortLabel: "巡检", Description: "检查即将到期和阻塞任务", Verb: "巡检", Level: 2},
				{ID: "daily", Label: "生成每日摘要", ShortLabel: "日报", Description: "汇总今日任务", Verb: "日报", Level: 2},
				{ID: "error", Label: "分析错误日志", ShortLabel: "日志", Description: "读取错误输出并给出修复建议", Verb: "日志", Level: 2},
				{ID: "draft", Label: "生成消息草稿", ShortLabel: "草稿", Description: "生成沟通草稿", Verb: "草稿", Level: 3},
				{ID: "next", Label: "建议下一步", ShortLabel: "下一步", Description: "给出工程下一步建议", Verb: "下一步", Level: 1},
				{ID: "more", Label: "更多动作", ShortLabel: "更多", Description: "打开动作库", Verb: "更多", Level: 0},
			},
		},
		skills: []SkillPackage{
			{
				ID:          "skill-web-search",
				Name:        "网页搜索",
				Source:      "内置",
				Version:     "0.1.0",
				Installed:   true,
				Description: "搜索公开网页并提取证据片段",
				Scope:       "共享",
			},
			{
				ID:          "skill-note-inspector",
				Name:        "便签巡检",
				Source:      "内置",
				Version:     "0.1.0",
				Installed:   true,
				Description: "巡检 markdown 任务并计算到期风险",
				Scope:       "工作助手",
			},
			{
				ID:          "skill-github-fetch",
				Name:        "GitHub 技能加载器",
				Source:      "GitHub",
				Version:     "0.1.0",
				Installed:   false,
				Description: "从 GitHub 仓库安装社区技能",
				Scope:       "可选",
			},
		},
		noteTasks: []NoteTask{
			{
				ID:            "task-1",
				Title:         "提交周报",
				Status:        "待处理",
				Priority:      "P1",
				DueAt:         now.Add(2 * time.Hour),
				Tags:          []string{"周报", "工作"},
				SourceFile:    "notes/work.md",
				PendingDays:   1,
				SuggestedNext: "打开上周模板并填写里程碑",
			},
			{
				ID:            "task-2",
				Title:         "联系客户确认会议时间",
				Status:        "待处理",
				Priority:      "P1",
				DueAt:         now.Add(26 * time.Hour),
				Tags:          []string{"客户", "会议"},
				SourceFile:    "notes/crm.md",
				PendingDays:   3,
				SuggestedNext: "生成一条可直接发送的确认消息",
			},
			{
				ID:            "task-3",
				Title:         "整理截图归档",
				Status:        "已滞留",
				Priority:      "P3",
				DueAt:         now.Add(-8 * time.Hour),
				Tags:          []string{"运维", "归档"},
				SourceFile:    "notes/backlog.md",
				PendingDays:   5,
				SuggestedNext: "将重复任务转为自动化流程",
			},
		},
		settings: AppSettings{
			Language:              "zh-CN",
			AutoStart:             true,
			MinimizeToTray:        true,
			KeepRunningOnClose:    true,
			SystemNotifyEnabled:   true,
			SoundReminderEnabled:  false,
			ShowFloatingBall:      true,
			AutoDock:              true,
			IdleHalfTransparent:   true,
			BallSize:              96,
			EnableTaskRing:        true,
			ActiveAssistEnabled:   true,
			AssistLevelDefault:    1,
			NudgeMinIntervalSec:   45,
			NudgeMaxPerHour:       6,
			MemoryLifecycle:       "30d",
			InspectFrequency:      "15m",
			InspectOnStartup:      true,
			InspectOnFileChanged:  true,
			DueReminderEnabled:    true,
			LongPendingAlert:      true,
			CurrentModel:          "gpt-5.4-mini",
			CurrentProvider:       "本地模拟",
			BudgetAutoDowngrade:   true,
			WorkspacePath:         "D:/Code/GO/CialloClaw",
			ProactiveOnlySafeMode: true,
		},
		agentStates: map[string]string{
			"life-agent": "idle",
			"work-agent": "idle",
		},
		rnd: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/agents", s.handleAgents)
	mux.HandleFunc("/api/skills", s.handleSkills)
	mux.HandleFunc("/api/quick-tasks", s.handleQuickTasks)
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/api/tasks/inspect", s.handleInspectTasks)
	mux.HandleFunc("/api/dashboard/snapshot", s.handleDashboardSnapshot)
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/sse", s.handleSSE)

	return withCORS(withRequestLog(mux))
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "cialloclaw-go-core"})
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	agents := make([]Agent, 0, len(s.agents))
	agents = append(agents, s.agents...)
	sort.Slice(agents, func(i, j int) bool {
		if agents[i].Default {
			return true
		}
		return agents[i].Name < agents[j].Name
	})
	writeJSON(w, http.StatusOK, map[string]any{"agents": agents})
}

func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	skills := make([]SkillPackage, 0, len(s.skills))
	skills = append(skills, s.skills...)
	sort.Slice(skills, func(i, j int) bool {
		if skills[i].Installed != skills[j].Installed {
			return skills[i].Installed
		}
		return skills[i].Name < skills[j].Name
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"skills":        skills,
		"compatibility": "技能接口-v1",
	})
}

func (s *Server) handleQuickTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	agentID := strings.TrimSpace(r.URL.Query().Get("agent_id"))
	if agentID == "" {
		agentID = "life-agent"
	}
	tasks, ok := s.quickTasks[agentID]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "未找到助手"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agent_id": agentID, "tasks": tasks})
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, s.settings)
}

func (s *Server) handleInspectTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	noteTasks := make([]NoteTask, 0, len(s.noteTasks))
	noteTasks = append(noteTasks, s.noteTasks...)
	sort.Slice(noteTasks, func(i, j int) bool {
		return noteTasks[i].DueAt.Before(noteTasks[j].DueAt)
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"inspected_at": time.Now(),
		"total":        len(noteTasks),
		"tasks":        noteTasks,
	})
}

func (s *Server) handleDashboardSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}

	now := time.Now()
	snapshot := DashboardSnapshot{
		BrowserFeed: []map[string]any{
			{"url": "https://example.com/report", "action": "抓取", "note": "准备提取页面摘要", "timestamp": now.Add(-12 * time.Second)},
		},
		TerminalFeed: []map[string]any{
			{"cmd": "go test ./...", "stdout": "测试通过：module/core 0.86s", "exit_code": 0, "timestamp": now.Add(-8 * time.Second)},
		},
		FileFeed: []map[string]any{
			{"path": "notes/work.md", "action": "修改", "diff": "+ [ ] 完成周报草稿", "timestamp": now.Add(-5 * time.Second)},
		},
		DecisionFeed: []map[string]any{
			{"goal": "完成日报自动化", "next": "生成模板并请求确认", "risk": "需确认", "timestamp": now.Add(-3 * time.Second)},
		},
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请求体无效"})
		return
	}

	var req ChatRequest
	if err = json.Unmarshal(body, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "JSON 格式无效"})
		return
	}

	req.AgentID = strings.TrimSpace(req.AgentID)
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.Text = strings.TrimSpace(req.Text)
	if req.AgentID == "" {
		req.AgentID = "life-agent"
	}
	if req.SessionID == "" {
		req.SessionID = "session-default"
	}
	if req.Text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "请输入内容"})
		return
	}
	if !s.agentExists(req.AgentID) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "未知助手"})
		return
	}

	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())
	writeJSON(w, http.StatusAccepted, ChatResponse{Accepted: true, RunID: runID})
	go s.simulateRun(req, runID)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "当前环境不支持流式输出"})
		return
	}

	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	agentID := strings.TrimSpace(r.URL.Query().Get("agent_id"))

	headers := w.Header()
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	headers.Set("X-Accel-Buffering", "no")

	sub := s.broker.Subscribe()
	defer s.broker.Unsubscribe(sub)

	connected := SSEEvent{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Type:      "connected",
		Timestamp: time.Now(),
		SessionID: sessionID,
		AgentID:   agentID,
		Data:      map[string]any{"message": "事件流已连接"},
	}
	s.writeSSEEvent(w, flusher, connected)

	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-keepAlive.C:
			fmt.Fprint(w, ": heartbeat\n\n")
			flusher.Flush()
		case ev := <-sub:
			if sessionID != "" && ev.SessionID != "" && ev.SessionID != sessionID {
				continue
			}
			if agentID != "" && ev.AgentID != "" && ev.AgentID != agentID {
				continue
			}
			s.writeSSEEvent(w, flusher, ev)
		}
	}
}

func (s *Server) writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, ev SSEEvent) {
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	fmt.Fprintf(w, "id: %s\n", ev.ID)
	fmt.Fprintf(w, "event: %s\n", ev.Type)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func (s *Server) simulateRun(req ChatRequest, runID string) {
	startedAt := time.Now()
	s.setAgentState(req.AgentID, "thinking", req.SessionID, runID)

	s.publish(SSEEvent{
		ID:        s.newEventID(),
		Type:      "run.started",
		Timestamp: time.Now(),
		SessionID: req.SessionID,
		AgentID:   req.AgentID,
		RunID:     runID,
		Data: map[string]any{
			"goal": fmt.Sprintf("处理用户请求：%s", req.Text),
			"route": map[string]any{
				"agent":           req.AgentID,
				"memory_boundary": s.lookupAgent(req.AgentID).MemoryBoundary,
				"permission":      "需确认",
			},
		},
	})

	time.Sleep(500 * time.Millisecond)
	s.publish(SSEEvent{
		ID:        s.newEventID(),
		Type:      "timeline",
		Timestamp: time.Now(),
		SessionID: req.SessionID,
		AgentID:   req.AgentID,
		RunID:     runID,
		Data: map[string]any{
			"title":       "解析输入与上下文",
			"status":      "已完成",
			"duration_ms": 420,
		},
	})

	time.Sleep(650 * time.Millisecond)
	s.setAgentState(req.AgentID, "executing", req.SessionID, runID)
	s.publish(SSEEvent{
		ID:        s.newEventID(),
		Type:      "tool.call",
		Timestamp: time.Now(),
		SessionID: req.SessionID,
		AgentID:   req.AgentID,
		RunID:     runID,
		Data: map[string]any{
			"name":       "workspace.inspect",
			"args":       map[string]any{"mode": "本地模拟", "scope": "任务便签"},
			"status":     "成功",
			"elapsed_ms": 811,
		},
	})

	time.Sleep(650 * time.Millisecond)
	s.publish(SSEEvent{
		ID:        s.newEventID(),
		Type:      "citation",
		Timestamp: time.Now(),
		SessionID: req.SessionID,
		AgentID:   req.AgentID,
		RunID:     runID,
		Data: map[string]any{
			"title":   "任务便签巡检结果",
			"source":  "notes/work.md",
			"snippet": "今天还有 3 项未完成，其中 1 项将在 2 小时后到期。",
		},
	})

	time.Sleep(650 * time.Millisecond)
	s.publish(SSEEvent{
		ID:        s.newEventID(),
		Type:      "decision",
		Timestamp: time.Now(),
		SessionID: req.SessionID,
		AgentID:   req.AgentID,
		RunID:     runID,
		Data: map[string]any{
			"goal":   "形成可执行建议",
			"next":   "外部动作前请求用户确认",
			"risk":   "需确认",
			"reason": "涉及外部副作用",
		},
	})

	reply := buildAssistantReply(req.Text)
	time.Sleep(600 * time.Millisecond)
	s.publish(SSEEvent{
		ID:        s.newEventID(),
		Type:      "reasoning.summary",
		Timestamp: time.Now(),
		SessionID: req.SessionID,
		AgentID:   req.AgentID,
		RunID:     runID,
		Data: map[string]any{
			"summary":     reply,
			"duration_ms": time.Since(startedAt).Milliseconds(),
		},
	})

	s.publish(SSEEvent{
		ID:        s.newEventID(),
		Type:      "run.completed",
		Timestamp: time.Now(),
		SessionID: req.SessionID,
		AgentID:   req.AgentID,
		RunID:     runID,
		Data: map[string]any{
			"reply":       reply,
			"duration_ms": time.Since(startedAt).Milliseconds(),
		},
	})

	s.setAgentState(req.AgentID, "completed", req.SessionID, runID)
	time.Sleep(1300 * time.Millisecond)
	s.setAgentState(req.AgentID, "idle", req.SessionID, runID)
}

func (s *Server) setAgentState(agentID string, state string, sessionID string, runID string) {
	s.mu.Lock()
	s.agentStates[agentID] = state
	s.mu.Unlock()

	s.publish(SSEEvent{
		ID:        s.newEventID(),
		Type:      "agent.state",
		Timestamp: time.Now(),
		SessionID: sessionID,
		AgentID:   agentID,
		RunID:     runID,
		Data: map[string]any{
			"state": state,
		},
	})
}

func (s *Server) publish(ev SSEEvent) {
	s.broker.Publish(ev)
}

func (s *Server) newEventID() string {
	return fmt.Sprintf("evt-%d", time.Now().UnixNano())
}

func (s *Server) lookupAgent(agentID string) Agent {
	for _, agent := range s.agents {
		if agent.ID == agentID {
			return agent
		}
	}
	return s.agents[0]
}

func (s *Server) agentExists(agentID string) bool {
	for _, agent := range s.agents {
		if agent.ID == agentID {
			return true
		}
	}
	return false
}

func (s *Server) startBackgroundEvents(ctx context.Context) {
	heartbeatTicker := time.NewTicker(20 * time.Second)
	proactiveTicker := time.NewTicker(35 * time.Second)
	musicTicker := time.NewTicker(95 * time.Second)

	go func() {
		defer heartbeatTicker.Stop()
		defer proactiveTicker.Stop()
		defer musicTicker.Stop()

		reminders := []string{
			"你在同一窗口停留较久，要不要我帮你提炼重点？",
			"检测到任务临近截止，需要我按紧急度排序吗？",
			"你刚复制了一段长文本，要我整理成结构化要点吗？",
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-heartbeatTicker.C:
				s.publish(SSEEvent{
					ID:        s.newEventID(),
					Type:      "heartbeat",
					Timestamp: time.Now(),
					Data: map[string]any{
						"active_agents": len(s.agents),
						"status":        "正常",
						"message":       "心跳巡检完成",
					},
				})
			case <-proactiveTicker.C:
				level := 1 + s.rnd.Intn(2)
				s.publish(SSEEvent{
					ID:        s.newEventID(),
					Type:      "reminder.suggested",
					Timestamp: time.Now(),
					AgentID:   s.pickAgentID(),
					Level:     level,
					Data: map[string]any{
						"title":   "主动协助提醒",
						"message": reminders[s.rnd.Intn(len(reminders))],
						"actions": []string{"执行", "稍后", "关闭此类"},
					},
				})
			case <-musicTicker.C:
				s.publish(SSEEvent{
					ID:        s.newEventID(),
					Type:      "agent.state",
					Timestamp: time.Now(),
					AgentID:   s.pickAgentID(),
					Data: map[string]any{
						"state": "music",
					},
				})
				time.Sleep(3 * time.Second)
				s.publish(SSEEvent{
					ID:        s.newEventID(),
					Type:      "agent.state",
					Timestamp: time.Now(),
					AgentID:   s.pickAgentID(),
					Data: map[string]any{
						"state": "idle",
					},
				})
			}
		}
	}()
}

func (s *Server) pickAgentID() string {
	if len(s.agents) == 0 {
		return ""
	}
	return s.agents[s.rnd.Intn(len(s.agents))].ID
}

func buildAssistantReply(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return "我已整理出行动草稿，可以先从最紧急项开始。"
	}
	if strings.Contains(text, "week") || strings.Contains(text, "report") || strings.Contains(text, "周报") || strings.Contains(text, "日报") {
		return "我提炼了里程碑、阻塞项和下一步建议，建议先确认截止时间再生成草稿。"
	}
	if strings.Contains(text, "error") || strings.Contains(text, "bug") || strings.Contains(text, "错误") || strings.Contains(text, "异常") {
		return "我已将问题分为环境与业务逻辑两类，建议先做最小复现并附上日志。"
	}
	return fmt.Sprintf("已收到：%s。我会先给出可执行步骤，并在外部动作前请求确认。", text)
}

func withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s (%s)", r.Method, r.URL.Path, time.Since(start).Truncate(time.Millisecond))
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("write json error: %v", err)
	}
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "请求方法不允许"})
}

func run() error {
	server := NewServer()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.startBackgroundEvents(ctx)

	httpServer := &http.Server{
		Addr:              ":18080",
		Handler:           server.routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("CialloClaw Go Core 已启动: http://127.0.0.1:18080")
	err := httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

