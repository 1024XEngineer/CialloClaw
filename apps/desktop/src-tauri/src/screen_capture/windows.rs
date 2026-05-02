use super::types::ScreenCapturePayload;
use image::{ImageBuffer, Rgba};
use std::fs;
use std::path::{Path, PathBuf};
use windows::Win32::Foundation::HWND;
use windows::Win32::Graphics::Gdi::{
    BitBlt, CreateCompatibleBitmap, CreateCompatibleDC, DeleteDC, DeleteObject, GetDC, GetDIBits,
    ReleaseDC, SelectObject, BITMAPINFO, BITMAPINFOHEADER, BI_RGB, DIB_RGB_COLORS, HGDIOBJ,
    SRCCOPY,
};
use windows::Win32::UI::WindowsAndMessaging::{GetSystemMetrics, SM_CXSCREEN, SM_CYSCREEN};

const SCREENSHOT_PREFIX: &str = "screenshot";

/// Captures the current primary screen into the runtime temp directory and returns the saved
/// screenshot metadata.
pub fn capture_screenshot(temp_dir: PathBuf) -> Result<ScreenCapturePayload, String> {
    let width = unsafe { GetSystemMetrics(SM_CXSCREEN) };
    let height = unsafe { GetSystemMetrics(SM_CYSCREEN) };

    if width <= 0 || height <= 0 {
        return Err("failed to resolve primary screen dimensions".to_string());
    }

    let screen_dc = unsafe { GetDC(Some(HWND(std::ptr::null_mut()))) };
    if screen_dc.0.is_null() {
        return Err("failed to acquire screen device context".to_string());
    }

    let memory_dc = unsafe { CreateCompatibleDC(Some(screen_dc)) };
    if memory_dc.0.is_null() {
        unsafe {
            let _ = ReleaseDC(Some(HWND(std::ptr::null_mut())), screen_dc);
        }
        return Err("failed to create memory device context".to_string());
    }

    let bitmap = unsafe { CreateCompatibleBitmap(screen_dc, width, height) };
    if bitmap.0.is_null() {
        unsafe {
            let _ = DeleteDC(memory_dc);
            let _ = ReleaseDC(Some(HWND(std::ptr::null_mut())), screen_dc);
        }
        return Err("failed to create compatible bitmap".to_string());
    }
    let previous_object = unsafe { SelectObject(memory_dc, HGDIOBJ(bitmap.0)) };

    let capture_result = (|| {
        unsafe {
            BitBlt(
                memory_dc,
                0,
                0,
                width,
                height,
                Some(screen_dc),
                0,
                0,
                SRCCOPY,
            )
            .map_err(|error| format!("failed to copy desktop pixels: {error}"))?;
        }

        let mut bitmap_info = BITMAPINFO {
            bmiHeader: BITMAPINFOHEADER {
                biSize: std::mem::size_of::<BITMAPINFOHEADER>() as u32,
                biWidth: width,
                biHeight: -height,
                biPlanes: 1,
                biBitCount: 32,
                biCompression: BI_RGB.0,
                ..Default::default()
            },
            ..Default::default()
        };

        let mut pixels = vec![0u8; (width as usize) * (height as usize) * 4];
        unsafe {
            let copied_scan_lines = GetDIBits(
                memory_dc,
                bitmap,
                0,
                height as u32,
                Some(pixels.as_mut_ptr().cast()),
                &mut bitmap_info,
                DIB_RGB_COLORS,
            );

            if copied_scan_lines == 0 {
                return Err("failed to read screenshot bitmap bits".to_string());
            }
        }

        for chunk in pixels.chunks_exact_mut(4) {
            chunk.swap(0, 2);
        }

        let image = ImageBuffer::<Rgba<u8>, _>::from_raw(width as u32, height as u32, pixels)
            .ok_or_else(|| "failed to build screenshot image buffer".to_string())?;
        let (absolute_path, relative_path) = next_screenshot_paths(&temp_dir)?;
        image
            .save(&absolute_path)
            .map_err(|error| format!("failed to save screenshot image: {error}"))?;

        Ok(ScreenCapturePayload::new(
            absolute_path.display().to_string(),
            relative_path,
            width as u32,
            height as u32,
        ))
    })();

    unsafe {
        let _ = SelectObject(memory_dc, previous_object);
        let _ = DeleteObject(bitmap.into());
        let _ = DeleteDC(memory_dc);
        let _ = ReleaseDC(Some(HWND(std::ptr::null_mut())), screen_dc);
    }

    capture_result
}

fn next_screenshot_paths(temp_dir: &Path) -> Result<(PathBuf, String), String> {
    fs::create_dir_all(&temp_dir)
        .map_err(|error| format!("failed to create screenshot temp dir: {error}"))?;

    let timestamp = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|duration| duration.as_millis())
        .unwrap_or(0);
    let file_name = format!("{SCREENSHOT_PREFIX}_{timestamp}.png");
    let absolute_path = temp_dir.join(&file_name);
    let relative_path = Path::new("temp").join(&file_name);

    Ok((absolute_path, relative_path.to_string_lossy().to_string()))
}
