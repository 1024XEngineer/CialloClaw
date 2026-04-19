use super::types::MouseActivitySnapshotPayload;
use once_cell::sync::Lazy;
use std::sync::Mutex;
use windows::Win32::Foundation::{LPARAM, LRESULT, WPARAM};
use windows::Win32::UI::WindowsAndMessaging::{
    CallNextHookEx, SetWindowsHookExW, WH_MOUSE_LL, WM_LBUTTONDOWN, WM_LBUTTONUP,
    WM_MBUTTONDOWN, WM_MBUTTONUP, WM_MOUSEMOVE, WM_MOUSEWHEEL, WM_RBUTTONDOWN,
    WM_RBUTTONUP,
};

static MOUSE_ACTIVITY_HOOK: Lazy<Mutex<Option<isize>>> = Lazy::new(|| Mutex::new(None));
static LAST_MOUSE_ACTIVITY: Lazy<Mutex<Option<MouseActivitySnapshotPayload>>> =
    Lazy::new(|| Mutex::new(None));

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

unsafe extern "system" fn mouse_activity_hook(
    n_code: i32,
    w_param: WPARAM,
    l_param: LPARAM,
) -> LRESULT {
    if n_code >= 0 && should_record_mouse_activity(w_param.0 as u32) {
      let snapshot = MouseActivitySnapshotPayload::now();

      if let Ok(mut state) = LAST_MOUSE_ACTIVITY.lock() {
          *state = Some(snapshot.clone());
      }

      println!("mouse activity at {}", snapshot.updated_at);
    }

    CallNextHookEx(None, n_code, w_param, l_param)
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
