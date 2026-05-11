use super::types::ActiveWindowContextPayload;
use crate::internal_windows::{INTERNAL_PINNED_WINDOW_PREFIX, INTERNAL_WINDOW_LABELS};
use once_cell::sync::Lazy;
use std::path::Path;
use std::sync::Mutex;
use std::thread;
use std::time::{Duration, Instant};
use tauri::{AppHandle, Manager};
use windows::core::{BSTR, PWSTR};
use windows::Win32::Foundation::{CloseHandle, HANDLE, HWND};
use windows::Win32::System::Com::{
    CoCreateInstance, CoInitializeEx, CoUninitialize, CLSCTX_INPROC_SERVER,
    COINIT_APARTMENTTHREADED,
};
use windows::Win32::System::ProcessStatus::GetModuleFileNameExW;
use windows::Win32::System::Threading::{
    OpenProcess, QueryFullProcessImageNameW, PROCESS_NAME_WIN32, PROCESS_QUERY_INFORMATION,
    PROCESS_QUERY_LIMITED_INFORMATION, PROCESS_VM_READ,
};
use windows::Win32::System::Variant::VARIANT;
use windows::Win32::UI::Accessibility::{
    CUIAutomation, IUIAutomation, IUIAutomationCondition, IUIAutomationElement,
    IUIAutomationElementArray, IUIAutomationValuePattern, SetWinEventHook, TreeScope_Subtree,
    UIA_ControlTypePropertyId, UIA_EditControlTypeId, UIA_ValuePatternId, HWINEVENTHOOK,
};
use windows::Win32::UI::WindowsAndMessaging::{
    GetAncestor, GetForegroundWindow, GetWindowTextLengthW, GetWindowTextW,
    GetWindowThreadProcessId, EVENT_SYSTEM_FOREGROUND, GA_ROOT, WINEVENT_OUTOFCONTEXT,
};

const BROWSER_KIND_CHROME: &str = "chrome";
const BROWSER_KIND_EDGE: &str = "edge";
const BROWSER_KIND_OTHER_BROWSER: &str = "other_browser";
const BROWSER_KIND_NON_BROWSER: &str = "non_browser";
const WINDOW_CONTEXT_URL_DEBOUNCE_MS: u64 = 320;
const INTERNAL_WINDOW_CONTEXT_REUSE_MAX_AGE_MS: u64 = 10_000;

static WINDOW_CONTEXT_APP_HANDLE: Lazy<Mutex<Option<AppHandle>>> = Lazy::new(|| Mutex::new(None));
static WINDOW_CONTEXT_FOREGROUND_HOOK: Lazy<Mutex<Option<isize>>> = Lazy::new(|| Mutex::new(None));
static LAST_EXTERNAL_WINDOW_CONTEXT: Lazy<Mutex<Option<CachedWindowContext>>> =
    Lazy::new(|| Mutex::new(None));
static WINDOW_CONTEXT_URL_REFRESH_STATE: Lazy<Mutex<UrlRefreshState>> =
    Lazy::new(|| Mutex::new(UrlRefreshState::default()));
static WINDOW_CONTEXT_ACTIVITY_STATE: Lazy<Mutex<WindowContextActivityState>> =
    Lazy::new(|| Mutex::new(WindowContextActivityState::default()));

#[derive(Clone)]
struct CachedWindowContext {
    hwnd: isize,
    context: ActiveWindowContextPayload,
    observed_at: Instant,
}

#[derive(Default)]
struct UrlRefreshState {
    in_flight_fingerprint: Option<String>,
    last_completed_fingerprint: Option<String>,
    last_completed_at: Option<Instant>,
}

