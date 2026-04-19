use super::types::MouseActivitySnapshotPayload;

/// Installs no-op mouse activity listeners on platforms that do not yet expose
/// a native global mouse hook implementation.
pub fn install_mouse_activity_listener() -> Result<(), String> {
    Ok(())
}

/// Returns no mouse activity snapshot on platforms that do not yet implement
/// native activity tracking.
pub fn read_mouse_activity_snapshot() -> Option<MouseActivitySnapshotPayload> {
    None
}
