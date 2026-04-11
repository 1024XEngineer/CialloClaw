// 该入口负责启动桌面端 Tauri 宿主。
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use serde_json::Value;
use std::collections::HashMap;
use std::fs::OpenOptions;
use std::io::{BufReader, BufWriter, Write};
use std::sync::atomic::{AtomicU32, Ordering};
use std::sync::{mpsc, Arc, Mutex};
use tauri::ipc::Channel;

#[cfg(windows)]
use once_cell::sync::Lazy;

#[cfg(windows)]
use std::collections::HashSet;

#[cfg(windows)]
use windows::Win32::{
    Foundation::{HWND, LPARAM, LRESULT, POINT, RECT, WPARAM},
    Graphics::Gdi::{PtInRect, ScreenToClient},
    UI::WindowsAndMessaging::*,
};

type JsonChannel = Channel<Value>;

const NAMED_PIPE_PATH: &str = r"\\.\pipe\cialloclaw-rpc";

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

fn main() {
    tauri::Builder::default()
        .manage(Arc::new(NamedPipeBridgeState::default()))
        .invoke_handler(tauri::generate_handler![
            named_pipe_request,
            named_pipe_subscribe,
            named_pipe_unsubscribe,
            shell_ball_set_ignore_cursor_events,
            shell_ball_get_mouse_position
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
