using System.Collections.ObjectModel;
using System.Windows.Media;
using CialloClaw.Shell.Infrastructure;
using CialloClaw.Shell.Models;
using CialloClaw.Shell.Services;

namespace CialloClaw.Shell.ViewModels;

public sealed class FloatingBallViewModel : ObservableObject
{
    private const double SourceCanvasWidth = 253;
    private const double SourceCanvasHeight = 192;

    private const double LeftTailBaseX = 6;
    private const double LeftTailBaseY = 57;
    private const double RightTailBaseX = 133;
    private const double RightTailBaseY = 34;
    private const double FrontHairBaseX = 65;
    private const double FrontHairBaseY = 17;

    private const double LeftPupilBaseX = 102;
    private const double LeftPupilBaseY = 108;
    private const double RightPupilBaseX = 176;
    private const double RightPupilBaseY = 103;

    private const double BlinkLayerX = 33;
    private const double BlinkLayerY = 109;

    private readonly ObservableCollection<QuickTask> _tasks = new();
    private readonly Random _random = new();

    private AssistantState _state = AssistantState.Idle;
    private string _stateText = "空闲";
    private string _imagePath = "Assets/pixel/idle.png";
    private Brush _indicatorBrush = AssistantVisualMapper.ToIndicatorBrush(AssistantState.Idle);
    private double _ballOpacity = 0.78;
    private bool _isRingVisible;
    private bool _isPaused;
    private int _ballSize = 96;
    private string _currentAgentName = "生活助手";
    private double _avatarScale = 1.0;
    private double _avatarOffsetY;
    private double _avatarRotation;

    private double _leftTailX = LeftTailBaseX;
    private double _leftTailY = LeftTailBaseY;
    private double _rightTailX = RightTailBaseX;
    private double _rightTailY = RightTailBaseY;
    private double _frontHairX = FrontHairBaseX;
    private double _frontHairY = FrontHairBaseY;
    private double _tailHairOpacity = 0.92;
    private double _frontHairOpacity = 0.95;

    private double _eyeOffsetX;
    private double _eyeOffsetY;
    private double _leftPupilX = LeftPupilBaseX;
    private double _leftPupilY = LeftPupilBaseY;
    private double _rightPupilX = RightPupilBaseX;
    private double _rightPupilY = RightPupilBaseY;
    private bool _showEyeOverlay = true;
    private bool _isBlinkVisible;
    private double _blinkCountdownSec = 2.8;
    private int _blinkFramesRemaining;

    public FloatingBallViewModel()
    {
        RadialTasks = new ObservableCollection<RadialTaskItem>();

        TogglePauseCommand = new RelayCommand(_ => PauseToggled?.Invoke());
        OpenControlPanelCommand = new RelayCommand(_ => OpenControlPanelRequested?.Invoke());
        ExitCommand = new RelayCommand(_ => ExitRequested?.Invoke());
        OpenChatCommand = new RelayCommand(_ => OpenChatRequested?.Invoke());
        QuickTaskCommand = new RelayCommand(OnQuickTaskInvoked);
    }

    public event Action? OpenChatRequested;
    public event Action? OpenControlPanelRequested;
    public event Action? ExitRequested;
    public event Action? PauseToggled;
    public event Action<QuickTask>? QuickTaskInvoked;

    public ObservableCollection<RadialTaskItem> RadialTasks { get; }

    public string CurrentAgentName
    {
        get => _currentAgentName;
        set => SetProperty(ref _currentAgentName, value);
    }

    public string StateText
    {
        get => _stateText;
        private set => SetProperty(ref _stateText, value);
    }

    public string ImagePath
    {
        get => _imagePath;
        private set => SetProperty(ref _imagePath, value);
    }

    public Brush IndicatorBrush
    {
        get => _indicatorBrush;
        private set => SetProperty(ref _indicatorBrush, value);
    }

    public double BallOpacity
    {
        get => _ballOpacity;
        private set => SetProperty(ref _ballOpacity, value);
    }

    public bool IsRingVisible
    {
        get => _isRingVisible;
        set
        {
            if (value && _tasks.Count == 0)
            {
                value = false;
            }

            if (SetProperty(ref _isRingVisible, value))
            {
                RebuildRadialTasks();
            }
        }
    }

    public bool IsPaused
    {
        get => _isPaused;
        set
        {
            if (SetProperty(ref _isPaused, value))
            {
                SetState(value ? AssistantState.Paused : AssistantState.Idle);
            }
        }
    }

    public int BallSize
    {
        get => _ballSize;
        set => SetProperty(ref _ballSize, value);
    }

    public bool HasConfiguredTasks => _tasks.Count > 0;

    public double AvatarScale
    {
        get => _avatarScale;
        set => SetProperty(ref _avatarScale, value);
    }

