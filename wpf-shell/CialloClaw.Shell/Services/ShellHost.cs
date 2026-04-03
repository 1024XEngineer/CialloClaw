using System.Text.Json;
using System.Windows;
using CialloClaw.Shell.Models;
using CialloClaw.Shell.ViewModels;
using CialloClaw.Shell.Views;

namespace CialloClaw.Shell.Services;

public sealed class ShellHost
{
    private readonly CoreApiClient _apiClient;
    private readonly SseClient _sseClient;
    private readonly ReminderGate _reminderGate;
    private readonly VoiceInteractionService _voiceService;

    private readonly FloatingBallViewModel _floatingBallVm;
    private readonly ChatViewModel _chatVm;
    private readonly ControlPanelViewModel _controlPanelVm;
    private readonly ReminderViewModel _reminderVm;

    private readonly Dictionary<string, List<ChatMessage>> _chatHistoryByAgent = new();
    private readonly Dictionary<string, string> _summaryByRunId = new();
    private readonly Dictionary<string, List<QuickTask>> _availableQuickTasksByAgent = new();
    private readonly Dictionary<string, List<string>> _enabledRingTaskIdsByAgent = new();

    private readonly CancellationTokenSource _cts = new();

    private FloatingBallWindow? _floatingBallWindow;
    private ChatWindow? _chatWindow;
    private ControlPanelWindow? _controlPanelWindow;
    private ReminderWindow? _reminderWindow;

    private AppSettingsModel _settings = new();
    private string _activeAgentId = "life-agent";
    private bool _syncingAgentSelection;

    public ShellHost()
    {
        _apiClient = new CoreApiClient();
        _sseClient = new SseClient();
        _reminderGate = new ReminderGate();
        _voiceService = new VoiceInteractionService();

        _floatingBallVm = new FloatingBallViewModel();
        _chatVm = new ChatViewModel();
        _controlPanelVm = new ControlPanelViewModel();
        _reminderVm = new ReminderViewModel();

        _floatingBallVm.OpenChatRequested += ShowChat;
        _floatingBallVm.OpenControlPanelRequested += () => ShowControlPanel(true);
        _floatingBallVm.ExitRequested += () => Application.Current.Shutdown();
        _floatingBallVm.PauseToggled += TogglePause;
        _floatingBallVm.QuickTaskInvoked += OnQuickTaskInvoked;

        _chatVm.SendRequestedAsync = SendChatAsync;
        _chatVm.ToggleVoiceRequestedAsync = () => _voiceService.ToggleListeningAsync();
        _chatVm.SpeakRequestedAsync = text => _voiceService.SpeakAsync(text);
        _chatVm.StopSpeakingRequested = _voiceService.StopSpeaking;
        _chatVm.AgentChanged += agent => _ = SwitchAgentAsync(agent, AgentChangeSource.Chat);

        _controlPanelVm.InspectRequestedAsync = () => _apiClient.InspectTasksAsync(_cts.Token);
        _controlPanelVm.AgentChanged += agent => _ = SwitchAgentAsync(agent, AgentChangeSource.Panel);
        _controlPanelVm.RingTasksChanged += OnRingTasksChanged;

        _reminderVm.ExecuteRequested += OnReminderExecute;
        _reminderVm.LaterRequested += () => _reminderWindow?.Hide();
        _reminderVm.DisableRequested += () =>
        {
            _settings.ActiveAssistEnabled = false;
            _reminderWindow?.Hide();
            _chatVm.AppendSystemMessage("已关闭主动提醒，可在控制面板重新开启。");
        };

        _sseClient.EventReceived += OnSseEventReceived;
        _voiceService.RecognizedTextReceived += OnVoiceRecognizedText;
        _voiceService.StatusChanged += OnVoiceStatusChanged;
        _voiceService.ListeningStateChanged += OnVoiceListeningStateChanged;
        _voiceService.SpeakingStateChanged += OnVoiceSpeakingStateChanged;
    }

