using System.Globalization;
using System.IO;
using System.Speech.Recognition;
using System.Speech.Synthesis;
using System.Windows.Media;

namespace CialloClaw.Shell.Services;

public sealed class VoiceInteractionService : IDisposable
{
    private enum PlaybackMode
    {
        None,
        VoiceBank,
        Synth
    }

    private readonly MediaPlayer _mediaPlayer;
    private readonly List<string> _voiceBankDirectoryCandidates;

    private SpeechRecognitionEngine? _recognizer;
    private SpeechSynthesizer? _synthesizer;
    private bool _isListening;
    private PlaybackMode _playbackMode;
    private TaskCompletionSource? _playbackCompletionSource;
    private string _requestedSpeechText = string.Empty;

    public VoiceInteractionService()
    {
        _mediaPlayer = new MediaPlayer();
        _mediaPlayer.MediaEnded += OnVoiceBankMediaEnded;
        _mediaPlayer.MediaFailed += OnVoiceBankMediaFailed;
        _voiceBankDirectoryCandidates = BuildVoiceBankCandidates();

        CurrentStatus = "语音待命（本地音库优先）";

        InitializeRecognizer();
        InitializeSynthesizer();
        RefreshPlaybackAvailability();

        if (!IsRecognitionAvailable && !IsSpeechPlaybackAvailable)
        {
            CurrentStatus = "当前设备没有可用语音能力";
        }
        else if (HasVoiceBankClips())
        {
            CurrentStatus = $"语音待命（已检测到本地音库 {GetVoiceBankFiles().Count} 条）";
        }
    }

    public event EventHandler<string>? RecognizedTextReceived;
    public event EventHandler<string>? StatusChanged;
    public event EventHandler<bool>? ListeningStateChanged;
    public event EventHandler<bool>? SpeakingStateChanged;

    public bool IsRecognitionAvailable { get; private set; }
    public bool IsSpeechPlaybackAvailable { get; private set; }
    public string CurrentStatus { get; private set; }

    public Task ToggleListeningAsync()
    {
        if (!IsRecognitionAvailable || _recognizer is null)
        {
            PublishStatus("当前设备没有可用语音识别");
            return Task.CompletedTask;
        }

        if (_isListening)
        {
            try
            {
                _recognizer.RecognizeAsyncCancel();
            }
            catch
            {
                PublishStatus("停止收音失败");
            }

            return Task.CompletedTask;
        }

        try
        {
            _recognizer.RecognizeAsync(RecognizeMode.Single);
            _isListening = true;
            ListeningStateChanged?.Invoke(this, true);
            PublishStatus("正在收音，请直接说话");
        }
        catch (Exception ex)
        {
            PublishStatus($"麦克风启动失败：{ex.Message}");
        }

        return Task.CompletedTask;
    }

    public Task SpeakAsync(string text)
    {
        var cleaned = text.Trim();
        if (string.IsNullOrWhiteSpace(cleaned))
        {
            return Task.CompletedTask;
        }

        StopSpeaking();
        RefreshPlaybackAvailability();

        if (!IsSpeechPlaybackAvailable)
        {
            PublishStatus("当前设备没有可用语音播报");
            return Task.CompletedTask;
        }

        _requestedSpeechText = cleaned;

        if (TryResolveVoiceBankClip(cleaned, out var clipPath))
        {
            return PlayVoiceBankClipAsync(clipPath);
        }

        if (_synthesizer is null)
        {
            PublishStatus("本地音库没有匹配语音，且系统语音不可用");
            return Task.CompletedTask;
        }

        return PlaySynthAsync(cleaned, useFallbackStatus: HasVoiceBankClips());
    }

    public void StopSpeaking()
    {
        if (_playbackMode == PlaybackMode.VoiceBank)
        {
            _mediaPlayer.Stop();
            FinalizePlayback("已停止朗读");
            return;
        }

        if (_playbackMode == PlaybackMode.Synth)
        {
            _playbackMode = PlaybackMode.None;
            var completion = _playbackCompletionSource;
            _playbackCompletionSource = null;

            try
            {
                _synthesizer?.SpeakAsyncCancelAll();
            }
            catch
            {
                SpeakingStateChanged?.Invoke(this, false);
                PublishStatus("已停止朗读");
                completion?.TrySetResult();
            }

            SpeakingStateChanged?.Invoke(this, false);
            PublishStatus("已停止朗读");
            completion?.TrySetResult();
            return;
        }

        SpeakingStateChanged?.Invoke(this, false);
    }

