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
    runtime_open_root: Option<PathBuf>,
    runtime_root: Option<PathBuf>,
    workspace_root: Option<PathBuf>,
}

const RUNTIME_OPEN_TEMP_PREFIX: &str = "temp";

impl LocalPathRoots {
    /// Builds the local path roots from trusted host-side runtime configuration.
    pub fn new(
        workspace_root: Option<PathBuf>,
        runtime_root: Option<PathBuf>,
        runtime_open_root: Option<PathBuf>,
    ) -> Self {
        Self {
            runtime_open_root: canonicalize_root(runtime_open_root),
            runtime_root: canonicalize_root(runtime_root),
            workspace_root: canonicalize_root(workspace_root),
        }
    }

    /// Returns the trusted workspace root used for workspace-relative desktop
    /// path resolution.
    pub(crate) fn workspace_root(&self) -> Option<&PathBuf> {
        self.workspace_root.as_ref()
    }

    /// Returns the trusted runtime root used for runtime-relative desktop path
    /// resolution.
    pub(crate) fn runtime_root(&self) -> Option<&PathBuf> {
        self.runtime_root.as_ref()
    }
}

/// Opens a local file or directory through the operating system shell.
pub fn open_local_path(raw_path: &str, roots: &LocalPathRoots) -> Result<(), String> {
    let target = resolve_existing_local_path(raw_path, roots)?;
    open_with_system_handler(&target)
}

/// Opens an external web url through the operating system's default browser.
pub fn open_external_url(raw_url: &str) -> Result<(), String> {
    let url = normalize_external_url(raw_url)?;
    open_url_with_system_handler(&url)
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

/// Opens one host-owned runtime directory after creating it when needed.
pub fn open_trusted_directory(target: &Path, trusted_root: &Path) -> Result<(), String> {
    let canonical_target = prepare_trusted_directory_target(target, trusted_root)?;
    open_with_system_handler(&canonical_target)
}

/// Opens one trusted external http/https URL through the operating system
/// shell so renderer links leave the embedded desktop WebView.
pub fn open_external_url(raw_url: &str) -> Result<(), String> {
    let target = resolve_external_url(raw_url)?;
    open_url_with_system_handler(&target)
}

/// Resolves delivery paths against trusted workspace or runtime-open roots and
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

    if let Some(runtime_open_relative_path) = strip_runtime_open_prefix(trimmed) {
        if let Some(runtime_open_root) = roots.runtime_open_root.as_ref() {
            return Ok(runtime_open_root.join(runtime_open_relative_path));
        }

        return Err(
            "runtime open root is not available for runtime-relative delivery paths".to_string(),
        );
    }

    Err("runtime-relative delivery paths must stay within the trusted temp/ scope".to_string())
}

fn resolve_external_url(raw_url: &str) -> Result<String, String> {
    let trimmed = raw_url.trim();
    if trimmed.is_empty() {
        return Err("external url is empty".to_string());
    }

    let lower = trimmed.to_ascii_lowercase();
    if !lower.starts_with("http://") && !lower.starts_with("https://") {
        return Err("external url must use http or https".to_string());
    }

    Ok(trimmed.to_string())
}

fn prepare_trusted_directory_target(target: &Path, trusted_root: &Path) -> Result<PathBuf, String> {
    std::fs::create_dir_all(target).map_err(|error| {
        format!(
            "failed to create trusted local directory {}: {error}",
            target.display()
        )
    })?;

    let canonical_target = target.canonicalize().map_err(|error| {
        format!(
            "failed to canonicalize trusted local directory {}: {error}",
            target.display()
        )
    })?;

    if !canonical_target.is_dir() {
        return Err(format!(
            "trusted local target is not a directory: {}",
            canonical_target.display()
        ));
    }

    // The runtime root captured during startup remains the host-trusted scope.
    // Even if a later symlink/junction rewires `data` somewhere else, the
    // canonicalized directory must still stay under that original root.
    if !canonical_target.starts_with(trusted_root) {
        return Err(format!(
            "trusted local target escaped the runtime root {}: {}",
            trusted_root.display(),
            canonical_target.display()
        ));
    }

    Ok(canonical_target)
}

