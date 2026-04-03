namespace CialloClaw.Shell.Models;

public sealed class ChatMessage
{
    public string Id { get; set; } = Guid.NewGuid().ToString("N");
    public string Role { get; set; } = "assistant";
    public string Content { get; set; } = string.Empty;
    public DateTime Timestamp { get; set; } = DateTime.Now;
    public long? RunDurationMs { get; set; }

    public string DisplayTime => Timestamp.ToString("HH:mm:ss");
    public string DisplayRoleLabel => Role switch
    {
        "user" => "你的意图",
        "system" => "系统提示",
        _ => "助手回应"
    };

    public string PreviewText
    {
        get
        {
            if (string.IsNullOrWhiteSpace(Content))
            {
                return "空白消息";
            }

            return Content.Length <= 34 ? Content : $"{Content[..34]}...";
        }
    }

    public string DurationText => RunDurationMs is > 0 ? $"耗时 {RunDurationMs}ms" : string.Empty;
}
