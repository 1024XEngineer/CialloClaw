using System.IO;
using System.Net.Http;
using System.Text;
using System.Text.Json;
using CialloClaw.Shell.Models;

namespace CialloClaw.Shell.Services;

public sealed class SseClient : IDisposable
{
    private readonly HttpClient _httpClient;
    private readonly JsonSerializerOptions _jsonOptions;

    private CancellationTokenSource? _loopCts;
    private Task? _loopTask;

    public event EventHandler<SseEvent>? EventReceived;

    public SseClient(string? baseUrl = null)
    {
        _httpClient = new HttpClient
        {
            Timeout = Timeout.InfiniteTimeSpan,
            BaseAddress = new Uri(baseUrl ?? Environment.GetEnvironmentVariable("CIALLOCLAW_CORE_URL") ?? "http://127.0.0.1:18080")
        };

        _jsonOptions = new JsonSerializerOptions
        {
            PropertyNameCaseInsensitive = true
        };
    }

    public void Start(string sessionId, string agentId)
    {
        Stop();

        _loopCts = new CancellationTokenSource();
        _loopTask = Task.Run(() => RunLoopAsync(sessionId, agentId, _loopCts.Token));
    }

    public void Stop()
    {
        try
        {
            _loopCts?.Cancel();
            _loopTask?.Wait(TimeSpan.FromSeconds(1));
        }
        catch
        {
            // ignored
        }
        finally
        {
            _loopTask = null;
            _loopCts?.Dispose();
            _loopCts = null;
        }
    }

    public void Dispose()
    {
        Stop();
        _httpClient.Dispose();
    }

    private async Task RunLoopAsync(string sessionId, string agentId, CancellationToken cancellationToken)
    {
        while (!cancellationToken.IsCancellationRequested)
        {
            try
            {
                var query = $"/api/sse?session_id={Uri.EscapeDataString(sessionId)}&agent_id={Uri.EscapeDataString(agentId)}";
                using var request = new HttpRequestMessage(HttpMethod.Get, query);
                using var response = await _httpClient.SendAsync(request, HttpCompletionOption.ResponseHeadersRead, cancellationToken);
                response.EnsureSuccessStatusCode();

                await using var stream = await response.Content.ReadAsStreamAsync(cancellationToken);
                using var reader = new StreamReader(stream);

                var eventName = string.Empty;
                var dataBuilder = new StringBuilder();

                while (!reader.EndOfStream && !cancellationToken.IsCancellationRequested)
                {
                    var line = await reader.ReadLineAsync(cancellationToken);
                    if (line is null)
                    {
                        break;
                    }

                    if (line.StartsWith(":", StringComparison.Ordinal))
                    {
                        continue;
                    }

                    if (line.StartsWith("event:", StringComparison.OrdinalIgnoreCase))
                    {
                        eventName = line.Substring(6).Trim();
                        continue;
                    }

                    if (line.StartsWith("data:", StringComparison.OrdinalIgnoreCase))
                    {
                        dataBuilder.Append(line.Substring(5).TrimStart());
                        continue;
                    }

                    if (line.Length == 0 && dataBuilder.Length > 0)
                    {
                        var payload = dataBuilder.ToString();
                        dataBuilder.Clear();

                        var evt = JsonSerializer.Deserialize<SseEvent>(payload, _jsonOptions);
                        if (evt is not null)
                        {
                            if (string.IsNullOrWhiteSpace(evt.Type))
                            {
                                evt.Type = eventName;
                            }

                            EventReceived?.Invoke(this, evt);
                        }

                        eventName = string.Empty;
                    }
                }
            }
            catch (OperationCanceledException)
            {
                break;
            }
            catch
            {
                await Task.Delay(TimeSpan.FromSeconds(2), cancellationToken);
            }
        }
    }
}
