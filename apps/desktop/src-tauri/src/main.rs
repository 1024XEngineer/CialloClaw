// This entry point boots the desktop Tauri host process.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod activity;
mod internal_windows;
mod local_path;
mod runtime_paths;
mod screen_capture;
mod selection;
mod source_notes;
mod window_context;

use serde::Serialize;
use serde_json::Value;
use std::collections::HashMap;
use std::fs::OpenOptions;
use std::io::{BufReader, BufWriter, Write};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicU32, Ordering};
use std::sync::{mpsc, Arc, Mutex, OnceLock};
use std::thread;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};
use tauri::ipc::Channel;
use tauri::menu::{MenuBuilder, MenuItemBuilder};
use tauri::tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent};
use tauri::{Emitter, Manager, RunEvent, WebviewUrl, WebviewWindowBuilder};
use tauri_plugin_shell::process::{CommandChild, CommandEvent};
use tauri_plugin_shell::ShellExt;

#[cfg(windows)]
use once_cell::sync::Lazy;

#[cfg(windows)]
use std::collections::HashSet;

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
    UI::Input::KeyboardAndMouse::{GetAsyncKeyState, VK_CONTROL, VK_DELETE, VK_SHIFT},
    UI::WindowsAndMessaging::*,
};

type JsonChannel = Channel<Value>;

const DEFAULT_NAMED_PIPE_PATH: &str = r"\\.\pipe\cialloclaw-rpc";
#[derive(Clone, Serialize)]
struct DesktopRuntimeDefaultsPayload {
    data_path: String,
    workspace_path: String,
    task_sources: Vec<String>,
}

fn format_runtime_defaults_path(path: &std::path::Path) -> String {
    let normalized = path.to_string_lossy().replace('\\', "/");

    if let Some(stripped) = normalized.strip_prefix("//?/UNC/") {
        return format!("//{stripped}");
    }

    if let Some(stripped) = normalized.strip_prefix("//?/") {
        return stripped.to_string();
    }

    normalized
}

const CONTROL_PANEL_WINDOW_LABEL: &str = "control-panel";
const DASHBOARD_WINDOW_LABEL: &str = "dashboard";
const ONBOARDING_WINDOW_LABEL: &str = "onboarding";
const SHELL_BALL_WINDOW_LABEL: &str = "shell-ball";
const SHELL_BALL_PINNED_WINDOW_PREFIX: &str = "shell-ball-bubble-pinned-";
const SHELL_BALL_DASHBOARD_TRANSITION_REQUEST_EVENT: &str =
    "desktop-shell-ball:dashboard-transition-request";
const SHELL_BALL_CLIPBOARD_SNAPSHOT_EVENT: &str = "desktop-shell-ball:clipboard-snapshot";
const TRAY_ICON_ID: &str = "main-tray";
const TRAY_MENU_SHOW_SHELL_BALL_ID: &str = "show-shell-ball";
const TRAY_MENU_HIDE_SHELL_BALL_ID: &str = "hide-shell-ball";
const TRAY_MENU_OPEN_CONTROL_PANEL_ID: &str = "open-control-panel";
const TRAY_MENU_QUIT_ID: &str = "quit-app";
const DESKTOP_SETTINGS_CLIENT_TIME: &str = "1970-01-01T00:00:00Z";
const LOCAL_SERVICE_LOG_FILE_NAME: &str = "local-service-sidecar.log";
const LOCAL_SERVICE_READY_TIMEOUT: Duration = Duration::from_secs(20);
const LOCAL_SERVICE_READY_RETRY_DELAY: Duration = Duration::from_millis(100);
const LOCAL_SERVICE_REQUEST_TIMEOUT: Duration = Duration::from_secs(330);
const LOCAL_SERVICE_PIPE_BUSY_RETRY_TIMEOUT: Duration = Duration::from_secs(2);
const LOCAL_SERVICE_PIPE_BUSY_RETRY_DELAY: Duration = Duration::from_millis(40);
const LOCAL_SERVICE_LOG_MAX_BYTES: u64 = 256 * 1024;
static LOCAL_SERVICE_LOG_WRITE_LOCK: OnceLock<Mutex<()>> = OnceLock::new();
static DESKTOP_SETTINGS_REQUEST_ID: AtomicU32 = AtomicU32::new(1);
static CONTROL_PANEL_WINDOW_CREATION_IN_PROGRESS: AtomicBool = AtomicBool::new(false);
static ONBOARDING_WINDOW_CREATION_IN_PROGRESS: AtomicBool = AtomicBool::new(false);
const DESKTOP_SETTINGS_REQUEST_TIMEOUT_MS: u64 = 1_500;

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
    pipe_name: Mutex<String>,
    session: Mutex<Option<BridgeSession>>,
    pending: Mutex<HashMap<String, mpsc::Sender<Result<Value, String>>>>,
    subscriptions: Mutex<HashMap<String, HashMap<u32, JsonChannel>>>,
    next_subscription_id: AtomicU32,
    isolated_transport_degraded: AtomicBool,
}

impl Default for NamedPipeBridgeState {
    fn default() -> Self {
        Self {
            pipe_name: Mutex::new(DEFAULT_NAMED_PIPE_PATH.to_string()),
            session: Mutex::new(None),
            pending: Mutex::new(HashMap::new()),
            subscriptions: Mutex::new(HashMap::new()),
            next_subscription_id: AtomicU32::new(1),
            isolated_transport_degraded: AtomicBool::new(false),
        }
    }
}

/// DesktopSettingsSnapshotState keeps the latest formal settings payload inside
/// the desktop host so platform bridges can reuse one startup fetch instead of
/// re-requesting settings on every local action.
struct DesktopSettingsSnapshotState {
    settings: Mutex<Option<Value>>,
}

impl Default for DesktopSettingsSnapshotState {
    fn default() -> Self {
        Self {
            settings: Mutex::new(None),
        }
    }
}

impl DesktopSettingsSnapshotState {
    fn seed(&self, settings: Value) -> Result<(), String> {
        validate_desktop_settings_snapshot(&settings)?;
        let mut snapshot = self
            .settings
            .lock()
            .map_err(|_| "desktop settings snapshot lock poisoned".to_string())?;
        if snapshot.is_none() {
            *snapshot = Some(settings);
        }
        Ok(())
    }

    fn replace(&self, settings: Value) -> Result<(), String> {
        validate_desktop_settings_snapshot(&settings)?;
        let mut snapshot = self
            .settings
            .lock()
            .map_err(|_| "desktop settings snapshot lock poisoned".to_string())?;
        *snapshot = Some(settings);
        Ok(())
    }

    #[cfg(test)]
    fn workspace_root(&self) -> Result<Option<PathBuf>, String> {
        let snapshot = self
            .settings
            .lock()
            .map_err(|_| "desktop settings snapshot lock poisoned".to_string())?;

        Ok(snapshot
            .as_ref()
            .and_then(read_workspace_root_from_settings_snapshot))
    }

    fn task_sources(&self) -> Result<Option<Vec<String>>, String> {
        let snapshot = self
            .settings
            .lock()
            .map_err(|_| "desktop settings snapshot lock poisoned".to_string())?;

        Ok(snapshot
            .as_ref()
            .map(read_task_sources_from_settings_snapshot))
    }
}

impl NamedPipeBridgeState {
    /// configure_pipe_name updates the runtime pipe path before the renderer
    /// issues any requests, allowing packaged builds to avoid global pipe name
    /// collisions with other desktop sessions.
    fn configure_pipe_name(&self, pipe_name: String) -> Result<(), String> {
        let mut configured_pipe_name = self
            .pipe_name
            .lock()
            .map_err(|_| "named pipe name lock poisoned".to_string())?;
        *configured_pipe_name = pipe_name;

        let mut session = self
            .session
            .lock()
            .map_err(|_| "named pipe session lock poisoned".to_string())?;
        *session = None;
        self.clear_isolated_transport_degraded();

        Ok(())
    }

    fn pipe_name(&self) -> Result<String, String> {
        self.pipe_name
            .lock()
            .map(|pipe_name| pipe_name.clone())
            .map_err(|_| "named pipe name lock poisoned".to_string())
    }

    fn request(self: &Arc<Self>, payload: Value) -> Result<Value, String> {
        self.request_internal(payload, Some(LOCAL_SERVICE_REQUEST_TIMEOUT))
    }

    fn request_with_timeout(
        self: &Arc<Self>,
        payload: Value,
        timeout: Duration,
    ) -> Result<Value, String> {
        self.request_internal(payload, Some(timeout))
    }

    fn request_internal(
        self: &Arc<Self>,
        payload: Value,
        timeout: Option<Duration>,
    ) -> Result<Value, String> {
        if self.should_use_isolated_transport(&payload) {
            return self.request_via_isolated_connection(payload, timeout);
        }

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

        match timeout {
            Some(timeout) => response_rx.recv_timeout(timeout).map_err(|error| {
                if let Ok(mut pending) = self.pending.lock() {
                    pending.remove(&request_id);
                }

                match error {
                    mpsc::RecvTimeoutError::Timeout => {
                        format!(
                            "named pipe response wait timed out after {}ms",
                            timeout.as_millis()
                        )
                    }
                    mpsc::RecvTimeoutError::Disconnected => {
                        "named pipe response wait failed: channel disconnected".to_string()
                    }
                }
            })?,
            None => response_rx.recv().map_err(|error| {
                if let Ok(mut pending) = self.pending.lock() {
                    pending.remove(&request_id);
                }

                format!("named pipe response wait failed: {error}")
            })?,
        }
    }

    /// request_via_isolated_connection keeps short control-panel style reads off
    /// the shared streaming session so a long-running shell-ball task cannot
    /// delay unrelated settings fetches behind the same pipe connection.
    /// Notifications observed on the isolated request are still forwarded into
    /// the shared desktop subscription fan-out so dashboard and security surfaces
    /// stay in sync when a shell-ball submit uses the isolated path.
    fn request_via_isolated_connection(
        self: &Arc<Self>,
        payload: Value,
        timeout: Option<Duration>,
    ) -> Result<Value, String> {
        let pipe_name = self.pipe_name()?;
        let request_id = extract_request_id(&payload)?;
        let (response_tx, response_rx) = mpsc::channel();

        let state = Arc::clone(self);
        std::thread::spawn(move || {
            let result = send_isolated_named_pipe_request(&state, &pipe_name, &request_id, payload);
            let _ = response_tx.send(result);
        });

        match timeout {
            Some(timeout) => response_rx
                .recv_timeout(timeout)
                .map_err(|error| match error {
                    mpsc::RecvTimeoutError::Timeout => {
                        self.mark_isolated_transport_degraded();
                        format!(
                            "isolated named pipe response wait timed out after {}ms",
                            timeout.as_millis()
                        )
                    }
                    mpsc::RecvTimeoutError::Disconnected => {
                        "isolated named pipe response wait failed: channel disconnected".to_string()
                    }
                })?,
            None => response_rx
                .recv()
                .map_err(|error| format!("isolated named pipe response wait failed: {error}"))?,
        }
    }