    public async Task StartAsync()
    {
        var agents = await SafeGetAgentsAsync();
        _chatVm.LoadAgents(agents);
        _controlPanelVm.LoadAgents(agents);

        _settings = await SafeGetSettingsAsync();
        _controlPanelVm.Settings = _settings;
        _floatingBallVm.BallSize = _settings.BallSize;

        var defaultAgent = agents.FirstOrDefault(item => item.Default) ?? agents.First();
        _activeAgentId = defaultAgent.Id;
        _floatingBallVm.CurrentAgentName = defaultAgent.DisplayName;

        _syncingAgentSelection = true;
        _chatVm.SelectedAgent = defaultAgent;
        _controlPanelVm.SelectedAgent = defaultAgent;
        _syncingAgentSelection = false;

        var availableTasks = await SafeGetQuickTasksAsync(_activeAgentId);
        _availableQuickTasksByAgent[_activeAgentId] = availableTasks.ToList();
        _controlPanelVm.LoadAvailableRingTasks(availableTasks);
        _controlPanelVm.LoadEnabledRingTasks([]);
        ApplyRingTasksForActiveAgent();

        var skills = await SafeGetSkillsAsync();
        _controlPanelVm.LoadSkills(skills);

        var snapshot = await SafeGetDashboardSnapshotAsync();
        _controlPanelVm.LoadDashboardSnapshot(snapshot);

        _floatingBallWindow = new FloatingBallWindow { DataContext = _floatingBallVm, CompanionDataContext = _chatVm };
        _chatWindow = new ChatWindow { DataContext = _chatVm };
        _controlPanelWindow = new ControlPanelWindow { DataContext = _controlPanelVm };
        _reminderWindow = new ReminderWindow { DataContext = _reminderVm };

        _chatVm.SetVoiceCapabilities(_voiceService.IsRecognitionAvailable, _voiceService.IsSpeechPlaybackAvailable, _voiceService.CurrentStatus);

        _floatingBallWindow.BallMoved += OnBallMoved;
        _floatingBallWindow.Show();

        if (_settings.InspectOnStartup)
        {
            _controlPanelVm.InspectTasksCommand.Execute(null);
        }

        _sseClient.Start(_chatVm.SessionId, _activeAgentId);
        _chatVm.AppendSystemMessage($"已连接 {_floatingBallVm.CurrentAgentName}，会话 {_chatVm.SessionId} 已就绪。");
    }

    public Task StopAsync()
    {
        _cts.Cancel();
        _sseClient.Stop();
        _apiClient.Dispose();
        _sseClient.Dispose();
        _voiceService.StopSpeaking();
        _voiceService.Dispose();

        _floatingBallWindow?.Hide();
        _chatWindow?.Hide();
        _controlPanelWindow?.Hide();
        _reminderWindow?.Hide();

        return Task.CompletedTask;
    }

    private async Task SendChatAsync(string text, AgentProfile? agent)
    {
        agent ??= _chatVm.SelectedAgent;
        if (agent is null)
        {
            _chatVm.AppendSystemMessage("未选择助手。");
            return;
        }

        try
        {
            var runId = await _apiClient.SendChatAsync(_chatVm.SessionId, agent.Id, text, _cts.Token);
            _chatVm.MarkRunStarted(runId);
        }
        catch (Exception ex)
        {
            _chatVm.MarkRunCompleted();
            _chatVm.AppendSystemMessage($"请求失败：{ex.Message}");
            _floatingBallVm.SetState(AssistantState.Error);
        }
    }

    private async Task SwitchAgentAsync(AgentProfile agent, AgentChangeSource source)
    {
        if (_syncingAgentSelection || agent.Id == _activeAgentId)
        {
            return;
        }

        PersistCurrentChatHistory(_activeAgentId);
        _activeAgentId = agent.Id;

        _syncingAgentSelection = true;
        if (source != AgentChangeSource.Chat)
        {
            _chatVm.SelectedAgent = agent;
        }

        if (source != AgentChangeSource.Panel)
        {
            _controlPanelVm.SelectedAgent = agent;
        }
        _syncingAgentSelection = false;

        _floatingBallVm.CurrentAgentName = agent.DisplayName;

        if (!_availableQuickTasksByAgent.TryGetValue(agent.Id, out var available))
        {
            available = (await SafeGetQuickTasksAsync(agent.Id)).ToList();
            _availableQuickTasksByAgent[agent.Id] = available;
        }

        _controlPanelVm.LoadAvailableRingTasks(available);

        var enabledTasks = ResolveEnabledTasks(agent.Id, available);
        _controlPanelVm.LoadEnabledRingTasks(enabledTasks);
        ApplyRingTasksForActiveAgent();

        LoadChatHistory(agent.Id);
        _chatVm.AppendSystemMessage($"已切换到 {agent.DisplayName}（记忆边界：{LocalizeMemoryBoundary(agent.MemoryBoundary)}）。");

        _sseClient.Start(_chatVm.SessionId, agent.Id);
    }

