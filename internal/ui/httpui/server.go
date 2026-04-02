package httpui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cialloclaw/internal/action/tooling"
	"cialloclaw/internal/config"
	"cialloclaw/internal/protocol"
	"cialloclaw/internal/runtime/eventbus"
	"cialloclaw/internal/runtime/id"
	"cialloclaw/internal/state/approval"
	logstate "cialloclaw/internal/state/log"
	"cialloclaw/internal/state/memory"
	"cialloclaw/internal/state/session"
	"cialloclaw/internal/state/task"
)

type Server struct {
	Addr             string
	Bus              eventbus.Bus
	CurrentSessionID func() string
	CurrentConfig    func() config.Config
	UpdateLLMConfig  func(config.LLMConfig) error
	SessionRepo      session.Repository
	TaskRepo         task.Repository
	ApprovalRepo     approval.Repository
	MemoryRepo       memory.Repository
	LogRepo          logstate.Repository
	Tools            *tooling.Registry
	TodoRoots        []string
	server           *http.Server
}

type modelConfigView struct {
	BaseURL      string `json:"base_url"`
	Model        string `json:"model"`
	APIKeyMasked string `json:"api_key_masked"`
	Configured   bool   `json:"configured"`
}

type stateSnapshot struct {
	Session     *session.Session         `json:"session,omitempty"`
	Working     *memory.WorkingMemory    `json:"working_memory,omitempty"`
	Tasks       []*task.Task             `json:"tasks"`
	Approvals   []*approval.Approval     `json:"approvals"`
	Logs        []*logstate.Entry        `json:"logs"`
	Episodes    []*memory.EpisodicMemory `json:"episodes"`
	Tools       []tooling.Metadata       `json:"tools"`
	ModelConfig modelConfigView          `json:"model_config"`
	ServerTime  time.Time                `json:"server_time"`
}

type chatRequest struct {
	Text   string `json:"text"`
	Action string `json:"action"`
}

type approvalRequest struct {
	ApprovalID string  `json:"approval_id"`
	Decision   string  `json:"decision"`
	Reason     *string `json:"reason,omitempty"`
}

type taskControlRequest struct {
	TaskID string `json:"task_id"`
	Action string `json:"action"`
}

type llmConfigRequest struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
}

func New(
	addr string,
	bus eventbus.Bus,
	currentSessionID func() string,
	currentConfig func() config.Config,
	updateLLMConfig func(config.LLMConfig) error,
	sessionRepo session.Repository,
	taskRepo task.Repository,
	approvalRepo approval.Repository,
	memoryRepo memory.Repository,
	logRepo logstate.Repository,
	tools *tooling.Registry,
	todoRoots []string,
) *Server {
	return &Server{
		Addr:             addr,
		Bus:              bus,
		CurrentSessionID: currentSessionID,
		CurrentConfig:    currentConfig,
		UpdateLLMConfig:  updateLLMConfig,
		SessionRepo:      sessionRepo,
		TaskRepo:         taskRepo,
		ApprovalRepo:     approvalRepo,
		MemoryRepo:       memoryRepo,
		LogRepo:          logRepo,
		Tools:            tools,
		TodoRoots:        todoRoots,
	}
}

func (s *Server) Name() string {
	return "ui.http"
}

func (s *Server) Start(_ context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/state", s.handleState)
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/todo/scan", s.handleTodoScan)
	mux.HandleFunc("/api/approvals/respond", s.handleApprovalRespond)
	mux.HandleFunc("/api/tasks/control", s.handleTaskControl)
	mux.HandleFunc("/api/config/llm", s.handleLLMConfigUpdate)

	s.server = &http.Server{
		Addr:    s.Addr,
		Handler: mux,
	}
	go func() {
		_ = s.server.ListenAndServe()
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

func (s *Server) handleIndex(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func (s *Server) handleState(w http.ResponseWriter, _ *http.Request) {
	ctx := context.Background()
	sessionID := s.CurrentSessionID()
	var currentSession *session.Session
	if sessionID != "" {
		currentSession, _ = s.SessionRepo.GetByID(ctx, sessionID)
	}
	working, _ := s.MemoryRepo.GetWorkingSnapshot(ctx, sessionID)
	tasks, _ := s.TaskRepo.ListBySession(ctx, sessionID)
	approvals, _ := s.ApprovalRepo.ListAll(ctx)
	logs, _ := s.LogRepo.Query(ctx, logstate.Query{SessionID: ptr(sessionID), Limit: 80})
	episodes, _ := s.MemoryRepo.ListEpisodes(ctx)
	cfg := s.CurrentConfig()

	writeJSON(w, http.StatusOK, stateSnapshot{
		Session:   currentSession,
		Working:   working,
		Tasks:     tasks,
		Approvals: approvals,
		Logs:      logs,
		Episodes:  episodes,
		Tools:     s.Tools.List(),
		ModelConfig: modelConfigView{
			BaseURL:      cfg.LLM.BaseURL,
			Model:        cfg.LLM.Model,
			APIKeyMasked: cfg.LLM.MaskedAPIKey(),
			Configured:   cfg.LLM.Configured(),
		},
		ServerTime: time.Now(),
	})
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "不支持该请求方法"})
		return
	}
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	sessionID := s.CurrentSessionID()
	event := protocol.NewEvent(protocol.EventTypeUserInputReceived, "ui.http", sessionID, protocol.PriorityCritical, protocol.UserInputPayload{
		Text:            req.Text,
		Raw:             req.Text,
		InputSource:     "ui.chat",
		RequestedAction: req.Action,
	})
	if err := s.Bus.Publish(r.Context(), event); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "已提交"})
}

