using System.Collections.ObjectModel;
using CialloClaw.Shell.Infrastructure;
using CialloClaw.Shell.Models;

namespace CialloClaw.Shell.ViewModels;

public sealed class ControlPanelViewModel : ObservableObject
{
    private AgentProfile? _selectedAgent;
    private AppSettingsModel _settings = new();
    private string _inspectInfo = "尚未巡检";
    private QuickTask? _selectedAvailableRingTask;
    private QuickTask? _selectedEnabledRingTask;

    public ControlPanelViewModel()
    {
        Agents = new ObservableCollection<AgentProfile>();
        NoteTasks = new ObservableCollection<NoteTaskItem>();
        Skills = new ObservableCollection<SkillItem>();

        AvailableRingTasks = new ObservableCollection<QuickTask>();
        EnabledRingTasks = new ObservableCollection<QuickTask>();

        Timeline = new ObservableCollection<TimelineItem>();
        ToolCalls = new ObservableCollection<ToolCallItem>();
        Citations = new ObservableCollection<CitationItem>();
        BrowserFeed = new ObservableCollection<DashboardEventItem>();
        TerminalFeed = new ObservableCollection<DashboardEventItem>();
        FileFeed = new ObservableCollection<DashboardEventItem>();
        DecisionFeed = new ObservableCollection<DashboardEventItem>();

        InspectTasksCommand = new AsyncRelayCommand(async _ => await InspectAsync());

        AddRingTaskCommand = new RelayCommand(_ => AddRingTask(), _ => SelectedAvailableRingTask is not null);
        RemoveRingTaskCommand = new RelayCommand(_ => RemoveRingTask(), _ => SelectedEnabledRingTask is not null);
        ClearRingTasksCommand = new RelayCommand(_ => ClearRingTasks(), _ => EnabledRingTasks.Count > 0);
    }

    public Func<Task<IReadOnlyList<NoteTaskItem>>>? InspectRequestedAsync { get; set; }
    public event Action<AgentProfile>? AgentChanged;
    public event Action<IReadOnlyList<QuickTask>>? RingTasksChanged;

    public ObservableCollection<AgentProfile> Agents { get; }
    public ObservableCollection<NoteTaskItem> NoteTasks { get; }
    public ObservableCollection<SkillItem> Skills { get; }

    public ObservableCollection<QuickTask> AvailableRingTasks { get; }
    public ObservableCollection<QuickTask> EnabledRingTasks { get; }

    public ObservableCollection<TimelineItem> Timeline { get; }
    public ObservableCollection<ToolCallItem> ToolCalls { get; }
    public ObservableCollection<CitationItem> Citations { get; }
    public ObservableCollection<DashboardEventItem> BrowserFeed { get; }
    public ObservableCollection<DashboardEventItem> TerminalFeed { get; }
    public ObservableCollection<DashboardEventItem> FileFeed { get; }
    public ObservableCollection<DashboardEventItem> DecisionFeed { get; }

    public AgentProfile? SelectedAgent
    {
        get => _selectedAgent;
        set
        {
            if (SetProperty(ref _selectedAgent, value) && value is not null)
            {
                RaisePropertyChanged(nameof(GreenActionsText));
                RaisePropertyChanged(nameof(YellowActionsText));
                RaisePropertyChanged(nameof(RedActionsText));
                AgentChanged?.Invoke(value);
            }
        }
    }

    public AppSettingsModel Settings
    {
        get => _settings;
        set => SetProperty(ref _settings, value);
    }

    public string InspectInfo
    {
        get => _inspectInfo;
        set => SetProperty(ref _inspectInfo, value);
    }

    public QuickTask? SelectedAvailableRingTask
    {
        get => _selectedAvailableRingTask;
        set
        {
            if (SetProperty(ref _selectedAvailableRingTask, value))
            {
                AddRingTaskCommand.RaiseCanExecuteChanged();
            }
        }
    }

    public QuickTask? SelectedEnabledRingTask
    {
        get => _selectedEnabledRingTask;
        set
        {
            if (SetProperty(ref _selectedEnabledRingTask, value))
            {
                RemoveRingTaskCommand.RaiseCanExecuteChanged();
            }
        }
    }

