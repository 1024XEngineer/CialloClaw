package orchestrator

import "testing"

func TestBrowserIntentDefaults(t *testing.T) {
	title, preview, bubble := resultSpecFromIntent(map[string]any{"name": "browser_snapshot"})
	if title != "浏览器快照结果" || preview != "结果已通过气泡返回" || bubble == "" {
		t.Fatalf("unexpected browser snapshot defaults: title=%q preview=%q bubble=%q", title, preview, bubble)
	}
	if deliveryTypeFromIntent(map[string]any{"name": "browser_navigate"}) != "bubble" {
		t.Fatal("expected browser_navigate to default to bubble delivery")
	}
	if !isMutatingToolCall("browser_interact") {
		t.Fatal("expected browser_interact to count as mutating")
	}
	if isMutatingToolCall("browser_snapshot") {
		t.Fatal("expected browser_snapshot to stay non-mutating")
	}
}
