// This entry point boots the desktop Tauri host process.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod activity;
mod screen_capture;
mod selection;
mod window_context;

use serde::Deserialize;
use serde_json::Value;
use once_cell::sync::Lazy;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::collections::{HashMap, HashSet};
use std::fs::{self, OpenOptions};
use std::io::{BufReader, BufWriter, Write};
use std::sync::atomic::{AtomicU32, Ordering};
use std::sync::{mpsc, Arc, Mutex};
use tauri::ipc::Channel;
use tauri::menu::{MenuBuilder, MenuItemBuilder};
use tauri::tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent};
use tauri::{Emitter, Manager, WebviewUrl, WebviewWindowBuilder};

#[cfg(windows)]
use std::ffi::OsStr;

#[cfg(windows)]
use std::os::windows::ffi::OsStrExt;

#[cfg(windows)]
use windows::Win32::{
    Foundation::{HGLOBAL, HWND, LPARAM, LRESULT, POINT, RECT, WPARAM},
    Graphics::Gdi::{PtInRect, ScreenToClient},
    System::{
        DataExchange::{
            CloseClipboard, GetClipboardData, GetClipboardSequenceNumber,
            IsClipboardFormatAvailable, OpenClipboard,
        },
        Memory::{GlobalLock, GlobalUnlock},
        Ole::CF_UNICODETEXT,
    },
    UI::Shell::ShellExecuteW,
    UI::Input::KeyboardAndMouse::{GetAsyncKeyState, VK_CONTROL, VK_DELETE, VK_SHIFT},
    UI::WindowsAndMessaging::*,
};

#[cfg(windows)]
use windows::core::PCWSTR;

type JsonChannel = Channel<Value>;

const NAMED_PIPE_PATH: &str = r"\\.\pipe\cialloclaw-rpc";
const CONTROL_PANEL_WINDOW_LABEL: &str = "control-panel";
const DASHBOARD_WINDOW_LABEL: &str = "dashboard";
const SHELL_BALL_WINDOW_LABEL: &str = "shell-ball";
const SHELL_BALL_BUBBLE_WINDOW_LABEL: &str = "shell-ball-bubble";
const SHELL_BALL_INPUT_WINDOW_LABEL: &str = "shell-ball-input";
const SHELL_BALL_VOICE_WINDOW_LABEL: &str = "shell-ball-voice";
const SHELL_BALL_PINNED_WINDOW_PREFIX: &str = "shell-ball-bubble-pinned-";
const SHELL_BALL_DASHBOARD_TRANSITION_REQUEST_EVENT: &str =
    "desktop-shell-ball:dashboard-transition-request";
const SHELL_BALL_CLIPBOARD_SNAPSHOT_EVENT: &str = "desktop-shell-ball:clipboard-snapshot";
const TRAY_ICON_ID: &str = "main-tray";
const TRAY_MENU_SHOW_SHELL_BALL_ID: &str = "show-shell-ball";
const TRAY_MENU_HIDE_SHELL_BALL_ID: &str = "hide-shell-ball";
const TRAY_MENU_OPEN_CONTROL_PANEL_ID: &str = "open-control-panel";
const TRAY_MENU_QUIT_ID: &str = "quit-app";

static DESKTOP_EXTERNAL_OPEN_APPROVALS: Lazy<Mutex<HashSet<String>>> =
    Lazy::new(|| Mutex::new(HashSet::new()));

#[cfg(windows)]
macro_rules! makelparam {
    ($low:expr, $high:expr) => {
        (((($low) & 0xffff) as u32) | (((($high) & 0xffff) as u32) << 16)) as _
    };
}

enum BridgeCommand {
    Request { payload: Value },
}

#[derive(Clone)]
struct BridgeSession {
    writer_tx: mpsc::Sender<BridgeCommand>,
}

struct NamedPipeBridgeState {
    session: Mutex<Option<BridgeSession>>,
    pending: Mutex<HashMap<String, mpsc::Sender<Result<Value, String>>>>,
    subscriptions: Mutex<HashMap<String, HashMap<u32, JsonChannel>>>,
    next_subscription_id: AtomicU32,
}

impl Default for NamedPipeBridgeState {
    fn default() -> Self {
        Self {
            session: Mutex::new(None),
            pending: Mutex::new(HashMap::new()),
            subscriptions: Mutex::new(HashMap::new()),
            next_subscription_id: AtomicU32::new(1),
        }
    }
}

impl NamedPipeBridgeState {
    fn request(self: &Arc<Self>, payload: Value) -> Result<Value, String> {
        let request_id = extract_request_id(&payload)?;
        let session = self.ensure_session()?;
        let (response_tx, response_rx) = mpsc::channel();

        self.pending
            .lock()
            .map_err(|_| "named pipe pending map lock poisoned".to_string())?
            .insert(request_id.clone(), response_tx);

        if let Err(error) = session.writer_tx.send(BridgeCommand::Request { payload }) {
            self.pending
                .lock()
                .map_err(|_| "named pipe pending map lock poisoned".to_string())?
                .remove(&request_id);
            return Err(format!("failed to queue named pipe request: {error}"));
        }

        response_rx
            .recv()
            .map_err(|error| format!("named pipe response wait failed: {error}"))?
    }

    fn subscribe(self: &Arc<Self>, topic: String, channel: JsonChannel) -> Result<u32, String> {
        self.ensure_session()?;

        let subscription_id = self.next_subscription_id.fetch_add(1, Ordering::Relaxed);
        let mut subscriptions = self
            .subscriptions
            .lock()
            .map_err(|_| "named pipe subscriptions lock poisoned".to_string())?;

        subscriptions
            .entry(topic)
            .or_insert_with(HashMap::new)
            .insert(subscription_id, channel);

        Ok(subscription_id)
    }