    private List<QuickTask> ResolveEnabledTasks(string agentId, IReadOnlyCollection<QuickTask> available)
    {
        if (!_enabledRingTaskIdsByAgent.TryGetValue(agentId, out var ids) || ids.Count == 0)
        {
            return [];
        }

        var map = available.ToDictionary(item => item.Id, item => item);
        var result = new List<QuickTask>();
        foreach (var id in ids)
        {
            if (map.TryGetValue(id, out var task))
            {
                result.Add(task);
            }
        }

        return result;
    }

    private void PersistCurrentChatHistory(string agentId)
    {
        if (string.IsNullOrWhiteSpace(agentId))
        {
            return;
        }

        _chatHistoryByAgent[agentId] = _chatVm.Messages
            .Select(item => new ChatMessage
            {
                Id = item.Id,
                Role = item.Role,
                Content = item.Content,
                Timestamp = item.Timestamp,
                RunDurationMs = item.RunDurationMs
            })
            .ToList();
    }

    private void LoadChatHistory(string agentId)
    {
        _chatVm.Messages.Clear();
        if (!_chatHistoryByAgent.TryGetValue(agentId, out var history))
        {
            return;
        }

        foreach (var item in history)
        {
            _chatVm.Messages.Add(new ChatMessage
            {
                Id = item.Id,
                Role = item.Role,
                Content = item.Content,
                Timestamp = item.Timestamp,
                RunDurationMs = item.RunDurationMs
            });
        }
    }

    private void OnQuickTaskInvoked(QuickTask task)
    {
        ShowChat();

        if (task.Id == "inspect")
        {
            ShowControlPanel(false);
            _controlPanelVm.InspectTasksCommand.Execute(null);
            return;
        }

        _chatVm.InputText = $"请帮我{task.DisplayLabel}";
        _chatVm.SendCommand.Execute(null);
    }

    private void OnRingTasksChanged(IReadOnlyList<QuickTask> tasks)
    {
        _enabledRingTaskIdsByAgent[_activeAgentId] = tasks.Select(item => item.Id).ToList();
        ApplyRingTasksForActiveAgent();
    }

    private void ApplyRingTasksForActiveAgent()
    {
        if (!_availableQuickTasksByAgent.TryGetValue(_activeAgentId, out var available))
        {
            _floatingBallVm.SetTasks([]);
            return;
        }

        var enabled = ResolveEnabledTasks(_activeAgentId, available);
        _floatingBallVm.SetTasks(enabled);
    }

    private void TogglePause()
    {
        _floatingBallVm.IsPaused = !_floatingBallVm.IsPaused;
        _chatVm.AppendSystemMessage(_floatingBallVm.IsPaused ? "助手已暂停。" : "助手已恢复运行。");
    }

    private void OnSseEventReceived(object? sender, SseEvent evt)
    {
        Application.Current.Dispatcher.Invoke(() => HandleSseEventOnUi(evt));
    }

