use serde::Serialize;
use std::time::{SystemTime, UNIX_EPOCH};

/// SelectionPageContextPayload keeps the smallest page-context payload needed by
/// shell-ball while platform adapters evolve independently.
#[derive(Clone, Serialize)]
pub struct SelectionPageContextPayload {
    pub title: String,
    pub url: String,
    pub app_name: String,
    pub window_title: Option<String>,
    pub visible_text: Option<String>,
    pub hover_target: Option<String>,
}

/// SelectionSnapshotPayload is the host-side selection snapshot forwarded to the
/// desktop frontend.
#[derive(Clone, Serialize)]
pub struct SelectionSnapshotPayload {
    pub text: String,
    pub page_context: SelectionPageContextPayload,
    pub source: String,
    pub updated_at: String,
}

impl SelectionSnapshotPayload {
    /// Creates a selection snapshot with a monotonic string timestamp suitable
    /// for frontend diffing.
    pub fn new(text: String, page_context: SelectionPageContextPayload, source: &str) -> Self {
        let updated_at = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|duration| duration.as_millis().to_string())
            .unwrap_or_else(|_| "0".to_string());

        Self {
            text,
            page_context,
            source: source.to_string(),
            updated_at,
        }
    }
}
