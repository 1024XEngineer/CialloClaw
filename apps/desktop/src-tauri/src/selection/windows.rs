use super::types::{SelectionPageContextPayload, SelectionSnapshotPayload};
use tauri::{AppHandle, Manager};
use windows::core::PWSTR;
use windows::Win32::Foundation::{CloseHandle, HWND, RPC_E_CHANGED_MODE};
use windows::Win32::System::Com::{
    CoCreateInstance, CoInitializeEx, CoUninitialize, CLSCTX_INPROC_SERVER,
    COINIT_APARTMENTTHREADED,
};
use windows::Win32::System::Threading::{
    OpenProcess, QueryFullProcessImageNameW, PROCESS_NAME_WIN32, PROCESS_QUERY_LIMITED_INFORMATION,
};
use windows::Win32::UI::Accessibility::{
    CUIAutomation, IUIAutomation, IUIAutomationElement, IUIAutomationTextPattern, UIA_TextPatternId,
};
use windows::Win32::UI::WindowsAndMessaging::{
    GetAncestor, GetForegroundWindow, GetWindowTextLengthW, GetWindowTextW, GA_ROOT,
};

const WINDOWS_UIA_SELECTION_SOURCE: &str = "windows_uia";
const WINDOWS_UIA_SELECTION_URL: &str = "native://windows-uia-selection";
const SHELL_BALL_WINDOW_LABELS: [&str; 4] = [
    "shell-ball",
    "shell-ball-bubble",
    "shell-ball-input",
    "shell-ball-voice",
];
const SHELL_BALL_PINNED_WINDOW_PREFIX: &str = "shell-ball-bubble-pinned-";

struct ComGuard {
    should_uninitialize: bool,
}

impl ComGuard {
    fn initialize() -> Result<Self, String> {
        let result = unsafe { CoInitializeEx(None, COINIT_APARTMENTTHREADED) };

        if result.is_ok() {
            Ok(Self {
                should_uninitialize: true,
            })
        } else if result == RPC_E_CHANGED_MODE {
            Ok(Self {
                should_uninitialize: false,
            })
        } else {
            Err(format!(
                "failed to initialize COM for UIA selection: {}",
                result.message()
            ))
        }
    }
}

impl Drop for ComGuard {
    fn drop(&mut self) {
        if self.should_uninitialize {
            unsafe {
                CoUninitialize();
            }
        }
    }
}

/// Reads the current Windows UI Automation text selection and normalizes it into
/// a shell-ball selection snapshot.
pub fn read_selection_snapshot(
    app: &AppHandle,
) -> Result<Option<SelectionSnapshotPayload>, String> {
    let _com_guard = ComGuard::initialize()?;
    let foreground_window = unsafe { GetForegroundWindow() };

    if foreground_window.0.is_null() || is_shell_ball_cluster_window(app, foreground_window) {
        return Ok(None);
    }

    let automation: IUIAutomation = unsafe {
        CoCreateInstance(&CUIAutomation, None, CLSCTX_INPROC_SERVER)
            .map_err(|error| format!("failed to create UI Automation instance: {error}"))?
    };

    let element = read_selection_target_element(&automation, foreground_window)?;
    let text = read_text_selection(&element)?;
    let normalized_text = text.trim().to_string();

    if normalized_text.is_empty() {
        return Ok(None);
    }

    Ok(Some(SelectionSnapshotPayload::new(
        normalized_text,
        SelectionPageContextPayload {
            title: get_window_title(foreground_window),
            url: WINDOWS_UIA_SELECTION_URL.to_string(),
            app_name: get_window_app_name(foreground_window)
                .unwrap_or_else(|| WINDOWS_UIA_SELECTION_SOURCE.to_string()),
        },
        WINDOWS_UIA_SELECTION_SOURCE,
    )))
}

