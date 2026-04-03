namespace CialloClaw.Shell.Models;

public sealed class QuickTask
{
    public string Id { get; set; } = string.Empty;
    public string Label { get; set; } = string.Empty;
    public string ShortLabel { get; set; } = string.Empty;
    public string Description { get; set; } = string.Empty;
    public string Verb { get; set; } = string.Empty;
    public int Level { get; set; }

    public string DisplayLabel => LocalizeLabel(Id, Label);
    public string DisplayShortLabel => LocalizeShortLabel(Id, ShortLabel);
    public string DisplayDescription => LocalizeDescription(Id, Description);

    private static string LocalizeLabel(string id, string fallback)
    {
        return id switch
        {
            "summary" => "总结当前内容",
            "translate" => "翻译选中内容",
            "explain" => "解释这段内容",
            "todo" => "记录为待办",
            "next" => "建议下一步",
            "more" => "更多动作",
            "inspect" => "巡检任务便签",
            "daily" => "生成每日摘要",
            "error" => "分析错误日志",
            "draft" => "生成消息草稿",
            _ => fallback
        };
    }

    private static string LocalizeShortLabel(string id, string fallback)
    {
        return id switch
        {
            "summary" => "总结",
            "translate" => "翻译",
            "explain" => "解释",
            "todo" => "待办",
            "next" => "下一步",
            "more" => "更多",
            "inspect" => "巡检",
            "daily" => "日报",
            "error" => "日志",
            "draft" => "草稿",
            _ => fallback
        };
    }

    private static string LocalizeDescription(string id, string fallback)
    {
        return id switch
        {
            "summary" => "提炼当前选中文本的关键点",
            "translate" => "将选中内容翻译为目标语言",
            "explain" => "用简洁方式解释当前内容",
            "todo" => "把当前事项加入任务巡检列表",
            "next" => "给出当前场景的下一步建议",
            "more" => "打开更多可执行动作",
            "inspect" => "检查即将到期和阻塞任务",
            "daily" => "汇总今日任务与状态",
            "error" => "读取错误输出并生成修复建议",
            "draft" => "根据任务生成消息模板",
            _ => fallback
        };
    }
}

public sealed class RadialTaskItem
{
    public QuickTask Task { get; init; } = new();
    public double X { get; init; }
    public double Y { get; init; }
}
