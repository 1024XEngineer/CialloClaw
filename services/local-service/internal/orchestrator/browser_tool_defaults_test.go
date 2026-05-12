package orchestrator

import (
	"testing"

	"github.com/cialloclaw/cialloclaw/services/local-service/internal/presentation"
)

func TestBrowserIntentDefaults(t *testing.T) {
	title, preview, bubble := resultSpecFromIntent(map[string]any{"name": "browser_snapshot"})
	spec := presentation.ResultSpecMessagesForIntent("browser_snapshot")
	rendered := presentation.RenderResultSpec("browser_snapshot")
	if spec.Title.Key != presentation.MessageResultTitleBrowserSnap {
		t.Fatalf("expected browser snapshot semantic title key, got %s", spec.Title.Key)
	}
	if spec.Preview.Key != presentation.MessagePreviewBubble {
		t.Fatalf("expected browser snapshot semantic preview key, got %s", spec.Preview.Key)
	}
	if title != rendered.Title || preview != rendered.Preview || bubble != rendered.BubbleText {
		t.Fatalf("unexpected browser snapshot defaults: title=%q preview=%q bubble=%q rendered=%+v", title, preview, bubble, rendered)
	}
	if deliveryTypeFromIntent(map[string]any{"name": "browser_navigate"}) != "bubble" {
		t.Fatal("expected browser_navigate to default to bubble delivery")
	}
	if deliveryTypeFromIntent(map[string]any{"name": "web_search"}) != "result_page" {
		t.Fatal("expected web_search to default to result_page delivery")
	}
	if !isMutatingToolCall("browser_interact") {
		t.Fatal("expected browser_interact to count as mutating")
	}
	if isMutatingToolCall("browser_snapshot") {
		t.Fatal("expected browser_snapshot to stay non-mutating")
	}
}