    fn should_use_isolated_transport(&self, payload: &Value) -> bool {
        should_use_isolated_named_pipe_payload(payload) && !self.isolated_transport_degraded()
    }

    fn isolated_transport_degraded(&self) -> bool {
        self.isolated_transport_degraded.load(Ordering::Relaxed)
    }

    fn mark_isolated_transport_degraded(&self) {
        self.isolated_transport_degraded
            .store(true, Ordering::Relaxed);
    }

    fn clear_isolated_transport_degraded(&self) {
        self.isolated_transport_degraded
            .store(false, Ordering::Relaxed);
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

        let pipe_name = self.pipe_name()?;

        let stream = open_named_pipe_with_retry(&pipe_name, "shared")?;

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
        self.clear_isolated_transport_degraded();
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

/// LocalServiceSidecarState retains the spawned Go service child so the desktop
/// host can keep it alive during runtime and terminate it during shutdown.
struct LocalServiceSidecarState {
    child: Mutex<Option<CommandChild>>,
    last_failure: Mutex<Option<String>>,
}

impl Default for LocalServiceSidecarState {
    fn default() -> Self {
        Self {
            child: Mutex::new(None),
            last_failure: Mutex::new(None),
        }
    }
}

impl LocalServiceSidecarState {
    fn is_running(&self) -> Result<bool, String> {
        self.child
            .lock()
            .map(|child| child.is_some())
            .map_err(|_| "local service sidecar lock poisoned".to_string())
    }

    fn store(&self, child: CommandChild) -> Result<(), String> {
        let mut guard = self
            .child
            .lock()
            .map_err(|_| "local service sidecar lock poisoned".to_string())?;
        *guard = Some(child);
        self.clear_last_failure();
        Ok(())
    }

    fn clear(&self) {
        if let Ok(mut guard) = self.child.lock() {
            *guard = None;
        }
    }

    fn stop(&self) -> Result<(), String> {
        let mut guard = self
            .child
            .lock()
            .map_err(|_| "local service sidecar lock poisoned".to_string())?;

        if let Some(child) = guard.take() {
            child
                .kill()
                .map_err(|error| format!("failed to stop local service sidecar: {error}"))?;
        }

        Ok(())
    }

    fn record_failure(&self, failure: String) {
        if let Ok(mut guard) = self.last_failure.lock() {
            *guard = Some(failure);
        }
    }

    fn clear_last_failure(&self) {
        if let Ok(mut guard) = self.last_failure.lock() {
            *guard = None;
        }
    }

    fn last_failure(&self) -> Option<String> {
        self.last_failure.lock().ok().and_then(|guard| guard.clone())
    }
}

/// resolve_required_path normalizes a host-side filesystem path before it is
/// forwarded to the bundled Go service as a CLI argument.
fn resolve_required_path(path: PathBuf, label: &str) -> Result<String, String> {
    let resolved = path.to_string_lossy().trim().to_string();
    if resolved.is_empty() {
        return Err(format!("failed to resolve {label}"));
    }
    Ok(resolved)
}

/// derive_local_service_pipe_name uses the per-user application data directory
/// as a stable input so packaged desktop builds do not collide with other local
/// CialloClaw sessions that may already own the default global pipe name.
fn derive_local_service_pipe_name(app_data_dir: &str) -> String {
    let mut hash: u64 = 0xcbf29ce484222325;
    for byte in app_data_dir.bytes() {
        hash ^= byte.to_ascii_lowercase() as u64;
        hash = hash.wrapping_mul(0x100000001b3);
    }

    format!(r"\\.\pipe\cialloclaw-rpc-{hash:016x}")
}

/// configure_sidecar_named_pipe keeps the bridge on the default pipe unless the
/// bundled sidecar actually started. Dev sessions may rely on a manually started
/// local-service that still listens on the default global pipe name.
fn configure_sidecar_named_pipe(
    bridge_state: &NamedPipeBridgeState,
    named_pipe_path: &str,
    sidecar_started: bool,
) -> Result<bool, String> {
    if !sidecar_started {
        return Ok(false);
    }

    bridge_state.configure_pipe_name(named_pipe_path.to_string())?;
    Ok(true)
}

/// append_local_service_log writes lightweight startup diagnostics into the
/// user data directory so packaged failures can be investigated without a console.
fn append_local_service_log(app: &tauri::AppHandle, message: &str) {
    let Ok(app_data_dir) = app.path().app_data_dir() else {
        return;
    };

    let log_dir = app_data_dir.join("logs");
    if std::fs::create_dir_all(&log_dir).is_err() {
        return;
    }

    let lock = LOCAL_SERVICE_LOG_WRITE_LOCK.get_or_init(|| Mutex::new(()));
    let Ok(_guard) = lock.lock() else {
        return;
    };

    let log_path = log_dir.join(LOCAL_SERVICE_LOG_FILE_NAME);
    if let Ok(metadata) = std::fs::metadata(&log_path) {
        if metadata.len() > LOCAL_SERVICE_LOG_MAX_BYTES {
            let _ = std::fs::remove_file(&log_path);
        }
    }

    let Ok(mut file) = OpenOptions::new().create(true).append(true).open(log_path) else {
        return;
    };

    let timestamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_secs())
        .unwrap_or_default();
    let sanitized_message = if message.len() > 1_024 {
        format!("{}…", &message[..1_024])
    } else {
        message.to_string()
    };
    let _ = writeln!(file, "[{timestamp}] {sanitized_message}");
}

/// start_local_service_sidecar boots the bundled Go service before the desktop
/// renderer starts issuing JSON-RPC requests against the named pipe bridge.
fn start_local_service_sidecar(
    app: &tauri::AppHandle,
    sidecar_state: &Arc<LocalServiceSidecarState>,
    named_pipe_path: &str,
) -> Result<bool, String> {
    if sidecar_state.is_running()? {
        append_local_service_log(app, "local service sidecar is already running");
        return Ok(true);
    }

    let app_data_dir = app
        .path()
        .app_data_dir()
        .map_err(|error| format!("failed to resolve app data dir: {error}"))?;
    std::fs::create_dir_all(&app_data_dir)
        .map_err(|error| format!("failed to create app data dir: {error}"))?;
    let app_data_dir_arg = resolve_required_path(app_data_dir, "app data dir")?;
    append_local_service_log(
        app,
        &format!(
            "starting local service sidecar with data_dir={app_data_dir_arg} named_pipe={named_pipe_path}"
        ),
    );

    let sidecar_command = match app.shell().sidecar("cialloclaw-service") {
        Ok(command) => command,
        Err(error) => {
            if cfg!(debug_assertions) {
                eprintln!("local service sidecar is unavailable in this debug session: {error}");
                append_local_service_log(
                    app,
                    &format!("local service sidecar unavailable in debug session: {error}"),
                );
                return Ok(false);
            }

            return Err(format!("failed to resolve local service sidecar: {error}"));
        }
    };

    let (mut rx, child) = sidecar_command
        .args([
            "--data-dir",
            app_data_dir_arg.as_str(),
            "--named-pipe",
            named_pipe_path,
            "--debug-http",
            "127.0.0.1:0",
        ])
        .spawn()
        .map_err(|error| format!("failed to start local service sidecar: {error}"))?;
    sidecar_state.store(child)?;

    let sidecar_app_handle = app.clone();
    let tracked_state = Arc::clone(sidecar_state);
    tauri::async_runtime::spawn(async move {
        // Drain sidecar events so stdout/stderr cannot block the child, while
        // also preserving the last packaged-runtime failure for startup errors.
        while let Some(event) = rx.recv().await {
            match event {
                CommandEvent::Stdout(line) => {
                    let line = String::from_utf8_lossy(&line).trim().to_string();
                    if !line.is_empty() {
                        append_local_service_log(
                            &sidecar_app_handle,
                            &format!("local service sidecar stdout: {line}"),
                        );
                    }
                }
                CommandEvent::Stderr(line) => {
                    let line = String::from_utf8_lossy(&line).trim().to_string();
                    if !line.is_empty() {
                        tracked_state.record_failure(format!("stderr: {line}"));
                        append_local_service_log(
                            &sidecar_app_handle,
                            &format!("local service sidecar stderr: {line}"),
                        );
                    }
                }
                CommandEvent::Error(error) => {
                    tracked_state.record_failure(format!("event error: {error}"));
                    append_local_service_log(
                        &sidecar_app_handle,
                        &format!("local service sidecar error: {error}"),
                    );
                }
                CommandEvent::Terminated(payload) => {
                    let failure = format!("terminated: {payload:?}");
                    tracked_state.record_failure(failure.clone());
                    append_local_service_log(
                        &sidecar_app_handle,
                        &format!("local service sidecar {failure}"),
                    );
                    tracked_state.clear();
                    break;
                }
                _ => {}
            }
        }
    });

    Ok(true)
}

/// wait_for_local_service_ready blocks the renderer bootstrap until the named
/// pipe becomes reachable, avoiding first-load request races in packaged builds.
fn wait_for_local_service_ready(
    app: &tauri::AppHandle,
    bridge_state: &Arc<NamedPipeBridgeState>,
) -> Result<(), String> {
    let start = Instant::now();
    let mut last_error = String::from("local service did not become ready");

    while start.elapsed() < LOCAL_SERVICE_READY_TIMEOUT {
        match bridge_state.ensure_session() {
            Ok(_) => {
                append_local_service_log(app, "local service named pipe became ready");
                return Ok(());
            }
            Err(error) => {
                last_error = error;
                thread::sleep(LOCAL_SERVICE_READY_RETRY_DELAY);
            }
        }
    }

    append_local_service_log(
        app,
        &format!("timed out waiting for local service readiness: {last_error}"),
    );
    Err(format!(
        "timed out waiting for local service readiness: {last_error}"
    ))
}

/// ensure_local_service_ready keeps the packaged Go sidecar aligned with the
/// configured named-pipe bridge before any desktop command issues formal RPC
/// traffic. The bridge already carries the currently selected pipe name, so
/// lazy request-time restarts reuse the same scoped transport as startup.
fn ensure_local_service_ready(
    app: &tauri::AppHandle,
    sidecar_state: &Arc<LocalServiceSidecarState>,
    bridge_state: &Arc<NamedPipeBridgeState>,
) -> Result<(), String> {
    let named_pipe_path = bridge_state.pipe_name()?;
    let sidecar_started = start_local_service_sidecar(app, sidecar_state, &named_pipe_path)?;
    if !sidecar_started {
        return Ok(());
    }

    wait_for_local_service_ready(app, bridge_state).map_err(|error| {
        if let Some(last_failure) = sidecar_state.last_failure() {
            format!("{error}; last sidecar failure: {last_failure}")
        } else {
            error
        }
    })
}

#[tauri::command]
async fn named_pipe_request(
    app: tauri::AppHandle,
    sidecar_state: tauri::State<'_, Arc<LocalServiceSidecarState>>,
    state: tauri::State<'_, Arc<NamedPipeBridgeState>>,
    payload: Value,
) -> Result<Value, String> {
    let request_method = extract_request_method(&payload).unwrap_or_else(|_| "unknown".to_string());
    let request_id = extract_request_id(&payload).unwrap_or_else(|_| "unknown".to_string());
    let request_mode = if should_use_isolated_named_pipe_payload(&payload) {
        "isolated"
    } else {
        "shared"
    };
    let started_at = Instant::now();
    append_local_service_log(
        &app,
        &format!(
            "named pipe request started: method={request_method} id={request_id} mode={request_mode}"
        ),
    );
    ensure_local_service_ready(&app, sidecar_state.inner(), state.inner())?;
    let state = Arc::clone(state.inner());
    let result = tauri::async_runtime::spawn_blocking(move || state.request(payload))
        .await
        .map_err(|error| format!("named pipe bridge task failed: {error}"))?;

    match &result {
        Ok(_) => append_local_service_log(
            &app,
            &format!(
                "named pipe request finished: method={request_method} id={request_id} mode={request_mode} duration_ms={}",
                started_at.elapsed().as_millis()
            ),
        ),
        Err(error) => append_local_service_log(
            &app,
            &format!(
                "named pipe request failed: method={request_method} id={request_id} mode={request_mode} duration_ms={} error={error}",
                started_at.elapsed().as_millis()
            ),
        ),
    }

    result
}

#[tauri::command]
async fn named_pipe_subscribe(
    app: tauri::AppHandle,
    sidecar_state: tauri::State<'_, Arc<LocalServiceSidecarState>>,
    state: tauri::State<'_, Arc<NamedPipeBridgeState>>,
    topic: String,
    on_event: JsonChannel,
) -> Result<u32, String> {
    ensure_local_service_ready(&app, sidecar_state.inner(), state.inner())?;
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
async fn desktop_capture_screenshot(
    runtime_paths_state: tauri::State<'_, Arc<runtime_paths::DesktopRuntimePaths>>,
) -> Result<screen_capture::ScreenCapturePayload, String> {
    let runtime_paths_state = Arc::clone(runtime_paths_state.inner());
    tauri::async_runtime::spawn_blocking(move || {
        screen_capture::capture_screenshot(runtime_paths_state.temp_dir())
    })
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

#[tauri::command]
async fn desktop_open_local_path(
    bridge_state: tauri::State<'_, Arc<NamedPipeBridgeState>>,
    settings_snapshot_state: tauri::State<'_, Arc<DesktopSettingsSnapshotState>>,
    runtime_paths_state: tauri::State<'_, Arc<runtime_paths::DesktopRuntimePaths>>,
    path: String,
) -> Result<(), String> {
    let bridge_state = Arc::clone(bridge_state.inner());
    let settings_snapshot_state = Arc::clone(settings_snapshot_state.inner());
    let runtime_paths_state = Arc::clone(runtime_paths_state.inner());
    tauri::async_runtime::spawn_blocking(move || {
        let roots = build_local_path_roots(
            &bridge_state,
            &settings_snapshot_state,
            &runtime_paths_state,
        );
        local_path::open_local_path(&path, &roots)
    })
    .await
    .map_err(|error| format!("desktop local open task failed: {error}"))?
}

#[tauri::command]
async fn desktop_open_external_url(url: String) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || local_path::open_external_url(&url))
        .await
        .map_err(|error| format!("desktop external url open task failed: {error}"))?
}

#[tauri::command]
async fn desktop_reveal_local_path(
    bridge_state: tauri::State<'_, Arc<NamedPipeBridgeState>>,
    settings_snapshot_state: tauri::State<'_, Arc<DesktopSettingsSnapshotState>>,
    runtime_paths_state: tauri::State<'_, Arc<runtime_paths::DesktopRuntimePaths>>,
    path: String,
) -> Result<(), String> {
    let bridge_state = Arc::clone(bridge_state.inner());
    let settings_snapshot_state = Arc::clone(settings_snapshot_state.inner());
    let runtime_paths_state = Arc::clone(runtime_paths_state.inner());
    tauri::async_runtime::spawn_blocking(move || {
        let roots = build_local_path_roots(
            &bridge_state,
            &settings_snapshot_state,
            &runtime_paths_state,
        );
        local_path::reveal_local_path(&path, &roots)
    })
    .await
    .map_err(|error| format!("desktop local reveal task failed: {error}"))?
}

#[tauri::command]
async fn desktop_open_runtime_data_path(
    runtime_paths_state: tauri::State<'_, Arc<runtime_paths::DesktopRuntimePaths>>,
) -> Result<(), String> {
    let runtime_paths_state = Arc::clone(runtime_paths_state.inner());
    tauri::async_runtime::spawn_blocking(move || {
        local_path::open_trusted_directory(
            runtime_paths_state.data_dir().as_path(),
            runtime_paths_state.runtime_root().as_path(),
        )
    })
    .await
    .map_err(|error| format!("desktop runtime data open task failed: {error}"))?
}

#[tauri::command]
async fn desktop_open_runtime_workspace_path(
    runtime_paths_state: tauri::State<'_, Arc<runtime_paths::DesktopRuntimePaths>>,
) -> Result<(), String> {
    let runtime_paths_state = Arc::clone(runtime_paths_state.inner());
    tauri::async_runtime::spawn_blocking(move || {
        local_path::open_trusted_directory(
            runtime_paths_state.workspace_root().as_path(),
            runtime_paths_state.runtime_root().as_path(),
        )
    })
    .await
    .map_err(|error| format!("desktop runtime workspace open task failed: {error}"))?
}

#[tauri::command]
async fn desktop_load_source_notes(
    bridge_state: tauri::State<'_, Arc<NamedPipeBridgeState>>,
    settings_snapshot_state: tauri::State<'_, Arc<DesktopSettingsSnapshotState>>,
    runtime_paths_state: tauri::State<'_, Arc<runtime_paths::DesktopRuntimePaths>>,
    sources: Vec<String>,
) -> Result<source_notes::DesktopSourceNoteSnapshot, String> {
    let bridge_state = Arc::clone(bridge_state.inner());
    let settings_snapshot_state = Arc::clone(settings_snapshot_state.inner());
    let runtime_paths_state = Arc::clone(runtime_paths_state.inner());
    tauri::async_runtime::spawn_blocking(move || {
        let trusted_sources =
            resolve_trusted_source_note_sources(&bridge_state, &settings_snapshot_state, &sources)?;
        let roots = build_source_note_roots(
            &bridge_state,
            &settings_snapshot_state,
            &runtime_paths_state,
            &trusted_sources,
        );
        source_notes::load_source_notes(&trusted_sources, &roots)
    })
    .await
    .map_err(|error| format!("desktop source notes load task failed: {error}"))?
}

#[tauri::command]
async fn desktop_load_source_note_index(
    bridge_state: tauri::State<'_, Arc<NamedPipeBridgeState>>,
    settings_snapshot_state: tauri::State<'_, Arc<DesktopSettingsSnapshotState>>,
    runtime_paths_state: tauri::State<'_, Arc<runtime_paths::DesktopRuntimePaths>>,
    sources: Vec<String>,
) -> Result<source_notes::DesktopSourceNoteIndexSnapshot, String> {
    let bridge_state = Arc::clone(bridge_state.inner());
    let settings_snapshot_state = Arc::clone(settings_snapshot_state.inner());
    let runtime_paths_state = Arc::clone(runtime_paths_state.inner());
    tauri::async_runtime::spawn_blocking(move || {
        let trusted_sources =
            resolve_trusted_source_note_sources(&bridge_state, &settings_snapshot_state, &sources)?;
        let roots = build_source_note_roots(
            &bridge_state,
            &settings_snapshot_state,
            &runtime_paths_state,
            &trusted_sources,
        );
        source_notes::load_source_note_index(&trusted_sources, &roots)
    })
    .await
    .map_err(|error| format!("desktop source note index load task failed: {error}"))?
}

#[tauri::command]
async fn desktop_create_source_note(
    bridge_state: tauri::State<'_, Arc<NamedPipeBridgeState>>,
    settings_snapshot_state: tauri::State<'_, Arc<DesktopSettingsSnapshotState>>,
    runtime_paths_state: tauri::State<'_, Arc<runtime_paths::DesktopRuntimePaths>>,
    sources: Vec<String>,
    content: String,
) -> Result<source_notes::DesktopSourceNoteDocument, String> {
    let bridge_state = Arc::clone(bridge_state.inner());
    let settings_snapshot_state = Arc::clone(settings_snapshot_state.inner());
    let runtime_paths_state = Arc::clone(runtime_paths_state.inner());
    tauri::async_runtime::spawn_blocking(move || {
        let trusted_sources =
            resolve_trusted_source_note_sources(&bridge_state, &settings_snapshot_state, &sources)?;
        let roots = build_source_note_roots(
            &bridge_state,
            &settings_snapshot_state,
            &runtime_paths_state,
            &trusted_sources,
        );
        source_notes::create_source_note(&trusted_sources, &roots, &content)
    })
    .await
    .map_err(|error| format!("desktop source note create task failed: {error}"))?
}

#[tauri::command]
async fn desktop_save_source_note(
    bridge_state: tauri::State<'_, Arc<NamedPipeBridgeState>>,
    settings_snapshot_state: tauri::State<'_, Arc<DesktopSettingsSnapshotState>>,
    runtime_paths_state: tauri::State<'_, Arc<runtime_paths::DesktopRuntimePaths>>,
    sources: Vec<String>,
    path: String,
    content: String,
) -> Result<source_notes::DesktopSourceNoteDocument, String> {
    let bridge_state = Arc::clone(bridge_state.inner());
    let settings_snapshot_state = Arc::clone(settings_snapshot_state.inner());
    let runtime_paths_state = Arc::clone(runtime_paths_state.inner());
    tauri::async_runtime::spawn_blocking(move || {
        let trusted_sources =
            resolve_trusted_source_note_sources(&bridge_state, &settings_snapshot_state, &sources)?;
        let roots = build_source_note_roots(
            &bridge_state,
            &settings_snapshot_state,
            &runtime_paths_state,
            &trusted_sources,
        );
        source_notes::save_source_note(&trusted_sources, &roots, &path, &content)
    })
    .await
    .map_err(|error| format!("desktop source note save task failed: {error}"))?
}

#[tauri::command]
fn desktop_sync_settings_snapshot(
    state: tauri::State<'_, Arc<DesktopSettingsSnapshotState>>,
    settings: Value,
) -> Result<(), String> {
    state.replace(settings)
}

#[tauri::command]
fn desktop_get_runtime_defaults(
    runtime_paths_state: tauri::State<'_, Arc<runtime_paths::DesktopRuntimePaths>>,
) -> DesktopRuntimeDefaultsPayload {
    let task_source = runtime_paths_state.task_source_root();

    DesktopRuntimeDefaultsPayload {
        data_path: format_runtime_defaults_path(runtime_paths_state.data_dir().as_path()),
        workspace_path: format_runtime_defaults_path(runtime_paths_state.workspace_root().as_path()),
        task_sources: vec![format_runtime_defaults_path(task_source.as_path())],
    }
}

fn build_local_path_roots(
    _bridge_state: &Arc<NamedPipeBridgeState>,
    _settings_snapshot_state: &Arc<DesktopSettingsSnapshotState>,
    runtime_paths_state: &Arc<runtime_paths::DesktopRuntimePaths>,
) -> local_path::LocalPathRoots {
    // Workspace delivery paths must stay pinned to the bootstrap runtime until
    // local-service restarts and actually rebinds the backend workspace.
    let workspace_root = Some(runtime_paths_state.workspace_root().clone());

    local_path::LocalPathRoots::new(
        workspace_root,
        Some(runtime_paths_state.runtime_root().clone()),
        Some(runtime_paths_state.local_open_runtime_root()),
    )
}

fn build_source_note_roots(
    _bridge_state: &Arc<NamedPipeBridgeState>,
    _settings_snapshot_state: &Arc<DesktopSettingsSnapshotState>,
    runtime_paths_state: &Arc<runtime_paths::DesktopRuntimePaths>,
    sources: &[String],
) -> local_path::LocalPathRoots {
    let workspace_root = if source_notes::sources_require_workspace_root(sources) {
        // Source-note access to workspace-relative sources must match the
        // currently running backend workspace instead of a restart-pending draft.
        Some(runtime_paths_state.workspace_root().clone())
    } else {
        None
    };

    local_path::LocalPathRoots::new(
        workspace_root,
        Some(runtime_paths_state.runtime_root().clone()),
        None,
    )
}

/// Source-note file access must be scoped by the host-side settings snapshot
/// instead of any renderer-provided allowlist. Renderer `sources` are kept only
/// for request compatibility and drift diagnostics.
fn resolve_trusted_source_note_sources(
    bridge_state: &Arc<NamedPipeBridgeState>,
    settings_snapshot_state: &Arc<DesktopSettingsSnapshotState>,
    renderer_sources: &[String],
) -> Result<Vec<String>, String> {
    let cached_task_sources = read_trusted_source_note_sources(settings_snapshot_state)?;
    if let Some(task_sources) = cached_task_sources {
        if !source_note_sources_drift(renderer_sources, &task_sources) {
            return Ok(task_sources);
        }

        replace_desktop_settings_snapshot(
            bridge_state,
            settings_snapshot_state,
            Duration::from_millis(DESKTOP_SETTINGS_REQUEST_TIMEOUT_MS),
        )?;

        let refreshed_task_sources = read_trusted_source_note_sources(settings_snapshot_state)?
            .ok_or_else(|| {
                "desktop settings snapshot is unavailable for trusted source note access"
                    .to_string()
            })?;

        report_source_note_source_drift(renderer_sources, &refreshed_task_sources);
        return Ok(refreshed_task_sources);
    }

    seed_desktop_settings_snapshot(
        bridge_state,
        settings_snapshot_state,
        Duration::from_millis(DESKTOP_SETTINGS_REQUEST_TIMEOUT_MS),
    )?;

    let task_sources =
        read_trusted_source_note_sources(settings_snapshot_state)?.ok_or_else(|| {
            "desktop settings snapshot is unavailable for trusted source note access".to_string()
        })?;
    report_source_note_source_drift(renderer_sources, &task_sources);
    Ok(task_sources)
}

#[cfg(test)]
fn resolve_workspace_root_from_snapshot(
    settings_snapshot_state: &Arc<DesktopSettingsSnapshotState>,
    runtime_paths_state: &Arc<runtime_paths::DesktopRuntimePaths>,
) -> Option<PathBuf> {
    settings_snapshot_state
        .workspace_root()
        .ok()
        .flatten()
        .map(|workspace_root| runtime_paths_state.resolve_workspace_setting(&workspace_root))
}

fn seed_desktop_settings_snapshot(
    bridge_state: &Arc<NamedPipeBridgeState>,
    settings_snapshot_state: &Arc<DesktopSettingsSnapshotState>,
    timeout: Duration,
) -> Result<(), String> {
    refresh_desktop_settings_snapshot(bridge_state, settings_snapshot_state, timeout, false)
}

fn replace_desktop_settings_snapshot(
    bridge_state: &Arc<NamedPipeBridgeState>,
    settings_snapshot_state: &Arc<DesktopSettingsSnapshotState>,
    timeout: Duration,
) -> Result<(), String> {
    refresh_desktop_settings_snapshot(bridge_state, settings_snapshot_state, timeout, true)
}

/// Host-side settings refresh supports both "seed if empty" and "replace with
/// the latest formal snapshot" modes so startup prefetch will not clobber newer
/// renderer syncs while bounded drift recovery can still replace stale caches.
fn refresh_desktop_settings_snapshot(
    bridge_state: &Arc<NamedPipeBridgeState>,
    settings_snapshot_state: &Arc<DesktopSettingsSnapshotState>,
    timeout: Duration,
    replace_existing: bool,
) -> Result<(), String> {
    let settings = fetch_settings_snapshot_with(bridge_state, |state, payload| {
        state.request_with_timeout(payload, timeout)
    })?;

    if replace_existing {
        settings_snapshot_state.replace(settings)
    } else {
        settings_snapshot_state.seed(settings)
    }
}

fn fetch_settings_snapshot_with<F>(
    state: &Arc<NamedPipeBridgeState>,
    request: F,
) -> Result<Value, String>
where
    F: FnOnce(&Arc<NamedPipeBridgeState>, Value) -> Result<Value, String>,
{
    let request_id = format!(
        "desktop_settings_snapshot_{}",
        DESKTOP_SETTINGS_REQUEST_ID.fetch_add(1, Ordering::Relaxed)
    );
    let response = request(
        state,
        serde_json::json!({
            "jsonrpc": "2.0",
            "id": request_id,
            "method": "agent.settings.get",
            "params": {
                "scope": "all",
                "request_meta": {
                    "trace_id": "trace_desktop_settings_snapshot",
                    "client_time": DESKTOP_SETTINGS_CLIENT_TIME,
                }
            }
        }),
    )?;

    response
        .get("result")
        .and_then(|result| result.get("data"))
        .and_then(|data| data.get("settings"))
        .cloned()
        .ok_or_else(|| "desktop settings snapshot response missing settings payload".to_string())
}

fn validate_desktop_settings_snapshot(settings: &Value) -> Result<(), String> {
    if settings.is_object() {
        Ok(())
    } else {
        Err("desktop settings snapshot must be a JSON object".to_string())
    }
}

#[cfg(test)]
fn read_workspace_root_from_settings_snapshot(settings: &Value) -> Option<PathBuf> {
    settings
        .get("general")
        .and_then(|general| general.get("download"))
        .and_then(|download| download.get("workspace_path"))
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|path| !path.is_empty())
        .map(PathBuf::from)
}

