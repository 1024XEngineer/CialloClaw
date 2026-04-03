namespace CialloClaw.Shell.Models;

public sealed class TimelineItem
{
    public DateTime Timestamp { get; set; } = DateTime.Now;
    public string Type { get; set; } = string.Empty;
    public string Title { get; set; } = string.Empty;
    public string Detail { get; set; } = string.Empty;

    public string TimestampText => Timestamp.ToString("HH:mm:ss");
}

public sealed class ToolCallItem
{
    public DateTime Timestamp { get; set; } = DateTime.Now;
    public string Name { get; set; } = string.Empty;
    public string Status { get; set; } = string.Empty;
    public string ArgsText { get; set; } = string.Empty;
    public long ElapsedMs { get; set; }
}

public sealed class CitationItem
{
    public DateTime Timestamp { get; set; } = DateTime.Now;
    public string Title { get; set; } = string.Empty;
    public string Source { get; set; } = string.Empty;
    public string Snippet { get; set; } = string.Empty;
}

public sealed class DashboardEventItem
{
    public DateTime Timestamp { get; set; } = DateTime.Now;
    public string Category { get; set; } = string.Empty;
    public string Primary { get; set; } = string.Empty;
    public string Detail { get; set; } = string.Empty;

    public string TimestampText => Timestamp.ToString("HH:mm:ss");
}
