use crate::local_path::LocalPathRoots;
use serde::Serialize;
use std::collections::HashSet;
use std::fs;
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

const PRIMARY_SOURCE_NOTE_FILE_NAME: &str = "notes.md";

/// DesktopSourceNoteDocument keeps the smallest file-backed markdown note shape
/// that the renderer needs for note-source editing.
#[derive(Clone, Serialize)]
pub struct DesktopSourceNoteDocument {
    pub content: String,
    pub file_name: String,
    pub modified_at_ms: Option<u64>,
    pub path: String,
    pub source_root: String,
    pub title: String,
}

/// DesktopSourceNoteIndexEntry keeps the lightweight file metadata used for
/// change detection without rereading every markdown note body.
#[derive(Clone, Serialize)]
pub struct DesktopSourceNoteIndexEntry {
    pub file_name: String,
    pub modified_at_ms: Option<u64>,
    pub path: String,
    pub size_bytes: u64,
    pub source_root: String,
}

/// DesktopSourceNoteSnapshot returns the current configured source roots plus
/// the markdown notes discovered under those roots.
#[derive(Clone, Serialize)]
pub struct DesktopSourceNoteSnapshot {
    pub default_source_root: Option<String>,
    pub notes: Vec<DesktopSourceNoteDocument>,
    pub source_roots: Vec<String>,
}

/// DesktopSourceNoteIndexSnapshot returns the current configured source roots
/// plus lightweight note metadata for fast polling.
#[derive(Clone, Serialize)]
pub struct DesktopSourceNoteIndexSnapshot {
    pub default_source_root: Option<String>,
    pub notes: Vec<DesktopSourceNoteIndexEntry>,
    pub source_roots: Vec<String>,
}

/// Loads the primary markdown source note selected from the configured
/// task-source roots.
pub fn load_source_notes(
    trusted_sources: &[String],
    roots: &LocalPathRoots,
) -> Result<DesktopSourceNoteSnapshot, String> {
    let resolved_roots = resolve_source_roots(trusted_sources, roots)?;
    let mut notes = Vec::new();

    if let Some((note_path, source_root)) = resolve_primary_source_note(&resolved_roots)? {
        notes.push(build_source_note_document(&note_path, source_root)?);
    }

    notes.sort_by(|left, right| {
        right
            .modified_at_ms
            .cmp(&left.modified_at_ms)
            .then_with(|| left.title.cmp(&right.title))
            .then_with(|| left.path.cmp(&right.path))
    });

    Ok(DesktopSourceNoteSnapshot {
        default_source_root: resolved_roots
            .first()
            .map(|path| path.to_string_lossy().to_string()),
        notes,
        source_roots: resolved_roots
            .iter()
            .map(|path| path.to_string_lossy().to_string())
            .collect(),
    })
}

/// Loads lightweight metadata for the primary markdown source note without
/// rereading every file body under the task-source roots.
pub fn load_source_note_index(
    trusted_sources: &[String],
    roots: &LocalPathRoots,
) -> Result<DesktopSourceNoteIndexSnapshot, String> {
    let resolved_roots = resolve_source_roots(trusted_sources, roots)?;
    let mut notes = Vec::new();

    if let Some((note_path, source_root)) = resolve_primary_source_note(&resolved_roots)? {
        notes.push(build_source_note_index_entry(&note_path, source_root)?);
    }

    notes.sort_by(|left, right| {
        right
            .modified_at_ms
            .cmp(&left.modified_at_ms)
            .then_with(|| left.path.cmp(&right.path))
    });

    Ok(DesktopSourceNoteIndexSnapshot {
        default_source_root: resolved_roots
            .first()
            .map(|path| path.to_string_lossy().to_string()),
        notes,
        source_roots: resolved_roots
            .iter()
            .map(|path| path.to_string_lossy().to_string())
            .collect(),
    })
}

