namespace CialloClaw.Shell.Models;

public sealed class PermissionBoundary
{
    public List<string> GreenActions { get; set; } = new();
    public List<string> YellowActions { get; set; } = new();
    public List<string> RedActions { get; set; } = new();
}

public sealed class AgentProfile
{
    public string Id { get; set; } = string.Empty;
    public string Name { get; set; } = string.Empty;
    public string Style { get; set; } = string.Empty;
    public string MemoryBoundary { get; set; } = string.Empty;
    public string RouteBoundary { get; set; } = string.Empty;
    public PermissionBoundary PermissionBoundary { get; set; } = new();
    public bool Default { get; set; }

    public string DisplayName
    {
        get
        {
            return Id switch
            {
                "life-agent" => "生活助手",
                "work-agent" => "工作助手",
                _ => Name
            };
        }
    }

    public override string ToString() => DisplayName;
}
