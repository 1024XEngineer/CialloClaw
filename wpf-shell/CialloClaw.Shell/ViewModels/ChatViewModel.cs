using System.Collections.ObjectModel;
using CialloClaw.Shell.Infrastructure;
using CialloClaw.Shell.Models;

namespace CialloClaw.Shell.ViewModels;

public sealed class ChatViewModel : ObservableObject
{
    private AgentProfile? _selectedAgent;
    private string _inputText = string.Empty;
    private string _statusText = "空闲";
    private bool _isBusy;
    private string _activeRunId = string.Empty;
    private DateTime _runStartedAt;

    private string _featuredTitle = "会话舞台";
    private string _featuredRoleLabel = "待命";
    private string _featuredContent = "把鼠标停在头像旁边，说一句需求，或写下一句意图。我会把本轮会话整理成舞台卡片，而不是普通聊天框。";
    private string _featuredMeta = "等待新的动作";
    private string _latestAssistantMessage = string.Empty;

    private string _voiceStatusText = "语音待命";
    private string _voiceDraftText = string.Empty;
    private bool _isVoiceAvailable;
    private bool _isSpeechPlaybackAvailable;
    private bool _isListening;
    private bool _isSpeaking;
    private bool _autoSpeakEnabled;
    private bool _voiceAutoSendEnabled = true;

    public ChatViewModel()
    {
        SessionId = $"session-{Guid.NewGuid():N}";
        Messages = new ObservableCollection<ChatMessage>();
        Agents = new ObservableCollection<AgentProfile>();
        EchoTrail = new ObservableCollection<ChatMessage>();

        Messages.CollectionChanged += (_, _) => RefreshConversationStage();

        SendCommand = new AsyncRelayCommand(async _ => await SendAsync(), _ => IsSendEnabled);
        ClearCommand = new RelayCommand(_ =>
        {
            Messages.Clear();
            VoiceDraftText = string.Empty;
        });
        ToggleVoiceCommand = new AsyncRelayCommand(async _ => await ToggleVoiceAsync(), _ => IsVoiceAvailable);
        SpeakLatestCommand = new AsyncRelayCommand(async _ => await SpeakLatestAsync(), _ => IsSpeechPlaybackAvailable && !string.IsNullOrWhiteSpace(LatestAssistantMessage));
        StopSpeakingCommand = new RelayCommand(_ => StopSpeakingRequested?.Invoke(), _ => IsSpeaking);
    }

    public Func<string, AgentProfile?, Task>? SendRequestedAsync { get; set; }
    public Func<Task>? ToggleVoiceRequestedAsync { get; set; }
    public Func<string, Task>? SpeakRequestedAsync { get; set; }
    public Action? StopSpeakingRequested { get; set; }

    public event Action<AgentProfile>? AgentChanged;

    public string SessionId { get; }

    public ObservableCollection<ChatMessage> Messages { get; }
    public ObservableCollection<ChatMessage> EchoTrail { get; }
    public ObservableCollection<AgentProfile> Agents { get; }

    public AgentProfile? SelectedAgent
    {
        get => _selectedAgent;
        set
        {
            if (SetProperty(ref _selectedAgent, value) && value is not null)
            {
                AgentChanged?.Invoke(value);
            }
        }
    }

    public string InputText
    {
        get => _inputText;
        set
        {
            if (SetProperty(ref _inputText, value))
            {
                RaiseSendStateChanged();
            }
        }
    }

    public string StatusText
    {
        get => _statusText;
        set => SetProperty(ref _statusText, value);
    }

    public bool IsBusy
    {
        get => _isBusy;
        set
        {
            if (SetProperty(ref _isBusy, value))
            {
                RaisePropertyChanged(nameof(IsSendEnabled));
                RaisePropertyChanged(nameof(ComposerStateText));
                RaiseSendStateChanged();
            }
        }
    }

    public bool IsSendEnabled => !IsBusy && !string.IsNullOrWhiteSpace(InputText);

    public string ActiveRunId
    {
        get => _activeRunId;
        set => SetProperty(ref _activeRunId, value);
    }

    public string FeaturedTitle
    {
        get => _featuredTitle;
        private set => SetProperty(ref _featuredTitle, value);
    }

    public string FeaturedRoleLabel
    {
        get => _featuredRoleLabel;
        private set => SetProperty(ref _featuredRoleLabel, value);
    }

