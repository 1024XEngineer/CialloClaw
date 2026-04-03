using System.Windows;
using CialloClaw.Shell.Services;

namespace CialloClaw.Shell;

public partial class App : Application
{
    private ShellHost? _host;
    private CoreProcessManager? _coreManager;
    private readonly CancellationTokenSource _cts = new();

    protected override async void OnStartup(StartupEventArgs e)
    {
        base.OnStartup(e);

        ShutdownMode = ShutdownMode.OnExplicitShutdown;
        _coreManager = new CoreProcessManager();
        _host = new ShellHost();

        try
        {
            await _coreManager.EnsureCoreReadyAsync(_cts.Token);
            await _host.StartAsync();
        }
        catch (Exception ex)
        {
            MessageBox.Show($"应用启动失败：{ex.Message}", "CialloClaw", MessageBoxButton.OK, MessageBoxImage.Error);
            Shutdown();
        }
    }

    protected override async void OnExit(ExitEventArgs e)
    {
        _cts.Cancel();

        if (_host is not null)
        {
            await _host.StopAsync();
        }

        if (_coreManager is not null)
        {
            await _coreManager.StopOwnedCoreAsync();
            _coreManager.Dispose();
        }

        _cts.Dispose();
        base.OnExit(e);
    }
}
