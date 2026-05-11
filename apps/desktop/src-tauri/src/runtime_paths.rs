use std::env;
use std::fs;
#[cfg(test)]
use std::path::Path;
use std::path::PathBuf;

const DEFAULT_RUNTIME_DIRECTORY_NAME: &str = "CialloClaw";
const DEFAULT_WORKSPACE_DIRECTORY_NAME: &str = "workspace";
const DEFAULT_TASK_SOURCE_DIRECTORY_NAME: &str = "todos";

/// DesktopRuntimePaths keeps the trusted runtime directories shared by the
/// packaged desktop host and the local-service defaults.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct DesktopRuntimePaths {
    runtime_root: PathBuf,
    workspace_root: PathBuf,
}

impl DesktopRuntimePaths {
    /// Detects the canonical runtime directories for the current user profile.
    pub fn detect() -> Result<Self, String> {
        let runtime_root = resolve_runtime_root()?;
        if let Some(workspace_root) = clean_env_path("CIALLOCLAW_WORKSPACE_ROOT") {
            return Ok(Self::from_runtime_and_workspace_root(
                runtime_root,
                workspace_root,
            ));
        }

        Ok(Self::from_runtime_root(runtime_root))
    }

    /// Builds the runtime directory set from one trusted root.
    pub fn from_runtime_root(runtime_root: PathBuf) -> Self {
        Self::from_runtime_and_workspace_root(
            runtime_root.clone(),
            runtime_root.join(DEFAULT_WORKSPACE_DIRECTORY_NAME),
        )
    }

    /// Builds the runtime directory set from one trusted runtime root and one
    /// trusted workspace root.
    pub fn from_runtime_and_workspace_root(runtime_root: PathBuf, workspace_root: PathBuf) -> Self {
        let runtime_root = runtime_root.canonicalize().unwrap_or(runtime_root);
        let workspace_root = workspace_root.canonicalize().unwrap_or(workspace_root);

        Self {
            runtime_root,
            workspace_root,
        }
    }

    /// Returns the canonical runtime root used for host-managed relative paths.
    pub fn runtime_root(&self) -> &PathBuf {
        &self.runtime_root
    }

    /// Returns the canonical workspace root used for workspace-relative paths.
    pub fn workspace_root(&self) -> &PathBuf {
        &self.workspace_root
    }

    /// Returns the primary task-source root used by the desktop note bridge.
    pub fn task_source_root(&self) -> PathBuf {
        self.workspace_root.join(DEFAULT_TASK_SOURCE_DIRECTORY_NAME)
    }

    /// Returns the runtime temp directory used for transient desktop captures.
    pub fn temp_dir(&self) -> PathBuf {
        self.runtime_root.join("temp")
    }

    /// Returns the runtime data directory used by local-service storage.
    pub fn data_dir(&self) -> PathBuf {
        self.runtime_root.join("data")
    }

    /// Returns the trusted runtime subroot that desktop open/reveal flows may
    /// expose to the renderer for transient runtime-managed files.
    pub fn local_open_runtime_root(&self) -> PathBuf {
        self.temp_dir()
    }

    /// Ensures the desktop-host runtime directories exist before renderer flows
    /// attempt to open them or persist source-note files into them.
    pub fn ensure_runtime_directories(&self) -> Result<(), String> {
        for target in [
            self.runtime_root.clone(),
            self.data_dir(),
            self.temp_dir(),
            self.workspace_root.clone(),
            self.task_source_root(),
        ] {
            fs::create_dir_all(&target).map_err(|error| {
                format!(
                    "failed to create desktop runtime directory {}: {error}",
                    target.display()
                )
            })?;
        }

        Ok(())
    }

    /// Resolves a persisted workspace_path setting against the canonical runtime
    /// root when that setting still contains a legacy relative placeholder.
    #[cfg(test)]
    pub fn resolve_workspace_setting(&self, configured_root: &Path) -> PathBuf {
        if configured_root.is_absolute() {
            return configured_root.to_path_buf();
        }

        let trimmed = configured_root.to_string_lossy().trim().to_string();
        if uses_legacy_workspace_placeholder(&trimmed) {
            return self.workspace_root.clone();
        }

        if !is_safe_runtime_relative_path(configured_root) {
            return self.workspace_root.clone();
        }

        self.runtime_root.join(configured_root)
    }
}

fn resolve_runtime_root() -> Result<PathBuf, String> {
    if let Some(value) = clean_env_path("CIALLOCLAW_RUNTIME_ROOT") {
        return Ok(value);
    }

    if cfg!(target_os = "windows") {
        if let Some(local_app_data) = clean_env_path("LOCALAPPDATA") {
            return Ok(local_app_data.join(DEFAULT_RUNTIME_DIRECTORY_NAME));
        }
    }

    if cfg!(target_os = "macos") {
        if let Some(home) = clean_env_path("HOME") {
            return Ok(home
                .join("Library")
                .join("Application Support")
                .join(DEFAULT_RUNTIME_DIRECTORY_NAME));
        }
    }

    if let Some(xdg_data_home) = clean_env_path("XDG_DATA_HOME") {
        return Ok(xdg_data_home.join(DEFAULT_RUNTIME_DIRECTORY_NAME));
    }
    if let Some(home) = clean_env_path("HOME") {
        return Ok(home
            .join(".local")
            .join("share")
            .join(DEFAULT_RUNTIME_DIRECTORY_NAME));
    }

    if let Ok(current_dir) = env::current_dir() {
        return Ok(current_dir.join(DEFAULT_RUNTIME_DIRECTORY_NAME));
    }

    Ok(PathBuf::from(DEFAULT_RUNTIME_DIRECTORY_NAME))
}