    public void Dispose()
    {
        if (_recognizer is not null)
        {
            _recognizer.SpeechRecognized -= OnSpeechRecognized;
            _recognizer.SpeechRecognitionRejected -= OnSpeechRecognitionRejected;
            _recognizer.RecognizeCompleted -= OnRecognizeCompleted;
            _recognizer.Dispose();
        }

        if (_synthesizer is not null)
        {
            _synthesizer.SpeakCompleted -= OnSynthSpeakCompleted;
            _synthesizer.Dispose();
        }

        _mediaPlayer.Close();
        _mediaPlayer.MediaEnded -= OnVoiceBankMediaEnded;
        _mediaPlayer.MediaFailed -= OnVoiceBankMediaFailed;
    }

    private void InitializeRecognizer()
    {
        try
        {
            var recognizerInfo = SpeechRecognitionEngine.InstalledRecognizers()
                .FirstOrDefault(item => string.Equals(item.Culture.Name, "zh-CN", StringComparison.OrdinalIgnoreCase))
                ?? SpeechRecognitionEngine.InstalledRecognizers()
                    .FirstOrDefault(item => item.Culture.Name.StartsWith("zh", StringComparison.OrdinalIgnoreCase))
                ?? SpeechRecognitionEngine.InstalledRecognizers().FirstOrDefault();

            if (recognizerInfo is null)
            {
                PublishStatus("未检测到可用语音识别器");
                return;
            }

            _recognizer = new SpeechRecognitionEngine(recognizerInfo);
            _recognizer.LoadGrammar(new DictationGrammar());
            _recognizer.SetInputToDefaultAudioDevice();
            _recognizer.InitialSilenceTimeout = TimeSpan.FromSeconds(5);
            _recognizer.BabbleTimeout = TimeSpan.FromSeconds(3);
            _recognizer.EndSilenceTimeout = TimeSpan.FromSeconds(1.1);
            _recognizer.EndSilenceTimeoutAmbiguous = TimeSpan.FromSeconds(1.5);

            _recognizer.SpeechRecognized += OnSpeechRecognized;
            _recognizer.SpeechRecognitionRejected += OnSpeechRecognitionRejected;
            _recognizer.RecognizeCompleted += OnRecognizeCompleted;

            IsRecognitionAvailable = true;
        }
        catch (Exception ex)
        {
            PublishStatus($"语音识别不可用：{ex.Message}");
        }
    }

    private void InitializeSynthesizer()
    {
        try
        {
            _synthesizer = new SpeechSynthesizer
            {
                Rate = 0,
                Volume = 90
            };

            var chineseVoice = _synthesizer.GetInstalledVoices()
                .Where(item => item.Enabled)
                .Select(item => item.VoiceInfo)
                .FirstOrDefault(item => string.Equals(item.Culture.Name, "zh-CN", StringComparison.OrdinalIgnoreCase))
                ?? _synthesizer.GetInstalledVoices()
                    .Where(item => item.Enabled)
                    .Select(item => item.VoiceInfo)
                    .FirstOrDefault(item => item.Culture.Name.StartsWith("zh", StringComparison.OrdinalIgnoreCase));

            if (chineseVoice is not null)
            {
                _synthesizer.SelectVoice(chineseVoice.Name);
            }

            _synthesizer.SpeakCompleted += OnSynthSpeakCompleted;
        }
        catch (Exception ex)
        {
            PublishStatus($"语音播报不可用：{ex.Message}");
        }
    }

    private void OnSpeechRecognized(object? sender, SpeechRecognizedEventArgs e)
    {
        var text = e.Result.Text?.Trim() ?? string.Empty;
        if (string.IsNullOrWhiteSpace(text) || e.Result.Confidence < 0.45)
        {
            PublishStatus("没有听清，请再说一遍");
            return;
        }

        RecognizedTextReceived?.Invoke(this, text);
        PublishStatus($"已识别：{Preview(text)}");
    }