    fn unsubscribe(&self, subscription_id: u32) -> Result<(), String> {
        let mut subscriptions = self
            .subscriptions
            .lock()
            .map_err(|_| "named pipe subscriptions lock poisoned".to_string())?;

        for topic_channels in subscriptions.values_mut() {
            if topic_channels.remove(&subscription_id).is_some() {
                return Ok(());
            }
        }

        Ok(())
    }

    fn ensure_session(self: &Arc<Self>) -> Result<BridgeSession, String> {
        let mut session_guard = self
            .session
            .lock()
            .map_err(|_| "named pipe session lock poisoned".to_string())?;

        if let Some(session) = session_guard.clone() {
            return Ok(session);
        }

        let stream = OpenOptions::new()
            .read(true)
            .write(true)
            .open(NAMED_PIPE_PATH)
            .map_err(|error| format!("failed to open named pipe {NAMED_PIPE_PATH}: {error}"))?;

        let reader = stream
            .try_clone()
            .map_err(|error| format!("failed to clone named pipe handle: {error}"))?;

        let writer = stream;
        let (writer_tx, writer_rx) = mpsc::channel();
        let state = Arc::clone(self);
        let writer_state = Arc::clone(&state);
        std::thread::spawn(move || writer_loop(writer, writer_rx, writer_state));
        std::thread::spawn(move || reader_loop(reader, state));

        let session = BridgeSession { writer_tx };
        *session_guard = Some(session.clone());
        Ok(session)
    }

    fn dispatch_incoming(&self, message: Value) {
        if let Some(method) = message.get("method").and_then(Value::as_str) {
            self.dispatch_notification(method, &message);
            return;
        }

        if let Some(id) = message.get("id") {
            let request_id = normalize_id(id);
            if let Ok(mut pending) = self.pending.lock() {
                if let Some(sender) = pending.remove(&request_id) {
                    let _ = sender.send(Ok(message));
                }
            }
        }
    }

    fn dispatch_notification(&self, topic: &str, message: &Value) {
        let channels = self
            .subscriptions
            .lock()
            .ok()
            .and_then(|subscriptions| subscriptions.get(topic).cloned());

        if let Some(channels) = channels {
            for (_, channel) in channels {
                let _ = channel.send(message.clone());
            }
        }
    }

    fn handle_disconnect(&self, reason: String) {
        if let Ok(mut session) = self.session.lock() {
            *session = None;
        }

        if let Ok(mut pending) = self.pending.lock() {
            for (_, sender) in pending.drain() {
                let _ = sender.send(Err(reason.clone()));
            }
        }

        let message = serde_json::json!({
            "method": "bridge.disconnected",
            "params": {
                "reason": reason,
            }
        });
        self.dispatch_notification("bridge.disconnected", &message);
    }
}

#[tauri::command]
async fn named_pipe_request(
    state: tauri::State<'_, Arc<NamedPipeBridgeState>>,
    payload: Value,
) -> Result<Value, String> {
    let state = Arc::clone(state.inner());
    tauri::async_runtime::spawn_blocking(move || state.request(payload))
        .await
        .map_err(|error| format!("named pipe bridge task failed: {error}"))?
}

#[tauri::command]
async fn named_pipe_subscribe(
    state: tauri::State<'_, Arc<NamedPipeBridgeState>>,
    topic: String,
    on_event: JsonChannel,
) -> Result<u32, String> {
    let state = Arc::clone(state.inner());
    tauri::async_runtime::spawn_blocking(move || state.subscribe(topic, on_event))
        .await
        .map_err(|error| format!("named pipe subscribe task failed: {error}"))?
}

#[tauri::command]
async fn named_pipe_unsubscribe(
    state: tauri::State<'_, Arc<NamedPipeBridgeState>>,
    subscription_id: u32,
) -> Result<(), String> {
    let state = Arc::clone(state.inner());
    tauri::async_runtime::spawn_blocking(move || state.unsubscribe(subscription_id))
        .await
        .map_err(|error| format!("named pipe unsubscribe task failed: {error}"))?
}

#[tauri::command]
fn desktop_get_mouse_activity_snapshot() -> Option<activity::MouseActivitySnapshotPayload> {
    activity::read_mouse_activity_snapshot()
}

#[tauri::command]
async fn desktop_capture_screenshot() -> Result<screen_capture::ScreenCapturePayload, String> {
    tauri::async_runtime::spawn_blocking(screen_capture::capture_screenshot)
        .await
        .map_err(|error| format!("desktop screenshot task failed: {error}"))?
}

#[tauri::command]
async fn desktop_get_active_window_context(
) -> Result<Option<window_context::ActiveWindowContextPayload>, String> {
    tauri::async_runtime::spawn_blocking(window_context::read_active_window_context)
        .await
        .map_err(|error| format!("desktop window-context task failed: {error}"))?
}

fn writer_loop(
    writer: std::fs::File,
    receiver: mpsc::Receiver<BridgeCommand>,
    state: Arc<NamedPipeBridgeState>,
) {
    let mut writer = BufWriter::new(writer);

    while let Ok(command) = receiver.recv() {
        let result = match command {
            BridgeCommand::Request { payload } => (|| -> Result<(), String> {
                serde_json::to_writer(&mut writer, &payload)
                    .map_err(|error| format!("failed to serialize json-rpc payload: {error}"))?;
                writer
                    .write_all(b"\n")
                    .map_err(|error| format!("failed to write named pipe delimiter: {error}"))?;
                writer
                    .flush()
                    .map_err(|error| format!("failed to flush named pipe payload: {error}"))?;
                Ok(())
            })(),
        };

        if let Err(error) = result {
            state.handle_disconnect(error);
            return;
        }
    }
}

