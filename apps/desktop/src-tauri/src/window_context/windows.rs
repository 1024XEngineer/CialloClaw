use super::types::ActiveWindowContextPayload;
use crate::activity::read_recent_mouse_points;
use crate::internal_windows::{INTERNAL_PINNED_WINDOW_PREFIX, INTERNAL_WINDOW_LABELS};
use once_cell::sync::Lazy;
use std::path::Path;
use std::sync::Mutex;
use std::thread;
use std::time::{Duration, Instant};
use tauri::{AppHandle, Manager};
use windows::core::{BSTR, PWSTR};
use windows::Win32::Foundation::{CloseHandle, HANDLE, HWND, POINT};
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
    UIA_ControlTypePropertyId, UIA_EditControlTypeId, UIA_TextControlTypeId, UIA_ValuePatternId,
    HWINEVENTHOOK,
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
const WINDOW_CONTEXT_VISIBLE_TEXT_MAX_CHARS: usize = 320;
const WINDOW_CONTEXT_VISIBLE_TEXT_MAX_CANDIDATES: usize = 8;
const WINDOW_CONTEXT_TARGET_MAX_CHARS: usize = 96;
const WINDOW_CONTEXT_ERROR_TEXT_MAX_CHARS: usize = 180;
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
    cached_at: Instant,
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

    let mut context = read_window_context_for_hwnd(hwnd);
    context.hover_target = context
        .hover_target
        .clone()
        .or_else(|| read_hover_target_for_window_context(hwnd, context.title.as_deref()));
    context.error_text = context.error_text.clone().or_else(|| {
        derive_error_text(
            context.visible_text.as_deref(),
            context.hover_target.as_deref(),
            context.title.as_deref(),
        )
    });
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

pub(crate) fn read_lightweight_window_context_for_hwnd(
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
        visible_text: None,
        hover_target: None,
        error_text: None,
        browser_kind: browser_kind.to_string(),
        window_switch_count: None,
        page_switch_count: None,
    })
}

fn default_window_context() -> ActiveWindowContextPayload {
    ActiveWindowContextPayload {
        app_name: "unknown".to_string(),
        process_path: None,
        process_id: None,
        title: None,
        url: None,
        visible_text: None,
        hover_target: None,
        error_text: None,
        browser_kind: BROWSER_KIND_NON_BROWSER.to_string(),
        window_switch_count: None,
        page_switch_count: None,
    }
}

pub(crate) fn read_cached_or_lightweight_window_context_for_hwnd(
    hwnd: HWND,
) -> ActiveWindowContextPayload {
    read_cached_window_context_for_hwnd(hwnd).unwrap_or_else(|| {
        read_lightweight_window_context_for_hwnd(hwnd).unwrap_or_else(|_| default_window_context())
    })
}

pub(crate) fn read_live_or_cached_window_context_for_hwnd(
    hwnd: HWND,
) -> ActiveWindowContextPayload {
    let cached_context = read_cached_window_context_for_hwnd(hwnd);
    if let Some(live_context) = try_read_live_window_context_for_hwnd(hwnd) {
        return merge_unresolved_browser_context_fields(live_context, cached_context.as_ref());
    }

    cached_context.unwrap_or_else(|| read_cached_or_lightweight_window_context_for_hwnd(hwnd))
}

fn read_window_context_for_hwnd(hwnd: HWND) -> ActiveWindowContextPayload {
    let cached_context = read_cached_window_context_for_hwnd(hwnd);
    if let Some(live_context) = try_read_live_window_context_for_hwnd(hwnd) {
        return merge_unresolved_browser_context_fields(live_context, cached_context.as_ref());
    }

    cached_context.unwrap_or_else(default_window_context)
}