func (s *Server) handleTodoScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "不支持该请求方法"})
		return
	}
	sessionID := s.CurrentSessionID()
	event := protocol.NewEvent(protocol.EventTypeUserInputReceived, "ui.http", sessionID, protocol.PriorityHigh, protocol.UserInputPayload{
		InputSource:     "ui.todo_scan",
		RequestedAction: "todo_scan",
		Metadata: map[string]any{
			"roots": s.TodoRoots,
		},
	})
	if err := s.Bus.Publish(r.Context(), event); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "已开始扫描"})
}

func (s *Server) handleApprovalRespond(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "不支持该请求方法"})
		return
	}
	var req approvalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	sessionID := s.CurrentSessionID()
	event := protocol.NewEvent(protocol.EventTypeApprovalResponded, "ui.http", sessionID, protocol.PriorityCritical, protocol.ApprovalRespondedPayload{
		Decision: req.Decision,
		Reason:   req.Reason,
	})
	event.ApprovalID = &req.ApprovalID
	if err := s.Bus.Publish(r.Context(), event); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "审批状态已更新"})
}

func (s *Server) handleTaskControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "不支持该请求方法"})
		return
	}
	var req taskControlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	item, err := s.TaskRepo.GetByID(r.Context(), req.TaskID)
	if err != nil || item == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "任务不存在"})
		return
	}
	now := time.Now()
	switch req.Action {
	case "pause":
		item.Status = task.StatusPaused
	case "resume":
		item.Status = task.StatusRunning
		if item.CompletedAt == nil && item.AssigneeAgent != nil {
			requeue := protocol.NewEvent(protocol.EventTypeTaskAssigned, "ui.http", item.SessionID, protocol.PriorityHigh, protocol.TaskAssignedPayload{
				AgentName: *item.AssigneeAgent,
			})
			requeue.TaskID = &item.ID
			_ = s.Bus.Publish(r.Context(), requeue)
		}
	case "cancel":
		item.Status = task.StatusCanceled
		item.CompletedAt = &now
		if sessionItem, sessionErr := s.SessionRepo.GetByID(r.Context(), item.SessionID); sessionErr == nil && sessionItem != nil {
			sessionItem.ActiveTaskIDs = removeValue(sessionItem.ActiveTaskIDs, item.ID)
			if sessionItem.CurrentTaskID != nil && *sessionItem.CurrentTaskID == item.ID {
				sessionItem.CurrentTaskID = nil
			}
			_ = s.SessionRepo.Update(r.Context(), sessionItem)
		}
		if wm, wmErr := ensureWorkingMemory(r.Context(), s.MemoryRepo, item.SessionID); wmErr == nil {
			wm.ActiveTaskIDs = removeValue(wm.ActiveTaskIDs, item.ID)
			_ = s.MemoryRepo.SaveWorkingSnapshot(r.Context(), wm)
		}
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "不支持的任务操作"})
		return
	}
	item.UpdatedAt = now
	if err := s.TaskRepo.Update(r.Context(), item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = s.LogRepo.Append(r.Context(), &logstate.Entry{
		ID:        id.New("log"),
		Timestamp: now,
		Level:     logstate.LevelInfo,
		SessionID: &item.SessionID,
		TaskID:    &item.ID,
		TraceID:   id.New("trace"),
		SpanID:    id.New("span"),
		Category:  "ui.task_control",
		Message:   fmt.Sprintf("任务操作：%s", req.Action),
	})
	writeJSON(w, http.StatusOK, map[string]string{"status": "任务状态已更新"})
}

