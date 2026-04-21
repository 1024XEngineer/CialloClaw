use std::env;
use std::path::{Path, PathBuf};
use std::process::Command;

#[cfg(windows)]
use std::ffi::OsStr;

#[cfg(windows)]
use std::os::windows::ffi::OsStrExt;

#[cfg(windows)]
use windows::core::PCWSTR;

#[cfg(windows)]
use windows::Win32::UI::Shell::ShellExecuteW;

#[cfg(windows)]
use windows::Win32::UI::WindowsAndMessaging::SW_SHOWNORMAL;

/// Opens a local file or directory through the operating system shell.
pub fn open_local_path(raw_path: &str) -> Result<(), String> {
    let target = resolve_existing_local_path(raw_path)?;
    open_with_system_handler(&target)
}

/// Reveals a local file in the system file manager, or opens the directory
/// directly when the target already points at a folder.
pub fn reveal_local_path(raw_path: &str) -> Result<(), String> {
    let target = resolve_existing_local_path(raw_path)?;

    if target.is_dir() {
        return open_with_system_handler(&target);
    }

    reveal_with_system_handler(&target)
}

/// Resolves renderer-provided local paths against the current working
/// directory so both absolute artifact targets and relative workspace paths can
/// be handled by the desktop host.
fn resolve_existing_local_path(raw_path: &str) -> Result<PathBuf, String> {
    let current_dir =
        env::current_dir().map_err(|error| format!("failed to read current directory: {error}"))?;
    let candidate = resolve_path_candidate(raw_path, &current_dir)?;

    if !candidate.exists() {
        return Err(format!("local target does not exist: {}", candidate.display()));
    }

    candidate
        .canonicalize()
        .map_err(|error| format!("failed to canonicalize local target {}: {error}", candidate.display()))
}

fn resolve_path_candidate(raw_path: &str, current_dir: &Path) -> Result<PathBuf, String> {
    let trimmed = raw_path.trim();
    if trimmed.is_empty() {
        return Err("local target path is empty".to_string());
    }

    let candidate = PathBuf::from(trimmed);
    if candidate.is_absolute() {
        return Ok(candidate);
    }

    Ok(current_dir.join(candidate))
}

#[cfg(windows)]
fn open_with_system_handler(target: &Path) -> Result<(), String> {
    let operation = encode_wide(OsStr::new("open"));
    let target_wide = encode_wide(target.as_os_str());
    let result = unsafe {
        ShellExecuteW(
            None,
            PCWSTR(operation.as_ptr()),
            PCWSTR(target_wide.as_ptr()),
            PCWSTR::null(),
            PCWSTR::null(),
            SW_SHOWNORMAL,
        )
    };

    let code = result.0 as isize;
    if code <= 32 {
        return Err(format!("shell open failed with code {code}"));
    }

    Ok(())
}

#[cfg(windows)]
fn reveal_with_system_handler(target: &Path) -> Result<(), String> {
    let select_arg = format!("/select,{}", target.display());
    run_platform_command(
        "explorer.exe",
        &[select_arg.as_str()],
        &format!("reveal local target {}", target.display()),
    )
}

#[cfg(windows)]
fn encode_wide(value: &OsStr) -> Vec<u16> {
    value.encode_wide().chain(Some(0)).collect()
}

#[cfg(target_os = "macos")]
fn open_with_system_handler(target: &Path) -> Result<(), String> {
    run_platform_command("open", &[target], &format!("open local target {}", target.display()))
}

#[cfg(target_os = "macos")]
fn reveal_with_system_handler(target: &Path) -> Result<(), String> {
    run_platform_command(
        "open",
        &[Path::new("-R"), target],
        &format!("reveal local target {}", target.display()),
    )
}

#[cfg(all(not(windows), not(target_os = "macos")))]
fn open_with_system_handler(target: &Path) -> Result<(), String> {
    run_platform_command(
        "xdg-open",
        &[target],
        &format!("open local target {}", target.display()),
    )
}

#[cfg(all(not(windows), not(target_os = "macos")))]
fn reveal_with_system_handler(target: &Path) -> Result<(), String> {
    let parent = target.parent().unwrap_or(target);
    run_platform_command(
        "xdg-open",
        &[parent],
        &format!("reveal local target {}", target.display()),
    )
}

#[cfg(windows)]
fn run_platform_command(program: &str, args: &[&str], description: &str) -> Result<(), String> {
    let status = Command::new(program)
        .args(args)
        .status()
        .map_err(|error| format!("failed to {description}: {error}"))?;

    if !status.success() {
        return Err(format!("failed to {description}: exit status {status}"));
    }

    Ok(())
}

#[cfg(not(windows))]
fn run_platform_command(program: &str, args: &[&Path], description: &str) -> Result<(), String> {
    let status = Command::new(program)
        .args(args)
        .status()
        .map_err(|error| format!("failed to {description}: {error}"))?;

    if !status.success() {
        return Err(format!("failed to {description}: exit status {status}"));
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::{resolve_existing_local_path, resolve_path_candidate};
    use std::env;
    use std::fs;
    use std::path::PathBuf;
    use std::time::{SystemTime, UNIX_EPOCH};

    #[test]
    fn resolve_path_candidate_rejects_empty_input() {
        let current_dir = env::current_dir().expect("read current directory");
        assert!(resolve_path_candidate("   ", &current_dir).is_err());
    }

    #[test]
    fn resolve_path_candidate_joins_relative_paths_against_current_dir() {
        let current_dir = PathBuf::from("C:/workspace");
        let resolved = resolve_path_candidate("artifacts/result.docx", &current_dir)
            .expect("resolve relative path");

        assert_eq!(resolved, current_dir.join("artifacts/result.docx"));
    }

    #[test]
    fn resolve_existing_local_path_accepts_existing_targets() {
        let target = unique_temp_path("existing-target.txt");
        fs::write(&target, "artifact").expect("write temp target");

        let resolved = resolve_existing_local_path(target.to_string_lossy().as_ref())
            .expect("resolve existing target");

        assert!(resolved.is_absolute());
        assert!(resolved.exists());

        let _ = fs::remove_file(target);
    }

    #[test]
    fn resolve_existing_local_path_rejects_missing_targets() {
        let target = unique_temp_path("missing-target.txt");
        let error = resolve_existing_local_path(target.to_string_lossy().as_ref())
            .expect_err("missing target should fail");

        assert!(error.contains("does not exist"));
    }

    fn unique_temp_path(name: &str) -> PathBuf {
        let unique = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("read system time")
            .as_nanos();

        env::temp_dir().join(format!("cialloclaw-desktop-{unique}-{name}"))
    }
}