fn reader_loop(reader: std::fs::File, state: Arc<NamedPipeBridgeState>) {
    let mut responses =
        serde_json::Deserializer::from_reader(BufReader::new(reader)).into_iter::<Value>();

    while let Some(result) = responses.next() {
        match result {
            Ok(message) => state.dispatch_incoming(message),
            Err(error) => {
                state.handle_disconnect(format!("failed to decode named pipe response: {error}"));
                return;
            }
        }
    }

    state.handle_disconnect(
        "named pipe response stream ended before any json-rpc envelope was returned".to_string(),
    );
}

fn extract_request_id(payload: &Value) -> Result<String, String> {
    let id = payload
        .get("id")
        .ok_or_else(|| "json-rpc payload missing id".to_string())?;

    Ok(normalize_id(id))
}

fn normalize_id(id: &Value) -> String {
    serde_json::to_string(id).unwrap_or_else(|_| "null".to_string())
}

fn focus_webview_window(app: &tauri::AppHandle, label: &str) -> Result<(), String> {
    let window = app
        .get_webview_window(label)
        .ok_or_else(|| format!("webview window not found: {label}"))?;

    window
        .unminimize()
        .map_err(|error| format!("failed to unminimize {label}: {error}"))?;
    window
        .show()
        .map_err(|error| format!("failed to show {label}: {error}"))?;
    window
        .set_focus()
        .map_err(|error| format!("failed to focus {label}: {error}"))?;

    Ok(())
}

fn open_or_focus_control_panel_window(app: &tauri::AppHandle) {
    if app.get_webview_window(CONTROL_PANEL_WINDOW_LABEL).is_some() {
        if let Err(error) = focus_webview_window(app, CONTROL_PANEL_WINDOW_LABEL) {
            eprintln!("failed to focus control panel from tray: {error}");
        }
        return;
    }

    let handle = app.clone();
    std::thread::spawn(move || {
        let create_result = WebviewWindowBuilder::new(
            &handle,
            CONTROL_PANEL_WINDOW_LABEL,
            WebviewUrl::App("control-panel.html".into()),
        )
        .title("CialloClaw Control Panel")
        .inner_size(1080.0, 760.0)
        .decorations(false)
        .visible(true)
        .focused(true)
        .build();

        if let Err(error) = create_result {
            eprintln!("failed to create control panel from tray: {error}");
        }
    });
}

fn request_shell_ball_dashboard_open_transition(app: &tauri::AppHandle) -> Result<(), String> {
    app.emit_to(
        SHELL_BALL_WINDOW_LABEL,
        SHELL_BALL_DASHBOARD_TRANSITION_REQUEST_EVENT,
        serde_json::json!({
            "direction": "open"
        }),
    )
    .map_err(|error| format!("failed to emit shell-ball dashboard transition request: {error}"))
}

fn hide_shell_ball_cluster(app: &tauri::AppHandle) -> Result<(), String> {
    let shell_ball_labels = [
        SHELL_BALL_WINDOW_LABEL,
        SHELL_BALL_BUBBLE_WINDOW_LABEL,
        SHELL_BALL_INPUT_WINDOW_LABEL,
        SHELL_BALL_VOICE_WINDOW_LABEL,
    ];

    for label in shell_ball_labels {
        if let Some(window) = app.get_webview_window(label) {
            window
                .hide()
                .map_err(|error| format!("failed to hide {label}: {error}"))?;
        }
    }

    for window in app.webview_windows().values() {
        if window.label().starts_with(SHELL_BALL_PINNED_WINDOW_PREFIX) {
            window.hide().map_err(|error| {
                format!(
                    "failed to hide shell-ball pinned bubble {}: {error}",
                    window.label()
                )
            })?;
        }
    }

    Ok(())
}

fn show_shell_ball(app: &tauri::AppHandle) -> Result<(), String> {
    let window = app
        .get_webview_window(SHELL_BALL_WINDOW_LABEL)
        .ok_or_else(|| format!("webview window not found: {SHELL_BALL_WINDOW_LABEL}"))?;

    window
        .unminimize()
        .map_err(|error| format!("failed to unminimize {SHELL_BALL_WINDOW_LABEL}: {error}"))?;
    window
        .show()
        .map_err(|error| format!("failed to show {SHELL_BALL_WINDOW_LABEL}: {error}"))?;
    window
        .set_focus()
        .map_err(|error| format!("failed to focus {SHELL_BALL_WINDOW_LABEL}: {error}"))?;

    Ok(())
}

#[cfg(windows)]
fn emit_shell_ball_clipboard_snapshot(app: &tauri::AppHandle, text: String) {
    let _ = app.emit_to(
        SHELL_BALL_WINDOW_LABEL,
        SHELL_BALL_CLIPBOARD_SNAPSHOT_EVENT,
        serde_json::json!({
            "text": text,
        }),
    );
}

#[cfg(windows)]
fn schedule_shell_ball_clipboard_probe(delay_ms: u64) {
    std::thread::spawn(move || {
        std::thread::sleep(std::time::Duration::from_millis(delay_ms));

        let Some(app) = SHELL_BALL_APP_HANDLE
            .lock()
            .ok()
            .and_then(|guard| guard.as_ref().cloned())
        else {
            return;
        };

        let Ok(sequence_number) = read_clipboard_sequence_number() else {
            return;
        };

        let should_emit = {
            let mut state = match SHELL_BALL_CLIPBOARD_STATE.lock() {
                Ok(guard) => guard,
                Err(_) => return,
            };

            if sequence_number == 0 || sequence_number == state.last_sequence_number {
                false
            } else {
                state.last_sequence_number = sequence_number;
                true
            }
        };

        if !should_emit {
            return;
        }

        if let Ok(Some(text)) = read_windows_clipboard_text() {
            emit_shell_ball_clipboard_snapshot(&app, text);
        }
    });
}

