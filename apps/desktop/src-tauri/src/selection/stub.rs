use super::types::SelectionSnapshotPayload;
use tauri::AppHandle;

/// Installs no native selection activity hooks on unsupported platforms.
pub fn install_selection_activity_hook(_app: &AppHandle) -> Result<(), String> {
    Ok(())
}

/// Returns no selection snapshot on platforms that do not yet provide a native
/// selection adapter.
pub fn read_selection_snapshot(
    _app: &AppHandle,
) -> Result<Option<SelectionSnapshotPayload>, String> {
    Ok(None)
}
