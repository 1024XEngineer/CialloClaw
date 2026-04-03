using System.Net.Http;
using System.Net.Http.Json;
using System.Text;
using System.Text.Json;
using CialloClaw.Shell.Models;

namespace CialloClaw.Shell.Services;

public sealed class CoreApiClient : IDisposable
{
    private readonly HttpClient _httpClient;
    private readonly JsonSerializerOptions _jsonOptions;

    public CoreApiClient(string? baseUrl = null)
    {
        _httpClient = new HttpClient
        {
            BaseAddress = new Uri(baseUrl ?? Environment.GetEnvironmentVariable("CIALLOCLAW_CORE_URL") ?? "http://127.0.0.1:18080")
        };

        _jsonOptions = new JsonSerializerOptions
        {
            PropertyNameCaseInsensitive = true
        };
    }

    public async Task<IReadOnlyList<AgentProfile>> GetAgentsAsync(CancellationToken cancellationToken)
    {
        var payload = await _httpClient.GetFromJsonAsync<AgentsPayload>("/api/agents", _jsonOptions, cancellationToken);
        return payload?.Agents ?? [];
    }

    public async Task<IReadOnlyList<QuickTask>> GetQuickTasksAsync(string agentId, CancellationToken cancellationToken)
    {
        var payload = await _httpClient.GetFromJsonAsync<QuickTasksPayload>($"/api/quick-tasks?agent_id={Uri.EscapeDataString(agentId)}", _jsonOptions, cancellationToken);
        return payload?.Tasks ?? [];
    }

    public async Task<IReadOnlyList<SkillItem>> GetSkillsAsync(CancellationToken cancellationToken)
    {
        var payload = await _httpClient.GetFromJsonAsync<SkillsPayload>("/api/skills", _jsonOptions, cancellationToken);
        return payload?.Skills ?? [];
    }

    public async Task<AppSettingsModel> GetSettingsAsync(CancellationToken cancellationToken)
    {
        return await _httpClient.GetFromJsonAsync<AppSettingsModel>("/api/settings", _jsonOptions, cancellationToken) ?? new AppSettingsModel();
    }

    public async Task<IReadOnlyList<NoteTaskItem>> InspectTasksAsync(CancellationToken cancellationToken)
    {
        var payload = await _httpClient.GetFromJsonAsync<InspectTasksPayload>("/api/tasks/inspect", _jsonOptions, cancellationToken);
        return payload?.Tasks ?? [];
    }

    public async Task<string> SendChatAsync(string sessionId, string agentId, string text, CancellationToken cancellationToken)
    {
        var body = JsonSerializer.Serialize(new
        {
            session_id = sessionId,
            agent_id = agentId,
            text
        });

        using var content = new StringContent(body, Encoding.UTF8, "application/json");
        using var response = await _httpClient.PostAsync("/api/chat", content, cancellationToken);
        response.EnsureSuccessStatusCode();

        var payload = await response.Content.ReadFromJsonAsync<ChatResponsePayload>(_jsonOptions, cancellationToken);
        return payload?.RunId ?? string.Empty;
    }

    public async Task<IReadOnlyList<DashboardEventItem>> GetDashboardSnapshotAsync(CancellationToken cancellationToken)
    {
        using var response = await _httpClient.GetAsync("/api/dashboard/snapshot", cancellationToken);
        response.EnsureSuccessStatusCode();
        using var stream = await response.Content.ReadAsStreamAsync(cancellationToken);

        using var document = await JsonDocument.ParseAsync(stream, cancellationToken: cancellationToken);
        var events = new List<DashboardEventItem>();

        if (document.RootElement.TryGetProperty("browser_feed", out var browserFeed))
        {
            foreach (var item in browserFeed.EnumerateArray())
            {
                events.Add(new DashboardEventItem
                {
                    Timestamp = item.GetPropertyOrDefault("timestamp", DateTime.Now),
                    Category = "浏览器",
                    Primary = item.GetPropertyOrDefault("url", ""),
                    Detail = item.GetPropertyOrDefault("action", "") + " | " + item.GetPropertyOrDefault("note", "")
                });
            }
        }

        if (document.RootElement.TryGetProperty("terminal_feed", out var terminalFeed))
        {
            foreach (var item in terminalFeed.EnumerateArray())
            {
                events.Add(new DashboardEventItem
                {
                    Timestamp = item.GetPropertyOrDefault("timestamp", DateTime.Now),
                    Category = "终端",
                    Primary = item.GetPropertyOrDefault("cmd", ""),
                    Detail = item.GetPropertyOrDefault("stdout", "")
                });
            }
        }

        if (document.RootElement.TryGetProperty("file_feed", out var fileFeed))
        {
            foreach (var item in fileFeed.EnumerateArray())
            {
                events.Add(new DashboardEventItem
                {
                    Timestamp = item.GetPropertyOrDefault("timestamp", DateTime.Now),
                    Category = "文件",
                    Primary = item.GetPropertyOrDefault("path", ""),
                    Detail = item.GetPropertyOrDefault("action", "") + " | " + item.GetPropertyOrDefault("diff", "")
                });
            }
        }

        if (document.RootElement.TryGetProperty("decision_feed", out var decisionFeed))
        {
            foreach (var item in decisionFeed.EnumerateArray())
            {
                events.Add(new DashboardEventItem
                {
                    Timestamp = item.GetPropertyOrDefault("timestamp", DateTime.Now),
                    Category = "决策",
                    Primary = item.GetPropertyOrDefault("goal", ""),
                    Detail = item.GetPropertyOrDefault("next", "") + " | 风险：" + item.GetPropertyOrDefault("risk", "")
                });
            }
        }

        return events.OrderByDescending(item => item.Timestamp).ToList();
    }

    public void Dispose()
    {
        _httpClient.Dispose();
    }

    private sealed class AgentsPayload
    {
        public List<AgentProfile> Agents { get; set; } = new();
    }

    private sealed class QuickTasksPayload
    {
        public List<QuickTask> Tasks { get; set; } = new();
    }

    private sealed class SkillsPayload
    {
        public List<SkillItem> Skills { get; set; } = new();
    }

    private sealed class InspectTasksPayload
    {
        public List<NoteTaskItem> Tasks { get; set; } = new();
    }

    private sealed class ChatResponsePayload
    {
        public bool Accepted { get; set; }
        public string RunId { get; set; } = string.Empty;
    }
}

internal static class JsonExtensions
{
    public static string GetPropertyOrDefault(this JsonElement element, string name, string fallback)
    {
        if (!element.TryGetProperty(name, out var value))
        {
            return fallback;
        }

        return value.ValueKind switch
        {
            JsonValueKind.String => value.GetString() ?? fallback,
            JsonValueKind.Number => value.GetRawText(),
            JsonValueKind.True => "是",
            JsonValueKind.False => "否",
            _ => fallback
        };
    }

    public static DateTime GetPropertyOrDefault(this JsonElement element, string name, DateTime fallback)
    {
        if (!element.TryGetProperty(name, out var value))
        {
            return fallback;
        }

        if (value.ValueKind == JsonValueKind.String && DateTime.TryParse(value.GetString(), out var dt))
        {
            return dt;
        }

        return fallback;
    }
}
