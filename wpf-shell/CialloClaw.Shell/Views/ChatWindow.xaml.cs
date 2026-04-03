using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using CialloClaw.Shell.ViewModels;

namespace CialloClaw.Shell.Views;

public partial class ChatWindow : Window
{
    private ChatViewModel? Vm => DataContext as ChatViewModel;

    public ChatWindow()
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

        Left = anchor.Left + anchor.Width - 20;
        Top = anchor.Top - 18;

        if (Left + Width > screen.Right)
        {
            Left = anchor.Left - Width + 56;
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

    private void QuickChip_OnClick(object sender, RoutedEventArgs e)
    {
        if (Vm is null || sender is not Button button || button.Tag is not string prompt)
        {
            return;
        }

        Vm.InputText = prompt;
        Vm.SendCommand.Execute(null);
    }

    private void InputBox_OnKeyDown(object sender, KeyEventArgs e)
    {
        if (e.Key == Key.Enter && (Keyboard.Modifiers & ModifierKeys.Shift) == 0)
        {
            Vm?.SendCommand.Execute(null);
            e.Handled = true;
        }
    }
}
