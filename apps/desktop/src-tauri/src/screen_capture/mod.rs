mod types;

#[cfg(windows)]
mod windows;

#[cfg(not(windows))]
mod stub;

pub use types::ScreenCapturePayload;

/// Captures a desktop screenshot using the active platform implementation.
pub fn capture_screenshot() -> Result<ScreenCapturePayload, String> {
    #[cfg(windows)]
    {
        return windows::capture_screenshot();
    }

    #[cfg(not(windows))]
    {
        stub::capture_screenshot()
    }
}
