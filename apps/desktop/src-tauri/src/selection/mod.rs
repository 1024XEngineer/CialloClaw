mod types;

#[cfg(windows)]
mod windows;

#[cfg(not(windows))]
mod stub;

pub use types::SelectionSnapshotPayload;

use tauri::AppHandle;

/// Installs the native activity hook used to mark selection sensing as dirty on
/// the active platform.
pub fn install_selection_activity_hook(app: &AppHandle) -> Result<(), String> {
    #[cfg(windows)]
    {
        return windows::install_selection_activity_hook(app);
    }

    #[cfg(not(windows))]
    {
        stub::install_selection_activity_hook(app)
    }
}

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