fn read_selection_target_element(
    automation: &IUIAutomation,
    foreground_window: HWND,
) -> Result<IUIAutomationElement, String> {
    unsafe {
        automation
            .GetFocusedElement()
            .or_else(|_| automation.ElementFromHandle(foreground_window))
            .map_err(|error| format!("failed to resolve UIA selection target: {error}"))
    }
}

fn read_text_selection(element: &IUIAutomationElement) -> Result<String, String> {
    let text_pattern: IUIAutomationTextPattern =
        unsafe { element.GetCurrentPatternAs(UIA_TextPatternId) }
            .map_err(|_| "selection target does not expose TextPattern".to_string())?;

    let ranges = unsafe { text_pattern.GetSelection() }
        .map_err(|error| format!("failed to read UIA text selection ranges: {error}"))?;
    let range_count = unsafe { ranges.Length() }
        .map_err(|error| format!("failed to inspect UIA selection length: {error}"))?;

    if range_count <= 0 {
        return Ok(String::new());
    }

    let mut parts = Vec::new();

    for index in 0..range_count {
        let text_range = unsafe { ranges.GetElement(index) }
            .map_err(|error| format!("failed to inspect UIA text selection range: {error}"))?;
        let text = unsafe { text_range.GetText(-1) }
            .map_err(|error| format!("failed to read UIA selection text: {error}"))?
            .to_string();

        if !text.trim().is_empty() {
            parts.push(text);
        }
    }

    Ok(parts.join("\n"))
}

fn is_shell_ball_cluster_window(app: &AppHandle, hwnd: HWND) -> bool {
    let root_window = get_root_window(hwnd);

    for label in SHELL_BALL_WINDOW_LABELS {
        let Some(window) = app.get_webview_window(label) else {
            continue;
        };

        let Ok(window_hwnd) = window.hwnd() else {
            continue;
        };

        if window_hwnd == root_window {
            return true;
        }
    }

    for window in app.webview_windows().values() {
        if !window.label().starts_with(SHELL_BALL_PINNED_WINDOW_PREFIX) {
            continue;
        }

        let Ok(window_hwnd) = window.hwnd() else {
            continue;
        };

        if window_hwnd == root_window {
            return true;
        }
    }

    false
}

fn get_root_window(hwnd: HWND) -> HWND {
    unsafe {
        let root = GetAncestor(hwnd, GA_ROOT);
        if root.0.is_null() {
            hwnd
        } else {
            root
        }
    }
}

fn get_window_title(hwnd: HWND) -> String {
    let text_length = unsafe { GetWindowTextLengthW(hwnd) };
    if text_length <= 0 {
        return WINDOWS_UIA_SELECTION_SOURCE.to_string();
    }

    let mut buffer = vec![0u16; text_length as usize + 1];
    let written = unsafe { GetWindowTextW(hwnd, &mut buffer) };
    if written <= 0 {
        return WINDOWS_UIA_SELECTION_SOURCE.to_string();
    }

    String::from_utf16_lossy(&buffer[..written as usize])
}

fn get_window_app_name(hwnd: HWND) -> Option<String> {
    let process_id = unsafe {
        let mut process_id = 0u32;
        windows::Win32::UI::WindowsAndMessaging::GetWindowThreadProcessId(
            hwnd,
            Some(&mut process_id),
        );
        process_id
    };

    if process_id == 0 {
        return None;
    }

    let process =
        unsafe { OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, process_id).ok()? };

    let mut buffer = vec![0u16; 512];
    let mut size = buffer.len() as u32;
    let result = unsafe {
        QueryFullProcessImageNameW(
            process,
            PROCESS_NAME_WIN32,
            PWSTR(buffer.as_mut_ptr()),
            &mut size,
        )
    };

    unsafe {
        let _ = CloseHandle(process);
    }

    if result.is_err() || size == 0 {
        return None;
    }

    let full_path = String::from_utf16_lossy(&buffer[..size as usize]);
    std::path::Path::new(&full_path)
        .file_stem()
        .and_then(|stem| stem.to_str())
        .map(ToString::to_string)
}
