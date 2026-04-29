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

/// Trusted local path roots that the desktop host may open on behalf of the
/// renderer after formal delivery resolution.
pub struct LocalPathRoots {
    repo_root: Option<PathBuf>,
    workspace_root: Option<PathBuf>,
}

impl LocalPathRoots {
    /// Builds the local path roots from trusted host-side configuration.
    pub fn new(workspace_root: Option<PathBuf>, repo_root: Option<PathBuf>) -> Self {
        Self {
            repo_root: canonicalize_root(repo_root),
            workspace_root: canonicalize_root(workspace_root),
        }
    }

    /// Returns the trusted workspace root used for workspace-relative desktop
    /// path resolution.
    pub(crate) fn workspace_root(&self) -> Option<&PathBuf> {
        self.workspace_root.as_ref()
    }

    /// Returns the trusted repository root used for repo-relative desktop path
    /// resolution.
    pub(crate) fn repo_root(&self) -> Option<&PathBuf> {
        self.repo_root.as_ref()
    }
}

/// Opens a local file or directory through the operating system shell.
pub fn open_local_path(raw_path: &str, roots: &LocalPathRoots) -> Result<(), String> {
    let target = resolve_existing_local_path(raw_path, roots)?;
    open_with_system_handler(&target)
}

/// Reveals a local file in the system file manager, or opens the directory
/// directly when the target already points at a folder.
pub fn reveal_local_path(raw_path: &str, roots: &LocalPathRoots) -> Result<(), String> {
    let target = resolve_existing_local_path(raw_path, roots)?;

    if target.is_dir() {
        return open_with_system_handler(&target);
    }

    reveal_with_system_handler(&target)
}

/// Resolves delivery paths against trusted workspace or repository roots and
/// rejects any target that escapes those formal desktop-open scopes.
fn resolve_existing_local_path(raw_path: &str, roots: &LocalPathRoots) -> Result<PathBuf, String> {
    let candidate = resolve_path_candidate(raw_path, roots)?;

    if !candidate.exists() {
        return Err(format!(
            "local target does not exist: {}",
            candidate.display()
        ));
    }

    let canonical_target = candidate.canonicalize().map_err(|error| {
        format!(
            "failed to canonicalize local target {}: {error}",
            candidate.display()
        )
    })?;

    ensure_path_within_allowed_roots(&canonical_target, roots)?;

    Ok(canonical_target)
}

fn resolve_path_candidate(raw_path: &str, roots: &LocalPathRoots) -> Result<PathBuf, String> {
    let trimmed = raw_path.trim();
    if trimmed.is_empty() {
        return Err("local target path is empty".to_string());
    }

    let candidate = PathBuf::from(trimmed);
    if candidate.is_absolute() {
        return Ok(candidate);
    }

    if let Some(workspace_relative_path) = strip_workspace_prefix(trimmed) {
        if let Some(workspace_root) = roots.workspace_root.as_ref() {
            return Ok(workspace_root.join(workspace_relative_path));
        }

        return Err(
            "workspace root is not available for workspace-relative delivery paths".to_string(),
        );
    }

    if let Some(repo_root) = roots.repo_root.as_ref() {
        return Ok(repo_root.join(candidate));
    }

    Err("repository root is not available for repo-relative delivery paths".to_string())
}

fn strip_workspace_prefix(raw_path: &str) -> Option<&str> {
    if raw_path == "workspace" {
        return Some("");
    }

    raw_path
        .strip_prefix("workspace/")
        .or_else(|| raw_path.strip_prefix("workspace\\"))
}

fn canonicalize_root(root: Option<PathBuf>) -> Option<PathBuf> {
    root.map(|path| path.canonicalize().unwrap_or(path))
}

