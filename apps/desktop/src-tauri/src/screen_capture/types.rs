use serde::Serialize;
use std::time::{SystemTime, UNIX_EPOCH};

/// ScreenCapturePayload describes one desktop screenshot stored in the workspace
/// temp folder for later review.
#[derive(Clone, Serialize)]
pub struct ScreenCapturePayload {
    pub path: String,
    pub relative_path: String,
    pub width: u32,
    pub height: u32,
    pub captured_at: String,
}

impl ScreenCapturePayload {
    /// Creates a new screenshot payload with a monotonic timestamp string.
    pub fn new(path: String, relative_path: String, width: u32, height: u32) -> Self {
        let captured_at = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|duration| duration.as_millis().to_string())
            .unwrap_or_else(|_| "0".to_string());

        Self {
            path,
            relative_path,
            width,
            height,
            captured_at,
        }
    }
}
