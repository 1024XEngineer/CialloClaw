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