fn read_task_sources_from_settings_snapshot(settings: &Value) -> Vec<String> {
    let mut task_sources: Vec<String> = Vec::new();

    if let Some(raw_sources) = settings
        .get("task_automation")
        .and_then(|automation| automation.get("task_sources"))
        .and_then(Value::as_array)
    {
        for raw_source in raw_sources {
            let Some(raw_source) = raw_source.as_str() else {
                continue;
            };
            let trimmed = raw_source.trim();
            if trimmed.is_empty() {
                continue;
            }
            if task_sources
                .iter()
                .any(|existing| existing.as_str() == trimmed)
            {
                continue;
            }
            task_sources.push(trimmed.to_string());
        }
    }

    task_sources
}

fn read_trusted_source_note_sources(
    settings_snapshot_state: &DesktopSettingsSnapshotState,
) -> Result<Option<Vec<String>>, String> {
    settings_snapshot_state.task_sources()
}

fn source_note_sources_drift(renderer_sources: &[String], trusted_sources: &[String]) -> bool {
    !renderer_sources.is_empty() && normalize_source_entries(renderer_sources) != trusted_sources
}

fn report_source_note_source_drift(renderer_sources: &[String], trusted_sources: &[String]) {
    if !source_note_sources_drift(renderer_sources, trusted_sources) {
        return;
    }

    eprintln!(
        "desktop source note bridge ignored renderer-supplied sources because they diverged from the trusted settings snapshot"
    );
}

