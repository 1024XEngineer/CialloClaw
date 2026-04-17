use super::types::{SelectionPageContextPayload, SelectionSnapshotPayload};
use once_cell::sync::Lazy;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Mutex;
use tauri::{AppHandle, Emitter, Manager};
use windows::core::PWSTR;
use windows::Win32::Foundation::{CloseHandle, HWND, LPARAM, LRESULT, RPC_E_CHANGED_MODE, WPARAM};
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
use windows::Win32::UI::Input::KeyboardAndMouse::{
    GetAsyncKeyState, VK_CONTROL, VK_DOWN, VK_END, VK_HOME, VK_LEFT, VK_RIGHT, VK_SHIFT, VK_UP,
};
use windows::Win32::UI::WindowsAndMessaging::{
    CallNextHookEx, GetAncestor, GetForegroundWindow, GetWindowTextLengthW, GetWindowTextW,
    SetWindowsHookExW, GA_ROOT, KBDLLHOOKSTRUCT, MSLLHOOKSTRUCT, WH_KEYBOARD_LL, WH_MOUSE_LL,
    WM_KEYUP, WM_LBUTTONUP, WM_SYSKEYUP,
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

static SELECTION_ACTIVITY_DIRTY: AtomicBool = AtomicBool::new(false);
static SHELL_BALL_SELECTION_APP_HANDLE: Lazy<Mutex<Option<AppHandle>>> =
    Lazy::new(|| Mutex::new(None));
static SHELL_BALL_SELECTION_MOUSE_HOOK: Lazy<Mutex<Option<isize>>> = Lazy::new(|| Mutex::new(None));
static SHELL_BALL_SELECTION_KEYBOARD_HOOK: Lazy<Mutex<Option<isize>>> =
    Lazy::new(|| Mutex::new(None));

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
    SELECTION_ACTIVITY_DIRTY.store(false, Ordering::SeqCst);
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

/// Installs Windows-only activity hooks that mark shell-ball selection sensing
/// as dirty after the user performs a likely text-selection action.
pub fn install_selection_activity_hook(app: &AppHandle) -> Result<(), String> {
    if let Ok(mut app_handle) = SHELL_BALL_SELECTION_APP_HANDLE.lock() {
        *app_handle = Some(app.clone());
    }

    let mut mouse_hook = SHELL_BALL_SELECTION_MOUSE_HOOK
        .lock()
        .map_err(|_| "selection mouse hook lock poisoned".to_string())?;
    if mouse_hook.is_none() {
        unsafe {
            *mouse_hook = Some(
                SetWindowsHookExW(WH_MOUSE_LL, Some(selection_mouse_observer), None, 0)
                    .map_err(|error| format!("failed to install selection mouse hook: {error}"))?
                    .0 as isize,
            );
        }
    }

    let mut keyboard_hook = SHELL_BALL_SELECTION_KEYBOARD_HOOK
        .lock()
        .map_err(|_| "selection keyboard hook lock poisoned".to_string())?;
    if keyboard_hook.is_none() {
        unsafe {
            *keyboard_hook = Some(
                SetWindowsHookExW(WH_KEYBOARD_LL, Some(selection_keyboard_observer), None, 0)
                    .map_err(|error| format!("failed to install selection keyboard hook: {error}"))?
                    .0 as isize,
            );
        }
    }

    Ok(())
}

fn mark_selection_activity(source: &str) {
    let Some(app) = SHELL_BALL_SELECTION_APP_HANDLE
        .lock()
        .ok()
        .and_then(|guard| guard.as_ref().cloned())
    else {
        return;
    };

    if SELECTION_ACTIVITY_DIRTY.swap(true, Ordering::SeqCst) {
        return;
    }

    let _ = app.emit_to(
        "shell-ball",
        "desktop-shell-ball:selection-activity",
        serde_json::json!({
            "source": source,
        }),
    );
}

unsafe extern "system" fn selection_mouse_observer(
    n_code: i32,
    w_param: WPARAM,
    l_param: LPARAM,
) -> LRESULT {
    if n_code < 0 {
        return CallNextHookEx(None, n_code, w_param, l_param);
    }

    if w_param.0 as u32 == WM_LBUTTONUP {
        let _point = (*(l_param.0 as *const MSLLHOOKSTRUCT)).pt;
        let foreground_window = GetForegroundWindow();
        let app_handle = SHELL_BALL_SELECTION_APP_HANDLE
            .lock()
            .ok()
            .and_then(|guard| guard.as_ref().cloned());

        if let Some(app) = app_handle {
            if foreground_window.0.is_null()
                || is_shell_ball_cluster_window(&app, foreground_window)
            {
                return CallNextHookEx(None, n_code, w_param, l_param);
            }
        }

        mark_selection_activity("pointer");
    }

    CallNextHookEx(None, n_code, w_param, l_param)
}

unsafe extern "system" fn selection_keyboard_observer(
    n_code: i32,
    w_param: WPARAM,
    l_param: LPARAM,
) -> LRESULT {
    if n_code < 0 {
        return CallNextHookEx(None, n_code, w_param, l_param);
    }

    if w_param.0 as u32 == WM_KEYUP || w_param.0 as u32 == WM_SYSKEYUP {
        let keyboard = &*(l_param.0 as *const KBDLLHOOKSTRUCT);
        let foreground_window = GetForegroundWindow();
        let app_handle = SHELL_BALL_SELECTION_APP_HANDLE
            .lock()
            .ok()
            .and_then(|guard| guard.as_ref().cloned());

        if let Some(app) = app_handle {
            if foreground_window.0.is_null()
                || is_shell_ball_cluster_window(&app, foreground_window)
            {
                return CallNextHookEx(None, n_code, w_param, l_param);
            }
        }

        let virtual_key = keyboard.vkCode as u16;
        let shift_down = (GetAsyncKeyState(VK_SHIFT.0 as i32) as u16 & 0x8000) != 0;
        let control_down = (GetAsyncKeyState(VK_CONTROL.0 as i32) as u16 & 0x8000) != 0;
        let is_navigation_key = matches!(
            virtual_key,
            key if key == VK_LEFT.0 || key == VK_RIGHT.0 || key == VK_UP.0 || key == VK_DOWN.0 || key == VK_HOME.0 || key == VK_END.0
        );

        if (shift_down && is_navigation_key) || (control_down && virtual_key == b'A' as u16) {
            mark_selection_activity("keyboard");
        }
    }

    CallNextHookEx(None, n_code, w_param, l_param)
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
