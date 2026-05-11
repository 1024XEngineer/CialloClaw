# Security & Privacy

## Data Storage

All data is stored locally on your machine using SQLite with WAL mode. Your conversations, task history, files and configuration never leave your computer unless you explicitly authorise an external action.

## Network Access

CialloClaw only makes network requests to:
- The model provider API you configure
- GitHub for update checks (if enabled)

No telemetry or analytics data is sent. No user data is collected.

## Risk Controls

CialloClaw classifies actions into risk levels:

- **Informational**: Displaying search results or summaries
- **File operations**: Reading or writing to files you specify
- **Command execution**: Running shell commands
- **System changes**: Modifying system settings

High-risk actions require explicit authorisation before execution. The safety panel gives you a full audit trail of every action taken.

## Recovery Points

Before executing high-risk actions, CialloClaw creates recovery points that allow you to revert changes if something goes wrong.
