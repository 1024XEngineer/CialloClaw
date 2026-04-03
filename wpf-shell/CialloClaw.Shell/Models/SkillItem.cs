namespace CialloClaw.Shell.Models;

public sealed class SkillItem
{
    public string Id { get; set; } = string.Empty;
    public string Name { get; set; } = string.Empty;
    public string Source { get; set; } = string.Empty;
    public string Version { get; set; } = string.Empty;
    public bool Installed { get; set; }
    public string Description { get; set; } = string.Empty;
    public string Scope { get; set; } = string.Empty;

    public string DisplaySource => Source switch
    {
        "builtin" => "内置",
        "github" => "GitHub",
        _ => Source
    };

    public string DisplayScope => Scope switch
    {
        "shared" => "共享",
        "work-agent" => "工作助手",
        "life-agent" => "生活助手",
        "optional" => "可选",
        _ => Scope
    };
}