func (s *Server) handleLLMConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "不支持该请求方法"})
		return
	}
	var req llmConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	next := config.LLMConfig{
		BaseURL: req.BaseURL,
		APIKey:  req.APIKey,
		Model:   req.Model,
	}
	if err := s.UpdateLLMConfig(next); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	cfg := s.CurrentConfig()
	writeJSON(w, http.StatusOK, modelConfigView{
		BaseURL:      cfg.LLM.BaseURL,
		Model:        cfg.LLM.Model,
		APIKeyMasked: cfg.LLM.MaskedAPIKey(),
		Configured:   cfg.LLM.Configured(),
	})
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

func removeValue(values []string, candidate string) []string {
	var out []string
	for _, value := range values {
		if value != candidate {
			out = append(out, value)
		}
	}
	return out
}

func ptr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

const indexHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>CialloClaw 控制面板</title>
  <style>
    :root {
      --paper: #f4efe4;
      --ink: #1e2a2f;
      --accent: #d96c3f;
      --line: rgba(30,42,47,0.12);
      --panel: rgba(255,255,255,0.78);
      --shadow: 0 20px 60px rgba(37, 49, 55, 0.12);
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "Microsoft YaHei", "PingFang SC", "Segoe UI", sans-serif;
      color: var(--ink);
      background:
        radial-gradient(circle at top right, rgba(217,108,63,0.18), transparent 30%),
        linear-gradient(180deg, #f9f4ea 0%, #efe5d4 100%);
      min-height: 100vh;
    }
    main {
      width: min(1400px, calc(100vw - 32px));
      margin: 24px auto 48px;
      display: grid;
      grid-template-columns: 1.45fr 1fr;
      gap: 18px;
    }
    .hero, .panel {
      background: var(--panel);
      backdrop-filter: blur(14px);
      border: 1px solid var(--line);
      border-radius: 24px;
      box-shadow: var(--shadow);
    }
    .hero {
      grid-column: 1 / -1;
      padding: 24px;
      display: grid;
      grid-template-columns: 1.15fr 1fr;
      gap: 18px;
      position: relative;
      overflow: hidden;
    }
    .hero::after {
      content: "";
      position: absolute;
      right: -30px;
      top: -30px;
      width: 200px;
      height: 200px;
      border-radius: 999px;
      background: linear-gradient(135deg, rgba(217,108,63,0.16), rgba(30,42,47,0.04));
      filter: blur(8px);
    }
    h1, h2, h3 {
      margin: 0 0 12px;
      font-family: Georgia, "Times New Roman", serif;
      font-weight: 700;
      letter-spacing: 0.02em;
    }
    p { margin: 0; line-height: 1.6; }
    .stack { display: grid; gap: 18px; }
    .panel { padding: 18px; }
    .statbar {
      display: grid;
      grid-template-columns: repeat(5, 1fr);
      gap: 12px;
      margin-top: 18px;
    }
    .stat {
      padding: 14px;
      border-radius: 18px;
      background: rgba(255,255,255,0.7);
      border: 1px solid var(--line);
    }
    .label { font-size: 12px; letter-spacing: 0.08em; opacity: 0.68; }
    .value { margin-top: 8px; font-size: 18px; font-weight: 700; }
    textarea, select, input, button { font: inherit; }
    textarea, select, input {
      width: 100%;
      padding: 12px 14px;
      border-radius: 16px;
      border: 1px solid var(--line);
      background: rgba(255,255,255,0.88);
      color: var(--ink);
    }
    textarea { min-height: 110px; resize: vertical; }
    button {
      border: none;
      border-radius: 999px;
      padding: 10px 16px;
      background: var(--ink);
      color: white;
      cursor: pointer;
      transition: transform 160ms ease;
    }
    button.alt {
      background: rgba(30,42,47,0.08);
      color: var(--ink);
      border: 1px solid var(--line);
    }
    button:hover { transform: translateY(-1px); }
    .actions {
      display: flex;
      gap: 10px;
      flex-wrap: wrap;
      margin-top: 12px;
    }
    .chips {
      display: flex;
      gap: 10px;
      flex-wrap: wrap;
      margin-top: 10px;
    }
    .chip {
      padding: 9px 12px;
      border-radius: 999px;
      background: rgba(217,108,63,0.12);
      border: 1px solid rgba(217,108,63,0.22);
      cursor: pointer;
    }
    .list {
      display: grid;
      gap: 10px;
      margin-top: 12px;
      max-height: 480px;
      overflow: auto;
    }
    .item {
      border: 1px solid var(--line);
      border-radius: 16px;
      padding: 14px;
      background: rgba(255,255,255,0.82);
    }
    .item small { opacity: 0.72; }
    .item .actions { margin-top: 10px; }
    .mono {
      font-family: "Consolas", "Courier New", monospace;
      white-space: pre-wrap;
      word-break: break-word;
      font-size: 13px;
      line-height: 1.5;
    }
    .config-grid {
      display: grid;
      gap: 10px;
      margin-top: 10px;
    }
    .hint {
      margin-top: 8px;
      font-size: 12px;
      opacity: 0.72;
    }
    .status-ok { color: #1f7a1f; }
    .status-bad { color: #b54b2e; }
    @media (max-width: 980px) {
      main, .hero { grid-template-columns: 1fr; }
      .statbar { grid-template-columns: repeat(2, 1fr); }
    }
  </style>
</head>
<body>
  <main>
    <section class="hero">
      <div>
        <h1>CialloClaw 本地桌面 Agent</h1>
        <p>当前主干已经接通：会话、任务、审批、记忆、剪贴板、待办巡检，以及基于 OpenAI 兼容接口的大模型内容处理。你现在可以直接在本页配置 Base URL、API Key、Model。</p>
        <div class="statbar" id="statbar"></div>
      </div>
      <div class="panel" style="background: rgba(255,255,255,0.68);">
        <h2>聊天与快捷动作</h2>
        <select id="action">
          <option value="">自动判断</option>
          <option value="summary">总结</option>
          <option value="translate">翻译</option>
          <option value="explain">解释</option>
          <option value="next_steps">下一步建议</option>
        </select>
        <textarea id="input" placeholder="输入问题，或者留空后直接点快捷动作使用当前剪贴板。"></textarea>
        <div class="actions">
          <button onclick="sendChat()">提交</button>
          <button class="alt" onclick="sendClipboardAction('summary')">总结剪贴板</button>
          <button class="alt" onclick="sendClipboardAction('translate')">翻译剪贴板</button>
          <button class="alt" onclick="scanTodos()">扫描待办</button>
        </div>
      </div>
    </section>

    <div class="stack">
      <section class="panel">
        <h2>模型设置</h2>
        <div class="config-grid">
          <input id="baseUrl" placeholder="Base URL，例如 https://api.openai.com/v1">
          <input id="apiKey" type="password" placeholder="API Key，留空则保留当前值">
          <input id="model" placeholder="Model，例如 gpt-4.1-mini 或你自己的兼容模型">
        </div>
        <div class="actions">
          <button onclick="saveModelConfig()">保存模型配置</button>
        </div>
        <div id="modelStatus" class="hint"></div>
      </section>

      <section class="panel">
        <h2>剪贴板与建议</h2>
        <div id="clipboard" class="mono">等待剪贴板内容…</div>
        <div class="chips" id="suggestions"></div>
      </section>

      <section class="panel">
        <h2>任务面板</h2>
        <div class="list" id="tasks"></div>
      </section>
    </div>

    <div class="stack">
      <section class="panel">
        <h2>审批</h2>
        <div class="list" id="approvals"></div>
      </section>

      <section class="panel">
        <h2>待办巡检</h2>
        <div id="todo" class="mono">暂无数据</div>
      </section>

      <section class="panel">
        <h2>运行日志</h2>
        <div class="list" id="logs"></div>
      </section>
    </div>
  </main>

  <script>
    async function request(path, options = {}) {
      const response = await fetch(path, {
        headers: { 'Content-Type': 'application/json' },
        ...options
      });
      if (!response.ok) {
        const text = await response.text();
        throw new Error(text || 'request failed');
      }
      return response.json();
    }

    async function poll() {
      try {
        const state = await request('/api/state');
        render(state);
      } catch (error) {
        console.error(error);
      }
    }

    function render(state) {
      const session = state.session || {};
      const working = state.working_memory || {};
      const tasks = state.tasks || [];
      const approvals = (state.approvals || []).filter(item => item.status === 'pending');
      const logs = state.logs || [];
      const episodes = state.episodes || [];
      const modelConfig = state.model_config || {};

      document.getElementById('statbar').innerHTML = [
        stat('会话状态', session.status || '未知'),
        stat('活跃任务', (working.active_task_ids || []).length),
        stat('待处理审批', approvals.length),
        stat('记忆条目', episodes.length),
        stat('模型状态', modelConfig.configured ? '已配置' : '未配置')
      ].join('');

      document.getElementById('clipboard').textContent = working.recent_clipboard || '暂无剪贴板文本';
      document.getElementById('suggestions').innerHTML = (working.suggestions || []).map(item =>
        '<button class="chip" onclick="sendClipboardAction(\'' + item.action + '\')">' + item.title + '</button>'
      ).join('');

      document.getElementById('baseUrl').value = modelConfig.base_url || '';
      document.getElementById('model').value = modelConfig.model || '';
      document.getElementById('modelStatus').innerHTML = modelConfig.configured
        ? '<span class="status-ok">模型已配置</span>，当前 API Key：' + (modelConfig.api_key_masked || '已隐藏')
        : '<span class="status-bad">模型未配置</span>，请填写 Base URL、API Key、Model。';

      document.getElementById('tasks').innerHTML = tasks.map(task => {
        const step = task.steps && task.steps.length ? task.steps[0] : null;
        return '<div class="item">' +
          '<h3>' + task.title + '</h3>' +
          '<small>' + task.status + (step ? ' · ' + step.name + ' / ' + step.status : '') + '</small>' +
          '<p class="mono">' + (task.summary || task.goal || '') + '</p>' +
          '<div class="actions">' +
            '<button class="alt" onclick="taskControl(\'' + task.id + '\', \'pause\')">暂停</button>' +
            '<button class="alt" onclick="taskControl(\'' + task.id + '\', \'resume\')">恢复</button>' +
            '<button class="alt" onclick="taskControl(\'' + task.id + '\', \'cancel\')">终止</button>' +
          '</div>' +
        '</div>';
      }).join('') || '<div class="item">暂无任务</div>';

      document.getElementById('approvals').innerHTML = approvals.map(item =>
        '<div class="item">' +
          '<h3>' + item.action_summary + '</h3>' +
          '<small>风险等级：' + item.risk + '</small>' +
          '<p class="mono">' + JSON.stringify(item.proposed_args || {}, null, 2) + '</p>' +
          '<div class="actions">' +
            '<button onclick="approvalRespond(\'' + item.id + '\', \'approved\')">批准</button>' +
            '<button class="alt" onclick="approvalRespond(\'' + item.id + '\', \'rejected\')">拒绝</button>' +
          '</div>' +
        '</div>'
      ).join('') || '<div class="item">暂无待处理审批</div>';

      const todo = working.todo_overview;
      document.getElementById('todo').textContent = todo
        ? ('未完成：' + (todo.pending || []).length + '\n已完成：' + (todo.completed || []).length + '\n\n' +
           (todo.pending || []).slice(0, 6).map(item => '- ' + item.text + ' [' + item.file_path + ']').join('\n'))
        : '暂无数据';

      document.getElementById('logs').innerHTML = logs.map(item =>
        '<div class="item">' +
          '<small>' + item.timestamp + ' · ' + item.level + ' · ' + item.category + '</small>' +
          '<div class="mono">' + item.message + '</div>' +
        '</div>'
      ).join('') || '<div class="item">暂无日志</div>';
    }

    function stat(label, value) {
      return '<div class="stat"><div class="label">' + label + '</div><div class="value">' + value + '</div></div>';
    }

    async function sendChat() {
      const text = document.getElementById('input').value;
      const action = document.getElementById('action').value;
      await request('/api/chat', {
        method: 'POST',
        body: JSON.stringify({ text, action })
      });
      document.getElementById('input').value = '';
      setTimeout(poll, 300);
    }

    async function sendClipboardAction(action) {
      await request('/api/chat', {
        method: 'POST',
        body: JSON.stringify({ text: '', action })
      });
      setTimeout(poll, 300);
    }

    async function approvalRespond(approvalId, decision) {
      await request('/api/approvals/respond', {
        method: 'POST',
        body: JSON.stringify({ approval_id: approvalId, decision })
      });
      setTimeout(poll, 300);
    }

    async function taskControl(taskId, action) {
      await request('/api/tasks/control', {
        method: 'POST',
        body: JSON.stringify({ task_id: taskId, action })
      });
      setTimeout(poll, 300);
    }

    async function scanTodos() {
      await request('/api/todo/scan', { method: 'POST', body: '{}' });
      setTimeout(poll, 300);
    }

    async function saveModelConfig() {
      const baseUrl = document.getElementById('baseUrl').value.trim();
      const apiKey = document.getElementById('apiKey').value.trim();
      const model = document.getElementById('model').value.trim();
      await request('/api/config/llm', {
        method: 'POST',
        body: JSON.stringify({ base_url: baseUrl, api_key: apiKey, model: model })
      });
      document.getElementById('apiKey').value = '';
      setTimeout(poll, 300);
    }

    poll();
    setInterval(poll, 2500);
  </script>
</body>
</html>`