/// Appends a new markdown note block into the primary task-source markdown
/// file, creating that file under the first configured root when needed.
pub fn create_source_note(
    trusted_sources: &[String],
    roots: &LocalPathRoots,
    content: &str,
) -> Result<DesktopSourceNoteDocument, String> {
    let resolved_roots = resolve_source_roots(trusted_sources, roots)?;
    let (target_path, target_root) = resolve_primary_source_note_write_target(&resolved_roots)?;
    fs::create_dir_all(target_root).map_err(|error| {
        format!(
            "failed to create task source directory {}: {error}",
            target_root.display()
        )
    })?;

    let next_content = if target_path.exists() {
        let existing_content = fs::read_to_string(&target_path).map_err(|error| {
            format!(
                "failed to read source note {} before append: {error}",
                target_path.display()
            )
        })?;
        append_source_note_block(&existing_content, content)
    } else {
        normalize_markdown_content(&normalize_new_source_note_block(content))
    };

    fs::write(&target_path, next_content).map_err(|error| {
        format!(
            "failed to write source note {}: {error}",
            target_path.display()
        )
    })?;

    build_source_note_document(&target_path, target_root)
}

/// Saves the updated markdown content back into an existing source note file.
pub fn save_source_note(
    trusted_sources: &[String],
    roots: &LocalPathRoots,
    raw_path: &str,
    content: &str,
) -> Result<DesktopSourceNoteDocument, String> {
    let resolved_roots = resolve_source_roots(trusted_sources, roots)?;
    let (canonical_target, source_root) = if raw_path.trim().is_empty() {
        resolve_primary_source_note_write_target(&resolved_roots)?
    } else {
        resolve_source_note_target(raw_path, &resolved_roots)?
    };
    let normalized_content = normalize_markdown_content(content);

    fs::write(&canonical_target, normalized_content).map_err(|error| {
        format!(
            "failed to save source note {}: {error}",
            canonical_target.display()
        )
    })?;

    build_source_note_document(&canonical_target, source_root)
}

fn resolve_source_note_target<'a>(
    raw_path: &str,
    roots: &'a [PathBuf],
) -> Result<(PathBuf, &'a PathBuf), String> {
    let trimmed = raw_path.trim();
    if trimmed.is_empty() {
        return Err("source note path is empty".to_string());
    }

    let canonical_target = PathBuf::from(trimmed)
        .canonicalize()
        .map_err(|error| format!("failed to resolve source note {trimmed}: {error}"))?;
    let metadata = fs::metadata(&canonical_target).map_err(|error| {
        format!(
            "failed to inspect source note {}: {error}",
            canonical_target.display()
        )
    })?;
    if !metadata.is_file() {
        return Err(format!(
            "source note path is not a file: {}",
            canonical_target.display()
        ));
    }
    if !is_markdown_file(&canonical_target) {
        return Err(format!(
            "source note path is not a markdown file: {}",
            canonical_target.display()
        ));
    }

    let source_root = match_source_root(&canonical_target, roots)?;
    Ok((canonical_target, source_root))
}

fn resolve_primary_source_note<'a>(
    roots: &'a [PathBuf],
) -> Result<Option<(PathBuf, &'a PathBuf)>, String> {
    let (preferred_path, preferred_root) = resolve_primary_source_note_write_target(roots)?;
    if preferred_path.exists() {
        return Ok(Some((preferred_path, preferred_root)));
    }

    let mut existing_notes = collect_existing_markdown_files(roots)?;
    existing_notes.sort_by(|left, right| {
        read_modified_at_ms(right)
            .cmp(&read_modified_at_ms(left))
            .then_with(|| left.cmp(right))
    });

    if let Some(note_path) = existing_notes.into_iter().next() {
        let source_root = match_source_root(&note_path, roots)?;
        return Ok(Some((note_path, source_root)));
    }

    Ok(None)
}

fn resolve_primary_source_note_write_target<'a>(
    roots: &'a [PathBuf],
) -> Result<(PathBuf, &'a PathBuf), String> {
    let default_root = roots
        .first()
        .ok_or_else(|| "task source list is empty".to_string())?;

    for root in roots {
        let preferred_path = build_primary_source_note_path(root);
        if preferred_path.exists() {
            if preferred_path.is_file() {
                return Ok((preferred_path, root));
            }
            return Err(format!(
                "primary source note path is not a file: {}",
                preferred_path.display()
            ));
        }
    }

    let existing_notes = collect_existing_markdown_files(roots)?;
    if existing_notes.len() == 1 {
        let note_path = existing_notes
            .into_iter()
            .next()
            .ok_or_else(|| "failed to resolve the single existing markdown note".to_string())?;
        let source_root = match_source_root(&note_path, roots)?;
        return Ok((note_path, source_root));
    }

    Ok((build_primary_source_note_path(default_root), default_root))
}

