namespace CialloClaw.Shell.Models;

public sealed class NoteTaskItem
{
    public string Id { get; set; } = string.Empty;
    public string Title { get; set; } = string.Empty;
    public string Status { get; set; } = string.Empty;
    public string Priority { get; set; } = string.Empty;
    public DateTime DueAt { get; set; }
    public List<string> Tags { get; set; } = new();
    public string SourceFile { get; set; } = string.Empty;
    public int PendingDays { get; set; }
    public string SuggestedNext { get; set; } = string.Empty;

    public string DueText => DueAt.ToString("MM-dd HH:mm");
    public string TagsText => string.Join("、", Tags);

    public string DisplayStatus => Status switch
    {
        "pending" => "待处理",
        "stale" => "已滞留",
        "done" => "已完成",
        "processing" => "处理中",
        _ => Status
    };
}
