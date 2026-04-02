use serde::Serialize;
use std::{
    path::{Path, PathBuf},
    process::{Child, Command, Stdio},
    sync::Mutex,
};
use tauri::window::Color;
use tauri::{
    menu::MenuBuilder, tray::TrayIconBuilder, AppHandle, Emitter, Manager, PhysicalPosition,
    Position, WebviewUrl, WebviewWindowBuilder, WindowEvent,
};

const ORB_LABEL: &str = "orb";
const CHAT_LABEL: &str = "chat";
const SETTINGS_LABEL: &str = "settings";
const NUDGE_LABEL: &str = "nudge";
const SIDECAR_PORT: u16 = 47831;

const MENU_RESTORE: &str = "restore";
const MENU_SETTINGS: &str = "settings";
const MENU_HIDE: &str = "hide";
const MENU_QUIT: &str = "quit";

struct RuntimeState {
    sidecar: Mutex<Option<Child>>,
    motion: Mutex<WindowMotionState>,
    orb_snap_guard: Mutex<bool>,
}

impl Default for RuntimeState {
    fn default() -> Self {
        Self {
            sidecar: Mutex::new(None),
            motion: Mutex::new(WindowMotionState::default()),
            orb_snap_guard: Mutex::new(false),
        }
    }
}

#[derive(Debug, Serialize, Clone, Copy)]
#[serde(rename_all = "camelCase")]
struct WindowMotionState {
    orb_x: i32,
    orb_y: i32,
    orb_width: u32,
    orb_height: u32,
    edge: OrbEdge,
    chat_x: i32,
    chat_y: i32,
    nudge_x: i32,
    nudge_y: i32,
    cue: MotionCue,
}

impl Default for WindowMotionState {
    fn default() -> Self {
        Self {
            orb_x: 24,
            orb_y: 140,
            orb_width: 78,
            orb_height: 78,
            edge: OrbEdge::Right,
            chat_x: 0,
            chat_y: 0,
            nudge_x: 0,
            nudge_y: 0,
            cue: MotionCue::Idle,
        }
    }
}

#[derive(Debug, Serialize, Clone, Copy)]
#[serde(rename_all = "camelCase")]
enum OrbEdge {
    Left,
    Right,
}

#[derive(Debug, Serialize, Clone, Copy)]
#[serde(rename_all = "kebab-case")]
enum MotionCue {
    Idle,
    Snap,
    ChatBloom,
    NudgeBloom,
}

#[derive(Debug, Serialize, Clone)]
#[serde(rename_all = "camelCase")]
struct SidecarInfo {
    base_url: String,
    source: &'static str,
}

fn base_url() -> String {
    format!("http://127.0.0.1:{SIDECAR_PORT}")
}

fn emit_motion(app: &AppHandle, cue: MotionCue) {
    let payload = {
        let runtime = app.state::<RuntimeState>();
        let mut state = runtime.motion.lock().expect("motion lock poisoned");
        state.cue = cue;
        *state
    };

    let _ = app.emit("window-motion", payload);
}

fn clamp_i32(value: i32, min: i32, max: i32) -> i32 {
    value.max(min).min(max)
}

fn position_linked_windows(state: &mut WindowMotionState, monitor: &tauri::window::Monitor) {
    let monitor_pos = monitor.position();
    let monitor_size = monitor.size();
    let screen_left = monitor_pos.x;
    let screen_top = monitor_pos.y;
    let screen_right = screen_left + monitor_size.width as i32;
    let screen_bottom = screen_top + monitor_size.height as i32;

    let chat_width = 460;
    let chat_height = 760;
    let nudge_width = 320;
    let nudge_height = 180;
    let gap = 18;

    state.chat_x = match state.edge {
        OrbEdge::Left => state.orb_x + state.orb_width as i32 + gap,
        OrbEdge::Right => state.orb_x - chat_width - gap,
    };
    state.chat_y = clamp_i32(
        state.orb_y - 90,
        screen_top + 26,
        screen_bottom - chat_height - 26,
    );

    state.nudge_x = match state.edge {
        OrbEdge::Left => state.orb_x + state.orb_width as i32 + 6,
        OrbEdge::Right => state.orb_x - nudge_width - 6,
    };
    state.nudge_y = clamp_i32(
        state.orb_y + (state.orb_height as i32 / 2) - (nudge_height / 2),
        screen_top + 20,
        screen_bottom - nudge_height - 20,
    );

    state.chat_x = clamp_i32(
        state.chat_x,
        screen_left + 20,
        screen_right - chat_width - 20,
    );
    state.nudge_x = clamp_i32(
        state.nudge_x,
        screen_left + 20,
        screen_right - nudge_width - 20,
    );
}

