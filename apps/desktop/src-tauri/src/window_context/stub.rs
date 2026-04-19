use super::types::ActiveWindowContextPayload;
use tauri::AppHandle;

/// Installs a no-op active-window listener on unsupported platforms.
pub fn install_window_context_listener(_app: &AppHandle) -> Result<(), String> {
    Ok(())
}

/// Returns no active window context on platforms that do not yet expose a
/// native desktop-window implementation.
pub fn read_active_window_context() -> Result<Option<ActiveWindowContextPayload>, String> {
    Ok(None)
}