    public string GreenActionsText => string.Join("、", SelectedAgent?.PermissionBoundary.GreenActions ?? []);
    public string YellowActionsText => string.Join("、", SelectedAgent?.PermissionBoundary.YellowActions ?? []);
    public string RedActionsText => string.Join("、", SelectedAgent?.PermissionBoundary.RedActions ?? []);

    public AsyncRelayCommand InspectTasksCommand { get; }
    public RelayCommand AddRingTaskCommand { get; }
    public RelayCommand RemoveRingTaskCommand { get; }
    public RelayCommand ClearRingTasksCommand { get; }

    public void LoadAgents(IEnumerable<AgentProfile> agents)
    {
        Agents.Clear();
        foreach (var agent in agents)
        {
            Agents.Add(agent);
        }

        SelectedAgent ??= Agents.FirstOrDefault(item => item.Default) ?? Agents.FirstOrDefault();
    }

    public void LoadSkills(IEnumerable<SkillItem> skills)
    {
        Skills.Clear();
        foreach (var skill in skills.OrderByDescending(item => item.Installed).ThenBy(item => item.Name))
        {
            Skills.Add(skill);
        }
    }

    public void LoadAvailableRingTasks(IEnumerable<QuickTask> tasks)
    {
        var list = tasks.ToList();
        AvailableRingTasks.Clear();
        foreach (var item in list)
        {
            AvailableRingTasks.Add(item);
        }

        var validEnabled = EnabledRingTasks.Where(enabled => list.Any(candidate => candidate.Id == enabled.Id)).ToList();
        EnabledRingTasks.Clear();
        foreach (var item in validEnabled)
        {
            EnabledRingTasks.Add(item);
        }

        SelectedAvailableRingTask = AvailableRingTasks.FirstOrDefault();
        SelectedEnabledRingTask = EnabledRingTasks.FirstOrDefault();

        NotifyRingTasksChanged();
    }

    public void LoadEnabledRingTasks(IEnumerable<QuickTask> tasks)
    {
        EnabledRingTasks.Clear();
        foreach (var item in tasks)
        {
            EnabledRingTasks.Add(item);
        }

        SelectedEnabledRingTask = EnabledRingTasks.FirstOrDefault();
        NotifyRingTasksChanged();
    }

    public void LoadDashboardSnapshot(IEnumerable<DashboardEventItem> snapshot)
    {
        BrowserFeed.Clear();
        TerminalFeed.Clear();
        FileFeed.Clear();
        DecisionFeed.Clear();

        foreach (var item in snapshot)
        {
            AddDashboardItem(item);
        }
    }

    public void ApplyDashboardEvent(SseEvent evt)
    {
        var timestamp = evt.Timestamp == default ? DateTime.Now : evt.Timestamp.ToLocalTime();

        switch (evt.Type)
        {
            case "timeline":
            case "run.started":
            case "run.completed":
                Timeline.Insert(0, new TimelineItem
                {
                    Timestamp = timestamp,
                    Type = evt.Type,
                    Title = evt.Data.GetPropertyOrDefault("title", LocalizeEventTitle(evt.Type)),
                    Detail = LocalizeRunStatus(evt.Data.GetPropertyOrDefault("status", evt.Data.GetPropertyOrDefault("goal", string.Empty)))
                });
                Trim(Timeline, 120);
                break;

            case "tool.call":
                ToolCalls.Insert(0, new ToolCallItem
                {
                    Timestamp = timestamp,
                    Name = evt.Data.GetPropertyOrDefault("name", "工具"),
                    Status = LocalizeRunStatus(evt.Data.GetPropertyOrDefault("status", "已完成")),
                    ArgsText = evt.Data.GetPropertyOrDefault("args", "{}"),
                    ElapsedMs = evt.Data.GetPropertyOrDefaultLong("elapsed_ms", 0)
                });
                Trim(ToolCalls, 80);
                break;

            case "citation":
                Citations.Insert(0, new CitationItem
                {
                    Timestamp = timestamp,
                    Title = evt.Data.GetPropertyOrDefault("title", "引用"),
                    Source = evt.Data.GetPropertyOrDefault("source", ""),
                    Snippet = evt.Data.GetPropertyOrDefault("snippet", "")
                });
                Trim(Citations, 80);
                break;

            case "decision":
                DecisionFeed.Insert(0, new DashboardEventItem
                {
                    Timestamp = timestamp,
                    Category = "决策",
                    Primary = evt.Data.GetPropertyOrDefault("goal", "当前目标"),
                    Detail = evt.Data.GetPropertyOrDefault("next", "") + " | " + evt.Data.GetPropertyOrDefault("risk", "")
                });
                Trim(DecisionFeed, 80);
                break;

            case "tool.browser":
                BrowserFeed.Insert(0, new DashboardEventItem
                {
                    Timestamp = timestamp,
                    Category = "浏览器",
                    Primary = evt.Data.GetPropertyOrDefault("url", ""),
                    Detail = evt.Data.GetPropertyOrDefault("action", "")
                });
                Trim(BrowserFeed, 80);
                break;

            case "tool.terminal":
                TerminalFeed.Insert(0, new DashboardEventItem
                {
                    Timestamp = timestamp,
                    Category = "终端",
                    Primary = evt.Data.GetPropertyOrDefault("cmd", ""),
                    Detail = evt.Data.GetPropertyOrDefault("stdout", "")
                });
                Trim(TerminalFeed, 80);
                break;

            case "tool.file":
                FileFeed.Insert(0, new DashboardEventItem
                {
                    Timestamp = timestamp,
                    Category = "文件",
                    Primary = evt.Data.GetPropertyOrDefault("path", ""),
                    Detail = evt.Data.GetPropertyOrDefault("action", "")
                });
                Trim(FileFeed, 80);
                break;
        }
    }