fn strip_workspace_prefix(raw_path: &str) -> Option<&str> {
    if raw_path == "workspace" {
        return Some("");
    }

    raw_path
        .strip_prefix("workspace/")
        .or_else(|| raw_path.strip_prefix("workspace\\"))
}

fn strip_runtime_open_prefix(raw_path: &str) -> Option<&str> {
    if raw_path == RUNTIME_OPEN_TEMP_PREFIX {
        return Some("");
    }

    raw_path
        .strip_prefix("temp/")
        .or_else(|| raw_path.strip_prefix("temp\\"))
}

fn canonicalize_root(root: Option<PathBuf>) -> Option<PathBuf> {
    root.map(|path| path.canonicalize().unwrap_or(path))
}

fn ensure_path_within_allowed_roots(target: &Path, roots: &LocalPathRoots) -> Result<(), String> {
    let mut allowed_roots = Vec::new();

    if let Some(workspace_root) = roots.workspace_root.as_ref() {
        allowed_roots.push(workspace_root);
    }

    if let Some(runtime_open_root) = roots.runtime_open_root.as_ref() {
        allowed_roots.push(runtime_open_root);
    }

    if allowed_roots.is_empty() {
        return Err("desktop local path roots are unavailable".to_string());
    }

    if allowed_roots.iter().any(|root| target.starts_with(root)) {
        return Ok(());
    }

    Err(format!(
        "local target is outside the allowed workspace and runtime roots: {}",
        target.display()
    ))
}

