namespace CialloClaw.Shell.Models;

public sealed class AppSettingsModel
{
    public string Language { get; set; } = "zh-CN";
    public bool AutoStart { get; set; }
    public bool MinimizeToTray { get; set; }
    public bool KeepRunningOnClose { get; set; }
    public bool SystemNotifyEnabled { get; set; }
    public bool SoundReminderEnabled { get; set; }
    public bool ShowFloatingBall { get; set; }
    public bool AutoDock { get; set; }
    public bool IdleHalfTransparent { get; set; }
    public int BallSize { get; set; } = 96;
    public bool EnableTaskRing { get; set; }
    public bool ActiveAssistEnabled { get; set; }
    public int AssistLevelDefault { get; set; } = 1;
    public int NudgeMinIntervalSec { get; set; } = 45;
    public int NudgeMaxPerHour { get; set; } = 6;
    public string MemoryLifecycle { get; set; } = "30d";
    public string InspectFrequency { get; set; } = "15m";
    public bool InspectOnStartup { get; set; }
    public bool InspectOnFileChanged { get; set; }
    public bool DueReminderEnabled { get; set; }
    public bool LongPendingAlert { get; set; }
    public string CurrentModel { get; set; } = "gpt-5.4-mini";
    public string CurrentProvider { get; set; } = "mock-provider";
    public bool BudgetAutoDowngrade { get; set; }
    public string WorkspacePath { get; set; } = string.Empty;
    public bool ProactiveOnlySafeMode { get; set; }
}