fn normalize_source_entries(raw_sources: &[String]) -> Vec<String> {
    let mut normalized: Vec<String> = Vec::new();

    for raw_source in raw_sources {
        let trimmed = raw_source.trim();
        if trimmed.is_empty() {
            continue;
        }
        if normalized
            .iter()
            .any(|existing| existing.as_str() == trimmed)
        {
            continue;
        }
        normalized.push(trimmed.to_string());
    }

    normalized
}

fn prefetch_desktop_settings_snapshot(app: &mut tauri::App) {
    let bridge_state = Arc::clone(app.state::<Arc<NamedPipeBridgeState>>().inner());
    let settings_snapshot_state =
        Arc::clone(app.state::<Arc<DesktopSettingsSnapshotState>>().inner());

    std::thread::spawn(move || {
        if let Err(error) = seed_desktop_settings_snapshot(
            &bridge_state,
            &settings_snapshot_state,
            Duration::from_millis(DESKTOP_SETTINGS_REQUEST_TIMEOUT_MS),
        ) {
            eprintln!("failed to prefetch desktop settings snapshot: {error}");
        }
    });
}

#[cfg(test)]
mod desktop_settings_snapshot_tests {
    use super::{
        build_local_path_roots, build_source_note_roots, format_runtime_defaults_path,
        read_task_sources_from_settings_snapshot, read_trusted_source_note_sources,
        read_workspace_root_from_settings_snapshot, resolve_workspace_root_from_snapshot,
        source_note_sources_drift, DesktopSettingsSnapshotState, NamedPipeBridgeState,
    };
    use crate::runtime_paths::DesktopRuntimePaths;
    use serde_json::json;
    use std::env;
    use std::fs;
    use std::path::PathBuf;
    use std::sync::Arc;