    public string FeaturedContent
    {
        get => _featuredContent;
        private set => SetProperty(ref _featuredContent, value);
    }

    public string FeaturedMeta
    {
        get => _featuredMeta;
        private set => SetProperty(ref _featuredMeta, value);
    }

    public string LatestAssistantMessage
    {
        get => _latestAssistantMessage;
        private set
        {
            if (SetProperty(ref _latestAssistantMessage, value))
            {
                SpeakLatestCommand.RaiseCanExecuteChanged();
            }
        }
    }

    public string EchoTrailHeader => EchoTrail.Count == 0 ? "还没有回声" : $"最近 {EchoTrail.Count} 条回声";

    public string VoiceStatusText
    {
        get => _voiceStatusText;
        set => SetProperty(ref _voiceStatusText, value);
    }

    public string VoiceDraftText
    {
        get => _voiceDraftText;
        set => SetProperty(ref _voiceDraftText, value);
    }

    public bool IsVoiceAvailable
    {
        get => _isVoiceAvailable;
        private set
        {
            if (SetProperty(ref _isVoiceAvailable, value))
            {
                RaisePropertyChanged(nameof(VoiceOrbText));
                RaisePropertyChanged(nameof(VoiceOrbHint));
                ToggleVoiceCommand.RaiseCanExecuteChanged();
            }
        }
    }

    public bool IsSpeechPlaybackAvailable
    {
        get => _isSpeechPlaybackAvailable;
        private set
        {
            if (SetProperty(ref _isSpeechPlaybackAvailable, value))
            {
                SpeakLatestCommand.RaiseCanExecuteChanged();
            }
        }
    }

    public bool IsListening
    {
        get => _isListening;
        private set
        {
            if (SetProperty(ref _isListening, value))
            {
                RaisePropertyChanged(nameof(VoiceOrbText));
                RaisePropertyChanged(nameof(VoiceOrbHint));
            }
        }
    }

    public bool IsSpeaking
    {
        get => _isSpeaking;
        private set
        {
            if (SetProperty(ref _isSpeaking, value))
            {
                StopSpeakingCommand.RaiseCanExecuteChanged();
            }
        }
    }

    public bool AutoSpeakEnabled
    {
        get => _autoSpeakEnabled;
        set => SetProperty(ref _autoSpeakEnabled, value);
    }

    public bool VoiceAutoSendEnabled
    {
        get => _voiceAutoSendEnabled;
        set => SetProperty(ref _voiceAutoSendEnabled, value);
    }

    public string VoiceOrbText => !IsVoiceAvailable ? "无语音" : IsListening ? "停止" : "开麦";

    public string VoiceOrbHint
    {
        get
        {
            if (!IsVoiceAvailable)
            {
                return "当前设备没有可用语音识别";
            }

            return IsListening ? "正在收音，点一下结束" : "点一下直接说话";
        }
    }

    public string ComposerStateText => IsBusy ? "本轮任务执行中" : "可以输入文本，或直接开麦";

    public AsyncRelayCommand SendCommand { get; }
    public RelayCommand ClearCommand { get; }
    public AsyncRelayCommand ToggleVoiceCommand { get; }
    public AsyncRelayCommand SpeakLatestCommand { get; }
    public RelayCommand StopSpeakingCommand { get; }

    public void LoadAgents(IEnumerable<AgentProfile> agents)
    {
        Agents.Clear();
        foreach (var agent in agents)
        {
            Agents.Add(agent);
        }

        SelectedAgent ??= Agents.FirstOrDefault(item => item.Default) ?? Agents.FirstOrDefault();
    }

    public void AppendUserMessage(string text)
    {
        Messages.Add(new ChatMessage
        {
            Role = "user",
            Content = text,
            Timestamp = DateTime.Now
        });
    }

    public void AppendSystemMessage(string text)
    {
        Messages.Add(new ChatMessage
        {
            Role = "system",
            Content = text,
            Timestamp = DateTime.Now
        });
    }

    public void AppendAssistantMessage(string text, long? durationMs = null)
    {
        Messages.Add(new ChatMessage
        {
            Role = "assistant",
            Content = text,
            Timestamp = DateTime.Now,
            RunDurationMs = durationMs
        });
    }

    public void MarkRunStarted(string runId)
    {
        ActiveRunId = runId;
        _runStartedAt = DateTime.Now;
        IsBusy = true;
        StatusText = "执行中";
    }