    private void OnSpeechRecognitionRejected(object? sender, SpeechRecognitionRejectedEventArgs e)
    {
        PublishStatus("没有识别出有效语句");
    }

    private void OnRecognizeCompleted(object? sender, RecognizeCompletedEventArgs e)
    {
        _isListening = false;
        ListeningStateChanged?.Invoke(this, false);

        if (e.Error is not null)
        {
            PublishStatus($"语音识别中断：{e.Error.Message}");
            return;
        }

        if (e.Cancelled)
        {
            PublishStatus("已停止收音");
            return;
        }

        PublishStatus("语音待命");
    }

    private Task PlayVoiceBankClipAsync(string clipPath)
    {
        _playbackMode = PlaybackMode.VoiceBank;
        _playbackCompletionSource = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        SpeakingStateChanged?.Invoke(this, true);
        PublishStatus($"正在播放本地音库：{Path.GetFileNameWithoutExtension(clipPath)}");

        _mediaPlayer.Open(new Uri(clipPath, UriKind.Absolute));
        _mediaPlayer.Position = TimeSpan.Zero;
        _mediaPlayer.Play();

        return _playbackCompletionSource.Task;
    }

    private Task PlaySynthAsync(string text, bool useFallbackStatus)
    {
        if (_synthesizer is null)
        {
            PublishStatus("系统语音不可用");
            return Task.CompletedTask;
        }

        _playbackMode = PlaybackMode.Synth;
        _playbackCompletionSource ??= new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        SpeakingStateChanged?.Invoke(this, true);
        PublishStatus(useFallbackStatus ? "本地音库未命中，改用系统语音播报" : "正在使用系统语音播报");
        _synthesizer.SpeakAsync(text);
        return _playbackCompletionSource.Task;
    }

    private void OnSynthSpeakCompleted(object? sender, SpeakCompletedEventArgs e)
    {
        if (_playbackMode != PlaybackMode.Synth)
        {
            return;
        }

        FinalizePlayback(e.Cancelled ? "已停止朗读" : "朗读完成");
    }

    private void OnVoiceBankMediaEnded(object? sender, EventArgs e)
    {
        if (_playbackMode != PlaybackMode.VoiceBank)
        {
            return;
        }

        FinalizePlayback("本地音库播报完成");
    }

    private void OnVoiceBankMediaFailed(object? sender, ExceptionEventArgs e)
    {
        if (_playbackMode != PlaybackMode.VoiceBank)
        {
            return;
        }

        if (_synthesizer is not null && !string.IsNullOrWhiteSpace(_requestedSpeechText))
        {
            PublishStatus("本地音库播放失败，切换到系统语音");
            _playbackMode = PlaybackMode.None;
            _ = PlaySynthAsync(_requestedSpeechText, useFallbackStatus: false);
            return;
        }

        FinalizePlayback($"本地音库播放失败：{e.ErrorException.Message}");
    }

    private void FinalizePlayback(string status)
    {
        _playbackMode = PlaybackMode.None;
        _requestedSpeechText = string.Empty;

        var completion = _playbackCompletionSource;
        _playbackCompletionSource = null;

        SpeakingStateChanged?.Invoke(this, false);
        PublishStatus(status);
        completion?.TrySetResult();
    }

    private void RefreshPlaybackAvailability()
    {
        IsSpeechPlaybackAvailable = _synthesizer is not null || HasVoiceBankClips();
    }

    private bool HasVoiceBankClips()
    {
        return GetVoiceBankFiles().Count > 0;
    }

    private List<string> GetVoiceBankFiles()
    {
        return _voiceBankDirectoryCandidates
            .Where(Directory.Exists)
            .SelectMany(directory => Directory.EnumerateFiles(directory, "*.*", SearchOption.AllDirectories))
            .Where(file => file.EndsWith(".wav", StringComparison.OrdinalIgnoreCase)
                || file.EndsWith(".mp3", StringComparison.OrdinalIgnoreCase)
                || file.EndsWith(".wma", StringComparison.OrdinalIgnoreCase)
                || file.EndsWith(".m4a", StringComparison.OrdinalIgnoreCase))
            .Distinct(StringComparer.OrdinalIgnoreCase)
            .ToList();
    }