    #[test]
    fn read_workspace_root_from_settings_snapshot_reads_workspace_path() {
        let workspace_root = env::temp_dir().join("desktop-settings-snapshot");
        let snapshot = json!({
            "general": {
                "download": {
                    "workspace_path": workspace_root.to_string_lossy().to_string(),
                }
            }
        });

        assert_eq!(
            read_workspace_root_from_settings_snapshot(&snapshot),
            Some(workspace_root)
        );
    }

    #[test]
    fn format_runtime_defaults_path_strips_windows_extended_prefixes() {
        assert_eq!(
            format_runtime_defaults_path(PathBuf::from(r"\\?\C:\Users\Administrator\AppData\Local\CialloClaw\data").as_path()),
            "C:/Users/Administrator/AppData/Local/CialloClaw/data"
        );
        assert_eq!(
            format_runtime_defaults_path(PathBuf::from(r"\\?\UNC\fileserver\share\CialloClaw\data").as_path()),
            "//fileserver/share/CialloClaw/data"
        );
    }

    #[test]
    fn read_task_sources_from_settings_snapshot_reads_task_sources() {
        let snapshot = json!({
            "task_automation": {
                "task_sources": [
                    " D:/trusted-notes ",
                    "",
                    "D:/trusted-notes",
                    "workspace/notes",
                    42
                ]
            }
        });

        assert_eq!(
            read_task_sources_from_settings_snapshot(&snapshot),
            vec![
                "D:/trusted-notes".to_string(),
                "workspace/notes".to_string()
            ]
        );
    }

    #[test]
    fn read_trusted_source_note_sources_ignores_renderer_supplied_sources() {
        let state = DesktopSettingsSnapshotState::default();
        state
            .replace(json!({
                "task_automation": {
                    "task_sources": ["D:/trusted-notes"]
                }
            }))
            .expect("replace settings snapshot");

        let trusted_sources = read_trusted_source_note_sources(&state)
            .expect("read trusted source note sources")
            .expect("source note sources from snapshot");

        assert_eq!(trusted_sources, vec!["D:/trusted-notes".to_string()]);
    }

    #[test]
    fn source_note_sources_drift_detects_stale_cached_snapshot() {
        assert!(source_note_sources_drift(
            &[String::from("D:/trusted-notes-next")],
            &[String::from("D:/trusted-notes")]
        ));
        assert!(!source_note_sources_drift(
            &[String::from(" D:/trusted-notes ")],
            &[String::from("D:/trusted-notes")]
        ));
        assert!(!source_note_sources_drift(
            &[],
            &[String::from("D:/trusted-notes")]
        ));
    }

    #[test]
    fn replace_updates_existing_task_source_snapshot() {
        let state = DesktopSettingsSnapshotState::default();
        state
            .replace(json!({
                "task_automation": {
                    "task_sources": ["D:/trusted-notes"]
                }
            }))
            .expect("replace initial settings snapshot");
        state
            .replace(json!({
                "task_automation": {
                    "task_sources": ["D:/trusted-notes-next"]
                }
            }))
            .expect("replace stale task sources");

        assert_eq!(
            read_trusted_source_note_sources(&state).expect("read refreshed task sources"),
            Some(vec!["D:/trusted-notes-next".to_string()])
        );
    }

    #[test]
    fn seed_does_not_override_newer_settings_snapshot() {
        let initial_root = env::temp_dir().join("desktop-settings-initial");
        let newer_root = env::temp_dir().join("desktop-settings-newer");
        let state = DesktopSettingsSnapshotState::default();

        state
            .seed(json!({
                "general": {
                    "download": {
                        "workspace_path": initial_root.to_string_lossy().to_string(),
                    }
                }
            }))
            .expect("seed initial settings");
        state
            .replace(json!({
                "general": {
                    "download": {
                        "workspace_path": newer_root.to_string_lossy().to_string(),
                    }
                }
            }))
            .expect("replace settings snapshot");
        state
            .seed(json!({
                "general": {
                    "download": {
                        "workspace_path": initial_root.to_string_lossy().to_string(),
                    }
                }
            }))
            .expect("seed stale snapshot");

        assert_eq!(
            state.workspace_root().expect("read workspace root"),
            Some(newer_root)
        );
    }

    #[test]
    fn resolve_workspace_root_from_snapshot_maps_legacy_relative_workspace_to_runtime_root() {
        let runtime_root = env::temp_dir().join("desktop-runtime-root");
        let runtime_paths = Arc::new(DesktopRuntimePaths::from_runtime_root(runtime_root.clone()));
        let state = Arc::new(DesktopSettingsSnapshotState::default());
        state
            .replace(json!({
                "general": {
                    "download": {
                        "workspace_path": "workspace",
                    }
                }
            }))
            .expect("replace legacy workspace settings snapshot");

        assert_eq!(
            resolve_workspace_root_from_snapshot(&state, &runtime_paths),
            Some(runtime_root.join("workspace"))
        );
    }

    #[test]
    fn build_local_path_roots_stays_pinned_to_runtime_workspace_before_restart() {
        let runtime_root = env::temp_dir().join("desktop-runtime-pinned-local-path-roots");
        let runtime_workspace = runtime_root.join("workspace");
        fs::create_dir_all(&runtime_workspace).expect("create runtime workspace root");
        fs::create_dir_all(runtime_root.join("temp")).expect("create runtime temp root");
        let runtime_paths = Arc::new(DesktopRuntimePaths::from_runtime_root(runtime_root));
        let pending_workspace = env::temp_dir().join("desktop-pending-workspace");
        let bridge_state = Arc::new(NamedPipeBridgeState::default());
        let snapshot_state = Arc::new(DesktopSettingsSnapshotState::default());
        snapshot_state
            .replace(json!({
                "general": {
                    "download": {
                        "workspace_path": pending_workspace.to_string_lossy().to_string(),
                    }
                }
            }))
            .expect("replace pending workspace settings snapshot");

        let roots = build_local_path_roots(&bridge_state, &snapshot_state, &runtime_paths);

        assert_eq!(roots.workspace_root(), Some(runtime_paths.workspace_root()));
    }

    #[test]
    fn build_source_note_roots_stays_pinned_to_runtime_workspace_before_restart() {
        let runtime_root = env::temp_dir().join("desktop-runtime-pinned-source-note-roots");
        let runtime_workspace = runtime_root.join("workspace");
        fs::create_dir_all(&runtime_workspace).expect("create runtime workspace root");
        let runtime_paths = Arc::new(DesktopRuntimePaths::from_runtime_root(runtime_root));
        let pending_workspace = env::temp_dir().join("desktop-pending-source-workspace");
        let bridge_state = Arc::new(NamedPipeBridgeState::default());
        let snapshot_state = Arc::new(DesktopSettingsSnapshotState::default());
        snapshot_state
            .replace(json!({
                "general": {
                    "download": {
                        "workspace_path": pending_workspace.to_string_lossy().to_string(),
                    }
                }
            }))
            .expect("replace pending source-note workspace snapshot");

        let roots = build_source_note_roots(
            &bridge_state,
            &snapshot_state,
            &runtime_paths,
            &[String::from("workspace/notes")],
        );

        assert_eq!(roots.workspace_root(), Some(runtime_paths.workspace_root()));
    }
}

#[cfg(test)]
mod named_pipe_routing_tests {
    use super::{
        classify_isolated_named_pipe_message, configure_sidecar_named_pipe,
        requires_shared_named_pipe_request, should_use_isolated_named_pipe_payload,
        IsolatedNamedPipeMessage, NamedPipeBridgeState, DEFAULT_NAMED_PIPE_PATH,
    };
    use serde_json::json;

    #[test]
    fn isolated_payload_routing_keeps_floating_ball_bubble_submit_isolated() {
        let payload = json!({
            "jsonrpc": "2.0",
            "id": "req_submit",
            "method": "agent.input.submit",
            "params": {
                "source": "floating_ball",
                "options": {
                    "preferred_delivery": "bubble"
                }
            }
        });

        assert!(should_use_isolated_named_pipe_payload(&payload));
    }

    #[test]
    fn isolated_payload_routing_keeps_non_bubble_submit_shared() {
        let dashboard_payload = json!({
            "jsonrpc": "2.0",
            "id": "req_submit_dashboard",
            "method": "agent.input.submit",
            "params": {
                "source": "dashboard"
            }
        });
        let task_start_payload = json!({
            "jsonrpc": "2.0",
            "id": "req_task_start",
            "method": "agent.task.start",
            "params": {}
        });

        assert!(!should_use_isolated_named_pipe_payload(&dashboard_payload));
        assert!(!should_use_isolated_named_pipe_payload(&task_start_payload));
        assert!(requires_shared_named_pipe_request("agent.task.start"));
    }

    #[test]
    fn configure_sidecar_named_pipe_keeps_default_pipe_when_sidecar_is_unavailable() {
        let bridge_state = NamedPipeBridgeState::default();

        let configured = configure_sidecar_named_pipe(
            &bridge_state,
            r"\\.\pipe\cialloclaw-rpc-user-test",
            false,
        )
        .expect("keep default pipe when sidecar is unavailable");

        assert!(!configured);
        assert_eq!(
            bridge_state.pipe_name().expect("read configured pipe name"),
            DEFAULT_NAMED_PIPE_PATH
        );
    }