    private async Task InspectAsync()
    {
        if (InspectRequestedAsync is null)
        {
            InspectInfo = "巡检服务未连接";
            return;
        }

        var result = await InspectRequestedAsync();
        NoteTasks.Clear();
        foreach (var task in result.OrderBy(item => item.DueAt))
        {
            NoteTasks.Add(task);
        }

        InspectInfo = $"最近巡检：{DateTime.Now:HH:mm:ss}，任务 {NoteTasks.Count} 项";
    }

    private void AddRingTask()
    {
        if (SelectedAvailableRingTask is null)
        {
            return;
        }

        if (EnabledRingTasks.Any(item => item.Id == SelectedAvailableRingTask.Id))
        {
            return;
        }

        EnabledRingTasks.Add(SelectedAvailableRingTask);
        SelectedEnabledRingTask = SelectedAvailableRingTask;
        NotifyRingTasksChanged();
    }

    private void RemoveRingTask()
    {
        if (SelectedEnabledRingTask is null)
        {
            return;
        }

        var target = SelectedEnabledRingTask;
        EnabledRingTasks.Remove(target);
        SelectedEnabledRingTask = EnabledRingTasks.FirstOrDefault();
        NotifyRingTasksChanged();
    }

    private void ClearRingTasks()
    {
        EnabledRingTasks.Clear();
        SelectedEnabledRingTask = null;
        NotifyRingTasksChanged();
    }

    private void NotifyRingTasksChanged()
    {
        AddRingTaskCommand.RaiseCanExecuteChanged();
        RemoveRingTaskCommand.RaiseCanExecuteChanged();
        ClearRingTasksCommand.RaiseCanExecuteChanged();

        RingTasksChanged?.Invoke(EnabledRingTasks.ToList());
    }

    private void AddDashboardItem(DashboardEventItem item)
    {
        switch (item.Category)
        {
            case "Browser":
            case "浏览器":
                BrowserFeed.Add(item);
                break;
            case "Terminal":
            case "终端":
                TerminalFeed.Add(item);
                break;
            case "File":
            case "文件":
                FileFeed.Add(item);
                break;
            case "Decision":
            case "决策":
                DecisionFeed.Add(item);
                break;
        }
    }

    private static string LocalizeEventTitle(string type)
    {
        return type switch
        {
            "timeline" => "时间线",
            "run.started" => "任务开始",
            "run.completed" => "任务完成",
            _ => type
        };
    }

    private static string LocalizeRunStatus(string status)
    {
        return status switch
        {
            "completed" => "已完成",
            "success" => "成功",
            "done" => "完成",
            "failed" => "失败",
            "running" => "执行中",
            _ => status
        };
    }

    private static void Trim<T>(ObservableCollection<T> collection, int limit)
    {
        while (collection.Count > limit)
        {
            collection.RemoveAt(collection.Count - 1);
        }
    }
}