    private void HandleSseEventOnUi(SseEvent evt)
    {
        _controlPanelVm.ApplyDashboardEvent(evt);

        switch (evt.Type)
        {
            case "agent.state":
                {
                    var state = AssistantVisualMapper.Parse(GetString(evt.Data, "state", "idle"));
                    _floatingBallVm.SetState(state);
                    _chatVm.StatusText = AssistantVisualMapper.ToDisplayText(state);
                    break;
                }

            case "run.started":
                if (!string.IsNullOrWhiteSpace(evt.RunId))
                {
                    _chatVm.MarkRunStarted(evt.RunId);
                }
                break;

            case "reasoning.summary":
                {
                    var summary = GetString(evt.Data, "summary", string.Empty);
                    if (!string.IsNullOrWhiteSpace(summary) && !string.IsNullOrWhiteSpace(evt.RunId))
                    {
                        _summaryByRunId[evt.RunId] = summary;
                    }

                    break;
                }

            case "run.completed":
                {
                    var reply = GetString(evt.Data, "reply", string.Empty);
                    if (string.IsNullOrWhiteSpace(reply) && !string.IsNullOrWhiteSpace(evt.RunId))
                    {
                        _summaryByRunId.TryGetValue(evt.RunId, out reply);
                    }

                    if (string.IsNullOrWhiteSpace(reply))
                    {
                        reply = "任务已完成。";
                    }

                    var duration = GetLong(evt.Data, "duration_ms", _chatVm.CurrentRunElapsedMs());
                    _chatVm.AppendAssistantMessage(reply, duration);
                    _chatVm.MarkRunCompleted();
                    if (_chatVm.AutoSpeakEnabled && _voiceService.IsSpeechPlaybackAvailable)
                    {
                        _ = _voiceService.SpeakAsync(reply);
                    }
                    break;
                }

            case "reminder.suggested":
                HandleReminder(evt);
                break;
        }
    }

    private void HandleReminder(SseEvent evt)
    {
        if (!_settings.ActiveAssistEnabled)
        {
            return;
        }

        var minInterval = Math.Max(_settings.NudgeMinIntervalSec, 5);
        var maxPerHour = Math.Max(_settings.NudgeMaxPerHour, 1);
        if (!_reminderGate.CanEmit(minInterval, maxPerHour))
        {
            return;
        }

        var level = evt.Level <= 0 ? 1 : evt.Level;
        var title = GetString(evt.Data, "title", "主动协助提醒");
        var message = GetString(evt.Data, "message", "检测到可协助场景");

        _reminderVm.Level = level;
        _reminderVm.Title = $"{title} (L{level})";
        _reminderVm.Message = message;

        if (_reminderWindow is null || _floatingBallWindow is null)
        {
            return;
        }

        _reminderWindow.PositionNear(_floatingBallWindow);
        _reminderWindow.Show();
    }

    private void OnReminderExecute()
    {
        _reminderWindow?.Hide();
        ShowChat();
        _chatVm.InputText = "请根据刚才的提醒直接执行建议，先给我确认步骤。";
        _chatVm.SendCommand.Execute(null);
    }

    private void ShowChat()
    {
        if (_chatWindow is null || _floatingBallWindow is null)
        {
            return;
        }

        _chatWindow.PositionNear(_floatingBallWindow);
        _chatWindow.Show();
        _chatWindow.Activate();
    }

    private void ShowControlPanel(bool showDashboardHome)
    {
        if (_controlPanelWindow is null)
        {
            return;
        }

        _controlPanelWindow.Show();
        if (showDashboardHome)
        {
            _controlPanelWindow.SelectDashboardHome();
        }

        _controlPanelWindow.Activate();
    }

    private void OnBallMoved()
    {
        if (_floatingBallWindow is null)
        {
            return;
        }

        if (_chatWindow?.IsVisible == true)
        {
            _chatWindow.PositionNear(_floatingBallWindow);
        }

        if (_reminderWindow?.IsVisible == true)
        {
            _reminderWindow.PositionNear(_floatingBallWindow);
        }
    }

    private void OnVoiceRecognizedText(object? sender, string text)
    {
        Application.Current.Dispatcher.Invoke(() => _chatVm.ApplyRecognizedText(text));
    }

    private void OnVoiceStatusChanged(object? sender, string status)
    {
        Application.Current.Dispatcher.Invoke(() => _chatVm.VoiceStatusText = status);
    }

    private void OnVoiceListeningStateChanged(object? sender, bool isListening)
    {
        Application.Current.Dispatcher.Invoke(() => _chatVm.UpdateVoiceListening(isListening));
    }