#[cfg(windows)]
fn read_clipboard_sequence_number() -> Result<u32, String> {
    let sequence_number = unsafe { GetClipboardSequenceNumber() };
    Ok(sequence_number)
}

#[cfg(windows)]
fn read_windows_clipboard_text() -> Result<Option<String>, String> {
    unsafe {
        OpenClipboard(None).map_err(|error| format!("failed to open clipboard: {error}"))?;

        let result = (|| {
            if IsClipboardFormatAvailable(CF_UNICODETEXT.0 as u32).is_err() {
                return Ok(None);
            }

            let clipboard_handle = GetClipboardData(CF_UNICODETEXT.0 as u32)
                .map_err(|error| format!("failed to get clipboard handle: {error}"))?;
            let clipboard_ptr = GlobalLock(HGLOBAL(clipboard_handle.0));
            if clipboard_ptr.is_null() {
                return Err("failed to lock clipboard handle".to_string());
            }

            let text = read_utf16_null_terminated(clipboard_ptr as *const u16);
            let _ = GlobalUnlock(HGLOBAL(clipboard_handle.0));

            if text.trim().is_empty() {
                return Ok(None);
            }

            Ok(Some(text))
        })();

        let _ = CloseClipboard();
        result
    }
}

#[cfg(windows)]
fn read_utf16_null_terminated(mut ptr: *const u16) -> String {
    let mut buffer = Vec::new();

    unsafe {
        while !ptr.is_null() && *ptr != 0 {
            buffer.push(*ptr);
            ptr = ptr.add(1);
        }
    }

    String::from_utf16_lossy(&buffer)
}

#[cfg(windows)]
unsafe extern "system" fn shell_ball_clipboard_mouse_hook(
    n_code: i32,
    w_param: WPARAM,
    l_param: LPARAM,
) -> LRESULT {
    if n_code >= 0 && w_param.0 as u32 == WM_RBUTTONUP {
        schedule_shell_ball_clipboard_probe(SHELL_BALL_CLIPBOARD_RIGHT_CLICK_DELAY_MS);
    }

    CallNextHookEx(None, n_code, w_param, l_param)
}

#[cfg(windows)]
unsafe extern "system" fn shell_ball_clipboard_keyboard_hook(
    n_code: i32,
    w_param: WPARAM,
    l_param: LPARAM,
) -> LRESULT {
    if n_code >= 0 && (w_param.0 as u32 == WM_KEYDOWN || w_param.0 as u32 == WM_SYSKEYDOWN) {
        let keyboard_info = *(l_param.0 as *const KBDLLHOOKSTRUCT);
        let ctrl_down = (GetAsyncKeyState(VK_CONTROL.0 as i32) as u16 & 0x8000) != 0;
        let shift_down = (GetAsyncKeyState(VK_SHIFT.0 as i32) as u16 & 0x8000) != 0;

        if ctrl_down && (keyboard_info.vkCode == b'C' as u32 || keyboard_info.vkCode == b'X' as u32) {
            schedule_shell_ball_clipboard_probe(SHELL_BALL_CLIPBOARD_COPY_DELAY_MS);
        }

        if shift_down && keyboard_info.vkCode == VK_DELETE.0 as u32 {
            schedule_shell_ball_clipboard_probe(SHELL_BALL_CLIPBOARD_COPY_DELAY_MS);
        }
    }

    CallNextHookEx(None, n_code, w_param, l_param)
}

#[cfg(windows)]
fn install_shell_ball_clipboard_hooks(app: &tauri::AppHandle) -> Result<(), String> {
    if let Ok(mut app_handle) = SHELL_BALL_APP_HANDLE.lock() {
        *app_handle = Some(app.clone());
    }

    if let Ok(mut state) = SHELL_BALL_CLIPBOARD_STATE.lock() {
        state.last_sequence_number = read_clipboard_sequence_number().unwrap_or(0);
    }

    let mut mouse_hook = SHELL_BALL_CLIPBOARD_MOUSE_HOOK
        .lock()
        .map_err(|_| "clipboard mouse hook lock poisoned".to_string())?;
    let mut keyboard_hook = SHELL_BALL_CLIPBOARD_KEYBOARD_HOOK
        .lock()
        .map_err(|_| "clipboard keyboard hook lock poisoned".to_string())?;

    if mouse_hook.is_none() {
        unsafe {
            *mouse_hook = Some(
                SetWindowsHookExW(WH_MOUSE_LL, Some(shell_ball_clipboard_mouse_hook), None, 0)
                    .map_err(|error| format!("failed to install clipboard mouse hook: {error}"))?
                    .0 as isize,
            );
        }
    }

    if keyboard_hook.is_none() {
        unsafe {
            *keyboard_hook = Some(
                SetWindowsHookExW(WH_KEYBOARD_LL, Some(shell_ball_clipboard_keyboard_hook), None, 0)
                    .map_err(|error| format!("failed to install clipboard keyboard hook: {error}"))?
                    .0 as isize,
            );
        }
    }

    Ok(())
}

#[cfg(not(windows))]
fn install_shell_ball_clipboard_hooks(_app: &tauri::AppHandle) -> Result<(), String> {
    Ok(())
}