/// Source roots are resolved only from the host-trusted settings snapshot.
/// The Tauri command layer is responsible for filtering out renderer-provided
/// paths before calling into this module.
fn resolve_source_roots(
    trusted_sources: &[String],
    roots: &LocalPathRoots,
) -> Result<Vec<PathBuf>, String> {
    let mut seen = HashSet::new();
    let mut resolved = Vec::new();

    for raw_source in trusted_sources {
        let candidate = resolve_source_root(raw_source, roots)?;
        let fingerprint = candidate.to_string_lossy().to_lowercase();
        if seen.insert(fingerprint) {
            resolved.push(candidate);
        }
    }

    Ok(resolved)
}

fn resolve_source_root(raw_source: &str, roots: &LocalPathRoots) -> Result<PathBuf, String> {
    let trimmed = raw_source.trim();
    if trimmed.is_empty() {
        return Err("task source path is empty".to_string());
    }

    let candidate = PathBuf::from(trimmed);
    let resolved = if candidate.is_absolute() {
        candidate
    } else if let Some(workspace_relative_path) = strip_workspace_prefix(trimmed) {
        let workspace_root = roots.workspace_root().ok_or_else(|| {
            "workspace root is unavailable for task source resolution".to_string()
        })?;
        workspace_root.join(workspace_relative_path)
    } else {
        let repo_root = roots.repo_root().ok_or_else(|| {
            "repository root is unavailable for task source resolution".to_string()
        })?;
        repo_root.join(candidate)
    };

    Ok(resolved.canonicalize().unwrap_or(resolved))
}

/// Reports whether any configured source still depends on the trusted
/// workspace root for path resolution.
pub(crate) fn sources_require_workspace_root(raw_sources: &[String]) -> bool {
    raw_sources
        .iter()
        .any(|raw_source| source_requires_workspace_root(raw_source))
}

fn source_requires_workspace_root(raw_source: &str) -> bool {
    let trimmed = raw_source.trim();
    if trimmed.is_empty() {
        return false;
    }

    !PathBuf::from(trimmed).is_absolute() && strip_workspace_prefix(trimmed).is_some()
}

fn strip_workspace_prefix(raw_path: &str) -> Option<&str> {
    if raw_path == "workspace" {
        return Some("");
    }

    raw_path
        .strip_prefix("workspace/")
        .or_else(|| raw_path.strip_prefix("workspace\\"))
}

fn build_primary_source_note_path(root: &Path) -> PathBuf {
    root.join(PRIMARY_SOURCE_NOTE_FILE_NAME)
}

fn collect_existing_markdown_files(roots: &[PathBuf]) -> Result<Vec<PathBuf>, String> {
    let mut result = Vec::new();

    for root in roots {
        if !root.exists() {
            continue;
        }
        if !root.is_dir() {
            return Err(format!(
                "task source is not a directory: {}",
                root.display()
            ));
        }

        collect_markdown_files(root, &mut result)?;
    }

    Ok(result)
}

fn collect_markdown_files(root: &Path, result: &mut Vec<PathBuf>) -> Result<(), String> {
    let entries = fs::read_dir(root).map_err(|error| {
        format!(
            "failed to read task source directory {}: {error}",
            root.display()
        )
    })?;

    for entry in entries {
        let entry = entry.map_err(|error| {
            format!(
                "failed to inspect task source directory entry in {}: {error}",
                root.display()
            )
        })?;
        let path = entry.path();
        let file_type = entry
            .file_type()
            .map_err(|error| format!("failed to read file type for {}: {error}", path.display()))?;

        if file_type.is_dir() {
            collect_markdown_files(&path, result)?;
            continue;
        }

        if file_type.is_file() && is_markdown_file(&path) {
            result.push(path);
        }
    }

    Ok(())
}

fn is_markdown_file(path: &Path) -> bool {
    matches!(
        path.extension().and_then(|extension| extension.to_str()),
        Some("md") | Some("markdown")
    )
}