    public double AvatarOffsetY
    {
        get => _avatarOffsetY;
        set => SetProperty(ref _avatarOffsetY, value);
    }

    public double AvatarRotation
    {
        get => _avatarRotation;
        set => SetProperty(ref _avatarRotation, value);
    }

    public double SourceCanvasWidthValue => SourceCanvasWidth;
    public double SourceCanvasHeightValue => SourceCanvasHeight;

    public string AvatarBaseLayerPath => "Assets/pixel/layers/processed/avatar_base.png";
    public string HairLeftLayerPath => "Assets/pixel/layers/processed/hair_left.png";
    public string HairRightLayerPath => "Assets/pixel/layers/processed/hair_right.png";
    public string HairFrontLayerPath => "Assets/pixel/layers/processed/hair_front.png";
    public string BlinkLayerPath => "Assets/pixel/layers/processed/blink.png";

    public double LeftTailX
    {
        get => _leftTailX;
        private set => SetProperty(ref _leftTailX, value);
    }

    public double LeftTailY
    {
        get => _leftTailY;
        private set => SetProperty(ref _leftTailY, value);
    }

    public double RightTailX
    {
        get => _rightTailX;
        private set => SetProperty(ref _rightTailX, value);
    }

    public double RightTailY
    {
        get => _rightTailY;
        private set => SetProperty(ref _rightTailY, value);
    }

    public double FrontHairX
    {
        get => _frontHairX;
        private set => SetProperty(ref _frontHairX, value);
    }

    public double FrontHairY
    {
        get => _frontHairY;
        private set => SetProperty(ref _frontHairY, value);
    }

    public double TailHairOpacity
    {
        get => _tailHairOpacity;
        private set => SetProperty(ref _tailHairOpacity, value);
    }

    public double FrontHairOpacity
    {
        get => _frontHairOpacity;
        private set => SetProperty(ref _frontHairOpacity, value);
    }

    public double LeftPupilX
    {
        get => _leftPupilX;
        private set => SetProperty(ref _leftPupilX, value);
    }

    public double LeftPupilY
    {
        get => _leftPupilY;
        private set => SetProperty(ref _leftPupilY, value);
    }

    public double RightPupilX
    {
        get => _rightPupilX;
        private set => SetProperty(ref _rightPupilX, value);
    }

    public double RightPupilY
    {
        get => _rightPupilY;
        private set => SetProperty(ref _rightPupilY, value);
    }

    public bool ShowEyeOverlay
    {
        get => _showEyeOverlay;
        private set => SetProperty(ref _showEyeOverlay, value);
    }

    public bool IsBlinkVisible
    {
        get => _isBlinkVisible;
        private set => SetProperty(ref _isBlinkVisible, value);
    }

    public double BlinkX => BlinkLayerX;
    public double BlinkY => BlinkLayerY;

    public AssistantState State => _state;

    public RelayCommand TogglePauseCommand { get; }
    public RelayCommand OpenControlPanelCommand { get; }
    public RelayCommand ExitCommand { get; }
    public RelayCommand OpenChatCommand { get; }
    public RelayCommand QuickTaskCommand { get; }

    public void SetTasks(IEnumerable<QuickTask> tasks)
    {
        _tasks.Clear();
        foreach (var task in tasks.Take(8))
        {
            _tasks.Add(task);
        }

        RaisePropertyChanged(nameof(HasConfiguredTasks));

        if (_tasks.Count == 0)
        {
            IsRingVisible = false;
        }

        RebuildRadialTasks();
    }

    public void SetState(AssistantState state)
    {
        _state = state;
        RaisePropertyChanged(nameof(State));
        StateText = AssistantVisualMapper.ToDisplayText(state);
        ImagePath = AssistantVisualMapper.ToImagePath(state);
        IndicatorBrush = AssistantVisualMapper.ToIndicatorBrush(state);
        BallOpacity = AssistantVisualMapper.ToOpacity(state);

        ShowEyeOverlay = state is AssistantState.Idle
            or AssistantState.Ready
            or AssistantState.Thinking
            or AssistantState.Executing
            or AssistantState.Attention
            or AssistantState.Completed;

        if (!ShowEyeOverlay)
        {
            IsBlinkVisible = false;
            _blinkFramesRemaining = 0;
            ResetPupilPosition();
        }
    }