fn try_read_live_window_context_for_hwnd(hwnd: HWND) -> Option<ActiveWindowContextPayload> {
    let mut context = read_lightweight_window_context_for_hwnd(hwnd).ok()?;

    if let Some(snapshot) = read_window_automation_snapshot(hwnd, &context) {
        context.url = snapshot.url;
        context.visible_text = snapshot.visible_text;
        context.hover_target = snapshot.hover_target;
        context.error_text = snapshot.error_text;
    } else {
        context.url = read_url_for_window_context(hwnd, &context);
    }
    Some(context)
}

fn merge_unresolved_browser_context_fields(
    mut live_context: ActiveWindowContextPayload,
    cached_context: Option<&ActiveWindowContextPayload>,
) -> ActiveWindowContextPayload {
    let Some(cached_context) = cached_context else {
        return live_context;
    };

    // Only preserve the last resolved browser URL. Reusing visible text, hover
    // targets, or error hints would mix old page content into a newly navigated tab.
    if should_refresh_window_context_url(&live_context) && live_context.url.is_none() {
        live_context.url = cached_context.url.clone();
    }

    live_context
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
    let root_window = get_root_window(hwnd);
    if let Ok(mut cached_context) = LAST_EXTERNAL_WINDOW_CONTEXT.lock() {
        *cached_context = Some(CachedWindowContext {
            hwnd: root_window.0 as isize,
            context: context.clone(),
            cached_at: Instant::now(),
        });
    }
}

// Internal desktop windows should only reuse the last external foreground
// snapshot for a short time. Otherwise a dashboard or shell-ball submit can
// incorrectly report a long-stale browser tab as the current webpage.
fn read_fresh_cached_window_context() -> Option<CachedWindowContext> {
    let mut cached_context = LAST_EXTERNAL_WINDOW_CONTEXT.lock().ok()?;
    let cached = cached_context.clone()?;

    if cached.cached_at.elapsed()
        > Duration::from_millis(INTERNAL_WINDOW_CONTEXT_REUSE_MAX_AGE_MS)
    {
        *cached_context = None;
        return None;
    }

    Some(cached)
}

fn read_cached_window_context_for_hwnd(hwnd: HWND) -> Option<ActiveWindowContextPayload> {
    read_fresh_cached_window_context()
        .and_then(|cached| {
            let root_window = get_root_window(hwnd).0 as isize;
            (cached.hwnd == root_window).then_some(cached.context)
        })
}

