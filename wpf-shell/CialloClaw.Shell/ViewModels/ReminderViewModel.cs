using CialloClaw.Shell.Infrastructure;

namespace CialloClaw.Shell.ViewModels;

public sealed class ReminderViewModel : ObservableObject
{
    private string _title = "主动协助提醒";
    private string _message = string.Empty;
    private int _level = 1;

    public ReminderViewModel()
    {
        ExecuteCommand = new RelayCommand(_ => ExecuteRequested?.Invoke());
        LaterCommand = new RelayCommand(_ => LaterRequested?.Invoke());
        DisableCommand = new RelayCommand(_ => DisableRequested?.Invoke());
    }

    public event Action? ExecuteRequested;
    public event Action? LaterRequested;
    public event Action? DisableRequested;

    public string Title
    {
        get => _title;
        set => SetProperty(ref _title, value);
    }

    public string Message
    {
        get => _message;
        set => SetProperty(ref _message, value);
    }

    public int Level
    {
        get => _level;
        set => SetProperty(ref _level, value);
    }

    public RelayCommand ExecuteCommand { get; }
    public RelayCommand LaterCommand { get; }
    public RelayCommand DisableCommand { get; }
}
