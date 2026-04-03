using System.Windows;

namespace CialloClaw.Shell.Views;

public partial class ControlPanelWindow : Window
{
    public ControlPanelWindow()
    {
        InitializeComponent();
        SelectDashboardHome();
    }

    public void SelectDashboardHome()
    {
        MainTabControl.SelectedIndex = 0;
    }

    private void CloseButton_OnClick(object sender, RoutedEventArgs e)
    {
        Hide();
    }

    protected override void OnClosing(System.ComponentModel.CancelEventArgs e)
    {
        e.Cancel = true;
        Hide();
    }
}
