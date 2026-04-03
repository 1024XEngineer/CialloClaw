using System.Windows.Media;
using CialloClaw.Shell.Models;

namespace CialloClaw.Shell.Services;

public static class AssistantVisualMapper
{
    public static AssistantState Parse(string? raw)
    {
        return (raw ?? string.Empty).Trim().ToLowerInvariant() switch
        {
            "idle" => AssistantState.Idle,
            "ready" => AssistantState.Ready,
            "thinking" => AssistantState.Thinking,
            "executing" => AssistantState.Executing,
            "attention" => AssistantState.Attention,
            "need-user" => AssistantState.Attention,
            "error" => AssistantState.Error,
            "failed" => AssistantState.Error,
            "completed" => AssistantState.Completed,
            "paused" => AssistantState.Paused,
            "dnd" => AssistantState.Dnd,
            "music" => AssistantState.Music,
            _ => AssistantState.Idle
        };
    }

    public static string ToDisplayText(AssistantState state)
    {
        return state switch
        {
            AssistantState.Idle => "空闲",
            AssistantState.Ready => "就绪",
            AssistantState.Thinking => "思考中",
            AssistantState.Executing => "执行中",
            AssistantState.Attention => "需要你确认",
            AssistantState.Error => "错误",
            AssistantState.Completed => "已完成",
            AssistantState.Paused => "暂停",
            AssistantState.Dnd => "低打扰",
            AssistantState.Music => "音乐模式",
            _ => "空闲"
        };
    }

    public static string ToImagePath(AssistantState state)
    {
        return state switch
        {
            AssistantState.Thinking => "Assets/pixel/thinking.png",
            AssistantState.Error => "Assets/pixel/error.png",
            AssistantState.Music => "Assets/pixel/music.png",
            _ => "Assets/pixel/idle.png"
        };
    }

    public static Brush ToIndicatorBrush(AssistantState state)
    {
        return state switch
        {
            AssistantState.Idle => new SolidColorBrush(Color.FromRgb(140, 148, 158)),
            AssistantState.Ready => new SolidColorBrush(Color.FromRgb(102, 214, 182)),
            AssistantState.Thinking => new SolidColorBrush(Color.FromRgb(248, 199, 92)),
            AssistantState.Executing => new SolidColorBrush(Color.FromRgb(80, 195, 255)),
            AssistantState.Attention => new SolidColorBrush(Color.FromRgb(255, 181, 92)),
            AssistantState.Error => new SolidColorBrush(Color.FromRgb(255, 122, 122)),
            AssistantState.Completed => new SolidColorBrush(Color.FromRgb(138, 225, 140)),
            AssistantState.Paused => new SolidColorBrush(Color.FromRgb(170, 170, 190)),
            AssistantState.Dnd => new SolidColorBrush(Color.FromRgb(125, 125, 132)),
            AssistantState.Music => new SolidColorBrush(Color.FromRgb(198, 147, 250)),
            _ => new SolidColorBrush(Color.FromRgb(140, 148, 158))
        };
    }

    public static double ToOpacity(AssistantState state)
    {
        return state switch
        {
            AssistantState.Idle => 0.78,
            AssistantState.Dnd => 0.55,
            AssistantState.Music => 0.88,
            _ => 1.0
        };
    }
}