    #[test]
    fn configure_sidecar_named_pipe_switches_to_user_scoped_pipe_after_start() {
        let bridge_state = NamedPipeBridgeState::default();
        let scoped_pipe = r"\\.\pipe\cialloclaw-rpc-user-test";

        let configured = configure_sidecar_named_pipe(&bridge_state, scoped_pipe, true)
            .expect("configure user-scoped pipe");

        assert!(configured);
        assert_eq!(
            bridge_state.pipe_name().expect("read configured pipe name"),
            scoped_pipe
        );
    }

    #[test]
    fn isolated_transport_degradation_forces_floating_ball_submit_back_to_shared() {
        let bridge_state = NamedPipeBridgeState::default();
        let payload = json!({
            "jsonrpc": "2.0",
            "id": "req_submit",
            "method": "agent.input.submit",
            "params": {
                "source": "floating_ball",
                "options": {
                    "preferred_delivery": "bubble"
                }
            }
        });

        assert!(bridge_state.should_use_isolated_transport(&payload));

        bridge_state.mark_isolated_transport_degraded();

        assert!(!bridge_state.should_use_isolated_transport(&payload));
    }

    #[test]
    fn isolated_named_pipe_message_classification_marks_matching_response() {
        let message = json!({
            "jsonrpc": "2.0",
            "id": "req_submit",
            "result": {
                "data": {
                    "task": {
                        "task_id": "task-shell-ball"
                    }
                }
            }
        });

        let classified = classify_isolated_named_pipe_message("\"req_submit\"", message);

        match classified {
            IsolatedNamedPipeMessage::MatchedResponse(message) => {
                assert_eq!(message.get("id"), Some(&json!("req_submit")));
            }
            _ => panic!("expected matching isolated response to be returned"),
        }
    }

    #[test]
    fn isolated_named_pipe_message_classification_forwards_notifications() {
        let message = json!({
            "jsonrpc": "2.0",
            "method": "task.updated",
            "params": {
                "task_id": "task-shell-ball",
                "status": "processing"
            }
        });

        let classified = classify_isolated_named_pipe_message("\"req_submit\"", message);

        match classified {
            IsolatedNamedPipeMessage::ForwardedNotification { topic, message } => {
                assert_eq!(topic, "task.updated");
                assert_eq!(
                    message
                        .get("params")
                        .and_then(|params| params.get("task_id")),
                    Some(&json!("task-shell-ball"))
                );
            }
            _ => panic!("expected isolated notification to be forwarded"),
        }
    }
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

/// should_use_isolated_named_pipe_request identifies request types that can use
/// a fresh response channel without depending on request/response ordering on
/// the shared session. Isolated requests may still forward notifications into
/// the host-side subscription fan-out when the backend emits them.
fn should_use_isolated_named_pipe_request(method: &str) -> bool {
    !requires_shared_named_pipe_request(method)
}

fn should_use_isolated_named_pipe_payload(payload: &Value) -> bool {
    let Ok(method) = extract_request_method(payload) else {
        return false;
    };

    if method == "agent.input.submit" {
        let params = payload.get("params").and_then(Value::as_object);
        let source = params
            .and_then(|params| params.get("source"))
            .and_then(Value::as_str);
        let preferred_delivery = params
            .and_then(|params| params.get("options"))
            .and_then(Value::as_object)
            .and_then(|options| options.get("preferred_delivery"))
            .and_then(Value::as_str);

        return source == Some("floating_ball") && preferred_delivery == Some("bubble");
    }

    should_use_isolated_named_pipe_request(&method)
}

/// requires_shared_named_pipe_request keeps task-mutating calls on the shared
/// session because those methods may emit request-scoped runtime notifications
/// that shell-ball and dashboard flows consume through the bridge subscription
/// fan-out.
fn requires_shared_named_pipe_request(method: &str) -> bool {
    matches!(
        method,
        "agent.task.start"
            | "agent.task.confirm"
            | "agent.task.control"
            | "agent.task.steer"
            | "agent.security.respond"
            | "agent.security.restore.apply"
            | "agent.notepad.convert_to_task"
    )
}

enum IsolatedNamedPipeMessage {
    ForwardedNotification { topic: String, message: Value },
    MatchedResponse(Value),
    Ignored,
}

fn classify_isolated_named_pipe_message(
    request_id: &str,
    message: Value,
) -> IsolatedNamedPipeMessage {
    if let Some(method) = message.get("method").and_then(Value::as_str) {
        return IsolatedNamedPipeMessage::ForwardedNotification {
            topic: method.to_string(),
            message,
        };
    }

    if let Some(id) = message.get("id") {
        if normalize_id(id) == request_id {
            return IsolatedNamedPipeMessage::MatchedResponse(message);
        }
    }

    IsolatedNamedPipeMessage::Ignored
}

/// send_isolated_named_pipe_request performs one request/response exchange on a
/// fresh pipe connection while forwarding any notifications into the shared
/// desktop subscription fan-out.
fn send_isolated_named_pipe_request(
    state: &Arc<NamedPipeBridgeState>,
    pipe_name: &str,
    request_id: &str,
    payload: Value,
) -> Result<Value, String> {
    let stream = open_named_pipe_with_retry(pipe_name, "isolated")?;
    let reader = stream
        .try_clone()
        .map_err(|error| format!("failed to clone isolated named pipe handle: {error}"))?;
    let mut writer = BufWriter::new(stream);

    serde_json::to_writer(&mut writer, &payload)
        .map_err(|error| format!("failed to serialize isolated json-rpc payload: {error}"))?;
    writer
        .write_all(b"\n")
        .map_err(|error| format!("failed to write isolated named pipe delimiter: {error}"))?;
    writer
        .flush()
        .map_err(|error| format!("failed to flush isolated named pipe payload: {error}"))?;

    let mut responses =
        serde_json::Deserializer::from_reader(BufReader::new(reader)).into_iter::<Value>();
    while let Some(result) = responses.next() {
        let message = result
            .map_err(|error| format!("failed to decode isolated named pipe response: {error}"))?;

        match classify_isolated_named_pipe_message(request_id, message) {
            IsolatedNamedPipeMessage::ForwardedNotification { topic, message } => {
                state.dispatch_notification(&topic, &message);
            }
            IsolatedNamedPipeMessage::MatchedResponse(message) => {
                return Ok(message);
            }
            IsolatedNamedPipeMessage::Ignored => {}
        }
    }

    Err("isolated named pipe response stream ended before the matching json-rpc envelope was returned".to_string())
}

/// open_named_pipe_with_retry tolerates transient `ERROR_PIPE_BUSY` windows that
/// occur while the listener rotates pipe instances for concurrent desktop
/// requests. Without this retry, short-lived control-panel reads can fail even
/// though the local service is healthy and ready.
fn open_named_pipe_with_retry(pipe_name: &str, mode: &str) -> Result<std::fs::File, String> {
    let started_at = Instant::now();

    loop {
        match OpenOptions::new().read(true).write(true).open(pipe_name) {
            Ok(stream) => return Ok(stream),
            Err(error) if error.raw_os_error() == Some(231) => {
                if started_at.elapsed() >= LOCAL_SERVICE_PIPE_BUSY_RETRY_TIMEOUT {
                    return Err(format!(
                        "failed to open {mode} named pipe {pipe_name} before retry timeout: {}",
                        error
                    ));
                }
                thread::sleep(LOCAL_SERVICE_PIPE_BUSY_RETRY_DELAY);
            }
            Err(error) => {
                return Err(format!(
                    "failed to open {mode} named pipe {pipe_name}: {error}"
                ));
            }
        }
    }
}

fn extract_request_id(payload: &Value) -> Result<String, String> {
    let id = payload
        .get("id")
        .ok_or_else(|| "json-rpc payload missing id".to_string())?;

    Ok(normalize_id(id))
}

fn extract_request_method(payload: &Value) -> Result<String, String> {
    payload
        .get("method")
        .and_then(Value::as_str)
        .map(|method| method.trim().to_string())
        .filter(|method| !method.is_empty())
        .ok_or_else(|| "json-rpc payload missing method".to_string())
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

    if CONTROL_PANEL_WINDOW_CREATION_IN_PROGRESS
        .compare_exchange(false, true, Ordering::SeqCst, Ordering::SeqCst)
        .is_err()
    {
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

        CONTROL_PANEL_WINDOW_CREATION_IN_PROGRESS.store(false, Ordering::SeqCst);

        if let Err(error) = create_result {
            eprintln!("failed to create control panel from tray: {error}");
        }
    });
}

#[tauri::command]
fn desktop_open_or_focus_control_panel(app: tauri::AppHandle) -> Result<(), String> {
    open_or_focus_control_panel_window(&app);
    Ok(())
}

fn ensure_onboarding_window(app: &tauri::AppHandle) {
    if app.get_webview_window(ONBOARDING_WINDOW_LABEL).is_some() {
        return;
    }

    if ONBOARDING_WINDOW_CREATION_IN_PROGRESS
        .compare_exchange(false, true, Ordering::SeqCst, Ordering::SeqCst)
        .is_err()
    {
        return;
    }

    let handle = app.clone();
    std::thread::spawn(move || {
        let create_result = WebviewWindowBuilder::new(
            &handle,
            ONBOARDING_WINDOW_LABEL,
            WebviewUrl::App("onboarding.html".into()),
        )
        .title("CialloClaw Onboarding")
        .inner_size(460.0, 340.0)
        .decorations(false)
        .transparent(true)
        .always_on_top(true)
        .resizable(false)
        .skip_taskbar(true)
        .shadow(false)
        // Keep the card window hidden until the frontend finishes its first
        // layout, then promote it as a normal interactive topmost surface.
        .visible(false)
        .focused(false)
        .build();

        ONBOARDING_WINDOW_CREATION_IN_PROGRESS.store(false, Ordering::SeqCst);

        match create_result {
            Ok(window) => {
                if let Ok(hwnd) = window.hwnd() {
                    unsafe {
                        set_forward_mouse_messages(hwnd, false);
                        set_window_ignore_cursor_events(hwnd, false);
                    }
                }
            }
            Err(error) => {
                eprintln!("failed to create onboarding window: {error}");
            }
        }
    });
}

#[tauri::command]
fn desktop_open_or_focus_onboarding(app: tauri::AppHandle) -> Result<(), String> {
    ensure_onboarding_window(&app);
    Ok(())
}

