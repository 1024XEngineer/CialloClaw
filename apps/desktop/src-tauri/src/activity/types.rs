use serde::Serialize;
use std::time::{SystemTime, UNIX_EPOCH};

/// MouseActivitySnapshotPayload stores the latest mouse activity timestamp that
/// desktop surfaces can query without depending on Windows-specific hook state.
#[derive(Clone, Serialize)]
pub struct MouseActivitySnapshotPayload {
    pub updated_at: String,
    pub cursor_x: Option<i32>,
    pub cursor_y: Option<i32>,
}

impl MouseActivitySnapshotPayload {
    /// Creates a new snapshot using a millisecond unix timestamp string so the
    /// frontend can compare updates without extra parsing rules.
    pub fn now(cursor_x: Option<i32>, cursor_y: Option<i32>) -> Self {
        let updated_at = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|duration| duration.as_millis().to_string())
            .unwrap_or_else(|_| "0".to_string());

        Self {
            updated_at,
            cursor_x,
            cursor_y,
        }
    }
}
