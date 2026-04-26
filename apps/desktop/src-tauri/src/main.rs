// This entry point boots the desktop Tauri host process.
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod activity;
mod local_path;
mod screen_capture;
mod selection;
mod source_notes;
mod window_context;

use serde_json::Value;
use std::collections::HashMap;
use std::fs::OpenOptions;
use std::io::{BufReader, BufWriter, Write};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicU32, Ordering};
use std::sync::{mpsc, Arc, Mutex};
use std::time::Duration;
use tauri::ipc::Channel;
use tauri::menu::{MenuBuilder, MenuItemBuilder};
use tauri::tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent};
use tauri::{Emitter, Manager, WebviewUrl, WebviewWindowBuilder};

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

const NAMED_PIPE_PATH: &str = r"\\.\pipe\cialloclaw-rpc";
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

    fn has_snapshot(&self) -> Result<bool, String> {
        let snapshot = self
            .settings
            .lock()
            .map_err(|_| "desktop settings snapshot lock poisoned".to_string())?;

        Ok(snapshot.is_some())
    }
}

impl NamedPipeBridgeState {
    fn request(self: &Arc<Self>, payload: Value) -> Result<Value, String> {
        self.request_internal(payload, None)
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

#[tauri::command]
async fn desktop_open_local_path(
    bridge_state: tauri::State<'_, Arc<NamedPipeBridgeState>>,
    settings_snapshot_state: tauri::State<'_, Arc<DesktopSettingsSnapshotState>>,
    path: String,
) -> Result<(), String> {
    let bridge_state = Arc::clone(bridge_state.inner());
    let settings_snapshot_state = Arc::clone(settings_snapshot_state.inner());
    tauri::async_runtime::spawn_blocking(move || {
        let roots = build_local_path_roots(&bridge_state, &settings_snapshot_state);
        local_path::open_local_path(&path, &roots)
    })
    .await
    .map_err(|error| format!("desktop local open task failed: {error}"))?
}

#[tauri::command]
async fn desktop_reveal_local_path(
    bridge_state: tauri::State<'_, Arc<NamedPipeBridgeState>>,
    settings_snapshot_state: tauri::State<'_, Arc<DesktopSettingsSnapshotState>>,
    path: String,
) -> Result<(), String> {
    let bridge_state = Arc::clone(bridge_state.inner());
    let settings_snapshot_state = Arc::clone(settings_snapshot_state.inner());
    tauri::async_runtime::spawn_blocking(move || {
        let roots = build_local_path_roots(&bridge_state, &settings_snapshot_state);
        local_path::reveal_local_path(&path, &roots)
    })
    .await
    .map_err(|error| format!("desktop local reveal task failed: {error}"))?
}

#[tauri::command]
async fn desktop_load_source_notes(
    bridge_state: tauri::State<'_, Arc<NamedPipeBridgeState>>,
    settings_snapshot_state: tauri::State<'_, Arc<DesktopSettingsSnapshotState>>,
    sources: Vec<String>,
) -> Result<source_notes::DesktopSourceNoteSnapshot, String> {
    let bridge_state = Arc::clone(bridge_state.inner());
    let settings_snapshot_state = Arc::clone(settings_snapshot_state.inner());
    tauri::async_runtime::spawn_blocking(move || {
        let trusted_sources =
            resolve_trusted_source_note_sources(&bridge_state, &settings_snapshot_state, &sources)?;
        let roots =
            build_source_note_roots(&bridge_state, &settings_snapshot_state, &trusted_sources);
        source_notes::load_source_notes(&trusted_sources, &roots)
    })
    .await
    .map_err(|error| format!("desktop source notes load task failed: {error}"))?
}

#[tauri::command]
async fn desktop_load_source_note_index(
    bridge_state: tauri::State<'_, Arc<NamedPipeBridgeState>>,
    settings_snapshot_state: tauri::State<'_, Arc<DesktopSettingsSnapshotState>>,
    sources: Vec<String>,
) -> Result<source_notes::DesktopSourceNoteIndexSnapshot, String> {
    let bridge_state = Arc::clone(bridge_state.inner());
    let settings_snapshot_state = Arc::clone(settings_snapshot_state.inner());
    tauri::async_runtime::spawn_blocking(move || {
        let trusted_sources =
            resolve_trusted_source_note_sources(&bridge_state, &settings_snapshot_state, &sources)?;
        let roots =
            build_source_note_roots(&bridge_state, &settings_snapshot_state, &trusted_sources);
        source_notes::load_source_note_index(&trusted_sources, &roots)
    })
    .await
    .map_err(|error| format!("desktop source note index load task failed: {error}"))?
}

#[tauri::command]
async fn desktop_create_source_note(
    bridge_state: tauri::State<'_, Arc<NamedPipeBridgeState>>,
    settings_snapshot_state: tauri::State<'_, Arc<DesktopSettingsSnapshotState>>,
    sources: Vec<String>,
    content: String,
) -> Result<source_notes::DesktopSourceNoteDocument, String> {
    let bridge_state = Arc::clone(bridge_state.inner());
    let settings_snapshot_state = Arc::clone(settings_snapshot_state.inner());
    tauri::async_runtime::spawn_blocking(move || {
        let trusted_sources =
            resolve_trusted_source_note_sources(&bridge_state, &settings_snapshot_state, &sources)?;
        let roots =
            build_source_note_roots(&bridge_state, &settings_snapshot_state, &trusted_sources);
        source_notes::create_source_note(&trusted_sources, &roots, &content)
    })
    .await
    .map_err(|error| format!("desktop source note create task failed: {error}"))?
}

#[tauri::command]
async fn desktop_save_source_note(
    bridge_state: tauri::State<'_, Arc<NamedPipeBridgeState>>,
    settings_snapshot_state: tauri::State<'_, Arc<DesktopSettingsSnapshotState>>,
    sources: Vec<String>,
    path: String,
    content: String,
) -> Result<source_notes::DesktopSourceNoteDocument, String> {
    let bridge_state = Arc::clone(bridge_state.inner());
    let settings_snapshot_state = Arc::clone(settings_snapshot_state.inner());
    tauri::async_runtime::spawn_blocking(move || {
        let trusted_sources =
            resolve_trusted_source_note_sources(&bridge_state, &settings_snapshot_state, &sources)?;
        let roots =
            build_source_note_roots(&bridge_state, &settings_snapshot_state, &trusted_sources);
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

fn build_local_path_roots(
    bridge_state: &Arc<NamedPipeBridgeState>,
    settings_snapshot_state: &Arc<DesktopSettingsSnapshotState>,
) -> local_path::LocalPathRoots {
    let repo_root = build_repo_root();
    let workspace_root = resolve_workspace_root(
        bridge_state,
        settings_snapshot_state,
        repo_root.as_ref(),
        true,
    )
    .or_else(|| repo_root.as_ref().map(|root| root.join("workspace")));

    local_path::LocalPathRoots::new(workspace_root, repo_root)
}

fn build_source_note_roots(
    bridge_state: &Arc<NamedPipeBridgeState>,
    settings_snapshot_state: &Arc<DesktopSettingsSnapshotState>,
    sources: &[String],
) -> local_path::LocalPathRoots {
    let repo_root = build_repo_root();
    let workspace_root = if source_notes::sources_require_workspace_root(sources) {
        resolve_workspace_root(
            bridge_state,
            settings_snapshot_state,
            repo_root.as_ref(),
            true,
        )
        .or_else(|| repo_root.as_ref().map(|root| root.join("workspace")))
    } else {
        None
    };

    local_path::LocalPathRoots::new(workspace_root, repo_root)
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

fn build_repo_root() -> Option<PathBuf> {
    PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .join("../..")
        .canonicalize()
        .ok()
}

fn resolve_workspace_root_from_snapshot(
    settings_snapshot_state: &Arc<DesktopSettingsSnapshotState>,
    repo_root: Option<&PathBuf>,
) -> Option<PathBuf> {
    settings_snapshot_state
        .workspace_root()
        .ok()
        .flatten()
        .and_then(|workspace_root| {
            if workspace_root.is_absolute() {
                Some(workspace_root)
            } else {
                repo_root.map(|root| root.join(workspace_root))
            }
        })
}

/// The desktop host prefers the cached settings snapshot, but it may still be
/// empty during the first few local actions if startup prefetch raced the pipe.
/// In that case, perform one bounded refresh and keep the result for later calls.
fn resolve_workspace_root(
    bridge_state: &Arc<NamedPipeBridgeState>,
    settings_snapshot_state: &Arc<DesktopSettingsSnapshotState>,
    repo_root: Option<&PathBuf>,
    allow_refresh: bool,
) -> Option<PathBuf> {
    let workspace_root = resolve_workspace_root_from_snapshot(settings_snapshot_state, repo_root);
    if workspace_root.is_some() || !allow_refresh {
        return workspace_root;
    }

    let should_refresh = settings_snapshot_state.has_snapshot().ok() == Some(false);
    if should_refresh {
        let _ = seed_desktop_settings_snapshot(
            bridge_state,
            settings_snapshot_state,
            Duration::from_millis(DESKTOP_SETTINGS_REQUEST_TIMEOUT_MS),
        );
    }

    resolve_workspace_root_from_snapshot(settings_snapshot_state, repo_root)
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
        read_task_sources_from_settings_snapshot, read_trusted_source_note_sources,
        read_workspace_root_from_settings_snapshot, source_note_sources_drift,
        DesktopSettingsSnapshotState,
    };
    use serde_json::json;
    use std::env;

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

#[tauri::command]
fn desktop_recreate_onboarding(
    app: tauri::AppHandle,
    url: String,
    x: f64,
    y: f64,
    width: f64,
    height: f64,
) -> Result<(), String> {
    if let Some(window) = app.get_webview_window(ONBOARDING_WINDOW_LABEL) {
        window
            .destroy()
            .map_err(|error| format!("failed to destroy existing onboarding window: {error}"))?;
    }

    let window = WebviewWindowBuilder::new(
        &app,
        ONBOARDING_WINDOW_LABEL,
        WebviewUrl::App(url.into()),
    )
    .title("CialloClaw Onboarding")
    .inner_size(width, height)
    .position(x, y)
    .decorations(false)
    .transparent(true)
    .always_on_top(true)
    .resizable(false)
    .skip_taskbar(true)
    .shadow(false)
    .visible(true)
    .focused(true)
    .build()
    .map_err(|error| format!("failed to recreate onboarding window: {error}"))?;

    if let Ok(hwnd) = window.hwnd() {
        unsafe {
            // The recreated guide is a normal card-sized window, not a
            // fullscreen pass-through overlay.
            set_forward_mouse_messages(hwnd, false);
            set_window_ignore_cursor_events(hwnd, false);
        }
    }

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
fn onboarding_set_ignore_cursor_events(_app: tauri::AppHandle, _ignore: bool) -> Result<(), String> {
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
    tauri::Builder::default()
        .manage(Arc::new(NamedPipeBridgeState::default()))
        .manage(Arc::new(DesktopSettingsSnapshotState::default()))
        .plugin(tauri_plugin_clipboard_manager::init())
        .setup(|app| {
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
            desktop_recreate_onboarding,
            desktop_promote_onboarding,
            desktop_open_local_path,
            desktop_reveal_local_path,
            desktop_sync_settings_snapshot,
            desktop_load_source_notes,
            desktop_load_source_note_index,
            desktop_create_source_note,
            desktop_save_source_note,
            pick_shell_ball_files,
            shell_ball_apply_window_frame,
            shell_ball_read_selection_snapshot
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