fn sync_orb_motion(app: &AppHandle, cue: MotionCue) {
    let Some(orb) = app.get_webview_window(ORB_LABEL) else {
        return;
    };
    let Ok(Some(monitor)) = orb.current_monitor() else {
        return;
    };
    let Ok(position) = orb.outer_position() else {
        return;
    };
    let Ok(size) = orb.outer_size() else {
        return;
    };

    let monitor_pos = monitor.position();
    let monitor_size = monitor.size();
    let screen_left = monitor_pos.x;
    let screen_top = monitor_pos.y;
    let screen_right = screen_left + monitor_size.width as i32;
    let screen_bottom = screen_top + monitor_size.height as i32;

    let threshold = 28;
    let margin = 14;
    let left_gap = position.x - screen_left;
    let right_gap = screen_right - (position.x + size.width as i32);

    let mut snapped_position = position;
    let edge = if left_gap <= right_gap {
        OrbEdge::Left
    } else {
        OrbEdge::Right
    };
    match edge {
        OrbEdge::Left if left_gap <= threshold => {
            snapped_position.x = screen_left + margin;
        }
        OrbEdge::Right if right_gap <= threshold => {
            snapped_position.x = screen_right - size.width as i32 - margin;
        }
        _ => {}
    }

    snapped_position.y = clamp_i32(
        snapped_position.y,
        screen_top + margin,
        screen_bottom - size.height as i32 - margin,
    );

    if snapped_position != position {
        let runtime = app.state::<RuntimeState>();
        let mut guard = runtime
            .orb_snap_guard
            .lock()
            .expect("orb snap guard lock poisoned");
        if !*guard {
            *guard = true;
            let _ = orb.set_position(Position::Physical(snapped_position));
        }
    }

    {
        let runtime = app.state::<RuntimeState>();
        let mut state = runtime.motion.lock().expect("motion lock poisoned");
        state.orb_x = snapped_position.x;
        state.orb_y = snapped_position.y;
        state.orb_width = size.width;
        state.orb_height = size.height;
        state.edge = edge;
        position_linked_windows(&mut state, &monitor);
    }

    emit_motion(app, cue);
}

fn place_window(app: &AppHandle, label: &str, x: i32, y: i32) {
    if let Some(window) = app.get_webview_window(label) {
        let _ = window.set_position(Position::Physical(PhysicalPosition::new(x, y)));
    }
}

fn place_linked_window(app: &AppHandle, label: &str) {
    let state = *app
        .state::<RuntimeState>()
        .motion
        .lock()
        .expect("motion lock poisoned");
    match label {
        CHAT_LABEL => place_window(app, CHAT_LABEL, state.chat_x, state.chat_y),
        NUDGE_LABEL => place_window(app, NUDGE_LABEL, state.nudge_x, state.nudge_y),
        _ => {}
    }
}

fn show_window(app: &AppHandle, label: &str) {
    if let Some(window) = app.get_webview_window(label) {
        let _ = window.show();
        let _ = window.set_focus();
    }
}

fn hide_window(app: &AppHandle, label: &str) {
    if let Some(window) = app.get_webview_window(label) {
        let _ = window.hide();
    }
}

fn hide_application(app: &AppHandle) {
    hide_window(app, ORB_LABEL);
    hide_window(app, CHAT_LABEL);
    hide_window(app, SETTINGS_LABEL);
    hide_window(app, NUDGE_LABEL);
}

fn restore_application(app: &AppHandle) {
    show_window(app, ORB_LABEL);
    show_window(app, CHAT_LABEL);
}

fn quit_application(app: &AppHandle) {
    if let Some(mut child) = app
        .state::<RuntimeState>()
        .sidecar
        .lock()
        .ok()
        .and_then(|mut guard| guard.take())
    {
        let _ = child.kill();
        let _ = child.wait();
    }

    for label in [ORB_LABEL, CHAT_LABEL, SETTINGS_LABEL, NUDGE_LABEL] {
        if let Some(window) = app.get_webview_window(label) {
            let _ = window.destroy();
        }
    }

    app.exit(0);
}

fn orb_menu(app: &AppHandle) -> tauri::Result<tauri::menu::Menu<tauri::Wry>> {
    MenuBuilder::new(app)
        .text(MENU_SETTINGS, "设置")
        .text(MENU_HIDE, "隐藏")
        .separator()
        .text(MENU_QUIT, "退出")
        .build()
}