    private void OnVoiceSpeakingStateChanged(object? sender, bool isSpeaking)
    {
        Application.Current.Dispatcher.Invoke(() => _chatVm.UpdateVoiceSpeaking(isSpeaking));
    }

    private async Task<IReadOnlyList<AgentProfile>> SafeGetAgentsAsync()
    {
        try
        {
            var agents = await _apiClient.GetAgentsAsync(_cts.Token);
            if (agents.Count > 0)
            {
                return agents;
            }
        }
        catch
        {
            // ignored
        }

        return
        [
            new AgentProfile
            {
                Id = "life-agent",
                Name = "生活助手",
                Style = "陪伴优先",
                MemoryBoundary = "个人记忆",
                RouteBoundary = "日常场景",
                Default = true,
                PermissionBoundary = new PermissionBoundary
                {
                    GreenActions = ["总结", "翻译"],
                    YellowActions = ["打开网站"],
                    RedActions = ["执行命令"]
                }
            }
        ];
    }

    private async Task<IReadOnlyList<QuickTask>> SafeGetQuickTasksAsync(string agentId)
    {
        try
        {
            return await _apiClient.GetQuickTasksAsync(agentId, _cts.Token);
        }
        catch
        {
            return
            [
                new QuickTask { Id = "summary", Label = "总结当前内容", ShortLabel = "总结", Description = "总结" },
                new QuickTask { Id = "translate", Label = "翻译选中内容", ShortLabel = "翻译", Description = "翻译" },
                new QuickTask { Id = "next", Label = "问它下一步", ShortLabel = "下一步", Description = "下一步" }
            ];
        }
    }

    private async Task<AppSettingsModel> SafeGetSettingsAsync()
    {
        try
        {
            return await _apiClient.GetSettingsAsync(_cts.Token);
        }
        catch
        {
            return new AppSettingsModel
            {
                ActiveAssistEnabled = true,
                NudgeMinIntervalSec = 45,
                NudgeMaxPerHour = 6,
                BallSize = 96,
                ShowFloatingBall = true
            };
        }
    }

    private async Task<IReadOnlyList<DashboardEventItem>> SafeGetDashboardSnapshotAsync()
    {
        try
        {
            return await _apiClient.GetDashboardSnapshotAsync(_cts.Token);
        }
        catch
        {
            return [];
        }
    }

    private async Task<IReadOnlyList<SkillItem>> SafeGetSkillsAsync()
    {
        try
        {
            return await _apiClient.GetSkillsAsync(_cts.Token);
        }
        catch
        {
            return
            [
                new SkillItem
                {
                    Id = "skill-web-search",
                    Name = "网页搜索",
                    Source = "内置",
                    Version = "0.1.0",
                    Installed = true,
                    Scope = "共享",
                    Description = "后端不可用时的本地模拟技能列表。"
                }
            ];
        }
    }

    private static string LocalizeMemoryBoundary(string boundary)
    {
        return boundary switch
        {
            "personal-memory" => "个人记忆",
            "work-memory" => "工作记忆",
            "shared-memory" => "共享记忆",
            _ => boundary
        };
    }

    private static string GetString(JsonElement data, string property, string fallback)
    {
        if (data.ValueKind == JsonValueKind.Object && data.TryGetProperty(property, out var value))
        {
            return value.ValueKind switch
            {
                JsonValueKind.String => value.GetString() ?? fallback,
                JsonValueKind.Number => value.GetRawText(),
                JsonValueKind.True => "是",
                JsonValueKind.False => "否",
                _ => fallback
            };
        }

        return fallback;
    }

    private static long GetLong(JsonElement data, string property, long fallback)
    {
        if (data.ValueKind == JsonValueKind.Object && data.TryGetProperty(property, out var value))
        {
            if (value.ValueKind == JsonValueKind.Number && value.TryGetInt64(out var intValue))
            {
                return intValue;
            }

            if (value.ValueKind == JsonValueKind.String && long.TryParse(value.GetString(), out intValue))
            {
                return intValue;
            }
        }

        return fallback;
    }

    private enum AgentChangeSource
    {
        Chat,
        Panel
    }
}