#[cfg(windows)]
#[tauri::command]
fn desktop_promote_onboarding(app: tauri::AppHandle) -> Result<(), String> {
    let window = app
        .get_webview_window(ONBOARDING_WINDOW_LABEL)
        .ok_or_else(|| format!("webview window not found: {ONBOARDING_WINDOW_LABEL}"))?;

    if let Err(error) = window.unminimize() {
        eprintln!("failed to unminimize onboarding window: {error}");
    }

    let hwnd = window
        .hwnd()
        .map_err(|error| format!("failed to get onboarding hwnd: {error}"))?;

    unsafe {
        // Promote the card-sized onboarding window in one native operation.
        // SWP_NOACTIVATE avoids stealing focus from the workflow surface while
        // still making the first visible frame reliable on cold launches.
        SetWindowPos(
            hwnd,
            Some(HWND_TOPMOST),
            0,
            0,
            0,
            0,
            SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE | SWP_SHOWWINDOW,
        )
        .map_err(|error| format!("failed to promote onboarding window: {error}"))?;
    }

    Ok(())
}

#[cfg(not(windows))]
#[tauri::command]
fn desktop_promote_onboarding(app: tauri::AppHandle) -> Result<(), String> {
    let window = app
        .get_webview_window(ONBOARDING_WINDOW_LABEL)
        .ok_or_else(|| format!("webview window not found: {ONBOARDING_WINDOW_LABEL}"))?;

    window
        .unminimize()
        .map_err(|error| format!("failed to unminimize onboarding window: {error}"))?;
    window
        .show()
        .map_err(|error| format!("failed to show onboarding window: {error}"))?;

    Ok(())
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
    if let Some(window) = app.get_webview_window(SHELL_BALL_WINDOW_LABEL) {
        window
            .hide()
            .map_err(|error| format!("failed to hide {SHELL_BALL_WINDOW_LABEL}: {error}"))?;
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

        if ctrl_down && (keyboard_info.vkCode == b'C' as u32 || keyboard_info.vkCode == b'X' as u32)
        {
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
                SetWindowsHookExW(
                    WH_KEYBOARD_LL,
                    Some(shell_ball_clipboard_keyboard_hook),
                    None,
                    0,
                )
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
    let open_control_panel =
        MenuItemBuilder::with_id(TRAY_MENU_OPEN_CONTROL_PANEL_ID, "打开控制面板").build(app)?;
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
                open_or_focus_dashboard_window(tray.app_handle());

                if let Err(error) = request_shell_ball_dashboard_open_transition(tray.app_handle())
                {
                    eprintln!(
                        "failed to trigger shell-ball dashboard transition from tray: {error}"
                    );
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

#[cfg(windows)]
static SHELL_BALL_CLIPBOARD_MOUSE_HOOK: Lazy<Mutex<Option<isize>>> = Lazy::new(|| Mutex::new(None));

#[cfg(windows)]
static SHELL_BALL_CLIPBOARD_KEYBOARD_HOOK: Lazy<Mutex<Option<isize>>> =
    Lazy::new(|| Mutex::new(None));

#[cfg(windows)]
static SHELL_BALL_APP_HANDLE: Lazy<Mutex<Option<tauri::AppHandle>>> =
    Lazy::new(|| Mutex::new(None));

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

#[tauri::command]
fn pick_shell_ball_files(window: tauri::Window) -> Result<Vec<String>, String> {
    if window.label() != SHELL_BALL_WINDOW_LABEL {
        return Err("pick_shell_ball_files is only available to the shell-ball window".into());
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

#[derive(Clone, serde::Deserialize)]
struct ShellBallInteractiveRect {
    x: i32,
    y: i32,
    width: i32,
    height: i32,
}

#[cfg(windows)]
#[derive(Clone, Default)]
struct ShellBallInteractiveState {
    hwnd: Option<isize>,
    regions: Vec<ShellBallInteractiveRect>,
    press_lock: bool,
    current_ignore: Option<bool>,
}

#[cfg(windows)]
static SHELL_BALL_INTERACTIVE_STATE: Lazy<Mutex<ShellBallInteractiveState>> =
    Lazy::new(|| Mutex::new(ShellBallInteractiveState::default()));

#[cfg(windows)]
#[derive(Clone, Default)]
struct OnboardingInteractiveState {
    hwnd: Option<isize>,
    regions: Vec<ShellBallInteractiveRect>,
    current_ignore: Option<bool>,
}

#[cfg(windows)]
static ONBOARDING_INTERACTIVE_STATE: Lazy<Mutex<OnboardingInteractiveState>> =
    Lazy::new(|| Mutex::new(OnboardingInteractiveState::default()));

fn open_or_focus_dashboard_window(app: &tauri::AppHandle) {
    if let Some(window) = app.get_webview_window(DASHBOARD_WINDOW_LABEL) {
        if let Err(error) = window.unminimize() {
            eprintln!("failed to unminimize dashboard from tray: {error}");
        }
        if let Err(error) = window.set_fullscreen(true) {
            eprintln!("failed to set dashboard fullscreen from tray: {error}");
        }
        if let Err(error) = window.show() {
            eprintln!("failed to show dashboard from tray: {error}");
        }
        if let Err(error) = window.set_focus() {
            eprintln!("failed to focus dashboard from tray: {error}");
        }
        return;
    }

    let handle = app.clone();
    std::thread::spawn(move || {
        let create_result = WebviewWindowBuilder::new(
            &handle,
            DASHBOARD_WINDOW_LABEL,
            WebviewUrl::App("dashboard.html".into()),
        )
        .title("CialloClaw Dashboard")
        .inner_size(1280.0, 860.0)
        .decorations(false)
        .visible(true)
        .focused(true)
        .fullscreen(true)
        .build();

        if let Err(error) = create_result {
            eprintln!("failed to create dashboard from tray: {error}");
        }
    });
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
unsafe fn set_window_ignore_cursor_events(hwnd: HWND, ignore: bool) {
    let current_style = GetWindowLongPtrW(hwnd, GWL_EXSTYLE) as u32;
    let layered_style = current_style | WS_EX_LAYERED.0 as u32;
    let next_style = if ignore {
        layered_style | WS_EX_TRANSPARENT.0 as u32
    } else {
        layered_style & !(WS_EX_TRANSPARENT.0 as u32)
    };

    if next_style == current_style {
        return;
    }

    let _ = SetWindowLongPtrW(hwnd, GWL_EXSTYLE, next_style as isize);
    let _ = SetWindowPos(
        hwnd,
        Some(HWND(std::ptr::null_mut())),
        0,
        0,
        0,
        0,
        SWP_NOMOVE | SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE | SWP_FRAMECHANGED,
    );
}

#[cfg(windows)]
unsafe fn sync_shell_ball_native_hit_testing(screen_point: POINT) {
    let snapshot = match SHELL_BALL_INTERACTIVE_STATE.lock() {
        Ok(state) => state.clone(),
        Err(_) => return,
    };

    let Some(hwnd_value) = snapshot.hwnd else {
        return;
    };

    let hwnd = HWND(hwnd_value as _);
    let mut client_point = screen_point;
    if !ScreenToClient(hwnd, &mut client_point).as_bool() {
        return;
    }

    let hit_interactive_region = snapshot.press_lock
        || snapshot.regions.iter().any(|region| {
            client_point.x >= region.x
                && client_point.x <= region.x + region.width
                && client_point.y >= region.y
                && client_point.y <= region.y + region.height
        });
    let next_ignore = !hit_interactive_region;

    if snapshot.current_ignore == Some(next_ignore) {
        return;
    }

    set_window_ignore_cursor_events(hwnd, next_ignore);

    if let Ok(mut state) = SHELL_BALL_INTERACTIVE_STATE.lock() {
        if state.hwnd == Some(hwnd_value) {
            state.current_ignore = Some(next_ignore);
        }
    }
}

#[cfg(windows)]
unsafe fn update_shell_ball_native_tracking() {
    let snapshot = match SHELL_BALL_INTERACTIVE_STATE.lock() {
        Ok(state) => state.clone(),
        Err(_) => return,
    };

    let Some(hwnd_value) = snapshot.hwnd else {
        return;
    };

    let hwnd = HWND(hwnd_value as _);
    let should_track = snapshot.press_lock || !snapshot.regions.is_empty();
    set_forward_mouse_messages(hwnd, should_track);

    if !should_track {
        set_window_ignore_cursor_events(hwnd, false);
        if let Ok(mut state) = SHELL_BALL_INTERACTIVE_STATE.lock() {
            if state.hwnd == Some(hwnd_value) {
                state.current_ignore = Some(false);
            }
        }
    }
}

#[cfg(windows)]
unsafe fn sync_onboarding_native_hit_testing(screen_point: POINT) {
    let snapshot = match ONBOARDING_INTERACTIVE_STATE.lock() {
        Ok(state) => state.clone(),
        Err(_) => return,
    };

    let Some(hwnd_value) = snapshot.hwnd else {
        return;
    };

    let hwnd = HWND(hwnd_value as _);
    let mut client_point = screen_point;
    if !ScreenToClient(hwnd, &mut client_point).as_bool() {
        return;
    }

    let hit_interactive_region = snapshot.regions.iter().any(|region| {
        client_point.x >= region.x
            && client_point.x <= region.x + region.width
            && client_point.y >= region.y
            && client_point.y <= region.y + region.height
    });
    let next_ignore = !hit_interactive_region;

    if snapshot.current_ignore == Some(next_ignore) {
        return;
    }

    set_window_ignore_cursor_events(hwnd, next_ignore);

    if let Ok(mut state) = ONBOARDING_INTERACTIVE_STATE.lock() {
        if state.hwnd == Some(hwnd_value) {
            state.current_ignore = Some(next_ignore);
        }
    }
}

#[cfg(windows)]
unsafe fn update_onboarding_native_tracking() {
    let snapshot = match ONBOARDING_INTERACTIVE_STATE.lock() {
        Ok(state) => state.clone(),
        Err(_) => return,
    };

    let Some(hwnd_value) = snapshot.hwnd else {
        return;
    };

    let hwnd = HWND(hwnd_value as _);
    let should_track = !snapshot.regions.is_empty();
    set_forward_mouse_messages(hwnd, should_track);

    if !should_track {
        set_window_ignore_cursor_events(hwnd, true);
        if let Ok(mut state) = ONBOARDING_INTERACTIVE_STATE.lock() {
            if state.hwnd == Some(hwnd_value) {
                state.current_ignore = Some(true);
            }
        }
    }
}

#[cfg(windows)]
#[tauri::command]
fn onboarding_reset_interactive_state(app: tauri::AppHandle) -> Result<(), String> {
    if let Some(window) = app.get_webview_window(ONBOARDING_WINDOW_LABEL) {
        let hwnd = window
            .hwnd()
            .map_err(|error| format!("failed to get onboarding hwnd: {error}"))?;

        unsafe {
            set_forward_mouse_messages(hwnd, false);
            set_window_ignore_cursor_events(hwnd, true);
        }
    }

    let mut state = ONBOARDING_INTERACTIVE_STATE
        .lock()
        .map_err(|_| "onboarding interactive state lock poisoned".to_string())?;
    state.hwnd = None;
    state.regions.clear();
    state.current_ignore = None;
    Ok(())
}

#[cfg(not(windows))]
#[tauri::command]
fn onboarding_reset_interactive_state(_app: tauri::AppHandle) -> Result<(), String> {
    Ok(())
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

        sync_shell_ball_native_hit_testing(point);
        sync_onboarding_native_hit_testing(point);

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
fn onboarding_set_ignore_cursor_events(app: tauri::AppHandle, ignore: bool) -> Result<(), String> {
    if let Some(window) = app.get_webview_window(ONBOARDING_WINDOW_LABEL) {
        let hwnd = window
            .hwnd()
            .map_err(|error| format!("failed to get onboarding hwnd: {error}"))?;

        unsafe {
            set_forward_mouse_messages(hwnd, false);
            set_window_ignore_cursor_events(hwnd, ignore);
        }
    }

    Ok(())
}

#[cfg(not(windows))]
#[tauri::command]
fn onboarding_set_ignore_cursor_events(
    _app: tauri::AppHandle,
    _ignore: bool,
) -> Result<(), String> {
    Ok(())
}

#[cfg(windows)]
#[tauri::command]
fn onboarding_set_interactive_regions(
    window: tauri::Window,
    regions: Vec<ShellBallInteractiveRect>,
) -> Result<(), String> {
    if window.label() != ONBOARDING_WINDOW_LABEL {
        return Err(
            "onboarding_set_interactive_regions is only available to the onboarding window".into(),
        );
    }

    let hwnd = window
        .hwnd()
        .map_err(|error| format!("failed to get onboarding hwnd: {error}"))?;

    {
        let mut state = ONBOARDING_INTERACTIVE_STATE
            .lock()
            .map_err(|_| "onboarding interactive state lock poisoned".to_string())?;
        state.hwnd = Some(hwnd.0 as isize);
        state.regions = regions;
        state.current_ignore = None;
    }

    let mut point = POINT { x: 0, y: 0 };
    unsafe {
        update_onboarding_native_tracking();
        if GetCursorPos(&mut point).is_ok() {
            sync_onboarding_native_hit_testing(point);
        }
    }

    Ok(())
}

#[cfg(not(windows))]
#[tauri::command]
fn onboarding_set_interactive_regions(
    _window: tauri::Window,
    _regions: Vec<ShellBallInteractiveRect>,
) -> Result<(), String> {
    Ok(())
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

#[cfg(windows)]
#[tauri::command]
fn shell_ball_set_interactive_regions(
    window: tauri::Window,
    regions: Vec<ShellBallInteractiveRect>,
) -> Result<(), String> {
    let hwnd = window
        .hwnd()
        .map_err(|error| format!("failed to get shell-ball hwnd: {error}"))?;

    {
        let mut state = SHELL_BALL_INTERACTIVE_STATE
            .lock()
            .map_err(|_| "shell-ball interactive state lock poisoned".to_string())?;
        state.hwnd = Some(hwnd.0 as isize);
        state.regions = regions;
        state.current_ignore = None;
    }

    let mut point = POINT { x: 0, y: 0 };
    unsafe {
        update_shell_ball_native_tracking();
        if GetCursorPos(&mut point).is_ok() {
            sync_shell_ball_native_hit_testing(point);
        }
    }

    Ok(())
}

#[cfg(not(windows))]
#[tauri::command]
fn shell_ball_set_interactive_regions(
    _window: tauri::Window,
    _regions: Vec<ShellBallInteractiveRect>,
) -> Result<(), String> {
    Ok(())
}

#[cfg(windows)]
#[tauri::command]
fn shell_ball_set_press_lock(window: tauri::Window, locked: bool) -> Result<(), String> {
    let hwnd = window
        .hwnd()
        .map_err(|error| format!("failed to get shell-ball hwnd: {error}"))?;

    {
        let mut state = SHELL_BALL_INTERACTIVE_STATE
            .lock()
            .map_err(|_| "shell-ball interactive state lock poisoned".to_string())?;
        state.hwnd = Some(hwnd.0 as isize);
        state.press_lock = locked;
        state.current_ignore = None;
    }

    let mut point = POINT { x: 0, y: 0 };
    unsafe {
        update_shell_ball_native_tracking();
        if GetCursorPos(&mut point).is_ok() {
            sync_shell_ball_native_hit_testing(point);
        }
    }

    Ok(())
}

#[cfg(not(windows))]
#[tauri::command]
fn shell_ball_set_press_lock(_window: tauri::Window, _locked: bool) -> Result<(), String> {
    Ok(())
}

#[tauri::command]
fn shell_ball_apply_window_frame(
    window: tauri::Window,
    x: f64,
    y: f64,
    width: f64,
    height: f64,
) -> Result<(), String> {
    let scale_factor = window
        .scale_factor()
        .map_err(|error| format!("failed to read shell-ball scale factor: {error}"))?;
    let physical_x = (x * scale_factor).round();
    let physical_y = (y * scale_factor).round();
    let physical_width = (width * scale_factor).round().max(1.0);
    let physical_height = (height * scale_factor).round().max(1.0);

    window
        .set_size(tauri::PhysicalSize::new(
            physical_width as u32,
            physical_height as u32,
        ))
        .map_err(|error| format!("failed to set shell-ball window size: {error}"))?;

    window
        .set_position(tauri::PhysicalPosition::new(
            physical_x as i32,
            physical_y as i32,
        ))
        .map_err(|error| format!("failed to set shell-ball window position: {error}"))
}

#[tauri::command]
async fn shell_ball_read_selection_snapshot(
    app: tauri::AppHandle,
) -> Result<Option<selection::SelectionSnapshotPayload>, String> {
    tauri::async_runtime::spawn_blocking(move || selection::read_selection_snapshot(&app))
        .await
        .map_err(|error| format!("selection snapshot task failed: {error}"))?
}

fn main() {
    let app = tauri::Builder::default()
        .manage(Arc::new(NamedPipeBridgeState::default()))
        .manage(Arc::new(LocalServiceSidecarState::default()))
        .manage(Arc::new(DesktopSettingsSnapshotState::default()))
        .plugin(tauri_plugin_clipboard_manager::init())
        .plugin(tauri_plugin_shell::init())
        .setup(|app| {
            let runtime_paths = runtime_paths::DesktopRuntimePaths::detect()
                .map_err(|error| std::io::Error::other(error))?;
            runtime_paths
                .ensure_runtime_directories()
                .map_err(|error| std::io::Error::other(error))?;
            app.manage(Arc::new(runtime_paths));

            let sidecar_state = app.state::<Arc<LocalServiceSidecarState>>();
            let bridge_state = app.state::<Arc<NamedPipeBridgeState>>();
            let app_data_dir = app.path().app_data_dir().map_err(|error| {
                std::io::Error::other(format!("failed to resolve app data dir: {error}"))
            })?;
            let app_data_dir = resolve_required_path(app_data_dir, "app data dir")
                .map_err(std::io::Error::other)?;
            let named_pipe_path = derive_local_service_pipe_name(&app_data_dir);
            let sidecar_started =
                start_local_service_sidecar(app.handle(), sidecar_state.inner(), &named_pipe_path)
                    .map_err(std::io::Error::other)?;
            if configure_sidecar_named_pipe(bridge_state.inner(), &named_pipe_path, sidecar_started)
                .map_err(std::io::Error::other)?
            {
                append_local_service_log(
                    app.handle(),
                    &format!("configured local service named pipe: {named_pipe_path}"),
                );
                wait_for_local_service_ready(app.handle(), bridge_state.inner())
                    .map_err(std::io::Error::other)?;
            }
            activity::install_mouse_activity_listener()
                .map_err(|error| std::io::Error::other(error))?;
            install_shell_ball_clipboard_hooks(app.handle())
                .map_err(|error| std::io::Error::other(error))?;
            selection::install_selection_listener(app.handle())
                .map_err(|error| std::io::Error::other(error))?;
            window_context::install_window_context_listener(app.handle())
                .map_err(|error| std::io::Error::other(error))?;
            prefetch_desktop_settings_snapshot(app);

            Ok(install_system_tray(app)?)
        })
        .invoke_handler(tauri::generate_handler![
            named_pipe_request,
            named_pipe_subscribe,
            named_pipe_unsubscribe,
            shell_ball_set_ignore_cursor_events,
            shell_ball_get_mouse_position,
            shell_ball_set_interactive_regions,
            onboarding_set_ignore_cursor_events,
            onboarding_set_interactive_regions,
            onboarding_reset_interactive_state,
            shell_ball_set_press_lock,
            desktop_get_mouse_activity_snapshot,
            desktop_capture_screenshot,
            desktop_get_active_window_context,
            desktop_open_or_focus_control_panel,
            desktop_open_or_focus_onboarding,
            desktop_promote_onboarding,
            desktop_open_local_path,
            desktop_open_external_url,
            desktop_reveal_local_path,
            desktop_open_runtime_data_path,
            desktop_open_runtime_workspace_path,
            desktop_sync_settings_snapshot,
            desktop_get_runtime_defaults,
            desktop_load_source_notes,
            desktop_load_source_note_index,
            desktop_create_source_note,
            desktop_save_source_note,
            pick_shell_ball_files,
            shell_ball_apply_window_frame,
            shell_ball_read_selection_snapshot
        ])
        .build(tauri::generate_context!())
        .expect("error while building tauri application");

    app.run(|app_handle, event| {
        if matches!(event, RunEvent::Exit | RunEvent::ExitRequested { .. }) {
            let sidecar_state = app_handle.state::<Arc<LocalServiceSidecarState>>();
            let _ = sidecar_state.stop();
        }
    });
}
