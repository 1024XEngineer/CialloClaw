use super::types::ScreenCapturePayload;
use std::path::PathBuf;

/// Returns a clear error on platforms where desktop screenshot capture is not
/// implemented yet.
pub fn capture_screenshot(_temp_dir: PathBuf) -> Result<ScreenCapturePayload, String> {
    Err("desktop screenshot capture is not supported on this platform".to_string())
}
