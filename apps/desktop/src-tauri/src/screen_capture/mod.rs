mod types;

#[cfg(windows)]
mod windows;

#[cfg(not(windows))]
mod stub;

use std::path::PathBuf;
pub use types::ScreenCapturePayload;

/// Captures a desktop screenshot using the active platform implementation.
pub fn capture_screenshot(temp_dir: PathBuf) -> Result<ScreenCapturePayload, String> {
    #[cfg(windows)]
    {
        return windows::capture_screenshot(temp_dir);
    }

    #[cfg(not(windows))]
    {
        stub::capture_screenshot(temp_dir)
    }
}