fn ensure_path_within_allowed_roots(target: &Path, roots: &LocalPathRoots) -> Result<(), String> {
    let mut allowed_roots = Vec::new();

    if let Some(workspace_root) = roots.workspace_root.as_ref() {
        allowed_roots.push(workspace_root);
    }

    if let Some(repo_root) = roots.repo_root.as_ref() {
        allowed_roots.push(repo_root);
    }

    if allowed_roots.is_empty() {
        return Err("desktop local path roots are unavailable".to_string());
    }

    if allowed_roots.iter().any(|root| target.starts_with(root)) {
        return Ok(());
    }

    Err(format!(
        "local target is outside the allowed workspace and repository roots: {}",
        target.display()
    ))
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
    run_platform_command(
        "open",
        &[target],
        &format!("open local target {}", target.display()),
    )
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
    use super::{resolve_existing_local_path, resolve_path_candidate, LocalPathRoots};
    use std::env;
    use std::fs;
    use std::path::PathBuf;
    use std::time::{SystemTime, UNIX_EPOCH};

    fn normalize_windows_verbatim_prefix(path: PathBuf) -> String {
        path.to_string_lossy().replace(r"\\?\", "")
    }

    #[test]
    fn resolve_path_candidate_rejects_empty_input() {
        assert!(resolve_path_candidate("   ", &LocalPathRoots::new(None, None)).is_err());
    }

    #[test]
    fn resolve_path_candidate_joins_workspace_relative_paths_against_workspace_root() {
        let fixture = create_root_fixture("workspace-root");
        let roots = LocalPathRoots::new(
            Some(fixture.workspace_root.clone()),
            Some(fixture.repo_root),
        );
        let resolved = resolve_path_candidate("workspace/artifacts/result.docx", &roots)
            .expect("resolve workspace-relative path");

        assert_eq!(
            normalize_windows_verbatim_prefix(resolved),
            normalize_windows_verbatim_prefix(
                fixture.workspace_root.join("artifacts").join("result.docx")
            )
        );
    }

    #[test]
    fn resolve_path_candidate_joins_repo_relative_paths_against_repo_root() {
        let fixture = create_root_fixture("repo-root");
        let roots = LocalPathRoots::new(
            Some(fixture.workspace_root),
            Some(fixture.repo_root.clone()),
        );
        let resolved = resolve_path_candidate("apps/desktop/src/main.ts", &roots)
            .expect("resolve repo-relative path");

        assert_eq!(
            normalize_windows_verbatim_prefix(resolved),
            normalize_windows_verbatim_prefix(
                fixture
                    .repo_root
                    .join("apps")
                    .join("desktop")
                    .join("src")
                    .join("main.ts")
            )
        );
    }

    #[test]
    fn resolve_existing_local_path_accepts_existing_workspace_targets() {
        let fixture = create_root_fixture("existing-target");
        let target = fixture.workspace_root.join("reports").join("summary.txt");
        fs::create_dir_all(target.parent().expect("workspace target parent"))
            .expect("create workspace target parent");
        fs::write(&target, "artifact").expect("write temp target");
        let roots = LocalPathRoots::new(
            Some(fixture.workspace_root.clone()),
            Some(fixture.repo_root),
        );

        let resolved = resolve_existing_local_path("workspace/reports/summary.txt", &roots)
            .expect("resolve existing target");

        assert!(resolved.is_absolute());
        assert!(resolved.exists());
    }

    #[test]
    fn resolve_existing_local_path_rejects_missing_targets() {
        let fixture = create_root_fixture("missing-target");
        let roots = LocalPathRoots::new(Some(fixture.workspace_root), Some(fixture.repo_root));
        let error = resolve_existing_local_path("workspace/missing-target.txt", &roots)
            .expect_err("missing target should fail");

        assert!(error.contains("does not exist"));
    }

    #[test]
    fn resolve_existing_local_path_rejects_absolute_targets_outside_allowed_roots() {
        let fixture = create_root_fixture("outside-scope");
        let outside_target = unique_temp_path("outside-scope-target.txt");
        fs::write(&outside_target, "artifact").expect("write outside target");
        let roots = LocalPathRoots::new(Some(fixture.workspace_root), Some(fixture.repo_root));
        let error = resolve_existing_local_path(outside_target.to_string_lossy().as_ref(), &roots)
            .expect_err("outside target should fail");

        assert!(error.contains("outside the allowed workspace and repository roots"));

        let _ = fs::remove_file(outside_target);
    }

    #[test]
    fn local_path_roots_keep_missing_workspace_root_for_later_resolution() {
        let fixture = create_root_fixture("missing-root-retained");
        let missing_workspace_root = fixture.workspace_root.join("missing");
        let roots = LocalPathRoots::new(
            Some(missing_workspace_root.clone()),
            Some(fixture.repo_root),
        );

        assert_eq!(roots.workspace_root(), Some(&missing_workspace_root));
    }

    struct RootFixture {
        repo_root: PathBuf,
        workspace_root: PathBuf,
    }

    fn create_root_fixture(name: &str) -> RootFixture {
        let root = unique_temp_path(name);
        let repo_root = root.join("repo");
        let workspace_root = root.join("workspace");
        fs::create_dir_all(&repo_root).expect("create repo root");
        fs::create_dir_all(&workspace_root).expect("create workspace root");

        RootFixture {
            repo_root,
            workspace_root,
        }
    }

    fn unique_temp_path(name: &str) -> PathBuf {
        let unique = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("read system time")
            .as_nanos();

        env::temp_dir().join(format!("cialloclaw-desktop-{unique}-{name}"))
    }
}
