mod types;

#[cfg(windows)]
mod windows;

#[cfg(not(windows))]
mod stub;

pub use types::ActiveWindowContextPayload;

use tauri::AppHandle;

/// Installs the native active-window listener for the active platform.
pub fn install_window_context_listener(app: &AppHandle) -> Result<(), String> {
    #[cfg(windows)]
    {
        return windows::install_window_context_listener(app);
    }

    #[cfg(not(windows))]
    {
        stub::install_window_context_listener(app)
    }
}

/// Reads the current active window context using the active platform adapter.
pub fn read_active_window_context() -> Result<Option<ActiveWindowContextPayload>, String> {
    #[cfg(windows)]
    {
        return windows::read_active_window_context();
    }

    #[cfg(not(windows))]
    {
        stub::read_active_window_context()
    }
}
