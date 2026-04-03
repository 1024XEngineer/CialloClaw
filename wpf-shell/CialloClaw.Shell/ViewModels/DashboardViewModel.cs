using System.Collections.ObjectModel;
using System.Text.Json;
using CialloClaw.Shell.Infrastructure;
using CialloClaw.Shell.Models;

namespace CialloClaw.Shell.ViewModels;

public sealed class DashboardViewModel : ObservableObject
{
    public DashboardViewModel()
    {
        Timeline = new ObservableCollection<TimelineItem>();
        ToolCalls = new ObservableCollection<ToolCallItem>();
        Citations = new ObservableCollection<CitationItem>();

        BrowserFeed = new ObservableCollection<DashboardEventItem>();
        TerminalFeed = new ObservableCollection<DashboardEventItem>();
        FileFeed = new ObservableCollection<DashboardEventItem>();
        DecisionFeed = new ObservableCollection<DashboardEventItem>();
    }

    public ObservableCollection<TimelineItem> Timeline { get; }
    public ObservableCollection<ToolCallItem> ToolCalls { get; }
    public ObservableCollection<CitationItem> Citations { get; }

    public ObservableCollection<DashboardEventItem> BrowserFeed { get; }
    public ObservableCollection<DashboardEventItem> TerminalFeed { get; }
    public ObservableCollection<DashboardEventItem> FileFeed { get; }
    public ObservableCollection<DashboardEventItem> DecisionFeed { get; }

    public void LoadSnapshot(IEnumerable<DashboardEventItem> snapshot)
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

    public void ApplyEvent(SseEvent evt)
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
                    Title = evt.Data.GetPropertyOrDefault("title", evt.Type),
                    Detail = evt.Data.GetPropertyOrDefault("status", evt.Data.GetPropertyOrDefault("goal", string.Empty))
                });
                Trim(Timeline, 120);
                break;

            case "tool.call":
                ToolCalls.Insert(0, new ToolCallItem
                {
                    Timestamp = timestamp,
                    Name = evt.Data.GetPropertyOrDefault("name", "tool"),
                    Status = evt.Data.GetPropertyOrDefault("status", "done"),
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
                    Category = "Decision",
                    Primary = evt.Data.GetPropertyOrDefault("goal", "当前目标"),
                    Detail = evt.Data.GetPropertyOrDefault("next", "") + " | " + evt.Data.GetPropertyOrDefault("risk", "")
                });
                Trim(DecisionFeed, 80);
                break;

            case "tool.browser":
                BrowserFeed.Insert(0, new DashboardEventItem
                {
                    Timestamp = timestamp,
                    Category = "Browser",
                    Primary = evt.Data.GetPropertyOrDefault("url", ""),
                    Detail = evt.Data.GetPropertyOrDefault("action", "")
                });
                Trim(BrowserFeed, 80);
                break;

            case "tool.terminal":
                TerminalFeed.Insert(0, new DashboardEventItem
                {
                    Timestamp = timestamp,
                    Category = "Terminal",
                    Primary = evt.Data.GetPropertyOrDefault("cmd", ""),
                    Detail = evt.Data.GetPropertyOrDefault("stdout", "")
                });
                Trim(TerminalFeed, 80);
                break;

            case "tool.file":
                FileFeed.Insert(0, new DashboardEventItem
                {
                    Timestamp = timestamp,
                    Category = "File",
                    Primary = evt.Data.GetPropertyOrDefault("path", ""),
                    Detail = evt.Data.GetPropertyOrDefault("action", "")
                });
                Trim(FileFeed, 80);
                break;
        }
    }

    private void AddDashboardItem(DashboardEventItem item)
    {
        switch (item.Category)
        {
            case "Browser":
                BrowserFeed.Add(item);
                break;
            case "Terminal":
                TerminalFeed.Add(item);
                break;
            case "File":
                FileFeed.Add(item);
                break;
            case "Decision":
                DecisionFeed.Add(item);
                break;
        }
    }

    private static void Trim<T>(ObservableCollection<T> collection, int limit)
    {
        while (collection.Count > limit)
        {
            collection.RemoveAt(collection.Count - 1);
        }
    }
}

internal static class DashboardJsonExtensions
{
    public static string GetPropertyOrDefault(this JsonElement element, string name, string fallback)
    {
        if (element.ValueKind == JsonValueKind.Null || element.ValueKind == JsonValueKind.Undefined)
        {
            return fallback;
        }

        if (!element.TryGetProperty(name, out var value))
        {
            return fallback;
        }

        return value.ValueKind switch
        {
            JsonValueKind.String => value.GetString() ?? fallback,
            JsonValueKind.Number => value.GetRawText(),
            JsonValueKind.Object => value.GetRawText(),
            JsonValueKind.Array => value.GetRawText(),
            JsonValueKind.True => "true",
            JsonValueKind.False => "false",
            _ => fallback
        };
    }

    public static long GetPropertyOrDefaultLong(this JsonElement element, string name, long fallback)
    {
        if (!element.TryGetProperty(name, out var value))
        {
            return fallback;
        }

        if (value.ValueKind == JsonValueKind.Number && value.TryGetInt64(out var intValue))
        {
            return intValue;
        }

        if (value.ValueKind == JsonValueKind.String && long.TryParse(value.GetString(), out intValue))
        {
            return intValue;
        }

        return fallback;
    }
}