#[derive(Default)]
struct WindowContextActivityState {
    window_switch_count: u32,
    page_switch_count: u32,
    last_window_fingerprint: Option<String>,
    last_page_fingerprint: Option<String>,
}

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
        } else {
            Ok(Self {
                should_uninitialize: false,
            })
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

/// Reads the current active desktop window context, resolving browser URL when
/// the active process exposes one.
pub fn read_active_window_context() -> Result<Option<ActiveWindowContextPayload>, String> {
    let hwnd = unsafe { GetForegroundWindow() };
    if hwnd.0.is_null() {
        return Ok(read_cached_window_context().map(with_window_context_activity_counts));
    }

    if is_internal_app_window(hwnd) {
        return Ok(read_cached_window_context_for_internal_window());
    }

    let context = read_window_context_for_hwnd(hwnd);
    record_window_switch(&context);
    record_page_switch(&context);
    cache_window_context(hwnd, &context);
    schedule_window_context_url_refresh(hwnd, &context);
    Ok(Some(with_window_context_activity_counts(context)))
}

/// Installs the Windows foreground-window listener used to keep a cached copy
/// of the last external active window context.
pub fn install_window_context_listener(app: &AppHandle) -> Result<(), String> {
    if let Ok(mut app_handle) = WINDOW_CONTEXT_APP_HANDLE.lock() {
        *app_handle = Some(app.clone());
    }

    let mut hook = WINDOW_CONTEXT_FOREGROUND_HOOK
        .lock()
        .map_err(|_| "window context foreground hook lock poisoned".to_string())?;

    if hook.is_some() {
        return Ok(());
    }

    unsafe {
        let installed_hook = SetWinEventHook(
            EVENT_SYSTEM_FOREGROUND,
            EVENT_SYSTEM_FOREGROUND,
            None,
            Some(window_context_foreground_hook),
            0,
            0,
            WINEVENT_OUTOFCONTEXT,
        );

        if installed_hook.0.is_null() {
            return Err("failed to install window context foreground hook".to_string());
        }

        *hook = Some(installed_hook.0 as isize);
    }

    if let Some((hwnd, current_context)) = read_current_external_window_context() {
        record_window_switch(&current_context);
        cache_window_context(hwnd, &current_context);
        schedule_window_context_url_refresh(hwnd, &current_context);
    }

    Ok(())
}

fn read_current_external_window_context() -> Option<(HWND, ActiveWindowContextPayload)> {
    let hwnd = unsafe { GetForegroundWindow() };
    if hwnd.0.is_null() || is_internal_app_window(hwnd) {
        return None;
    }

    read_lightweight_window_context_for_hwnd(hwnd)
        .ok()
        .map(|context| (hwnd, context))
}

fn read_lightweight_window_context_for_hwnd(
    hwnd: HWND,
) -> Result<ActiveWindowContextPayload, String> {
    let process_id = get_process_id(hwnd);
    let process_path = process_id.and_then(get_process_path);
    let app_name = process_path
        .as_deref()
        .and_then(extract_process_stem)
        .unwrap_or_else(|| "unknown".to_string());
    let browser_kind = classify_browser_kind(&app_name);
    let title = get_window_title(hwnd);

    Ok(ActiveWindowContextPayload {
        app_name,
        process_path,
        process_id,
        title,
        url: None,
        browser_kind: browser_kind.to_string(),
        window_switch_count: None,
        page_switch_count: None,
    })
}

fn read_window_context_for_hwnd(hwnd: HWND) -> ActiveWindowContextPayload {
    let mut context =
        read_lightweight_window_context_for_hwnd(hwnd).unwrap_or(ActiveWindowContextPayload {
            app_name: "unknown".to_string(),
            process_path: None,
            process_id: None,
            title: None,
            url: None,
            browser_kind: BROWSER_KIND_NON_BROWSER.to_string(),
            window_switch_count: None,
            page_switch_count: None,
        });

    context.url = read_url_for_window_context(hwnd, &context);
    context
}

fn with_window_context_activity_counts(
    mut context: ActiveWindowContextPayload,
) -> ActiveWindowContextPayload {
    if let Ok(activity_state) = WINDOW_CONTEXT_ACTIVITY_STATE.lock() {
        context.window_switch_count = Some(activity_state.window_switch_count);
        context.page_switch_count = Some(activity_state.page_switch_count);
    }

    context
}

fn record_window_switch(context: &ActiveWindowContextPayload) {
    let fingerprint = format!(
        "{}|{}|{}",
        context.app_name,
        context.title.clone().unwrap_or_default(),
        context.process_path.clone().unwrap_or_default()
    );

    if let Ok(mut activity_state) = WINDOW_CONTEXT_ACTIVITY_STATE.lock() {
        if activity_state
            .last_window_fingerprint
            .as_deref()
            .is_some_and(|current| current != fingerprint.as_str())
        {
            activity_state.window_switch_count =
                activity_state.window_switch_count.saturating_add(1);
        }

        activity_state.last_window_fingerprint = Some(fingerprint);
    }
}

fn record_page_switch(context: &ActiveWindowContextPayload) {
    record_page_switch_internal(context, false);
}

fn record_page_switch_after_url_refresh(context: &ActiveWindowContextPayload) {
    record_page_switch_internal(context, true);
}

fn record_page_switch_internal(
    context: &ActiveWindowContextPayload,
    prefer_lightweight_match: bool,
) {
    let fingerprint = create_page_switch_fingerprint(context);
    let lightweight_fingerprint =
        prefer_lightweight_match.then(|| create_page_switch_lightweight_fingerprint(context));

    if let Ok(mut activity_state) = WINDOW_CONTEXT_ACTIVITY_STATE.lock() {
        if activity_state
            .last_page_fingerprint
            .as_deref()
            .is_some_and(|current| {
                current != fingerprint.as_str()
                    && lightweight_fingerprint
                        .as_deref()
                        .map_or(true, |lightweight| current != lightweight)
            })
        {
            activity_state.page_switch_count = activity_state.page_switch_count.saturating_add(1);
        }

        activity_state.last_page_fingerprint = Some(fingerprint);
    }
}

fn create_page_switch_fingerprint(context: &ActiveWindowContextPayload) -> String {
    format!(
        "{}|{}|{}",
        context.app_name,
        context.title.clone().unwrap_or_default(),
        context.url.clone().unwrap_or_default()
    )
}

fn create_page_switch_lightweight_fingerprint(context: &ActiveWindowContextPayload) -> String {
    format!(
        "{}|{}|",
        context.app_name,
        context.title.clone().unwrap_or_default()
    )
}

fn cache_window_context(hwnd: HWND, context: &ActiveWindowContextPayload) {
    if let Ok(mut cached_context) = LAST_EXTERNAL_WINDOW_CONTEXT.lock() {
        *cached_context = Some(CachedWindowContext {
            hwnd: hwnd.0 as isize,
            context: context.clone(),
            observed_at: Instant::now(),
        });
    }
}

// Internal desktop windows should only reuse the last external foreground
// snapshot for a short time. Otherwise a dashboard or shell-ball submit can
// incorrectly report a long-stale browser tab as the current webpage.
fn read_fresh_cached_window_context() -> Option<CachedWindowContext> {
    let mut cached_context = LAST_EXTERNAL_WINDOW_CONTEXT.lock().ok()?;
    let cached = cached_context.clone()?;

    if cached.observed_at.elapsed()
        > Duration::from_millis(INTERNAL_WINDOW_CONTEXT_REUSE_MAX_AGE_MS)
    {
        *cached_context = None;
        return None;
    }

    Some(cached)
}

fn read_cached_window_context() -> Option<ActiveWindowContextPayload> {
    read_fresh_cached_window_context()
        .map(|cached| with_window_context_activity_counts(cached.context))
}

fn read_cached_window_context_for_internal_window() -> Option<ActiveWindowContextPayload> {
    let cached = read_fresh_cached_window_context()?;
    let cached_context = with_window_context_activity_counts(cached.context.clone());

    // Internal desktop activations can happen after the last external browser
    // page kept navigating without another foreground-window event. Re-read the
    // cached HWND on demand so task input sees the freshest page URL, but keep
    // the cached snapshot as a fallback when the refresh path cannot resolve.
    if !should_refresh_cached_shell_ball_window_context(&cached.context) {
        return Some(cached_context);
    }

    read_cached_window_context_with_url().or(Some(cached_context))
}

fn read_cached_window_context_with_url() -> Option<ActiveWindowContextPayload> {
    let cached = read_fresh_cached_window_context()?;

    let hwnd = HWND(cached.hwnd as *mut core::ffi::c_void);
    if hwnd.0.is_null() {
        return Some(with_window_context_activity_counts(cached.context));
    }

    let context = read_window_context_for_hwnd(hwnd);
    record_page_switch_after_url_refresh(&context);
    cache_window_context(hwnd, &context);
    Some(with_window_context_activity_counts(context))
}

// Desktop-owned windows should keep using the latest external foreground
// context so dashboard and shell-ball submissions can still carry the browser
// page URL that was active right before the desktop surface opened.
fn is_internal_app_window(hwnd: HWND) -> bool {
    let Some(app) = WINDOW_CONTEXT_APP_HANDLE
        .lock()
        .ok()
        .and_then(|guard| guard.as_ref().cloned())
    else {
        return false;
    };

    let root_window = get_root_window(hwnd);

    for label in INTERNAL_WINDOW_LABELS {
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
        if !window.label().starts_with(INTERNAL_PINNED_WINDOW_PREFIX) {
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

unsafe extern "system" fn window_context_foreground_hook(
    _hook: HWINEVENTHOOK,
    _event: u32,
    hwnd: HWND,
    _id_object: i32,
    _id_child: i32,
    _thread_id: u32,
    _event_time: u32,
) {
    if hwnd.0.is_null() || is_internal_app_window(hwnd) {
        return;
    }

    if let Ok(context) = read_lightweight_window_context_for_hwnd(hwnd) {
        record_window_switch(&context);
        cache_window_context(hwnd, &context);
        schedule_window_context_url_refresh(hwnd, &context);
    }
}

fn schedule_window_context_url_refresh(hwnd: HWND, context: &ActiveWindowContextPayload) {
    if !should_refresh_window_context_url(context) {
        return;
    }

    let context = context.clone();
    let hwnd_handle = hwnd.0 as isize;
    let fingerprint = create_window_context_fingerprint(&context);
    let should_schedule = {
        let mut state = match WINDOW_CONTEXT_URL_REFRESH_STATE.lock() {
            Ok(guard) => guard,
            Err(_) => return,
        };

        if state.in_flight_fingerprint.as_deref() == Some(fingerprint.as_str()) {
            false
        } else if state.last_completed_fingerprint.as_deref() == Some(fingerprint.as_str())
            && state.last_completed_at.is_some_and(|instant| {
                instant.elapsed() < Duration::from_millis(WINDOW_CONTEXT_URL_DEBOUNCE_MS)
            })
        {
            false
        } else {
            state.in_flight_fingerprint = Some(fingerprint.clone());
            true
        }
    };

    if !should_schedule {
        return;
    }

    thread::spawn(move || {
        thread::sleep(Duration::from_millis(WINDOW_CONTEXT_URL_DEBOUNCE_MS));

        let hwnd = HWND(hwnd_handle as *mut core::ffi::c_void);
        let url = read_url_for_window_context(hwnd, &context);
        let mut next_context = context.clone();
        next_context.url = url;
        record_page_switch_after_url_refresh(&next_context);
        cache_window_context(hwnd, &next_context);

        if let Ok(mut state) = WINDOW_CONTEXT_URL_REFRESH_STATE.lock() {
            let completed_fingerprint = create_window_context_fingerprint(&next_context);
            state.in_flight_fingerprint = None;
            state.last_completed_fingerprint = Some(completed_fingerprint);
            state.last_completed_at = Some(Instant::now());
        }
    });
}

fn should_refresh_window_context_url(context: &ActiveWindowContextPayload) -> bool {
    matches!(
        context.browser_kind.as_str(),
        BROWSER_KIND_CHROME | BROWSER_KIND_EDGE | BROWSER_KIND_OTHER_BROWSER
    )
}

fn should_refresh_cached_shell_ball_window_context(context: &ActiveWindowContextPayload) -> bool {
    should_refresh_window_context_url(context)
}

fn create_window_context_fingerprint(context: &ActiveWindowContextPayload) -> String {
    format!(
        "{}|{}|{}",
        context.app_name,
        context.title.clone().unwrap_or_default(),
        context.process_path.clone().unwrap_or_default()
    )
}

fn read_url_for_window_context(hwnd: HWND, context: &ActiveWindowContextPayload) -> Option<String> {
    match context.browser_kind.as_str() {
        BROWSER_KIND_CHROME | BROWSER_KIND_EDGE | BROWSER_KIND_OTHER_BROWSER => {
            read_browser_url_via_uia(hwnd)
        }
        _ => None,
    }
}

fn classify_browser_kind(app_name: &str) -> &'static str {
    match app_name.to_ascii_lowercase().as_str() {
        "chrome" => BROWSER_KIND_CHROME,
        "msedge" => BROWSER_KIND_EDGE,
        "firefox" | "opera" | "brave" | "vivaldi" => BROWSER_KIND_OTHER_BROWSER,
        _ => BROWSER_KIND_NON_BROWSER,
    }
}

fn get_process_id(hwnd: HWND) -> Option<u32> {
    let process_id = unsafe {
        let mut process_id = 0u32;
        GetWindowThreadProcessId(hwnd, Some(&mut process_id));
        process_id
    };

    if process_id == 0 {
        return None;
    }

    Some(process_id)
}

fn get_process_path(process_id: u32) -> Option<String> {
    let process_handle = open_process(process_id)?;
    let path = get_module_file_name(process_handle)
        .or_else(|| get_query_process_image_name(process_handle));

    unsafe {
        let _ = CloseHandle(process_handle);
    }

    path
}

fn open_process(process_id: u32) -> Option<HANDLE> {
    unsafe {
        OpenProcess(
            PROCESS_QUERY_LIMITED_INFORMATION | PROCESS_QUERY_INFORMATION | PROCESS_VM_READ,
            false,
            process_id,
        )
        .ok()
    }
}

fn get_module_file_name(process: HANDLE) -> Option<String> {
    let mut buffer = vec![0u16; 1024];
    let size = unsafe { GetModuleFileNameExW(Some(process), None, &mut buffer) };
    if size == 0 {
        return None;
    }

    Some(String::from_utf16_lossy(&buffer[..size as usize]))
}

fn get_query_process_image_name(process: HANDLE) -> Option<String> {
    let mut buffer = vec![0u16; 1024];
    let mut size = buffer.len() as u32;

    if unsafe {
        QueryFullProcessImageNameW(
            process,
            PROCESS_NAME_WIN32,
            PWSTR(buffer.as_mut_ptr()),
            &mut size,
        )
    }
    .is_err()
        || size == 0
    {
        return None;
    }

    Some(String::from_utf16_lossy(&buffer[..size as usize]))
}

#[cfg(test)]
mod tests {
    use super::{
        classify_browser_kind, should_refresh_cached_shell_ball_window_context,
        should_refresh_window_context_url, ActiveWindowContextPayload, BROWSER_KIND_CHROME,
        BROWSER_KIND_EDGE, BROWSER_KIND_NON_BROWSER, BROWSER_KIND_OTHER_BROWSER,
    };

    fn build_context(browser_kind: &str) -> ActiveWindowContextPayload {
        ActiveWindowContextPayload {
            app_name: "browser".to_string(),
            process_path: Some("C:/browser.exe".to_string()),
            process_id: Some(42),
            title: Some("Title".to_string()),
            url: None,
            browser_kind: browser_kind.to_string(),
            window_switch_count: None,
            page_switch_count: None,
        }
    }

    fn build_cached_context(browser_kind: &str, url: Option<&str>) -> ActiveWindowContextPayload {
        let mut context = build_context(browser_kind);
        context.url = url.map(ToString::to_string);
        context
    }

    #[test]
    fn classify_browser_kind_distinguishes_supported_and_unsupported_targets() {
        assert_eq!(classify_browser_kind("chrome"), BROWSER_KIND_CHROME);
        assert_eq!(classify_browser_kind("msedge"), BROWSER_KIND_EDGE);
        assert_eq!(classify_browser_kind("firefox"), BROWSER_KIND_OTHER_BROWSER);
        assert_eq!(classify_browser_kind("brave"), BROWSER_KIND_OTHER_BROWSER);
        assert_eq!(classify_browser_kind("notepad"), BROWSER_KIND_NON_BROWSER);
    }

    #[test]
    fn refreshable_browser_kinds_match_the_supported_takeover_boundary() {
        assert!(should_refresh_window_context_url(&build_context(
            BROWSER_KIND_CHROME
        )));
        assert!(should_refresh_window_context_url(&build_context(
            BROWSER_KIND_EDGE
        )));
        assert!(should_refresh_window_context_url(&build_context(
            BROWSER_KIND_OTHER_BROWSER,
        )));
        assert!(!should_refresh_window_context_url(&build_context(
            BROWSER_KIND_NON_BROWSER
        )));
    }

    #[test]
    fn shell_ball_cached_context_refreshes_browser_context_even_with_recent_urls() {
        assert!(should_refresh_cached_shell_ball_window_context(
            &build_cached_context(BROWSER_KIND_CHROME, Some("https://example.com/build"))
        ));
        assert!(should_refresh_cached_shell_ball_window_context(
            &build_cached_context(BROWSER_KIND_EDGE, None)
        ));
        assert!(!should_refresh_cached_shell_ball_window_context(
            &build_cached_context(BROWSER_KIND_NON_BROWSER, None)
        ));
    }
}

fn extract_process_stem(path: &str) -> Option<String> {
    Path::new(path)
        .file_stem()
        .and_then(|stem| stem.to_str())
        .map(ToString::to_string)
}

fn get_window_title(hwnd: HWND) -> Option<String> {
    let text_length = unsafe { GetWindowTextLengthW(hwnd) };
    if text_length <= 0 {
        return None;
    }

    let mut buffer = vec![0u16; text_length as usize + 1];
    let written = unsafe { GetWindowTextW(hwnd, &mut buffer) };
    if written <= 0 {
        return None;
    }

    Some(String::from_utf16_lossy(&buffer[..written as usize]))
}

fn read_browser_url_via_uia(hwnd: HWND) -> Option<String> {
    let _com_guard = ComGuard::initialize().ok()?;
    let automation: IUIAutomation =
        unsafe { CoCreateInstance(&CUIAutomation, None, CLSCTX_INPROC_SERVER).ok()? };
    let root_element = unsafe { automation.ElementFromHandle(hwnd).ok()? };
    let edit_control_type = VARIANT::from(UIA_EditControlTypeId.0);
    let condition: IUIAutomationCondition = unsafe {
        automation
            .CreatePropertyCondition(UIA_ControlTypePropertyId, &edit_control_type)
            .ok()?
    };
    let matches: IUIAutomationElementArray =
        unsafe { root_element.FindAll(TreeScope_Subtree, &condition).ok()? };
    let length = unsafe { matches.Length().ok()? };

    for index in 0..length {
        let element = unsafe { matches.GetElement(index).ok()? };
        if let Some(candidate_url) = read_element_url_candidate(&element) {
            return Some(candidate_url);
        }
    }

    None
}

fn read_element_url_candidate(element: &IUIAutomationElement) -> Option<String> {
    let name: BSTR = unsafe { element.CurrentName().ok()? };
    let normalized_name = name.to_string().trim().to_string();

    let value_pattern: IUIAutomationValuePattern =
        unsafe { element.GetCurrentPatternAs(UIA_ValuePatternId).ok()? };
    let value = unsafe { value_pattern.CurrentValue().ok()? }.to_string();
    let trimmed_value = value.trim();
    if looks_like_url(trimmed_value) {
        return Some(trimmed_value.to_string());
    }

    if looks_like_address_bar_name(&normalized_name) && !trimmed_value.is_empty() {
        return Some(trimmed_value.to_string());
    }

    if looks_like_url(&normalized_name) {
        return Some(normalized_name);
    }

    None
}

fn looks_like_address_bar_name(value: &str) -> bool {
    let lower = value.to_ascii_lowercase();

    lower.contains("address and search bar")
        || lower.contains("address bar")
        || lower.contains("search bar")
        || lower.contains("search or enter address")
        || lower.contains("search google or type a url")
        || value.contains("地址栏")
        || value.contains("地址和搜索栏")
        || value.contains("搜索栏")
        || value.contains("输入网址")
}

fn looks_like_url(value: &str) -> bool {
    let lower = value.to_ascii_lowercase();
    lower.starts_with("http://")
        || lower.starts_with("https://")
        || lower.starts_with("file://")
        || lower.starts_with("edge://")
        || lower.starts_with("chrome://")
        || lower.starts_with("about:")
}