fn install_system_tray(app: &mut tauri::App) -> tauri::Result<()> {
    let show_shell_ball_menu_item =
        MenuItemBuilder::with_id(TRAY_MENU_SHOW_SHELL_BALL_ID, "展示悬浮球").build(app)?;
    let hide_shell_ball =
        MenuItemBuilder::with_id(TRAY_MENU_HIDE_SHELL_BALL_ID, "隐藏悬浮球").build(app)?;
    let open_control_panel = MenuItemBuilder::with_id(
        TRAY_MENU_OPEN_CONTROL_PANEL_ID,
        "打开控制面板",
    )
    .build(app)?;
    let quit_app = MenuItemBuilder::with_id(TRAY_MENU_QUIT_ID, "关闭程序").build(app)?;
    let tray_menu = MenuBuilder::new(app)
        .items(&[
            &show_shell_ball_menu_item,
            &hide_shell_ball,
            &open_control_panel,
            &quit_app,
        ])
        .build()?;

    let tray_builder = TrayIconBuilder::with_id(TRAY_ICON_ID)
        .tooltip("CialloClaw")
        .menu(&tray_menu)
        .show_menu_on_left_click(false)
        .on_menu_event(|app, event| match event.id.as_ref() {
            TRAY_MENU_SHOW_SHELL_BALL_ID => {
                if let Err(error) = show_shell_ball(app) {
                    eprintln!("failed to show shell-ball from tray: {error}");
                }
            }
            TRAY_MENU_HIDE_SHELL_BALL_ID => {
                if let Err(error) = hide_shell_ball_cluster(app) {
                    eprintln!("failed to hide shell-ball from tray: {error}");
                }
            }
            TRAY_MENU_OPEN_CONTROL_PANEL_ID => {
                open_or_focus_control_panel_window(app);
            }
            TRAY_MENU_QUIT_ID => {
                app.exit(0);
            }
            _ => {}
        })
        .on_tray_icon_event(|tray, event| {
            if let TrayIconEvent::Click {
                button: MouseButton::Left,
                button_state: MouseButtonState::Up,
                ..
            } = event
            {
                if let Err(error) = request_shell_ball_dashboard_open_transition(tray.app_handle())
                {
                    eprintln!(
                        "failed to trigger shell-ball dashboard transition from tray: {error}"
                    );
                }

                if let Err(error) = focus_webview_window(tray.app_handle(), DASHBOARD_WINDOW_LABEL)
                {
                    eprintln!("failed to open dashboard from tray: {error}");
                }
            }
        });

    let tray_builder = if let Some(icon) = app.default_window_icon() {
        tray_builder.icon(icon.clone())
    } else {
        tray_builder
    };

    let _ = tray_builder.build(app)?;
    Ok(())
}

#[derive(Clone, serde::Serialize)]
struct CursorPosition {
    client_x: i32,
    client_y: i32,
}

#[cfg(windows)]
static SHELL_BALL_MOUSE_HOOK: Lazy<Mutex<Option<isize>>> = Lazy::new(|| Mutex::new(None));

#[cfg(windows)]
static FORWARDING_WINDOWS: Lazy<Mutex<HashSet<isize>>> = Lazy::new(|| Mutex::new(HashSet::new()));

#[cfg(windows)]
static SHELL_BALL_CLIPBOARD_MOUSE_HOOK: Lazy<Mutex<Option<isize>>> = Lazy::new(|| Mutex::new(None));

#[cfg(windows)]
static SHELL_BALL_CLIPBOARD_KEYBOARD_HOOK: Lazy<Mutex<Option<isize>>> = Lazy::new(|| Mutex::new(None));

#[cfg(windows)]
static SHELL_BALL_APP_HANDLE: Lazy<Mutex<Option<tauri::AppHandle>>> = Lazy::new(|| Mutex::new(None));

#[cfg(windows)]
static SHELL_BALL_CLIPBOARD_STATE: Lazy<Mutex<ClipboardMonitorState>> =
    Lazy::new(|| Mutex::new(ClipboardMonitorState::default()));

#[cfg(windows)]
const SHELL_BALL_CLIPBOARD_COPY_DELAY_MS: u64 = 140;

#[cfg(windows)]
const SHELL_BALL_CLIPBOARD_RIGHT_CLICK_DELAY_MS: u64 = 3_000;

#[cfg(windows)]
#[derive(Default)]
struct ClipboardMonitorState {
    last_sequence_number: u32,
}

#[cfg(windows)]
unsafe fn set_forward_mouse_messages(hwnd: HWND, forward: bool) {
    let browser_hwnd = {
        let host = match GetWindow(hwnd, GW_CHILD) {
            Ok(value) => value,
            Err(_) => return,
        };

        match GetWindow(host, GW_CHILD) {
            Ok(value) => value,
            Err(_) => return,
        }
    };

    let mut forwarding_windows = match FORWARDING_WINDOWS.lock() {
        Ok(guard) => guard,
        Err(_) => return,
    };

    let mut mouse_hook = match SHELL_BALL_MOUSE_HOOK.lock() {
        Ok(guard) => guard,
        Err(_) => return,
    };

    if forward {
        forwarding_windows.insert(browser_hwnd.0 as isize);

        if mouse_hook.is_none() {
            *mouse_hook = Some(
                SetWindowsHookExW(WH_MOUSE_LL, Some(mousemove_forward), None, 0)
                    .expect("failed to install shell-ball mouse hook")
                    .0 as isize,
            );
        }
    } else {
        forwarding_windows.remove(&(browser_hwnd.0 as isize));

        if forwarding_windows.is_empty() {
            if let Some(hook) = mouse_hook.take() {
                let _ = UnhookWindowsHookEx(HHOOK(hook as _));
            }
        }
    }
}