fn read_cached_window_context() -> Option<ActiveWindowContextPayload> {
    read_fresh_cached_window_context().map(|cached| cached.context)
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

    if let Some(context) = try_read_live_window_context_for_hwnd(hwnd)
        .map(|context| merge_unresolved_browser_context_fields(context, Some(&cached.context)))
    {
        record_page_switch_after_url_refresh(&context);
        cache_window_context(hwnd, &context);
        schedule_window_context_url_refresh(hwnd, &context);
        return Some(with_window_context_activity_counts(context));
    }

    let mut context = cached.context;
    context.hover_target = read_hover_target_for_window_context(hwnd, context.title.as_deref())
        .or_else(|| context.hover_target.clone());
    context.error_text = derive_error_text(
        context.visible_text.as_deref(),
        context.hover_target.as_deref(),
        context.title.as_deref(),
    );
    cache_window_context(hwnd, &context);
    schedule_window_context_url_refresh(hwnd, &context);
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
        let next_context = read_window_context_for_hwnd(hwnd);
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

struct WindowAutomationSnapshot {
    url: Option<String>,
    visible_text: Option<String>,
    hover_target: Option<String>,
    error_text: Option<String>,
}

fn read_window_automation_snapshot(
    hwnd: HWND,
    context: &ActiveWindowContextPayload,
) -> Option<WindowAutomationSnapshot> {
    let _com_guard = ComGuard::initialize().ok()?;
    let automation: IUIAutomation =
        unsafe { CoCreateInstance(&CUIAutomation, None, CLSCTX_INPROC_SERVER).ok()? };
    let root_element = unsafe { automation.ElementFromHandle(hwnd).ok()? };
    let url = if should_refresh_window_context_url(context) {
        read_browser_url_from_root(&automation, &root_element)
    } else {
        None
    };
    let visible_text =
        read_visible_text_from_root(&automation, &root_element, context.title.as_deref());
    let hover_target =
        read_target_candidate_from_recent_points(&automation, hwnd, context.title.as_deref())
            .or_else(|| {
                read_target_candidate_from_focused_element(
                    &automation,
                    hwnd,
                    context.title.as_deref(),
                )
            });
    let error_text = derive_error_text(
        visible_text.as_deref(),
        hover_target.as_deref(),
        context.title.as_deref(),
    );

    Some(WindowAutomationSnapshot {
        url,
        visible_text,
        hover_target,
        error_text,
    })
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
    read_browser_url_from_root(&automation, &root_element)
}

fn read_hover_target_for_window_context(hwnd: HWND, window_title: Option<&str>) -> Option<String> {
    let _com_guard = ComGuard::initialize().ok()?;
    let automation: IUIAutomation =
        unsafe { CoCreateInstance(&CUIAutomation, None, CLSCTX_INPROC_SERVER).ok()? };

    read_target_candidate_from_recent_points(&automation, hwnd, window_title)
        .or_else(|| read_target_candidate_from_focused_element(&automation, hwnd, window_title))
}

fn read_browser_url_from_root(
    automation: &IUIAutomation,
    root_element: &IUIAutomationElement,
) -> Option<String> {
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

fn read_visible_text_from_root(
    automation: &IUIAutomation,
    root_element: &IUIAutomationElement,
    window_title: Option<&str>,
) -> Option<String> {
    let mut candidates = Vec::new();
    collect_visible_text_candidates(
        automation,
        root_element,
        UIA_TextControlTypeId.0,
        window_title,
        &mut candidates,
    );
    collect_visible_text_candidates(
        automation,
        root_element,
        UIA_EditControlTypeId.0,
        window_title,
        &mut candidates,
    );

    if candidates.is_empty() {
        return None;
    }

    Some(truncate_text(
        &candidates.join(" "),
        WINDOW_CONTEXT_VISIBLE_TEXT_MAX_CHARS,
    ))
}

fn collect_visible_text_candidates(
    automation: &IUIAutomation,
    root_element: &IUIAutomationElement,
    control_type_id: i32,
    window_title: Option<&str>,
    candidates: &mut Vec<String>,
) {
    if candidates.len() >= WINDOW_CONTEXT_VISIBLE_TEXT_MAX_CANDIDATES {
        return;
    }

    let control_type = VARIANT::from(control_type_id);
    let Some(condition) = (unsafe {
        automation
            .CreatePropertyCondition(UIA_ControlTypePropertyId, &control_type)
            .ok()
    }) else {
        return;
    };
    let Some(matches) = (unsafe { root_element.FindAll(TreeScope_Subtree, &condition).ok() })
    else {
        return;
    };
    let Some(length) = (unsafe { matches.Length().ok() }) else {
        return;
    };

    for index in 0..length {
        if candidates.len() >= WINDOW_CONTEXT_VISIBLE_TEXT_MAX_CANDIDATES {
            break;
        }

        let Some(element) = (unsafe { matches.GetElement(index).ok() }) else {
            continue;
        };

        if let Some(candidate) = read_visible_text_candidate(&element, window_title) {
            push_unique_text_candidate(
                candidates,
                candidate,
                WINDOW_CONTEXT_VISIBLE_TEXT_MAX_CANDIDATES,
            );
        }
    }
}

fn read_visible_text_candidate(
    element: &IUIAutomationElement,
    window_title: Option<&str>,
) -> Option<String> {
    let value_candidate = read_value_pattern_text(element)
        .and_then(|value| normalize_text_candidate(&value, window_title));
    let name_candidate = read_current_name_text(element)
        .and_then(|value| normalize_text_candidate(&value, window_title));

    match (value_candidate, name_candidate) {
        (Some(value), Some(name)) if value != name && value.len() < name.len() => Some(name),
        (Some(value), Some(_)) => Some(value),
        (Some(value), None) => Some(value),
        (None, Some(name)) => Some(name),
        (None, None) => None,
    }
}

fn read_target_candidate_from_focused_element(
    automation: &IUIAutomation,
    hwnd: HWND,
    window_title: Option<&str>,
) -> Option<String> {
    let focused = unsafe { automation.GetFocusedElement().ok()? };
    let focused_hwnd = unsafe { focused.CurrentNativeWindowHandle().ok()? };
    if focused_hwnd.0.is_null() {
        return None;
    }

    if get_root_window(focused_hwnd) != get_root_window(hwnd) {
        return None;
    }

    read_target_candidate(&focused, window_title)
}

fn read_target_candidate_from_recent_points(
    automation: &IUIAutomation,
    hwnd: HWND,
    window_title: Option<&str>,
) -> Option<String> {
    let root_hwnd = get_root_window(hwnd);

    for (cursor_x, cursor_y) in read_recent_mouse_points() {
        let point = POINT {
            x: cursor_x,
            y: cursor_y,
        };
        let Some(element) = (unsafe { automation.ElementFromPoint(point).ok() }) else {
            continue;
        };
        let Some(element_hwnd) = (unsafe { element.CurrentNativeWindowHandle().ok() }) else {
            continue;
        };

        if element_hwnd.0.is_null() || get_root_window(element_hwnd) != root_hwnd {
            continue;
        }

        if let Some(candidate) = read_target_candidate(&element, window_title) {
            return Some(candidate);
        }
    }

    None
}

fn read_target_candidate(
    element: &IUIAutomationElement,
    window_title: Option<&str>,
) -> Option<String> {
    let candidate = read_current_name_text(element)
        .and_then(|value| normalize_text_candidate(&value, window_title))
        .or_else(|| {
            read_value_pattern_text(element)
                .and_then(|value| normalize_text_candidate(&value, window_title))
        })?;

    Some(truncate_text(&candidate, WINDOW_CONTEXT_TARGET_MAX_CHARS))
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

fn read_current_name_text(element: &IUIAutomationElement) -> Option<String> {
    let name: BSTR = unsafe { element.CurrentName().ok()? };
    let normalized = name.to_string();
    (!normalized.trim().is_empty()).then_some(normalized)
}

fn read_value_pattern_text(element: &IUIAutomationElement) -> Option<String> {
    let value_pattern: IUIAutomationValuePattern =
        unsafe { element.GetCurrentPatternAs(UIA_ValuePatternId).ok()? };
    let value = unsafe { value_pattern.CurrentValue().ok()? }.to_string();
    (!value.trim().is_empty()).then_some(value)
}

fn normalize_text_candidate(value: &str, window_title: Option<&str>) -> Option<String> {
    let trimmed = value.split_whitespace().collect::<Vec<_>>().join(" ");
    let normalized = trimmed.trim();

    if normalized.is_empty()
        || looks_like_url(normalized)
        || looks_like_address_bar_name(normalized)
    {
        return None;
    }

    if window_title.is_some_and(|title| normalized.eq_ignore_ascii_case(title.trim())) {
        return None;
    }

    Some(normalized.to_string())
}

fn push_unique_text_candidate(candidates: &mut Vec<String>, candidate: String, max_len: usize) {
    if candidates
        .iter()
        .any(|existing| existing.eq_ignore_ascii_case(candidate.as_str()))
    {
        return;
    }

    candidates.push(candidate);
    if candidates.len() > max_len {
        candidates.truncate(max_len);
    }
}

fn derive_error_text(
    visible_text: Option<&str>,
    hover_target: Option<&str>,
    window_title: Option<&str>,
) -> Option<String> {
    for value in [visible_text, hover_target, window_title]
        .into_iter()
        .flatten()
    {
        if looks_like_actionable_error_signal(value) {
            return Some(truncate_text(value, WINDOW_CONTEXT_ERROR_TEXT_MAX_CHARS));
        }
    }

    None
}

#[allow(dead_code)]
fn looks_like_error_signal(value: &str) -> bool {
    let normalized = value.to_lowercase();
    [
        "error",
        "failed",
        "failure",
        "exception",
        "错误",
        "失败",
        "异常",
        "报错",
        "出错",
    ]
    .iter()
    .any(|token| normalized.contains(token))
}

fn looks_like_actionable_error_signal(value: &str) -> bool {
    let normalized = value
        .to_lowercase()
        .split_whitespace()
        .collect::<Vec<_>>()
        .join(" ");
    if normalized.is_empty() {
        return false;
    }

    if [
        "fatal error",
        "syntax error",
        "runtime error",
        "server error",
        "internal error",
        "error occurred",
        "exception occurred",
        "uncaught exception",
        "unhandled exception",
        "operation failed",
        "request failed",
        "build failed",
        "login failed",
        "\u{53D1}\u{751F}\u{9519}\u{8BEF}",
        "\u{51FA}\u{73B0}\u{9519}\u{8BEF}",
        "\u{68C0}\u{6D4B}\u{5230}\u{9519}\u{8BEF}",
        "\u{53D1}\u{751F}\u{5F02}\u{5E38}",
        "\u{51FA}\u{73B0}\u{5F02}\u{5E38}",
        "\u{64CD}\u{4F5C}\u{5931}\u{8D25}",
        "\u{8BF7}\u{6C42}\u{5931}\u{8D25}",
        "\u{52A0}\u{8F7D}\u{5931}\u{8D25}",
        "\u{6267}\u{884C}\u{5931}\u{8D25}",
    ]
    .iter()
    .any(|phrase| normalized.contains(phrase))
    {
        return true;
    }

    ["error", "exception", "\u{9519}\u{8BEF}", "\u{5F02}\u{5E38}"]
        .iter()
        .any(|label| looks_like_labeled_error_phrase(&normalized, label))
        || [
            "failed to",
            " failure:",
            " failure ",
            "\u{5931}\u{8D25}",
            "\u{62A5}\u{9519}",
            "\u{51FA}\u{9519}",
        ]
        .iter()
        .any(|phrase| normalized.contains(phrase))
}

fn looks_like_labeled_error_phrase(normalized: &str, label: &str) -> bool {
    let ascii_label = format!("{label}:");
    let wide_label = format!("{label}\u{FF1A}");
    normalized.starts_with(ascii_label.as_str())
        || normalized.starts_with(wide_label.as_str())
        || normalized.contains(format!(" {ascii_label}").as_str())
        || normalized.contains(format!(" {wide_label}").as_str())
}

fn truncate_text(value: &str, max_chars: usize) -> String {
    if value.chars().count() <= max_chars {
        return value.to_string();
    }

    value.chars().take(max_chars).collect()
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

#[cfg(test)]
mod tests {
    use super::looks_like_actionable_error_signal as looks_like_error_signal;

    #[test]
    fn matches_explicit_failure_tokens_across_languages() {
        assert!(looks_like_error_signal("Error: publish failed"));
        assert!(looks_like_error_signal("当前操作失败，请稍后重试"));
        assert!(looks_like_error_signal("检测到错误：配置无效"));
        assert!(looks_like_error_signal("服务出现异常"));
        assert!(!looks_like_error_signal(
            "Warning: release notes are incomplete."
        ));
    }

    #[test]
    fn skips_generic_error_discussion_copy() {
        assert!(!looks_like_error_signal("Rust error handling patterns"));
        assert!(!looks_like_error_signal("Exception safety checklist"));
    }
}