fn build_source_note_document(
    note_path: &Path,
    source_root: &Path,
) -> Result<DesktopSourceNoteDocument, String> {
    let content = fs::read_to_string(note_path).map_err(|error| {
        format!(
            "failed to read source note {}: {error}",
            note_path.display()
        )
    })?;
    let file_name = note_path
        .file_name()
        .and_then(|name| name.to_str())
        .ok_or_else(|| format!("source note has no file name: {}", note_path.display()))?
        .to_string();

    Ok(DesktopSourceNoteDocument {
        content: content.clone(),
        file_name: file_name.clone(),
        modified_at_ms: read_modified_at_ms(note_path),
        path: note_path.to_string_lossy().to_string(),
        source_root: source_root.to_string_lossy().to_string(),
        title: derive_note_title(&content, &file_name),
    })
}

fn build_source_note_index_entry(
    note_path: &Path,
    source_root: &Path,
) -> Result<DesktopSourceNoteIndexEntry, String> {
    let metadata = fs::metadata(note_path).map_err(|error| {
        format!(
            "failed to inspect source note {}: {error}",
            note_path.display()
        )
    })?;
    let file_name = note_path
        .file_name()
        .and_then(|name| name.to_str())
        .ok_or_else(|| format!("source note has no file name: {}", note_path.display()))?
        .to_string();

    Ok(DesktopSourceNoteIndexEntry {
        file_name,
        modified_at_ms: metadata.modified().ok().and_then(system_time_to_unix_ms),
        path: note_path.to_string_lossy().to_string(),
        size_bytes: metadata.len(),
        source_root: source_root.to_string_lossy().to_string(),
    })
}

fn read_modified_at_ms(note_path: &Path) -> Option<u64> {
    fs::metadata(note_path)
        .ok()
        .and_then(|metadata| metadata.modified().ok())
        .and_then(system_time_to_unix_ms)
}

fn system_time_to_unix_ms(value: SystemTime) -> Option<u64> {
    value
        .duration_since(UNIX_EPOCH)
        .ok()
        .and_then(|duration| duration.as_millis().try_into().ok())
}

fn derive_note_title(content: &str, file_name: &str) -> String {
    for line in content.lines() {
        let trimmed = line.trim();
        if let Some(heading) = trimmed.strip_prefix('#') {
            let heading = heading.trim_start_matches('#').trim();
            if !heading.is_empty() {
                return heading.to_string();
            }
        }
    }

    for line in content.lines() {
        let trimmed = line.trim();
        if let Some(checklist_title) = parse_checklist_title(trimmed) {
            return checklist_title.to_string();
        }
        if !trimmed.is_empty() {
            return trimmed.to_string();
        }
    }

    Path::new(file_name)
        .file_stem()
        .and_then(|stem| stem.to_str())
        .map(str::to_string)
        .unwrap_or_else(|| "Untitled note".to_string())
}

fn normalize_markdown_content(content: &str) -> String {
    let trimmed = content.trim();
    if trimmed.is_empty() {
        return "# New note\n\n- [ ] Add the first task\n".to_string();
    }

    let normalized = content.replace("\r\n", "\n");
    if normalized.ends_with('\n') {
        normalized
    } else {
        format!("{normalized}\n")
    }
}

fn normalize_new_source_note_block(content: &str) -> String {
    let normalized = content.replace("\r\n", "\n");
    let trimmed = normalized.trim();
    if trimmed.is_empty() {
        return "- [ ] New note\nbucket: later\nnote: Add details here".to_string();
    }

    if normalized
        .lines()
        .map(str::trim)
        .any(|line| parse_checklist_title(line).is_some())
    {
        return trimmed.to_string();
    }

    let lines = normalized.lines().collect::<Vec<_>>();
    let first_non_empty_index = lines.iter().position(|line| !line.trim().is_empty());
    let Some(first_non_empty_index) = first_non_empty_index else {
        return "- [ ] New note".to_string();
    };

    let first_line = lines[first_non_empty_index].trim();
    let title = first_line
        .trim_start_matches('#')
        .trim()
        .trim_start_matches("- ")
        .trim_start_matches("* ")
        .trim();
    let rest = lines
        .iter()
        .skip(first_non_empty_index + 1)
        .copied()
        .collect::<Vec<_>>()
        .join("\n")
        .trim()
        .to_string();

    if rest.is_empty() {
        format!("- [ ] {title}\nbucket: later")
    } else {
        format!("- [ ] {title}\nbucket: later\nnote: {rest}")
    }
}

