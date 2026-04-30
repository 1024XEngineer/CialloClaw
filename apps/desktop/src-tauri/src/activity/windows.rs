use super::types::MouseActivitySnapshotPayload;
use once_cell::sync::Lazy;
use std::sync::Mutex;
use windows::Win32::Foundation::{LPARAM, LRESULT, WPARAM};
use windows::Win32::UI::WindowsAndMessaging::{
    CallNextHookEx, MSLLHOOKSTRUCT, SetWindowsHookExW, WH_MOUSE_LL, WM_LBUTTONDOWN,
    WM_LBUTTONUP, WM_MBUTTONDOWN, WM_MBUTTONUP, WM_MOUSEMOVE, WM_MOUSEWHEEL,
    WM_RBUTTONDOWN, WM_RBUTTONUP,
};

static MOUSE_ACTIVITY_HOOK: Lazy<Mutex<Option<isize>>> = Lazy::new(|| Mutex::new(None));
static LAST_MOUSE_ACTIVITY: Lazy<Mutex<Option<MouseActivitySnapshotPayload>>> =
    Lazy::new(|| Mutex::new(None));
static RECENT_MOUSE_POINTS: Lazy<Mutex<Vec<(i32, i32)>>> = Lazy::new(|| Mutex::new(Vec::new()));
const MAX_RECENT_MOUSE_POINTS: usize = 8;

/// Installs the Windows low-level mouse hook used to track the latest mouse
/// activity timestamp.
pub fn install_mouse_activity_listener() -> Result<(), String> {
    let mut hook = MOUSE_ACTIVITY_HOOK
        .lock()
        .map_err(|_| "mouse activity hook lock poisoned".to_string())?;

    if hook.is_some() {
        return Ok(());
    }

    unsafe {
        *hook = Some(
            SetWindowsHookExW(WH_MOUSE_LL, Some(mouse_activity_hook), None, 0)
                .map_err(|error| format!("failed to install mouse activity hook: {error}"))?
                .0 as isize,
        );
    }

    Ok(())
}

/// Returns the latest mouse activity snapshot tracked by the host hook.
pub fn read_mouse_activity_snapshot() -> Option<MouseActivitySnapshotPayload> {
    LAST_MOUSE_ACTIVITY
        .lock()
        .ok()
        .and_then(|snapshot| snapshot.clone())
}

/// Returns the latest native mouse points in newest-first order so nearby
/// desktop interactions can recover a recent external hover target.
pub fn read_recent_mouse_points() -> Vec<(i32, i32)> {
    RECENT_MOUSE_POINTS
        .lock()
        .map(|points| points.clone())
        .unwrap_or_default()
}

unsafe extern "system" fn mouse_activity_hook(
    n_code: i32,
    w_param: WPARAM,
    l_param: LPARAM,
) -> LRESULT {
    if n_code >= 0 && should_record_mouse_activity(w_param.0 as u32) {
        let mouse_info = (l_param.0 as *const MSLLHOOKSTRUCT).as_ref();
        let cursor_position = mouse_info.map(|value| (value.pt.x, value.pt.y));
        let snapshot = MouseActivitySnapshotPayload::now(
            cursor_position.map(|(x, _)| x),
            cursor_position.map(|(_, y)| y),
        );

        if let Ok(mut state) = LAST_MOUSE_ACTIVITY.lock() {
            *state = Some(snapshot);
        }

        if let Some((cursor_x, cursor_y)) = cursor_position {
            record_recent_mouse_point(cursor_x, cursor_y);
        }
    }

    CallNextHookEx(None, n_code, w_param, l_param)
}

fn record_recent_mouse_point(cursor_x: i32, cursor_y: i32) {
    if let Ok(mut points) = RECENT_MOUSE_POINTS.lock() {
        if points
            .first()
            .is_some_and(|(existing_x, existing_y)| *existing_x == cursor_x && *existing_y == cursor_y)
        {
            return;
        }

        points.insert(0, (cursor_x, cursor_y));
        if points.len() > MAX_RECENT_MOUSE_POINTS {
            points.truncate(MAX_RECENT_MOUSE_POINTS);
        }
    }
}

fn should_record_mouse_activity(message: u32) -> bool {
    matches!(
        message,
        WM_MOUSEMOVE
            | WM_LBUTTONDOWN
            | WM_LBUTTONUP
            | WM_RBUTTONDOWN
            | WM_RBUTTONUP
            | WM_MBUTTONDOWN
            | WM_MBUTTONUP
            | WM_MOUSEWHEEL
    )
}