    public void MarkRunCompleted()
    {
        IsBusy = false;
        ActiveRunId = string.Empty;
        StatusText = "空闲";
    }

    public long CurrentRunElapsedMs()
    {
        if (_runStartedAt == default)
        {
            return 0;
        }

        return (long)(DateTime.Now - _runStartedAt).TotalMilliseconds;
    }

    public void SetVoiceCapabilities(bool recognitionAvailable, bool speechPlaybackAvailable, string statusText)
    {
        IsVoiceAvailable = recognitionAvailable;
        IsSpeechPlaybackAvailable = speechPlaybackAvailable;
        VoiceStatusText = statusText;
    }

    public void UpdateVoiceListening(bool isListening, string? statusText = null)
    {
        IsListening = isListening;
        if (!string.IsNullOrWhiteSpace(statusText))
        {
            VoiceStatusText = statusText;
        }
    }

    public void UpdateVoiceSpeaking(bool isSpeaking, string? statusText = null)
    {
        IsSpeaking = isSpeaking;
        if (!string.IsNullOrWhiteSpace(statusText))
        {
            VoiceStatusText = statusText;
        }
    }

    public void ApplyRecognizedText(string text)
    {
        var cleaned = text.Trim();
        if (string.IsNullOrWhiteSpace(cleaned))
        {
            return;
        }

        VoiceDraftText = cleaned;
        InputText = cleaned;

        if (VoiceAutoSendEnabled && SendCommand.CanExecute(null))
        {
            SendCommand.Execute(null);
        }
    }

    public Task RequestSpeakLatestAsync()
    {
        if (string.IsNullOrWhiteSpace(LatestAssistantMessage) || SpeakRequestedAsync is null)
        {
            return Task.CompletedTask;
        }

        return SpeakRequestedAsync(LatestAssistantMessage);
    }

    private async Task SendAsync()
    {
        var text = InputText.Trim();
        if (string.IsNullOrWhiteSpace(text))
        {
            return;
        }

        InputText = string.Empty;
        VoiceDraftText = text;
        AppendUserMessage(text);

        if (SendRequestedAsync is null)
        {
            AppendSystemMessage("后端未连接，请先启动核心服务。");
            return;
        }

        await SendRequestedAsync(text, SelectedAgent);
    }

    private async Task ToggleVoiceAsync()
    {
        if (ToggleVoiceRequestedAsync is null)
        {
            VoiceStatusText = "语音服务未连接";
            return;
        }

        await ToggleVoiceRequestedAsync();
    }

    private async Task SpeakLatestAsync()
    {
        if (string.IsNullOrWhiteSpace(LatestAssistantMessage) || SpeakRequestedAsync is null)
        {
            return;
        }

        await SpeakRequestedAsync(LatestAssistantMessage);
    }

    private void RefreshConversationStage()
    {
        EchoTrail.Clear();
        foreach (var item in Messages.Reverse().Take(7))
        {
            EchoTrail.Add(item);
        }

        RaisePropertyChanged(nameof(EchoTrailHeader));

        LatestAssistantMessage = Messages.LastOrDefault(item => item.Role == "assistant")?.Content ?? string.Empty;

        var featured = Messages.LastOrDefault(item => item.Role != "system") ?? Messages.LastOrDefault();
        if (featured is null)
        {
            FeaturedTitle = "会话舞台";
            FeaturedRoleLabel = "待命";
            FeaturedContent = "把鼠标停在头像旁边，说一句需求，或写下一句意图。我会把本轮会话整理成舞台卡片，而不是普通聊天框。";
            FeaturedMeta = "等待新的动作";
            return;
        }

        FeaturedTitle = featured.Role switch
        {
            "user" => "当前意图",
            "assistant" => "当前回应",
            _ => "系统提示"
        };
        FeaturedRoleLabel = featured.DisplayRoleLabel;
        FeaturedContent = featured.Content;
        FeaturedMeta = string.IsNullOrWhiteSpace(featured.DurationText)
            ? $"{featured.DisplayTime} · {featured.DisplayRoleLabel}"
            : $"{featured.DisplayTime} · {featured.DurationText}";
    }

    private void RaiseSendStateChanged()
    {
        RaisePropertyChanged(nameof(IsSendEnabled));
        RaisePropertyChanged(nameof(ComposerStateText));
        SendCommand.RaiseCanExecuteChanged();
    }
}
