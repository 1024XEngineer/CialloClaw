use std::env;
use std::path::{Path, PathBuf};

const DEFAULT_RUNTIME_DIRECTORY_NAME: &str = "CialloClaw";
const DEFAULT_WORKSPACE_DIRECTORY_NAME: &str = "workspace";

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

    /// Returns the runtime temp directory used for transient desktop captures.
    pub fn temp_dir(&self) -> PathBuf {
        self.runtime_root.join("temp")
    }

    /// Returns the trusted runtime subroot that desktop open/reveal flows may
    /// expose to the renderer for transient runtime-managed files.
    pub fn local_open_runtime_root(&self) -> PathBuf {
        self.temp_dir()
    }

    /// Resolves a persisted workspace_path setting against the canonical runtime
    /// root when that setting still contains a legacy relative placeholder.
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

fn uses_legacy_workspace_placeholder(raw_path: &str) -> bool {
    let normalized = raw_path.replace('\\', "/");
    let trimmed = normalized.trim().trim_matches('/');
    trimmed.is_empty() || trimmed == "." || trimmed == DEFAULT_WORKSPACE_DIRECTORY_NAME
}

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
    use std::path::Path;

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
}