fn normalize_external_url(raw_url: &str) -> Result<String, String> {
    let trimmed = raw_url.trim();
    if trimmed.is_empty() {
        return Err("external url is empty".to_string());
    }

    let Some((scheme, remainder)) = trimmed.split_once(':') else {
        return Err("external url is missing a scheme".to_string());
    };

    if !scheme.eq_ignore_ascii_case("https") && !scheme.eq_ignore_ascii_case("http") {
        return Err("external url must use http or https".to_string());
    }

    if !remainder.starts_with("//") {
        return Err("external url must include // after the scheme".to_string());
    }

    if trimmed.chars().any(char::is_whitespace) {
        return Err("external url must not contain whitespace".to_string());
    }

    Ok(trimmed.to_string())
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
fn open_url_with_system_handler(target: &str) -> Result<(), String> {
    let operation = encode_wide(OsStr::new("open"));
    let target_wide = encode_wide(OsStr::new(target));
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
fn open_url_with_system_handler(target: &str) -> Result<(), String> {
    let status = Command::new("open")
        .arg(target)
        .status()
        .map_err(|error| format!("failed to open external url {target}: {error}"))?;

    if !status.success() {
        return Err(format!("failed to open external url {target}: exit status {status}"));
    }

    Ok(())
}

#[cfg(target_os = "macos")]
fn reveal_with_system_handler(target: &Path) -> Result<(), String> {
    run_platform_command(
        "open",
        &[Path::new("-R"), target],
        &format!("reveal local target {}", target.display()),
    )
}

#[cfg(target_os = "macos")]
fn open_url_with_system_handler(target: &str) -> Result<(), String> {
    run_platform_string_command("open", &[target], &format!("open external url {target}"))
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
fn open_url_with_system_handler(target: &str) -> Result<(), String> {
    let status = Command::new("xdg-open")
        .arg(target)
        .status()
        .map_err(|error| format!("failed to open external url {target}: {error}"))?;

    if !status.success() {
        return Err(format!("failed to open external url {target}: exit status {status}"));
    }

    Ok(())
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

#[cfg(all(not(windows), not(target_os = "macos")))]
fn open_url_with_system_handler(target: &str) -> Result<(), String> {
    run_platform_string_command("xdg-open", &[target], &format!("open external url {target}"))
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
fn run_platform_string_command(program: &str, args: &[&str], description: &str) -> Result<(), String> {
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
    use super::{
        normalize_external_url, open_trusted_directory, prepare_trusted_directory_target,
        resolve_existing_local_path, resolve_path_candidate, LocalPathRoots,
    };
    use std::env;
    use std::fs;
    use std::path::PathBuf;
    use std::time::{SystemTime, UNIX_EPOCH};

    #[test]
    fn resolve_path_candidate_rejects_empty_input() {
        assert!(resolve_path_candidate("   ", &LocalPathRoots::new(None, None, None)).is_err());
    }

    #[test]
    fn normalize_external_url_accepts_uppercase_http_schemes() {
        let normalized = normalize_external_url("HTTPS://example.com/path")
            .expect("uppercase https scheme should pass validation");

        assert_eq!(normalized, "HTTPS://example.com/path");
    }

    #[test]
    fn normalize_external_url_rejects_schemes_without_double_slash() {
        let error = normalize_external_url("https:example.com")
            .expect_err("scheme without double slash should fail");

        assert!(error.contains("include //"));
    }

    #[test]
    fn resolve_path_candidate_joins_workspace_relative_paths_against_workspace_root() {
        let fixture = create_root_fixture("workspace-root");
        let roots = LocalPathRoots::new(
            Some(fixture.workspace_root.clone()),
            Some(fixture.runtime_root.clone()),
            Some(fixture.runtime_root),
        );
        let resolved = resolve_path_candidate("workspace/artifacts/result.docx", &roots)
            .expect("resolve workspace-relative path");

        assert_eq!(
            resolved,
            fixture.workspace_root.join("artifacts").join("result.docx")
        );
    }

    #[test]
    fn resolve_path_candidate_joins_runtime_relative_paths_against_runtime_open_root() {
        let fixture = create_root_fixture("runtime-root");
        let roots = LocalPathRoots::new(
            Some(fixture.workspace_root),
            Some(fixture.runtime_root.clone()),
            Some(fixture.runtime_root.join("temp")),
        );
        let resolved = resolve_path_candidate("temp/screenshot.png", &roots)
            .expect("resolve runtime-relative path");

        assert_eq!(
            resolved,
            fixture.runtime_root.join("temp").join("screenshot.png")
        );
    }

    #[test]
    fn resolve_path_candidate_rejects_runtime_relative_paths_outside_temp_scope() {
        let fixture = create_root_fixture("runtime-scope");
        let roots = LocalPathRoots::new(
            Some(fixture.workspace_root),
            Some(fixture.runtime_root.clone()),
            Some(fixture.runtime_root.join("temp")),
        );

        let error = resolve_path_candidate("data/cialloclaw.db", &roots)
            .expect_err("non-temp runtime-relative path should fail");

        assert!(error.contains("trusted temp/ scope"));
    }

    #[test]
    fn resolve_path_candidate_rejects_runtime_relative_paths_without_runtime_open_root() {
        let fixture = create_root_fixture("runtime-open-root-required");
        let roots = LocalPathRoots::new(
            Some(fixture.workspace_root),
            Some(fixture.runtime_root),
            None,
        );

        let error = resolve_path_candidate("temp/screenshot.png", &roots)
            .expect_err("runtime-relative path should require runtime open root");

        assert!(error.contains("runtime open root is not available"));
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
            Some(fixture.runtime_root.clone()),
            Some(fixture.runtime_root),
        );

        let resolved = resolve_existing_local_path("workspace/reports/summary.txt", &roots)
            .expect("resolve existing target");

        assert!(resolved.is_absolute());
        assert!(resolved.exists());
    }

    #[test]
    fn resolve_existing_local_path_rejects_missing_targets() {
        let fixture = create_root_fixture("missing-target");
        let roots = LocalPathRoots::new(
            Some(fixture.workspace_root),
            Some(fixture.runtime_root.clone()),
            Some(fixture.runtime_root),
        );
        let error = resolve_existing_local_path("workspace/missing-target.txt", &roots)
            .expect_err("missing target should fail");

        assert!(error.contains("does not exist"));
    }

    #[test]
    fn resolve_existing_local_path_rejects_absolute_targets_outside_allowed_roots() {
        let fixture = create_root_fixture("outside-scope");
        let outside_target = unique_temp_path("outside-scope-target.txt");
        fs::write(&outside_target, "artifact").expect("write outside target");
        let roots = LocalPathRoots::new(
            Some(fixture.workspace_root),
            Some(fixture.runtime_root.clone()),
            Some(fixture.runtime_root),
        );
        let error = resolve_existing_local_path(outside_target.to_string_lossy().as_ref(), &roots)
            .expect_err("outside target should fail");

        assert!(error.contains("outside the allowed workspace and runtime roots"));

        let _ = fs::remove_file(outside_target);
    }

    #[test]
    fn local_path_roots_keep_missing_workspace_root_for_later_resolution() {
        let fixture = create_root_fixture("missing-root-retained");
        let missing_workspace_root = fixture.workspace_root.join("missing");
        let roots = LocalPathRoots::new(
            Some(missing_workspace_root.clone()),
            Some(fixture.runtime_root.clone()),
            Some(fixture.runtime_root),
        );

        assert_eq!(roots.workspace_root(), Some(&missing_workspace_root));
    }

    #[test]
    fn prepare_trusted_directory_target_creates_missing_directory() {
        let fixture = create_root_fixture("trusted-runtime-data-dir");
        let target = fixture.runtime_root.join("data");

        let resolved = prepare_trusted_directory_target(&target, &fixture.runtime_root)
            .expect("prepare trusted runtime data directory");

        assert!(resolved.is_absolute());
        assert!(resolved.is_dir());

        let _ = fs::remove_dir_all(target);
    }

    #[test]
    fn prepare_trusted_directory_target_rejects_existing_files() {
        let fixture = create_root_fixture("trusted-runtime-data-file");
        let target = fixture.runtime_root.join("data");
        fs::write(&target, "not a directory").expect("write trusted runtime file");

        let error = prepare_trusted_directory_target(&target, &fixture.runtime_root)
            .expect_err("file target should not masquerade as a directory");

        assert!(error.contains("failed to create trusted local directory"));

        let _ = fs::remove_file(target);
    }

    #[test]
    fn prepare_trusted_directory_target_rejects_directories_outside_runtime_root() {
        let fixture = create_root_fixture("trusted-runtime-outside-root");
        let target = unique_temp_path("trusted-runtime-external-data-dir");

        let error = prepare_trusted_directory_target(&target, &fixture.runtime_root)
            .expect_err("external directories should not be treated as trusted runtime data");

        assert!(error.contains("escaped the runtime root"));

        let _ = fs::remove_dir_all(target);
    }

    #[test]
    fn open_trusted_directory_rejects_directories_outside_runtime_root_before_shell_open() {
        let fixture = create_root_fixture("trusted-runtime-open-boundary");
        let target = unique_temp_path("trusted-runtime-open-external-data-dir");

        let error = open_trusted_directory(&target, &fixture.runtime_root)
            .expect_err("external directories should fail before shell open");

        assert!(error.contains("escaped the runtime root"));

        let _ = fs::remove_dir_all(target);
    }

    struct RootFixture {
        runtime_root: PathBuf,
        workspace_root: PathBuf,
    }

    fn create_root_fixture(name: &str) -> RootFixture {
        let root = unique_temp_path(name);
        let runtime_root = root.join("runtime");
        let workspace_root = runtime_root.join("workspace");
        fs::create_dir_all(&runtime_root).expect("create runtime root");
        fs::create_dir_all(&workspace_root).expect("create workspace root");

        RootFixture {
            runtime_root: runtime_root
                .canonicalize()
                .expect("canonicalize runtime root"),
            workspace_root: workspace_root
                .canonicalize()
                .expect("canonicalize workspace root"),
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
