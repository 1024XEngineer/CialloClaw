use super::types::SelectionSnapshotPayload;
use tauri::AppHandle;

/// Returns no selection snapshot on platforms that do not yet provide a native
/// selection adapter.
pub fn read_selection_snapshot(
    _app: &AppHandle,
) -> Result<Option<SelectionSnapshotPayload>, String> {
    Ok(None)
}
