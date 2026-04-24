use crate::local_path::LocalPathRoots;
use serde::Serialize;
use std::collections::HashSet;
use std::fs;
use std::path::{Path, PathBuf};
use std::time::{SystemTime, UNIX_EPOCH};

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

/// DesktopSourceNoteSnapshot returns the current configured source roots plus
/// the markdown notes discovered under those roots.
#[derive(Clone, Serialize)]
pub struct DesktopSourceNoteSnapshot {
    pub default_source_root: Option<String>,
    pub notes: Vec<DesktopSourceNoteDocument>,
    pub source_roots: Vec<String>,
}

/// Loads every markdown file found under the configured task-source roots.
pub fn load_source_notes(
    raw_sources: &[String],
    roots: &LocalPathRoots,
) -> Result<DesktopSourceNoteSnapshot, String> {
    let resolved_roots = resolve_source_roots(raw_sources, roots)?;
    let mut notes = Vec::new();

    for source_root in &resolved_roots {
        if !source_root.exists() {
            continue;
        }
        if !source_root.is_dir() {
            return Err(format!(
                "task source is not a directory: {}",
                source_root.display()
            ));
        }

        let mut markdown_paths = Vec::new();
        collect_markdown_files(source_root, &mut markdown_paths)?;
        for markdown_path in markdown_paths {
            notes.push(build_source_note_document(&markdown_path, source_root)?);
        }
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

/// Creates one markdown note file under the first configured task-source root.
pub fn create_source_note(
    raw_sources: &[String],
    roots: &LocalPathRoots,
    content: &str,
) -> Result<DesktopSourceNoteDocument, String> {
    let resolved_roots = resolve_source_roots(raw_sources, roots)?;
    let target_root = resolved_roots
        .first()
        .ok_or_else(|| "task source list is empty".to_string())?;
    fs::create_dir_all(target_root).map_err(|error| {
        format!(
            "failed to create task source directory {}: {error}",
            target_root.display()
        )
    })?;

    let normalized_content = normalize_markdown_content(content);
    let target_path = build_unique_note_path(target_root, &normalized_content);
    fs::write(&target_path, normalized_content).map_err(|error| {
        format!(
            "failed to write source note {}: {error}",
            target_path.display()
        )
    })?;

    build_source_note_document(&target_path, target_root)
}

/// Saves the updated markdown content back into an existing source note file.
pub fn save_source_note(
    raw_sources: &[String],
    roots: &LocalPathRoots,
    raw_path: &str,
    content: &str,
) -> Result<DesktopSourceNoteDocument, String> {
    let resolved_roots = resolve_source_roots(raw_sources, roots)?;
    let canonical_target = PathBuf::from(raw_path.trim())
        .canonicalize()
        .map_err(|error| format!("failed to resolve source note {}: {error}", raw_path.trim()))?;
    let source_root = match_source_root(&canonical_target, &resolved_roots)?;
    let normalized_content = normalize_markdown_content(content);

    fs::write(&canonical_target, normalized_content).map_err(|error| {
        format!(
            "failed to save source note {}: {error}",
            canonical_target.display()
        )
    })?;

    build_source_note_document(&canonical_target, source_root)
}

fn resolve_source_roots(
    raw_sources: &[String],
    roots: &LocalPathRoots,
) -> Result<Vec<PathBuf>, String> {
    let mut seen = HashSet::new();
    let mut resolved = Vec::new();

    for raw_source in raw_sources {
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

fn build_unique_note_path(root: &Path, content: &str) -> PathBuf {
    let base_name = build_note_file_name(content);
    let mut candidate = root.join(&base_name);
    if !candidate.exists() {
        return candidate;
    }

    let stem = candidate
        .file_stem()
        .and_then(|value| value.to_str())
        .unwrap_or("note")
        .to_string();
    let extension = candidate
        .extension()
        .and_then(|value| value.to_str())
        .unwrap_or("md")
        .to_string();

    let mut suffix = 2_u32;
    while candidate.exists() {
        candidate = root.join(format!("{stem}-{suffix}.{extension}"));
        suffix += 1;
    }

    candidate
}

fn build_note_file_name(content: &str) -> String {
    let title = derive_note_title(content, "note.md");
    let slug = slugify_title(&title);
    let timestamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis();

    format!("{slug}-{timestamp}.md")
}

fn slugify_title(title: &str) -> String {
    let mut slug = String::new();
    let mut last_was_dash = false;

    for character in title.chars() {
        if character.is_ascii_alphanumeric() {
            slug.push(character.to_ascii_lowercase());
            last_was_dash = false;
            continue;
        }

        if character.is_whitespace() || character == '-' || character == '_' {
            if !last_was_dash && !slug.is_empty() {
                slug.push('-');
                last_was_dash = true;
            }
        }
    }

    let slug = slug.trim_matches('-').to_string();
    if slug.is_empty() {
        "note".to_string()
    } else {
        slug
    }
}

fn match_source_root<'a>(target: &Path, roots: &'a [PathBuf]) -> Result<&'a PathBuf, String> {
    roots
        .iter()
        .find(|root| target.starts_with(root))
        .ok_or_else(|| {
            format!(
                "source note path is outside the configured task source roots: {}",
                target.display()
            )
        })
}

#[cfg(test)]
mod tests {
    use super::{resolve_source_root, sources_require_workspace_root};
    use crate::local_path::LocalPathRoots;
    use std::env;
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

    fn unique_temp_path(name: &str) -> PathBuf {
        let unique = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("read system time")
            .as_nanos();

        env::temp_dir().join(format!("cialloclaw-source-note-{unique}-{name}"))
    }
}