    private bool TryResolveVoiceBankClip(string text, out string clipPath)
    {
        var files = GetVoiceBankFiles();
        if (files.Count == 0)
        {
            clipPath = string.Empty;
            return false;
        }

        var normalizedText = NormalizeKey(text);
        if (!string.IsNullOrWhiteSpace(normalizedText))
        {
            var exactMatch = files.FirstOrDefault(file =>
                NormalizeKey(Path.GetFileNameWithoutExtension(file)).Contains(normalizedText, StringComparison.Ordinal));
            if (!string.IsNullOrWhiteSpace(exactMatch))
            {
                clipPath = exactMatch;
                return true;
            }
        }

        foreach (var keyword in ExtractClipKeywords(text))
        {
            var match = files.FirstOrDefault(file =>
                NormalizeKey(Path.GetFileNameWithoutExtension(file)).Contains(keyword, StringComparison.Ordinal));
            if (!string.IsNullOrWhiteSpace(match))
            {
                clipPath = match;
                return true;
            }
        }

        var genericKeys = new[] { "通用", "默认", "回应", "播报", "陪伴", "voice", "reply", "default" }
            .Select(NormalizeKey)
            .ToArray();

        var genericMatches = files
            .Where(file => genericKeys.Any(key =>
                NormalizeKey(Path.GetFileNameWithoutExtension(file)).Contains(key, StringComparison.Ordinal)))
            .ToList();

        if (genericMatches.Count > 0)
        {
            clipPath = genericMatches[Random.Shared.Next(genericMatches.Count)];
            return true;
        }

        clipPath = string.Empty;
        return false;
    }

    private static IEnumerable<string> ExtractClipKeywords(string text)
    {
        var keywords = new List<string>();
        var rules = new[]
        {
            ("完成", new[] { "完成", "done", "结束", "ok" }),
            ("开始", new[] { "开始", "执行", "run", "action" }),
            ("提醒", new[] { "提醒", "notice", "alert", "tip" }),
            ("错误", new[] { "错误", "失败", "error", "fail" }),
            ("你好", new[] { "你好", "问候", "hello", "greet" }),
            ("好的", new[] { "好的", "收到", "确认", "ack" }),
            ("总结", new[] { "总结", "概括", "summary" }),
            ("翻译", new[] { "翻译", "translate" }),
            ("下一步", new[] { "下一步", "next" })
        };

        foreach (var (needle, values) in rules)
        {
            if (text.Contains(needle, StringComparison.OrdinalIgnoreCase))
            {
                keywords.AddRange(values);
            }
        }

        return keywords
            .Select(NormalizeKey)
            .Where(item => !string.IsNullOrWhiteSpace(item))
            .Distinct(StringComparer.Ordinal);
    }

    private static List<string> BuildVoiceBankCandidates()
    {
        var desktop = Environment.GetFolderPath(Environment.SpecialFolder.DesktopDirectory);
        var baseDirectory = AppContext.BaseDirectory;

        return
        [
            Path.Combine(baseDirectory, "Assets", "voice-bank"),
            Path.Combine(baseDirectory, "Assets", "voices"),
            Path.Combine(desktop, "油库里的声音"),
            Path.Combine(desktop, "音库里的声音"),
            Path.Combine(desktop, "页面设计", "油库里的声音"),
            Path.Combine(desktop, "页面设计", "音库里的声音")
        ];
    }

    private static string NormalizeKey(string text)
    {
        return new string(text
                .Where(ch => !char.IsWhiteSpace(ch) && !char.IsPunctuation(ch) && !char.IsSymbol(ch))
                .ToArray())
            .ToLowerInvariant();
    }

    private void PublishStatus(string text)
    {
        CurrentStatus = text;
        StatusChanged?.Invoke(this, text);
    }

    private static string Preview(string text)
    {
        return text.Length <= 24 ? text : $"{text[..24]}...";
    }
}
