use serde::Serialize;

/// ActiveWindowContextPayload captures the current foreground desktop window and
/// optional browser URL without exposing Windows-only details to the frontend.
#[derive(Clone, Serialize)]
pub struct ActiveWindowContextPayload {
    pub app_name: String,
    pub process_path: Option<String>,
    pub process_id: Option<u32>,
    pub title: Option<String>,
    pub url: Option<String>,
    pub browser_kind: String,
    pub window_switch_count: Option<u32>,
    pub page_switch_count: Option<u32>,
}

#[cfg(test)]
mod tests {
    use super::ActiveWindowContextPayload;

    #[test]
    fn active_window_context_payload_serializes_attach_hints() {
        let payload = ActiveWindowContextPayload {
            app_name: "Chrome".to_string(),
            process_path: Some("C:/Program Files/Google/Chrome/Application/chrome.exe".to_string()),
            process_id: Some(4412),
            title: Some("Build Dashboard".to_string()),
            url: Some("https://example.com/build".to_string()),
            browser_kind: "chrome".to_string(),
            window_switch_count: Some(2),
            page_switch_count: Some(1),
        };

        let serialized = serde_json::to_value(payload).expect("payload should serialize");
        assert_eq!(serialized["browser_kind"], "chrome");
        assert_eq!(serialized["process_id"], 4412);
        assert_eq!(
            serialized["process_path"],
            "C:/Program Files/Google/Chrome/Application/chrome.exe"
        );
    }
}
