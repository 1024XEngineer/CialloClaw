use serde::Serialize;
use std::time::{SystemTime, UNIX_EPOCH};

/// SelectionPageContextPayload carries the page-context fields that shell-ball
/// selected-text entry needs, including browser attach hints for real-browser
/// takeover planning.
#[derive(Clone, Serialize)]
pub struct SelectionPageContextPayload {
    pub title: String,
    pub url: String,
    pub app_name: String,
    pub window_title: Option<String>,
    pub visible_text: Option<String>,
    pub hover_target: Option<String>,
    pub browser_kind: String,
    pub process_path: Option<String>,
    pub process_id: Option<u32>,
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

#[cfg(test)]
mod tests {
    use super::{SelectionPageContextPayload, SelectionSnapshotPayload};

    #[test]
    fn selection_snapshot_payload_serializes_attach_hints() {
        let payload = SelectionSnapshotPayload::new(
            "selected text".to_string(),
            SelectionPageContextPayload {
                title: "Release Notes".to_string(),
                url: "native://windows-uia-selection".to_string(),
                app_name: "chrome".to_string(),
                window_title: Some("Release Notes".to_string()),
                visible_text: Some("Selected browser text".to_string()),
                hover_target: Some("Publish button".to_string()),
                browser_kind: "chrome".to_string(),
                process_path: Some(
                    "C:/Program Files/Google/Chrome/Application/chrome.exe".to_string(),
                ),
                process_id: Some(4412),
            },
            "windows_uia",
        );

        let serialized = serde_json::to_value(payload).expect("snapshot should serialize");
        assert_eq!(serialized["page_context"]["browser_kind"], "chrome");
        assert_eq!(serialized["page_context"]["process_id"], 4412);
        assert_eq!(
            serialized["page_context"]["process_path"],
            "C:/Program Files/Google/Chrome/Application/chrome.exe"
        );
        assert_eq!(serialized["page_context"]["window_title"], "Release Notes");
    }
}
