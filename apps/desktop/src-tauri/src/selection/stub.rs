use super::types::SelectionSnapshotPayload;
use tauri::AppHandle;

/// Installs no-op selection listeners on platforms that do not yet expose a
/// native text selection adapter.
pub fn install_selection_listener(_app: &AppHandle) -> Result<(), String> {
    Ok(())
}

/// Returns no selection snapshot on platforms that do not yet provide a native
/// selection adapter.
pub fn read_selection_snapshot(
    _app: &AppHandle,
) -> Result<Option<SelectionSnapshotPayload>, String> {
    Ok(None)
}
