mod types;

#[cfg(windows)]
mod windows;

#[cfg(not(windows))]
mod stub;

pub use types::MouseActivitySnapshotPayload;

/// Installs the native mouse activity listener for the active platform.
pub fn install_mouse_activity_listener() -> Result<(), String> {
    #[cfg(windows)]
    {
        return windows::install_mouse_activity_listener();
    }

    #[cfg(not(windows))]
    {
        stub::install_mouse_activity_listener()
    }
}

/// Reads the latest native mouse activity snapshot for the active platform.
pub fn read_mouse_activity_snapshot() -> Option<MouseActivitySnapshotPayload> {
    #[cfg(windows)]
    {
        return windows::read_mouse_activity_snapshot();
    }

    #[cfg(not(windows))]
    {
        stub::read_mouse_activity_snapshot()
    }
}

/// Returns a small history of recent native mouse positions so nearby desktop
/// surfaces can resolve hover context without depending on a focused element.
pub fn read_recent_mouse_points() -> Vec<(i32, i32)> {
    #[cfg(windows)]
    {
        return windows::read_recent_mouse_points();
    }

    #[cfg(not(windows))]
    {
        stub::read_recent_mouse_points()
    }
}