fn append_source_note_block(existing_content: &str, new_block_content: &str) -> String {
    let normalized_existing = existing_content.replace("\r\n", "\n");
    let trimmed_existing = normalized_existing.trim_end_matches('\n');
    let normalized_block =
        normalize_markdown_content(&normalize_new_source_note_block(new_block_content));
    let trimmed_block = normalized_block.trim_end_matches('\n');

    if trimmed_existing.trim().is_empty() {
        return format!("{trimmed_block}\n");
    }

    format!("{trimmed_existing}\n\n{trimmed_block}\n")
}

fn parse_checklist_title(line: &str) -> Option<&str> {
    line.strip_prefix("- [ ] ")
        .or_else(|| line.strip_prefix("* [ ] "))
        .or_else(|| line.strip_prefix("- [x] "))
        .or_else(|| line.strip_prefix("* [x] "))
        .or_else(|| line.strip_prefix("- [X] "))
        .or_else(|| line.strip_prefix("* [X] "))
        .map(str::trim)
        .filter(|value| !value.is_empty())
}

fn match_source_root<'a>(target: &Path, roots: &'a [PathBuf]) -> Result<&'a PathBuf, String> {
    roots
        .iter()
        .find(|root| target.strip_prefix(root).is_ok())
        .ok_or_else(|| {
            format!(
                "source note path is outside the configured task source roots: {}",
                target.display()
            )
        })
}

#[cfg(test)]
mod tests {
    use super::{resolve_source_note_target, resolve_source_root, sources_require_workspace_root};
    use crate::local_path::LocalPathRoots;
    use std::env;
    use std::fs;
    use std::path::PathBuf;
    use std::time::{SystemTime, UNIX_EPOCH};

    #[test]
    fn resolve_source_root_accepts_absolute_paths_without_trusted_roots() {
        let absolute = unique_temp_path("你好").join("notes");
        let resolved = resolve_source_root(
            absolute.to_string_lossy().as_ref(),
            &LocalPathRoots::new(None, None),
        )
        .expect("resolve absolute path without workspace root");

        assert_eq!(resolved, absolute);
    }

    #[test]
    fn sources_require_workspace_root_only_for_workspace_relative_sources() {
        let absolute = unique_temp_path("absolute-source")
            .to_string_lossy()
            .to_string();

        assert!(!sources_require_workspace_root(&[
            absolute,
            "notes/manual".to_string(),
        ]));
        assert!(sources_require_workspace_root(&[
            "workspace/notes".to_string()
        ]));
        assert!(sources_require_workspace_root(&[
            "workspace\\notes".to_string()
        ]));
    }

    #[test]
    fn resolve_source_note_target_rejects_sibling_directory_with_shared_prefix() {
        let allowed_root = unique_temp_path("allowed-root");
        let sibling_root = PathBuf::from(format!("{}-archive", allowed_root.to_string_lossy()));
        fs::create_dir_all(&allowed_root).expect("create allowed root");
        fs::create_dir_all(&sibling_root).expect("create sibling root");

        let sibling_note = sibling_root.join("secret.md");
        fs::write(&sibling_note, "# Secret\n").expect("write sibling note");

        let error = resolve_source_note_target(
            sibling_note.to_string_lossy().as_ref(),
            std::slice::from_ref(&allowed_root),
        )
        .expect_err("reject sibling path outside configured root");

        assert!(error.contains("outside the configured task source roots"));
    }

    #[test]
    fn resolve_source_note_target_rejects_non_markdown_files_inside_source_root() {
        let allowed_root = unique_temp_path("non-markdown-root");
        fs::create_dir_all(&allowed_root).expect("create allowed root");

        let plain_text_note = allowed_root.join("secret.txt");
        fs::write(&plain_text_note, "secret").expect("write plain text note");

        let error = resolve_source_note_target(
            plain_text_note.to_string_lossy().as_ref(),
            std::slice::from_ref(&allowed_root),
        )
        .expect_err("reject non-markdown file inside configured root");

        assert!(error.contains("not a markdown file"));
    }

    fn unique_temp_path(name: &str) -> PathBuf {
        let unique = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("read system time")
            .as_nanos();

        env::temp_dir().join(format!("cialloclaw-source-note-{unique}-{name}"))
    }
}
