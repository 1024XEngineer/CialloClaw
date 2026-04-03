using System.Windows;

namespace CialloClaw.Shell.Views;

public partial class ReminderWindow : Window
{
    public ReminderWindow()
    {
        InitializeComponent();
    }

    protected override void OnClosing(System.ComponentModel.CancelEventArgs e)
    {
        e.Cancel = true;
        Hide();
    }

    public void PositionNear(Window anchor)
    {
        var screen = SystemParameters.WorkArea;

        Left = anchor.Left + anchor.Width + 8;
        Top = anchor.Top + 20;

        if (Left + Width > screen.Right)
        {
            Left = anchor.Left - Width - 8;
        }

        if (Left < screen.Left)
        {
            Left = screen.Left + 8;
        }

        if (Top + Height > screen.Bottom)
        {
            Top = screen.Bottom - Height - 8;
        }

        if (Top < screen.Top)
        {
            Top = screen.Top + 8;
        }
    }
}