#[cfg(windows)]
unsafe extern "system" fn mousemove_forward(
    n_code: i32,
    w_param: WPARAM,
    l_param: LPARAM,
) -> LRESULT {
    if n_code < 0 {
        return CallNextHookEx(None, n_code, w_param, l_param);
    }

    if w_param.0 as u32 == WM_MOUSEMOVE {
        let point = (*(l_param.0 as *const MSLLHOOKSTRUCT)).pt;

        let forwarding_windows = match FORWARDING_WINDOWS.lock() {
            Ok(guard) => guard,
            Err(_) => return CallNextHookEx(None, n_code, w_param, l_param),
        };

        for &hwnd in forwarding_windows.iter() {
            let hwnd = HWND(hwnd as _);
            let mut client_rect = RECT {
                left: 0,
                top: 0,
                right: 0,
                bottom: 0,
            };

            if GetClientRect(hwnd, &mut client_rect).is_err() {
                continue;
            }

            let mut client_point = point;
            if !ScreenToClient(hwnd, &mut client_point).as_bool() {
                continue;
            }

            if PtInRect(&client_rect, client_point).as_bool() {
                let w = Some(WPARAM(1));
                let l = Some(LPARAM(makelparam!(client_point.x, client_point.y)));
                SendMessageW(hwnd, WM_MOUSEMOVE, w, l);
            }
        }
    }

    CallNextHookEx(None, n_code, w_param, l_param)
}

#[cfg(windows)]
#[tauri::command]
fn shell_ball_set_ignore_cursor_events(
    window: tauri::Window,
    ignore: bool,
    forward: bool,
) -> Result<(), String> {
    window
        .set_ignore_cursor_events(ignore)
        .map_err(|error| format!("failed to update shell-ball ignore cursor events: {error}"))?;

    let hwnd = window
        .hwnd()
        .map_err(|error| format!("failed to get shell-ball hwnd: {error}"))?;

    let should_forward = if ignore { forward } else { false };
    unsafe {
        set_forward_mouse_messages(hwnd, should_forward);
    }

    Ok(())
}

#[cfg(not(windows))]
#[tauri::command]
fn shell_ball_set_ignore_cursor_events(
    window: tauri::Window,
    ignore: bool,
    _forward: bool,
) -> Result<(), String> {
    window
        .set_ignore_cursor_events(ignore)
        .map_err(|error| format!("failed to update shell-ball ignore cursor events: {error}"))
}

#[cfg(windows)]
#[tauri::command]
fn shell_ball_get_mouse_position() -> Option<CursorPosition> {
    let mut point = POINT { x: 0, y: 0 };
    unsafe {
        if GetCursorPos(&mut point).is_ok() {
            Some(CursorPosition {
                client_x: point.x,
                client_y: point.y,
            })
        } else {
            None
        }
    }
}

#[cfg(not(windows))]
#[tauri::command]
fn shell_ball_get_mouse_position() -> Option<CursorPosition> {
    None
}

#[tauri::command]
fn pick_shell_ball_files(window: tauri::Window) -> Result<Vec<String>, String> {
    if window.label() != SHELL_BALL_INPUT_WINDOW_LABEL {
        return Err("pick_shell_ball_files is only available to shell-ball input window".into());
    }

    let selected_files = rfd::FileDialog::new()
        .set_title("Select files")
        .pick_files()
        .unwrap_or_default();

    Ok(selected_files
        .into_iter()
        .map(|path| path.display().to_string())
        .collect())
}

#[tauri::command]
async fn shell_ball_read_selection_snapshot(
    app: tauri::AppHandle,
) -> Result<Option<selection::SelectionSnapshotPayload>, String> {
    tauri::async_runtime::spawn_blocking(move || selection::read_selection_snapshot(&app))
        .await
        .map_err(|error| format!("selection snapshot task failed: {error}"))?
}

#[derive(Clone, Copy, Debug, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
enum DesktopOpenResourceAction {
    OpenFile,
    RevealInFolder,
}

#[derive(Clone, Debug, PartialEq, Eq)]
enum DesktopOpenOperation {
    OpenPath(PathBuf),
    RevealFile(PathBuf),
}

#[tauri::command]
fn desktop_open_resource(
    window: tauri::Window,
    action: DesktopOpenResourceAction,
    path: String,
) -> Result<(), String> {
    let resolved_path = resolve_desktop_resource_path(&path)?;
    if desktop_open_requires_host_confirmation(&resolved_path)
        && !desktop_window_has_external_open_approval(window.label())
    {
        if !prompt_desktop_external_open_approval(window.label(), &resolved_path)? {
            return Err("desktop host rejected the outside-workspace open request".to_string());
        }

        mark_desktop_window_external_open_approval(window.label());
    }

    let operation = build_desktop_open_operation(action, resolved_path);
    execute_desktop_open_operation(&operation)
}

fn resolve_desktop_resource_path(raw_path: &str) -> Result<PathBuf, String> {
    let roots = build_desktop_resource_resolution_roots();
    resolve_desktop_resource_path_with_roots(raw_path, &roots)
}

fn resolve_desktop_resource_path_with_roots(
    raw_path: &str,
    roots: &[PathBuf],
) -> Result<PathBuf, String> {
    let trimmed_path = raw_path.trim();
    if trimmed_path.is_empty() {
        return Err("resource path is empty".to_string());
    }

    let candidate_path = PathBuf::from(trimmed_path);
    if candidate_path.is_absolute() {
        return canonicalize_existing_path(candidate_path, trimmed_path);
    }

    for root in roots {
        let joined_path = root.join(&candidate_path);
        let canonical_root = match fs::canonicalize(root) {
            Ok(path) => path,
            Err(_) => continue,
        };
        let canonical_target = match fs::canonicalize(&joined_path) {
            Ok(path) => path,
            Err(_) => continue,
        };

        if canonical_target.starts_with(&canonical_root) {
            return Ok(canonical_target);
        }
    }

    Err(format!("resource path does not exist: {trimmed_path}"))
}

fn canonicalize_existing_path(path: PathBuf, original_path: &str) -> Result<PathBuf, String> {
    fs::canonicalize(path).map_err(|_| format!("resource path does not exist: {original_path}"))
}

