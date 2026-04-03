using System.Runtime.InteropServices;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Controls.Primitives;
using System.Windows.Input;
using System.Windows.Media;
using System.Windows.Threading;
using CialloClaw.Shell.ViewModels;

namespace CialloClaw.Shell.Views;

public partial class FloatingBallWindow : Window
{
    [StructLayout(LayoutKind.Sequential)]
    private struct NativePoint
    {
        public int X;
        public int Y;
    }

    [DllImport("user32.dll")]
    [return: MarshalAs(UnmanagedType.Bool)]
    private static extern bool GetCursorPos(out NativePoint point);

    private readonly DispatcherTimer _showRingTimer;
    private readonly DispatcherTimer _hideRingTimer;
    private readonly DispatcherTimer _pulseTimer;
    private readonly DispatcherTimer _showCompanionTimer;
    private readonly DispatcherTimer _hideCompanionTimer;

    private Point _mouseDownPoint;
    private bool _isMouseDown;
    private bool _isDragging;
    private bool _companionPinned;
    private double _phase;

    public FloatingBallWindow()
    {
        InitializeComponent();

        _showRingTimer = new DispatcherTimer { Interval = TimeSpan.FromMilliseconds(250) };
        _showRingTimer.Tick += (_, _) =>
        {
            _showRingTimer.Stop();
            if (Vm is not null && Vm.HasConfiguredTasks)
            {
                Vm.IsRingVisible = true;
            }
        };

        _hideRingTimer = new DispatcherTimer { Interval = TimeSpan.FromMilliseconds(170) };
        _hideRingTimer.Tick += (_, _) =>
        {
            _hideRingTimer.Stop();
            if (Vm is not null)
            {
                Vm.IsRingVisible = false;
            }
        };

        _showCompanionTimer = new DispatcherTimer { Interval = TimeSpan.FromMilliseconds(120) };
        _showCompanionTimer.Tick += (_, _) =>
        {
            _showCompanionTimer.Stop();
            OpenCompanion();
        };

        _hideCompanionTimer = new DispatcherTimer { Interval = TimeSpan.FromMilliseconds(240) };
        _hideCompanionTimer.Tick += (_, _) =>
        {
            _hideCompanionTimer.Stop();
            if (_companionPinned || IsPointerOverInteractiveSurface())
            {
                return;
            }

            CompanionPopup.IsOpen = false;
        };

        _pulseTimer = new DispatcherTimer { Interval = TimeSpan.FromMilliseconds(48) };
        _pulseTimer.Tick += (_, _) =>
        {
            _phase += 0.11;
            Vm?.ApplyPulseFrame(_phase);
            Vm?.TickFrame(_pulseTimer.Interval.TotalSeconds);
            UpdateEyeTracking();
        };
        _pulseTimer.Start();

        Left = SystemParameters.WorkArea.Width - Width - 36;
        Top = SystemParameters.WorkArea.Height * 0.35;
    }

    public event Action? BallMoved;

    public object? CompanionDataContext
    {
        get => CompanionHost.DataContext;
        set => CompanionHost.DataContext = value;
    }

    private FloatingBallViewModel? Vm => DataContext as FloatingBallViewModel;
    private ChatViewModel? ChatVm => CompanionDataContext as ChatViewModel;

    private void BallHitArea_OnMouseLeftButtonDown(object sender, MouseButtonEventArgs e)
    {
        _isMouseDown = true;
        _isDragging = false;
        _mouseDownPoint = e.GetPosition(this);
        BallHitArea.CaptureMouse();
    }

    private void BallHitArea_OnMouseMove(object sender, MouseEventArgs e)
    {
        if (!_isMouseDown || e.LeftButton != MouseButtonState.Pressed)
        {
            return;
        }

        var current = e.GetPosition(this);
        if (Math.Abs(current.X - _mouseDownPoint.X) + Math.Abs(current.Y - _mouseDownPoint.Y) < 6)
        {
            return;
        }

        _isDragging = true;
        BallHitArea.ReleaseMouseCapture();

        var restoreCompanion = CompanionPopup.IsOpen || _companionPinned;
        CompanionPopup.IsOpen = false;

        try
        {
            DragMove();
            SnapToEdge();
            BallMoved?.Invoke();
        }
        catch
        {
            // ignored
        }
        finally
        {
            _isMouseDown = false;
            if (restoreCompanion)
            {
                OpenCompanion();
            }
        }
    }

    private void BallHitArea_OnMouseLeftButtonUp(object sender, MouseButtonEventArgs e)
    {
        BallHitArea.ReleaseMouseCapture();

        var shouldHandleClick = _isMouseDown && !_isDragging;
        _isMouseDown = false;
        _isDragging = false;

        if (!shouldHandleClick)
        {
            return;
        }

        if (e.ClickCount >= 2)
        {
            Vm?.OpenChatCommand.Execute(null);
            return;
        }

        ToggleCompanionPinned();
    }

    private void RootGrid_OnMouseEnter(object sender, MouseEventArgs e)
    {
        _hideRingTimer.Stop();
        _hideCompanionTimer.Stop();

        if (Vm?.HasConfiguredTasks == true)
        {
            _showRingTimer.Start();
        }

        _showCompanionTimer.Start();
    }

    private void RootGrid_OnMouseLeave(object sender, MouseEventArgs e)
    {
        _showRingTimer.Stop();
        _showCompanionTimer.Stop();
        _hideRingTimer.Start();
        ScheduleCompanionClose();
    }

