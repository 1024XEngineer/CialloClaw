# Skills Directory

Place your skills here. Each skill should be in its own folder with a `skill.md` file.

## Format

```
skills/
├── my-skill/
│   ├── skill.md      # Skill definition with tools
│   └── scripts/       # Scripts used by the skill
│       └── run.sh
└── another-skill/
    ├── skill.md
    └── ...
```

## skill.md Format

```markdown
# skill-name

Description of what this skill does.

## Tools

### tool-name
**命令**: ./scripts/run.sh {{arg1}} {{arg2}}
**描述**: What this tool does

**参数**:
- arg1 (string): Description of arg1
- arg2 (string): Description of arg2
```