fn tray_menu(app: &AppHandle) -> tauri::Result<tauri::menu::Menu<tauri::Wry>> {
    MenuBuilder::new(app)
        .text(MENU_RESTORE, "显示/恢复")
        .text(MENU_SETTINGS, "设置")
        .separator()
        .text(MENU_HIDE, "隐藏")
        .text(MENU_QUIT, "退出")
        .build()
}

fn register_window_behavior(window: &tauri::WebviewWindow) {
    let label = window.label().to_string();
    let handle = window.app_handle().clone();

    window.on_window_event(move |event| match event {
        WindowEvent::CloseRequested { api, .. } => {
            api.prevent_close();
            match label.as_str() {
                ORB_LABEL => {
                    show_window(&handle, ORB_LABEL);
                }
                CHAT_LABEL | SETTINGS_LABEL | NUDGE_LABEL => {
                    hide_window(&handle, &label);
                }
                _ => {}
            }
        }
        WindowEvent::Moved(_) if label == ORB_LABEL => {
            let runtime = handle.state::<RuntimeState>();
            let mut guard = runtime
                .orb_snap_guard
                .lock()
                .expect("orb snap guard lock poisoned");
            if *guard {
                *guard = false;
            } else {
                drop(guard);
                sync_orb_motion(&handle, MotionCue::Snap);
            }
        }
        _ => {}
    });
}

fn create_window(
    app: &AppHandle,
    label: &str,
    title: &str,
    route: &str,
    width: f64,
    height: f64,
    decorations: bool,
    transparent: bool,
    always_on_top: bool,
    visible: bool,
    resizable: bool,
    skip_taskbar: bool,
) -> tauri::Result<tauri::WebviewWindow> {
    let mut builder = WebviewWindowBuilder::new(app, label, WebviewUrl::App(route.into()))
        .title(title)
        .inner_size(width, height)
        .decorations(decorations)
        .transparent(transparent)
        .always_on_top(always_on_top)
        .visible(visible)
        .resizable(resizable)
        .skip_taskbar(skip_taskbar);

    if transparent {
        builder = builder.shadow(false).background_color(Color(0, 0, 0, 0));
    }

    let window = builder.build()?;

    register_window_behavior(&window);
    Ok(window)
}

fn ensure_windows(app: &AppHandle) -> tauri::Result<()> {
    if app.get_webview_window(ORB_LABEL).is_none() {
        let _orb = create_window(
            app,
            ORB_LABEL,
            "CialloClaw",
            "/orb",
            78.0,
            78.0,
            false,
            true,
            true,
            true,
            false,
            true,
        )?;
    }

    if app.get_webview_window(CHAT_LABEL).is_none() {
        let _chat = create_window(
            app,
            CHAT_LABEL,
            "CialloClaw",
            "/chat",
            460.0,
            760.0,
            true,
            false,
            false,
            false,
            true,
            false,
        )?;
    }

    if app.get_webview_window(SETTINGS_LABEL).is_none() {
        let _settings = create_window(
            app,
            SETTINGS_LABEL,
            "CialloClaw 设置",
            "/settings",
            420.0,
            720.0,
            true,
            false,
            false,
            false,
            true,
            false,
        )?;
    }

    if app.get_webview_window(NUDGE_LABEL).is_none() {
        let _nudge = create_window(
            app,
            NUDGE_LABEL,
            "CialloClaw 提示",
            "/nudge",
            320.0,
            180.0,
            false,
            true,
            true,
            false,
            false,
            true,
        )?;
    }

    Ok(())
}

fn sidecar_binary_candidates(manifest_dir: &Path) -> Vec<PathBuf> {
    let root = manifest_dir.parent().unwrap_or(manifest_dir);
    let exe_name = if cfg!(target_os = "windows") {
        "cialloclaw-sidecar.exe"
    } else {
        "cialloclaw-sidecar"
    };

    vec![
        root.join("go-backend").join("bin").join(exe_name),
        manifest_dir.join("binaries").join(exe_name),
    ]
}

fn build_sidecar_if_missing(manifest_dir: &Path) {
    let root = manifest_dir.parent().unwrap_or(manifest_dir);
    let bin_dir = root.join("go-backend").join("bin");
    let _ = std::fs::create_dir_all(&bin_dir);
    let output = if cfg!(target_os = "windows") {
        bin_dir.join("cialloclaw-sidecar.exe")
    } else {
        bin_dir.join("cialloclaw-sidecar")
    };

    if output.exists() {
        return;
    }

    let _ = Command::new("go")
        .args([
            "build",
            "-o",
            output.to_string_lossy().as_ref(),
            "./cmd/sidecar",
        ])
        .current_dir(root.join("go-backend"))
        .stdout(Stdio::null())
        .stderr(Stdio::null())
        .status();
}

