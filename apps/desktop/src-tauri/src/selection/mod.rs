mod types;

#[cfg(windows)]
mod windows;

#[cfg(not(windows))]
mod stub;

pub use types::SelectionSnapshotPayload;

use tauri::AppHandle;

/// Reads the current native text selection using the active platform adapter.
pub fn read_selection_snapshot(
    app: &AppHandle,
) -> Result<Option<SelectionSnapshotPayload>, String> {
    #[cfg(windows)]
    {
        return windows::read_selection_snapshot(app);
    }

    #[cfg(not(windows))]
    {
        stub::read_selection_snapshot(app)
    }
}