fn desktop_open_requires_host_confirmation(target_path: &Path) -> bool {
    !is_desktop_resource_path_within_roots(
        target_path,
        &build_desktop_resource_resolution_roots(),
    )
}

fn is_desktop_resource_path_within_roots(target_path: &Path, roots: &[PathBuf]) -> bool {
    let canonical_target = match fs::canonicalize(target_path) {
        Ok(path) => path,
        Err(_) => return false,
    };

    roots.iter().any(|root| {
        fs::canonicalize(root)
            .map(|canonical_root| canonical_target.starts_with(&canonical_root))
            .unwrap_or(false)
    })
}

fn desktop_window_has_external_open_approval(window_label: &str) -> bool {
    DESKTOP_EXTERNAL_OPEN_APPROVALS
        .lock()
        .expect("desktop external approvals mutex should not be poisoned")
        .contains(window_label)
}

fn mark_desktop_window_external_open_approval(window_label: &str) {
    DESKTOP_EXTERNAL_OPEN_APPROVALS
        .lock()
        .expect("desktop external approvals mutex should not be poisoned")
        .insert(window_label.to_string());
}

fn prompt_desktop_external_open_approval(
    window_label: &str,
    target_path: &Path,
) -> Result<bool, String> {
    let prompt_result = rfd::MessageDialog::new()
        .set_title("Confirm outside-workspace open")
        .set_description(format!(
            "The dashboard window \"{window_label}\" is trying to open a path outside the trusted desktop roots:\n{}",
            target_path.display()
        ))
        .set_level(rfd::MessageLevel::Warning)
        .set_buttons(rfd::MessageButtons::YesNo)
        .show();

    Ok(matches!(
        prompt_result,
        rfd::MessageDialogResult::Yes | rfd::MessageDialogResult::Ok
    ))
}

fn build_desktop_resource_resolution_roots() -> Vec<PathBuf> {
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let mut roots = Vec::new();

    if let Ok(current_dir) = std::env::current_dir() {
        push_unique_path(&mut roots, current_dir);
    }

    push_unique_path(&mut roots, manifest_dir.clone());

    if let Some(desktop_root) = manifest_dir.parent() {
        push_unique_path(&mut roots, desktop_root.to_path_buf());
    }

    if let Some(apps_root) = manifest_dir.parent().and_then(Path::parent) {
        push_unique_path(&mut roots, apps_root.to_path_buf());
    }

    if let Some(repo_root) = manifest_dir
        .parent()
        .and_then(Path::parent)
        .and_then(Path::parent)
    {
        push_unique_path(&mut roots, repo_root.to_path_buf());
    }

    roots
}

fn push_unique_path(roots: &mut Vec<PathBuf>, candidate: PathBuf) {
    if roots.iter().any(|existing| existing == &candidate) {
        return;
    }

    roots.push(candidate);
}

fn build_desktop_open_operation(
    action: DesktopOpenResourceAction,
    target_path: PathBuf,
) -> DesktopOpenOperation {
    match action {
        DesktopOpenResourceAction::OpenFile => DesktopOpenOperation::OpenPath(target_path),
        DesktopOpenResourceAction::RevealInFolder => {
            if target_path.is_dir() {
                DesktopOpenOperation::OpenPath(target_path)
            } else {
                DesktopOpenOperation::RevealFile(target_path)
            }
        }
    }
}

fn execute_desktop_open_operation(operation: &DesktopOpenOperation) -> Result<(), String> {
    match operation {
        DesktopOpenOperation::OpenPath(target_path) => open_path_with_default_app(target_path),
        DesktopOpenOperation::RevealFile(target_path) => reveal_file_in_folder(target_path),
    }
}

#[cfg(windows)]
fn open_path_with_default_app(target_path: &Path) -> Result<(), String> {
    let operation = encode_wide_string(OsStr::new("open"));
    let target = encode_wide_string(target_path.as_os_str());

    let result = unsafe {
        ShellExecuteW(
            None,
            PCWSTR(operation.as_ptr()),
            PCWSTR(target.as_ptr()),
            PCWSTR::null(),
            PCWSTR::null(),
            SW_SHOWNORMAL,
        )
    };

    let result_code = result.0 as isize;
    if result_code <= 32 {
        return Err(format!("shell open failed with code {result_code}"));
    }

    Ok(())
}

#[cfg(not(windows))]
fn open_path_with_default_app(target_path: &Path) -> Result<(), String> {
    let command_name = if cfg!(target_os = "macos") {
        "open"
    } else {
        "xdg-open"
    };

    Command::new(command_name)
        .arg(target_path)
        .spawn()
        .map(|_| ())
        .map_err(|error| format!("failed to launch {command_name}: {error}"))
}

#[cfg(windows)]
fn reveal_file_in_folder(target_path: &Path) -> Result<(), String> {
    Command::new("explorer.exe")
        .arg(format!("/select,{}", target_path.display()))
        .spawn()
        .map(|_| ())
        .map_err(|error| format!("failed to reveal file in Explorer: {error}"))
}

#[cfg(not(windows))]
fn reveal_file_in_folder(target_path: &Path) -> Result<(), String> {
    let parent_directory = target_path
        .parent()
        .ok_or_else(|| format!("resource path has no parent directory: {}", target_path.display()))?;

    open_path_with_default_app(parent_directory)
}

#[cfg(windows)]
fn encode_wide_string(value: &OsStr) -> Vec<u16> {
    value.encode_wide().chain(std::iter::once(0)).collect()
}