fn start_sidecar(app: &AppHandle) {
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    build_sidecar_if_missing(&manifest_dir);

    let candidates = sidecar_binary_candidates(&manifest_dir);
    for path in candidates {
        if path.exists() {
            if let Ok(child) = Command::new(path)
                .env("CIALLOCLAW_PORT", SIDECAR_PORT.to_string())
                .stdout(Stdio::null())
                .stderr(Stdio::null())
                .spawn()
            {
                if let Ok(mut guard) = app.state::<RuntimeState>().sidecar.lock() {
                    *guard = Some(child);
                }
                break;
            }
        }
    }
}

#[tauri::command]
fn get_sidecar_info() -> SidecarInfo {
    SidecarInfo {
        base_url: base_url(),
        source: "go-sidecar",
    }
}

#[tauri::command]
fn get_motion_state(app: AppHandle) -> WindowMotionState {
    *app.state::<RuntimeState>()
        .motion
        .lock()
        .expect("motion lock poisoned")
}

#[tauri::command]
fn toggle_chat(app: AppHandle) {
    sync_orb_motion(&app, MotionCue::Idle);
    if let Some(window) = app.get_webview_window(CHAT_LABEL) {
        if window.is_visible().unwrap_or(false) {
            let _ = window.hide();
            emit_motion(&app, MotionCue::Idle);
        } else {
            place_linked_window(&app, CHAT_LABEL);
            let _ = window.show();
            let _ = window.set_focus();
            emit_motion(&app, MotionCue::ChatBloom);
        }
    }
}

#[tauri::command]
fn show_settings(app: AppHandle) {
    sync_orb_motion(&app, MotionCue::Idle);
    show_window(&app, SETTINGS_LABEL);
}

#[tauri::command]
fn hide_settings(app: AppHandle) {
    hide_window(&app, SETTINGS_LABEL);
}

#[tauri::command]
fn hide_app(app: AppHandle) {
    hide_application(&app);
}

#[tauri::command]
fn restore_app(app: AppHandle) {
    restore_application(&app);
}

#[tauri::command]
fn quit_app(app: AppHandle) {
    quit_application(&app);
}

#[tauri::command]
fn show_nudge(app: AppHandle) {
    sync_orb_motion(&app, MotionCue::Idle);
    place_linked_window(&app, NUDGE_LABEL);
    show_window(&app, NUDGE_LABEL);
    emit_motion(&app, MotionCue::NudgeBloom);
}

#[tauri::command]
fn hide_nudge(app: AppHandle) {
    hide_window(&app, NUDGE_LABEL);
    emit_motion(&app, MotionCue::Idle);
}

#[tauri::command]
fn focus_chat(app: AppHandle) {
    sync_orb_motion(&app, MotionCue::Idle);
    place_linked_window(&app, CHAT_LABEL);
    show_window(&app, CHAT_LABEL);
    emit_motion(&app, MotionCue::ChatBloom);
}

#[tauri::command]
fn show_orb_menu(app: AppHandle) -> Result<(), String> {
    let Some(window) = app.get_webview_window(ORB_LABEL) else {
        return Err("orb window missing".into());
    };

    let menu = orb_menu(&app).map_err(|err| err.to_string())?;
    window.popup_menu(&menu).map_err(|err| err.to_string())
}

fn main() {
    tauri::Builder::default()
        .manage(RuntimeState::default())
        .on_menu_event(|app, event| match event.id().0.as_str() {
            MENU_RESTORE => restore_application(app),
            MENU_SETTINGS => show_window(app, SETTINGS_LABEL),
            MENU_HIDE => hide_application(app),
            MENU_QUIT => quit_application(app),
            _ => {}
        })
        .setup(|app| {
            ensure_windows(app.handle())?;
            start_sidecar(app.handle());
            sync_orb_motion(app.handle(), MotionCue::Idle);

            let menu = tray_menu(app.handle())?;
            let mut tray_builder = TrayIconBuilder::with_id("main")
                .menu(&menu)
                .tooltip("CialloClaw");
            if let Some(icon) = app.default_window_icon() {
                tray_builder = tray_builder.icon(icon.clone());
            }
            let _tray = tray_builder.build(app)?;

            if let Some(orb) = app.get_webview_window(ORB_LABEL) {
                let _ = orb.emit("sidecar:booted", get_sidecar_info());
            }

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            get_sidecar_info,
            get_motion_state,
            toggle_chat,
            show_settings,
            hide_settings,
            hide_app,
            restore_app,
            quit_app,
            show_nudge,
            hide_nudge,
            focus_chat,
            show_orb_menu,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