    private void BallHitArea_OnMouseEnter(object sender, MouseEventArgs e)
    {
        _hideRingTimer.Stop();
        _hideCompanionTimer.Stop();
        _showCompanionTimer.Start();
    }

    private void BallHitArea_OnMouseLeave(object sender, MouseEventArgs e)
    {
        _hideRingTimer.Start();
        ScheduleCompanionClose();
    }

    private void CompanionHost_OnMouseEnter(object sender, MouseEventArgs e)
    {
        _hideCompanionTimer.Stop();
    }

    private void CompanionHost_OnMouseLeave(object sender, MouseEventArgs e)
    {
        ScheduleCompanionClose();
    }

    private void OpenStageButton_OnClick(object sender, RoutedEventArgs e)
    {
        Vm?.OpenChatCommand.Execute(null);
    }

    private void QuickChip_OnClick(object sender, RoutedEventArgs e)
    {
        if (ChatVm is null || sender is not Button button || button.Tag is not string prompt)
        {
            return;
        }

        ChatVm.InputText = prompt;
        ChatVm.SendCommand.Execute(null);
    }

    private void CompanionInputBox_OnKeyDown(object sender, KeyEventArgs e)
    {
        if (e.Key == Key.Enter && (Keyboard.Modifiers & ModifierKeys.Shift) == 0)
        {
            ChatVm?.SendCommand.Execute(null);
            e.Handled = true;
        }
    }

    private CustomPopupPlacement[] CompanionPopup_CustomPopupPlacement(Size popupSize, Size targetSize, Point offset)
    {
        var workArea = SystemParameters.WorkArea;
        var ballCenterX = Left + (Width / 2.0);
        var showRight = ballCenterX < (workArea.Left + (workArea.Width / 2.0));
        var x = showRight ? targetSize.Width + 18 : -popupSize.Width - 18;
        var y = -((popupSize.Height - targetSize.Height) / 2.0);

        var targetTopOnScreen = Top + ((Height - targetSize.Height) / 2.0);
        var popupTopOnScreen = targetTopOnScreen + y;
        if (popupTopOnScreen < workArea.Top + 8)
        {
            y += (workArea.Top + 8) - popupTopOnScreen;
        }

        var popupBottomOnScreen = targetTopOnScreen + y + popupSize.Height;
        if (popupBottomOnScreen > workArea.Bottom - 8)
        {
            y -= popupBottomOnScreen - (workArea.Bottom - 8);
        }

        return
        [
            new CustomPopupPlacement(new Point(x, y), PopupPrimaryAxis.None)
        ];
    }

    private void SnapToEdge()
    {
        var workArea = SystemParameters.WorkArea;
        const double snapThreshold = 16;

        var leftDistance = Math.Abs(Left - workArea.Left);
        var rightDistance = Math.Abs((workArea.Right - Width) - Left);
        var topDistance = Math.Abs(Top - workArea.Top);
        var bottomDistance = Math.Abs((workArea.Bottom - Height) - Top);

        var min = Math.Min(Math.Min(leftDistance, rightDistance), Math.Min(topDistance, bottomDistance));

        if (min > snapThreshold)
        {
            Left = Math.Max(workArea.Left, Math.Min(Left, workArea.Right - Width));
            Top = Math.Max(workArea.Top, Math.Min(Top, workArea.Bottom - Height));
            return;
        }

        if (Math.Abs(min - leftDistance) < 0.1)
        {
            Left = workArea.Left + 4;
        }
        else if (Math.Abs(min - rightDistance) < 0.1)
        {
            Left = workArea.Right - Width - 4;
        }
        else if (Math.Abs(min - topDistance) < 0.1)
        {
            Top = workArea.Top + 4;
        }
        else
        {
            Top = workArea.Bottom - Height - 4;
        }

        if (Top < workArea.Top)
        {
            Top = workArea.Top;
        }

        if (Top + Height > workArea.Bottom)
        {
            Top = workArea.Bottom - Height;
        }
    }

    private void UpdateEyeTracking()
    {
        if (Vm is null || !IsLoaded)
        {
            return;
        }

        if (!GetCursorPos(out var point))
        {
            return;
        }

        var dpi = VisualTreeHelper.GetDpi(this);
        var cursorX = point.X / dpi.DpiScaleX;
        var cursorY = point.Y / dpi.DpiScaleY;

        var centerX = Left + (Width / 2.0);
        var centerY = Top + (Height / 2.0);

        Vm.ApplyEyeTracking(cursorX - centerX, cursorY - centerY);
    }

    protected override void OnClosed(EventArgs e)
    {
        _showRingTimer.Stop();
        _hideRingTimer.Stop();
        _showCompanionTimer.Stop();
        _hideCompanionTimer.Stop();
        _pulseTimer.Stop();
        base.OnClosed(e);
    }

    private void ToggleCompanionPinned()
    {
        _companionPinned = !_companionPinned;
        if (_companionPinned)
        {
            OpenCompanion();
            return;
        }

        ScheduleCompanionClose();
    }

    private void OpenCompanion()
    {
        _hideCompanionTimer.Stop();
        if (!CompanionPopup.IsOpen)
        {
            CompanionPopup.IsOpen = true;
        }
    }

    private void ScheduleCompanionClose()
    {
        _showCompanionTimer.Stop();
        if (_companionPinned)
        {
            return;
        }

        _hideCompanionTimer.Start();
    }

    private bool IsPointerOverInteractiveSurface()
    {
        return RootGrid.IsMouseOver || BallHitArea.IsMouseOver || CompanionHost.IsMouseOver;
    }
}
