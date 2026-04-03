using System.Diagnostics;
using System.IO;
using System.Net.Http;

namespace CialloClaw.Shell.Services;

public sealed class CoreProcessManager : IDisposable
{
    private readonly HttpClient _httpClient = new()
    {
        Timeout = TimeSpan.FromSeconds(2)
    };

    private readonly Uri _baseUri;
    private Process? _ownedCoreProcess;

    public CoreProcessManager(string? baseUrl = null)
    {
        var url = baseUrl ?? Environment.GetEnvironmentVariable("CIALLOCLAW_CORE_URL") ?? "http://127.0.0.1:18080";
        _baseUri = new Uri(url);
    }

    public async Task EnsureCoreReadyAsync(CancellationToken cancellationToken)
    {
        if (await IsHealthyAsync(cancellationToken))
        {
            return;
        }

        var corePath = ResolveBundledCorePath();
        if (corePath is null)
        {
            throw new InvalidOperationException(
                "Core service is not running, and bundled core executable was not found. Expected core/go-core.exe next to CialloClaw.exe.");
        }

        _ownedCoreProcess = StartCoreProcess(corePath);

        var deadline = DateTime.UtcNow.AddSeconds(12);
        while (DateTime.UtcNow < deadline)
        {
            cancellationToken.ThrowIfCancellationRequested();

            if (_ownedCoreProcess.HasExited)
            {
                throw new InvalidOperationException($"Bundled core exited unexpectedly with code {_ownedCoreProcess.ExitCode}.");
            }

            if (await IsHealthyAsync(cancellationToken))
            {
                return;
            }

            await Task.Delay(250, cancellationToken);
        }

        throw new TimeoutException("Timed out waiting for bundled core service to become healthy.");
    }

    public Task StopOwnedCoreAsync()
    {
        if (_ownedCoreProcess is not null)
        {
            try
            {
                if (!_ownedCoreProcess.HasExited)
                {
                    _ownedCoreProcess.Kill(entireProcessTree: true);
                    _ownedCoreProcess.WaitForExit(2000);
                }
            }
            catch
            {
                // ignored
            }
            finally
            {
                _ownedCoreProcess.Dispose();
                _ownedCoreProcess = null;
            }
        }

        return Task.CompletedTask;
    }

    public void Dispose()
    {
        _httpClient.Dispose();
        _ownedCoreProcess?.Dispose();
        _ownedCoreProcess = null;
    }

    private async Task<bool> IsHealthyAsync(CancellationToken cancellationToken)
    {
        try
        {
            using var response = await _httpClient.GetAsync(new Uri(_baseUri, "/health"), cancellationToken);
            return response.IsSuccessStatusCode;
        }
        catch
        {
            return false;
        }
    }

    private static string? ResolveBundledCorePath()
    {
        var candidates = new[]
        {
            Path.Combine(AppContext.BaseDirectory, "core", "go-core.exe"),
            Path.Combine(AppContext.BaseDirectory, "go-core.exe"),
            Path.GetFullPath(Path.Combine(AppContext.BaseDirectory, "..", "..", "..", "..", "..", "go-core", "go-core.exe"))
        };

        return candidates.FirstOrDefault(File.Exists);
    }

    private static Process StartCoreProcess(string corePath)
    {
        var startInfo = new ProcessStartInfo
        {
            FileName = corePath,
            WorkingDirectory = Path.GetDirectoryName(corePath) ?? AppContext.BaseDirectory,
            UseShellExecute = false,
            CreateNoWindow = true,
            WindowStyle = ProcessWindowStyle.Hidden
        };

        var process = Process.Start(startInfo);
        if (process is null)
        {
            throw new InvalidOperationException("Failed to launch bundled core process.");
        }

        return process;
    }
}