    public void ApplyPulseFrame(double time)
    {
        var floatAmplitude = _state switch
        {
            AssistantState.Thinking => 2.7,
            AssistantState.Executing => 1.9,
            AssistantState.Attention => 2.1,
            AssistantState.Completed => 2.9,
            AssistantState.Music => 3.2,
            AssistantState.Paused => 0.5,
            AssistantState.Dnd => 0.7,
            _ => 1.4
        };

        var floatSpeed = _state switch
        {
            AssistantState.Thinking => 2.7,
            AssistantState.Executing => 3.2,
            AssistantState.Completed => 4.0,
            AssistantState.Music => 2.0,
            AssistantState.Paused => 0.9,
            _ => 1.65
        };

        var wave = Math.Sin(time * floatSpeed);
        AvatarOffsetY = wave * floatAmplitude;
        AvatarScale = 1.0 + (Math.Cos(time * floatSpeed * 0.68) * 0.016);
        AvatarRotation = Math.Sin(time * floatSpeed * 0.48) * 1.4;

        FrontHairX = FrontHairBaseX + Math.Round(Math.Sin(time * 2.35) * 1.1, 1);
        FrontHairY = FrontHairBaseY + Math.Round(Math.Cos(time * 2.9) * 0.8, 1);

        LeftTailX = LeftTailBaseX + Math.Round(Math.Sin(time * 1.9 + 0.45) * 1.4, 1);
        LeftTailY = LeftTailBaseY + Math.Round(Math.Cos(time * 2.25 + 0.2) * 0.9, 1);
        RightTailX = RightTailBaseX + Math.Round(Math.Sin(time * 1.8 + 1.12) * 1.3, 1);
        RightTailY = RightTailBaseY + Math.Round(Math.Cos(time * 2.12 + 1.1) * 0.9, 1);

        TailHairOpacity = 0.9 + (Math.Sin(time * 2.4) * 0.05);
        FrontHairOpacity = 0.93 + (Math.Cos(time * 2.8) * 0.04);
    }

    public void TickFrame(double deltaSeconds)
    {
        UpdateBlink(deltaSeconds);
    }

    public void ApplyEyeTracking(double mouseDeltaX, double mouseDeltaY)
    {
        if (!ShowEyeOverlay || IsBlinkVisible)
        {
            return;
        }

        var targetX = Math.Clamp(mouseDeltaX / 98.0, -1.0, 1.0) * 2.6;
        var targetY = Math.Clamp(mouseDeltaY / 98.0, -1.0, 1.0) * 1.8;

        _eyeOffsetX = Math.Round(((_eyeOffsetX * 5.0) + targetX) / 6.0, 2);
        _eyeOffsetY = Math.Round(((_eyeOffsetY * 5.0) + targetY) / 6.0, 2);

        LeftPupilX = LeftPupilBaseX + _eyeOffsetX;
        LeftPupilY = LeftPupilBaseY + _eyeOffsetY;
        RightPupilX = RightPupilBaseX + (_eyeOffsetX * 0.95);
        RightPupilY = RightPupilBaseY + (_eyeOffsetY * 0.9);
    }

    private void UpdateBlink(double deltaSeconds)
    {
        if (!ShowEyeOverlay)
        {
            IsBlinkVisible = false;
            _blinkFramesRemaining = 0;
            return;
        }

        if (_blinkFramesRemaining > 0)
        {
            _blinkFramesRemaining--;
            IsBlinkVisible = true;
            if (_blinkFramesRemaining == 0)
            {
                IsBlinkVisible = false;
            }

            return;
        }

        _blinkCountdownSec -= deltaSeconds;
        if (_blinkCountdownSec > 0)
        {
            return;
        }

        _blinkFramesRemaining = 2;
        IsBlinkVisible = true;
        _blinkCountdownSec = 2.4 + (_random.NextDouble() * 4.9);
    }

    private void ResetPupilPosition()
    {
        _eyeOffsetX = 0;
        _eyeOffsetY = 0;
        LeftPupilX = LeftPupilBaseX;
        LeftPupilY = LeftPupilBaseY;
        RightPupilX = RightPupilBaseX;
        RightPupilY = RightPupilBaseY;
    }

    private void RebuildRadialTasks()
    {
        RadialTasks.Clear();
        if (!IsRingVisible)
        {
            return;
        }

        var count = _tasks.Count;
        if (count == 0)
        {
            return;
        }

        const double center = 130;
        const double radius = 98;
        const double buttonHalf = 21;
        var startAngle = -90d;
        var step = 360d / count;

        for (var i = 0; i < count; i++)
        {
            var task = _tasks[i];
            var angle = (startAngle + (step * i)) * Math.PI / 180d;
            var x = center + (radius * Math.Cos(angle)) - buttonHalf;
            var y = center + (radius * Math.Sin(angle)) - buttonHalf;

            RadialTasks.Add(new RadialTaskItem
            {
                Task = task,
                X = x,
                Y = y
            });
        }
    }

    private void OnQuickTaskInvoked(object? parameter)
    {
        if (parameter is QuickTask task)
        {
            QuickTaskInvoked?.Invoke(task);
        }
    }
}
