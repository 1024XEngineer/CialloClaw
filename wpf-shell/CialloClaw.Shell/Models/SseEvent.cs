using System.Text.Json;

namespace CialloClaw.Shell.Models;

public sealed class SseEvent
{
    public string Id { get; set; } = string.Empty;
    public string Type { get; set; } = string.Empty;
    public DateTime Timestamp { get; set; }
    public string SessionId { get; set; } = string.Empty;
    public string AgentId { get; set; } = string.Empty;
    public string RunId { get; set; } = string.Empty;
    public int Level { get; set; }
    public JsonElement Data { get; set; }
}