fn main() {
    tauri::Builder::default()
        .manage(Arc::new(NamedPipeBridgeState::default()))
        .plugin(tauri_plugin_clipboard_manager::init())
        .setup(|app| {
            // activity::install_mouse_activity_listener()
            //     .map_err(|error| std::io::Error::other(error))?;
            install_shell_ball_clipboard_hooks(app.handle())
                .map_err(|error| std::io::Error::other(error))?;
            selection::install_selection_listener(app.handle())
                .map_err(|error| std::io::Error::other(error))?;
            window_context::install_window_context_listener(app.handle())
                .map_err(|error| std::io::Error::other(error))?;

            Ok(install_system_tray(app)?)
        })
        .invoke_handler(tauri::generate_handler![
            named_pipe_request,
            named_pipe_subscribe,
            named_pipe_unsubscribe,
            shell_ball_set_ignore_cursor_events,
            shell_ball_get_mouse_position,
            desktop_get_mouse_activity_snapshot,
            desktop_capture_screenshot,
            desktop_get_active_window_context,
            pick_shell_ball_files,
            shell_ball_read_selection_snapshot,
            desktop_open_resource
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}

#[cfg(test)]
mod tests {
    use super::{
        build_desktop_open_operation, is_desktop_resource_path_within_roots,
        resolve_desktop_resource_path_with_roots,
        DesktopOpenOperation, DesktopOpenResourceAction,
    };
    use std::fs;
    use std::path::PathBuf;
    use std::time::{SystemTime, UNIX_EPOCH};

    fn create_temp_root(name: &str) -> PathBuf {
        let unique_suffix = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("current time should be after unix epoch")
            .as_nanos();
        let root = std::env::temp_dir().join(format!("cialloclaw_desktop_{name}_{unique_suffix}"));
        fs::create_dir_all(&root).expect("temp root should be created");
        root
    }

    #[test]
    fn resolve_desktop_resource_path_with_roots_keeps_absolute_paths() {
        let root = create_temp_root("absolute");
        let target = root.join("artifact.txt");
        fs::write(&target, "artifact").expect("temp file should be created");

        let resolved = resolve_desktop_resource_path_with_roots(
            target.to_str().expect("temp path should be utf-8"),
            &[],
        )
        .expect("absolute path should resolve");

        assert_eq!(resolved, target);
        let _ = fs::remove_dir_all(root);
    }

    #[test]
    fn resolve_desktop_resource_path_with_roots_uses_repo_relative_candidates() {
        let missing_root = create_temp_root("missing_root");
        let repo_root = create_temp_root("repo_root");
        let relative_path = PathBuf::from("docs").join("dashboard-design.md");
        let target = repo_root.join(&relative_path);
        fs::create_dir_all(
            target
                .parent()
                .expect("target should have a parent directory"),
        )
        .expect("target parent should be created");
        fs::write(&target, "dashboard").expect("relative target should be created");

        let resolved = resolve_desktop_resource_path_with_roots(
            relative_path.to_str().expect("relative path should be utf-8"),
            &[missing_root.clone(), repo_root.clone()],
        )
        .expect("repo-relative path should resolve");

        assert_eq!(resolved, target);
        let _ = fs::remove_dir_all(missing_root);
        let _ = fs::remove_dir_all(repo_root);
    }

    #[test]
    fn resolve_desktop_resource_path_with_roots_rejects_relative_escape() {
        let repo_root = create_temp_root("repo_escape_root");
        let outside_root = create_temp_root("repo_escape_outside");
        let escaped_target = outside_root.join("secret.txt");
        fs::write(&escaped_target, "secret").expect("escaped target should be created");

        let relative_escape = PathBuf::from("..")
            .join(outside_root.file_name().expect("outside root should have a file name"))
            .join("secret.txt");
        let resolution = resolve_desktop_resource_path_with_roots(
            relative_escape
                .to_str()
                .expect("relative escape should be utf-8"),
            &[repo_root.clone()],
        );

        assert!(resolution.is_err(), "relative escape should be rejected");

        let _ = fs::remove_dir_all(repo_root);
        let _ = fs::remove_dir_all(outside_root);
    }

    #[test]
    fn is_desktop_resource_path_within_roots_accepts_rooted_targets_only() {
        let repo_root = create_temp_root("rooted_target");
        let repo_target = repo_root.join("docs").join("artifact.txt");
        fs::create_dir_all(repo_target.parent().expect("repo target should have a parent"))
            .expect("repo target parent should be created");
        fs::write(&repo_target, "artifact").expect("repo target should be created");

        let outside_root = create_temp_root("outside_target");
        let outside_target = outside_root.join("artifact.txt");
        fs::write(&outside_target, "artifact").expect("outside target should be created");

        assert!(is_desktop_resource_path_within_roots(
            &repo_target,
            &[repo_root.clone()]
        ));
        assert!(!is_desktop_resource_path_within_roots(
            &outside_target,
            &[repo_root.clone()]
        ));

        let _ = fs::remove_dir_all(repo_root);
        let _ = fs::remove_dir_all(outside_root);
    }

    #[test]
    fn build_desktop_open_operation_opens_directories_and_reveals_files() {
        let root = create_temp_root("reveal");
        let file_target = root.join("artifact.txt");
        fs::write(&file_target, "artifact").expect("temp file should be created");

        assert_eq!(
            build_desktop_open_operation(
                DesktopOpenResourceAction::RevealInFolder,
                root.clone(),
            ),
            DesktopOpenOperation::OpenPath(root.clone())
        );
        assert_eq!(
            build_desktop_open_operation(
                DesktopOpenResourceAction::RevealInFolder,
                file_target.clone(),
            ),
            DesktopOpenOperation::RevealFile(file_target.clone())
        );
        assert_eq!(
            build_desktop_open_operation(DesktopOpenResourceAction::OpenFile, file_target.clone()),
            DesktopOpenOperation::OpenPath(file_target.clone())
        );

        let _ = fs::remove_dir_all(root);
    }
}