fn clean_env_path(key: &str) -> Option<PathBuf> {
    env::var_os(key).and_then(|value| {
        let path = PathBuf::from(value);
        let trimmed = path.to_string_lossy().trim().to_string();
        if trimmed.is_empty() {
            None
        } else {
            Some(PathBuf::from(trimmed))
        }
    })
}

#[cfg(test)]
fn uses_legacy_workspace_placeholder(raw_path: &str) -> bool {
    let normalized = raw_path.replace('\\', "/");
    let trimmed = normalized.trim().trim_matches('/');
    trimmed.is_empty() || trimmed == "." || trimmed == DEFAULT_WORKSPACE_DIRECTORY_NAME
}

#[cfg(test)]
fn is_safe_runtime_relative_path(configured_root: &Path) -> bool {
    configured_root.components().all(|component| {
        matches!(
            component,
            std::path::Component::CurDir | std::path::Component::Normal(_)
        )
    })
}

#[cfg(test)]
mod tests {
    use super::DesktopRuntimePaths;
    use std::fs;
    use std::path::Path;
    use std::time::{SystemTime, UNIX_EPOCH};

    fn unique_temp_path(label: &str) -> std::path::PathBuf {
        let timestamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("system time before unix epoch")
            .as_nanos();
        std::env::temp_dir().join(format!("cialloclaw-{label}-{timestamp}"))
    }

    #[test]
    fn resolve_workspace_setting_promotes_legacy_workspace_placeholder() {
        let runtime_paths = DesktopRuntimePaths::from_runtime_root(
            Path::new("C:/Users/test/AppData/Local/CialloClaw").to_path_buf(),
        );

        assert_eq!(
            runtime_paths.resolve_workspace_setting(Path::new("workspace")),
            Path::new("C:/Users/test/AppData/Local/CialloClaw/workspace")
        );
        assert_eq!(
            runtime_paths.resolve_workspace_setting(Path::new("./workspace")),
            Path::new("C:/Users/test/AppData/Local/CialloClaw/workspace")
        );
    }

    #[test]
    fn resolve_workspace_setting_keeps_absolute_paths() {
        let runtime_paths = DesktopRuntimePaths::from_runtime_root(
            Path::new("C:/Users/test/AppData/Local/CialloClaw").to_path_buf(),
        );
        let absolute = Path::new("D:/Projects/CustomWorkspace");

        assert_eq!(runtime_paths.resolve_workspace_setting(absolute), absolute);
    }

    #[test]
    fn resolve_workspace_setting_joins_other_relative_paths_under_runtime_root() {
        let runtime_paths = DesktopRuntimePaths::from_runtime_root(
            Path::new("C:/Users/test/AppData/Local/CialloClaw").to_path_buf(),
        );

        assert_eq!(
            runtime_paths.resolve_workspace_setting(Path::new("custom-workspace")),
            Path::new("C:/Users/test/AppData/Local/CialloClaw/custom-workspace")
        );
    }

    #[test]
    fn resolve_workspace_setting_rejects_escape_paths() {
        let runtime_paths = DesktopRuntimePaths::from_runtime_root(
            Path::new("C:/Users/test/AppData/Local/CialloClaw").to_path_buf(),
        );

        assert_eq!(
            runtime_paths.resolve_workspace_setting(Path::new("../outside")),
            Path::new("C:/Users/test/AppData/Local/CialloClaw/workspace")
        );
        assert_eq!(
            runtime_paths.resolve_workspace_setting(Path::new("..\\outside")),
            Path::new("C:/Users/test/AppData/Local/CialloClaw/workspace")
        );
    }

    #[test]
    fn from_runtime_and_workspace_root_keeps_workspace_override() {
        let runtime_paths = DesktopRuntimePaths::from_runtime_and_workspace_root(
            Path::new("C:/Users/test/AppData/Local/CialloClaw").to_path_buf(),
            Path::new("D:/CustomWorkspace").to_path_buf(),
        );

        assert_eq!(
            runtime_paths.workspace_root(),
            Path::new("D:/CustomWorkspace")
        );
    }

    #[test]
    fn data_dir_stays_under_runtime_root() {
        let runtime_paths = DesktopRuntimePaths::from_runtime_root(
            Path::new("C:/Users/test/AppData/Local/CialloClaw").to_path_buf(),
        );

        assert_eq!(
            runtime_paths.data_dir(),
            Path::new("C:/Users/test/AppData/Local/CialloClaw/data")
        );
    }

    #[test]
    fn ensure_runtime_directories_creates_workspace_and_task_source_roots() {
        let runtime_root = unique_temp_path("desktop-runtime-directories");
        let runtime_paths = DesktopRuntimePaths::from_runtime_root(runtime_root.clone());

        runtime_paths
            .ensure_runtime_directories()
            .expect("create desktop runtime directories");

        assert!(runtime_root.is_dir());
        assert!(runtime_paths.data_dir().is_dir());
        assert!(runtime_paths.temp_dir().is_dir());
        assert!(runtime_paths.workspace_root().is_dir());
        assert!(runtime_paths.task_source_root().is_dir());

        fs::remove_dir_all(runtime_root).expect("remove temporary runtime root");
    }
}
